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

// NewAddBranchCommand returns the command for creating a model branch.
func NewAddBranchCommand() cmd.Command {
	return modelcmd.Wrap(&addBranchCommand{})
}

type addBranchCommand struct {
	generationCommandBase
	branchName string
}

func (c *addBranchCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:    "add-branch",
		Args:    "<branch name>",
		Purpose: "Creates a branch for staged model changes.",
		Examples: `
    juju add-branch test
`,
		SeeAlso: []string{"track", "commit", "abort"},
	})
}

func (c *addBranchCommand) SetFlags(f *gnuflag.FlagSet) {
	c.ModelCommandBase.SetFlags(f)
}

func (c *addBranchCommand) Init(args []string) error {
	if len(args) != 1 {
		return errors.New("expected a branch name")
	}
	if err := validateBranchName(args[0]); err != nil {
		return errors.Trace(err)
	}
	c.branchName = args[0]
	return nil
}

func (c *addBranchCommand) Run(ctx *cmd.Context) error {
	client, err := c.getGenerationAPI(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	defer client.Close()

	if err := client.AddBranch(ctx, c.branchName); err != nil {
		return errors.Trace(err)
	}
	_, err = fmt.Fprintf(ctx.Stdout, "Created branch %q\n", c.branchName)
	return errors.Trace(err)
}
