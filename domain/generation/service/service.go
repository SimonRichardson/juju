// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"

	"github.com/juju/juju/domain/generation/internal"
)

// State describes the methods that a state implementation must provide to
// manage model generations (branches). It deals exclusively in scalar types
// and the internal DTOs.
type State interface {
	// AddBranch creates a new in-flight branch with the given name and returns
	// its monotonic generation id.
	AddBranch(ctx context.Context, genUUID, name, createdBy string) (uint64, error)

	// GetBranchByName returns the in-flight branch with the given name.
	GetBranchByName(ctx context.Context, name string) (internal.Generation, error)

	// ListBranches returns all in-flight branches.
	ListBranches(ctx context.Context) ([]internal.Generation, error)

	// TrackUnits records that the given units are tracking the branch
	// identified by generationUUID.
	TrackUnits(ctx context.Context, generationUUID string, unitUUIDs []string) error

	// UntrackUnits removes the given units from tracking the branch identified
	// by generationUUID.
	UntrackUnits(ctx context.Context, generationUUID string, unitUUIDs []string) error

	// GetTrackedUnitNames returns the names of the units tracking the branch
	// identified by generationUUID.
	GetTrackedUnitNames(ctx context.Context, generationUUID string) ([]string, error)

	// HasTrackedUnits reports whether any units are tracking the branch
	// identified by generationUUID.
	HasTrackedUnits(ctx context.Context, generationUUID string) (bool, error)

	// GetBranchForUnit returns the in-flight branch that the given unit is
	// tracking.
	GetBranchForUnit(ctx context.Context, unitUUID string) (internal.Generation, error)

	// Commit applies the branch's changes, archives the history and marks the
	// branch committed. It returns the branch's generation id.
	Commit(ctx context.Context, generationUUID, commitUUID, committedBy string) (uint64, error)

	// Abort marks the branch aborted and discards its changes.
	Abort(ctx context.Context, generationUUID, abortedBy string) error

	// ListCommits returns the committed generation history, oldest first.
	ListCommits(ctx context.Context) ([]internal.Commit, error)

	// GetCommit returns the commit identified by generation id.
	GetCommit(ctx context.Context, generationID uint64) (internal.Commit, error)
}

// Service provides the API for managing model generations (branches).
type Service struct {
	st State
}

// NewService returns a new service wrapping the input state.
func NewService(st State) *Service {
	return &Service{
		st: st,
	}
}
