// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.
package main

import (
	"github.com/juju/cmd"
	"github.com/juju/errors"
	"github.com/juju/gnuflag"

	jujucmd "github.com/juju/juju/cmd"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/jujuclient"
)

func newPopCommand(clientStore jujuclient.ClientStore, history *history) cmd.Command {
	return modelcmd.Wrap(&popCommand{
		clientStore: clientStore,
		history:     history,
	})
}

type popCommand struct {
	modelcmd.ModelCommandBase

	clientStore jujuclient.ClientStore
	history     *history
	store       bool
}

// SetFlags implements Command.SetFlags.
func (c *popCommand) SetFlags(f *gnuflag.FlagSet) {
	c.ModelCommandBase.SetFlags(f)

	f.BoolVar(&c.store, "store", false, "store pop in history to allow flip-flopping")
}

// Init implements Command.Init.
func (c *popCommand) Init(args []string) (err error) {
	c.SetClientStore(c.clientStore)
	if len(args) != 0 {
		return errors.New("expects no arguments")
	}
	return nil
}

// Info implements Command.Info.
func (c *popCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:    "pop",
		Purpose: "pop moves to a model from the stash history",
		Doc: `
Pop moves to a model from the stash history that was put in last.

Examples:
	juju stash pop
	juju stash pop --store

See:
	juju stash push
	juju stash list
	juju switch
`,
	})
}

// Run implements Command.Run.
func (c *popCommand) Run(ctx *cmd.Context) error {
	modelName, _, err := c.ModelDetails()
	if err != nil {
		return errors.Trace(err)
	}
	snapshot, err := c.history.Pop()
	if err != nil {
		return errors.Trace(err)
	}
	store := modelcmd.QualifyingClientStore{ClientStore: c.clientStore}
	if err := store.SetCurrentModel(snapshot.controllerName, snapshot.modelName); err != nil {
		// TODO: should we put the snapshot back?
		return errors.Trace(err)
	}
	logSwitch(ctx, modelName, snapshot.modelName)
	if !c.store {
		return nil
	}
	controllerName, err := modelcmd.DetermineCurrentController(store)
	if err != nil {
		return errors.Trace(err)
	}
	return c.history.Push(historySnapshot{
		controllerName: controllerName,
		modelName:      modelName,
	})
}
