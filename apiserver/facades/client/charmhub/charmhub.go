// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charmhub

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/charmhub"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/environs/context"
)

var logger = loggo.GetLogger("juju.apiserver.charmhub")

type Backend interface {
}

type Model interface {
	ModelConfigValues() (config.ConfigValues, error)
}

// API provides the charmhub API facade for version 1.
type CharmHubAPI struct {
	auth    facade.Authorizer
	context context.ProviderCallContext
}

func NewFacade(ctx facade.Context) (*CharmHubAPI, error) {
	auth := ctx.Auth()
	return NewCharmHubAPI(auth)
}

func NewCharmHubAPI(authorizer facade.Authorizer) (*CharmHubAPI, error) {
	if !authorizer.AuthClient() {
		return nil, common.ErrPerm
	}
	return &CharmHubAPI{auth: authorizer}, nil
}

func (api *CharmHubAPI) Info(arg params.EntityString) (params.CharmHubCharmInfoResult, error) {
	name := arg.Value
	if name == "" {
		return params.CharmHubCharmInfoResult{}, errors.BadRequestf("arg value is empty")
	}
	logger.Criticalf("Info(%s)", name)
	chClient := charmhub.New(charmhub.Params{})
	info, err := chClient.Info(name)
	return params.CharmHubCharmInfoResult{Result: info}, err
}
