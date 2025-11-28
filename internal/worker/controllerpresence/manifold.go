// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package controllerpresence

import (
	"context"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/worker/v4"
	"github.com/juju/worker/v4/dependency"

	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/internal/worker/apiremotecaller"
)

// ManifoldConfig defines the names of the manifolds on which a Manifold will
// depend.
type ManifoldConfig struct {
	APIRemoteCallerName string

	Logger logger.Logger
	Clock  clock.Clock

	NewWorker func(WorkerConfig) (worker.Worker, error)
}

// Validate validates the manifold configuration.
func (config ManifoldConfig) Validate() error {
	if config.APIRemoteCallerName == "" {
		return errors.NotValidf("empty APIRemoteCallerName")
	}
	return nil
}

// Manifold returns a dependency manifold that runs an API remote caller worker,
// using the resource names defined in the supplied config.
func Manifold(config ManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: []string{
			config.APIRemoteCallerName,
		},
		Start: func(ctx context.Context, getter dependency.Getter) (worker.Worker, error) {
			if err := config.Validate(); err != nil {
				return nil, errors.Trace(err)
			}

			var apiRemoteCallers apiremotecaller.APIRemoteCallers
			if err := getter.Get(config.APIRemoteCallerName, &apiRemoteCallers); err != nil {
				return nil, errors.Trace(err)
			}

			cfg := WorkerConfig{
				APIRemoteCallers: apiRemoteCallers,
				Logger:           config.Logger,
				Clock:            config.Clock,
			}

			w, err := config.NewWorker(cfg)
			if err != nil {
				return nil, errors.Trace(err)
			}
			return w, nil
		},
	}
}
