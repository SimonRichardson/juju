// Copyright 2016 Canonical Ltd.
// Copyright 2016 Cloudbase Solutions
// Licensed under the AGPLv3, see LICENCE file for details.

package retrystrategy

import (
	"context"
	"fmt"

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

// Client provides access to the retry strategy api
type Client struct {
	facade base.FacadeCaller
}

// NewClient creates a client for accessing the retry strategy api
func NewClient(apiCaller base.APICaller, options ...Option) *Client {
	return &Client{base.NewFacadeCaller(apiCaller, "RetryStrategy", options...)}
}

// RetryStrategy returns the configuration for the agent specified by the agentTag.
func (c *Client) RetryStrategy(ctx context.Context, agentTag names.Tag) (params.RetryStrategy, error) {
	var results params.RetryStrategyResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: agentTag.String()}},
	}
	err := c.facade.FacadeCall(ctx, "RetryStrategy", args, &results)
	if err != nil {
		return params.RetryStrategy{}, errors.Trace(err)
	}
	if len(results.Results) != 1 {
		return params.RetryStrategy{}, fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return params.RetryStrategy{}, errors.Trace(result.Error)
	}
	return *result.Result, nil
}

// WatchRetryStrategy returns a notify watcher that looks for changes in the
// retry strategy config for the agent specified by agentTag
// Right now only the boolean that decides whether we retry can be modified.
func (c *Client) WatchRetryStrategy(ctx context.Context, agentTag names.Tag) (watcher.NotifyWatcher, error) {
	var results params.NotifyWatchResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: agentTag.String()}},
	}
	err := c.facade.FacadeCall(ctx, "WatchRetryStrategy", args, &results)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(results.Results) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, errors.Trace(result.Error)
	}
	w := apiwatcher.NewNotifyWatcher(c.facade.RawAPICaller(), result)
	return w, nil
}
