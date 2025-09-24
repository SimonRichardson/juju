// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package gce

import (
	"fmt"
	"maps"
	"path"
	"slices"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/juju/errors"

	"github.com/juju/juju/cloud"
	"github.com/juju/juju/cloudconfig/instancecfg"
	"github.com/juju/juju/cloudconfig/providerinit"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/os/ostype"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/context"
	"github.com/juju/juju/environs/imagemetadata"
	"github.com/juju/juju/environs/instances"
	"github.com/juju/juju/internal/provider/common"
	"github.com/juju/juju/internal/provider/gce/internal/google"
)

// StartInstance implements environs.InstanceBroker.
func (env *environ) StartInstance(ctx context.ProviderCallContext, args environs.StartInstanceParams) (*environs.StartInstanceResult, error) {
	// Start a new instance.

	spec, err := env.buildInstanceSpec(ctx, args)
	if err != nil {
		return nil, environs.ZoneIndependentError(err)
	}

	if err := env.finishInstanceConfig(args, spec); err != nil {
		return nil, environs.ZoneIndependentError(err)
	}

	inst, err := env.startInstance(ctx, args, spec.Image.Id, spec.InstanceType.Name)
	if err != nil {
		return nil, errors.Trace(err)
	}
	envInst := newInstance(inst, env)

	// Build the result.
	hwc := env.getHardwareCharacteristics(spec, envInst)
	logger.Infof("started instance %q in zone %q", inst.GetName(), *hwc.AvailabilityZone)
	result := environs.StartInstanceResult{
		Instance: envInst,
		Hardware: hwc,
	}
	return &result, nil
}

// finishInstanceConfig updates args.InstanceConfig in place. Setting up
// the API, StateServing, and SSHkeys information.
func (env *environ) finishInstanceConfig(args environs.StartInstanceParams, spec *instances.InstanceSpec) error {
	if err := args.InstanceConfig.SetTools(args.Tools); err != nil {
		return errors.Trace(err)
	}
	return instancecfg.FinishInstanceConfig(args.InstanceConfig, env.Config())
}

// buildInstanceSpec builds an instance spec from the provided args
// and returns it. This includes pulling the simplestreams data for the
// machine type, region, and other constraints.
func (env *environ) buildInstanceSpec(ctx context.ProviderCallContext, args environs.StartInstanceParams) (*instances.InstanceSpec, error) {
	instTypesAndCosts, err := env.InstanceTypes(ctx, constraints.Value{})
	if err != nil {
		return nil, errors.Trace(err)
	}

	arch, err := args.Tools.OneArch()
	if err != nil {
		return nil, errors.Trace(err)
	}
	spec, err := env.findInstanceSpec(
		&instances.InstanceConstraint{
			Region:      env.cloud.Region,
			Base:        args.InstanceConfig.Base,
			Arch:        arch,
			Constraints: args.Constraints,
		},
		args.ImageMetadata,
		instTypesAndCosts.InstanceTypes,
	)
	return spec, errors.Trace(err)
}

// findInstanceSpec initializes a new instance spec for the given
// constraints and returns it. This only covers populating the
// initial data for the spec.
func (env *environ) findInstanceSpec(
	ic *instances.InstanceConstraint,
	imageMetadata []*imagemetadata.ImageMetadata,
	allInstanceTypes []instances.InstanceType,
) (*instances.InstanceSpec, error) {
	images := instances.ImageMetadataToImages(imageMetadata)
	spec, err := instances.FindInstanceSpec(images, ic, allInstanceTypes)
	return spec, errors.Trace(err)
}

func (env *environ) imageURLBase(os ostype.OSType) (string, error) {
	env.lock.Lock()
	base, useCustomPath := env.ecfg.baseImagePath()
	env.lock.Unlock()
	if useCustomPath {
		return base, nil
	}

	switch os {
	case ostype.Ubuntu:
		switch env.Config().ImageStream() {
		case "daily":
			base = ubuntuDailyImageBasePath
		case "pro":
			base = ubuntuProImageBasePath
		default:
			base = ubuntuImageBasePath
		}
	default:
		return "", errors.Errorf("os %s is not supported on the gce provider", os.String())
	}

	return base, nil
}

// packMetadata composes the provided data into the format required
// by the GCE API.
func packMetadata(data map[string]string) *computepb.Metadata {
	// Sort for testing.
	keys := maps.Keys(data)
	var items []*computepb.Items
	for _, key := range slices.Sorted(keys) {
		localValue := data[key]
		item := computepb.Items{
			Key:   &key,
			Value: &localValue,
		}
		items = append(items, &item)
	}
	return &computepb.Metadata{Items: items}
}

func formatMachineType(zone, name string) string {
	return fmt.Sprintf("zones/%s/machineTypes/%s", zone, name)
}

func (env *environ) serviceAccount(ctx context.ProviderCallContext, args environs.StartInstanceParams) (string, error) {
	var serviceAccount string

	// For controllers, the service account can come from the credential.
	if args.InstanceConfig.IsController() {
		if args.InstanceConfig.Bootstrap != nil && args.InstanceConfig.Bootstrap.BootstrapMachineConstraints.HasInstanceRole() {
			serviceAccount = *args.InstanceConfig.Bootstrap.BootstrapMachineConstraints.InstanceRole
			if serviceAccount != "" {
				logger.Debugf("using bootstrap service account: %s", serviceAccount)
			}
		} else if env.cloud.Credential.AuthType() == cloud.ServiceAccountAuthType {
			serviceAccount = env.cloud.Credential.Attributes()[credServiceAccount]
			if serviceAccount != "" {
				logger.Debugf("using credential service account: %s", serviceAccount)
			}
		}
	}
	if serviceAccount != "" {
		return serviceAccount, nil
	}

	// Next use the constraint value if supplied.
	if args.Constraints.HasInstanceRole() {
		serviceAccount = *args.Constraints.InstanceRole
		if serviceAccount != "" {
			logger.Debugf("using constraints service account: %s", serviceAccount)
		}
	}
	if serviceAccount != "" {
		return serviceAccount, nil
	}

	// Fallback to the project default.
	var err error
	serviceAccount, err = env.gce.DefaultServiceAccount(ctx)
	if err != nil {
		return "", errors.Trace(err)
	}
	logger.Debugf("using project service account: %s", serviceAccount)
	return serviceAccount, nil
}

// startInstance is where the new physical instance is actually
// provisioned, relative to the provided args and spec. Info for that
// low-level instance is returned.
func (env *environ) startInstance(
	ctx context.ProviderCallContext, args environs.StartInstanceParams, imageID, instanceTypeName string,
) (_ *computepb.Instance, err error) {
	hostname, err := env.namespace.Hostname(args.InstanceConfig.MachineId)
	if err != nil {
		return nil, environs.ZoneIndependentError(err)
	}

	os := ostype.OSTypeForName(args.InstanceConfig.Base.OS)
	metadata, err := getMetadata(args, os)
	if err != nil {
		return nil, environs.ZoneIndependentError(err)
	}
	tags := []string{
		env.globalFirewallName(),
		hostname,
	}

	imageURLBase, err := env.imageURLBase(os)
	if err != nil {
		return nil, environs.ZoneIndependentError(err)
	}
	imageURL := imageURLBase + imageID

	disks, err := getDisks(imageURL, args.Constraints, os)
	if err != nil {
		return nil, environs.ZoneIndependentError(err)
	}

	allocatePublicIP := true
	if args.Constraints.HasAllocatePublicIP() {
		allocatePublicIP = *args.Constraints.AllocatePublicIP
	}

	var nics []*computepb.NetworkInterface
	vpcLink, subnets, err := env.subnetsForInstance(ctx, args)
	if err != nil {
		return nil, environs.ZoneIndependentError(err)
	}
	if len(subnets) == 0 {
		nics = []*computepb.NetworkInterface{{
			Network: vpcLink,
		}}
	} else {
		for _, subnet := range subnets {
			if err := isSubnetReady(subnet); err != nil {
				return nil, environs.ZoneIndependentError(err)
			}
			nics = append(nics, &computepb.NetworkInterface{
				Network:    vpcLink,
				Subnetwork: ptr(subnet.GetSelfLink()),
			})
		}
	}

	if allocatePublicIP {
		nics[0].AccessConfigs = []*computepb.AccessConfig{{
			Name: ptr(google.ExternalNetworkName),
			Type: ptr(google.NetworkAccessOneToOneNAT),
		}}
	}

	serviceAccount, err := env.serviceAccount(ctx, args)
	if err != nil {
		return nil, google.HandleCredentialError(errors.Trace(err), ctx)
	}

	hasAccelerator, err := env.hasAccelerator(ctx, args.AvailabilityZone, instanceTypeName)
	if err != nil {
		return nil, google.HandleCredentialError(errors.Trace(err), ctx)
	}
	logger.Debugf("Accelerator support in zone %s for instance type %s: %t",
		args.AvailabilityZone, args.Constraints.InstanceType, hasAccelerator)

	instArg := &computepb.Instance{
		Name:              &hostname,
		Zone:              &args.AvailabilityZone,
		MachineType:       ptr(formatMachineType(args.AvailabilityZone, instanceTypeName)),
		Disks:             disks,
		NetworkInterfaces: nics,
		Metadata:          packMetadata(metadata),
		Tags:              &computepb.Tags{Items: tags},
	}
	if serviceAccount != "" {
		instArg.ServiceAccounts = []*computepb.ServiceAccount{{
			Email:  &serviceAccount,
			Scopes: google.Scopes,
		}}
	}

	// For the instance types which don't support live migration E.g. gpu instance
	// https://cloud.google.com/compute/docs/instances/live-migration-process#limitations
	if hasAccelerator {
		instArg.Scheduling = &computepb.Scheduling{
			OnHostMaintenance: ptr(google.HostMaintenanceTerminate),
		}
	}
	inst, err := env.gce.AddInstance(ctx, instArg)
	if err != nil {
		// We currently treat all AddInstance failures
		// as being zone-specific, so we'll retry in
		// another zone.
		return nil, google.HandleCredentialError(errors.Trace(err), ctx)
	}

	return inst, nil
}

// getMetadata builds the raw "user-defined" metadata for the new
// instance (relative to the provided args) and returns it.
func getMetadata(args environs.StartInstanceParams, os ostype.OSType) (map[string]string, error) {
	userData, err := providerinit.ComposeUserData(args.InstanceConfig, nil, GCERenderer{})
	if err != nil {
		return nil, errors.Annotate(err, "cannot make user data")
	}
	logger.Debugf("GCE user data; %d bytes", len(userData))

	metadata := make(map[string]string)
	for tag, value := range args.InstanceConfig.Tags {
		metadata[tag] = value
	}
	switch os {
	case ostype.Ubuntu:
		// We store a gz snapshop of information that is used by
		// cloud-init and unpacked in to the /var/lib/cloud/instances folder
		// for the instance. Due to a limitation with GCE and binary blobs
		// we base64 encode the data before storing it.
		metadata[metadataKeyCloudInit] = string(userData)
		// Valid encoding values are determined by the cloudinit GCE data source.
		// See: http://cloudinit.readthedocs.org
		metadata[metadataKeyEncoding] = "base64"

	default:
		return nil, errors.Errorf("cannot pack metadata for os %s on the gce provider", os.String())
	}

	return metadata, nil
}

// getDisks builds the raw spec for the disks that should be attached to
// the new instances and returns it. This will always include a root
// disk with characteristics determined by the provides args and
// constraints.
func getDisks(imageURL string, cons constraints.Value, os ostype.OSType) ([]*computepb.AttachedDisk, error) {
	size := common.MinRootDiskSizeGiB(os)
	if cons.RootDisk != nil && *cons.RootDisk > size {
		size = common.MiBToGiB(*cons.RootDisk)
		if size < google.MinDiskSizeGB {
			msg := "Ignoring root-disk constraint of %dM because it is smaller than the GCE image size of %dG"
			logger.Infof(msg, *cons.RootDisk, google.MinDiskSizeGB)
		}
	}
	if size < google.MinDiskSizeGB {
		size = google.MinDiskSizeGB
	}
	logger.Infof("fetching disk image from %v", imageURL)

	disk := &computepb.AttachedDisk{
		Type:       ptr(google.DiskPersistenceTypePersistent),
		Boot:       ptr(true),
		Mode:       ptr(string(google.ModeRW)),
		AutoDelete: ptr(true),
		InitializeParams: &computepb.AttachedDiskInitializeParams{
			// DiskName (defaults to instance name)
			DiskSizeGb: ptr(int64(size)),
			// DiskType (defaults to pd-standard, pd-ssd, local-ssd)
			SourceImage: &imageURL,
		},
		// Interface (defaults to SCSI)
		// DeviceName (GCE sets this, persistent disk only)
	}
	return []*computepb.AttachedDisk{disk}, nil
}

// hasAccelerator checks if the given instance type has any accelerators (e.g., GPUs).
func (env *environ) hasAccelerator(ctx context.ProviderCallContext, zone string, instanceType string) (bool, error) {
	if len(instanceType) == 0 {
		return false, nil
	}

	mt, err := env.gce.MachineType(ctx, zone, instanceType)
	if err != nil {
		return false, google.HandleCredentialError(errors.Trace(err), ctx)
	}

	for _, accelerator := range mt.GetAccelerators() {
		if accelerator != nil && accelerator.GuestAcceleratorCount != nil && *accelerator.GuestAcceleratorCount > 0 {
			return true, nil
		}
	}
	return false, nil
}

// getHardwareCharacteristics compiles hardware-related details about
// the given instance and relative to the provided spec and returns it.
func (env *environ) getHardwareCharacteristics(spec *instances.InstanceSpec, inst *environInstance) *instance.HardwareCharacteristics {
	rootDiskMB := uint64(0)
	if len(inst.base.Disks) > 0 {
		rootDiskMB = uint64(inst.base.Disks[0].GetDiskSizeGb() * 1024)
	}
	zone := path.Base(inst.base.GetZone())
	hwc := instance.HardwareCharacteristics{
		Arch:             &spec.Image.Arch,
		Mem:              &spec.InstanceType.Mem,
		CpuCores:         &spec.InstanceType.CpuCores,
		CpuPower:         spec.InstanceType.CpuPower,
		RootDisk:         &rootDiskMB,
		AvailabilityZone: &zone,
		// Tags: not supported in GCE.
	}
	return &hwc
}

// AllInstances implements environs.InstanceBroker.
func (env *environ) AllInstances(ctx context.ProviderCallContext) ([]instances.Instance, error) {
	// We want all statuses here except for "terminated" - these instances are truly dead to us.
	// According to https://cloud.google.com/compute/docs/instances/instance-life-cycle
	// there are now only "provisioning", "staging", "running", "stopping" and "terminated" states.
	// The others might have been needed for older versions of gce... Keeping here for potential
	// backward compatibility.
	nonLiveStatuses := []string{
		google.StatusDone,
		google.StatusDown,
		google.StatusProvisioning,
		google.StatusStopped,
		google.StatusStopping,
		google.StatusUp,
	}
	filters := append(instStatuses, nonLiveStatuses...)
	instances, err := env.instances(ctx, filters...)
	return instances, errors.Trace(err)
}

// AllRunningInstances implements environs.InstanceBroker.
func (env *environ) AllRunningInstances(ctx context.ProviderCallContext) ([]instances.Instance, error) {
	instances, err := env.instances(ctx)
	return instances, errors.Trace(err)
}

// StopInstances implements environs.InstanceBroker.
func (env *environ) StopInstances(ctx context.ProviderCallContext, instances ...instance.Id) error {
	var ids []string
	for _, id := range instances {
		ids = append(ids, string(id))
	}

	prefix := env.namespace.Prefix()
	err := env.gce.RemoveInstances(ctx, prefix, ids...)
	return google.HandleCredentialError(errors.Trace(err), ctx)
}
