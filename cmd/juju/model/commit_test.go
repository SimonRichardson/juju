// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"testing"

	"github.com/juju/errors"
	"github.com/juju/tc"

	"github.com/juju/juju/cmd/cmd/cmdtesting"
)

type commitCommandSuite struct{}

func TestCommitCommandSuite(t *testing.T) {
	tc.Run(t, &commitCommandSuite{})
}

func (s *commitCommandSuite) TestCommitAllowsGenerationZero(c *tc.C) {
	api := &fakeGenerationAPI{generationID: 0}
	command := &commitCommand{generationCommandBase: commandBase(api)}
	ctx, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), tc.Equals, "model is now at generation 0\n")
	c.Check(api.branchName, tc.Equals, "test")
	c.Check(api.closed, tc.Equals, 1)
}

func (s *commitCommandSuite) TestMissingBranchName(c *tc.C) {
	command := &commitCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command))
	c.Check(err, tc.ErrorMatches, "expected a branch name")
}

func (s *commitCommandSuite) TestTooManyBranchNames(c *tc.C) {
	command := &commitCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "one", "two")
	c.Check(err, tc.ErrorMatches, "expected a branch name")
}

func (s *commitCommandSuite) TestReservedMainBranch(c *tc.C) {
	command := &commitCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "main")
	c.Check(err, tc.ErrorMatches, `branch name "main" is reserved`)
}

func (s *commitCommandSuite) TestAPIError(c *tc.C) {
	wantErr := errors.New("boom")
	api := &fakeGenerationAPI{err: wantErr}
	command := &commitCommand{generationCommandBase: commandBase(api)}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test")
	c.Check(err, tc.ErrorIs, wantErr)
	c.Check(api.closed, tc.Equals, 1)
}
