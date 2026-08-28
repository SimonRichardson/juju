// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"testing"

	"github.com/juju/errors"
	"github.com/juju/tc"

	"github.com/juju/juju/cmd/cmd/cmdtesting"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/juju/osenv"
	"github.com/juju/juju/rpc/params"
)

type showCommitCommandSuite struct{}

func TestShowCommitCommandSuite(t *testing.T) {
	tc.Run(t, &showCommitCommandSuite{})
}

func (s *showCommitCommandSuite) TestShowCommitYAML(c *tc.C) {
	api := &fakeGenerationAPI{commit: params.Generation{
		GenerationId: 0,
		BranchName:   "test",
		Completed:    84,
		CompletedBy:  "bob",
		CreatedBy:    "alice",
		Applications: []params.GenerationApplication{{
			ApplicationName: "mysql",
			ConfigChanges:   map[string]any{"foo": "bar"},
		}},
	}}
	command := &showCommitCommand{generationCommandBase: commandBase(api)}
	ctx, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "0", "--utc")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(api.generationID, tc.Equals, 0)
	c.Check(cmdtesting.Stdout(ctx), tc.Equals, `generation-id: 0
branch:
  test:
    applications:
    - application: mysql
      config:
        foo: bar
committed-at: 1970-01-01 00:01:24Z
committed-by: bob
created-by: alice
`)
	c.Check(api.closed, tc.Equals, 1)
}

func (s *showCommitCommandSuite) TestMissingGenerationID(c *tc.C) {
	command := &showCommitCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command))
	c.Check(err, tc.ErrorMatches, "expected exactly one generation id, got 0 arguments")
}

func (s *showCommitCommandSuite) TestTooManyGenerationIDs(c *tc.C) {
	command := &showCommitCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "0", "1")
	c.Check(err, tc.ErrorMatches, "expected exactly one generation id, got 2 arguments")
}

func (s *showCommitCommandSuite) TestInvalidGenerationID(c *tc.C) {
	command := &showCommitCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "invalid")
	c.Check(err, tc.ErrorMatches, `invalid generation id "invalid"`)
}

func (s *showCommitCommandSuite) TestNegativeGenerationID(c *tc.C) {
	command := &showCommitCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "--", "-1")
	c.Check(err, tc.ErrorMatches, "generation id cannot be negative")
}

func (s *showCommitCommandSuite) TestInvalidISOTimestampEnvironment(c *tc.C) {
	c.Cleanup(testhelpers.PatchEnvironment(osenv.JujuStatusIsoTimeEnvKey, "invalid"))
	command := &showCommitCommand{generationCommandBase: commandBase(&fakeGenerationAPI{})}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "0")
	c.Check(err, tc.ErrorMatches, "invalid JUJU_STATUS_ISO_TIME env var, expected true\\|false.*")
}

func (s *showCommitCommandSuite) TestAPIError(c *tc.C) {
	wantErr := errors.New("boom")
	api := &fakeGenerationAPI{err: wantErr}
	command := &showCommitCommand{generationCommandBase: commandBase(api)}
	_, err := cmdtesting.RunCommand(c, wrapGenerationCommand(command), "0")
	c.Check(err, tc.ErrorIs, wantErr)
	c.Check(api.closed, tc.Equals, 1)
}
