// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package bundle

import (
	"context"

	"github.com/juju/errors"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/rpc/params"
)

// Option is a function that can be used to configure a Client.
type Option = base.Option

// WithTracer returns an Option that configures the Client to use the
// supplied tracer.
var WithTracer = base.WithTracer

// Client allows access to the bundle API end point.
type Client struct {
	base.ClientFacade
	facade base.FacadeCaller
}

// NewClient creates a new client for accessing the bundle api.
func NewClient(st base.APICallCloser, options ...Option) *Client {
	frontend, backend := base.NewClientFacade(st, "Bundle", options...)
	return &Client{
		ClientFacade: frontend,
		facade:       backend}
}

// GetChangesMapArgs returns back the changes for a given bundle that need to be
// applied, with the args of a method as a map.
// NOTE(jack-w-shaw) This client method is currently unused. It's being kept in
// incase it's used in the future. We may want to re-evaluate in future
func (c *Client) GetChangesMapArgs(ctx context.Context, bundleURL, bundleDataYAML string) (params.BundleChangesMapArgsResults, error) {
	var result params.BundleChangesMapArgsResults
	if err := c.facade.FacadeCall(ctx, "GetChangesMapArgs", params.BundleChangesParams{
		BundleURL:      bundleURL,
		BundleDataYAML: bundleDataYAML,
	}, &result); err != nil {
		return result, errors.Trace(err)
	}
	return result, nil
}

// ExportBundle exports the current model configuration.
func (c *Client) ExportBundle(ctx context.Context, includeDefaults bool) (string, error) {
	var result params.StringResult
	arg := params.ExportBundleParams{
		IncludeCharmDefaults: includeDefaults,
	}
	if err := c.facade.FacadeCall(ctx, "ExportBundle", arg, &result); err != nil {
		return "", errors.Trace(err)
	}

	if result.Error != nil {
		return "", errors.Trace(result.Error)
	}

	return result.Result, nil
}
