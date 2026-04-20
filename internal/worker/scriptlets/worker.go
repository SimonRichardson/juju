// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package scriptlets

import (
	"context"
	"time"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/worker/v5"
	"github.com/juju/worker/v5/catacomb"

	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/watcher"
	internalworker "github.com/juju/juju/internal/worker"
)

const (
	// States which report the state of the worker.
	stateStarted = "started"
)

// ScriptletService provides the domain operations required by the scriptlets
// worker.
type ScriptletService interface {
	// WatchScriptlets returns a watcher that emits the IDs of scriptlets
	// that have been created, changed, or removed.
	WatchScriptlets(ctx context.Context) (watcher.StringsWatcher, error)

	// GetScript returns the Starlark source for the scriptlet identified
	// by the given ID.
	GetScript(ctx context.Context, id string) (string, error)
}

// WorkerConfig encapsulates the configuration options for the scriptlets
// worker.
type WorkerConfig struct {
	ScriptletService ScriptletService
	Clock            clock.Clock
	Logger           logger.Logger
}

// Validate ensures that the config values are valid.
func (c *WorkerConfig) Validate() error {
	if c.ScriptletService == nil {
		return errors.NotValidf("nil ScriptletService")
	}
	if c.Clock == nil {
		return errors.NotValidf("nil Clock")
	}
	if c.Logger == nil {
		return errors.NotValidf("nil Logger")
	}
	return nil
}

type scriptletsWorker struct {
	internalStates chan string
	cfg            WorkerConfig
	catacomb       catacomb.Catacomb
	runner         *worker.Runner
}

// NewWorker creates a new scriptlets worker.
func NewWorker(cfg WorkerConfig) (worker.Worker, error) {
	return newWorker(cfg, nil)
}

func newWorker(cfg WorkerConfig, internalStates chan string) (*scriptletsWorker, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Trace(err)
	}

	runner, err := worker.NewRunner(worker.RunnerParams{
		Name:  "scriptlets",
		Clock: cfg.Clock,
		IsFatal: func(err error) bool {
			return false
		},
		ShouldRestart: internalworker.ShouldRunnerRestart,
		RestartDelay:  time.Second * 10,
		Logger:        internalworker.WrapLogger(cfg.Logger),
	})
	if err != nil {
		return nil, errors.Trace(err)
	}

	w := &scriptletsWorker{
		internalStates: internalStates,
		cfg:            cfg,
		runner:         runner,
	}

	if err := catacomb.Invoke(catacomb.Plan{
		Name: "scriptlets",
		Site: &w.catacomb,
		Work: w.loop,
		Init: []worker.Worker{
			w.runner,
		},
	}); err != nil {
		return nil, errors.Trace(err)
	}

	return w, nil
}

// Kill is part of the worker.Worker interface.
func (w *scriptletsWorker) Kill() {
	w.catacomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (w *scriptletsWorker) Wait() error {
	return w.catacomb.Wait()
}

func (w *scriptletsWorker) loop() error {
	w.reportInternalState(stateStarted)

	ctx, cancel := w.scopedContext()
	defer cancel()

	scriptletWatcher, err := w.cfg.ScriptletService.WatchScriptlets(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	if err := w.catacomb.Add(scriptletWatcher); err != nil {
		return errors.Trace(err)
	}

	for {
		select {
		case <-w.catacomb.Dying():
			return w.catacomb.ErrDying()

		case ids, ok := <-scriptletWatcher.Changes():
			if !ok {
				return errors.New("scriptlet watcher closed")
			}
			if err := w.handleScriptletChanges(ctx, ids); err != nil {
				return errors.Trace(err)
			}
		}
	}
}

func (w *scriptletsWorker) handleScriptletChanges(ctx context.Context, ids []string) error {
	for _, id := range ids {
		w.cfg.Logger.Debugf(ctx, "scriptlet change for %q", id)

		if err := w.ensureScriptletWorker(ctx, id); err != nil {
			return errors.Annotatef(err, "ensuring scriptlet worker for %q", id)
		}
	}
	return nil
}

func (w *scriptletsWorker) ensureScriptletWorker(ctx context.Context, id string) error {
	// Check if a worker already exists for this scriptlet. If it does,
	// stop it so we can restart with the latest script.
	if _, err := w.runner.Worker(id, w.catacomb.Dying()); err == nil {
		if err := w.runner.StopAndRemoveWorker(id, ctx.Done()); err != nil && !errors.Is(err, errors.NotFound) {
			return errors.Trace(err)
		}
	}

	err := w.runner.StartWorker(ctx, id, func(ctx context.Context) (worker.Worker, error) {
		script, err := w.cfg.ScriptletService.GetScript(ctx, id)
		if err != nil {
			return nil, errors.Annotatef(err, "getting script for scriptlet %q", id)
		}

		return newScriptletRunner(scriptletRunnerConfig{
			ID:     id,
			Script: script,
			Logger: w.cfg.Logger,
		})
	})
	if errors.Is(err, errors.AlreadyExists) {
		return nil
	}
	return errors.Trace(err)
}

func (w *scriptletsWorker) scopedContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(w.catacomb.Context(context.Background()))
}

func (w *scriptletsWorker) reportInternalState(state string) {
	if w.internalStates != nil {
		select {
		case w.internalStates <- state:
		default:
		}
	}
}
