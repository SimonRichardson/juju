// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc

import (
	"github.com/juju/errors"
	"github.com/juju/gnuflag"

	jujucmd "github.com/juju/juju/cmd"
	"github.com/juju/juju/cmd/cmd"
	"github.com/juju/juju/rpc/params"
)

// SnapshotContext provides the snapshot attached to a holistic hook context.
type SnapshotContext interface {
	UnitSnapshot() (params.UnitSnapshot, error)
}

// UnitSnapshotCommand prints the state captured by the holistic uniter for
// the current hook. The state is never written to disk or an environment
// variable.
type UnitSnapshotCommand struct {
	cmd.CommandBase
	ctx SnapshotContext
	out cmd.Output
}

// NewUnitSnapshotCommand creates the unit-snapshot hook command.
func NewUnitSnapshotCommand(ctx Context) (cmd.Command, error) {
	snapshotContext, ok := ctx.(SnapshotContext)
	if !ok {
		return nil, errors.NotSupportedf("unit snapshot is unavailable")
	}
	return &UnitSnapshotCommand{ctx: snapshotContext}, nil
}

// Info describes the unit-snapshot command.
func (c *UnitSnapshotCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:    "unit-snapshot",
		Purpose: "Prints the holistic unit snapshot for the current hook.",
	})
}

// SetFlags adds output-format flags.
func (c *UnitSnapshotCommand) SetFlags(f *gnuflag.FlagSet) {
	c.out.AddFlags(f, "yaml", map[string]cmd.Formatter{
		"yaml": cmd.FormatYaml,
		"json": cmd.FormatJson,
	})
}

// Init validates command arguments.
func (c *UnitSnapshotCommand) Init(args []string) error {
	return cmd.CheckEmpty(args)
}

// Run writes the snapshot captured for this hook execution.
func (c *UnitSnapshotCommand) Run(ctx *cmd.Context) error {
	snapshot, err := c.ctx.UnitSnapshot()
	if err != nil {
		return errors.Trace(err)
	}
	return c.out.Write(ctx, snapshot)
}
