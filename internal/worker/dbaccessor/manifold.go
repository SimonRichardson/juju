// Copyright 2021 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package dbaccessor

import (
	"context"
	"path"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/worker/v4"
	"github.com/juju/worker/v4/dependency"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/juju/juju/agent"
	coredatabase "github.com/juju/juju/core/database"
	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/internal/database"
	"github.com/juju/juju/internal/database/app"
	"github.com/juju/juju/internal/worker/common"
	"github.com/juju/juju/internal/worker/controlleragentconfig"
)

// NewDBWorkerFunc creates a tracked db worker.
type NewDBWorkerFunc func(context.Context, DBApp, string, ...TrackedDBWorkerOption) (TrackedDB, error)

// NewNodeManagerFunc creates a NodeManager
type NewNodeManagerFunc func(agent.Config, logger.Logger, coredatabase.SlowQueryLogger) NodeManager

// ManifoldConfig contains:
// - The names of other manifolds on which the DB accessor depends.
// - Other dependencies from ManifoldsConfig required by the worker.
type ManifoldConfig struct {
	AgentName                 string
	QueryLoggerName           string
	ControllerAgentConfigName string
	Clock                     clock.Clock
	Logger                    logger.Logger
	LogDir                    string
	PrometheusRegisterer      prometheus.Registerer
	NewApp                    func(string, ...app.Option) (DBApp, error)
	NewDBWorker               NewDBWorkerFunc
	NewNodeManager            NewNodeManagerFunc
	NewMetricsCollector       func() *Collector
}

func (cfg ManifoldConfig) Validate() error {
	if cfg.AgentName == "" {
		return errors.NotValidf("empty AgentName")
	}
	if cfg.QueryLoggerName == "" {
		return errors.NotValidf("empty QueryLoggerName")
	}
	if cfg.ControllerAgentConfigName == "" {
		return errors.NotValidf("empty ControllerAgentConfigName")
	}
	if cfg.Clock == nil {
		return errors.NotValidf("nil Clock")
	}
	if cfg.Logger == nil {
		return errors.NotValidf("nil Logger")
	}
	if cfg.LogDir == "" {
		return errors.NotValidf("empty LogDir")
	}
	if cfg.PrometheusRegisterer == nil {
		return errors.NotValidf("nil PrometheusRegisterer")
	}
	if cfg.NewApp == nil {
		return errors.NotValidf("nil NewApp")
	}
	if cfg.NewDBWorker == nil {
		return errors.NotValidf("nil NewDBWorker")
	}
	if cfg.NewNodeManager == nil {
		return errors.NotValidf("nil NewNodeManager")
	}
	if cfg.NewMetricsCollector == nil {
		return errors.NotValidf("nil NewMetricsCollector")
	}
	return nil
}

// Manifold returns a dependency manifold that runs the dbaccessor
// worker, using the resource names defined in the supplied config.
func Manifold(config ManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: []string{
			config.AgentName,
			config.QueryLoggerName,
			config.ControllerAgentConfigName,
		},
		Output: dbAccessorOutput,
		Start: func(ctx context.Context, getter dependency.Getter) (worker.Worker, error) {
			if err := config.Validate(); err != nil {
				return nil, errors.Trace(err)
			}

			var thisAgent agent.Agent
			if err := getter.Get(config.AgentName, &thisAgent); err != nil {
				return nil, err
			}
			agentConfig := thisAgent.CurrentConfig()
			controllerID := agentConfig.Tag().Id()
			configPath := path.Join(agentConfig.DataDir(), "agents", "controller-"+controllerID, "controller.conf")
			controllerConf := controllerConfigReader{configPath: configPath}

			var controllerConfigWatcher controlleragentconfig.ConfigWatcher
			if err := getter.Get(config.ControllerAgentConfigName, &controllerConfigWatcher); err != nil {
				return nil, err
			}

			// Register the metrics collector against the prometheus register.
			metricsCollector := config.NewMetricsCollector()
			if err := config.PrometheusRegisterer.Register(metricsCollector); err != nil {
				return nil, errors.Trace(err)
			}

			var slowQueryLogger coredatabase.SlowQueryLogger
			if err := getter.Get(config.QueryLoggerName, &slowQueryLogger); err != nil {
				config.PrometheusRegisterer.Unregister(metricsCollector)
				return nil, err
			}

			cfg := WorkerConfig{
				NodeManager:             config.NewNodeManager(agentConfig, config.Logger, slowQueryLogger),
				Clock:                   config.Clock,
				ControllerID:            controllerID,
				MetricsCollector:        metricsCollector,
				Logger:                  config.Logger,
				NewApp:                  config.NewApp,
				NewDBWorker:             config.NewDBWorker,
				ControllerConfigWatcher: controllerConfigWatcher,
				ClusterConfig:           controllerConf,
			}

			w, err := NewWorker(cfg)
			if err != nil {
				config.PrometheusRegisterer.Unregister(metricsCollector)
				controllerConfigWatcher.Unsubscribe()
				return nil, errors.Trace(err)
			}
			return common.NewCleanupWorker(w, func() {
				// Clean up the metrics for the worker, so the next time a
				// worker is created we can safely register the metrics again.
				config.PrometheusRegisterer.Unregister(metricsCollector)
				controllerConfigWatcher.Unsubscribe()
			}), nil
		},
	}
}

func dbAccessorOutput(in worker.Worker, out interface{}) error {
	if w, ok := in.(*common.CleanupWorker); ok {
		in = w.Worker
	}
	w, ok := in.(*dbWorker)
	if !ok {
		return errors.Errorf("expected input of type dbWorker, got %T", in)
	}

	switch out := out.(type) {
	case *coredatabase.DBGetter:
		var target coredatabase.DBGetter = w
		*out = target
	case *coredatabase.DBDeleter:
		var target coredatabase.DBDeleter = w
		*out = target
	default:
		return errors.Errorf("expected output of *database.DBGetter or *database.DBDeleter, got %T", out)
	}
	return nil
}

// IAASNodeManager returns a NodeManager that is configured to use
// the cloud-local TLS terminated address for Dqlite.
func IAASNodeManager(cfg agent.Config, logger logger.Logger, slowQueryLogger coredatabase.SlowQueryLogger) NodeManager {
	return database.NewNodeManager(cfg, true, logger, slowQueryLogger)
}

// CAASNodeManager returns a NodeManager that is configured to use
// the loopback address for Dqlite.
func CAASNodeManager(cfg agent.Config, logger logger.Logger, slowQueryLogger coredatabase.SlowQueryLogger) NodeManager {
	return database.NewNodeManager(cfg, true, logger, slowQueryLogger)
}
