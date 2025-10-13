// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package firewaller

import (
	"context"

	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"github.com/juju/worker/v4"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/controller/crossmodelrelations"
	"github.com/juju/juju/api/controller/firewaller"
	"github.com/juju/juju/internal/worker/apicaller"
)

// NewFirewallerFacade creates a firewaller API facade.
func NewFirewallerFacade(apiCaller base.APICaller) (FirewallerAPI, error) {
	facade, err := firewaller.NewClient(apiCaller)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &firewallerShim{Client: facade}, nil
}

type firewallerShim struct {
	*firewaller.Client
}

func (s *firewallerShim) Machine(ctx context.Context, tag names.MachineTag) (Machine, error) {
	return s.Client.Machine(ctx, tag)
}

func (s *firewallerShim) Unit(ctx context.Context, tag names.UnitTag) (Unit, error) {
	u, err := s.Client.Unit(ctx, tag)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &unitShim{Unit: u}, nil
}

type unitShim struct {
	*firewaller.Unit
}

func (s *unitShim) Application() (Application, error) {
	return s.Unit.Application()
}

// NewWorker creates a firewaller worker.
func NewWorker(cfg Config) (worker.Worker, error) {
	w, err := NewFirewaller(cfg)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return w, nil
}

// crossmodelFirewallerFacadeFunc returns a function that
// can be used to construct instances which manage remote relation
// firewall changes for a given model.

// For now we use a facade, but in future this may evolve into a REST caller.
func crossmodelFirewallerFacadeFunc(
	connectionFunc apicaller.NewExternalControllerConnectionFunc,
) newCrossModelFacadeFunc {
	return func(ctx context.Context, apiInfo *api.Info) (CrossModelFirewallerFacadeCloser, error) {
		apiInfo.Tag = names.NewUserTag(api.AnonymousUsername)
		conn, err := connectionFunc(ctx, apiInfo)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return crossmodelrelations.NewClient(conn), nil
	}
}
