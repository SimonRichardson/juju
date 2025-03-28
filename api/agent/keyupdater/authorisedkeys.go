// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package keyupdater

import (
	"context"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/api/base"
	apiwatcher "github.com/juju/juju/api/watcher"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/rpc/params"
)

// Option is a function that can be used to configure a Client.
type Option = base.Option

// WithTracer returns an Option that configures the Client to use the
// supplied tracer.
var WithTracer = base.WithTracer

// Client provides access to a worker's client facade.
type Client struct {
	facade base.FacadeCaller
}

// NewClient returns a version of the state that provides functionality required by the worker.
func NewClient(caller base.APICaller, options ...Option) *Client {
	return &Client{base.NewFacadeCaller(caller, "KeyUpdater", options...)}
}

// AuthorisedKeys returns the authorised ssh keys for the machine specified by machineTag.
func (c *Client) AuthorisedKeys(ctx context.Context, tag names.MachineTag) ([]string, error) {
	var results params.StringsResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: tag.String()}},
	}
	err := c.facade.FacadeCall(ctx, "AuthorisedKeys", args, &results)
	if err != nil {
		// TODO: Not directly tested
		return nil, err
	}
	if len(results.Results) != 1 {
		// TODO: Not directly tested
		return nil, errors.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if err := result.Error; err != nil {
		return nil, err
	}
	return result.Result, nil
}

// WatchAuthorisedKeys returns a notify watcher that looks for changes in the
// authorised ssh keys for the machine specified by machineTag.
func (c *Client) WatchAuthorisedKeys(ctx context.Context, tag names.MachineTag) (watcher.NotifyWatcher, error) {
	var results params.NotifyWatchResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: tag.String()}},
	}
	err := c.facade.FacadeCall(ctx, "WatchAuthorisedKeys", args, &results)
	if err != nil {
		// TODO: Not directly tested
		return nil, err
	}
	if len(results.Results) != 1 {
		// TODO: Not directly tested
		return nil, errors.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		//  TODO: Not directly tested
		return nil, result.Error
	}
	w := apiwatcher.NewNotifyWatcher(c.facade.RawAPICaller(), result)
	return w, nil
}
