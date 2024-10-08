// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package statushistory

import (
	"context"
	"time"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/rpc/params"
)

// Option is a function that can be used to configure a Client.
type Option = base.Option

// WithTracer returns an Option that configures the Client to use the
// supplied tracer.
var WithTracer = base.WithTracer

const apiName = "StatusHistory"

// Client allows calls to "StatusHistory" endpoints.
type Client struct {
	facade base.FacadeCaller
}

// NewClient returns a status "StatusHistory" Client.
func NewClient(caller base.APICaller, options ...Option) *Client {
	facadeCaller := base.NewFacadeCaller(caller, apiName, options...)
	return &Client{facade: facadeCaller}
}

// Prune calls "StatusHistory.Prune"
func (s *Client) Prune(ctx context.Context, maxHistoryTime time.Duration, maxHistoryMB int) error {
	p := params.StatusHistoryPruneArgs{
		MaxHistoryTime: maxHistoryTime,
		MaxHistoryMB:   maxHistoryMB,
	}
	return s.facade.FacadeCall(ctx, "Prune", p, nil)
}
