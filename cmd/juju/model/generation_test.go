// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"context"

	"github.com/juju/juju/api/jujuclient/jujuclienttesting"
	"github.com/juju/juju/cmd/cmd"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/rpc/params"
)

type fakeGenerationAPI struct {
	closed int
	err    error

	branchName string
	entities   []string
	numUnits   int

	generationID int
	commits      []params.Generation
	commit       params.Generation
}

func (f *fakeGenerationAPI) Close() error {
	f.closed++
	return nil
}

func (f *fakeGenerationAPI) AddBranch(_ context.Context, branchName string) error {
	f.branchName = branchName
	return f.err
}

func (f *fakeGenerationAPI) TrackBranch(
	_ context.Context, branchName string, entities []string, numUnits int,
) error {
	f.branchName = branchName
	f.entities = entities
	f.numUnits = numUnits
	return f.err
}

func (f *fakeGenerationAPI) CommitBranch(_ context.Context, branchName string) (int, error) {
	f.branchName = branchName
	return f.generationID, f.err
}

func (f *fakeGenerationAPI) AbortBranch(_ context.Context, branchName string) error {
	f.branchName = branchName
	return f.err
}

func (f *fakeGenerationAPI) ListCommits(context.Context) ([]params.Generation, error) {
	return f.commits, f.err
}

func (f *fakeGenerationAPI) ShowCommit(_ context.Context, generationID int) (params.Generation, error) {
	f.generationID = generationID
	return f.commit, f.err
}

func commandBase(api generationAPI) generationCommandBase {
	return generationCommandBase{api: api}
}

func wrapGenerationCommand(command modelcmd.ModelCommand) cmd.Command {
	command.SetClientStore(jujuclienttesting.MinimalStore())
	return modelcmd.Wrap(command)
}
