// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package applicationscaler

import (
	"context"

	"github.com/juju/errors"
	"github.com/juju/worker/v4"

	"github.com/juju/juju/core/watcher"
)

// Facade defines the capabilities required by the worker.
type Facade interface {

	// Watch returns a StringsWatcher reporting names of
	// applications which may have insufficient units.
	Watch(context.Context) (watcher.StringsWatcher, error)

	// Rescale scales up any named application observed to be
	// running too few units.
	Rescale(ctx context.Context, applications []string) error
}

// Config defines a worker's dependencies.
type Config struct {
	Facade Facade
}

// Validate returns an error if the config can't be expected
// to run a functional worker.
func (config Config) Validate() error {
	if config.Facade == nil {
		return errors.NotValidf("nil Facade")
	}
	return nil
}

// New returns a worker that will attempt to rescale any
// applications that might be undersized.
func New(config Config) (worker.Worker, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}
	swConfig := watcher.StringsConfig{
		Handler: &handler{config: config},
	}
	return watcher.NewStringsWorker(swConfig)
}

// handler implements watcher.StringsHandler, backed by the
// configured facade.
type handler struct {
	config Config
}

// SetUp is part of the watcher.StringsHandler interface.
func (handler *handler) SetUp(ctx context.Context) (watcher.StringsWatcher, error) {
	return handler.config.Facade.Watch(ctx)
}

// Handle is part of the watcher.StringsHandler interface.
func (handler *handler) Handle(ctx context.Context, applications []string) error {
	return handler.config.Facade.Rescale(ctx, applications)
}

// TearDown is part of the watcher.StringsHandler interface.
func (handler *handler) TearDown() error {
	return nil
}
