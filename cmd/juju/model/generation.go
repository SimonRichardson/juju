// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"context"

	"github.com/juju/errors"

	"github.com/juju/juju/api/client/modelgeneration"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/rpc/params"
)

type generationAPI interface {
	Close() error
	AddBranch(context.Context, string) error
	TrackBranch(context.Context, string, []string, int) error
	CommitBranch(context.Context, string) (int, error)
	AbortBranch(context.Context, string) error
	ListCommits(context.Context) ([]params.Generation, error)
	ShowCommit(context.Context, int) (params.Generation, error)
}

type generationCommandBase struct {
	modelcmd.ModelCommandBase
	api generationAPI
}

func (c *generationCommandBase) getGenerationAPI(ctx context.Context) (generationAPI, error) {
	if c.api != nil {
		return c.api, nil
	}
	root, err := c.NewAPIRoot(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "opening API connection")
	}
	return modelgeneration.NewClient(root), nil
}

func validateBranchName(name string) error {
	if name == "" {
		return errors.New("branch name cannot be empty")
	}
	if name == "main" {
		return errors.New(`branch name "main" is reserved`)
	}
	return nil
}
