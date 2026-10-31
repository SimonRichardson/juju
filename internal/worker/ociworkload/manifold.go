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
	"github.com/juju/juju/agent/tools"
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
	// Puller downloads and unpacks images from unit resources.
	Puller ImagePuller
	// Runtime converges machine containerd tasks for the assigned unit.
	Runtime ContainerRuntime

	// NewClients builds unit-authorised clients from the shared API connection.
	// If nil, the production uniter and resource clients are used.
	NewClients ClientFactory
}

// Validate reports a missing manifold dependency before the dependency engine
// is queried. Puller and Runtime are required even for ordinary machine charms
// so a container charm cannot start without a configured host runtime.
func (config ManifoldConfig) Validate() error {
	switch {
	case config.AgentName == "":
		return errors.New("missing AgentName")
	case config.APICallerName == "":
		return errors.New("missing APICallerName")
	case config.CharmDirName == "":
		return errors.New("missing CharmDirName")
	case config.Clock == nil:
		return errors.New("missing Clock")
	case config.Logger == nil:
		return errors.New("missing Logger")
	case config.Puller == nil:
		return errors.New("missing Puller")
	case config.Runtime == nil:
		return errors.New("missing Runtime")
	}

	return nil
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
	if err := config.Validate(); err != nil {
		return nil, err
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

	unitDir := agent.Dir(agentConfig.DataDir(), unitTag)

	return NewWorker(WorkerConfig{
		Application:   application,
		Resources:     resources,
		CharmDir:      filepath.Join(unitDir, "charm"),
		CharmDirGuest: guest,
		Puller:        config.Puller,
		Runtime:       config.Runtime,
		UnitName:      unitTag.Id(),
		PebbleBinary:  filepath.Join(tools.ToolsDir(agentConfig.DataDir(), unitTag.String()), "pebble"),
		ContainersDir: filepath.Join(unitDir, "containers"),
		StateDir:      filepath.Join(unitDir, "oci"),
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
