// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package scriptlets

import (
	"context"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/worker/v5"
	"github.com/juju/worker/v5/dependency"

	"github.com/juju/juju/core/logger"
)

// GetScriptletServiceFunc is a helper function that extracts a
// ScriptletService from the dependency getter.
type GetScriptletServiceFunc func(getter dependency.Getter, name string) (ScriptletService, error)

// NewWorkerFunc is the function type used to create a new scriptlets worker.
type NewWorkerFunc func(WorkerConfig) (worker.Worker, error)

// ManifoldConfig defines the configuration for the scriptlets manifold.
type ManifoldConfig struct {
	DomainServicesName string

	Clock                clock.Clock
	Logger               logger.Logger
	NewWorker            NewWorkerFunc
	GetScriptletService  GetScriptletServiceFunc
}

// Validate ensures that the manifold configuration is valid.
func (cfg ManifoldConfig) Validate() error {
	if cfg.DomainServicesName == "" {
		return errors.NotValidf("empty DomainServicesName")
	}
	if cfg.Clock == nil {
		return errors.NotValidf("nil Clock")
	}
	if cfg.Logger == nil {
		return errors.NotValidf("nil Logger")
	}
	if cfg.NewWorker == nil {
		return errors.NotValidf("nil NewWorker")
	}
	if cfg.GetScriptletService == nil {
		return errors.NotValidf("nil GetScriptletService")
	}
	return nil
}

// Manifold returns a dependency manifold that runs the scriptlets worker.
func Manifold(config ManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: []string{
			config.DomainServicesName,
		},
		Start: func(ctx context.Context, getter dependency.Getter) (worker.Worker, error) {
			if err := config.Validate(); err != nil {
				return nil, errors.Trace(err)
			}

			scriptletService, err := config.GetScriptletService(getter, config.DomainServicesName)
			if err != nil {
				return nil, errors.Trace(err)
			}

			w, err := config.NewWorker(WorkerConfig{
				ScriptletService: scriptletService,
				Clock:            config.Clock,
				Logger:           config.Logger,
			})
			if err != nil {
				return nil, errors.Trace(err)
			}

			return w, nil
		},
	}
}
