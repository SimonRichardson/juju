// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"fmt"
	"strconv"

	"github.com/juju/errors"
	"github.com/juju/gnuflag"
	"github.com/juju/names/v6"

	jujucmd "github.com/juju/juju/cmd"
	"github.com/juju/juju/cmd/cmd"
	"github.com/juju/juju/cmd/modelcmd"
)

// NewTrackBranchCommand returns the command for enrolling units in a branch.
func NewTrackBranchCommand() cmd.Command {
	return modelcmd.Wrap(&trackBranchCommand{})
}

type trackBranchCommand struct {
	generationCommandBase
	branchName string
	entities   []string
	numUnits   optionalIntValue
}

func (c *trackBranchCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:    "track",
		Args:    "<branch name> <application or unit> [...]",
		Purpose: "Enrolls units to track changes made under a branch.",
		Examples: `
    juju track test mysql/0
    juju track test mysql
    juju track test mysql -n 2
`,
		SeeAlso: []string{"add-branch", "commit", "abort"},
	})
}

func (c *trackBranchCommand) SetFlags(f *gnuflag.FlagSet) {
	c.ModelCommandBase.SetFlags(f)
	f.Var(&c.numUnits, "n", "Number of application units to track")
}

func (c *trackBranchCommand) Init(args []string) error {
	if len(args) < 2 {
		return errors.New("expected a branch name and at least one application or unit")
	}
	if err := validateBranchName(args[0]); err != nil {
		return errors.Trace(err)
	}

	if c.numUnits.value != nil && *c.numUnits.value <= 0 {
		return errors.New("number of units to track must be greater than zero")
	}
	for _, entity := range args[1:] {
		if !names.IsValidApplication(entity) && !names.IsValidUnit(entity) {
			return errors.Errorf("invalid application or unit name %q", entity)
		}
	}
	if c.numUnits.value != nil {
		if len(args[1:]) != 1 {
			return errors.New("-n can only be used with one application")
		}
		if names.IsValidUnit(args[1]) {
			return errors.New("-n cannot be used with a unit")
		}
	}

	c.branchName = args[0]
	c.entities = args[1:]
	return nil
}

func (c *trackBranchCommand) Run(ctx *cmd.Context) error {
	client, err := c.getGenerationAPI(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	defer client.Close()

	numUnits := 0
	if c.numUnits.value != nil {
		numUnits = *c.numUnits.value
	}
	return errors.Trace(client.TrackBranch(ctx, c.branchName, c.entities, numUnits))
}

type optionalIntValue struct {
	value *int
}

func (v *optionalIntValue) Set(value string) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return errors.Trace(err)
	}
	v.value = &n
	return nil
}

func (v *optionalIntValue) Get() any {
	if v.value == nil {
		return nil
	}
	return *v.value
}

func (v *optionalIntValue) String() string {
	if v.value == nil {
		return "<all>"
	}
	return fmt.Sprint(*v.value)
}
