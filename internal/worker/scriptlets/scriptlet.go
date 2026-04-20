// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package scriptlets

import (
	"context"

	"github.com/juju/errors"
	"github.com/juju/worker/v5/catacomb"

	"github.com/juju/juju/core/logger"
	"go.starlark.net/starlark"
)

// scriptletRunnerConfig holds the configuration for a single scriptlet
// execution worker.
type scriptletRunnerConfig struct {
	ID     string
	Script string
	Logger logger.Logger
}

func (c *scriptletRunnerConfig) validate() error {
	if c.ID == "" {
		return errors.NotValidf("empty ID")
	}
	if c.Script == "" {
		return errors.NotValidf("empty Script")
	}
	if c.Logger == nil {
		return errors.NotValidf("nil Logger")
	}
	return nil
}

// scriptletRunner is a worker that executes a single Starlark script.
type scriptletRunner struct {
	catacomb catacomb.Catacomb
	cfg      scriptletRunnerConfig
}

func newScriptletRunner(cfg scriptletRunnerConfig) (*scriptletRunner, error) {
	if err := cfg.validate(); err != nil {
		return nil, errors.Trace(err)
	}

	w := &scriptletRunner{
		cfg: cfg,
	}

	if err := catacomb.Invoke(catacomb.Plan{
		Name: cfg.ID,
		Site: &w.catacomb,
		Work: w.loop,
	}); err != nil {
		return nil, errors.Trace(err)
	}

	return w, nil
}

// Kill is part of the worker.Worker interface.
func (w *scriptletRunner) Kill() {
	w.catacomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (w *scriptletRunner) Wait() error {
	return w.catacomb.Wait()
}

func (w *scriptletRunner) loop() error {
	ctx, cancel := w.scopedContext()
	defer cancel()

	w.cfg.Logger.Infof(ctx, "executing scriptlet %q", w.cfg.ID)

	thread := &starlark.Thread{
		Name: w.cfg.ID,
		Print: func(_ *starlark.Thread, msg string) {
			w.cfg.Logger.Infof(ctx, "scriptlet %q: %s", w.cfg.ID, msg)
		},
	}

	// Execute the script in the Starlark interpreter.
	globals, err := starlark.ExecFile(thread, w.cfg.ID, w.cfg.Script, nil)
	if err != nil {
		return errors.Annotatef(err, "executing scriptlet %q", w.cfg.ID)
	}

	w.cfg.Logger.Debugf(ctx, "scriptlet %q completed with %d globals", w.cfg.ID, len(globals))

	// Block until the worker is killed, keeping the child worker alive
	// so the runner can track it.
	select {
	case <-w.catacomb.Dying():
		return w.catacomb.ErrDying()
	}
}

func (w *scriptletRunner) scopedContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(w.catacomb.Context(context.Background()))
}
