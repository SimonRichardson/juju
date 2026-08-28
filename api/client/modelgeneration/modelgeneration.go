// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelgeneration

import (
	"context"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/rpc/params"
)

// Option is a function that configures a Client.
type Option = base.Option

// WithTracer returns an Option that configures the Client to use the supplied
// tracer.
var WithTracer = base.WithTracer

// Client provides access to model generation operations.
type Client struct {
	base.ClientFacade
	facade base.FacadeCaller
}

// NewClient creates a model generation client.
func NewClient(caller base.APICallCloser, options ...Option) *Client {
	frontend, backend := base.NewClientFacade(caller, "ModelGeneration", options...)
	return &Client{ClientFacade: frontend, facade: backend}
}

// AddBranch creates an in-flight branch.
func (c *Client) AddBranch(ctx context.Context, branchName string) error {
	var result params.ErrorResult
	if err := c.facade.FacadeCall(ctx, "AddBranch", branchArg(branchName), &result); err != nil {
		return errors.Trace(err)
	}
	if result.Error != nil {
		return errors.Trace(result.Error)
	}
	return nil
}

// TrackBranch enrolls units or all or some units of applications in a branch.
func (c *Client) TrackBranch(
	ctx context.Context, branchName string, entities []string, numUnits int,
) error {
	return c.updateTrackedUnits(ctx, "TrackBranch", branchName, entities, numUnits)
}

// UntrackBranch removes units or all or some units of applications from a
// branch.
func (c *Client) UntrackBranch(
	ctx context.Context, branchName string, entities []string, numUnits int,
) error {
	return c.updateTrackedUnits(ctx, "UntrackBranch", branchName, entities, numUnits)
}

func (c *Client) updateTrackedUnits(
	ctx context.Context, method, branchName string, entities []string, numUnits int,
) error {
	if len(entities) == 0 {
		return errors.New("no units or applications specified")
	}
	if numUnits > 0 && len(entities) > 1 {
		return errors.New("number of units can only be specified for one application")
	}

	arg := params.BranchTrackArg{
		BranchName: branchName,
		Entities:   make([]params.Entity, len(entities)),
		NumUnits:   numUnits,
	}
	for i, entity := range entities {
		switch {
		case names.IsValidApplication(entity):
			arg.Entities[i] = params.Entity{Tag: names.NewApplicationTag(entity).String()}
		case names.IsValidUnit(entity):
			arg.Entities[i] = params.Entity{Tag: names.NewUnitTag(entity).String()}
		default:
			return errors.Errorf("%q is not an application or a unit", entity)
		}
	}

	var result params.ErrorResults
	if err := c.facade.FacadeCall(ctx, method, arg, &result); err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(result.Combine())
}

// CommitBranch commits a branch to main and returns its generation id.
func (c *Client) CommitBranch(ctx context.Context, branchName string) (int, error) {
	var result params.IntResult
	if err := c.facade.FacadeCall(ctx, "CommitBranch", branchArg(branchName), &result); err != nil {
		return 0, errors.Trace(err)
	}
	if result.Error != nil {
		return 0, errors.Trace(result.Error)
	}
	return result.Result, nil
}

// AbortBranch aborts an in-flight branch without applying its changes.
func (c *Client) AbortBranch(ctx context.Context, branchName string) error {
	var result params.ErrorResult
	if err := c.facade.FacadeCall(ctx, "AbortBranch", branchArg(branchName), &result); err != nil {
		return errors.Trace(err)
	}
	if result.Error != nil {
		return errors.Trace(result.Error)
	}
	return nil
}

// HasActiveBranch reports whether the named branch is in flight.
func (c *Client) HasActiveBranch(ctx context.Context, branchName string) (bool, error) {
	var result params.BoolResult
	if err := c.facade.FacadeCall(ctx, "HasActiveBranch", branchArg(branchName), &result); err != nil {
		return false, errors.Trace(err)
	}
	if result.Error != nil {
		return false, errors.Trace(result.Error)
	}
	return result.Result, nil
}

// BranchInfo returns information about the requested in-flight branches. An
// empty branchNames slice requests all in-flight branches.
func (c *Client) BranchInfo(
	ctx context.Context, branchNames []string, detailed bool,
) ([]params.Generation, error) {
	arg := params.BranchInfoArgs{BranchNames: branchNames, Detailed: detailed}
	var result params.BranchResults
	if err := c.facade.FacadeCall(ctx, "BranchInfo", arg, &result); err != nil {
		return nil, errors.Trace(err)
	}
	if result.Error != nil {
		return nil, errors.Trace(result.Error)
	}
	return result.Generations, nil
}

// ListCommits returns committed generation history.
func (c *Client) ListCommits(ctx context.Context) ([]params.Generation, error) {
	var result params.BranchResults
	if err := c.facade.FacadeCall(ctx, "ListCommits", nil, &result); err != nil {
		return nil, errors.Trace(err)
	}
	if result.Error != nil {
		return nil, errors.Trace(result.Error)
	}
	return result.Generations, nil
}

// ShowCommit returns a committed generation by id.
func (c *Client) ShowCommit(ctx context.Context, generationID int) (params.Generation, error) {
	arg := params.GenerationId{GenerationId: generationID}
	var result params.GenerationResult
	if err := c.facade.FacadeCall(ctx, "ShowCommit", arg, &result); err != nil {
		return params.Generation{}, errors.Trace(err)
	}
	if result.Error != nil {
		return params.Generation{}, errors.Trace(result.Error)
	}
	return result.Generation, nil
}

func branchArg(branchName string) params.BranchArg {
	return params.BranchArg{BranchName: branchName}
}
