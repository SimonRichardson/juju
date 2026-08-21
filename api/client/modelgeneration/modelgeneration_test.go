// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelgeneration_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/canonical/gomock/gomock"
	"github.com/juju/names/v6"
	"github.com/juju/tc"

	basemocks "github.com/juju/juju/api/base/mocks"
	"github.com/juju/juju/api/client/modelgeneration"
	"github.com/juju/juju/rpc/params"
)

type clientSuite struct{}

func TestClientSuite(t *testing.T) {
	tc.Run(t, &clientSuite{})
}

func (s *clientSuite) expectCall(
	c *tc.C, method string, arg, result any,
) *modelgeneration.Client {
	ctrl := gomock.NewController(c)
	caller := basemocks.NewMockFacadeCaller(ctrl)
	caller.EXPECT().FacadeCall(c.Context(), method, arg, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ any, out any) error {
			reflect.ValueOf(out).Elem().Set(reflect.ValueOf(result))
			return nil
		},
	)
	return modelgeneration.NewClientFromCaller(caller)
}

func (s *clientSuite) TestAddAndAbortBranch(c *tc.C) {
	arg := params.BranchArg{BranchName: "test"}
	client := s.expectCall(c, "AddBranch", arg, params.ErrorResult{})
	c.Check(client.AddBranch(c.Context(), "test"), tc.ErrorIsNil)

	client = s.expectCall(c, "AbortBranch", arg, params.ErrorResult{})
	c.Check(client.AbortBranch(c.Context(), "test"), tc.ErrorIsNil)
}

func (s *clientSuite) TestTrackAndUntrackBranch(c *tc.C) {
	arg := params.BranchTrackArg{
		BranchName: "test",
		Entities: []params.Entity{
			{Tag: names.NewApplicationTag("mysql").String()},
			{Tag: names.NewUnitTag("redis/0").String()},
		},
	}
	client := s.expectCall(c, "TrackBranch", arg, params.ErrorResults{
		Results: []params.ErrorResult{{}, {}},
	})
	c.Check(client.TrackBranch(c.Context(), "test", []string{"mysql", "redis/0"}, 0), tc.ErrorIsNil)

	client = s.expectCall(c, "UntrackBranch", arg, params.ErrorResults{
		Results: []params.ErrorResult{{}, {}},
	})
	c.Check(client.UntrackBranch(c.Context(), "test", []string{"mysql", "redis/0"}, 0), tc.ErrorIsNil)
}

func (s *clientSuite) TestTrackBranchValidation(c *tc.C) {
	client := modelgeneration.NewClientFromCaller(nil)
	c.Check(client.TrackBranch(c.Context(), "test", nil, 0), tc.ErrorMatches, "no units or applications specified")
	c.Check(client.TrackBranch(c.Context(), "test", []string{"mysql", "redis"}, 1), tc.ErrorMatches,
		"number of units can only be specified for one application")
	c.Check(client.TrackBranch(c.Context(), "test", []string{"not/a/unit/x"}, 0), tc.ErrorMatches,
		`"not/a/unit/x" is not an application or a unit`)
}

func (s *clientSuite) TestCommitBranch(c *tc.C) {
	client := s.expectCall(c, "CommitBranch", params.BranchArg{BranchName: "test"}, params.IntResult{Result: 4})
	id, err := client.CommitBranch(c.Context(), "test")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(id, tc.Equals, 4)
}

func (s *clientSuite) TestHasActiveBranch(c *tc.C) {
	client := s.expectCall(c, "HasActiveBranch", params.BranchArg{BranchName: "test"}, params.BoolResult{Result: true})
	active, err := client.HasActiveBranch(c.Context(), "test")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(active, tc.IsTrue)
}

func (s *clientSuite) TestBranchInfo(c *tc.C) {
	want := []params.Generation{{BranchName: "test"}}
	arg := params.BranchInfoArgs{BranchNames: []string{"test"}, Detailed: true}
	client := s.expectCall(c, "BranchInfo", arg, params.BranchResults{Generations: want})
	got, err := client.BranchInfo(c.Context(), []string{"test"}, true)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.DeepEquals, want)
}

func (s *clientSuite) TestListAndShowCommits(c *tc.C) {
	want := params.Generation{BranchName: "test", GenerationId: 3}
	client := s.expectCall(c, "ListCommits", nil, params.BranchResults{Generations: []params.Generation{want}})
	commits, err := client.ListCommits(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(commits, tc.DeepEquals, []params.Generation{want})

	client = s.expectCall(c, "ShowCommit", params.GenerationId{GenerationId: 3}, params.GenerationResult{Generation: want})
	commit, err := client.ShowCommit(c.Context(), 3)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(commit, tc.DeepEquals, want)
}

func (s *clientSuite) TestEmbeddedError(c *tc.C) {
	wantErr := &params.Error{Message: "boom"}
	client := s.expectCall(c, "AddBranch", params.BranchArg{BranchName: "test"}, params.ErrorResult{Error: wantErr})
	c.Check(client.AddBranch(c.Context(), "test"), tc.ErrorMatches, "boom")
}
