// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.
package main

import (
	"os"

	"github.com/juju/cmd"
	"github.com/juju/utils/featureflag"

	"github.com/juju/juju/juju"
	"github.com/juju/juju/juju/osenv"
	"github.com/juju/juju/jujuclient"
)

var stashDoc = `
Juju stash is used to move between models without having to specify model names
`

func Main(args []string) {
	ctx, err := cmd.DefaultContext()
	if err != nil {
		cmd.WriteError(os.Stderr, err)
		os.Exit(2)
	}
	if err := juju.InitJujuXDGDataHome(); err != nil {
		cmd.WriteError(ctx.Stderr, err)
		os.Exit(2)
	}
	os.Exit(cmd.Main(NewSuperCommand(ctx), ctx, args[1:]))
}

func NewSuperCommand(ctx *cmd.Context) cmd.Command {
	stashcmd := cmd.NewSuperCommand(cmd.SuperCommandParams{
		Name:        "stash",
		UsagePrefix: "juju",
		Doc:         stashDoc,
		Purpose:     "cmd line tool for moving easily between models",
		Log:         &cmd.Log{},
	})

	history, err := newHistory()
	if err != nil {
		cmd.WriteError(ctx.Stderr, err)
		os.Exit(2)
	}
	clientStore := jujuclient.NewFileClientStore()
	stashcmd.Register(newPushCommand(clientStore, history))
	stashcmd.Register(newPopCommand(clientStore, history))
	stashcmd.Register(newListCommand(history))
	return stashcmd
}

func init() {
	featureflag.SetFlagsFromEnvironment(osenv.JujuFeatureFlagEnvKey)
}

func main() {
	Main(os.Args)
}
