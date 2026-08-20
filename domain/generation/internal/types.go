// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package internal

import "time"

// Generation is the scalar representation of a branch, shared between the
// state and service layers.
type Generation struct {
	UUID         string
	GenerationID uint64
	Name         string
	State        string
	CreatedBy    string
	CreatedAt    time.Time
	CompletedBy  string
	CompletedAt  time.Time
}

// ConfigChange is a single application config delta.
type ConfigChange struct {
	Key   string
	Value any
}

// ApplicationConfigChange holds the config changes made to a single
// application under a branch.
type ApplicationConfigChange struct {
	ApplicationUUID string
	ApplicationName string
	Config          []ConfigChange
}

// Commit is the scalar representation of a committed generation.
type Commit struct {
	UUID         string
	GenerationID uint64
	Name         string
	CreatedBy    string
	CommittedBy  string
	CommittedAt  time.Time
	Applications []ApplicationConfigChange
}
