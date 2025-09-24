// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package gce

import (
	"fmt"
	"path"
	"strings"
	"sync"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/juju/collections/set"
	"github.com/juju/errors"
	"github.com/juju/utils/v3"

	"github.com/juju/juju/environs/context"
	"github.com/juju/juju/environs/tags"
	"github.com/juju/juju/internal/provider/gce/internal/google"
	"github.com/juju/juju/storage"
)

const (
	storageProviderType = storage.ProviderType("gce")
	diskTypeAttribute   = "disk-type"
)

// StorageProviderTypes implements storage.ProviderRegistry.
func (env *environ) StorageProviderTypes() ([]storage.ProviderType, error) {
	return []storage.ProviderType{storageProviderType}, nil
}

// StorageProvider implements storage.ProviderRegistry.
func (env *environ) StorageProvider(t storage.ProviderType) (storage.Provider, error) {
	if t == storageProviderType {
		return &storageProvider{env}, nil
	}
	return nil, errors.NotFoundf("storage provider %q", t)
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

func (g *storageProvider) DefaultPools() []*storage.Config {
	// TODO(perrito666) Add explicit pools.
	return nil
}

func (g *storageProvider) FilesystemSource(providerConfig *storage.Config) (storage.FilesystemSource, error) {
	return nil, errors.NotSupportedf("filesystems")
}

type volumeSource struct {
	gce       ComputeService
	envName   string // non-unique, informational only
	modelUUID string
}

func (g *storageProvider) VolumeSource(cfg *storage.Config) (storage.VolumeSource, error) {
	environConfig := g.env.Config()
	source := &volumeSource{
		gce:       g.env.gce,
		envName:   environConfig.Name(),
		modelUUID: environConfig.UUID(),
	}
	return source, nil
}

type instanceCache map[string]*computepb.Instance

func (c instanceCache) update(gceClient ComputeService, ctx context.ProviderCallContext, ids ...string) error {
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
		return google.HandleCredentialError(errors.Annotate(err, "querying instance details"), ctx)
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

func (v *volumeSource) CreateVolumes(ctx context.ProviderCallContext, params []storage.VolumeParams) (_ []storage.CreateVolumesResult, err error) {
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
			logger.Debugf("querying running instances: %v", err)
			// We ignore the error, because we don't want an invalid
			// InstanceId reference from one VolumeParams to prevent
			// the creation of another volume.
			// ... Unless the error is due to an invalid credential, in which case, continuing with this call
			// is pointless and creates an unnecessary churn: we know all calls will fail with the same error.
			if google.HasDenialStatusCode(err) {
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
			logger.Errorf("could not create one volume (or attach it): %v", err)
			// ... Unless the error is due to an invalid credential, in which case, continuing with this call
			// is pointless and creates an unnecessary churn: we know all calls will fail with the same error.
			if google.HasDenialStatusCode(err) {
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
	volumeUUID, err := utils.NewUUID()
	if err != nil {
		return "", errors.Annotate(err, "cannot generate uuid to name the volume")
	}
	// type-zone-uuid
	volumeName := fmt.Sprintf("%s--%s", zone, volumeUUID.String())
	return volumeName, nil
}

func (v *volumeSource) createOneVolume(ctx context.ProviderCallContext, p storage.VolumeParams, instances instanceCache) (volume *storage.Volume, volumeAttachment *storage.VolumeAttachment, err error) {
	var volumeName, zone string
	defer func() {
		if err == nil || volumeName == "" {
			return
		}
		if err := v.gce.RemoveDisk(ctx, zone, volumeName); err != nil {
			logger.Errorf("error cleaning up volume %v: %v", volumeName, google.HandleCredentialError(err, ctx))
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
		return nil, nil, google.HandleCredentialError(errors.Annotate(err, "cannot create disk"), ctx)
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

func (v *volumeSource) DestroyVolumes(ctx context.ProviderCallContext, volNames []string) ([]error, error) {
	return v.foreachVolume(ctx, volNames, v.destroyOneVolume), nil
}

func (v *volumeSource) ReleaseVolumes(ctx context.ProviderCallContext, volNames []string) ([]error, error) {
	return v.foreachVolume(ctx, volNames, v.releaseOneVolume), nil
}

func (v *volumeSource) foreachVolume(ctx context.ProviderCallContext, volNames []string, f func(context.ProviderCallContext, string) error) []error {
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

func (v *volumeSource) destroyOneVolume(ctx context.ProviderCallContext, volName string) error {
	zone, _, err := parseVolumeId(volName)
	if err != nil {
		return errors.Annotatef(err, "invalid volume id %q", volName)
	}
	if err := v.gce.RemoveDisk(ctx, zone, volName); err != nil {
		return google.HandleCredentialError(errors.Annotatef(err, "cannot destroy volume %q", volName), ctx)
	}
	return nil
}

func (v *volumeSource) releaseOneVolume(ctx context.ProviderCallContext, volName string) error {
	zone, _, err := parseVolumeId(volName)
	if err != nil {
		return errors.Annotatef(err, "invalid volume id %q", volName)
	}
	disk, err := v.gce.Disk(ctx, zone, volName)
	if err != nil {
		return google.HandleCredentialError(errors.Trace(err), ctx)
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
		return google.HandleCredentialError(errors.Annotatef(err, "cannot remove labels from volume %q", volName), ctx)
	}
	return nil
}

func (v *volumeSource) ListVolumes(ctx context.ProviderCallContext) ([]string, error) {
	var volumes []string
	disks, err := v.gce.Disks(ctx)
	if err != nil {
		return nil, google.HandleCredentialError(errors.Trace(err), ctx)
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
func (v *volumeSource) ImportVolume(ctx context.ProviderCallContext, volName string, storageName string, tags map[string]string, force bool) (storage.VolumeInfo, error) {
	zone, _, err := parseVolumeId(volName)
	if err != nil {
		return storage.VolumeInfo{}, errors.Annotatef(err, "cannot get volume %q", volName)
	}
	disk, err := v.gce.Disk(ctx, zone, volName)
	if err != nil {
		return storage.VolumeInfo{}, google.HandleCredentialError(errors.Annotatef(err, "cannot get volume %q", volName), ctx)
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
		return storage.VolumeInfo{}, google.HandleCredentialError(errors.Annotatef(err, "cannot update labels on volume %q", volName), ctx)
	}
	return storage.VolumeInfo{
		VolumeId:   disk.GetName(),
		Size:       gibToMib(disk.GetSizeGb()),
		Persistent: true,
	}, nil
}

func (v *volumeSource) DescribeVolumes(ctx context.ProviderCallContext, volNames []string) ([]storage.DescribeVolumesResult, error) {
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

func (v *volumeSource) describeOneVolume(ctx context.ProviderCallContext, volName string) (storage.DescribeVolumesResult, error) {
	zone, _, err := parseVolumeId(volName)
	if err != nil {
		return storage.DescribeVolumesResult{}, errors.Annotatef(err, "cannot describe %q", volName)
	}
	disk, err := v.gce.Disk(ctx, zone, volName)
	if err != nil {
		return storage.DescribeVolumesResult{}, google.HandleCredentialError(errors.Annotatef(err, "cannot get volume %q", volName), ctx)
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

func (v *volumeSource) AttachVolumes(ctx context.ProviderCallContext, attachParams []storage.VolumeAttachmentParams) ([]storage.AttachVolumesResult, error) {
	results := make([]storage.AttachVolumesResult, len(attachParams))
	for i, attachment := range attachParams {
		volumeName := attachment.VolumeId
		mode := google.ModeRW
		if attachment.ReadOnly {
			mode = google.ModeRW
		}
		instanceId := attachment.InstanceId
		attached, err := v.attachOneVolume(ctx, volumeName, mode, string(instanceId))
		if err != nil {
			logger.Errorf("could not attach %q to %q: %v", volumeName, instanceId, err)
			results[i].Error = err
			// ... Unless the error is due to an invalid credential, in which case, continuing with this call
			// is pointless and creates an unnecessary churn: we know all calls will fail with the same error.
			if google.HasDenialStatusCode(err) {
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

func (v *volumeSource) attachOneVolume(ctx context.ProviderCallContext, volumeName string, mode google.DiskMode, instanceId string) (*computepb.AttachedDisk, error) {
	zone, _, err := parseVolumeId(volumeName)
	if err != nil {
		return nil, errors.Annotate(err, "invalid volume name")
	}
	instanceDisks, err := v.gce.InstanceDisks(ctx, zone, instanceId)
	if err != nil {
		return nil, google.HandleCredentialError(errors.Annotate(err, "cannot verify if the disk is already in the instance"), ctx)
	}
	// Is it already attached?
	for _, disk := range instanceDisks {
		if sourceToVolumeName(disk.GetSource()) == volumeName {
			return disk, nil
		}
	}

	attachment, err := v.gce.AttachDisk(ctx, zone, volumeName, instanceId, mode)
	if err != nil {
		return nil, google.HandleCredentialError(errors.Annotate(err, "cannot attach volume"), ctx)
	}
	return attachment, nil
}

func (v *volumeSource) DetachVolumes(ctx context.ProviderCallContext, attachParams []storage.VolumeAttachmentParams) ([]error, error) {
	result := make([]error, len(attachParams))
	for i, volumeAttachment := range attachParams {
		err := v.detachOneVolume(ctx, volumeAttachment)
		if google.HasDenialStatusCode(err) {
			// no need to continue as we'll keep getting the same invalid credential error.
			return result, err
		}
		result[i] = err
	}
	return result, nil
}

func (v *volumeSource) detachOneVolume(ctx context.ProviderCallContext, attachParam storage.VolumeAttachmentParams) error {
	instId := attachParam.InstanceId
	volumeName := attachParam.VolumeId
	zone, _, err := parseVolumeId(volumeName)
	if err != nil {
		return errors.Annotatef(err, "%q is not a valid volume id", volumeName)
	}
	return google.HandleCredentialError(v.gce.DetachDisk(ctx, zone, string(instId), volumeName), ctx)
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
