// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"

	coremodel "github.com/juju/juju/core/model"
	corerelation "github.com/juju/juju/core/relation"
	coresecrets "github.com/juju/juju/core/secrets"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/core/watcher/eventsource"
	"github.com/juju/juju/domain/secret"
	"github.com/juju/juju/domain/unitstate"
	"github.com/juju/juju/domain/unitstate/internal"
	"github.com/juju/juju/environs"
)

// State defines an interface for interacting with the underlying state.
type State interface {
	CommitHookState
	UnitStateState
	UnitSnapshotState
}

// WatcherFactory creates watchers over model change streams.
type WatcherFactory interface {
	// NewNotifyWatcher returns a watcher that emits when one of the supplied
	// filters matches a model change.
	NewNotifyWatcher(
		ctx context.Context,
		summary string,
		filter eventsource.FilterOption,
		filters ...eventsource.FilterOption,
	) (watcher.NotifyWatcher, error)
}

// CommitHookState defines a persistence layer interface for commit hook changes.
type CommitHookState interface {
	// CommitHookChanges persists a set of changes after a hook successfully
	// completes and executes them in a single transaction.
	CommitHookChanges(ctx context.Context, arg internal.CommitHookChangesArg) error

	// GetPeerRelationUUIDByEndpointIdentifiers gets the UUID of a peer
	// relation specified by a single endpoint identifier.
	//
	// The following error types can be expected to be returned:
	//   - [relationerrors.RelationNotFound] is returned if endpoint cannot be
	//     found.
	GetPeerRelationUUIDByEndpointIdentifiers(
		ctx context.Context,
		endpoint corerelation.EndpointIdentifier,
	) (corerelation.UUID, error)

	// GetRegularRelationUUIDByEndpointIdentifiers gets the UUID of a regular
	// relation specified by two endpoint identifiers.
	//
	// The following error types can be expected to be returned:
	//   - [relationerrors.RelationNotFound] is returned if endpoints cannot be
	//     found.
	GetRegularRelationUUIDByEndpointIdentifiers(
		ctx context.Context,
		endpoint1, endpoint2 corerelation.EndpointIdentifier,
	) (corerelation.UUID, error)

	// GetCommitHookUnitInfo returns the unit UUID and machine UUID if assigned,
	// returning an error satisfying
	// [applicationerrors.UnitNotFound] if the unit doesn't exist.
	GetCommitHookUnitInfo(ctx context.Context, unitName string) (internal.CommitHookUnitInfo, error)

	// GetModelUUID returns the UUID of the model for the unit state domain.
	GetModelUUID(ctx context.Context) (string, error)

	// GetSecretRotatePolicy returns the current rotate policy for the
	// secret identified by the given secret ID. If the secret does not
	// exist, an error satisfying [secreterrors.SecretNotFound] is returned.
	GetSecretRotatePolicy(ctx context.Context, secretID string) (coresecrets.RotatePolicy, error)
}

// SecretGrantAuthorizer provides access checks and ownership details for
// persisted secrets. Grants for secrets created in the same hook commit are
// resolved from the incoming create arguments instead.
type SecretGrantAuthorizer interface {
	// CheckSecretManageAccess verifies the unit has RoleManage access on the
	// given secret, including app-owned secrets if the unit is the leader.
	CheckSecretManageAccess(ctx context.Context, uri *coresecrets.URI, unitName coreunit.Name) error

	// GetSecretOwnerKinds returns the owner kind for each of the given
	// secret URIs. Secrets that no longer exist are silently omitted.
	GetSecretOwnerKinds(ctx context.Context, uris []*coresecrets.URI) ([]secret.SecretOwnerInfo, error)
}

// UnitStateState defines a persistence layer interface for retrieving
// and persisting unit agent state.
type UnitStateState interface {
	// GetUnitState returns the full unit agent state.
	// If no unit with the uuid exists, a [unitstateerrors.UnitNotFound] error
	// is returned.
	// If the units state is empty [unitstateerrors.EmptyUnitState] error is
	// returned.
	GetUnitState(context.Context, string) (unitstate.RetrievedUnitState, error)

	// SetUnitState persists the input unit state selectively,
	// based on its populated values.
	SetUnitState(context.Context, unitstate.UnitState) error
}

// UnitSnapshotState defines the persistence needed to create a watcher for a
// unit snapshot.
type UnitSnapshotState interface {
	// GetUnitSnapshot returns the model-database projection for a unit snapshot.
	GetUnitSnapshot(context.Context, coreunit.Name) (unitstate.UnitSnapshot, error)

	// GetUnitSnapshotWatchIdentifiers returns the stable identifiers used to
	// watch all model state represented by a unit snapshot.
	GetUnitSnapshotWatchIdentifiers(context.Context, coreunit.Name) (unitstate.SnapshotWatchIdentifiers, error)
}

// SecretBackendReferenceMutator describes methods for modifying secret
// backend references in the controller database.
type SecretBackendReferenceMutator interface {
	// AddSecretBackendReference adds a reference to the secret backend
	// for the given secret revision. It returns a rollback function which
	// can be used to revert the changes.
	AddSecretBackendReference(
		ctx context.Context, valueRef *coresecrets.ValueRef, modelID coremodel.UUID, revisionID string, secretID string,
	) (func() error, error)

	// UpdateSecretBackendReference updates the reference to the secret
	// backend for the given secret revision. It returns a rollback function
	// which can be used to revert the changes.
	UpdateSecretBackendReference(
		ctx context.Context, valueRef *coresecrets.ValueRef, modelID coremodel.UUID, revisionID string, secretID string,
	) (func() error, error)
}

// ProviderWithNetworking describes the interface needed from providers that
// support networking capabilities.
type ProviderWithNetworking interface {
	environs.Networking
}
