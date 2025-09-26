// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package deployer

import (
	"context"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/apiserver/common"
	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/apiserver/internal"
	"github.com/juju/juju/controller"
	"github.com/juju/juju/core/leadership"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/core/machine"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/objectstore"
	corestatus "github.com/juju/juju/core/status"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/core/watcher"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	"github.com/juju/juju/rpc/params"
)

// TODO (manadart 2020-10-21): Remove the ModelUUID method
// from the next version of this facade.

// AgentPasswordService defines the methods required to set an agent password
// hash.
type AgentPasswordService interface {
	// SetUnitPassword sets the password hash for the given unit.
	SetUnitPassword(context.Context, coreunit.Name, string) error
	// SetMachinePassword sets the password hash for the given machine.
	SetMachinePassword(context.Context, machine.Name, string) error
	// SetControllerNodePassword sets the password hash for the given
	// controller node.
	SetControllerNodePassword(context.Context, string, string) error
	// IsMachineController returns whether the machine is a controller machine.
	// It returns a NotFound if the given machine doesn't exist.
	IsMachineController(ctx context.Context, machineName machine.Name) (bool, error)

	// SetModelPassword sets the password for the model overriding any previously
	// set value.
	SetModelPassword(ctx context.Context, password string) error
}

// ControllerConfigGetter is the interface that the facade needs to get controller config.
type ControllerConfigGetter interface {
	ControllerConfig(context.Context) (controller.Config, error)
}

// ApplicationService removes a unit from the dqlite database.
type ApplicationService interface {
	// GetUnitLife looks up the life of the specified unit.
	//
	// The following errors may be returned:
	// - [applicationerrors.UnitNotFound] if the unit doesn't exist.
	GetUnitLife(context.Context, coreunit.Name) (life.Value, error)

	// WatchUnitAddRemoveOnMachine returns a watcher that observes changes to
	// the units on a specified machine, emitting the names of the units. That
	// is, we emit unit names only when a unit is created or deleted on the
	// specified machine.
	//
	// The following errors may be returned:
	// - [applicationerrors.MachineNotFound] if the machine does not exist
	WatchUnitAddRemoveOnMachine(context.Context, machine.Name) (watcher.StringsWatcher, error)

	// GetUnitUUID returns the UUID for the named unit.
	//
	// The following errors may be returned:
	// - [applicationerrors.UnitNotFound] if the unit doesn't exist.
	GetUnitUUID(context.Context, coreunit.Name) (coreunit.UUID, error)

	// GetUnitNamesOnMachine returns a slice of the unit names on the given machine.
	// The following errors may be returned:
	// - [applicationerrors.MachineNotFound] if the machine does not exist
	GetUnitNamesOnMachine(context.Context, machine.Name) ([]coreunit.Name, error)
}

// ControllerNodeService defines the methods on the controller node service
// that are needed by APIAddresser used by the deployer API.
type ControllerNodeService interface {
	// GetAPIAddressesByControllerIDForAgents returns a map of controller IDs to
	// their API addresses that are available for agents. The map is keyed by
	// controller ID, and the values are slices of strings representing the API
	// addresses for each controller node.
	GetAPIAddressesByControllerIDForAgents(ctx context.Context) (map[string][]string, error)
	// GetAllAPIAddressesForAgents returns a string of api
	// addresses available for agents ordered to prefer local-cloud scoped
	// addresses and IPv4 over IPv6 for each machine.
	GetAllAPIAddressesForAgents(ctx context.Context) ([]string, error)
	// GetAPIHostPortsForAgents returns API HostPorts that are available for
	// agents. HostPorts are grouped by controller node, though each specific
	// controller is not identified.
	GetAPIHostPortsForAgents(ctx context.Context) ([]network.HostPorts, error)
	// WatchControllerAPIAddresses returns a watcher that observes changes to the
	// controller ip addresses.
	WatchControllerAPIAddresses(context.Context) (watcher.NotifyWatcher, error)
}

type StatusService interface {
	// GetUnitWorkloadStatus returns the workload status of the specified unit, returning an
	// error satisfying [applicationerrors.UnitNotFound] if the unit doesn't exist.
	GetUnitWorkloadStatus(context.Context, coreunit.Name) (corestatus.StatusInfo, error)

	// SetUnitWorkloadStatus sets the workload status of the specified unit, returning an
	// error satisfying [applicationerrors.UnitNotFound] if the unit doesn't exist.
	SetUnitWorkloadStatus(context.Context, coreunit.Name, corestatus.StatusInfo) error
}

// RemovalService defines operations for removing juju entities.
type RemovalService interface {
	// MarkUnitAsDead marks the unit as dead. It will not remove the unit as
	// that is a separate operation. This will advance the unit's life to dead
	// and will not allow it to be transitioned back to alive.
	MarkUnitAsDead(context.Context, coreunit.UUID) error
}

// DeployerAPI provides access to the Deployer API facade.
type DeployerAPI struct {
	*common.PasswordChanger
	*common.APIAddresser
	unitStatusSetter *common.UnitStatusSetter

	getAuth common.GetAuthFunc

	controllerConfigGetter ControllerConfigGetter
	applicationService     ApplicationService
	removalService         RemovalService
	leadershipRevoker      leadership.Revoker

	store           objectstore.ObjectStore
	authorizer      facade.Authorizer
	getCanWatch     common.GetAuthFunc
	watcherRegistry facade.WatcherRegistry
}

// NewDeployerAPI creates a new server-side DeployerAPI facade.
func NewDeployerAPI(
	agentPasswordService AgentPasswordService,
	controllerConfigGetter ControllerConfigGetter,
	applicationService ApplicationService,
	controllerNodeService ControllerNodeService,
	statusService StatusService,
	removalService RemovalService,
	authorizer facade.Authorizer,
	store objectstore.ObjectStore,
	leadershipRevoker leadership.Revoker,
	watcherRegistry facade.WatcherRegistry,
	clock clock.Clock,
) (*DeployerAPI, error) {
	getAuthFunc := func(ctx context.Context) (common.AuthFunc, error) {
		// Get all units of the machine and cache them.
		thisMachineName := machine.Name(authorizer.GetAuthTag().Id())
		unitNames, err := applicationService.GetUnitNamesOnMachine(ctx, thisMachineName)
		if err != nil {
			return nil, err
		}
		unitNameIndex := make(map[string]struct{}, len(unitNames))
		for _, unitName := range unitNames {
			unitNameIndex[unitName.String()] = struct{}{}
		}
		// Then we just check if the unit is already known.
		return func(tag names.Tag) bool {
			if _, ok := unitNameIndex[tag.Id()]; ok {
				return true
			}
			return false
		}, nil
	}
	getCanWatch := func(context.Context) (common.AuthFunc, error) {
		return authorizer.AuthOwner, nil
	}

	return &DeployerAPI{
		PasswordChanger:        common.NewPasswordChanger(agentPasswordService, getAuthFunc),
		APIAddresser:           common.NewAPIAddresser(controllerNodeService, watcherRegistry),
		unitStatusSetter:       common.NewUnitStatusSetter(statusService, clock, getAuthFunc),
		controllerConfigGetter: controllerConfigGetter,
		applicationService:     applicationService,
		removalService:         removalService,
		leadershipRevoker:      leadershipRevoker,
		getAuth:                getAuthFunc,
		store:                  store,
		authorizer:             authorizer,
		getCanWatch:            getCanWatch,
		watcherRegistry:        watcherRegistry,
	}, nil
}

func (d *DeployerAPI) WatchUnits(ctx context.Context, args params.Entities) (params.StringsWatchResults, error) {
	result := params.StringsWatchResults{
		Results: make([]params.StringsWatchResult, len(args.Entities)),
	}
	if len(args.Entities) == 0 {
		return result, nil
	}

	canWatch, err := d.getCanWatch(ctx)
	if err != nil {
		return params.StringsWatchResults{}, errors.Trace(err)
	}
	for i, entity := range args.Entities {
		machineTag, err := names.ParseMachineTag(entity.Tag)
		if err != nil {
			result.Results[i].Error = apiservererrors.ServerError(apiservererrors.ErrPerm)
			continue
		}
		entityResult, err := d.watchOneUnit(ctx, canWatch, machineTag)
		result.Results[i] = entityResult
		result.Results[i].Error = apiservererrors.ServerError(err)
	}
	return result, nil
}

func (s *DeployerAPI) watchOneUnit(ctx context.Context, canWatch common.AuthFunc, machineTag names.MachineTag) (params.StringsWatchResult, error) {
	if !canWatch(machineTag) {
		return params.StringsWatchResult{}, apiservererrors.ErrPerm
	}

	machineName := machine.Name(machineTag.Id())
	watcher, err := s.applicationService.WatchUnitAddRemoveOnMachine(ctx, machineName)
	if errors.Is(err, applicationerrors.MachineNotFound) {
		return params.StringsWatchResult{}, errors.NotFoundf("machine %q", machineName)
	} else if err != nil {
		return params.StringsWatchResult{}, errors.Trace(err)
	}

	id, initial, err := internal.EnsureRegisterWatcher(ctx, s.watcherRegistry, watcher)
	if err != nil {
		return params.StringsWatchResult{}, errors.Trace(err)
	}
	return params.StringsWatchResult{
		StringsWatcherId: id,
		Changes:          initial,
	}, nil
}

// ConnectionInfo returns all the address information that the
// deployer task needs in one call.
func (d *DeployerAPI) ConnectionInfo(ctx context.Context) (result params.DeployerConnectionValues, err error) {
	apiAddrs, err := d.APIAddresses(ctx)
	if err != nil {
		return result, errors.Trace(err)
	}
	result = params.DeployerConnectionValues{
		APIAddresses: apiAddrs.Result,
	}
	return result, nil
}

// SetStatus sets the status of the specified entities.
func (d *DeployerAPI) SetStatus(ctx context.Context, args params.SetStatus) (params.ErrorResults, error) {
	return d.unitStatusSetter.SetStatus(ctx, args)
}

// ModelUUID returns the model UUID that this facade is deploying into.
// It is implemented here directly as a result of removing it from
// embedded APIAddresser *without* bumping the facade version.
// It should be blanked when this facade version is next incremented.
func (d *DeployerAPI) ModelUUID() params.StringResult {
	return params.StringResult{Result: ""}
}

// Life returns the life of the specified units.
func (d *DeployerAPI) Life(ctx context.Context, args params.Entities) (params.LifeResults, error) {
	result := params.LifeResults{
		Results: make([]params.LifeResult, len(args.Entities)),
	}
	if len(args.Entities) == 0 {
		return result, nil
	}
	canRead, err := d.getAuth(ctx)
	if err != nil {
		return params.LifeResults{}, errors.Trace(err)
	}

	for i, entity := range args.Entities {
		tag, err := names.ParseTag(entity.Tag)
		if err != nil {
			result.Results[i].Error = apiservererrors.ServerError(apiservererrors.ErrPerm)
			continue
		}

		if !canRead(tag) {
			result.Results[i].Error = apiservererrors.ServerError(apiservererrors.ErrPerm)
			continue
		}
		unitName, err := coreunit.NewName(tag.Id())
		if err != nil {
			result.Results[i].Error = apiservererrors.ServerError(err)
			continue
		}
		lifeValue, err := d.applicationService.GetUnitLife(ctx, unitName)
		if errors.Is(err, applicationerrors.UnitNotFound) {
			err = errors.NotFoundf("unit %s", unitName)
		}
		result.Results[i].Life = lifeValue
		result.Results[i].Error = apiservererrors.ServerError(err)
	}
	return result, nil
}

// Remove removes every given unit from the application domain, this is ensured
// by the removal domain.
func (d *DeployerAPI) Remove(ctx context.Context, args params.Entities) (params.ErrorResults, error) {
	result := params.ErrorResults{
		Results: make([]params.ErrorResult, len(args.Entities)),
	}
	if len(args.Entities) == 0 {
		return result, nil
	}
	canWrite, err := d.getAuth(ctx)
	if err != nil {
		return params.ErrorResults{}, errors.Trace(err)
	}

	for i, entity := range args.Entities {
		tag, err := names.ParseUnitTag(entity.Tag)
		if err != nil {
			result.Results[i].Error = apiservererrors.ServerError(apiservererrors.ErrPerm)
			continue
		}
		if !canWrite(tag) {
			result.Results[i].Error = apiservererrors.ServerError(apiservererrors.ErrPerm)
			continue
		}

		unitUUID, err := d.applicationService.GetUnitUUID(ctx, coreunit.Name(tag.Id()))
		if errors.Is(err, applicationerrors.UnitNotFound) {
			continue
		} else if err != nil {
			result.Results[i].Error = apiservererrors.ServerError(err)
			continue
		}

		// Mark the unit as dead. This is because we know that the deployer
		// ensures that the unit is not alive before calling unit.Remove, so
		// we can just mark it as dead here.
		// The deployer also guards against the uniter existing and attempting
		// to call remove on the unit, preventing the race between the deployer
		// and the uniter calling ensure dead.
		err = d.removalService.MarkUnitAsDead(ctx, unitUUID)
		if errors.Is(err, applicationerrors.UnitNotFound) {
			continue
		} else if err != nil {
			result.Results[i].Error = apiservererrors.ServerError(err)
			continue
		}
	}
	return result, nil
}
