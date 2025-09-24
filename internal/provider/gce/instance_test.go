// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package gce_test

import (
	"cloud.google.com/go/compute/apiv1/computepb"
	jc "github.com/juju/testing/checkers"
	"go.uber.org/mock/gomock"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/network/firewall"
	"github.com/juju/juju/internal/provider/gce"
	"github.com/juju/juju/internal/provider/gce/internal/google"
)

type instanceSuite struct {
	gce.BaseSuite
}

var _ = gc.Suite(&instanceSuite{})

func (s *instanceSuite) TestID(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	inst := s.NewEnvironInstance(env, "inst-0")
	id := inst.Id()
	c.Assert(id, gc.Equals, instance.Id("inst-0"))
}

func (s *instanceSuite) TestStatus(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	inst := s.NewEnvironInstance(env, "inst-0")
	status := inst.Status(s.CallCtx).Message
	c.Assert(status, gc.Equals, google.StatusRunning)
}

func (s *instanceSuite) TestAddresses(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	inst := s.NewEnvironInstance(env, "inst-0")
	s.GoogleInstance(c, inst).NetworkInterfaces = []*computepb.NetworkInterface{{
		Name:       ptr("somenetif"),
		NetworkIP:  ptr("10.0.10.3"),
		Network:    ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/global/networks/default"),
		Subnetwork: ptr("https://www.googleapis.com/compute/v1/projects/sonic-youth/regions/asia-east1/subnetworks/sub-network1"),
	}}

	addresses, err := inst.Addresses(s.CallCtx)
	c.Assert(err, jc.ErrorIsNil)

	expectedAddresses := network.ProviderAddresses{
		network.NewMachineAddress("10.0.10.3", network.WithScope(network.ScopeCloudLocal)).AsProviderAddress(),
	}
	c.Assert(addresses, jc.DeepEquals, expectedAddresses)
}

func (s *instanceSuite) TestOpenPorts(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	inst := s.NewEnvironInstance(env, "inst-0")

	fwName := s.Prefix(env) + "42"
	s.MockService.EXPECT().Firewalls(gomock.Any(), fwName).Return([]*computepb.Firewall{{
		Name:         &fwName,
		TargetTags:   []string{fwName},
		SourceRanges: []string{"0.0.0.0/0"},
		Allowed: []*computepb.Allowed{{
			IPProtocol: ptr("tcp"),
			Ports:      []string{"81"},
		}},
	}}, nil)
	s.MockService.EXPECT().UpdateFirewall(gomock.Any(), fwName, &computepb.Firewall{
		Name:         &fwName,
		Description:  ptr("created by Juju with target " + fwName),
		TargetTags:   []string{fwName},
		SourceRanges: []string{"0.0.0.0/0"},
		Allowed: []*computepb.Allowed{{
			IPProtocol: ptr("tcp"),
			Ports:      []string{"81", "80"},
		}},
	})

	rules := firewall.IngressRules{
		firewall.NewIngressRule(network.MustParsePortRange("80/tcp"), "0.0.0.0/0"),
	}
	err := inst.OpenPorts(s.CallCtx, "42", rules)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *instanceSuite) TestClosePorts(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)
	inst := s.NewEnvironInstance(env, "inst-0")

	fwName := s.Prefix(env) + "42"
	s.MockService.EXPECT().Firewalls(gomock.Any(), fwName).Return([]*computepb.Firewall{{
		Name:         &fwName,
		TargetTags:   []string{fwName},
		SourceRanges: []string{"0.0.0.0/0"},
		Allowed: []*computepb.Allowed{{
			IPProtocol: ptr("tcp"),
			Ports:      []string{"80"},
		}},
	}}, nil)
	s.MockService.EXPECT().RemoveFirewall(gomock.Any(), fwName).Return(nil)

	rules := firewall.IngressRules{
		firewall.NewIngressRule(network.MustParsePortRange("80/tcp"), "0.0.0.0/0"),
	}
	err := inst.ClosePorts(s.CallCtx, "42", rules)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *instanceSuite) TestPorts(c *gc.C) {
	ctrl := s.SetupMocks(c)
	defer ctrl.Finish()

	env := s.SetupEnv(c, s.MockService)

	fwName := s.Prefix(env) + "42"
	s.MockService.EXPECT().Firewalls(gomock.Any(), fwName).Return([]*computepb.Firewall{{
		Name:         &fwName,
		TargetTags:   []string{fwName},
		SourceRanges: []string{"0.0.0.0/0"},
		Allowed: []*computepb.Allowed{{
			IPProtocol: ptr("tcp"),
			Ports:      []string{"80"},
		}},
	}}, nil)

	inst := s.NewEnvironInstance(env, "inst-0")
	ports, err := inst.IngressRules(s.CallCtx, "42")
	c.Assert(err, jc.ErrorIsNil)

	rules := firewall.IngressRules{
		firewall.NewIngressRule(network.MustParsePortRange("80/tcp"), "0.0.0.0/0"),
	}
	c.Assert(ports, jc.DeepEquals, rules)
}
