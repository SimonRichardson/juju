// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.
package main

import (
	"encoding/csv"

	"github.com/juju/cmd"
	"github.com/juju/errors"

	jujucmd "github.com/juju/juju/cmd"
	"github.com/juju/juju/cmd/modelcmd"
)

func newListCommand(history *history) cmd.Command {
	return modelcmd.Wrap(&listCommand{history: history})
}

type listCommand struct {
	modelcmd.ModelCommandBase

	history *history
}

// Init implements Command.Init.
func (c *listCommand) Init(args []string) (err error) {
	if len(args) != 0 {
		return errors.New("expects no arguments")
	}
	return nil
}

// Info implements Command.Info.
func (c *listCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:    "list",
		Purpose: "list prints out the current stash history",
		Doc: `
List prints out the current stash history

See:
	juju stash pop
	juju stash push
	juju switch
`,
	})
}

// Run implements Command.Run.
func (c *listCommand) Run(ctx *cmd.Context) (err error) {
	w := csv.NewWriter(ctx.GetStdout())
	if err := w.Write([]string{"controller", "model"}); err != nil {
		return errors.Trace(err)
	}
	snapshots, err := c.history.Snapshots()
	if err != nil {
		return errors.Trace(err)
	}
	for _, snap := range snapshots {
		if err := w.Write([]string{snap.controllerName, snap.modelName}); err != nil {
			return errors.Trace(err)
		}
	}
	w.Flush()
	return w.Error()
}
