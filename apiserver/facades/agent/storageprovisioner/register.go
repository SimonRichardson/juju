// Copyright 2022 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package storageprovisioner

import (
	"context"
	"reflect"

	"github.com/juju/juju/apiserver/facade"
)

// Register is called to expose a package of facades onto a given registry.
func Register(registry facade.FacadeRegistry) {
	registry.MustRegister(
		"StorageProvisioner", 4,
		func(stdCtx context.Context, ctx facade.ModelContext) (facade.Facade, error) {
			return newFacadeV4(stdCtx, ctx)
		},
		reflect.TypeOf((*StorageProvisionerAPIv4)(nil)),
	)

	registry.MustRegister(
		"VolumeAttachmentsWatcher", 2,
		newMachineStorageIdsWatcherFromContext, reflect.TypeOf((*machineStorageIdsWatcher)(nil)),
	)
	registry.MustRegister(
		"VolumeAttachmentPlansWatcher", 1,
		newMachineStorageIdsWatcherFromContext, reflect.TypeOf((*machineStorageIdsWatcher)(nil)),
	)
	registry.MustRegister(
		"FilesystemAttachmentsWatcher", 2,
		newMachineStorageIdsWatcherFromContext, reflect.TypeOf((*machineStorageIdsWatcher)(nil)),
	)
}

// newFacadeV4 provides the signature required for facade registration.
func newFacadeV4(stdCtx context.Context, ctx facade.ModelContext) (*StorageProvisionerAPIv4, error) {
	domainServices := ctx.DomainServices()
	storageService := domainServices.Storage()

	return NewStorageProvisionerAPIv4(
		stdCtx,
		ctx.WatcherRegistry(),
		ctx.Clock(),
		domainServices.BlockDevice(),
		domainServices.Machine(),
		domainServices.Application(),
		domainServices.Removal(),
		ctx.Auth(),
		storageService,
		domainServices.Status(),
		domainServices.StorageProvisioning(),
		ctx.Logger().Child("storageprovisioner"),
		ctx.ModelUUID(),
		ctx.ControllerUUID(),
	)
}
