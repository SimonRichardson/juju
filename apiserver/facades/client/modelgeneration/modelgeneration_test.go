// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelgeneration

import (
	"testing"
	"time"

	"github.com/canonical/gomock/gomock"
	"github.com/juju/names/v6"
	"github.com/juju/tc"

	"github.com/juju/juju/apiserver/authentication"
	apiservererrors "github.com/juju/juju/apiserver/errors"
	facademocks "github.com/juju/juju/apiserver/facade/mocks"
	coreapplication "github.com/juju/juju/core/application"
	coremodel "github.com/juju/juju/core/model"
	"github.com/juju/juju/core/permission"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/generation"
	generationerrors "github.com/juju/juju/domain/generation/errors"
	internalerrors "github.com/juju/juju/internal/errors"
	internaluuid "github.com/juju/juju/internal/uuid"
	"github.com/juju/juju/rpc/params"
)

type facadeSuite struct {
	modelUUID coremodel.UUID
}

func TestFacadeSuite(t *testing.T) {
	tc.Run(t, &facadeSuite{})
}

func (s *facadeSuite) SetUpTest(c *tc.C) {
	s.modelUUID = tc.Must0(c, coremodel.NewUUID)
}

func (s *facadeSuite) setup(c *tc.C) (*API, *MockGenerationService, *MockApplicationService, *facademocks.MockAuthorizer) {
	ctrl := gomock.NewController(c)
	generationService := NewMockGenerationService(ctrl)
	applicationService := NewMockApplicationService(ctrl)
	authorizer := facademocks.NewMockAuthorizer(ctrl)
	authorizer.EXPECT().AuthClient().Return(true)
	authorizer.EXPECT().GetAuthTag().Return(names.NewUserTag("admin"))
	api, err := NewAPI(
		authorizer, "controller", s.modelUUID,
		generationService, applicationService,
	)
	c.Assert(err, tc.ErrorIsNil)
	return api, generationService, applicationService, authorizer
}

func (s *facadeSuite) expectAccess(
	authorizer *facademocks.MockAuthorizer, access permission.Access,
) {
	authorizer.EXPECT().HasPermission(
		gomock.Any(), permission.SuperuserAccess,
		names.NewControllerTag("controller"),
	).Return(authentication.ErrorEntityMissingPermission)
	authorizer.EXPECT().HasPermission(
		gomock.Any(), access, names.NewModelTag(s.modelUUID.String()),
	).Return(nil)
}

func (s *facadeSuite) TestNewAPIRejectsNonClient(c *tc.C) {
	ctrl := gomock.NewController(c)
	authorizer := facademocks.NewMockAuthorizer(ctrl)
	authorizer.EXPECT().AuthClient().Return(false)

	_, err := NewAPI(
		authorizer, "controller", s.modelUUID,
		NewMockGenerationService(ctrl), NewMockApplicationService(ctrl),
	)
	c.Check(err, tc.ErrorIs, apiservererrors.ErrPerm)
}

func (s *facadeSuite) TestAddBranch(c *tc.C) {
	api, generationService, _, authorizer := s.setup(c)
	s.expectAccess(authorizer, permission.AdminAccess)
	generationService.EXPECT().AddBranch(gomock.Any(), "test", "admin").Return(uint64(0), nil)

	result, err := api.AddBranch(c.Context(), params.BranchArg{BranchName: "test"})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(result.Error, tc.IsNil)
}

func (s *facadeSuite) TestAddBranchPermissionDenied(c *tc.C) {
	api, _, _, authorizer := s.setup(c)
	denied := internalerrors.New("denied")
	authorizer.EXPECT().HasPermission(
		gomock.Any(), permission.SuperuserAccess,
		names.NewControllerTag("controller"),
	).Return(authentication.ErrorEntityMissingPermission)
	authorizer.EXPECT().HasPermission(
		gomock.Any(), permission.AdminAccess,
		names.NewModelTag(s.modelUUID.String()),
	).Return(denied)

	_, err := api.AddBranch(c.Context(), params.BranchArg{BranchName: "test"})
	c.Check(err, tc.ErrorIs, denied)
}

func (s *facadeSuite) TestTrackBranchUnit(c *tc.C) {
	api, generationService, applicationService, authorizer := s.setup(c)
	s.expectAccess(authorizer, permission.AdminAccess)
	unitUUID := tc.Must(c, coreunit.NewUUID)
	applicationService.EXPECT().GetUnitUUID(gomock.Any(), coreunit.Name("mysql/0")).Return(unitUUID, nil)
	generationService.EXPECT().TrackBranch(gomock.Any(), "test", []coreunit.UUID{unitUUID}).Return(nil)

	result, err := api.TrackBranch(c.Context(), params.BranchTrackArg{
		BranchName: "test",
		Entities:   []params.Entity{{Tag: names.NewUnitTag("mysql/0").String()}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result.Results, tc.HasLen, 1)
	c.Check(result.Results[0].Error, tc.IsNil)
}

func (s *facadeSuite) TestTrackBranchApplicationSubset(c *tc.C) {
	api, generationService, applicationService, authorizer := s.setup(c)
	s.expectAccess(authorizer, permission.AdminAccess)
	firstUUID := tc.Must(c, coreunit.NewUUID)
	secondUUID := tc.Must(c, coreunit.NewUUID)
	applicationService.EXPECT().GetUnitNamesForApplication(gomock.Any(), "mysql").Return(
		[]coreunit.Name{"mysql/2", "mysql/0", "mysql/1"}, nil,
	)
	applicationService.EXPECT().GetUnitUUID(gomock.Any(), coreunit.Name("mysql/0")).Return(firstUUID, nil)
	applicationService.EXPECT().GetUnitUUID(gomock.Any(), coreunit.Name("mysql/1")).Return(secondUUID, nil)
	generationService.EXPECT().TrackBranch(
		gomock.Any(), "test", []coreunit.UUID{firstUUID, secondUUID},
	).Return(nil)

	result, err := api.TrackBranch(c.Context(), params.BranchTrackArg{
		BranchName: "test",
		Entities:   []params.Entity{{Tag: names.NewApplicationTag("mysql").String()}},
		NumUnits:   2,
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(result.Results[0].Error, tc.IsNil)
}

func (s *facadeSuite) TestUntrackBranchUnit(c *tc.C) {
	api, generationService, applicationService, authorizer := s.setup(c)
	s.expectAccess(authorizer, permission.AdminAccess)
	unitUUID := tc.Must(c, coreunit.NewUUID)
	applicationService.EXPECT().GetUnitUUID(gomock.Any(), coreunit.Name("mysql/0")).Return(unitUUID, nil)
	generationService.EXPECT().UntrackBranch(gomock.Any(), "test", []coreunit.UUID{unitUUID}).Return(nil)

	result, err := api.UntrackBranch(c.Context(), params.BranchTrackArg{
		BranchName: "test",
		Entities:   []params.Entity{{Tag: names.NewUnitTag("mysql/0").String()}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(result.Results[0].Error, tc.IsNil)
}

func (s *facadeSuite) TestTrackBranchInvalidEntity(c *tc.C) {
	api, _, _, authorizer := s.setup(c)
	s.expectAccess(authorizer, permission.AdminAccess)

	result, err := api.TrackBranch(c.Context(), params.BranchTrackArg{
		BranchName: "test",
		Entities:   []params.Entity{{Tag: names.NewMachineTag("0").String()}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result.Results, tc.HasLen, 1)
	c.Check(result.Results[0].Error.Message, tc.Matches, `expected unit or application tag.*`)
}

func (s *facadeSuite) TestCommitAndAbort(c *tc.C) {
	api, generationService, _, authorizer := s.setup(c)
	s.expectAccess(authorizer, permission.AdminAccess)
	generationService.EXPECT().CommitBranch(gomock.Any(), "test", "admin").Return(uint64(3), nil)

	commit, err := api.CommitBranch(c.Context(), params.BranchArg{BranchName: "test"})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(commit, tc.DeepEquals, params.IntResult{Result: 3})

	s.expectAccess(authorizer, permission.AdminAccess)
	generationService.EXPECT().AbortBranch(gomock.Any(), "next", "admin").Return(nil)
	abort, err := api.AbortBranch(c.Context(), params.BranchArg{BranchName: "next"})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(abort.Error, tc.IsNil)
}

func (s *facadeSuite) TestBranchInfo(c *tc.C) {
	api, generationService, applicationService, authorizer := s.setup(c)
	s.expectAccess(authorizer, permission.ReadAccess)
	createdAt := time.Unix(42, 0)
	branch := generation.Generation{
		UUID:         internaluuid.MustNewUUID(),
		GenerationID: 2,
		Name:         "test",
		State:        generation.StateInFlight,
		CreatedBy:    "admin",
		CreatedAt:    createdAt,
	}
	generationService.EXPECT().ListBranches(gomock.Any()).Return([]generation.Generation{branch}, nil)
	generationService.EXPECT().GetTrackedUnits(gomock.Any(), "test").Return(
		[]coreunit.Name{"mysql/0", "mysql/1"}, nil,
	)
	applicationService.EXPECT().GetUnitNamesForApplication(gomock.Any(), "mysql").Return(
		[]coreunit.Name{"mysql/0", "mysql/1", "mysql/2"}, nil,
	)

	result, err := api.BranchInfo(c.Context(), params.BranchInfoArgs{Detailed: true})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result.Error, tc.IsNil)
	c.Assert(result.Generations, tc.HasLen, 1)
	c.Check(result.Generations[0], tc.DeepEquals, params.Generation{
		BranchName:   "test",
		Created:      42,
		CreatedBy:    "admin",
		GenerationId: 2,
		Applications: []params.GenerationApplication{{
			ApplicationName: "mysql",
			UnitProgress:    "2/3",
			UnitsTracking:   []string{"mysql/0", "mysql/1"},
			UnitsPending:    []string{"mysql/2"},
		}},
	})
}

func (s *facadeSuite) TestHasActiveBranchNotFound(c *tc.C) {
	api, generationService, _, authorizer := s.setup(c)
	s.expectAccess(authorizer, permission.ReadAccess)
	generationService.EXPECT().GetBranchByName(gomock.Any(), "missing").Return(
		generation.Generation{}, generationerrors.BranchNotFound,
	)

	result, err := api.HasActiveBranch(c.Context(), params.BranchArg{BranchName: "missing"})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(result.Result, tc.IsFalse)
	c.Check(result.Error, tc.IsNil)
}

func (s *facadeSuite) TestShowCommit(c *tc.C) {
	api, generationService, _, authorizer := s.setup(c)
	s.expectAccess(authorizer, permission.ReadAccess)
	generationService.EXPECT().GetCommit(gomock.Any(), uint64(4)).Return(generation.Commit{
		UUID:         internaluuid.MustNewUUID(),
		GenerationID: 4,
		Name:         "test",
		CreatedBy:    "admin",
		CommittedBy:  "admin",
		CommittedAt:  time.Unix(84, 0),
		Applications: []generation.ApplicationConfigChange{{
			ApplicationUUID: tc.Must(c, coreapplication.NewUUID),
			ApplicationName: "mysql",
			Config:          []generation.ConfigChange{{Key: "foo", Value: "bar"}},
		}},
	}, nil)

	result, err := api.ShowCommit(c.Context(), params.GenerationId{GenerationId: 4})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(result.Error, tc.IsNil)
	c.Check(result.Generation.BranchName, tc.Equals, "test")
	c.Check(result.Generation.GenerationId, tc.Equals, 4)
	c.Check(result.Generation.Applications[0].ConfigChanges, tc.DeepEquals, map[string]any{"foo": "bar"})
}

func (s *facadeSuite) TestListCommits(c *tc.C) {
	api, generationService, _, authorizer := s.setup(c)
	s.expectAccess(authorizer, permission.ReadAccess)
	generationService.EXPECT().ListCommits(gomock.Any()).Return([]generation.Commit{{
		UUID:         internaluuid.MustNewUUID(),
		GenerationID: 4,
		Name:         "test",
		CreatedBy:    "creator",
		CommittedBy:  "committer",
		CommittedAt:  time.Unix(84, 0),
	}}, nil)

	result, err := api.ListCommits(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result.Error, tc.IsNil)
	c.Assert(result.Generations, tc.HasLen, 1)
	c.Check(result.Generations[0].BranchName, tc.Equals, "test")
	c.Check(result.Generations[0].CompletedBy, tc.Equals, "committer")
}
