// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"github.com/juju/juju/core/database"
	"github.com/juju/juju/domain"
)

// State describes the access mechanism for interacting with the generation
// state in the context of the model database.
type State struct {
	*domain.StateBase
}

// NewState constructs a new state for interacting with the underlying
// generation state of a model.
func NewState(factory database.TxnRunnerFactory) *State {
	return &State{
		StateBase: domain.NewStateBase(factory),
	}
}
