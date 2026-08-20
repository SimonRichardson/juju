// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"testing"

	"github.com/juju/errors"
	"github.com/juju/tc"

	"github.com/juju/juju/cmd/cmd/cmdtesting"
)

type addBranchCommandSuite struct{}

func TestAddBranchCommandSuite(t *testing.T) {
	tc.Run(t, &addBranchCommandSuite{})
}

func (s *addBranchCommandSuite) TestAddBranch(c *tc.C) {
	api := &fakeGenerationAPI{}
	command := &addBranchCommand{generationCommandBase: commandBase(api)}
	ctx, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), tc.Equals, "Created branch \"test\"\n")
	c.Check(api.branchName, tc.Equals, "test")
	c.Check(api.closed, tc.Equals, 1)
}

func (s *addBranchCommandSuite) TestMissingBranchName(c *tc.C) {
	command := &addBranchCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command))
	c.Check(err, tc.ErrorMatches, "expected a branch name")
}

func (s *addBranchCommandSuite) TestTooManyBranchNames(c *tc.C) {
	command := &addBranchCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "one", "two")
	c.Check(err, tc.ErrorMatches, "expected a branch name")
}

func (s *addBranchCommandSuite) TestReservedMainBranch(c *tc.C) {
	command := &addBranchCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "main")
	c.Check(err, tc.ErrorMatches, `branch name "main" is reserved`)
}

func (s *addBranchCommandSuite) TestAPIError(c *tc.C) {
	wantErr := errors.New("boom")
	api := &fakeGenerationAPI{err: wantErr}
	command := &addBranchCommand{generationCommandBase: commandBase(api)}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test")
	c.Check(err, tc.ErrorIs, wantErr)
	c.Check(api.closed, tc.Equals, 1)
}
