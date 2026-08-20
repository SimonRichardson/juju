// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelgeneration

import (
	"context"

	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/generation"
)

// GenerationService describes generation lifecycle operations used by the
// facade.
type GenerationService interface {
	AddBranch(ctx context.Context, name, createdBy string) (uint64, error)
	GetBranchByName(ctx context.Context, name string) (generation.Generation, error)
	ListBranches(ctx context.Context) ([]generation.Generation, error)
	TrackBranch(ctx context.Context, branchName string, unitUUIDs []coreunit.UUID) error
	UntrackBranch(ctx context.Context, branchName string, unitUUIDs []coreunit.UUID) error
	GetTrackedUnits(ctx context.Context, branchName string) ([]coreunit.Name, error)
	CommitBranch(ctx context.Context, branchName, committedBy string) (uint64, error)
	AbortBranch(ctx context.Context, branchName, abortedBy string) error
	ListCommits(ctx context.Context) ([]generation.Commit, error)
	GetCommit(ctx context.Context, generationID uint64) (generation.Commit, error)
}

// ApplicationService resolves application and unit identities for tracking.
type ApplicationService interface {
	GetUnitNamesForApplication(ctx context.Context, appName string) ([]coreunit.Name, error)
	GetUnitUUID(ctx context.Context, unitName coreunit.Name) (coreunit.UUID, error)
}
