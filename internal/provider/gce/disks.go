// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package gce

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/juju/collections/set"
	"github.com/juju/errors"

	"github.com/juju/juju/environs/tags"
	"github.com/juju/juju/internal/provider/common"
	"github.com/juju/juju/internal/provider/gce/internal/google"
	"github.com/juju/juju/internal/storage"
	"github.com/juju/juju/internal/uuid"
)

const (
	gceStorageProviderType = storage.ProviderType("gce")
	diskTypeAttribute      = "disk-type"
)

// RecommendedStoragePoolForKind returns the recommended storage pool to use for
// the given storage kind. If no pool can be recommended nil is returned.
//
// Implements [storage.PoolAdvisor] interface.
func (*environ) RecommendedStoragePoolForKind(
	kind storage.StorageKind,
) *storage.Config {
	return common.GetCommonRecommendedIAASPoolForKind(kind)
}

// StorageProviderTypes implements storage.ProviderRegistry.
func (*environ) StorageProviderTypes() ([]storage.ProviderType, error) {
	return append(
		common.CommonIAASStorageProviderTypes(),
		gceStorageProviderType,
	), nil
}

// StorageProvider implements storage.ProviderRegistry.
func (env *environ) StorageProvider(t storage.ProviderType) (storage.Provider, error) {
	switch t {
	case gceStorageProviderType:
		return &storageProvider{env}, nil
	default:
		return common.GetCommonIAASStorageProvider(t)
	}
}

type storageProvider struct {
	env *environ
}

var _ storage.Provider = (*storageProvider)(nil)

func (g *storageProvider) ValidateForK8s(map[string]any) error {
	// no validation required
	return nil
}

func (g *storageProvider) ValidateConfig(cfg *storage.Config) error {
	return nil
}

func (g *storageProvider) Supports(k storage.StorageKind) bool {
	return k == storage.StorageKindBlock
}

func (g *storageProvider) Scope() storage.Scope {
	return storage.ScopeEnviron
}

func (g *storageProvider) Dynamic() bool {
	return true
}

func (e *storageProvider) Releasable() bool {
	return true
}

// DefaultPools returns the default pools available through the gce provider.
// By default a pool by the same name as the provider is offered.
//
// Implements [storage.Provider] interface.
func (g *storageProvider) DefaultPools() []*storage.Config {
	defaultPool, _ := storage.NewConfig(
		gceStorageProviderType.String(), gceStorageProviderType, storage.Attrs{},
	)

	return []*storage.Config{defaultPool}
}

func (g *storageProvider) FilesystemSource(providerConfig *storage.Config) (storage.FilesystemSource, error) {
	return nil, errors.NotSupportedf("filesystems")
}

type volumeSource struct {
	gce                   ComputeService
	credentialInvalidator common.CredentialInvalidator
	envName               string // non-unique, informational only
	modelUUID             string
}

func (g *storageProvider) VolumeSource(cfg *storage.Config) (storage.VolumeSource, error) {
	environConfig := g.env.Config()
	source := &volumeSource{
		gce:                   g.env.gce,
		credentialInvalidator: g.env.CredentialInvalidator,
		envName:               environConfig.Name(),
		modelUUID:             environConfig.UUID(),
	}
	return source, nil
}

type instanceCache map[string]*computepb.Instance

func (c instanceCache) update(gceClient ComputeService, ctx context.Context, ids ...string) error {
	if len(ids) == 1 {
		if _, ok := c[ids[0]]; ok {
			return nil
		}
	}
	idMap := make(map[string]int, len(ids))
	for _, id := range ids {
		idMap[id] = 0
	}
	instances, err := gceClient.Instances(ctx, "", google.StatusRunning)
	if err != nil {
		return errors.Annotate(err, "querying instance details")
	}
	for _, instance := range instances {
		if _, ok := idMap[instance.GetName()]; !ok {
			continue
		}
		c[instance.GetName()] = instance
	}
	return nil
}

func (c instanceCache) get(id string) (*computepb.Instance, error) {
	inst, ok := c[id]
	if !ok {
		return nil, errors.Errorf("cannot attach to non-running instance %v", id)
	}
	return inst, nil
}

func (v *volumeSource) CreateVolumes(ctx context.Context, params []storage.VolumeParams) (_ []storage.CreateVolumesResult, err error) {
	results := make([]storage.CreateVolumesResult, len(params))
	instanceIds := set.NewStrings()
	for i, p := range params {
		if err := v.ValidateVolumeParams(p); err != nil {
			results[i].Error = err
			continue
		}
		instanceIds.Add(string(p.Attachment.InstanceId))
	}

	instances := make(instanceCache)
	if instanceIds.Size() > 1 {
		if err := instances.update(v.gce, ctx, instanceIds.Values()...); err != nil {
			logger.Debugf(ctx, "querying running instances: %v", err)
			// We ignore the error, because we don't want an invalid
			// InstanceId reference from one VolumeParams to prevent
			// the creation of another volume.
			// ... Unless the error is due to an invalid credential, in which case, continuing with this call
			// is pointless and creates an unnecessary churn: we know all calls will fail with the same error.
			if denied, _ := v.credentialInvalidator.MaybeInvalidateCredentialError(ctx, err); denied {
				return results, err
			}
		}
	}

	for i, p := range params {
		if results[i].Error != nil {
			continue
		}
		volume, attachment, err := v.createOneVolume(ctx, p, instances)
		if err != nil {
			results[i].Error = err
			logger.Errorf(ctx, "could not create one volume (or attach it): %v", err)
			// ... Unless the error is due to an invalid credential, in which case, continuing with this call
			// is pointless and creates an unnecessary churn: we know all calls will fail with the same error.
			if denied, _ := v.credentialInvalidator.MaybeInvalidateCredentialError(ctx, err); denied {
				return results, err
			}
			continue
		}
		results[i].Volume = volume
		results[i].VolumeAttachment = attachment
	}
	return results, nil
}

// gibToMib converts gibibytes to mebibytes.
func gibToMib(g int64) uint64 {
	return uint64(g) * 1024
}

// mibToGib converts mebibytes to gibibytes.
func mibToGib(m uint64) int64 {
	return int64(m / 1024)
}

func nameVolume(zone string) (string, error) {
	volumeUUID, err := uuid.NewUUID()
	if err != nil {
		return "", errors.Annotate(err, "cannot generate uuid to name the volume")
	}
	// type-zone-uuid
	volumeName := fmt.Sprintf("%s--%s", zone, volumeUUID.String())
	return volumeName, nil
}

func (v *volumeSource) createOneVolume(ctx context.Context, p storage.VolumeParams, instances instanceCache) (volume *storage.Volume, volumeAttachment *storage.VolumeAttachment, err error) {
	var volumeName, zone string
	defer func() {
		if err == nil || volumeName == "" {
			return
		}
		if err := v.gce.RemoveDisk(ctx, zone, volumeName); err != nil {
			logger.Errorf(ctx, "error cleaning up volume %v: %v", volumeName, v.credentialInvalidator.HandleCredentialError(ctx, err))
		}
	}()

	instId := string(p.Attachment.InstanceId)
	if err := instances.update(v.gce, ctx, instId); err != nil {
		return nil, nil, errors.Annotatef(err, "cannot add %q to instance cache", instId)
	}
	inst, err := instances.get(instId)
	if err != nil {
		// Can't create the volume without the instance,
		// because we need to know what its AZ is.
		return nil, nil, errors.Annotatef(err, "cannot obtain %q from instance cache", instId)
	}

	diskType := google.DiskPersistentStandard
	if val, ok := p.Attributes[diskTypeAttribute].(string); ok {
		dt := google.DiskType(val)
		switch dt {
		case google.DiskPersistentSSD, google.DiskPersistentStandard:
			diskType = dt
		case google.DiskLocalSSD:
			return nil, nil, errors.NotValidf("local SSD disk storage")
		default:
		}
	}

	// volume.Size is expressed in MiB, but GCE only accepts sizes in GiB.
	size := mibToGib(p.Size)
	if size < google.MinDiskSizeGB {
		size = google.MinDiskSizeGB
	}
	zone = path.Base(inst.GetZone())
	volumeName, err = nameVolume(zone)
	if err != nil {
		return nil, nil, errors.Annotate(err, "cannot create a new volume name")
	}
	// TODO(perrito666) the volumeName is arbitrary and it was crafted this
	// way to help solve the need to have zone all over the place.
	disk := &computepb.Disk{
		Name:   &volumeName,
		SizeGb: ptr(size),
		Type:   ptr(string(diskType)),
		Labels: resourceTagsToDiskLabels(p.ResourceTags),
	}
	err = v.gce.CreateDisks(ctx, zone, []*computepb.Disk{disk})
	if err != nil {
		return nil, nil, errors.Annotate(v.credentialInvalidator.HandleCredentialError(ctx, err), "cannot create disk")
	}

	attachedDisk, err := v.attachOneVolume(ctx, disk.GetName(), google.ModeRW, inst.GetName())
	if err != nil {
		return nil, nil, errors.Annotatef(err, "attaching %q to %q", disk.GetName(), instId)
	}

	volume = &storage.Volume{
		Tag: p.Tag,
		VolumeInfo: storage.VolumeInfo{
			VolumeId:   disk.GetName(),
			Size:       gibToMib(disk.GetSizeGb()),
			Persistent: true,
		},
	}

	volumeAttachment = &storage.VolumeAttachment{
		Volume:  p.Tag,
		Machine: p.Attachment.Machine,
		VolumeAttachmentInfo: storage.VolumeAttachmentInfo{
			DeviceLink: fmt.Sprintf(
				"/dev/disk/by-id/google-%s",
				attachedDisk.GetDeviceName(),
			),
		},
	}

	return volume, volumeAttachment, nil
}

func (v *volumeSource) DestroyVolumes(ctx context.Context, volNames []string) ([]error, error) {
	return v.foreachVolume(ctx, volNames, v.destroyOneVolume), nil
}

func (v *volumeSource) ReleaseVolumes(ctx context.Context, volNames []string) ([]error, error) {
	return v.foreachVolume(ctx, volNames, v.releaseOneVolume), nil
}

func (v *volumeSource) foreachVolume(ctx context.Context, volNames []string, f func(context.Context, string) error) []error {
	var wg sync.WaitGroup
	wg.Add(len(volNames))
	results := make([]error, len(volNames))
	for i, volumeName := range volNames {
		go func(i int, volumeName string) {
			defer wg.Done()
			results[i] = f(ctx, volumeName)
		}(i, volumeName)
	}
	wg.Wait()
	return results
}

func parseVolumeId(volName string) (string, string, error) {
	idRest := strings.SplitN(volName, "--", 2)
	if len(idRest) != 2 {
		return "", "", errors.New(fmt.Sprintf("malformed volume id %q", volName))
	}
	zone := idRest[0]
	volumeUUID := idRest[1]
	return zone, volumeUUID, nil
}

func isValidVolume(volumeName string) bool {
	_, _, err := parseVolumeId(volumeName)
	return err == nil
}

func (v *volumeSource) destroyOneVolume(ctx context.Context, volName string) error {
	zone, _, err := parseVolumeId(volName)
	if err != nil {
		return errors.Annotatef(err, "invalid volume id %q", volName)
	}
	if err := v.gce.RemoveDisk(ctx, zone, volName); err != nil {
		return errors.Annotatef(v.credentialInvalidator.HandleCredentialError(ctx, err), "cannot destroy volume %q", volName)
	}
	return nil
}

func (v *volumeSource) releaseOneVolume(ctx context.Context, volName string) error {
	zone, _, err := parseVolumeId(volName)
	if err != nil {
		return errors.Annotatef(err, "invalid volume id %q", volName)
	}
	disk, err := v.gce.Disk(ctx, zone, volName)
	if err != nil {
		return v.credentialInvalidator.HandleCredentialError(ctx, err)
	}
	switch google.DiskStatus(disk.GetStatus()) {
	case google.StatusReady, google.StatusFailed:
	default:
		return errors.Errorf(
			"cannot release volume %q with status %q",
			volName, disk.GetStatus(),
		)
	}
	if len(disk.GetUsers()) > 0 {
		attachedInstances := make([]string, len(disk.GetUsers()))
		for i, u := range disk.GetUsers() {
			attachedInstances[i] = path.Base(u)
		}
		return errors.Errorf(
			"cannot release volume %q, attached to instances %q",
			volName, attachedInstances,
		)
	}
	delete(disk.Labels, tags.JujuController)
	delete(disk.Labels, tags.JujuModel)
	if err := v.gce.SetDiskLabels(ctx, zone, volName, disk.GetLabelFingerprint(), disk.GetLabels()); err != nil {
		return errors.Annotatef(
			v.credentialInvalidator.HandleCredentialError(ctx, err), "cannot remove labels from volume %q", volName)
	}
	return nil
}

func (v *volumeSource) ListVolumes(ctx context.Context) ([]string, error) {
	var volumes []string
	disks, err := v.gce.Disks(ctx)
	if err != nil {
		return nil, v.credentialInvalidator.HandleCredentialError(ctx, err)
	}
	for _, disk := range disks {
		if !isValidVolume(disk.GetName()) {
			continue
		}
		if disk.Labels[tags.JujuModel] != v.modelUUID {
			continue
		}
		volumes = append(volumes, disk.GetName())
	}
	return volumes, nil
}

// ImportVolume is specified on the storage.VolumeImporter interface.
func (v *volumeSource) ImportVolume(ctx context.Context, volName string, storageName string, tags map[string]string, force bool) (storage.VolumeInfo, error) {
	zone, _, err := parseVolumeId(volName)
	if err != nil {
		return storage.VolumeInfo{}, errors.Annotatef(err, "cannot get volume %q", volName)
	}
	disk, err := v.gce.Disk(ctx, zone, volName)
	if err != nil {
		return storage.VolumeInfo{}, errors.Annotatef(
			v.credentialInvalidator.HandleCredentialError(ctx, err), "cannot get volume %q", volName)
	}
	if google.DiskStatus(disk.GetStatus()) != google.StatusReady {
		return storage.VolumeInfo{}, errors.Errorf(
			"cannot import volume %q with status %q",
			volName, disk.GetStatus(),
		)
	}
	if disk.Labels == nil {
		disk.Labels = make(map[string]string)
	}
	for k, v := range resourceTagsToDiskLabels(tags) {
		disk.Labels[k] = v
	}
	if err := v.gce.SetDiskLabels(ctx, zone, volName, disk.GetLabelFingerprint(), disk.GetLabels()); err != nil {
		return storage.VolumeInfo{}, errors.Annotatef(
			v.credentialInvalidator.HandleCredentialError(ctx, err), "cannot update labels on volume %q", volName)
	}
	return storage.VolumeInfo{
		VolumeId:   disk.GetName(),
		Size:       gibToMib(disk.GetSizeGb()),
		Persistent: true,
	}, nil
}

func (v *volumeSource) DescribeVolumes(ctx context.Context, volNames []string) ([]storage.DescribeVolumesResult, error) {
	results := make([]storage.DescribeVolumesResult, len(volNames))
	for i, vol := range volNames {
		res, err := v.describeOneVolume(ctx, vol)
		if err != nil {
			return nil, errors.Annotate(err, "cannot describe volumes")
		}
		results[i] = res
	}
	return results, nil
}

func (v *volumeSource) describeOneVolume(ctx context.Context, volName string) (storage.DescribeVolumesResult, error) {
	zone, _, err := parseVolumeId(volName)
	if err != nil {
		return storage.DescribeVolumesResult{}, errors.Annotatef(err, "cannot describe %q", volName)
	}
	disk, err := v.gce.Disk(ctx, zone, volName)
	if err != nil {
		return storage.DescribeVolumesResult{}, errors.Annotatef(
			v.credentialInvalidator.HandleCredentialError(ctx, err), "cannot get volume %q", volName)
	}
	desc := storage.DescribeVolumesResult{
		VolumeInfo: &storage.VolumeInfo{
			Size:     gibToMib(disk.GetSizeGb()),
			VolumeId: disk.GetName(),
		},
		Error: nil,
	}
	return desc, nil
}

// TODO(perrito666) These rules are yet to be defined.
func (v *volumeSource) ValidateVolumeParams(params storage.VolumeParams) error {
	return nil
}

func (v *volumeSource) AttachVolumes(ctx context.Context, attachParams []storage.VolumeAttachmentParams) ([]storage.AttachVolumesResult, error) {
	results := make([]storage.AttachVolumesResult, len(attachParams))
	for i, attachment := range attachParams {
		volumeName := attachment.VolumeId
		mode := google.ModeRW
		if attachment.ReadOnly {
			mode = google.ModeRW
		}
		instanceId := attachment.InstanceId
		attached, err := v.attachOneVolume(ctx, volumeName, mode, instanceId.String())
		if err != nil {
			logger.Errorf(ctx, "could not attach %q to %q: %v", volumeName, instanceId, err)
			results[i].Error = err
			// ... Unless the error is due to an invalid credential, in which case, continuing with this call
			// is pointless and creates an unnecessary churn: we know all calls will fail with the same error.
			if denied, err := v.credentialInvalidator.MaybeInvalidateCredentialError(ctx, err); denied {
				return results, err
			}
			continue
		}
		results[i].VolumeAttachment = &storage.VolumeAttachment{
			Volume:  attachment.Volume,
			Machine: attachment.Machine,
			VolumeAttachmentInfo: storage.VolumeAttachmentInfo{
				DeviceLink: fmt.Sprintf(
					"/dev/disk/by-id/google-%s",
					attached.GetDeviceName(),
				),
			},
		}
	}
	return results, nil
}

// sourceToVolumeName will return the disk Name part of a
// source URL for a compute disk, compute is a bit inconsistent
// on its handling of disk resources, when used in requests it will
// take the disk.Name but when used as a parameter it will take
// the source url.
// The source url (short) format is:
// /projects/project/zones/zone/disks/disk
// the relevant part is disk.
func sourceToVolumeName(source string) string {
	if source == "" {
		return ""
	}
	parts := strings.Split(source, "/")
	if len(parts) == 1 {
		return source
	}
	lastItem := len(parts) - 1
	return parts[lastItem]
}

func (v *volumeSource) attachOneVolume(ctx context.Context, volumeName string, mode google.DiskMode, instanceId string) (*computepb.AttachedDisk, error) {
	zone, _, err := parseVolumeId(volumeName)
	if err != nil {
		return nil, errors.Annotate(err, "invalid volume name")
	}
	instanceDisks, err := v.gce.InstanceDisks(ctx, zone, instanceId)
	if err != nil {
		return nil, errors.Annotate(
			v.credentialInvalidator.HandleCredentialError(ctx, err), "cannot verify if the disk is already in the instance")
	}
	// Is it already attached?
	for _, disk := range instanceDisks {
		if sourceToVolumeName(disk.GetSource()) == volumeName {
			return disk, nil
		}
	}

	attachment, err := v.gce.AttachDisk(ctx, zone, volumeName, instanceId, mode)
	if err != nil {
		return nil, errors.Annotate(
			v.credentialInvalidator.HandleCredentialError(ctx, err), "cannot attach volume")
	}
	return attachment, nil
}

func (v *volumeSource) DetachVolumes(ctx context.Context, attachParams []storage.VolumeAttachmentParams) ([]error, error) {
	result := make([]error, len(attachParams))
	for i, volumeAttachment := range attachParams {
		err := v.detachOneVolume(ctx, volumeAttachment)
		if denied, err := v.credentialInvalidator.MaybeInvalidateCredentialError(ctx, err); denied {
			// no need to continue as we'll keep getting the same invalid credential error.
			return result, err
		}
		result[i] = err
	}
	return result, nil
}

func (v *volumeSource) detachOneVolume(ctx context.Context, attachParam storage.VolumeAttachmentParams) error {
	instId := attachParam.InstanceId
	volumeName := attachParam.VolumeId
	zone, _, err := parseVolumeId(volumeName)
	if err != nil {
		return errors.Annotatef(err, "%q is not a valid volume id", volumeName)
	}
	err = v.gce.DetachDisk(ctx, zone, string(instId), volumeName)
	if err != nil {
		return v.credentialInvalidator.HandleCredentialError(ctx, err)
	}
	return nil
}

// resourceTagsToDiskLabels translates a set of
// resource tags, provided by Juju, to disk labels.
func resourceTagsToDiskLabels(in map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range in {
		// Only the controller and model UUID tags are carried
		// over, as they're known not to conflict with GCE's
		// rules regarding label values.
		switch k {
		case tags.JujuController, tags.JujuModel:
			out[k] = v
		}
	}
	return out
}
