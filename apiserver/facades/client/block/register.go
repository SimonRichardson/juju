// Copyright 2022 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package block

import (
	"context"
	"reflect"

	"github.com/juju/errors"

	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/apiserver/facade"
)

// Register is called to expose a package of facades onto a given registry.
func Register(registry facade.FacadeRegistry) {
	registry.MustRegister("Block", 2, func(stdCtx context.Context, ctx facade.ModelContext) (facade.Facade, error) {
		return NewAPI(ctx)
	}, reflect.TypeOf((*API)(nil)))
}

// NewAPI returns a new block API facade.
func NewAPI(ctx facade.ModelContext) (*API, error) {
	authorizer := ctx.Auth()
	if !authorizer.AuthClient() {
		return nil, apiservererrors.ErrPerm
	}

	st := ctx.State()
	m, err := st.Model()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &API{
		modelTag:   m.ModelTag(),
		service:    ctx.DomainServices().BlockCommand(),
		authorizer: authorizer,
	}, nil
}
