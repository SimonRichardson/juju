// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"fmt"

	"github.com/juju/errors"
	"github.com/juju/gnuflag"

	jujucmd "github.com/juju/juju/cmd"
	"github.com/juju/juju/cmd/cmd"
	"github.com/juju/juju/cmd/modelcmd"
)

// NewCommitCommand returns the command for committing a model branch.
func NewCommitCommand() cmd.Command {
	return modelcmd.Wrap(&commitCommand{})
}

type commitCommand struct {
	generationCommandBase
	branchName string
}

func (c *commitCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:    "commit",
		Args:    "<branch name>",
		Purpose: "Commits staged branch changes to the model.",
		Examples: `
    juju commit test
`,
		SeeAlso: []string{"add-branch", "track", "abort", "commits"},
	})
}

func (c *commitCommand) SetFlags(f *gnuflag.FlagSet) {
	c.ModelCommandBase.SetFlags(f)
}

func (c *commitCommand) Init(args []string) error {
	if len(args) != 1 {
		return errors.New("expected a branch name")
	}
	if err := validateBranchName(args[0]); err != nil {
		return errors.Trace(err)
	}
	c.branchName = args[0]
	return nil
}

func (c *commitCommand) Run(ctx *cmd.Context) error {
	client, err := c.getGenerationAPI(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	defer client.Close()

	generationID, err := client.CommitBranch(ctx, c.branchName)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = fmt.Fprintf(ctx.Stdout, "model is now at generation %d\n", generationID)
	return errors.Trace(err)
}
