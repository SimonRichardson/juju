// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package controllerpresence

import (
	"context"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/worker/v4"
	"gopkg.in/tomb.v2"

	"github.com/juju/juju/api"
	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/internal/worker/apiremotecaller"
)

// WorkerConfig defines the configuration values that the pubsub worker needs
// to operate.
type WorkerConfig struct {
	APIRemoteCallers apiremotecaller.APIRemoteCallers
	Clock            clock.Clock
	Logger           logger.Logger
}

// Validate checks that all the values have been set.
func (c *WorkerConfig) Validate() error {
	if c.APIRemoteCallers == nil {
		return errors.NotValidf("missing APIRemoteCallers")
	}
	if c.Clock == nil {
		return errors.NotValidf("missing Clock")
	}
	if c.Logger == nil {
		return errors.NotValidf("missing Logger")
	}
	return nil
}

type controllerWorker struct {
	tomb tomb.Tomb

	cfg WorkerConfig
}

// NewWorker exposes the remoteWorker as a Worker.
func NewWorker(config WorkerConfig) (worker.Worker, error) {
	return newWorker(config)
}

func newWorker(config WorkerConfig) (worker.Worker, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}

	w := &controllerWorker{
		cfg: config,
	}

	w.tomb.Go(w.loop)

	return w, nil
}

// Kill is part of the worker.Worker interface.
func (w *controllerWorker) Kill() {
	w.tomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (w *controllerWorker) Wait() error {
	return w.tomb.Wait()
}

// Report returns a map of internal state for the controllerWorker.
func (w *controllerWorker) Report() map[string]any {
	report := make(map[string]any)
	return report
}

func (w *controllerWorker) loop() error {
	ctx := w.tomb.Context(context.Background())

	remoteCallers := w.cfg.APIRemoteCallers

	subscriber, err := remoteCallers.SubscribeChanges()
	if err != nil {
		return errors.Trace(err)
	}
	defer subscriber.Close()

	for {
		select {
		case <-w.tomb.Dying():
			return nil

		case _, ok := <-subscriber.Changes():
			if !ok {
				return nil
			}

			callers, err := remoteCallers.GetAPIRemotes()
			if err != nil {
				return errors.Trace(err)
			}

			for _, caller := range callers {
				if err := caller.Connection(ctx, func(ctx context.Context, c api.Connection) error {
					select {
					case <-w.tomb.Dying():
						return nil
					case <-c.Broken():
						return nil
					}
				}); err != nil {
					return errors.Trace(err)
				}
			}
		}
	}
}
