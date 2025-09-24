// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package gce_test

import (
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/juju/collections/set"
	"github.com/juju/errors"
	jc "github.com/juju/testing/checkers"
	"go.uber.org/mock/gomock"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/instance"
	corenetwork "github.com/juju/juju/core/network"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/internal/provider/gce"
)

type environNetSuite struct {
	gce.BaseSuite

	zones     []*computepb.Zone
	instances []*computepb.Instance
	networks  []*computepb.Network
	subnets   []*computepb.Subnetwork
}

var _ = gc.Suite(&environNetSuite{})

func (s *environNetSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.zones = []*computepb.Zone{{
		Name:   ptr("home-zone"),
		Status: ptr("UP"),
	}, {
		Name:   ptr("away-zone"),
		Status: ptr("UP"),
	}}
	s.instances = []*computepb.Instance{{
		Name: ptr("inst-0"),
		Zone: ptr("home-zone"),
		NetworkInterfaces: []*computepb.NetworkInterface{{
			Name:       ptr("netif-0"),
			NetworkIP:  ptr("10.0.20.3"),
			Subnetwork: ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network2"),
		}},
	}, {
		Name: ptr("inst-1"),
		Zone: ptr("away-zone"),
		NetworkInterfaces: []*computepb.NetworkInterface{{
			Name:       ptr("netif-0"),
			NetworkIP:  ptr("10.0.10.42"),
			Subnetwork: ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network1"),
		}},
	}}
	s.networks = []*computepb.Network{{
		Name:     ptr("default"),
		SelfLink: ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/default"),
		Subnetworks: []string{
			"https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network1",
			"https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network2",
			"https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network3",
		},
	}, {
		Name:     ptr("another"),
		SelfLink: ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/another"),
		Subnetworks: []string{
			"https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network4",
		},
	}, {
		Name:      ptr("legacy"),
		IPv4Range: ptr("10.240.0.0/16"),
		SelfLink:  ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/legacy"),
	}}
	s.subnets = []*computepb.Subnetwork{{
		Name:        ptr("sub-network1"),
		IpCidrRange: ptr("10.0.10.0/24"),
		SelfLink:    ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network1"),
		Network:     ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/default"),
	}, {
		Name:        ptr("sub-network2"),
		IpCidrRange: ptr("10.0.20.0/24"),
		SelfLink:    ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network2"),
		Network:     ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/default"),
	}, {
		Name:        ptr("sub-network4"),
		IpCidrRange: ptr("10.0.40.0/24"),
		SelfLink:    ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network4"),
		Network:     ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/another"),
	}}
}

func (s *environNetSuite) TestSubnetsInvalidCredentialError(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	c.Assert(s.InvalidatedCredentials, jc.IsFalse)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(nil, gce.InvalidCredentialError)

	_, err := env.Subnets(s.CallCtx, instance.UnknownId, nil)
	c.Check(err, gc.NotNil)
	c.Assert(s.InvalidatedCredentials, jc.IsTrue)
}

func (s *environNetSuite) TestGettingAllSubnets(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1", []any{s.networks[0].Subnetworks}...).Return(s.subnets[:2], nil)
	subnets, err := env.Subnets(s.CallCtx, instance.UnknownId, nil)
	c.Assert(err, jc.ErrorIsNil)

	c.Assert(subnets, gc.DeepEquals, []corenetwork.SubnetInfo{{
		ProviderId:        "sub-network1",
		ProviderNetworkId: "default",
		CIDR:              "10.0.10.0/24",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		VLANTag:           0,
	}, {
		ProviderId:        "sub-network2",
		ProviderNetworkId: "default",
		CIDR:              "10.0.20.0/24",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		VLANTag:           0,
	}})
}

func (s *environNetSuite) TestGettingAllSubnetsLegacy(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcID(env, ptr("legacy"))

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Network(gomock.Any(), "legacy").Return(s.networks[2], nil)

	subnets, err := env.Subnets(s.CallCtx, instance.UnknownId, nil)
	c.Assert(err, jc.ErrorIsNil)

	c.Assert(subnets, gc.DeepEquals, []corenetwork.SubnetInfo{{
		ProviderId:        "legacy",
		ProviderNetworkId: "legacy",
		CIDR:              "10.240.0.0/16",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		VLANTag:           0,
	}})
}

func (s *environNetSuite) TestGettingAllSubnetsWithNoVPCCompat(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	// Upgraded models will not have a VPC set.
	s.SetVpcID(env, nil)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Networks(gomock.Any()).Return([]*computepb.Network{s.networks[2]}, nil)

	subnets, err := env.Subnets(s.CallCtx, instance.UnknownId, nil)
	c.Assert(err, jc.ErrorIsNil)

	c.Assert(subnets, gc.DeepEquals, []corenetwork.SubnetInfo{{
		ProviderId:        "legacy",
		ProviderNetworkId: "legacy",
		CIDR:              "10.240.0.0/16",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		VLANTag:           0,
	}})
}

func (s *environNetSuite) TestSuperSubnets(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, nil, false)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[1], nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1").Return(s.subnets, nil)

	subnets, err := env.SuperSubnets(s.CallCtx)
	c.Assert(err, jc.ErrorIsNil)

	c.Assert(subnets, gc.DeepEquals, []string{
		"10.0.40.0/24",
	})
}

func (s *environNetSuite) TestRestrictingToSubnets(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1", []any{s.networks[0].Subnetworks[0]}...).
		Return(s.subnets[:1], nil)

	subnets, err := env.Subnets(s.CallCtx, instance.UnknownId, []corenetwork.Id{
		"sub-network1",
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(subnets, gc.DeepEquals, []corenetwork.SubnetInfo{{
		ProviderId:        "sub-network1",
		ProviderNetworkId: "default",
		CIDR:              "10.0.10.0/24",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		VLANTag:           0,
	}})
}

func (s *environNetSuite) TestRestrictingToSubnetsWithMissing(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)

	subnets, err := env.Subnets(s.CallCtx, instance.UnknownId, []corenetwork.Id{"sub-network1", "sub-network4"})
	c.Assert(err, gc.ErrorMatches, `subnets \["sub-network4"\] not found`)
	c.Assert(err, jc.Satisfies, errors.IsNotFound)
	c.Assert(subnets, gc.IsNil)
}

func (s *environNetSuite) TestSpecificInstance(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil).Times(2)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)
	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return(s.instances, nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1", *s.instances[0].NetworkInterfaces[0].Subnetwork).
		Return(s.subnets, nil)

	subnets, err := env.Subnets(s.CallCtx, "inst-0", nil)
	c.Assert(err, jc.ErrorIsNil)

	c.Assert(subnets, gc.DeepEquals, []corenetwork.SubnetInfo{{
		ProviderId:        "sub-network2",
		ProviderNetworkId: "default",
		CIDR:              "10.0.20.0/24",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		VLANTag:           0,
	}})
}

func (s *environNetSuite) TestSpecificInstanceAndRestrictedSubnets(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil).Times(2)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)
	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return(s.instances, nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1", *s.instances[0].NetworkInterfaces[0].Subnetwork).
		Return(s.subnets, nil)

	subnets, err := env.Subnets(s.CallCtx, "inst-0", []corenetwork.Id{"sub-network2"})
	c.Assert(err, jc.ErrorIsNil)

	c.Assert(subnets, gc.DeepEquals, []corenetwork.SubnetInfo{{
		ProviderId:        "sub-network2",
		ProviderNetworkId: "default",
		CIDR:              "10.0.20.0/24",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		VLANTag:           0,
	}})
}

func (s *environNetSuite) TestSpecificInstanceAndRestrictedSubnetsWithMissing(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, s.networks[0].SelfLink, false)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil).Times(2)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)
	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return(s.instances, nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1", *s.instances[0].NetworkInterfaces[0].Subnetwork).
		Return(s.subnets, nil)

	subnets, err := env.Subnets(s.CallCtx, "inst-0", []corenetwork.Id{"sub-network1", "sub-network2"})
	c.Assert(err, gc.ErrorMatches, `subnets \["sub-network1"\] not found`)
	c.Assert(err, jc.Satisfies, errors.IsNotFound)
	c.Assert(subnets, gc.IsNil)
}

func (s *environNetSuite) TestInterfaces(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return(s.instances, nil)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1", s.networks[0].Subnetworks[1]).
		Return(s.subnets, nil)

	infoList, err := env.NetworkInterfaces(s.CallCtx, []instance.Id{"inst-0"})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(infoList, gc.HasLen, 1)
	infos := infoList[0]

	c.Assert(infos, gc.DeepEquals, corenetwork.InterfaceInfos{{
		DeviceIndex:       0,
		ProviderId:        "inst-0/netif-0",
		ProviderSubnetId:  "sub-network2",
		ProviderNetworkId: "default",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		InterfaceName:     "netif-0",
		InterfaceType:     corenetwork.EthernetDevice,
		Disabled:          false,
		NoAutoStart:       false,
		Addresses: corenetwork.ProviderAddresses{corenetwork.NewMachineAddress(
			"10.0.20.3",
			corenetwork.WithScope(corenetwork.ScopeCloudLocal),
			corenetwork.WithCIDR("10.0.20.0/24"),
			corenetwork.WithConfigType(corenetwork.ConfigDHCP),
		).AsProviderAddress()},
		Origin: corenetwork.OriginProvider,
	}})
}

func (s *environNetSuite) TestNetworkInterfaceInvalidCredentialError(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	c.Assert(s.InvalidatedCredentials, jc.IsFalse)

	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return(nil, gce.InvalidCredentialError)

	_, err := env.NetworkInterfaces(s.CallCtx, []instance.Id{"inst-0"})
	c.Check(err, gc.NotNil)
	c.Assert(s.InvalidatedCredentials, jc.IsTrue)
}

func (s *environNetSuite) TestInterfacesForMultipleInstances(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return(s.instances, nil)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1", s.networks[0].Subnetworks[0], s.networks[0].Subnetworks[1]).
		Return(s.subnets, nil)

	s.instances[1].NetworkInterfaces = append(s.instances[1].NetworkInterfaces, &computepb.NetworkInterface{
		Name:       ptr("netif-1"),
		NetworkIP:  ptr("10.0.20.44"),
		Network:    ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/default"),
		Subnetwork: ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network1"),
		AccessConfigs: []*computepb.AccessConfig{{
			Type:  ptr("ONE_TO_ONE_NAT"),
			Name:  ptr("External NAT"),
			NatIP: ptr("25.185.142.227"),
		}},
	})

	infoLists, err := env.NetworkInterfaces(s.CallCtx, []instance.Id{"inst-0", "inst-1"})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(infoLists, gc.HasLen, 2)

	// Check interfaces for first instance
	infos := infoLists[0]
	c.Assert(infos, jc.DeepEquals, corenetwork.InterfaceInfos{{
		DeviceIndex:       0,
		ProviderId:        "inst-0/netif-0",
		ProviderSubnetId:  "sub-network2",
		ProviderNetworkId: "default",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		InterfaceName:     "netif-0",
		InterfaceType:     corenetwork.EthernetDevice,
		Disabled:          false,
		NoAutoStart:       false,
		Addresses: corenetwork.ProviderAddresses{corenetwork.NewMachineAddress(
			"10.0.20.3",
			corenetwork.WithScope(corenetwork.ScopeCloudLocal),
			corenetwork.WithCIDR("10.0.20.0/24"),
			corenetwork.WithConfigType(corenetwork.ConfigDHCP),
		).AsProviderAddress()},
		Origin: corenetwork.OriginProvider,
	}})

	// Check interfaces for second instance
	infos = infoLists[1]
	c.Assert(infos, jc.DeepEquals, corenetwork.InterfaceInfos{{
		DeviceIndex:       0,
		ProviderId:        "inst-1/netif-0",
		ProviderSubnetId:  "sub-network1",
		ProviderNetworkId: "default",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		InterfaceName:     "netif-0",
		InterfaceType:     corenetwork.EthernetDevice,
		Disabled:          false,
		NoAutoStart:       false,
		Addresses: corenetwork.ProviderAddresses{corenetwork.NewMachineAddress(
			"10.0.10.42",
			corenetwork.WithScope(corenetwork.ScopeCloudLocal),
			corenetwork.WithCIDR("10.0.10.0/24"),
			corenetwork.WithConfigType(corenetwork.ConfigDHCP),
		).AsProviderAddress()},
		Origin: corenetwork.OriginProvider,
	}, {
		DeviceIndex:       1,
		ProviderId:        "inst-1/netif-1",
		ProviderSubnetId:  "sub-network1",
		ProviderNetworkId: "default",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		InterfaceName:     "netif-1",
		InterfaceType:     corenetwork.EthernetDevice,
		Disabled:          false,
		NoAutoStart:       false,
		Addresses: corenetwork.ProviderAddresses{corenetwork.NewMachineAddress(
			"10.0.20.44",
			corenetwork.WithScope(corenetwork.ScopeCloudLocal),
			corenetwork.WithCIDR("10.0.10.0/24"),
			corenetwork.WithConfigType(corenetwork.ConfigDHCP),
		).AsProviderAddress()},
		ShadowAddresses: corenetwork.ProviderAddresses{
			corenetwork.NewMachineAddress("25.185.142.227", corenetwork.WithScope(corenetwork.ScopePublic)).AsProviderAddress(),
		},
		Origin: corenetwork.OriginProvider,
	}})
}

func (s *environNetSuite) TestPartialInterfacesForMultipleInstances(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return(s.instances, nil)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1", []any{s.networks[0].Subnetworks[1]}...).
		Return(s.subnets[1:], nil)

	infoLists, err := env.NetworkInterfaces(s.CallCtx, []instance.Id{"inst-0", "bogus"})
	c.Assert(err, gc.Equals, environs.ErrPartialInstances)
	c.Assert(infoLists, gc.HasLen, 2)

	// Check interfaces for first instance
	infos := infoLists[0]
	c.Assert(infos, gc.DeepEquals, corenetwork.InterfaceInfos{{
		DeviceIndex:       0,
		ProviderId:        "inst-0/netif-0",
		ProviderSubnetId:  "sub-network2",
		ProviderNetworkId: "default",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		InterfaceName:     "netif-0",
		InterfaceType:     corenetwork.EthernetDevice,
		Disabled:          false,
		NoAutoStart:       false,
		Addresses: corenetwork.ProviderAddresses{corenetwork.NewMachineAddress(
			"10.0.20.3",
			corenetwork.WithScope(corenetwork.ScopeCloudLocal),
			corenetwork.WithCIDR("10.0.20.0/24"),
			corenetwork.WithConfigType(corenetwork.ConfigDHCP),
		).AsProviderAddress()},
		Origin: corenetwork.OriginProvider,
	}})

	// Check that the slot for the second instance is nil
	c.Assert(infoLists[1], gc.IsNil, gc.Commentf("expected slot for unknown instance to be nil"))
}

func (s *environNetSuite) TestInterfacesMulti(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.instances[0].NetworkInterfaces = append(s.instances[0].NetworkInterfaces, &computepb.NetworkInterface{
		Name:       ptr("othernetif"),
		NetworkIP:  ptr("10.0.10.4"),
		Network:    ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/default"),
		Subnetwork: ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network1"),
		AccessConfigs: []*computepb.AccessConfig{{
			Type:  ptr("ONE_TO_ONE_NAT"),
			Name:  ptr("External NAT"),
			NatIP: ptr("25.185.142.227"),
		}},
	})

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return(s.instances, nil)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1",
		[]any{s.networks[0].Subnetworks[0], s.networks[0].Subnetworks[1]}...).
		Return(s.subnets, nil)

	infoList, err := env.NetworkInterfaces(s.CallCtx, []instance.Id{"inst-0"})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(infoList, gc.HasLen, 1)
	infos := infoList[0]

	c.Assert(infos, gc.DeepEquals, corenetwork.InterfaceInfos{{
		DeviceIndex:       0,
		ProviderId:        "inst-0/netif-0",
		ProviderSubnetId:  "sub-network2",
		ProviderNetworkId: "default",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		InterfaceName:     "netif-0",
		InterfaceType:     corenetwork.EthernetDevice,
		Disabled:          false,
		NoAutoStart:       false,
		Addresses: corenetwork.ProviderAddresses{corenetwork.NewMachineAddress(
			"10.0.20.3",
			corenetwork.WithScope(corenetwork.ScopeCloudLocal),
			corenetwork.WithCIDR("10.0.20.0/24"),
			corenetwork.WithConfigType(corenetwork.ConfigDHCP),
		).AsProviderAddress()},
		Origin: corenetwork.OriginProvider,
	}, {
		DeviceIndex:       1,
		ProviderId:        "inst-0/othernetif",
		ProviderSubnetId:  "sub-network1",
		ProviderNetworkId: "default",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		InterfaceName:     "othernetif",
		InterfaceType:     corenetwork.EthernetDevice,
		Disabled:          false,
		NoAutoStart:       false,
		Addresses: corenetwork.ProviderAddresses{corenetwork.NewMachineAddress(
			"10.0.10.4",
			corenetwork.WithScope(corenetwork.ScopeCloudLocal),
			corenetwork.WithCIDR("10.0.10.0/24"),
			corenetwork.WithConfigType(corenetwork.ConfigDHCP),
		).AsProviderAddress()},
		ShadowAddresses: corenetwork.ProviderAddresses{
			corenetwork.NewMachineAddress("25.185.142.227", corenetwork.WithScope(corenetwork.ScopePublic)).AsProviderAddress(),
		},
		Origin: corenetwork.OriginProvider,
	}})
}

func (s *environNetSuite) TestInterfacesLegacy(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService) // When we're using a legacy network there'll be no subnet.

	s.instances[0].NetworkInterfaces = []*computepb.NetworkInterface{{
		Name:      ptr("somenetif"),
		NetworkIP: ptr("10.240.0.2"),
		Network:   ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/legacy"),
		AccessConfigs: []*computepb.AccessConfig{{
			Type:  ptr("ONE_TO_ONE_NAT"),
			Name:  ptr("External NAT"),
			NatIP: ptr("25.185.142.227"),
		}},
	}}

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return(s.instances, nil)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[2], nil)

	infoList, err := env.NetworkInterfaces(s.CallCtx, []instance.Id{"inst-0"})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(infoList, gc.HasLen, 1)
	infos := infoList[0]

	c.Assert(infos, gc.DeepEquals, corenetwork.InterfaceInfos{{
		DeviceIndex:       0,
		ProviderId:        "inst-0/somenetif",
		ProviderSubnetId:  "",
		ProviderNetworkId: "legacy",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		InterfaceName:     "somenetif",
		InterfaceType:     corenetwork.EthernetDevice,
		Disabled:          false,
		NoAutoStart:       false,
		Addresses: corenetwork.ProviderAddresses{corenetwork.NewMachineAddress(
			"10.240.0.2",
			corenetwork.WithScope(corenetwork.ScopeCloudLocal),
			corenetwork.WithCIDR("10.240.0.0/16"),
			corenetwork.WithConfigType(corenetwork.ConfigDHCP),
		).AsProviderAddress()},
		ShadowAddresses: corenetwork.ProviderAddresses{
			corenetwork.NewMachineAddress("25.185.142.227", corenetwork.WithScope(corenetwork.ScopePublic)).AsProviderAddress(),
		},
		Origin: corenetwork.OriginProvider,
	}})
}

func (s *environNetSuite) TestInterfacesSameSubnetwork(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.instances[0].NetworkInterfaces = append(s.instances[0].NetworkInterfaces, &computepb.NetworkInterface{
		Name:       ptr("othernetif"),
		NetworkIP:  ptr("10.0.10.4"),
		Network:    ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/default"),
		Subnetwork: ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/us-east1/subnetworks/sub-network1"),
		AccessConfigs: []*computepb.AccessConfig{{
			Type:  ptr("ONE_TO_ONE_NAT"),
			Name:  ptr("External NAT"),
			NatIP: ptr("25.185.142.227"),
		}},
	})

	s.MockService.EXPECT().AvailabilityZones(gomock.Any(), "us-east1").Return(s.zones, nil)
	s.MockService.EXPECT().Instances(gomock.Any(), s.Prefix(env), "PENDING", "STAGING", "RUNNING").
		Return(s.instances, nil)
	s.MockService.EXPECT().Network(gomock.Any(), "some-vpc").Return(s.networks[0], nil)
	s.MockService.EXPECT().Subnetworks(gomock.Any(), "us-east1",
		[]any{s.networks[0].Subnetworks[0], s.networks[0].Subnetworks[1]}...).
		Return(s.subnets, nil)

	infoList, err := env.NetworkInterfaces(s.CallCtx, []instance.Id{"inst-0"})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(infoList, gc.HasLen, 1)
	infos := infoList[0]

	c.Assert(infos, gc.DeepEquals, corenetwork.InterfaceInfos{{
		DeviceIndex:       0,
		ProviderId:        "inst-0/netif-0",
		ProviderSubnetId:  "sub-network2",
		ProviderNetworkId: "default",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		InterfaceName:     "netif-0",
		InterfaceType:     corenetwork.EthernetDevice,
		Disabled:          false,
		NoAutoStart:       false,
		Addresses: corenetwork.ProviderAddresses{corenetwork.NewMachineAddress(
			"10.0.20.3",
			corenetwork.WithScope(corenetwork.ScopeCloudLocal),
			corenetwork.WithCIDR("10.0.20.0/24"),
			corenetwork.WithConfigType(corenetwork.ConfigDHCP),
		).AsProviderAddress()},
		Origin: corenetwork.OriginProvider,
	}, {
		DeviceIndex:       1,
		ProviderId:        "inst-0/othernetif",
		ProviderSubnetId:  "sub-network1",
		ProviderNetworkId: "default",
		AvailabilityZones: []string{"home-zone", "away-zone"},
		InterfaceName:     "othernetif",
		InterfaceType:     corenetwork.EthernetDevice,
		Disabled:          false,
		NoAutoStart:       false,
		Addresses: corenetwork.ProviderAddresses{corenetwork.NewMachineAddress(
			"10.0.10.4",
			corenetwork.WithScope(corenetwork.ScopeCloudLocal),
			corenetwork.WithCIDR("10.0.10.0/24"),
			corenetwork.WithConfigType(corenetwork.ConfigDHCP),
		).AsProviderAddress()},
		ShadowAddresses: corenetwork.ProviderAddresses{
			corenetwork.NewMachineAddress("25.185.142.227", corenetwork.WithScope(corenetwork.ScopePublic)).AsProviderAddress(),
		},
		Origin: corenetwork.OriginProvider,
	}})
}

func (s *environNetSuite) TestSubnetsForInstanceNoSubnets(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").Return(nil, nil)

	_, _, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{})
	c.Assert(err, jc.ErrorIs, environs.ErrAvailabilityZoneIndependent)
	c.Assert(err, jc.ErrorIs, gce.ErrNoSubnets)
}

func (s *environNetSuite) TestSubnetsForInstanceNoSubnetsAuto(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").Return(nil, nil)

	vpcLink, subnets, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(vpcLink, gc.NotNil)
	c.Assert(*vpcLink, gc.Equals, "/path/to/vpc")
	c.Assert(subnets, gc.HasLen, 0)
}

func (s *environNetSuite) TestSubnetsForInstancePlacementNoSubnetsAuto(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").Return(nil, nil)

	_, _, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{
		Placement: "subnet=foo",
	})
	c.Assert(err, jc.ErrorIs, environs.ErrAvailabilityZoneIndependent)
	c.Assert(err, jc.ErrorIs, gce.ErrAutoSubnetsInvalid)
}

func (s *environNetSuite) TestSubnetsForInstanceSpacesNoSubnetsAuto(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").Return(nil, nil)

	_, _, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{
		Constraints: constraints.MustParse("spaces=foo"),
	})
	c.Assert(err, jc.ErrorIs, environs.ErrAvailabilityZoneIndependent)
	c.Assert(err, jc.ErrorIs, gce.ErrAutoSubnetsInvalid)
}

func (s *environNetSuite) TestSubnetsForInstanceNoSpacesOrPlacement(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			Name: ptr("subnet1"),
		}, {
			Name: ptr("subnet2"),
		}, {
			Name: ptr("subnet3"),
		}, {
			Name: ptr("subnet4"),
		}}, nil)

	vpcLink, subnets, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(vpcLink, gc.NotNil)
	c.Assert(*vpcLink, gc.Equals, "/path/to/vpc")
	c.Assert(subnets, gc.HasLen, 1)
	c.Assert(subnets[0].Name, gc.NotNil)
	// Result is picked at random from the available subnets.
	c.Assert(set.NewStrings("subnet1", "subnet2", "subnet3", "subnet4").Contains(*subnets[0].Name), jc.IsTrue)
}

func (s *environNetSuite) TestSubnetsForInstanceSpaces(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			Name: ptr("subnet1"),
		}, {
			Name: ptr("subnet2"),
		}, {
			Name: ptr("subnet3"),
		}, {
			Name: ptr("subnet4"),
		}}, nil)

	vpcLink, subnets, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{
		Constraints: constraints.MustParse("spaces=foo,bar"),
		SubnetsToZones: []map[corenetwork.Id][]string{
			{"subnet1": {"home-zone", "away-zone"}, "subnet2": {"home-zone", "away-zone"}},
			{"subnet3": {"home-zone", "away-zone"}},
		},
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(vpcLink, gc.NotNil)
	c.Assert(*vpcLink, gc.Equals, "/path/to/vpc")
	// Only a single nic is allowed since there's only 1 VPC.
	c.Assert(subnets, gc.HasLen, 1)
	c.Assert(subnets[0].Name, gc.NotNil)
	// Result is picked at random from the available subnets.
	c.Assert(set.NewStrings("subnet1", "subnet2").Contains(*subnets[0].Name), jc.IsTrue)
}

func (s *environNetSuite) TestSubnetsForInstanceSpacesFiltersFan(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			Name: ptr("subnet1"),
		}}, nil)

	vpcLink, subnets, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{
		Constraints: constraints.MustParse("spaces=foo"),
		SubnetsToZones: []map[corenetwork.Id][]string{
			{"subnet1": {"home-zone", "away-zone"}, "INFAN": {}},
		},
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(vpcLink, gc.NotNil)
	c.Assert(*vpcLink, gc.Equals, "/path/to/vpc")
	c.Assert(subnets, gc.HasLen, 1)
	c.Assert(subnets[0].Name, gc.NotNil)
	c.Assert(*subnets[0].Name, gc.Equals, "subnet1")
}

func (s *environNetSuite) TestSubnetsForInstancePlacement(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			Name: ptr("subnet1"),
		}, {
			Name: ptr("subnet2"),
		}, {
			Name: ptr("subnet3"),
		}, {
			Name: ptr("subnet4"),
		}}, nil)

	vpcLink, subnets, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{
		Placement: "subnet=subnet3",
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(vpcLink, gc.NotNil)
	c.Assert(*vpcLink, gc.Equals, "/path/to/vpc")
	c.Assert(subnets, gc.HasLen, 1)
	c.Assert(subnets[0].Name, gc.NotNil)
	c.Assert(*subnets[0].Name, gc.Equals, "subnet3")
}

func (s *environNetSuite) TestSubnetsForInstancePlacementCIDR(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			Name:        ptr("subnet1"),
			IpCidrRange: ptr("10.0.10.0/24"),
		}, {
			Name:        ptr("subnet2"),
			IpCidrRange: ptr("10.0.20.0/24"),
		}, {
			Name:        ptr("subnet3"),
			IpCidrRange: ptr("10.0.30.0/24"),
		}, {
			Name:        ptr("subnet4"),
			IpCidrRange: ptr("10.0.40.0/24"),
		}}, nil)

	vpcLink, subnets, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{
		Placement: "subnet=10.0.30.0/24",
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(vpcLink, gc.NotNil)
	c.Assert(*vpcLink, gc.Equals, "/path/to/vpc")
	c.Assert(subnets, gc.HasLen, 1)
	c.Assert(subnets[0].Name, gc.NotNil)
	c.Assert(*subnets[0].Name, gc.Equals, "subnet3")
}

func (s *environNetSuite) TestSubnetsForInstancePlacementWithSpaces(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			Name: ptr("subnet1"),
		}, {
			Name: ptr("subnet2"),
		}, {
			Name: ptr("subnet3"),
		}, {
			Name: ptr("subnet4"),
		}}, nil)

	vpcLink, subnets, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{
		Placement:   "subnet=subnet2",
		Constraints: constraints.MustParse("spaces=foo,bar"),
		SubnetsToZones: []map[corenetwork.Id][]string{
			{"subnet1": {"home-zone", "away-zone"}, "subnet2": {"home-zone", "away-zone"}},
			{"subnet3": {"home-zone", "away-zone"}},
		},
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(vpcLink, gc.NotNil)
	c.Assert(*vpcLink, gc.Equals, "/path/to/vpc")
	c.Assert(subnets, gc.HasLen, 1)
	c.Assert(subnets[0].Name, gc.NotNil)
	c.Assert(*subnets[0].Name, gc.Equals, "subnet2")
}

func (s *environNetSuite) TestSubnetsForInstancePlacementWithSpacesNotFound(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			Name: ptr("subnet1"),
		}, {
			Name: ptr("subnet2"),
		}, {
			Name: ptr("subnet3"),
		}, {
			Name: ptr("subnet4"),
		}}, nil)

	_, _, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{
		// We ask for subnet4 but that's not in the subnets in any of those filtered by the space constraint.
		Placement:   "subnet=subnet4",
		Constraints: constraints.MustParse("spaces=foo,bar"),
		SubnetsToZones: []map[corenetwork.Id][]string{
			{"subnet1": {"home-zone", "away-zone"}, "subnet2": {"home-zone", "away-zone"}},
			{"subnet3": {"home-zone", "away-zone"}},
		},
	})
	c.Assert(err, jc.ErrorIs, errors.NotFound)
}

func (s *environNetSuite) TestSubnetsForInstancePlacementNotFound(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	s.SetVpcInfo(env, ptr("/path/to/vpc"), true)

	s.MockService.EXPECT().NetworkSubnetworks(gomock.Any(), "us-east1", "/path/to/vpc").
		Return([]*computepb.Subnetwork{{
			Name: ptr("subnet1"),
		}, {
			Name: ptr("subnet2"),
		}, {
			Name: ptr("subnet3"),
		}, {
			Name: ptr("subnet4"),
		}}, nil)

	_, _, err := gce.SubnetsForInstance(env, s.CallCtx, environs.StartInstanceParams{
		Placement: "subnet=subnet5",
	})
	c.Assert(err, jc.ErrorIs, errors.NotFound)
}
