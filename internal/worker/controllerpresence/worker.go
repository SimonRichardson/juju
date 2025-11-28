// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package controllerpresence

import (
	"context"
	"strconv"

	"github.com/juju/clock"
	"github.com/juju/loggo"
	"github.com/juju/worker/v4"
	"gopkg.in/tomb.v2"

	"github.com/juju/juju/api"
	coreerrors "github.com/juju/juju/core/errors"
	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/machine"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/environs/bootstrap"
	"github.com/juju/juju/internal/errors"
	"github.com/juju/juju/internal/worker/apiremotecaller"
)

// StatusService is an interface that defines the status service required by the
// controller presence worker.
type StatusService interface {
	// DeleteMachinePresence removes the presence of the specified machine. If
	// the machine isn't found it ignores the error. The machine life is not
	// considered when making this query.
	DeleteMachinePresence(ctx context.Context, name machine.Name) error

	// DeleteUnitPresence removes the presence of the specified unit. If the
	// unit isn't found it ignores the error.
	// The unit life is not considered when making this query.
	DeleteUnitPresence(ctx context.Context, name coreunit.Name) error
}

// WorkerConfig defines the configuration values that the pubsub worker needs
// to operate.
type WorkerConfig struct {
	StatusService    StatusService
	APIRemoteCallers apiremotecaller.APIRemoteCallers
	Clock            clock.Clock
	Logger           logger.Logger
}

// Validate checks that all the values have been set.
func (c *WorkerConfig) Validate() error {
	if c.StatusService == nil {
		return errors.New("missing StatusService not valid").Add(coreerrors.NotValid)
	}
	if c.APIRemoteCallers == nil {
		return errors.New("missing APIRemoteCallers not valid").Add(coreerrors.NotValid)
	}
	if c.Clock == nil {
		return errors.New("missing Clock not valid").Add(coreerrors.NotValid)
	}
	if c.Logger == nil {
		return errors.New("missing Logger not valid").Add(coreerrors.NotValid)
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
		return nil, errors.Capture(err)
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
		return errors.Capture(err)
	}
	defer subscriber.Close()

	loggo.GetLogger("***").Criticalf(">>> 1")

	for {
		select {
		case <-w.tomb.Dying():
			return nil

		case _, ok := <-subscriber.Changes():
			if !ok {
				return nil
			}

			loggo.GetLogger("***").Criticalf(">>> 2")

			callers, err := remoteCallers.GetAPIRemotes()
			if err != nil {
				return errors.Capture(err)
			}

			loggo.GetLogger("***").Criticalf(">>> 3 %v %v", len(callers), callers)

			for _, caller := range callers {
				if err := caller.Connection(ctx, func(ctx context.Context, c api.Connection) error {
					// Start a runner to handle the connection, once it dies
					// we need to clean up presence information.
				}); err != nil {
					return errors.Capture(err)
				}
			}
		}
	}
}

func (w *controllerWorker) handleBrokenConnection(ctx context.Context, c api.Connection) error {
	// For a broken connection we want to remove any presence information for
	// machines and units that belong to this controller.
	controllerID := c.ControllerTag().Id()

	w.cfg.Logger.Criticalf(ctx, "API remote caller connection to controller %q broken", controllerID)

	machineName := machine.Name(controllerID)
	if err := w.cfg.StatusService.DeleteMachinePresence(ctx, machineName); err != nil {
		return errors.Errorf("deleting presence for machine %q: %w", machineName, err)
	}

	unitID, err := strconv.Atoi(controllerID)
	if err != nil {
		return errors.Errorf("parsing controller ID %q as unit number: %w", controllerID, err)
	}
	unitName, err := coreunit.NewNameFromParts(bootstrap.ControllerApplicationName, unitID)
	if err != nil {
		return errors.Errorf("creating unit name for controller ID %q: %w", controllerID, err)
	}
	if err := w.cfg.StatusService.DeleteUnitPresence(ctx, unitName); err != nil {
		return errors.Errorf("deleting presence for unit %q: %w", unitName, err)
	}

	return nil
}
