// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/juju/clock"
	"github.com/juju/names/v6"
	"github.com/juju/worker/v5"
	"github.com/juju/worker/v5/dependency"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/agent/uniter"
	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/internal/worker/fortress"
)

// ManifoldConfig identifies the unit-scoped dependencies used by the OCI
// workload reconciler.
type ManifoldConfig struct {
	// AgentName supplies the assigned unit's agent configuration.
	AgentName string
	// APICallerName supplies the unit-authorised Juju API connection.
	APICallerName string
	// CharmDirName supplies read access to the uniter's charm directory.
	CharmDirName string
	// Clock schedules retries and periodic resource checks.
	Clock clock.Clock
	// Logger reports reconciliation failures.
	Logger logger.Logger
	// Puller defaults to the host containerd runtime when nil.
	Puller ImagePuller

	// NewClients builds unit-authorised clients from the shared API connection.
	// If nil, the production uniter and resource clients are used.
	NewClients ClientFactory
}

// ClientFactory constructs clients scoped to an assigned unit. It does not
// take ownership of the shared connection managed by the API caller manifold.
type ClientFactory func(context.Context, api.Connection, names.UnitTag) (Application, ResourceReader, error)

// Manifold starts one OCI image reconciler per assigned machine unit.
func Manifold(config ManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: []string{config.AgentName, config.APICallerName, config.CharmDirName},
		Start: func(ctx context.Context, getter dependency.Getter) (worker.Worker, error) {
			return startManifold(ctx, getter, config)
		},
	}
}

func startManifold(ctx context.Context, getter dependency.Getter, config ManifoldConfig) (worker.Worker, error) {
	if config.AgentName == "" || config.APICallerName == "" || config.CharmDirName == "" {
		return nil, errors.New("OCI workload manifold requires dependency names")
	}
	if config.Clock == nil || config.Logger == nil {
		return nil, errors.New("OCI workload manifold requires a clock and logger")
	}

	var a agent.Agent
	if err := getter.Get(config.AgentName, &a); err != nil {
		return nil, err
	}

	var conn api.Connection
	if err := getter.Get(config.APICallerName, &conn); err != nil {
		return nil, err
	}

	var guest fortress.Guest
	if err := getter.Get(config.CharmDirName, &guest); err != nil {
		return nil, err
	}

	// A nested unit agent exists only after the machine deployer has assigned
	// its unit. Its tag also scopes all subsequent resource API requests.
	agentConfig := a.CurrentConfig()
	unitTag, ok := agentConfig.Tag().(names.UnitTag)
	if !ok {
		return nil, errors.New("OCI workload worker requires a unit agent")
	}

	newClients := config.NewClients
	if newClients == nil {
		newClients = newUnitClients
	}
	application, resources, err := newClients(ctx, conn, unitTag)
	if err != nil {
		return nil, err
	}

	puller := config.Puller
	if puller == nil {
		puller = Puller{}
	}

	return NewWorker(WorkerConfig{
		Application:   application,
		Resources:     resources,
		CharmDir:      filepath.Join(agent.Dir(agentConfig.DataDir(), unitTag), "charm"),
		CharmDirGuest: guest,
		Puller:        puller,
		Clock:         config.Clock,
		Logger:        config.Logger,
	})
}

func newUnitClients(ctx context.Context, conn api.Connection, tag names.UnitTag) (Application, ResourceReader, error) {
	appName, err := names.UnitApplication(tag.Id())
	if err != nil {
		return nil, nil, err
	}

	client := uniter.NewClient(conn, tag)
	application, err := client.Application(ctx, names.NewApplicationTag(appName))
	if err != nil {
		return nil, nil, err
	}

	resources, err := uniter.NewResourcesFacadeClient(conn, tag)
	if err != nil {
		return nil, nil, err
	}

	return application, resources, nil
}
