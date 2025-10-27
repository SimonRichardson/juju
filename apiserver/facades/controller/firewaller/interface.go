// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package firewaller

import (
	"context"

	"github.com/juju/collections/set"

	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/core/machine"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/unit"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/domain/application"
	"github.com/juju/juju/rpc/params"
)

// NetworkService is the interface that is used to interact with the
// network spaces/subnets.
type NetworkService interface {
	// GetAllSpaces returns all spaces for the model.
	GetAllSpaces(ctx context.Context) (network.SpaceInfos, error)

	// Watch returns a watcher that observes changes to subnets and their
	// association (fan underlays), filtered based on the provided list of subnets
	// to watch.
	WatchSubnets(ctx context.Context, subnetUUIDsToWatch set.Strings) (watcher.StringsWatcher, error)
}

// MachineService defines the methods that the facade assumes from the Machine
// service.
type MachineService interface {
	// AllMachineNames returns the names of all machines in the model.
	AllMachineNames(context.Context) ([]machine.Name, error)

	// GetMachineUUID returns the UUID of a machine identified by its name.
	GetMachineUUID(ctx context.Context, name machine.Name) (machine.UUID, error)

	// GetInstanceID returns the cloud specific instance id for this machine.
	GetInstanceID(ctx context.Context, mUUID machine.UUID) (instance.Id, error)

	// GetInstanceIDAndName returns the cloud specific instance ID and display
	// name for this machine.
	GetInstanceIDAndName(ctx context.Context, machineUUID machine.UUID) (instance.Id, string, error)

	// GetHardwareCharacteristics returns the hardware characteristics of the
	// specified machine.
	GetHardwareCharacteristics(ctx context.Context, machineUUID machine.UUID) (*instance.HardwareCharacteristics, error)

	// GetSupportedContainersTypes returns the supported container types for the
	// provider.
	GetSupportedContainersTypes(context.Context, machine.UUID) ([]instance.ContainerType, error)

	// IsMachineManuallyProvisioned returns whether the machine is a manual
	// machine.
	IsMachineManuallyProvisioned(ctx context.Context, machineName machine.Name) (bool, error)

	// GetMachineLife returns the lifecycle of the machine.
	GetMachineLife(ctx context.Context, name machine.Name) (life.Value, error)

	// WatchModelMachines watches for additions or updates to non-container
	// machines. It is used by workers that need to factor life value changes,
	// and so does not factor machine removals, which are considered to be
	// after their transition to the dead state.
	// It emits machine names rather than UUIDs.
	WatchModelMachines(ctx context.Context) (watcher.StringsWatcher, error)

	// WatchModelMachineLifeAndStartTimes returns a string watcher that emits machine names
	// for changes to machine life or agent start times.
	WatchModelMachineLifeAndStartTimes(ctx context.Context) (watcher.StringsWatcher, error)
}

// ApplicationService provides access to the application service.
type ApplicationService interface {
	// GetUnitLife looks up the life of the specified unit, returning an error
	// satisfying [applicationerrors.UnitNotFoundError] if the unit is not found.
	GetUnitLife(ctx context.Context, unitName unit.Name) (life.Value, error)

	// GetApplicationLifeByName looks up the life of the specified application, returning
	// an error satisfying [applicationerrors.ApplicationNotFoundError] if the
	// application is not found.
	GetApplicationLifeByName(ctx context.Context, appName string) (life.Value, error)

	// IsApplicationExposed returns whether the provided application is exposed or not.
	//
	// If no application is found, an error satisfying
	// [applicationerrors.ApplicationNotFound] is returned.
	IsApplicationExposed(ctx context.Context, appName string) (bool, error)

	// GetExposedEndpoints returns map where keys are endpoint names (or the ""
	// value which represents all endpoints) and values are ExposedEndpoint
	// instances that specify which sources (spaces or CIDRs) can access the
	// opened ports for each endpoint once the application is exposed.
	//
	// If no application is found, an error satisfying
	// [applicationerrors.ApplicationNotFound] is returned.
	GetExposedEndpoints(ctx context.Context, appName string) (map[string]application.ExposedEndpoint, error)
}

// ControllerConfigAPI provides the subset of common.ControllerConfigAPI
// required by the remote firewaller facade
type ControllerConfigAPI interface {
	// ControllerConfig returns the controller's configuration.
	ControllerConfig(context.Context) (params.ControllerConfigResult, error)

	// ControllerAPIInfoForModels returns the controller api connection details for the specified models.
	ControllerAPIInfoForModels(context.Context, params.Entities) (params.ControllerAPIInfoResults, error)
}

// ModelInfoService provides access to the model services.
type ModelInfoService interface {
	// IsControllerModel returns true if the model is the controller model.
	// The following errors may be returned:
	// - [modelerrors.NotFound] when the model does not exist.
	IsControllerModel(ctx context.Context) (bool, error)
}
