// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"testing"
	"time"

	"github.com/juju/errors"
	"github.com/juju/tc"

	"github.com/juju/juju/cmd/cmd/cmdtesting"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/juju/osenv"
	"github.com/juju/juju/rpc/params"
)

type commitsCommandSuite struct{}

func TestCommitsCommandSuite(t *testing.T) {
	tc.Run(t, &commitsCommandSuite{})
}

func (s *commitsCommandSuite) TestCommitsYAML(c *tc.C) {
	api := &fakeGenerationAPI{commits: []params.Generation{
		{GenerationId: 0, BranchName: "first", Completed: 42, CompletedBy: "alice"},
		{GenerationId: 1, BranchName: "second", Completed: 84, CompletedBy: "bob"},
	}}
	command := &commitsCommand{
		generationCommandBase: commandBase(api),
		now:                   func() time.Time { return time.Unix(100, 0) },
	}
	ctx, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "--format", "yaml", "--utc")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), tc.Equals, `commits:
- id: 1
  branch-name: second
  committed-at: 1970-01-01 00:01:24Z
  committed-by: bob
- id: 0
  branch-name: first
  committed-at: 1970-01-01 00:00:42Z
  committed-by: alice
`)
	c.Check(api.closed, tc.Equals, 1)
}

func (s *commitsCommandSuite) TestRejectsArguments(c *tc.C) {
	command := &commitsCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "unexpected")
	c.Check(err, tc.ErrorMatches, `unrecognized args: \["unexpected"\]`)
}

func (s *commitsCommandSuite) TestInvalidISOTimestampEnvironment(c *tc.C) {
	c.Cleanup(testhelpers.PatchEnvironment(osenv.JujuStatusIsoTimeEnvKey, "invalid"))
	command := &commitsCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command))
	c.Check(err, tc.ErrorMatches, "invalid JUJU_STATUS_ISO_TIME env var, expected true\\|false.*")
}

func (s *commitsCommandSuite) TestAPIError(c *tc.C) {
	wantErr := errors.New("boom")
	api := &fakeGenerationAPI{err: wantErr}
	command := &commitsCommand{generationCommandBase: commandBase(api)}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command))
	c.Check(err, tc.ErrorIs, wantErr)
	c.Check(api.closed, tc.Equals, 1)
}

func (s *commitsCommandSuite) TestNoCommits(c *tc.C) {
	api := &fakeGenerationAPI{}
	command := &commitsCommand{generationCommandBase: commandBase(api)}
	ctx, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command))
	c.Assert(err, tc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), tc.Equals, "")
	c.Check(cmdtesting.Stderr(ctx), tc.Equals, "No commits to list\n")
}
