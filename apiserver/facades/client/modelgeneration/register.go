// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelgeneration

import (
	"context"
	"reflect"

	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/apiserver/facade"
)

// Register exposes the model generation facade.
func Register(registry facade.FacadeRegistry) {
	registry.MustRegister("ModelGeneration", 4, func(_ context.Context, ctx facade.ModelContext) (facade.Facade, error) {
		return makeFacade(ctx)
	}, reflect.TypeFor[*API]())
}

func makeFacade(ctx facade.ModelContext) (*API, error) {
	auth := ctx.Auth()
	if !auth.AuthClient() {
		return nil, apiservererrors.ErrPerm
	}

	services := ctx.DomainServices()
	return NewAPI(
		auth,
		ctx.ControllerUUID(),
		ctx.ModelUUID(),
		services.Generation(),
		services.Application(),
	)
}
