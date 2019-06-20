// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.
package main

import (
	"github.com/juju/cmd"
	"github.com/juju/errors"

	jujucmd "github.com/juju/juju/cmd"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/jujuclient"
)

func newPushCommand(clientStore jujuclient.ClientStore, history *history) cmd.Command {
	return modelcmd.Wrap(&pushCommand{
		clientStore: clientStore,
		history:     history,
	})
}

type pushCommand struct {
	modelcmd.ModelCommandBase

	clientStore jujuclient.ClientStore
	history     *history
	target      string
}

// Init implements Command.Init.
func (c *pushCommand) Init(args []string) (err error) {
	c.SetClientStore(c.clientStore)
	c.target, err = cmd.ZeroOrOneArgs(args)
	return err
}

// Info implements Command.Info.
func (c *pushCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:    "push",
		Purpose: "push adds a model to the stash history",
		Doc: `
Push adds a model to the stash history

Examples:
	juju stash push mymodel

See:
	juju stash pop
	juju stash list
	juju switch
`,
	})
}

// Run implements Command.Run.
func (c *pushCommand) Run(ctx *cmd.Context) (err error) {
	store := modelcmd.QualifyingClientStore{ClientStore: c.clientStore}
	controllerName, err := modelcmd.DetermineCurrentController(store)
	if err != nil {
		return errors.Trace(err)
	}
	modelName, _, err := c.ModelDetails()
	if err != nil {
		return errors.Trace(err)
	}

	targetName, err := store.QualifiedModelName(controllerName, c.target)
	if err != nil {
		return errors.Trace(err)
	}

	if err := store.SetCurrentModel(controllerName, targetName); err != nil {
		return errors.Trace(err)
	}
	logSwitch(ctx, modelName, targetName)
	if modelName == targetName {
		return nil
	}
	return c.history.Push(historySnapshot{
		controllerName: controllerName,
		modelName:      modelName,
	})
}

func logSwitch(ctx *cmd.Context, oldName string, newName string) {
	if newName == oldName {
		ctx.Infof("%s (no change)", oldName)
	} else {
		ctx.Infof("%s -> %s", oldName, newName)
	}
}
