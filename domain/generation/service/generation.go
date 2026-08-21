// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"

	"github.com/juju/collections/transform"

	coreapplication "github.com/juju/juju/core/application"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/generation"
	"github.com/juju/juju/domain/generation/internal"
	"github.com/juju/juju/internal/errors"
	internaluuid "github.com/juju/juju/internal/uuid"
)

// AddBranch creates a new in-flight branch with the given name and returns its
// generation identifier.
func (s *Service) AddBranch(ctx context.Context, name, createdBy string) (uint64, error) {
	if name == "" {
		return 0, errors.Errorf("branch name cannot be empty")
	}

	uuid, err := internaluuid.NewUUID()
	if err != nil {
		return 0, errors.Errorf("generating branch uuid: %w", err)
	}
	return s.st.AddBranch(ctx, uuid.String(), name, createdBy)
}

// GetBranchByName returns the in-flight branch with the given name.
func (s *Service) GetBranchByName(ctx context.Context, name string) (generation.Generation, error) {
	branch, err := s.st.GetBranchByName(ctx, name)
	if err != nil {
		return generation.Generation{}, errors.Capture(err)
	}
	return generationFromInternal(branch)
}

// ListBranches returns all in-flight branches.
func (s *Service) ListBranches(ctx context.Context) ([]generation.Generation, error) {
	branches, err := s.st.ListBranches(ctx)
	if err != nil {
		return nil, errors.Capture(err)
	}
	return transform.SliceOrErr(branches, generationFromInternal)
}

// TrackBranch records that the given units are tracking the named branch. The
// units' applications are claimed exclusively by the branch until it commits
// or aborts.
func (s *Service) TrackBranch(ctx context.Context, branchName string, unitUUIDs []coreunit.UUID) error {
	branch, err := s.st.GetBranchByName(ctx, branchName)
	if err != nil {
		return errors.Errorf("getting branch %q: %w", branchName, err)
	}

	uuids := transform.Slice(unitUUIDs, func(u coreunit.UUID) string { return string(u) })
	return s.st.TrackUnits(ctx, branch.UUID, uuids)
}

// UntrackBranch removes the given units from tracking the named branch.
func (s *Service) UntrackBranch(ctx context.Context, branchName string, unitUUIDs []coreunit.UUID) error {
	branch, err := s.st.GetBranchByName(ctx, branchName)
	if err != nil {
		return errors.Errorf("getting branch %q: %w", branchName, err)
	}

	uuids := transform.Slice(unitUUIDs, func(u coreunit.UUID) string { return string(u) })
	return s.st.UntrackUnits(ctx, branch.UUID, uuids)
}

// GetTrackedUnits returns the names of the units tracking the named branch.
func (s *Service) GetTrackedUnits(ctx context.Context, branchName string) ([]coreunit.Name, error) {
	branch, err := s.st.GetBranchByName(ctx, branchName)
	if err != nil {
		return nil, errors.Errorf("getting branch %q: %w", branchName, err)
	}

	names, err := s.st.GetTrackedUnitNames(ctx, branch.UUID)
	if err != nil {
		return nil, errors.Capture(err)
	}
	return transform.Slice(names, func(n string) coreunit.Name { return coreunit.Name(n) }), nil
}

// HasTrackedUnits reports whether any units are tracking the named branch.
func (s *Service) HasTrackedUnits(ctx context.Context, branchName string) (bool, error) {
	branch, err := s.st.GetBranchByName(ctx, branchName)
	if err != nil {
		return false, errors.Errorf("getting branch %q: %w", branchName, err)
	}
	return s.st.HasTrackedUnits(ctx, branch.UUID)
}

// GetBranchForUnit returns the in-flight branch that the given unit is
// tracking.
func (s *Service) GetBranchForUnit(ctx context.Context, unitUUID coreunit.UUID) (generation.Generation, error) {
	branch, err := s.st.GetBranchForUnit(ctx, string(unitUUID))
	if err != nil {
		return generation.Generation{}, errors.Capture(err)
	}
	return generationFromInternal(branch)
}

// CommitBranch applies the named branch's changes, archives the history and
// marks the branch committed. It returns the branch's generation id.
func (s *Service) CommitBranch(ctx context.Context, branchName, committedBy string) (uint64, error) {
	branch, err := s.st.GetBranchByName(ctx, branchName)
	if err != nil {
		return 0, errors.Errorf("getting branch %q: %w", branchName, err)
	}

	commitUUID, err := internaluuid.NewUUID()
	if err != nil {
		return 0, errors.Errorf("generating commit uuid: %w", err)
	}
	return s.st.Commit(ctx, branch.UUID, commitUUID.String(), committedBy)
}

// AbortBranch marks the named branch aborted and discards its changes.
func (s *Service) AbortBranch(ctx context.Context, branchName, abortedBy string) error {
	branch, err := s.st.GetBranchByName(ctx, branchName)
	if err != nil {
		return errors.Errorf("getting branch %q: %w", branchName, err)
	}
	return s.st.Abort(ctx, branch.UUID, abortedBy)
}

// ListCommits returns the committed generation history by commit time, oldest
// first.
func (s *Service) ListCommits(ctx context.Context) ([]generation.Commit, error) {
	commits, err := s.st.ListCommits(ctx)
	if err != nil {
		return nil, errors.Capture(err)
	}
	return transform.SliceOrErr(commits, commitFromInternal)
}

// GetCommit returns the commit identified by generation id.
func (s *Service) GetCommit(ctx context.Context, generationID uint64) (generation.Commit, error) {
	commit, err := s.st.GetCommit(ctx, generationID)
	if err != nil {
		return generation.Commit{}, errors.Capture(err)
	}
	return commitFromInternal(commit)
}

// generationFromInternal converts a scalar internal DTO into the public
// Generation type.
func generationFromInternal(g internal.Generation) (generation.Generation, error) {
	uuid, err := internaluuid.UUIDFromString(g.UUID)
	if err != nil {
		return generation.Generation{}, errors.Errorf("invalid generation uuid %q: %w", g.UUID, err)
	}

	state, err := stateFromString(g.State)
	if err != nil {
		return generation.Generation{}, errors.Errorf("invalid state for generation %q: %w", g.UUID, err)
	}

	return generation.Generation{
		UUID:         uuid,
		GenerationID: g.GenerationID,
		Name:         g.Name,
		State:        state,
		CreatedBy:    g.CreatedBy,
		CreatedAt:    g.CreatedAt,
		CompletedBy:  g.CompletedBy,
		CompletedAt:  g.CompletedAt,
	}, nil
}

// commitFromInternal converts a scalar internal DTO into the public Commit
// type.
func commitFromInternal(c internal.Commit) (generation.Commit, error) {
	uuid, err := internaluuid.UUIDFromString(c.UUID)
	if err != nil {
		return generation.Commit{}, errors.Errorf("invalid commit uuid %q: %w", c.UUID, err)
	}

	applications := transform.Slice(c.Applications, func(a internal.ApplicationConfigChange) generation.ApplicationConfigChange {
		return generation.ApplicationConfigChange{
			ApplicationUUID: coreapplication.UUID(a.ApplicationUUID),
			ApplicationName: a.ApplicationName,
			Config: transform.Slice(a.Config, func(c internal.ConfigChange) generation.ConfigChange {
				return generation.ConfigChange{Key: c.Key, Value: c.Value}
			}),
		}
	})

	return generation.Commit{
		UUID:         uuid,
		GenerationID: c.GenerationID,
		Name:         c.Name,
		CreatedBy:    c.CreatedBy,
		CommittedBy:  c.CommittedBy,
		CommittedAt:  c.CommittedAt,
		Applications: applications,
	}, nil
}

// stateFromString converts a state string into a public State value.
func stateFromString(s string) (generation.State, error) {
	switch generation.State(s) {
	case generation.StateInFlight, generation.StateCommitted, generation.StateAborted:
		return generation.State(s), nil
	default:
		return "", errors.Errorf("unknown generation state %q", s)
	}
}
