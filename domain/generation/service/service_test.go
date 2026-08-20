// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	gomock "github.com/canonical/gomock/gomock"
	"github.com/juju/tc"

	coreapplication "github.com/juju/juju/core/application"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/generation"
	generationerrors "github.com/juju/juju/domain/generation/errors"
	"github.com/juju/juju/domain/generation/internal"
	internaluuid "github.com/juju/juju/internal/uuid"
)

type serviceSuite struct {
	state   *MockState
	service *Service
}

func TestServiceSuite(t *testing.T) {
	tc.Run(t, &serviceSuite{})
}

func (s *serviceSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)
	s.state = NewMockState(ctrl)
	s.service = NewService(s.state)
	return ctrl
}

func (s *serviceSuite) generation(c *tc.C) internal.Generation {
	return internal.Generation{
		UUID:         tc.Must(c, internaluuid.NewUUID).String(),
		GenerationID: 42,
		Name:         "test",
		State:        string(generation.StateInFlight),
		CreatedBy:    "creator",
		CreatedAt:    time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC),
	}
}

func (s *serviceSuite) commit(c *tc.C) internal.Commit {
	return internal.Commit{
		UUID:         tc.Must(c, internaluuid.NewUUID).String(),
		GenerationID: 42,
		Name:         "test",
		CreatedBy:    "creator",
		CommittedBy:  "committer",
		CommittedAt:  time.Date(2026, time.August, 19, 11, 0, 0, 0, time.UTC),
		Applications: []internal.ApplicationConfigChange{{
			ApplicationUUID: "application-uuid",
			ApplicationName: "wordpress",
			Config: []internal.ConfigChange{
				{Key: "count", Value: 3},
				{Key: "title", Value: "blog"},
			},
		}, {
			ApplicationUUID: "second-application-uuid",
			ApplicationName: "mysql",
			Config: []internal.ConfigChange{
				{Key: "enabled", Value: true},
				{Key: "removed", Value: nil},
			},
		}},
	}
}

func (s *serviceSuite) TestAddBranch(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().AddBranch(gomock.Any(), gomock.Any(), "test", "creator").DoAndReturn(
		func(_ context.Context, uuid, _, _ string) (uint64, error) {
			_, err := internaluuid.UUIDFromString(uuid)
			c.Check(err, tc.ErrorIsNil)
			return 42, nil
		},
	)

	id, err := s.service.AddBranch(c.Context(), "test", "creator")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(id, tc.Equals, uint64(42))
}

func (s *serviceSuite) TestAddBranchEmptyName(c *tc.C) {
	defer s.setupMocks(c).Finish()

	_, err := s.service.AddBranch(c.Context(), "", "creator")
	c.Check(err, tc.ErrorMatches, "branch name cannot be empty")
}

func (s *serviceSuite) TestGetBranchByName(c *tc.C) {
	defer s.setupMocks(c).Finish()

	want := s.generation(c)
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(want, nil)

	got, err := s.service.GetBranchByName(c.Context(), "test")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got.UUID.String(), tc.Equals, want.UUID)
	c.Check(got.GenerationID, tc.Equals, want.GenerationID)
	c.Check(got.Name, tc.Equals, want.Name)
	c.Check(got.State, tc.Equals, generation.StateInFlight)
	c.Check(got.CreatedBy, tc.Equals, want.CreatedBy)
	c.Check(got.CreatedAt, tc.Equals, want.CreatedAt)
}

func (s *serviceSuite) TestGetBranchByNameError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().GetBranchByName(gomock.Any(), "missing").Return(internal.Generation{}, generationerrors.BranchNotFound)

	_, err := s.service.GetBranchByName(c.Context(), "missing")
	c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *serviceSuite) TestListBranches(c *tc.C) {
	defer s.setupMocks(c).Finish()

	first := s.generation(c)
	second := s.generation(c)
	second.State = string(generation.StateAborted)
	second.CompletedBy = "admin"
	second.CompletedAt = first.CreatedAt.Add(time.Hour)
	s.state.EXPECT().ListBranches(gomock.Any()).Return([]internal.Generation{first, second}, nil)

	got, err := s.service.ListBranches(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(got, tc.HasLen, 2)
	c.Check(got[0].State, tc.Equals, generation.StateInFlight)
	c.Check(got[1].State, tc.Equals, generation.StateAborted)
	c.Check(got[1].CompletedBy, tc.Equals, "admin")
	c.Check(got[1].CompletedAt, tc.Equals, second.CompletedAt)
}

func (s *serviceSuite) TestListBranchesStateError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().ListBranches(gomock.Any()).Return(nil, generationerrors.BranchNotFound)
	_, err := s.service.ListBranches(c.Context())
	c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *serviceSuite) TestListBranchesConversionError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	branch.UUID = "not-a-uuid"
	s.state.EXPECT().ListBranches(gomock.Any()).Return([]internal.Generation{branch}, nil)

	_, err := s.service.ListBranches(c.Context())
	c.Check(err, tc.ErrorMatches, `transforming slice at index 0: invalid generation uuid "not-a-uuid": .*`)
}

func (s *serviceSuite) TestTrackBranch(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	units := []coreunit.UUID{"unit-1", "unit-2"}
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().TrackUnits(gomock.Any(), branch.UUID, []string{"unit-1", "unit-2"}).Return(nil)

	err := s.service.TrackBranch(c.Context(), "test", units)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *serviceSuite) TestTrackBranchLookupError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().GetBranchByName(gomock.Any(), "missing").Return(internal.Generation{}, generationerrors.BranchNotFound)
	err := s.service.TrackBranch(c.Context(), "missing", nil)
	c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
	c.Check(err, tc.ErrorMatches, `getting branch "missing": branch not found`)
}

func (s *serviceSuite) TestTrackBranchStateError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().TrackUnits(gomock.Any(), branch.UUID, []string{"unit-1"}).Return(generationerrors.UnitNotFound)

	err := s.service.TrackBranch(c.Context(), "test", []coreunit.UUID{"unit-1"})
	c.Check(err, tc.ErrorIs, generationerrors.UnitNotFound)
}

func (s *serviceSuite) TestUntrackBranch(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().UntrackUnits(gomock.Any(), branch.UUID, []string{"unit-1"}).Return(nil)

	err := s.service.UntrackBranch(c.Context(), "test", []coreunit.UUID{"unit-1"})
	c.Assert(err, tc.ErrorIsNil)
}

func (s *serviceSuite) TestUntrackBranchLookupError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().GetBranchByName(gomock.Any(), "missing").Return(internal.Generation{}, generationerrors.BranchNotFound)
	err := s.service.UntrackBranch(c.Context(), "missing", nil)
	c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *serviceSuite) TestUntrackBranchStateError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	boom := errors.New("boom")
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().UntrackUnits(gomock.Any(), branch.UUID, []string{"unit-1"}).Return(boom)

	err := s.service.UntrackBranch(c.Context(), "test", []coreunit.UUID{"unit-1"})
	c.Check(err, tc.ErrorIs, boom)
}

func (s *serviceSuite) TestGetTrackedUnits(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().GetTrackedUnitNames(gomock.Any(), branch.UUID).Return([]string{"app/0", "app/1"}, nil)

	got, err := s.service.GetTrackedUnits(c.Context(), "test")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.DeepEquals, []coreunit.Name{"app/0", "app/1"})
}

func (s *serviceSuite) TestGetTrackedUnitsError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	boom := errors.New("boom")
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().GetTrackedUnitNames(gomock.Any(), branch.UUID).Return(nil, boom)

	_, err := s.service.GetTrackedUnits(c.Context(), "test")
	c.Check(err, tc.ErrorIs, boom)
}

func (s *serviceSuite) TestGetTrackedUnitsLookupError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().GetBranchByName(gomock.Any(), "missing").Return(internal.Generation{}, generationerrors.BranchNotFound)
	_, err := s.service.GetTrackedUnits(c.Context(), "missing")
	c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *serviceSuite) TestHasTrackedUnits(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().HasTrackedUnits(gomock.Any(), branch.UUID).Return(true, nil)

	got, err := s.service.HasTrackedUnits(c.Context(), "test")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.IsTrue)
}

func (s *serviceSuite) TestHasTrackedUnitsLookupError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().GetBranchByName(gomock.Any(), "missing").Return(internal.Generation{}, generationerrors.BranchNotFound)
	_, err := s.service.HasTrackedUnits(c.Context(), "missing")
	c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *serviceSuite) TestHasTrackedUnitsStateError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	boom := errors.New("boom")
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().HasTrackedUnits(gomock.Any(), branch.UUID).Return(false, boom)

	_, err := s.service.HasTrackedUnits(c.Context(), "test")
	c.Check(err, tc.ErrorIs, boom)
}

func (s *serviceSuite) TestGetBranchForUnit(c *tc.C) {
	defer s.setupMocks(c).Finish()

	want := s.generation(c)
	s.state.EXPECT().GetBranchForUnit(gomock.Any(), "unit-1").Return(want, nil)

	got, err := s.service.GetBranchForUnit(c.Context(), coreunit.UUID("unit-1"))
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got.UUID.String(), tc.Equals, want.UUID)
}

func (s *serviceSuite) TestGetBranchForUnitError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().GetBranchForUnit(gomock.Any(), "unit-1").Return(internal.Generation{}, generationerrors.BranchNotFound)
	_, err := s.service.GetBranchForUnit(c.Context(), coreunit.UUID("unit-1"))
	c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *serviceSuite) TestCommitBranch(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().Commit(gomock.Any(), branch.UUID, gomock.Any(), "committer").DoAndReturn(
		func(_ context.Context, _, commitUUID, _ string) (uint64, error) {
			_, err := internaluuid.UUIDFromString(commitUUID)
			c.Check(err, tc.ErrorIsNil)
			return 42, nil
		},
	)

	id, err := s.service.CommitBranch(c.Context(), "test", "committer")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(id, tc.Equals, uint64(42))
}

func (s *serviceSuite) TestCommitBranchLookupError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().GetBranchByName(gomock.Any(), "missing").Return(internal.Generation{}, generationerrors.BranchNotFound)
	_, err := s.service.CommitBranch(c.Context(), "missing", "committer")
	c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *serviceSuite) TestCommitBranchStateError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	boom := errors.New("boom")
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().Commit(gomock.Any(), branch.UUID, gomock.Any(), "committer").Return(uint64(0), boom)

	_, err := s.service.CommitBranch(c.Context(), "test", "committer")
	c.Check(err, tc.ErrorIs, boom)
}

func (s *serviceSuite) TestAbortBranch(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	s.state.EXPECT().Abort(gomock.Any(), branch.UUID, "aborter").Return(nil)

	err := s.service.AbortBranch(c.Context(), "test", "aborter")
	c.Assert(err, tc.ErrorIsNil)
}

func (s *serviceSuite) TestAbortBranchLookupError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().GetBranchByName(gomock.Any(), "missing").Return(internal.Generation{}, generationerrors.BranchNotFound)
	err := s.service.AbortBranch(c.Context(), "missing", "aborter")
	c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *serviceSuite) TestAbortBranchStateError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	branch := s.generation(c)
	s.state.EXPECT().GetBranchByName(gomock.Any(), "test").Return(branch, nil)
	wantErr := errors.New("boom")
	s.state.EXPECT().Abort(gomock.Any(), branch.UUID, "aborter").Return(wantErr)

	err := s.service.AbortBranch(c.Context(), "test", "aborter")
	c.Check(err, tc.ErrorIs, wantErr)
}

func (s *serviceSuite) TestListCommits(c *tc.C) {
	defer s.setupMocks(c).Finish()

	want := s.commit(c)
	s.state.EXPECT().ListCommits(gomock.Any()).Return([]internal.Commit{want}, nil)

	got, err := s.service.ListCommits(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(got, tc.HasLen, 1)
	c.Check(got[0].UUID.String(), tc.Equals, want.UUID)
	c.Check(got[0].Applications, tc.DeepEquals, []generation.ApplicationConfigChange{{
		ApplicationUUID: coreapplication.UUID("application-uuid"),
		ApplicationName: "wordpress",
		Config: []generation.ConfigChange{
			{Key: "count", Value: 3},
			{Key: "title", Value: "blog"},
		},
	}, {
		ApplicationUUID: coreapplication.UUID("second-application-uuid"),
		ApplicationName: "mysql",
		Config: []generation.ConfigChange{
			{Key: "enabled", Value: true},
			{Key: "removed", Value: nil},
		},
	}})
}

func (s *serviceSuite) TestListCommitsErrors(c *tc.C) {
	defer s.setupMocks(c).Finish()

	boom := errors.New("boom")
	s.state.EXPECT().ListCommits(gomock.Any()).Return(nil, boom)
	_, err := s.service.ListCommits(c.Context())
	c.Check(err, tc.ErrorIs, boom)
}

func (s *serviceSuite) TestGetCommit(c *tc.C) {
	defer s.setupMocks(c).Finish()

	want := s.commit(c)
	s.state.EXPECT().GetCommit(gomock.Any(), uint64(42)).Return(want, nil)

	got, err := s.service.GetCommit(c.Context(), 42)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got.Name, tc.Equals, want.Name)
	c.Check(got.CommittedAt, tc.Equals, want.CommittedAt)
}

func (s *serviceSuite) TestGetCommitError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.state.EXPECT().GetCommit(gomock.Any(), uint64(42)).Return(internal.Commit{}, generationerrors.CommitNotFound)
	_, err := s.service.GetCommit(c.Context(), 42)
	c.Check(err, tc.ErrorIs, generationerrors.CommitNotFound)
}

func (s *serviceSuite) TestGenerationFromInternalInvalidState(c *tc.C) {
	branch := s.generation(c)
	branch.State = "unknown"

	_, err := generationFromInternal(branch)
	c.Check(err, tc.ErrorMatches, `invalid state for generation .*: unknown generation state "unknown"`)
}

func (s *serviceSuite) TestCommitFromInternalInvalidUUID(c *tc.C) {
	commit := s.commit(c)
	commit.UUID = "not-a-uuid"

	_, err := commitFromInternal(commit)
	c.Check(err, tc.ErrorMatches, `invalid commit uuid "not-a-uuid": .*`)
}

func (s *serviceSuite) TestStateFromString(c *tc.C) {
	for _, want := range []generation.State{
		generation.StateInFlight,
		generation.StateCommitted,
		generation.StateAborted,
	} {
		got, err := stateFromString(string(want))
		c.Assert(err, tc.ErrorIsNil)
		c.Check(got, tc.Equals, want)
	}

	_, err := stateFromString("unknown")
	c.Check(err, tc.ErrorMatches, `unknown generation state "unknown"`)
}
