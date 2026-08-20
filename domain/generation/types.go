// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package generation

import (
	"time"

	coreapplication "github.com/juju/juju/core/application"
	internaluuid "github.com/juju/juju/internal/uuid"
)

// State describes the lifecycle state of a generation (branch).
type State string

const (
	// StateInFlight indicates the branch is active and has not yet been
	// committed or aborted.
	StateInFlight State = "in-flight"

	// StateCommitted indicates the branch has been committed to the model.
	StateCommitted State = "committed"

	// StateAborted indicates the branch has been aborted and its changes
	// discarded.
	StateAborted State = "aborted"
)

// Generation describes a model branch.
type Generation struct {
	// UUID is the unique identifier of the branch.
	UUID internaluuid.UUID

	// GenerationID is the monotonic, human-facing sequence number of the
	// branch. It is shared with the commit history.
	GenerationID uint64

	// Name is the branch name, unique amongst in-flight branches.
	Name string

	// State is the lifecycle state of the branch.
	State State

	// CreatedBy is the user who created the branch.
	CreatedBy string

	// CreatedAt is the time the branch was created.
	CreatedAt time.Time

	// CompletedBy is the user who committed or aborted the branch. It is
	// empty while the branch is in flight.
	CompletedBy string

	// CompletedAt is the time the branch was committed or aborted. It is
	// zero while the branch is in flight.
	CompletedAt time.Time
}

// ConfigChange is a single application config delta.
type ConfigChange struct {
	// Key is the config option key.
	Key string

	// Value is the new value. A nil value represents an explicit unset
	// (tombstone): revert to the charm default, overriding any user-set value
	// on the committed state.
	Value any
}

// ApplicationConfigChange holds the config changes made to a single
// application under a branch, frozen at commit time.
type ApplicationConfigChange struct {
	// ApplicationUUID is the UUID of the application.
	ApplicationUUID coreapplication.UUID

	// ApplicationName is the name of the application at commit time. It may
	// be empty if the application has since been removed.
	ApplicationName string

	// Config holds the config deltas, in no particular order.
	Config []ConfigChange
}

// Commit describes a committed generation in the model history.
type Commit struct {
	// UUID is the unique identifier of the commit.
	UUID internaluuid.UUID

	// GenerationID is the monotonic sequence number of the commit.
	GenerationID uint64

	// Name is the branch name that was committed.
	Name string

	// CreatedBy is the user who created the branch.
	CreatedBy string

	// CommittedBy is the user who committed the branch.
	CommittedBy string

	// CommittedAt is the time the branch was committed.
	CommittedAt time.Time

	// Applications holds the per-application config changes committed.
	Applications []ApplicationConfigChange
}
