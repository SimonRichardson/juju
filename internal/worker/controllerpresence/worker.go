// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package controllerpresence

import (
	"context"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/worker/v4"
	"github.com/juju/worker/v4/catacomb"

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
	catacomb catacomb.Catacomb

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

	err := catacomb.Invoke(catacomb.Plan{
		Name: "controller-presence",
		Site: &w.catacomb,
		Work: w.loop,
	})
	if err != nil {
		return nil, errors.Trace(err)
	}

	return w, nil
}

// Kill is part of the worker.Worker interface.
func (w *controllerWorker) Kill() {
	w.catacomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (w *controllerWorker) Wait() error {
	return w.catacomb.Wait()
}

// Report returns a map of internal state for the controllerWorker.
func (w *controllerWorker) Report() map[string]any {
	report := make(map[string]any)
	return report
}

func (w *controllerWorker) loop() error {
	ctx := w.catacomb.Context(context.Background())

	remoteCallers := w.cfg.APIRemoteCallers

	subscriber := remoteCallers.SubscribeChanges()
	defer subscriber.Close()

	for {
		select {
		case <-w.catacomb.Dying():
			return nil

		case _, ok := <-subscriber.Changes():
			if !ok {
				return nil
			}

			// Remove all existing runners, we will recreate them as needed.

			callers, err := remoteCallers.GetAPIRemotes()
			if err != nil {
				return errors.Trace(err)
			}

			for _, caller := range callers {
				if err := caller.Connection(ctx, w.connection); err != nil {
					return errors.Trace(err)
				}
			}
		}
	}
}

func (w *controllerWorker) connection(ctx context.Context, conn api.Connection) error {
	// Start a runner that will monitory the connection. If it's borken, remove
	// the machine presence.
	// If the context is done... whilst we're starting the worker, just exit.
	// If the runner already exists, just kill it and create a new one.
	return nil
}
