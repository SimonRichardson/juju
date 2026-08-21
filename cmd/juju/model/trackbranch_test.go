// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"testing"

	"github.com/juju/errors"
	"github.com/juju/tc"

	"github.com/juju/juju/cmd/cmd/cmdtesting"
)

type trackBranchCommandSuite struct{}

func TestTrackBranchCommandSuite(t *testing.T) {
	tc.Run(t, &trackBranchCommandSuite{})
}

func (s *trackBranchCommandSuite) TestTrackApplicationSubset(c *tc.C) {
	api := &fakeGenerationAPI{}
	command := &trackBranchCommand{generationCommandBase: commandBase(api)}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test", "mysql", "-n", "2")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(api.branchName, tc.Equals, "test")
	c.Check(api.entities, tc.DeepEquals, []string{"mysql"})
	c.Check(api.numUnits, tc.Equals, 2)
	c.Check(api.closed, tc.Equals, 1)
}

func (s *trackBranchCommandSuite) TestTrackMultipleEntities(c *tc.C) {
	api := &fakeGenerationAPI{}
	command := &trackBranchCommand{generationCommandBase: commandBase(api)}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test", "mysql", "redis/0")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(api.entities, tc.DeepEquals, []string{"mysql", "redis/0"})
	c.Check(api.numUnits, tc.Equals, 0)
}

func (s *trackBranchCommandSuite) TestMissingEntities(c *tc.C) {
	command := &trackBranchCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test")
	c.Check(err, tc.ErrorMatches, "expected a branch name and at least one application or unit")
}

func (s *trackBranchCommandSuite) TestInvalidEntity(c *tc.C) {
	command := &trackBranchCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test", "mysql/invalid")
	c.Check(err, tc.ErrorMatches, `invalid application or unit name "mysql/invalid"`)
}

func (s *trackBranchCommandSuite) TestRejectsZeroUnitCount(c *tc.C) {
	command := &trackBranchCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test", "mysql", "-n", "0")
	c.Check(err, tc.ErrorMatches, "number of units to track must be greater than zero")
}

func (s *trackBranchCommandSuite) TestRejectsNegativeUnitCount(c *tc.C) {
	command := &trackBranchCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test", "mysql", "-n", "-1")
	c.Check(err, tc.ErrorMatches, "number of units to track must be greater than zero")
}

func (s *trackBranchCommandSuite) TestRejectsUnitCountWithUnit(c *tc.C) {
	command := &trackBranchCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test", "mysql/0", "-n", "1")
	c.Check(err, tc.ErrorMatches, "-n cannot be used with a unit")
}

func (s *trackBranchCommandSuite) TestRejectsUnitCountWithMultipleEntities(c *tc.C) {
	command := &trackBranchCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test", "mysql", "redis", "-n", "1")
	c.Check(err, tc.ErrorMatches, "-n can only be used with one application")
}

func (s *trackBranchCommandSuite) TestAPIError(c *tc.C) {
	wantErr := errors.New("boom")
	api := &fakeGenerationAPI{err: wantErr}
	command := &trackBranchCommand{generationCommandBase: commandBase(api)}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "test", "mysql")
	c.Check(err, tc.ErrorIs, wantErr)
	c.Check(api.closed, tc.Equals, 1)
}
