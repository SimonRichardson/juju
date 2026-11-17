// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter

import (
	"slices"
	"sync"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/worker/v5"
	"gopkg.in/tomb.v2"

	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/internal/worker/uniter/container"
)

// WorkloadMonitorConfig holds the dependencies of a workload monitor.
type WorkloadMonitorConfig struct {
	// ContainerNames is the initial set of workload containers. It may be
	// empty until a charm declaring containers is deployed.
	ContainerNames []string
	// SocketDir holds the unit's workload container pebble sockets. An
	// empty directory retains the Kubernetes layout.
	SocketDir string
	// WorkloadEventChannel receives workload event notifications, which
	// the workload hook resolvers consume.
	WorkloadEventChannel chan string
	// WorkloadEvents records the pending workload events.
	WorkloadEvents container.WorkloadEvents
	// NewPebbleClient creates clients for the workload pebble sockets. If
	// nil, the pollers default to the real pebble client.
	NewPebbleClient NewPebbleClientFunc
	// NewPoller and NewNoticer create the per-set workers and are always
	// required, so tests can observe monitor behaviour without live
	// sockets.
	NewPoller  PollerConstructor
	NewNoticer NoticerConstructor
	Clock      clock.Clock
	Logger     logger.Logger
}

// Validate reports any missing monitor dependency.
func (config WorkloadMonitorConfig) Validate() error {
	switch {
	case config.WorkloadEventChannel == nil:
		return errors.New("missing WorkloadEventChannel")
	case config.WorkloadEvents == nil:
		return errors.New("missing WorkloadEvents")
	case config.NewPoller == nil:
		return errors.New("missing NewPoller")
	case config.NewNoticer == nil:
		return errors.New("missing NewNoticer")
	case config.Clock == nil:
		return errors.New("missing Clock")
	case config.Logger == nil:
		return errors.New("missing Logger")
	}
	return nil
}

// PollerConstructor and NoticerConstructor create the per-set workers.
type (
	PollerConstructor  func(logger.Logger, clock.Clock, []string, string, chan string, container.WorkloadEvents, NewPebbleClientFunc) worker.Worker
	NoticerConstructor func(logger.Logger, clock.Clock, []string, string, chan string, container.WorkloadEvents, NewPebbleClientFunc) worker.Worker
)

// workloadMonitor owns the pebble poller and noticer watching the unit's
// workload containers. It replaces that pair whenever the deployed charm's
// container set changes, so machine units follow charm installs and upgrades
// without restarting the uniter: the pollers/noticers for new containers
// start, and the ones for removed containers stop.
type workloadMonitor struct {
	tomb tomb.Tomb
	cfg  WorkloadMonitorConfig

	mut     sync.Mutex
	names   []string
	changed chan struct{}
}

// newWorkloadMonitor starts a monitor for the supplied container set. The
// returned func reports container set changes; it never blocks.
func newWorkloadMonitor(cfg WorkloadMonitorConfig) (*workloadMonitor, func([]string), error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, errors.Trace(err)
	}
	m := &workloadMonitor{
		cfg:     cfg,
		names:   cfg.ContainerNames,
		changed: make(chan struct{}, 1),
	}
	m.tomb.Go(m.loop)
	return m, m.update, nil
}

// update records a new container set and wakes the monitor.
func (m *workloadMonitor) update(names []string) {
	m.mut.Lock()
	defer m.mut.Unlock()
	m.names = names
	select {
	case m.changed <- struct{}{}:
	default:
	}
}

// Kill is part of the worker.Worker interface.
func (m *workloadMonitor) Kill() {
	m.tomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (m *workloadMonitor) Wait() error {
	return m.tomb.Wait()
}

func (m *workloadMonitor) loop() error {
	var poller, noticer worker.Worker
	var started []string
	defer func() {
		if poller != nil {
			worker.Stop(poller)
		}
		if noticer != nil {
			worker.Stop(noticer)
		}
	}()

	for {
		m.mut.Lock()
		names := slices.Clone(m.names)
		m.mut.Unlock()

		if !slices.Equal(started, names) {
			if poller != nil {
				worker.Stop(poller)
				poller = nil
			}
			if noticer != nil {
				worker.Stop(noticer)
				noticer = nil
			}
			started = names

			// An empty container set needs no pollers; the next
			// update may still introduce one.
			if len(names) > 0 {
				poller = m.cfg.NewPoller(
					m.cfg.Logger, m.cfg.Clock, names, m.cfg.SocketDir,
					m.cfg.WorkloadEventChannel, m.cfg.WorkloadEvents, m.cfg.NewPebbleClient)
				noticer = m.cfg.NewNoticer(
					m.cfg.Logger, m.cfg.Clock, names, m.cfg.SocketDir,
					m.cfg.WorkloadEventChannel, m.cfg.WorkloadEvents, m.cfg.NewPebbleClient)
			}
		}

		select {
		case <-m.tomb.Dying():
			return tomb.ErrDying
		case <-m.changed:
		}
	}
}
