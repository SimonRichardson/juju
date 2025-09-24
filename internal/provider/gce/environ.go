// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package gce

import (
	stdcontext "context"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/juju/errors"
	jujuhttp "github.com/juju/http/v2"

	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/juju/core/instance"
	corelogger "github.com/juju/juju/core/logger"
	"github.com/juju/juju/environs"
	environscloudspec "github.com/juju/juju/environs/cloudspec"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/environs/context"
	"github.com/juju/juju/environs/instances"
	"github.com/juju/juju/environs/simplestreams"
	"github.com/juju/juju/internal/provider/common"
	"github.com/juju/juju/internal/provider/gce/internal/google"
)

// ComputeService defines a client used to interact with the Google Cloud API.
type ComputeService interface {
	// VerifyCredentials returns an error if the credential used is invalid.
	VerifyCredentials(ctx stdcontext.Context) error
	// DefaultServiceAccount returns the service account for the project.
	DefaultServiceAccount(ctx stdcontext.Context) (string, error)

	// Instance gets the up-to-date info about the given instance.
	Instance(ctx stdcontext.Context, id, zone string) (*computepb.Instance, error)
	// Instances returns the instances with the given name prefix and statuses.
	Instances(ctx stdcontext.Context, prefix string, statuses ...string) ([]*computepb.Instance, error)
	// AddInstance creates a new instance.
	AddInstance(ctx stdcontext.Context, inst *computepb.Instance) (*computepb.Instance, error)
	// RemoveInstances removes instances with the given ids.
	RemoveInstances(ctx stdcontext.Context, prefix string, ids ...string) error
	// UpdateMetadata updates the metadata for the given instance ids.
	UpdateMetadata(ctx stdcontext.Context, key, value string, ids ...string) error
	// ListMachineTypes returns a list of machines available in the project and zone provided.
	ListMachineTypes(ctx stdcontext.Context, zone string) ([]*computepb.MachineType, error)
	// MachineType retrieves the machine type definition for the specified instance type.
	MachineType(ctx stdcontext.Context, zone, instanceType string) (*computepb.MachineType, error)

	// Firewalls returns the firewalls with the given prefix.
	Firewalls(ctx stdcontext.Context, prefix string) ([]*computepb.Firewall, error)
	// NetworkFirewalls returns the firewalls associated with the specified network.
	NetworkFirewalls(ctx stdcontext.Context, networkURL string) ([]*computepb.Firewall, error)
	// AddFirewall creates a new firewall.
	AddFirewall(ctx stdcontext.Context, firewall *computepb.Firewall) error
	// UpdateFirewall updates the firewall with the given name.
	UpdateFirewall(ctx stdcontext.Context, name string, firewall *computepb.Firewall) error
	// RemoveFirewall removes the firewall with the given name.
	RemoveFirewall(ctx stdcontext.Context, fwname string) error

	// AvailabilityZones returns the availability zones for the region.
	AvailabilityZones(ctx stdcontext.Context, region string) ([]*computepb.Zone, error)
	// Subnetworks returns the subnetworks that machines can be
	// assigned to in the given region.
	Subnetworks(ctx stdcontext.Context, region string, urls ...string) ([]*computepb.Subnetwork, error)
	// NetworkSubnetworks returns the subnets in the specified network.
	NetworkSubnetworks(ctx stdcontext.Context, region string, networkURL string) ([]*computepb.Subnetwork, error)
	// Networks returns the available networks that exist across
	// regions.
	Networks(ctx stdcontext.Context) ([]*computepb.Network, error)
	// Network returns the network with the given id.
	Network(ctx stdcontext.Context, id string) (*computepb.Network, error)

	// CreateDisks will attempt to create the disks described by <disks> spec and
	// return a slice of Disk representing the created disks or error if one of them failed.
	CreateDisks(ctx stdcontext.Context, zone string, disks []*computepb.Disk) error
	// Disks will return a list of all Disks found in the project.
	Disks(ctx stdcontext.Context) ([]*computepb.Disk, error)
	// Disk will return a Disk representing the disk identified by the
	// passed <name> or error.
	Disk(ctx stdcontext.Context, zone, id string) (*computepb.Disk, error)
	// RemoveDisk will destroy the disk identified by <name> in <zone>.
	RemoveDisk(ctx stdcontext.Context, zone, id string) error
	// SetDiskLabels sets the labels on a disk, ensuring that the disk's
	// label fingerprint matches the one supplied.
	SetDiskLabels(ctx stdcontext.Context, zone, id, labelFingerprint string, labels map[string]string) error
	// AttachDisk will attach the volume identified by <volumeName> into the instance
	// <instanceId> and return an AttachedDisk representing it or error.
	AttachDisk(ctx stdcontext.Context, zone, volumeName, instanceId string, mode google.DiskMode) (*computepb.AttachedDisk, error)
	// DetachDisk will detach <volumeName> disk from <instanceId> if possible
	// and return error.
	DetachDisk(ctx stdcontext.Context, zone, instanceId, volumeName string) error
	// InstanceDisks returns a list of the disks attached to the passed instance.
	InstanceDisks(ctx stdcontext.Context, zone, instanceId string) ([]*computepb.AttachedDisk, error)
}

func ptr[T any](v T) *T {
	return &v
}

type environ struct {
	environs.NoSpaceDiscoveryEnviron
	environs.NoContainerAddressesEnviron

	name  string
	uuid  string
	cloud environscloudspec.CloudSpec
	gce   ComputeService

	// vpcURL is the URL of the vpc network, if any, to use.
	vpcURL *string
	// autoSubnets is true if the vpc creates subnets automatically.
	autoSubnets bool

	lock sync.Mutex // lock protects access to ecfg
	ecfg *environConfig

	instTypeListLock    sync.Mutex
	instCacheExpireAt   time.Time
	cachedInstanceTypes []instances.InstanceType

	// namespace is used to create the machine and device hostnames.
	namespace instance.Namespace
}

var _ environs.Environ = (*environ)(nil)
var _ environs.NetworkingEnviron = (*environ)(nil)

// Function entry points defined as variables so they can be overridden
// for testing purposes.
var (
	newConnection = func(ctx stdcontext.Context, conn google.ConnectionConfig, creds *google.Credentials) (ComputeService, error) {
		return google.Connect(ctx, conn, creds)
	}
	destroyEnv = common.Destroy
	bootstrap  = common.Bootstrap
)

func newEnviron(cloud environscloudspec.CloudSpec, cfg *config.Config) (*environ, error) {
	ecfg, err := newConfig(cfg, nil)
	if err != nil {
		return nil, errors.Annotate(err, "invalid config")
	}

	namespace, err := instance.NewNamespace(cfg.UUID())
	if err != nil {
		return nil, errors.Trace(err)
	}

	e := &environ{
		name:      ecfg.config.Name(),
		uuid:      ecfg.config.UUID(),
		ecfg:      ecfg,
		namespace: namespace,
	}
	if err = e.SetCloudSpec(stdcontext.TODO(), cloud); err != nil {
		return nil, err
	}
	if err := e.SetConfig(cfg); err != nil {
		return nil, errors.Trace(err)
	}
	return e, nil
}

// SetCloudSpec is specified in the environs.Environ interface.
func (env *environ) SetCloudSpec(ctx stdcontext.Context, spec environscloudspec.CloudSpec) error {
	env.lock.Lock()
	defer env.lock.Unlock()

	env.cloud = spec
	credAttrs := spec.Credential.Attributes()
	switch spec.Credential.AuthType() {
	case jujucloud.JSONFileAuthType:
		contents := credAttrs[credAttrFile]
		credential, err := parseJSONAuthFile(strings.NewReader(contents))
		if err != nil {
			return errors.Trace(err)
		}
		credAttrs = credential.Attributes()
	case jujucloud.ServiceAccountAuthType:
		if serviceAccount := credAttrs[credServiceAccount]; serviceAccount == "" {
			return errors.NotValidf("credential with missing service account")
		}
	}

	credential := &google.Credentials{
		ClientID:       credAttrs[credAttrClientID],
		ProjectID:      credAttrs[credAttrProjectID],
		ClientEmail:    credAttrs[credAttrClientEmail],
		PrivateKey:     []byte(credAttrs[credAttrPrivateKey]),
		ServiceAccount: credAttrs[credServiceAccount],
	}

	connectionConfig := google.ConnectionConfig{
		Region: spec.Region,

		// TODO (Stickupkid): Pass the http.Client through on the construction
		// of the environ.
		HTTPClient: jujuhttp.NewClient(
			jujuhttp.WithSkipHostnameVerification(spec.SkipTLSVerify),
			jujuhttp.WithLogger(logger.ChildWithLabels("http", corelogger.HTTP)),
		),
	}

	// Connect and authenticate.
	var err error
	env.gce, err = newConnection(ctx, connectionConfig, credential)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// Name returns the name of the environment.
func (env *environ) Name() string {
	return env.name
}

// Provider returns the environment provider that created this env.
func (*environ) Provider() environs.EnvironProvider {
	return providerInstance
}

// Region returns the CloudSpec to use for the provider, as configured.
func (env *environ) Region() (simplestreams.CloudSpec, error) {
	return simplestreams.CloudSpec{
		Region:   env.cloud.Region,
		Endpoint: env.cloud.Endpoint,
	}, nil
}

// SetConfig updates the env's configuration.
func (env *environ) SetConfig(cfg *config.Config) error {
	env.lock.Lock()
	defer env.lock.Unlock()

	ecfg, err := newConfig(cfg, env.ecfg.config)
	if err != nil {
		return errors.Annotate(err, "invalid config change")
	}
	env.ecfg = ecfg
	return nil
}

// getVpcInfo returns the VPC URL (if set) and whether it auto creates subnets.
func (env *environ) getVpcInfo(ctx stdcontext.Context) (*string, bool, error) {
	env.lock.Lock()
	defer env.lock.Unlock()

	// vpc is immutable. See if it has already been fetched.
	if env.vpcURL != nil {
		return env.vpcURL, env.autoSubnets, nil
	}

	vpcID, ok := env.ecfg.vpcID()
	if !ok {
		return nil, false, nil
	}
	vpc, err := env.gce.Network(ctx, vpcID)
	if err != nil {
		return nil, false, errors.Annotatef(err, "getting vpc %q", vpcID)
	}
	env.vpcURL = ptr(vpc.GetSelfLink())
	env.autoSubnets = autoCreateSubnets(vpc)
	return env.vpcURL, env.autoSubnets, nil
}

// Config returns the configuration data with which the env was created.
func (env *environ) Config() *config.Config {
	env.lock.Lock()
	defer env.lock.Unlock()
	return env.ecfg.config
}

func (env *environ) vpcID() (string, bool) {
	env.lock.Lock()
	defer env.lock.Unlock()
	return env.ecfg.vpcID()
}

// PrepareForBootstrap implements environs.Environ.
func (env *environ) PrepareForBootstrap(ctx environs.BootstrapContext, controllerName string) error {
	if ctx.ShouldVerifyCredentials() {
		if err := env.gce.VerifyCredentials(ctx.Context()); err != nil {
			return errors.Trace(err)
		}
	}
	vpcID, ok := env.ecfg.vpcID()
	if !ok {
		return nil
	}
	callCtx := context.NewCloudCallContext(ctx.Context())
	if err := validateBootstrapVPC(ctx, env.gce, env.cloud.Region, vpcID, env.ecfg.forceVPCID()); err != nil {
		return google.HandleCredentialError(errors.Trace(err), callCtx)
	}

	return nil
}

// Create implements environs.Environ.
func (env *environ) Create(ctx context.ProviderCallContext, p environs.CreateParams) error {
	if err := env.gce.VerifyCredentials(ctx); err != nil {
		return google.HandleCredentialError(errors.Trace(err), ctx)
	}
	vpcID, ok := env.vpcID()
	if !ok {
		return nil
	}
	if err := validateModelVPC(ctx, env.gce, env.cloud.Region, env.name, vpcID); err != nil {
		return google.HandleCredentialError(errors.Trace(err), ctx)
	}

	return nil
}

// Bootstrap creates a new instance, choosing the series and arch out of
// available tools. The series and arch are returned along with a func
// that must be called to finalize the bootstrap process by transferring
// the tools and installing the initial juju controller.
func (env *environ) Bootstrap(ctx environs.BootstrapContext, callCtx context.ProviderCallContext, params environs.BootstrapParams) (*environs.BootstrapResult, error) {
	return bootstrap(ctx, env, callCtx, params)
}

// Destroy shuts down all known machines and destroys the rest of the
// known environment.
func (env *environ) Destroy(ctx context.ProviderCallContext) error {
	err := destroyEnv(env, ctx)
	if err != nil {
		return errors.Trace(err)
	}
	err = env.cleanupFirewall(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// DestroyController implements the Environ interface.
func (env *environ) DestroyController(ctx context.ProviderCallContext, controllerUUID string) error {
	// TODO(wallyworld): destroy hosted model resources
	return env.Destroy(ctx)
}
