// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package controller

import (
	"github.com/juju/juju/core/database"
	"github.com/juju/juju/domain"
)

// State represents the access method for interacting the underlying model
// during model migration.
type State struct {
	*domain.StateBase
}

// New creates a new [State]
func New(modelFactory database.TxnRunnerFactory) *State {
	return &State{
		StateBase: domain.NewStateBase(modelFactory),
	}
}
