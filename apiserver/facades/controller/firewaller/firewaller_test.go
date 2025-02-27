// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package firewaller_test

import (
	"context"

	jc "github.com/juju/testing/checkers"
	"go.uber.org/mock/gomock"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/common/cloudspec"
	"github.com/juju/juju/apiserver/facade"
	facademocks "github.com/juju/juju/apiserver/facade/mocks"
	"github.com/juju/juju/apiserver/facades/controller/firewaller"
	apiservertesting "github.com/juju/juju/apiserver/testing"
	"github.com/juju/juju/core/machine"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/unit"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	machineerrors "github.com/juju/juju/domain/machine/errors"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
)

type firewallerSuite struct {
	firewallerBaseSuite

	firewaller *firewaller.FirewallerAPI

	watcherRegistry     *facademocks.MockWatcherRegistry
	controllerConfigAPI *MockControllerConfigAPI

	controllerConfigService *MockControllerConfigService
	modelConfigService      *MockModelConfigService
	networkService          *MockNetworkService
	machineService          *MockMachineService
	applicationService      *MockApplicationService
}

var _ = gc.Suite(&firewallerSuite{})

func (s *firewallerSuite) setupMocks(c *gc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.watcherRegistry = facademocks.NewMockWatcherRegistry(ctrl)

	s.controllerConfigAPI = NewMockControllerConfigAPI(ctrl)

	s.controllerConfigService = NewMockControllerConfigService(ctrl)
	s.networkService = NewMockNetworkService(ctrl)
	s.modelConfigService = NewMockModelConfigService(ctrl)
	s.applicationService = NewMockApplicationService(ctrl)
	s.machineService = NewMockMachineService(ctrl)

	return ctrl
}

func (s *firewallerSuite) setupAPI(c *gc.C) {
	st := s.ControllerModel(c).State()

	domainServices := s.ControllerDomainServices(c)

	cloudSpecAPI := cloudspec.NewCloudSpec(
		s.resources,
		cloudspec.MakeCloudSpecGetterForModel(st, domainServices.Cloud(), domainServices.Credential(), domainServices.Config()),
		cloudspec.MakeCloudSpecWatcherForModel(st, domainServices.Cloud()),
		cloudspec.MakeCloudSpecCredentialWatcherForModel(st),
		cloudspec.MakeCloudSpecCredentialContentWatcherForModel(st, domainServices.Credential()),
		common.AuthFuncForTag(s.ControllerModel(c).ModelTag()),
	)

	// Create a firewaller API for the machine.
	firewallerAPI, err := firewaller.NewStateFirewallerAPI(
		firewaller.StateShim(st, s.ControllerModel(c)),
		s.networkService,
		s.resources,
		s.watcherRegistry,
		s.authorizer,
		cloudSpecAPI,
		s.controllerConfigAPI,
		s.controllerConfigService,
		s.modelConfigService,
		s.applicationService,
		s.machineService,
		loggertesting.WrapCheckLog(c),
	)
	c.Assert(err, jc.ErrorIsNil)
	s.firewaller = firewallerAPI
}

func (s *firewallerSuite) TestFirewallerFailsWithNonControllerUser(c *gc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()
	s.setupAPI(c)

	constructor := func(ctx facade.ModelContext) error {
		_, err := firewaller.NewFirewallerAPIV7(ctx)
		return err
	}
	s.testFirewallerFailsWithNonControllerUser(c, constructor)
}

func (s *firewallerSuite) TestLife(c *gc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()
	s.setupAPI(c)

	s.applicationService.EXPECT().GetUnitLife(gomock.Any(), unit.Name("foo/0")).Return("", applicationerrors.UnitNotFound)

	s.testLife(c, s.firewaller)
}

func (s *firewallerSuite) TestInstanceId(c *gc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()
	s.setupAPI(c)

	s.machineService.EXPECT().GetMachineUUID(gomock.Any(), machine.Name("0")).Return("uuid-i-am", nil)
	s.machineService.EXPECT().InstanceID(gomock.Any(), "uuid-i-am").Return("i-am", nil)
	s.machineService.EXPECT().GetMachineUUID(gomock.Any(), machine.Name("1")).Return("uuid-i-am-not", nil)
	s.machineService.EXPECT().InstanceID(gomock.Any(), "uuid-i-am-not").Return("i-am-not", nil)
	s.machineService.EXPECT().GetMachineUUID(gomock.Any(), machine.Name("2")).Return("uuid-i-will", nil)
	s.machineService.EXPECT().InstanceID(gomock.Any(), "uuid-i-will").Return("", machineerrors.NotProvisioned)
	s.machineService.EXPECT().GetMachineUUID(gomock.Any(), machine.Name("42")).Return("", machineerrors.MachineNotFound)

	s.testInstanceId(c, s.firewaller)
}

func (s *firewallerSuite) TestWatchModelMachines(c *gc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()
	s.setupAPI(c)

	s.testWatchModelMachines(c, s.firewaller)
}

func (s *firewallerSuite) TestWatch(c *gc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()
	s.setupAPI(c)

	s.watcherRegistry.EXPECT().Register(gomock.Any()).Return("1", nil)

	args := addFakeEntities(params.Entities{Entities: []params.Entity{
		{Tag: s.machines[0].Tag().String()},
		{Tag: s.application.Tag().String()},
		{Tag: s.units[0].Tag().String()},
	}})
	result, err := s.firewaller.Watch(context.Background(), args)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, jc.DeepEquals, params.NotifyWatchResults{
		Results: []params.NotifyWatchResult{
			{Error: apiservertesting.ErrUnauthorized},
			{NotifyWatcherId: "1"},
			{Error: apiservertesting.ErrUnauthorized},
			{Error: apiservertesting.ErrUnauthorized},
			{Error: apiservertesting.ErrUnauthorized},
			{Error: apiservertesting.NotFoundError(`application "bar"`)},
			{Error: apiservertesting.ErrUnauthorized},
			{Error: apiservertesting.ErrUnauthorized},
			{Error: apiservertesting.ErrUnauthorized},
		},
	})
}

func (s *firewallerSuite) TestWatchUnits(c *gc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()
	s.setupAPI(c)

	s.testWatchUnits(c, s.firewaller)
}

func (s *firewallerSuite) TestGetAssignedMachine(c *gc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()
	s.setupAPI(c)

	s.testGetAssignedMachine(c, s.firewaller)
}

func (s *firewallerSuite) TestAreManuallyProvisioned(c *gc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()
	s.setupAPI(c)

	st := s.ControllerModel(c).State()
	m, err := st.AddOneMachine(
		state.MachineTemplate{
			Base:       state.UbuntuBase("12.10"),
			Jobs:       []state.MachineJob{state.JobHostUnits},
			InstanceId: "2",
			Nonce:      "manual:",
		},
	)
	c.Assert(err, jc.ErrorIsNil)

	args := addFakeEntities(params.Entities{Entities: []params.Entity{
		{Tag: s.machines[0].Tag().String()},
		{Tag: s.machines[1].Tag().String()},
		{Tag: m.Tag().String()},
		{Tag: s.application.Tag().String()},
		{Tag: s.units[0].Tag().String()},
	}})

	result, err := s.firewaller.AreManuallyProvisioned(context.Background(), args)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, jc.DeepEquals, params.BoolResults{
		Results: []params.BoolResult{
			{Result: false, Error: nil},
			{Result: false, Error: nil},
			{Result: true, Error: nil},
			{Result: false, Error: apiservertesting.ServerError(`"application-wordpress" is not a valid machine tag`)},
			{Result: false, Error: apiservertesting.ServerError(`"unit-wordpress-0" is not a valid machine tag`)},
			{Result: false, Error: apiservertesting.NotFoundError("machine 42")},
			{Result: false, Error: apiservertesting.ServerError(`"unit-foo-0" is not a valid machine tag`)},
			{Result: false, Error: apiservertesting.ServerError(`"application-bar" is not a valid machine tag`)},
			{Result: false, Error: apiservertesting.ServerError(`"user-foo" is not a valid machine tag`)},
			{Result: false, Error: apiservertesting.ServerError(`"foo-bar" is not a valid tag`)},
			{Result: false, Error: apiservertesting.ServerError(`"" is not a valid tag`)},
		},
	})
}

func (s *firewallerSuite) TestGetExposeInfo(c *gc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()
	s.setupAPI(c)

	// Set the application to exposed first.
	err := s.application.MergeExposeSettings(map[string]state.ExposedEndpoint{
		"": {
			ExposeToSpaceIDs: []string{network.AlphaSpaceId},
			ExposeToCIDRs:    []string{"10.0.0.0/0"},
		},
	})
	c.Assert(err, jc.ErrorIsNil)

	args := addFakeEntities(params.Entities{Entities: []params.Entity{
		{Tag: s.application.Tag().String()},
	}})
	result, err := s.firewaller.GetExposeInfo(context.Background(), args)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, jc.DeepEquals, params.ExposeInfoResults{
		Results: []params.ExposeInfoResult{
			{
				Exposed: true,
				ExposedEndpoints: map[string]params.ExposedEndpoint{
					"": {
						ExposeToSpaces: []string{network.AlphaSpaceId},
						ExposeToCIDRs:  []string{"10.0.0.0/0"},
					},
				},
			},
			{Error: apiservertesting.ErrUnauthorized},
			{Error: apiservertesting.ErrUnauthorized},
			{Error: apiservertesting.NotFoundError(`application "bar"`)},
			{Error: apiservertesting.ErrUnauthorized},
			{Error: apiservertesting.ErrUnauthorized},
			{Error: apiservertesting.ErrUnauthorized},
		},
	})

	// Now reset the exposed flag for the application and check again.
	err = s.application.ClearExposed()
	c.Assert(err, jc.ErrorIsNil)

	args = params.Entities{Entities: []params.Entity{
		{Tag: s.application.Tag().String()},
	}}
	result, err = s.firewaller.GetExposeInfo(context.Background(), args)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, jc.DeepEquals, params.ExposeInfoResults{
		Results: []params.ExposeInfoResult{
			{Exposed: false},
		},
	})
}
