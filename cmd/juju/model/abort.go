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

// NewAbortCommand returns the command for aborting a model branch.
func NewAbortCommand() cmd.Command {
	return modelcmd.Wrap(&abortCommand{})
}

type abortCommand struct {
	generationCommandBase
	branchName string
}

func (c *abortCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:    "abort",
		Args:    "<branch name>",
		Purpose: "Aborts a branch without applying its changes.",
		Examples: `
    juju abort test
`,
		SeeAlso: []string{"add-branch", "track", "commit"},
	})
}

func (c *abortCommand) SetFlags(f *gnuflag.FlagSet) {
	c.ModelCommandBase.SetFlags(f)
}

func (c *abortCommand) Init(args []string) error {
	if len(args) != 1 {
		return errors.New("expected a branch name")
	}
	if err := validateBranchName(args[0]); err != nil {
		return errors.Trace(err)
	}
	c.branchName = args[0]
	return nil
}

func (c *abortCommand) Run(ctx *cmd.Context) error {
	client, err := c.getGenerationAPI(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	defer client.Close()

	if err := client.AbortBranch(ctx, c.branchName); err != nil {
		return errors.Trace(err)
	}
	_, err = fmt.Fprintf(ctx.Stdout, "Aborted branch %q\n", c.branchName)
	return errors.Trace(err)
}
