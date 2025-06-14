// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasapplicationprovisioner_test

import (
	"context"
	"testing"

	"github.com/juju/clock/testclock"
	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	charmscommon "github.com/juju/juju/api/common/charms"
	api "github.com/juju/juju/api/controller/caasapplicationprovisioner"
	"github.com/juju/juju/caas"
	caasmocks "github.com/juju/juju/caas/mocks"
	corebase "github.com/juju/juju/core/base"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/devices"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/resource"
	"github.com/juju/juju/core/semversion"
	"github.com/juju/juju/core/status"
	"github.com/juju/juju/internal/charm"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/storage"
	coretesting "github.com/juju/juju/internal/testing"
	"github.com/juju/juju/internal/worker/caasapplicationprovisioner"
	"github.com/juju/juju/internal/worker/caasapplicationprovisioner/mocks"
	"github.com/juju/juju/rpc/params"
)

func TestOpsSuite(t *testing.T) {
	tc.Run(t, &OpsSuite{})
}

type OpsSuite struct {
	coretesting.BaseSuite

	modelTag names.ModelTag
	logger   logger.Logger
}

func (s *OpsSuite) SetUpTest(c *tc.C) {
	s.BaseSuite.SetUpTest(c)

	s.modelTag = names.NewModelTag("ffffffff-ffff-ffff-ffff-ffffffffffff")
	s.logger = loggertesting.WrapCheckLog(c)
}

func (s *OpsSuite) TestCheckCharmFormatV1(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	charmInfoV1 := &charmscommon.CharmInfo{
		Meta: &charm.Meta{Name: "test"},
	}

	facade := mocks.NewMockCAASProvisionerFacade(ctrl)

	// Wait till charm is v2
	facade.EXPECT().ApplicationCharmInfo(gomock.Any(), "test").Return(charmInfoV1, nil)

	isOk, err := caasapplicationprovisioner.AppOps.CheckCharmFormat(c.Context(), "test", facade, s.logger)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(isOk, tc.IsFalse)
}

func (s *OpsSuite) TestCheckCharmFormatV2(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	charmInfoV2 := &charmscommon.CharmInfo{
		Meta:     &charm.Meta{Name: "test"},
		Manifest: &charm.Manifest{Bases: []charm.Base{{}}},
	}

	facade := mocks.NewMockCAASProvisionerFacade(ctrl)

	// Wait till charm is v2
	facade.EXPECT().ApplicationCharmInfo(gomock.Any(), "test").Return(charmInfoV2, nil)

	isOk, err := caasapplicationprovisioner.AppOps.CheckCharmFormat(c.Context(), "test", facade, s.logger)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(isOk, tc.IsTrue)
}

func (s *OpsSuite) TestCheckCharmFormatNotFound(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	facade := mocks.NewMockCAASProvisionerFacade(ctrl)

	facade.EXPECT().ApplicationCharmInfo(gomock.Any(), "test").DoAndReturn(func(_ context.Context, appName string) (*charmscommon.CharmInfo, error) {
		return nil, errors.NotFoundf("test charm")
	})

	isOk, err := caasapplicationprovisioner.AppOps.CheckCharmFormat(c.Context(), "test", facade, s.logger)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(isOk, tc.IsFalse)
}

func (s *OpsSuite) TestEnsureTrust(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	applicationService := mocks.NewMockApplicationService(ctrl)
	app := caasmocks.NewMockApplication(ctrl)

	gomock.InOrder(
		applicationService.EXPECT().GetApplicationTrustSetting(gomock.Any(), "test").Return(true, nil),
		app.EXPECT().Trust(true).Return(nil),
	)

	err := caasapplicationprovisioner.AppOps.EnsureTrust(c.Context(), "test", app, applicationService, s.logger)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *OpsSuite) TestUpdateState(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	app := caasmocks.NewMockApplication(ctrl)
	broker := mocks.NewMockCAASBroker(ctrl)
	facade := mocks.NewMockCAASProvisionerFacade(ctrl)
	unitFacade := mocks.NewMockCAASUnitProvisionerFacade(ctrl)

	appTag := names.NewApplicationTag("test").String()
	service := &caas.Service{
		Id: "provider-id",
		Status: status.StatusInfo{
			Status:  status.Active,
			Message: "nice message",
			Data: map[string]interface{}{
				"nice": "data",
			},
		},
		Addresses: network.ProviderAddresses{{
			MachineAddress: network.NewMachineAddress("1.2.3.4"),
			SpaceName:      "space-name",
		}},
	}
	updateServiceArg := params.UpdateApplicationServiceArg{
		ApplicationTag: appTag,
		ProviderId:     "provider-id",
		Addresses: []params.Address{{
			Value:     "1.2.3.4",
			SpaceName: "space-name",
			Type:      "ipv4",
			Scope:     "public",
		}},
	}
	units := []caas.Unit{{
		Id:       "a",
		Address:  "1.2.3.5",
		Ports:    []string{"80", "443"},
		Stateful: true,
		Status: status.StatusInfo{
			Status:  status.Active,
			Message: "different",
		},
		FilesystemInfo: []caas.FilesystemInfo{{
			StorageName:  "s",
			FilesystemId: "fsid",
			Volume: caas.VolumeInfo{
				VolumeId: "vid",
			},
		}},
	}, {
		Id:       "b",
		Address:  "1.2.3.6",
		Ports:    []string{"80", "443"},
		Stateful: true,
		Status: status.StatusInfo{
			Status:  status.Allocating,
			Message: "same",
		},
	}}
	updateUnitsArg := params.UpdateApplicationUnits{
		ApplicationTag: appTag,
		Status: params.EntityStatus{
			Status: status.Active,
			Info:   "nice message",
			Data: map[string]interface{}{
				"nice": "data",
			},
		},
		Scale: nil,
		Units: []params.ApplicationUnitParams{{
			ProviderId: "a",
			Address:    "1.2.3.5",
			Ports:      []string{"80", "443"},
			Stateful:   true,
			Status:     "active",
			Info:       "different",
			FilesystemInfo: []params.KubernetesFilesystemInfo{{
				StorageName:  "s",
				FilesystemId: "fsid",
				Volume: params.KubernetesVolumeInfo{
					VolumeId: "vid",
				},
			}},
		}, {
			ProviderId: "b",
			Address:    "1.2.3.6",
			Ports:      []string{"80", "443"},
			Stateful:   true,
			Status:     "unknown",
		}},
	}
	appUnitInfo := &params.UpdateApplicationUnitsInfo{
		Units: []params.ApplicationUnitInfo{{
			UnitTag:    "unit-test-0",
			ProviderId: "a",
		}, {
			UnitTag:    "unit-test-1",
			ProviderId: "b",
		}},
	}
	gomock.InOrder(
		app.EXPECT().Service().Return(service, nil),
		unitFacade.EXPECT().UpdateApplicationService(gomock.Any(), updateServiceArg).Return(nil),
		app.EXPECT().Units().Return(units, nil),
		facade.EXPECT().UpdateUnits(gomock.Any(), updateUnitsArg).Return(appUnitInfo, nil),
		broker.EXPECT().AnnotateUnit(gomock.Any(), "test", "a", names.NewUnitTag("test/0")).Return(nil),
		broker.EXPECT().AnnotateUnit(gomock.Any(), "test", "b", names.NewUnitTag("test/1")).Return(nil),
	)

	lastReportedStatus := map[string]status.StatusInfo{
		"b": {
			Status:  status.Allocating,
			Message: "same",
		},
	}
	currentReportedStatus, err := caasapplicationprovisioner.AppOps.UpdateState(c.Context(), "test", app, lastReportedStatus, broker, facade, unitFacade, s.logger)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(currentReportedStatus, tc.DeepEquals, map[string]status.StatusInfo{
		"a": {Status: "active", Message: "different"},
		"b": {Status: "allocating", Message: "same"},
	})
}

func (s *OpsSuite) TestRefreshApplicationStatus(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	appLife := life.Alive
	app := caasmocks.NewMockApplication(ctrl)
	facade := mocks.NewMockCAASProvisionerFacade(ctrl)

	appState := caas.ApplicationState{
		DesiredReplicas: 2,
	}
	units := []params.CAASUnit{{
		UnitStatus: &params.UnitStatus{AgentStatus: params.DetailedStatus{Status: "active"}},
	}, {
		UnitStatus: &params.UnitStatus{AgentStatus: params.DetailedStatus{Status: "allocating"}},
	}}
	gomock.InOrder(
		app.EXPECT().State().Return(appState, nil),
		facade.EXPECT().Units(gomock.Any(), "test").Return(units, nil),
		facade.EXPECT().SetOperatorStatus(gomock.Any(), "test", status.Waiting, "waiting for units to settle down", nil).Return(nil),
	)

	err := caasapplicationprovisioner.AppOps.RefreshApplicationStatus(c.Context(), "test", app, appLife, facade, s.logger)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *OpsSuite) TestWaitForTerminated(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	app := caasmocks.NewMockApplication(ctrl)
	clk := testclock.NewDilatedWallClock(coretesting.ShortWait)

	gomock.InOrder(
		app.EXPECT().Exists().Return(caas.DeploymentState{
			Exists: true,
		}, nil),
	)
	err := caasapplicationprovisioner.AppOps.WaitForTerminated("test", app, clk)
	c.Assert(err, tc.ErrorMatches, `application "test" should be terminating but is now running`)

	gomock.InOrder(
		app.EXPECT().Exists().Return(caas.DeploymentState{
			Exists:      true,
			Terminating: true,
		}, nil),
		app.EXPECT().Exists().Return(caas.DeploymentState{}, nil),
	)
	err = caasapplicationprovisioner.AppOps.WaitForTerminated("test", app, clk)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *OpsSuite) TestReconcileDeadUnitScale(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	app := caasmocks.NewMockApplication(ctrl)
	facade := mocks.NewMockCAASProvisionerFacade(ctrl)

	units := []params.CAASUnit{{
		Tag: names.NewUnitTag("test/0"),
	}, {
		Tag: names.NewUnitTag("test/1"),
	}}
	ps := params.CAASApplicationProvisioningState{
		Scaling:     true,
		ScaleTarget: 1,
	}
	appState := caas.ApplicationState{
		Replicas: []string{
			"a",
		},
	}
	newPs := params.CAASApplicationProvisioningState{
		Scaling:     false,
		ScaleTarget: 0,
	}
	gomock.InOrder(
		facade.EXPECT().Units(gomock.Any(), "test").Return(units, nil),
		facade.EXPECT().ProvisioningState(gomock.Any(), "test").Return(&ps, nil),
		facade.EXPECT().Life(gomock.Any(), "test/0").Return(life.Alive, nil),
		facade.EXPECT().Life(gomock.Any(), "test/1").Return(life.Dead, nil),
		app.EXPECT().Scale(1).Return(nil),
		app.EXPECT().State().Return(appState, nil),
		facade.EXPECT().RemoveUnit(gomock.Any(), "test/1").Return(nil),
		facade.EXPECT().SetProvisioningState(gomock.Any(), "test", newPs).Return(nil),
	)

	err := caasapplicationprovisioner.AppOps.ReconcileDeadUnitScale(c.Context(), "test", app, facade, s.logger)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *OpsSuite) TestEnsureScaleAlive(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	app := caasmocks.NewMockApplication(ctrl)
	facade := mocks.NewMockCAASProvisionerFacade(ctrl)
	unitFacade := mocks.NewMockCAASUnitProvisionerFacade(ctrl)

	ps := params.CAASApplicationProvisioningState{
		Scaling:     true,
		ScaleTarget: 1,
	}
	units := []params.CAASUnit{{
		Tag: names.NewUnitTag("test/0"),
	}, {
		Tag: names.NewUnitTag("test/1"),
	}}
	unitsToDestroy := []string{"test/1"}
	gomock.InOrder(
		unitFacade.EXPECT().ApplicationScale(gomock.Any(), "test").Return(1, nil),
		facade.EXPECT().ProvisioningState(gomock.Any(), "test").Return(&params.CAASApplicationProvisioningState{}, nil),
		facade.EXPECT().SetProvisioningState(gomock.Any(), "test", ps).Return(nil),
		facade.EXPECT().Units(gomock.Any(), "test").Return(units, nil),
		app.EXPECT().UnitsToRemove(gomock.Any(), 1).Return(unitsToDestroy, nil),
		facade.EXPECT().DestroyUnits(gomock.Any(), unitsToDestroy).Return(nil),
	)

	err := caasapplicationprovisioner.AppOps.EnsureScale(c.Context(), "test", app, life.Alive, facade, unitFacade, s.logger)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *OpsSuite) TestEnsureScaleAliveRetry(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	app := caasmocks.NewMockApplication(ctrl)
	facade := mocks.NewMockCAASProvisionerFacade(ctrl)
	unitFacade := mocks.NewMockCAASUnitProvisionerFacade(ctrl)

	ps := params.CAASApplicationProvisioningState{
		Scaling:     true,
		ScaleTarget: 1,
	}
	units := []params.CAASUnit{{
		Tag: names.NewUnitTag("test/0"),
	}, {
		Tag: names.NewUnitTag("test/1"),
	}}
	unitsToDestroy := []string{"test/1"}
	gomock.InOrder(
		unitFacade.EXPECT().ApplicationScale(gomock.Any(), "test").Return(10, nil),
		facade.EXPECT().ProvisioningState(gomock.Any(), "test").Return(&ps, nil),
		facade.EXPECT().Units(gomock.Any(), "test").Return(units, nil),
		app.EXPECT().UnitsToRemove(gomock.Any(), 1).Return(unitsToDestroy, nil),
		facade.EXPECT().DestroyUnits(gomock.Any(), unitsToDestroy).Return(nil),
	)

	err := caasapplicationprovisioner.AppOps.EnsureScale(c.Context(), "test", app, life.Alive, facade, unitFacade, s.logger)
	c.Assert(err, tc.ErrorMatches, `try again`)
}

func (s *OpsSuite) TestEnsureScaleDyingDead(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	app := caasmocks.NewMockApplication(ctrl)
	facade := mocks.NewMockCAASProvisionerFacade(ctrl)
	unitFacade := mocks.NewMockCAASUnitProvisionerFacade(ctrl)

	ps := params.CAASApplicationProvisioningState{
		Scaling:     true,
		ScaleTarget: 0,
	}
	units := []params.CAASUnit{{
		Tag: names.NewUnitTag("test/0"),
	}, {
		Tag: names.NewUnitTag("test/1"),
	}}
	unitsToDestroy := []string{"test/0", "test/1"}
	gomock.InOrder(
		facade.EXPECT().ProvisioningState(gomock.Any(), "test").Return(&params.CAASApplicationProvisioningState{}, nil),
		facade.EXPECT().SetProvisioningState(gomock.Any(), "test", ps).Return(nil),
		facade.EXPECT().Units(gomock.Any(), "test").Return(units, nil),
		app.EXPECT().UnitsToRemove(gomock.Any(), 0).Return(unitsToDestroy, nil),
		facade.EXPECT().DestroyUnits(gomock.Any(), unitsToDestroy).Return(nil),
	)

	err := caasapplicationprovisioner.AppOps.EnsureScale(c.Context(), "test", app, life.Dead, facade, unitFacade, s.logger)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *OpsSuite) TestAppAlive(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	app := caasmocks.NewMockApplication(ctrl)
	facade := mocks.NewMockCAASProvisionerFacade(ctrl)

	clk := testclock.NewDilatedWallClock(coretesting.ShortWait)
	password := "123456789"
	lastApplied := caas.ApplicationConfig{}

	pi := api.ProvisioningInfo{
		CharmURL: charm.MustParseURL("ch:my-app"),
		ImageDetails: resource.DockerImageDetails{
			RegistryPath: "test-repo/jujud-operator:2.9.99",
			ImageRepoDetails: resource.ImageRepoDetails{
				Repository:    "test-repo",
				ServerAddress: "registry.com",
			},
		},
		Base: corebase.Base{
			OS: "ubuntu",
			Channel: corebase.Channel{
				Track: "22.04",
				Risk:  corebase.Stable,
			},
		},
		Version:              semversion.MustParse("2.9.99"),
		CharmModifiedVersion: 123,
		APIAddresses:         []string{"1.2.3.1", "1.2.3.2", "1.2.3.3"},
		CACert:               "CACERT",
		Tags: map[string]string{
			"tag": "tag-value",
		},
		Trust:       true,
		Scale:       10,
		Constraints: constraints.MustParse("mem=1G"),
		Filesystems: []storage.KubernetesFilesystemParams{{
			StorageName: "data",
			Size:        100,
		}},
		Devices: []devices.KubernetesDeviceParams{},
	}
	charmInfo := charmscommon.CharmInfo{
		Meta: &charm.Meta{
			Containers: map[string]charm.Container{
				"mysql": {
					Resource: "mysql-image",
					Mounts: []charm.Mount{{
						Storage:  "data",
						Location: "/data",
					}},
				},
				"rootless": {
					Resource: "rootless-image",
					Uid:      intPtr(5000),
					Gid:      intPtr(5001),
				},
			},
		},
	}
	ds := caas.DeploymentState{
		Exists:      true,
		Terminating: true,
	}
	oci := map[string]resource.DockerImageDetails{
		"mysql-image": {
			RegistryPath: "mysql/ubuntu:latest-22.04",
		},
		"rootless-image": {
			RegistryPath: "rootless:foo-bar",
		},
	}
	ensureParams := caas.ApplicationConfig{
		AgentVersion:         semversion.Number{Major: 2, Minor: 9, Patch: 99},
		AgentImagePath:       "test-repo/jujud-operator:2.9.99",
		CharmBaseImagePath:   "test-repo/charm-base:ubuntu-22.04",
		CharmModifiedVersion: 123,
		Containers: map[string]caas.ContainerConfig{
			"mysql": {
				Name: "mysql",
				Image: resource.DockerImageDetails{
					RegistryPath: "mysql/ubuntu:latest-22.04",
				},
				Mounts: []caas.MountConfig{{
					StorageName: "data",
					Path:        "/data",
				}},
			},
			"rootless": {
				Name: "rootless",
				Image: resource.DockerImageDetails{
					RegistryPath: "rootless:foo-bar",
				},
				Uid: intPtr(5000),
				Gid: intPtr(5001),
			},
		},
		IntroductionSecret:   "123456789",
		ControllerAddresses:  "1.2.3.1,1.2.3.2,1.2.3.3",
		ControllerCertBundle: "CACERT",
		ResourceTags: map[string]string{
			"tag": "tag-value",
		},
		Constraints: constraints.MustParse("mem=1G"),
		Filesystems: []storage.KubernetesFilesystemParams{{
			StorageName: "data",
			Size:        100,
		}},
		Devices:      []devices.KubernetesDeviceParams{},
		Trust:        true,
		InitialScale: 10,
		CharmUser:    caas.RunAsDefault,
	}
	gomock.InOrder(
		facade.EXPECT().ProvisioningInfo(gomock.Any(), "test").Return(pi, nil),
		facade.EXPECT().CharmInfo(gomock.Any(), "ch:my-app").Return(&charmInfo, nil),
		app.EXPECT().Exists().Return(ds, nil),
		app.EXPECT().Exists().Return(caas.DeploymentState{}, nil),
		facade.EXPECT().ApplicationOCIResources(gomock.Any(), "test").Return(oci, nil),
		app.EXPECT().Ensure(gomock.Any()).DoAndReturn(func(config caas.ApplicationConfig) error {
			c.Check(config, tc.DeepEquals, ensureParams)
			return nil
		}),
	)

	err := caasapplicationprovisioner.AppOps.AppAlive(c.Context(), "test", app, password, &lastApplied, facade, clk, s.logger)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *OpsSuite) TestAppDying(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	app := caasmocks.NewMockApplication(ctrl)
	facade := mocks.NewMockCAASProvisionerFacade(ctrl)
	unitFacade := mocks.NewMockCAASUnitProvisionerFacade(ctrl)

	ps := params.CAASApplicationProvisioningState{
		Scaling:     true,
		ScaleTarget: 0,
	}
	newPs := params.CAASApplicationProvisioningState{}
	gomock.InOrder(
		facade.EXPECT().ProvisioningState(gomock.Any(), "test").Return(&params.CAASApplicationProvisioningState{}, nil),
		facade.EXPECT().SetProvisioningState(gomock.Any(), "test", ps).Return(nil),
		facade.EXPECT().Units(gomock.Any(), "test").Return(nil, nil),
		app.EXPECT().Scale(0).Return(nil),
		facade.EXPECT().SetProvisioningState(gomock.Any(), "test", newPs).Return(nil),
		facade.EXPECT().Units(gomock.Any(), "test").Return(nil, nil),
		facade.EXPECT().ProvisioningState(gomock.Any(), "test").Return(&params.CAASApplicationProvisioningState{}, nil),
	)

	err := caasapplicationprovisioner.AppOps.AppDying(c.Context(), "test", app, life.Dying, facade, unitFacade, s.logger)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *OpsSuite) TestAppDead(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	app := caasmocks.NewMockApplication(ctrl)
	facade := mocks.NewMockCAASProvisionerFacade(ctrl)
	unitFacade := mocks.NewMockCAASUnitProvisionerFacade(ctrl)
	broker := mocks.NewMockCAASBroker(ctrl)

	clk := testclock.NewDilatedWallClock(coretesting.ShortWait)

	appTag := names.NewApplicationTag("test").String()
	updateUnitsArgs := params.UpdateApplicationUnits{
		ApplicationTag: appTag,
	}
	gomock.InOrder(
		app.EXPECT().Delete().Return(nil),
		app.EXPECT().Exists().Return(caas.DeploymentState{}, nil),
		app.EXPECT().Service().Return(nil, errors.NotFound),
		app.EXPECT().Units().Return(nil, nil),
		facade.EXPECT().UpdateUnits(gomock.Any(), updateUnitsArgs).Return(nil, nil),
		facade.EXPECT().ClearApplicationResources(gomock.Any(), "test").Return(nil),
	)

	err := caasapplicationprovisioner.AppOps.AppDead(c.Context(), "test", app, broker, facade, unitFacade, clk, s.logger)
	c.Assert(err, tc.ErrorIsNil)
}

func intPtr(i int) *int {
	return &i
}
