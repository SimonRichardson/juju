// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package machineundertaker

import (
	"context"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/core/machine"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state/watcher"
)

// API implements the API facade used by the machine undertaker.
type API struct {
	backend        Backend
	resources      facade.Resources
	canManageModel func(modelUUID string) bool

	machineRemover machineRemover
}

type machineRemover interface {
	DeleteMachine(context.Context, machine.Name) error
}

// NewAPI implements the API used by the machine undertaker worker to
// find out what provider-level resources need to be cleaned up when a
// machine goes away.
func NewAPI(
	modelUUID model.UUID,
	backend Backend,
	resources facade.Resources,
	authorizer facade.Authorizer,
	machineRemover machineRemover,
) (*API, error) {
	if !authorizer.AuthController() {
		return nil, errors.Trace(apiservererrors.ErrPerm)
	}

	api := &API{
		backend:   backend,
		resources: resources,
		canManageModel: func(requestedModelUUID string) bool {
			return modelUUID.String() == requestedModelUUID
		},
		machineRemover: machineRemover,
	}
	return api, nil
}

// AllMachineRemovals returns tags for all of the machines that have
// been marked for removal in the requested model.
func (m *API) AllMachineRemovals(ctx context.Context, models params.Entities) params.EntitiesResults {
	results := make([]params.EntitiesResult, len(models.Entities))
	for i, entity := range models.Entities {
		entities, err := m.allRemovalsForTag(entity.Tag)
		results[i].Entities = entities
		results[i].Error = apiservererrors.ServerError(err)
	}
	return params.EntitiesResults{Results: results}
}

func (m *API) allRemovalsForTag(tag string) ([]params.Entity, error) {
	err := m.checkModelAuthorization(tag)
	if err != nil {
		return nil, errors.Trace(err)
	}
	machineIds, err := m.backend.AllMachineRemovals()
	if err != nil {
		return nil, errors.Trace(err)
	}
	var entities []params.Entity
	for _, id := range machineIds {
		entities = append(entities, params.Entity{
			Tag: names.NewMachineTag(id).String(),
		})
	}
	return entities, nil
}

// GetMachineProviderInterfaceInfo returns the provider details for
// all network interfaces attached to the machines requested.
func (m *API) GetMachineProviderInterfaceInfo(ctx context.Context, machines params.Entities) params.ProviderInterfaceInfoResults {
	results := make([]params.ProviderInterfaceInfoResult, len(machines.Entities))
	for i, entity := range machines.Entities {
		results[i].MachineTag = entity.Tag

		interfaces, err := m.getInterfaceInfoForOneMachine(entity.Tag)
		if err != nil {
			results[i].Error = apiservererrors.ServerError(err)
			continue
		}

		infos := make([]params.ProviderInterfaceInfo, len(interfaces))
		for i, info := range interfaces {
			infos[i].InterfaceName = info.InterfaceName
			infos[i].MACAddress = info.HardwareAddress
			infos[i].ProviderId = string(info.ProviderId)
		}

		results[i].Interfaces = infos
	}
	return params.ProviderInterfaceInfoResults{results}
}

func (m *API) getInterfaceInfoForOneMachine(machineTag string) ([]network.ProviderInterfaceInfo, error) {
	tag, err := names.ParseMachineTag(machineTag)
	if err != nil {
		return nil, errors.Trace(err)
	}
	machine, err := m.backend.Machine(tag.Id())
	if err != nil {
		return nil, errors.Trace(err)
	}
	interfaces, err := machine.AllProviderInterfaceInfos()
	if err != nil {
		return nil, errors.Trace(err)
	}
	return interfaces, nil
}

// CompleteMachineRemovals removes the specified machines from the
// model database. It should only be called once any provider-level
// cleanup has been done for those machines.
func (m *API) CompleteMachineRemovals(ctx context.Context, machines params.Entities) error {
	machineIDs, err := collectMachineIDs(machines)
	if err != nil {
		return errors.Trace(err)
	}
	err = m.backend.CompleteMachineRemovals(machineIDs...)
	if err != nil {
		return errors.Trace(err)
	}

	// Remove the machines from dqlite.
	for _, id := range machineIDs {
		if err := m.machineRemover.DeleteMachine(ctx, machine.Name(id)); err != nil {
			return errors.Annotatef(err, "removing machine %q", id)
		}
	}
	return nil
}

// WatchMachineRemovals returns a watcher that will signal each time a
// machine is marked for removal.
func (m *API) WatchMachineRemovals(ctx context.Context, models params.Entities) params.NotifyWatchResults {
	results := make([]params.NotifyWatchResult, len(models.Entities))
	for i, entity := range models.Entities {
		id, err := m.watchRemovalsForTag(entity.Tag)
		results[i].NotifyWatcherId = id
		results[i].Error = apiservererrors.ServerError(err)
	}
	return params.NotifyWatchResults{Results: results}
}

func (m *API) watchRemovalsForTag(tag string) (string, error) {
	err := m.checkModelAuthorization(tag)
	if err != nil {
		return "", errors.Trace(err)
	}
	watch := m.backend.WatchMachineRemovals()
	if _, ok := <-watch.Changes(); ok {
		return m.resources.Register(watch), nil
	} else {
		return "", watcher.EnsureErr(watch)
	}
}

func (m *API) checkModelAuthorization(tag string) error {
	modelTag, err := names.ParseModelTag(tag)
	if err != nil {
		return errors.Trace(err)
	}
	if !m.canManageModel(modelTag.Id()) {
		return errors.Trace(apiservererrors.ErrPerm)
	}
	return nil
}

func collectMachineIDs(args params.Entities) ([]string, error) {
	machineIDs := make([]string, len(args.Entities))
	for i := range args.Entities {
		tag, err := names.ParseMachineTag(args.Entities[i].Tag)
		if err != nil {
			return nil, errors.Trace(err)
		}
		machineIDs[i] = tag.Id()
	}
	return machineIDs, nil
}
