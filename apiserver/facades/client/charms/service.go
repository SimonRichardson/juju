// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charms

import (
	"context"

	coreapplication "github.com/juju/juju/core/application"
	corecharm "github.com/juju/juju/core/charm"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/domain/application/charm"
	"github.com/juju/juju/environs/config"
)

// ModelConfigService provides access to the model configuration.
type ModelConfigService interface {
	// ModelConfig returns the current config for the model.
	ModelConfig(context.Context) (*config.Config, error)
}

// ApplicationService provides access to application related operations, this
// includes charms, units and resources.
type ApplicationService interface {
	// AddCharm persists the charm metadata, actions, config and manifest to
	// state.
	// If there are any non-blocking issues with the charm metadata, actions,
	// config or manifest, a set of warnings will be returned.
	AddCharm(context.Context, charm.AddCharmArgs) (corecharm.ID, []string, error)

	// ListCharmLocators returns a list of charms with the specified
	// locator by the name. If no names are provided, all charms are returned.
	ListCharmLocators(ctx context.Context, names ...string) ([]charm.CharmLocator, error)

	// GetApplicationUUIDByName returns an application ID by application name. It
	// returns an error if the application can not be found by the name.
	//
	// Returns [applicationerrors.ApplicationNameNotValid] if the name is not valid,
	// and [applicationerrors.ApplicationNotFound] if the application is not found.
	GetApplicationUUIDByName(ctx context.Context, name string) (coreapplication.UUID, error)

	// IsSubordinateApplication returns true if the application is a subordinate
	// application.
	// The following errors may be returned:
	// - [appliationerrors.ApplicationNotFound] if the application does not exist
	IsSubordinateApplication(context.Context, coreapplication.UUID) (bool, error)

	// GetApplicationConstraints returns the application constraints for the
	// specified application ID.
	// Empty constraints are returned if no constraints exist for the given
	// application ID.
	// If no application is found, an error satisfying
	// [applicationerrors.ApplicationNotFound] is returned.
	GetApplicationConstraints(context.Context, coreapplication.UUID) (constraints.Value, error)
}
