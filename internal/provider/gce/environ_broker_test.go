// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package gce_test

import (
	"errors"
	"fmt"
	"reflect"

	"cloud.google.com/go/compute/apiv1/computepb"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/version/v2"
	"go.uber.org/mock/gomock"
	gc "gopkg.in/check.v1"

	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/juju/cloudconfig/instancecfg"
	"github.com/juju/juju/cloudconfig/providerinit"
	"github.com/juju/juju/core/arch"
	corebase "github.com/juju/juju/core/base"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/os/ostype"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/imagemetadata"
	"github.com/juju/juju/environs/instances"
	"github.com/juju/juju/environs/tags"
	"github.com/juju/juju/internal/provider/gce"
	"github.com/juju/juju/internal/provider/gce/internal/google"
	"github.com/juju/juju/storage"
)

type environBrokerSuite struct {
	gce.BaseSuite

	spec *instances.InstanceSpec
}

var _ = gc.Suite(&environBrokerSuite{})

func (s *environBrokerSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)

	instanceType := instances.InstanceType{
		Name:     "n1-standard-1",
		Arch:     arch.AMD64,
		CpuCores: 2,
		Mem:      3750,
		RootDisk: 15 * 1024,
		VirtType: ptr("kvm"),
	}
	s.spec = &instances.InstanceSpec{
		InstanceType: instanceType,
		Image: instances.Image{
			Id:       "ubuntu-2204-jammy-v20141212",
			Arch:     arch.AMD64,
			VirtType: "kvm",
		},
	}
}

func (s *environBrokerSuite) expectImageMetadata() {
	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return([]*computepb.Zone{{
		Name:   ptr("home-zone"),
		Status: ptr("UP"),
	}}, nil)
	s.MockService.EXPECT().ListMachineTypes(gomock.Any(), "home-zone").Return([]*computepb.MachineType{{
		Id:           ptr(uint64(0)),
		Name:         ptr("n1-standard-1"),
		GuestCpus:    ptr(int32(s.spec.InstanceType.CpuCores)),
		MemoryMb:     ptr(int32(s.spec.InstanceType.Mem)),
		Architecture: ptr(s.spec.InstanceType.Arch),
	}}, nil)
	s.StartInstArgs.ImageMetadata = []*imagemetadata.ImageMetadata{{
		Id:   "ubuntu-220-jammy-v20141212",
		Arch: s.spec.InstanceType.Arch,
	}}
}

func (s *environBrokerSuite) startInstanceArg(c *gc.C, prefix string, hasGpuSupported bool) *computepb.Instance {
	instName := fmt.Sprintf("%s0", prefix)
	userData, err := providerinit.ComposeUserData(s.StartInstArgs.InstanceConfig, nil, gce.GCERenderer{})
	c.Assert(err, jc.ErrorIsNil)

	var scheduling *computepb.Scheduling
	if hasGpuSupported {
		scheduling = &computepb.Scheduling{
			OnHostMaintenance: ptr(google.HostMaintenanceTerminate),
		}
	}

	return &computepb.Instance{
		Name:        &instName,
		Zone:        ptr("home-zone"),
		MachineType: ptr("zones/home-zone/machineTypes/n1-standard-1"),
		Disks: []*computepb.AttachedDisk{{
			AutoDelete: ptr(true),
			Boot:       ptr(true),
			Mode:       ptr("READ_WRITE"),
			Type:       ptr("PERSISTENT"),
			InitializeParams: &computepb.AttachedDiskInitializeParams{
				DiskSizeGb:  ptr(int64(10)),
				SourceImage: ptr("projects/ubuntu-os-cloud/global/images/ubuntu-220-jammy-v20141212"),
			},
		}},
		NetworkInterfaces: []*computepb.NetworkInterface{{
			Network: ptr("/path/to/vpc"),
			AccessConfigs: []*computepb.AccessConfig{{
				Name: ptr("External NAT"),
				Type: ptr("ONE_TO_ONE_NAT"),
			}},
		}},
		Metadata: &computepb.Metadata{
			Items: []*computepb.Items{{
				Key:   ptr("juju-controller-uuid"),
				Value: ptr(s.ControllerUUID),
			}, {
				Key:   ptr("juju-is-controller"),
				Value: ptr("true"),
			}, {
				Key:   ptr("user-data"),
				Value: ptr(string(userData)),
			}, {
				Key:   ptr("user-data-encoding"),
				Value: ptr("base64"),
			}},
		},
		Tags: &computepb.Tags{Items: []string{"juju-" + s.ModelUUID, instName}},
		ServiceAccounts: []*computepb.ServiceAccount{{
			Email: ptr("fred@google.com"),
			Scopes: []string{
				"https://www.googleapis.com/auth/compute",
				"https://www.googleapis.com/auth/devstorage.full_control",
			},
		}},
		Scheduling: scheduling,
	}
}

// gceComputeArgMatcher is a gomock matcher for args passed to the
// gce rest api. The String() representation used to match strips
// out any underlying serialisation state which can affect the outcome.
type gceComputeArgMatcher struct {
	want fmt.Stringer
}

func (m gceComputeArgMatcher) Matches(x interface{}) bool {
	if reflect.TypeOf(x).Name() != reflect.TypeOf(m.want).Name() {
		return false
	}
	xStr, ok := x.(fmt.Stringer)
	if !ok {
		return false
	}
	return xStr.String() == m.want.String()
}

func (m gceComputeArgMatcher) String() string {
	return fmt.Sprintf("is equal to %v", m.want.String())
}

func (s *environBrokerSuite) TestStartInstance(c *gc.C) {
	s.testStartInstance(c, false)
}

func (s *environBrokerSuite) TestStartGpuInstance(c *gc.C) {
	s.testStartInstance(c, true)
}

func (s *environBrokerSuite) testStartInstance(c *gc.C, hasGpuSupported bool) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	err := gce.FinishInstanceConfig(env, s.StartInstArgs, s.spec)
	c.Assert(err, jc.ErrorIsNil)

	s.expectImageMetadata()
	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			SelfLink: ptr("/path/to/subnet1"),
		}, {
			SelfLink: ptr("/path/to/subnet2"),
		}}, nil)
	s.MockService.EXPECT().DefaultServiceAccount(gomock.Any()).Return("fred@google.com", nil)

	accelerators := []*computepb.Accelerators{}
	if hasGpuSupported {
		accelerators = []*computepb.Accelerators{
			{
				GuestAcceleratorType:  ptr("nvidia-l4"),
				GuestAcceleratorCount: ptr(int32(1)),
			},
		}
	}

	s.MockService.EXPECT().
		MachineType(gomock.Any(), "home-zone", s.spec.InstanceType.Name).
		Return(&computepb.MachineType{
			Name:         ptr(s.spec.InstanceType.Name),
			Accelerators: accelerators,
		}, nil)

	instArg := s.startInstanceArg(c, s.Prefix(env), hasGpuSupported)
	// Can't copy instArg as it contains a mutex.
	instResult := s.startInstanceArg(c, s.Prefix(env), hasGpuSupported)
	instResult.Zone = ptr("path/to/home-zone")
	instResult.Disks = []*computepb.AttachedDisk{{
		DiskSizeGb: ptr(int64(s.spec.InstanceType.RootDisk / 1024)),
	}}

	s.MockService.EXPECT().AddInstance(gomock.Any(), gceComputeArgMatcher{instArg}).Return(instResult, nil)

	s.StartInstArgs.AvailabilityZone = "home-zone"
	result, err := env.StartInstance(s.CallCtx, s.StartInstArgs)

	c.Assert(err, jc.ErrorIsNil)
	c.Check(s.GoogleInstance(c, result.Instance), jc.DeepEquals, instResult)

	hwc := &instance.HardwareCharacteristics{
		Arch:             &s.spec.InstanceType.Arch,
		Mem:              &s.spec.InstanceType.Mem,
		CpuCores:         &s.spec.InstanceType.CpuCores,
		CpuPower:         s.spec.InstanceType.CpuPower,
		RootDisk:         ptr(s.spec.InstanceType.RootDisk),
		AvailabilityZone: ptr("home-zone"),
	}
	c.Check(result.Hardware, jc.DeepEquals, hwc)
}

func (s *environBrokerSuite) TestStartInstanceAvailabilityZoneIndependentError(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(nil, errors.New("blargh"))

	_, err := env.StartInstance(s.CallCtx, s.StartInstArgs)
	c.Assert(err, gc.ErrorMatches, "blargh")
	c.Assert(errors.Is(err, environs.ErrAvailabilityZoneIndependent), jc.IsTrue)
}

func (s *environBrokerSuite) TestStartInstanceVolumeAvailabilityZone(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	err := gce.FinishInstanceConfig(env, s.StartInstArgs, s.spec)
	c.Assert(err, jc.ErrorIsNil)

	s.expectImageMetadata()
	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			SelfLink: ptr("/path/to/subnet1"),
		}, {
			SelfLink: ptr("/path/to/subnet2"),
		}}, nil)
	s.MockService.EXPECT().DefaultServiceAccount(gomock.Any()).Return("fred@google.com", nil)

	s.MockService.EXPECT().
		MachineType(gomock.Any(), "home-zone", s.spec.InstanceType.Name).
		Return(&computepb.MachineType{
			Name:         ptr(s.spec.InstanceType.Name),
			Accelerators: nil,
		}, nil)

	instArg := s.startInstanceArg(c, s.Prefix(env), false)
	// Can't copy instArg as it contains a mutex.
	instResult := s.startInstanceArg(c, s.Prefix(env), false)
	instResult.Zone = ptr("path/to/home-zone")
	instResult.Disks = []*computepb.AttachedDisk{{
		DiskSizeGb: ptr(int64(s.spec.InstanceType.RootDisk / 1024)),
	}}

	s.MockService.EXPECT().AddInstance(gomock.Any(), gceComputeArgMatcher{instArg}).Return(instResult, nil)

	s.StartInstArgs.VolumeAttachments = []storage.VolumeAttachmentParams{{
		VolumeId: "home-zone--c930380d-8337-4bf5-b07a-9dbb5ae771e4",
	}}
	s.StartInstArgs.AvailabilityZone = "home-zone"

	result, err := env.StartInstance(s.CallCtx, s.StartInstArgs)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(*result.Hardware.AvailabilityZone, gc.Equals, "home-zone")
}

func (s *environBrokerSuite) TestStartInstanceAutoSubnet(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	err := gce.FinishInstanceConfig(env, s.StartInstArgs, s.spec)
	c.Assert(err, jc.ErrorIsNil)

	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.expectImageMetadata()
	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").Return(nil, nil)
	s.MockService.EXPECT().DefaultServiceAccount(gomock.Any()).Return("fred@google.com", nil)

	s.MockService.EXPECT().
		MachineType(gomock.Any(), "home-zone", s.spec.InstanceType.Name).
		Return(&computepb.MachineType{
			Name:         ptr(s.spec.InstanceType.Name),
			Accelerators: nil,
		}, nil)

	instArg := s.startInstanceArg(c, s.Prefix(env), false)
	// Can't copy instArg as it contains a mutex.
	instResult := s.startInstanceArg(c, s.Prefix(env), false)
	instResult.Zone = ptr("path/to/home-zone")
	instResult.Disks = []*computepb.AttachedDisk{{
		DiskSizeGb: ptr(int64(s.spec.InstanceType.RootDisk / 1024)),
	}}

	s.MockService.EXPECT().AddInstance(gomock.Any(), gceComputeArgMatcher{instArg}).Return(instResult, nil)

	s.StartInstArgs.AvailabilityZone = "home-zone"

	result, err := env.StartInstance(s.CallCtx, s.StartInstArgs)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(*result.Hardware.AvailabilityZone, gc.Equals, "home-zone")
}

func (s *environBrokerSuite) TestStartInstanceSubnetPlacement(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	err := gce.FinishInstanceConfig(env, s.StartInstArgs, s.spec)
	c.Assert(err, jc.ErrorIsNil)

	s.expectImageMetadata()
	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			Name:     ptr("subnet1"),
			SelfLink: ptr("/path/to/subnet1"),
		}, {
			Name:     ptr("subnet2"),
			SelfLink: ptr("/path/to/subnet2"),
		}}, nil)
	s.MockService.EXPECT().DefaultServiceAccount(gomock.Any()).Return("fred@google.com", nil)

	s.MockService.EXPECT().
		MachineType(gomock.Any(), "home-zone", s.spec.InstanceType.Name).
		Return(&computepb.MachineType{
			Name:         ptr(s.spec.InstanceType.Name),
			Accelerators: nil,
		}, nil)

	instArg := s.startInstanceArg(c, s.Prefix(env), false)
	instArg.NetworkInterfaces[0].Subnetwork = ptr("/path/to/subnet1")
	// Can't copy instArg as it contains a mutex.
	instResult := s.startInstanceArg(c, s.Prefix(env), false)
	instResult.Zone = ptr("path/to/home-zone")
	instResult.Disks = []*computepb.AttachedDisk{{
		DiskSizeGb: ptr(int64(s.spec.InstanceType.RootDisk / 1024)),
	}}

	s.MockService.EXPECT().AddInstance(gomock.Any(), gceComputeArgMatcher{instArg}).Return(instResult, nil)

	s.StartInstArgs.AvailabilityZone = "home-zone"
	s.StartInstArgs.Placement = "subnet=subnet1"

	result, err := env.StartInstance(s.CallCtx, s.StartInstArgs)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(*result.Hardware.AvailabilityZone, gc.Equals, "home-zone")
}

func (s *environBrokerSuite) TestStartInstanceSubnetSpaces(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	err := gce.FinishInstanceConfig(env, s.StartInstArgs, s.spec)
	c.Assert(err, jc.ErrorIsNil)

	s.expectImageMetadata()
	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			Name:     ptr("subnet1"),
			SelfLink: ptr("/path/to/subnet1"),
		}, {
			Name:     ptr("subnet2"),
			SelfLink: ptr("/path/to/subnet2"),
		}}, nil)
	s.MockService.EXPECT().DefaultServiceAccount(gomock.Any()).Return("fred@google.com", nil)

	s.MockService.EXPECT().
		MachineType(gomock.Any(), "home-zone", s.spec.InstanceType.Name).
		Return(&computepb.MachineType{
			Name:         ptr(s.spec.InstanceType.Name),
			Accelerators: nil,
		}, nil)

	instArg := s.startInstanceArg(c, s.Prefix(env), false)
	instArg.NetworkInterfaces[0].Subnetwork = ptr("/path/to/subnet1")
	// Can't copy instArg as it contains a mutex.
	instResult := s.startInstanceArg(c, s.Prefix(env), false)
	instResult.Zone = ptr("path/to/home-zone")
	instResult.Disks = []*computepb.AttachedDisk{{
		DiskSizeGb: ptr(int64(s.spec.InstanceType.RootDisk / 1024)),
	}}

	s.MockService.EXPECT().AddInstance(gomock.Any(), gceComputeArgMatcher{instArg}).Return(instResult, nil)

	s.StartInstArgs.AvailabilityZone = "home-zone"
	s.StartInstArgs.Constraints.Spaces = ptr([]string{"s1", "s2"})
	s.StartInstArgs.SubnetsToZones = []map[network.Id][]string{
		{"subnet1": []string{"home-zone", "away-zone"}},
		{"subnet2": []string{"home-zone", "away-zone"}},
	}

	result, err := env.StartInstance(s.CallCtx, s.StartInstArgs)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(*result.Hardware.AvailabilityZone, gc.Equals, "home-zone")
}

func (s *environBrokerSuite) TestStartInstanceServiceAccount(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	err := gce.FinishInstanceConfig(env, s.StartInstArgs, s.spec)
	c.Assert(err, jc.ErrorIsNil)
	s.SetCredential(env, jujucloud.NewCredential(
		jujucloud.ServiceAccountAuthType, map[string]string{
			"service-account": "foo@googledev.com",
		}))

	s.expectImageMetadata()
	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			SelfLink: ptr("/path/to/subnet1"),
		}, {
			SelfLink: ptr("/path/to/subnet2"),
		}}, nil)

	s.MockService.EXPECT().
		MachineType(gomock.Any(), "home-zone", s.spec.InstanceType.Name).
		Return(&computepb.MachineType{
			Name:         ptr(s.spec.InstanceType.Name),
			Accelerators: nil,
		}, nil)

	instArg := s.startInstanceArg(c, s.Prefix(env), false)
	instArg.ServiceAccounts[0].Email = ptr("foo@googledev.com")
	// Can't copy instArg as it contains a mutex.
	instResult := s.startInstanceArg(c, s.Prefix(env), false)
	instResult.Zone = ptr("path/to/home-zone")
	instResult.Disks = []*computepb.AttachedDisk{{
		DiskSizeGb: ptr(int64(s.spec.InstanceType.RootDisk / 1024)),
	}}

	s.MockService.EXPECT().AddInstance(gomock.Any(), instArg).Return(instResult, nil)

	s.StartInstArgs.AvailabilityZone = "home-zone"
	result, err := env.StartInstance(s.CallCtx, s.StartInstArgs)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(*result.Hardware.AvailabilityZone, gc.Equals, "home-zone")
}

func (s *environBrokerSuite) TestStartInstanceInstanceRoleCredential(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	err := gce.FinishInstanceConfig(env, s.StartInstArgs, s.spec)
	c.Assert(err, jc.ErrorIsNil)

	s.expectImageMetadata()
	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			SelfLink: ptr("/path/to/subnet1"),
		}, {
			SelfLink: ptr("/path/to/subnet2"),
		}}, nil)

	s.MockService.EXPECT().
		MachineType(gomock.Any(), "home-zone", s.spec.InstanceType.Name).
		Return(&computepb.MachineType{
			Name:         ptr(s.spec.InstanceType.Name),
			Accelerators: nil,
		}, nil)

	instArg := s.startInstanceArg(c, s.Prefix(env), false)
	instArg.ServiceAccounts[0].Email = ptr("foo@googledev.com")
	// Can't copy instArg as it contains a mutex.
	instResult := s.startInstanceArg(c, s.Prefix(env), false)
	instResult.Zone = ptr("path/to/home-zone")
	instResult.Disks = []*computepb.AttachedDisk{{
		DiskSizeGb: ptr(int64(s.spec.InstanceType.RootDisk / 1024)),
	}}

	s.MockService.EXPECT().AddInstance(gomock.Any(), instArg).Return(instResult, nil)

	s.StartInstArgs.AvailabilityZone = "home-zone"
	s.StartInstArgs.Constraints = constraints.MustParse("instance-role=foo@googledev.com")
	result, err := env.StartInstance(s.CallCtx, s.StartInstArgs)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(*result.Hardware.AvailabilityZone, gc.Equals, "home-zone")
}

func (s *environBrokerSuite) TestStartInstanceBootstrapInstanceRoleCredential(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	err := gce.FinishInstanceConfig(env, s.StartInstArgs, s.spec)
	c.Assert(err, jc.ErrorIsNil)

	s.expectImageMetadata()
	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			SelfLink: ptr("/path/to/subnet1"),
		}, {
			SelfLink: ptr("/path/to/subnet2"),
		}}, nil)

	s.MockService.EXPECT().
		MachineType(gomock.Any(), "home-zone", s.spec.InstanceType.Name).
		Return(&computepb.MachineType{
			Name:         ptr(s.spec.InstanceType.Name),
			Accelerators: nil,
		}, nil)

	instArg := s.startInstanceArg(c, s.Prefix(env), false)
	instArg.ServiceAccounts[0].Email = ptr("foo@googledev.com")
	// Can't copy instArg as it contains a mutex.
	instResult := s.startInstanceArg(c, s.Prefix(env), false)
	instResult.Zone = ptr("path/to/home-zone")
	instResult.Disks = []*computepb.AttachedDisk{{
		DiskSizeGb: ptr(int64(s.spec.InstanceType.RootDisk / 1024)),
	}}

	s.MockService.EXPECT().AddInstance(gomock.Any(), instArg).Return(instResult, nil)

	s.StartInstArgs.AvailabilityZone = "home-zone"
	s.StartInstArgs.InstanceConfig.Bootstrap = &instancecfg.BootstrapConfig{
		StateInitializationParams: instancecfg.StateInitializationParams{
			BootstrapMachineConstraints: constraints.MustParse("instance-role=foo@googledev.com"),
		},
	}
	result, err := env.StartInstance(s.CallCtx, s.StartInstArgs)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(*result.Hardware.AvailabilityZone, gc.Equals, "home-zone")
}

func (s *environBrokerSuite) TestFinishInstanceConfig(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	err := gce.FinishInstanceConfig(env, s.StartInstArgs, s.spec)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(s.StartInstArgs.InstanceConfig.AgentVersion(), gc.Not(gc.Equals), version.Binary{})
}

func ptr[T any](v T) *T {
	return &v
}

func (s *environBrokerSuite) TestBuildInstanceSpec(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.expectImageMetadata()

	spec, err := gce.BuildInstanceSpec(env, s.CallCtx, s.StartInstArgs)

	c.Assert(err, jc.ErrorIsNil)
	c.Check(spec.InstanceType, jc.DeepEquals, instances.InstanceType{
		Id:         "0",
		Name:       "n1-standard-1",
		Arch:       "amd64",
		CpuCores:   2,
		Mem:        3750,
		Networking: instances.InstanceTypeNetworking{},
		VirtType:   ptr("kvm"),
	})
}

func (s *environBrokerSuite) TestFindInstanceSpec(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	ic := &instances.InstanceConstraint{
		Region:      "home",
		Base:        corebase.MakeDefaultBase("ubuntu", "22.04"),
		Arch:        arch.AMD64,
		Constraints: s.StartInstArgs.Constraints,
	}
	imageMetadata := []*imagemetadata.ImageMetadata{{
		Id:         "ubuntu-2204-jammy-v20141212",
		Arch:       "amd64",
		Version:    "22.04",
		RegionName: "us-central1",
		Endpoint:   "https://www.googleapis.com",
		Stream:     "<stream>",
		VirtType:   "kvm",
	}}
	spec, err := gce.FindInstanceSpec(env, ic, imageMetadata, []instances.InstanceType{s.spec.InstanceType})

	c.Assert(err, jc.ErrorIsNil)
	c.Check(spec, jc.DeepEquals, s.spec)
}

func (s *environBrokerSuite) TestGetMetadataUbuntu(c *gc.C) {
	metadata, err := gce.GetMetadata(s.StartInstArgs, ostype.Ubuntu)
	c.Assert(err, jc.ErrorIsNil)

	userData, err := providerinit.ComposeUserData(s.StartInstArgs.InstanceConfig, nil, gce.GCERenderer{})
	c.Assert(err, jc.ErrorIsNil)
	c.Check(metadata, jc.DeepEquals, map[string]string{
		tags.JujuIsController: "true",
		tags.JujuController:   s.ControllerUUID,
		"user-data":           string(userData),
		"user-data-encoding":  "base64",
	})
}

func (s *environBrokerSuite) TestGetMetadataOSNotSupported(c *gc.C) {
	metadata, err := gce.GetMetadata(s.StartInstArgs, ostype.GenericLinux)

	c.Assert(metadata, gc.IsNil)
	c.Assert(err, gc.ErrorMatches, "cannot pack metadata for os GenericLinux on the gce provider")
}

var getDisksTests = []struct {
	osname string
	error  error
}{
	{"ubuntu", nil},
	{"suse", errors.New("os Suse is not supported on the gce provider")},
}

func (s *environBrokerSuite) TestGetDisks(c *gc.C) {
	for _, test := range getDisksTests {
		os := ostype.OSTypeForName(test.osname)
		diskSpecs, err := gce.GetDisks("image-url", s.StartInstArgs.Constraints, os)
		if test.error != nil {
			c.Assert(err, gc.Equals, err)
		} else {
			c.Assert(err, jc.ErrorIsNil)
			c.Assert(diskSpecs, gc.HasLen, 1)
			diskSpec := diskSpecs[0]
			c.Assert(diskSpec.InitializeParams, gc.NotNil)
			c.Check(diskSpec.InitializeParams.GetDiskSizeGb(), gc.Equals, int64(10))
			c.Check(diskSpec.InitializeParams.GetSourceImage(), gc.Equals, "image-url")
		}
	}
}

func (s *environBrokerSuite) TestGetHardwareCharacteristics(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	hwc := gce.GetHardwareCharacteristics(env, s.spec, s.NewEnvironInstance(env, "inst-0"))

	c.Assert(hwc, gc.NotNil)
	c.Check(*hwc.Arch, gc.Equals, "amd64")
	c.Check(*hwc.AvailabilityZone, gc.Equals, "home-zone")
	c.Check(*hwc.CpuCores, gc.Equals, uint64(2))
	c.Check(*hwc.Mem, gc.Equals, uint64(3750))
	c.Check(*hwc.RootDisk, gc.Equals, uint64(15360))
}

func (s *environBrokerSuite) TestAllRunningInstances(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return([]*computepb.Instance{s.NewComputeInstance("inst-0")}, nil)

	insts, err := env.AllRunningInstances(s.CallCtx)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(insts, jc.DeepEquals, []instances.Instance{s.NewEnvironInstance(env, "inst-0")})
}

func (s *environBrokerSuite) TestStopInstances(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().RemoveInstances(gomock.Any(), s.Prefix(env), "inst-0").Return(nil)

	err := env.StopInstances(s.CallCtx, "inst-0")
	c.Assert(err, jc.ErrorIsNil)
}

func (s *environBrokerSuite) TestStopInstancesInvalidCredentialError(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	c.Assert(s.InvalidatedCredentials, jc.IsFalse)

	s.MockService.EXPECT().RemoveInstances(gomock.Any(), s.Prefix(env), "inst-0").Return(gce.InvalidCredentialError)

	err := env.StopInstances(s.CallCtx, "inst-0")
	c.Check(err, gc.NotNil)
	c.Assert(s.InvalidatedCredentials, jc.IsTrue)
}

func (s *environBrokerSuite) TestHasAccelerator(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	zone := "us-central1"

	// Empty Instance type
	hasGPU, err := gce.HasAccelerator(env, s.CallCtx, zone, "")
	c.Assert(err, gc.IsNil)
	c.Assert(hasGPU, jc.IsFalse)

	// Instance has no GPU
	instanceTypeNoGPU := "e2-standard-8"
	s.MockService.EXPECT().
		MachineType(gomock.Any(), zone, instanceTypeNoGPU).
		Return(&computepb.MachineType{
			Name:         &instanceTypeNoGPU,
			Accelerators: nil, // no accelerators
		}, nil)

	hasGPU, err = gce.HasAccelerator(env, s.CallCtx, zone, instanceTypeNoGPU)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(hasGPU, jc.IsFalse)

	// Instance type has GPU
	instanceTypeGPU := "g2-standard-8"
	s.MockService.EXPECT().
		MachineType(gomock.Any(), zone, instanceTypeGPU).
		Return(&computepb.MachineType{
			Name: &instanceTypeGPU,
			Accelerators: []*computepb.Accelerators{
				{
					GuestAcceleratorType:  ptr("nvidia-l4"),
					GuestAcceleratorCount: ptr(int32(1)),
				},
			},
		}, nil)

	hasGPU, err = gce.HasAccelerator(env, s.CallCtx, zone, instanceTypeGPU)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(hasGPU, jc.IsTrue)
}
