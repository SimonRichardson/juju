// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver

import (
	"context"
	"net/http"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/worker/v4"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/apiserver"
	"github.com/juju/juju/apiserver/apiserverhttp"
	"github.com/juju/juju/apiserver/authentication/jwt"
	"github.com/juju/juju/apiserver/authentication/macaroon"
	"github.com/juju/juju/core/auditlog"
	"github.com/juju/juju/core/changestream"
	"github.com/juju/juju/core/database"
	"github.com/juju/juju/core/flightrecorder"
	"github.com/juju/juju/core/lease"
	corelogger "github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/objectstore"
	"github.com/juju/juju/internal/jwtparser"
	"github.com/juju/juju/internal/services"
	"github.com/juju/juju/internal/worker/trace"
	"github.com/juju/juju/internal/worker/watcherregistry"
)

// Config is the configuration required for running an API server worker.
type Config struct {
	AgentConfig                       agent.Config
	Clock                             clock.Clock
	Mux                               *apiserverhttp.Mux
	LocalMacaroonAuthenticator        macaroon.LocalMacaroonAuthenticator
	JWTParser                         *jwtparser.Parser
	LeaseManager                      lease.Manager
	FlightRecorder                    flightrecorder.FlightRecorder
	LogSink                           corelogger.ModelLogger
	RegisterIntrospectionHTTPHandlers func(func(path string, _ http.Handler))
	UpgradeComplete                   func() bool
	GetAuditConfig                    func() auditlog.Config
	NewServer                         NewServerFunc
	MetricsCollector                  *apiserver.Collector
	EmbeddedCommand                   apiserver.ExecEmbeddedCommandFunc
	CharmhubHTTPClient                HTTPClient
	MacaroonHTTPClient                HTTPClient
	WatcherRegistryGetter             watcherregistry.WatcherRegistryGetter

	// DBGetter supplies WatchableDB implementations by namespace.
	DBGetter                changestream.WatchableDBGetter
	DBDeleter               database.DBDeleter
	DomainServicesGetter    services.DomainServicesGetter
	TracerGetter            trace.TracerGetter
	ObjectStoreGetter       objectstore.ObjectStoreGetter
	ControllerConfigService ControllerConfigService
	ModelService            ModelService
}

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// NewServerFunc is the type of function that will be used
// by the worker to create a new API server.
type NewServerFunc func(context.Context, apiserver.ServerConfig) (worker.Worker, error)

// Validate validates the API server configuration.
func (config Config) Validate() error {
	if config.AgentConfig == nil {
		return errors.NotValidf("nil AgentConfig")
	}
	if config.Clock == nil {
		return errors.NotValidf("nil Clock")
	}
	if config.Mux == nil {
		return errors.NotValidf("nil Mux")
	}
	if config.LocalMacaroonAuthenticator == nil {
		return errors.NotValidf("nil LocalMacaroonAuthenticator")
	}
	if config.LeaseManager == nil {
		return errors.NotValidf("nil LeaseManager")
	}
	if config.FlightRecorder == nil {
		return errors.NotValidf("nil FlightRecorder")
	}
	if config.RegisterIntrospectionHTTPHandlers == nil {
		return errors.NotValidf("nil RegisterIntrospectionHTTPHandlers")
	}
	if config.LogSink == nil {
		return errors.NotValidf("nil LogSink")
	}
	if config.UpgradeComplete == nil {
		return errors.NotValidf("nil UpgradeComplete")
	}
	if config.NewServer == nil {
		return errors.NotValidf("nil NewServer")
	}
	if config.MetricsCollector == nil {
		return errors.NotValidf("nil MetricsCollector")
	}
	if config.CharmhubHTTPClient == nil {
		return errors.NotValidf("nil CharmhubHTTPClient")
	}
	if config.MacaroonHTTPClient == nil {
		return errors.NotValidf("nil MacaroonHTTPClient")
	}
	if config.DomainServicesGetter == nil {
		return errors.NotValidf("nil DomainServicesGetter")
	}
	if config.DBGetter == nil {
		return errors.NotValidf("nil DBGetter")
	}
	if config.DBDeleter == nil {
		return errors.NotValidf("nil DBDeleter")
	}
	if config.TracerGetter == nil {
		return errors.NotValidf("nil TracerGetter")
	}
	if config.ObjectStoreGetter == nil {
		return errors.NotValidf("nil ObjectStoreGetter")
	}
	if config.ControllerConfigService == nil {
		return errors.NotValidf("nil ControllerConfigService")
	}
	if config.ModelService == nil {
		return errors.NotValidf("nil ModelService")
	}
	if config.JWTParser == nil {
		return errors.NotValidf("nil JWTParser")
	}
	if config.WatcherRegistryGetter == nil {
		return errors.NotValidf("nil WatcherRegistryGetter")
	}
	return nil
}

// NewWorker returns a new API server worker, with the given configuration.
func NewWorker(ctx context.Context, config Config) (worker.Worker, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}

	logSinkConfig, err := getLogSinkConfig(config.AgentConfig)
	if err != nil {
		return nil, errors.Annotate(err, "getting log sink config")
	}

	controllerConfig, err := config.ControllerConfigService.ControllerConfig(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "getting controller config")
	}

	controllerModel, err := config.ModelService.ControllerModel(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "getting controller model information")
	}

	observerFactory, err := newObserverFn(
		config.AgentConfig,
		config.DomainServicesGetter,
		config.Clock,
		config.MetricsCollector,
	)
	if err != nil {
		return nil, errors.Annotate(err, "cannot create RPC observer factory")
	}

	serverConfig := apiserver.ServerConfig{
		Clock:                         config.Clock,
		Tag:                           config.AgentConfig.Tag(),
		DataDir:                       config.AgentConfig.DataDir(),
		LogDir:                        config.AgentConfig.LogDir(),
		Mux:                           config.Mux,
		ControllerUUID:                controllerConfig.ControllerUUID(),
		ControllerModelUUID:           controllerModel.UUID,
		LocalMacaroonAuthenticator:    config.LocalMacaroonAuthenticator,
		JWTAuthenticator:              jwt.NewAuthenticator(config.JWTParser),
		UpgradeComplete:               config.UpgradeComplete,
		PublicDNSName:                 controllerConfig.AutocertDNSName(),
		AllowModelAccess:              controllerConfig.AllowModelAccess(),
		NewObserver:                   observerFactory,
		RegisterIntrospectionHandlers: config.RegisterIntrospectionHTTPHandlers,
		MetricsCollector:              config.MetricsCollector,
		LogSinkConfig:                 &logSinkConfig,
		GetAuditConfig:                config.GetAuditConfig,
		FlightRecorder:                config.FlightRecorder,
		LeaseManager:                  config.LeaseManager,
		ExecEmbeddedCommand:           config.EmbeddedCommand,
		LogSink:                       config.LogSink,
		CharmhubHTTPClient:            config.CharmhubHTTPClient,
		MacaroonHTTPClient:            config.MacaroonHTTPClient,
		DBGetter:                      config.DBGetter,
		DBDeleter:                     config.DBDeleter,
		DomainServicesGetter:          config.DomainServicesGetter,
		ControllerConfigService:       config.ControllerConfigService,
		TracerGetter:                  config.TracerGetter,
		ObjectStoreGetter:             config.ObjectStoreGetter,
		WatcherRegistryGetter:         config.WatcherRegistryGetter,
	}
	return config.NewServer(ctx, serverConfig)
}

func newServerShim(ctx context.Context, config apiserver.ServerConfig) (worker.Worker, error) {
	return apiserver.NewServer(ctx, config)
}

// NewMetricsCollector returns a new apiserver collector
func NewMetricsCollector() *apiserver.Collector {
	return apiserver.NewMetricsCollector()
}
