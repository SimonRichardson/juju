// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package common

import (
	"context"

	"github.com/juju/juju/api/base"
	apiwatcher "github.com/juju/juju/api/watcher"
	"github.com/juju/juju/controller"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/rpc/params"
)

// ControllerConfigAPI provides common client-side API functions
// to call into apiserver.common.ControllerConfig.
type ControllerConfigAPI struct {
	facade base.FacadeCaller
}

// NewControllerConfig creates a ControllerConfig on the specified facade,
// and uses this name when calling through the caller.
func NewControllerConfig(facade base.FacadeCaller) *ControllerConfigAPI {
	return &ControllerConfigAPI{facade}
}

// ControllerConfig returns the current controller configuration.
func (e *ControllerConfigAPI) ControllerConfig() (controller.Config, error) {
	var result params.ControllerConfigResult
	err := e.facade.FacadeCall(context.TODO(), "ControllerConfig", nil, &result)
	if err != nil {
		return nil, err
	}
	return controller.Config(result.Config), nil
}

// WatchControllerConfig provides a watcher for changes on controller config.
func (c *ControllerConfigAPI) WatchControllerConfig() (watcher.StringsWatcher, error) {
	var result params.StringsWatchResult
	if err := c.facade.FacadeCall(context.TODO(), "WatchControllerConfig", nil, &result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return apiwatcher.NewStringsWatcher(c.facade.RawAPICaller(), result), nil
}
