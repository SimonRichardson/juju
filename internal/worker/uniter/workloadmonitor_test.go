// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/canonical/pebble/client"
	"github.com/juju/clock"
	"github.com/juju/tc"
	"github.com/juju/worker/v5"
	"github.com/juju/worker/v5/workertest"

	"github.com/juju/juju/core/logger"
	internallogger "github.com/juju/juju/internal/logger"
	coretesting "github.com/juju/juju/internal/testing"
	"github.com/juju/juju/internal/worker/uniter/container"
)

type workloadMonitorSuite struct {
	coretesting.BaseSuite
}

func TestWorkloadMonitorSuite(t *testing.T) {
	tc.Run(t, &workloadMonitorSuite{})
}

// stubWorker is an injectable poller or noticer that records the container
// set it watches and whether it was stopped.
type stubWorker struct {
	names   []string
	stopped atomic.Bool
}

func (s *stubWorker) Kill() {
	s.stopped.Store(true)
}

func (s *stubWorker) Wait() error { return nil }

// recorder is an injectable poller/noticer constructor that records every
// created worker, two per container set.
type recorder struct {
	mut     sync.Mutex
	workers []*stubWorker
}

func (r *recorder) construct(_ logger.Logger, _ clock.Clock, names []string, _ string, _ chan string, _ container.WorkloadEvents, _ NewPebbleClientFunc) worker.Worker {
	r.mut.Lock()
	defer r.mut.Unlock()
	w := &stubWorker{names: names}
	r.workers = append(r.workers, w)
	return w
}

func (r *recorder) created() []*stubWorker {
	r.mut.Lock()
	defer r.mut.Unlock()
	return append([]*stubWorker(nil), r.workers...)
}

// waitFor runs check until it holds or the long wait deadline passes.
func waitFor(c *tc.C, check func() bool) {
	deadline := time.After(coretesting.LongWait)
	for !check() {
		select {
		case <-deadline:
			c.Fatal("timed out waiting for the workload monitor")
		case <-time.After(coretesting.ShortWait):
		}
	}
}

func monitorFixture(c *tc.C, rec *recorder, names []string) (worker.Worker, func([]string)) {
	monitor, update, err := newWorkloadMonitor(WorkloadMonitorConfig{
		ContainerNames:       names,
		WorkloadEventChannel: make(chan string),
		WorkloadEvents:       container.NewWorkloadEvents(),
		NewPebbleClient: func(*client.Config) (PebbleClient, error) {
			return nil, nil
		},
		NewPoller:  rec.construct,
		NewNoticer: rec.construct,
		Clock:      clock.WallClock,
		Logger:     internallogger.Noop(),
	})
	c.Assert(err, tc.ErrorIsNil)
	return monitor, update
}

// TestConfigValidate checks each required monitor dependency is reported
// when missing, while the optional container names and socket directory may
// be empty.
func (s *workloadMonitorSuite) TestConfigValidate(c *tc.C) {
	valid := WorkloadMonitorConfig{
		WorkloadEventChannel: make(chan string),
		WorkloadEvents:       container.NewWorkloadEvents(),
		NewPoller: func(logger.Logger, clock.Clock, []string, string, chan string, container.WorkloadEvents, NewPebbleClientFunc) worker.Worker {
			return nil
		},
		NewNoticer: func(logger.Logger, clock.Clock, []string, string, chan string, container.WorkloadEvents, NewPebbleClientFunc) worker.Worker {
			return nil
		},
		Clock:  clock.WallClock,
		Logger: internallogger.Noop(),
	}
	c.Check(valid.Validate(), tc.ErrorIsNil)

	missing := []struct {
		name   string
		change func(*WorkloadMonitorConfig)
	}{
		{"WorkloadEventChannel", func(cfg *WorkloadMonitorConfig) { cfg.WorkloadEventChannel = nil }},
		{"WorkloadEvents", func(cfg *WorkloadMonitorConfig) { cfg.WorkloadEvents = nil }},
		{"NewPoller", func(cfg *WorkloadMonitorConfig) { cfg.NewPoller = nil }},
		{"NewNoticer", func(cfg *WorkloadMonitorConfig) { cfg.NewNoticer = nil }},
		{"Clock", func(cfg *WorkloadMonitorConfig) { cfg.Clock = nil }},
		{"Logger", func(cfg *WorkloadMonitorConfig) { cfg.Logger = nil }},
	}
	for _, test := range missing {
		c.Logf("missing %s", test.name)
		cfg := valid
		test.change(&cfg)
		c.Check(cfg.Validate(), tc.ErrorMatches, "missing "+test.name)
	}

	// An empty initial container set is valid: the monitor starts with no
	// pollers and follows the first deployment that declares containers.
	_, _, err := newWorkloadMonitor(valid)
	c.Assert(err, tc.ErrorIsNil)
}

// TestMonitorFollowsContainerSets checks the monitor starts a poller and a
// noticer for the initial container set, replaces both when a deployment
// changes the set, stops the superseded pair, and does not restart for an
// unchanged set.
func (s *workloadMonitorSuite) TestMonitorFollowsContainerSets(c *tc.C) {
	rec := &recorder{}
	monitor, update := monitorFixture(c, rec, []string{"redis"})
	defer workertest.DirtyKill(c, monitor)

	// The initial pair is created as the monitor starts.
	waitFor(c, func() bool { return len(rec.created()) == 2 })

	// An unchanged set keeps the current pair; the changed set replaces
	// it. Once the new pair exists, the total is exactly two pairs, which
	// also proves the unchanged update did not restart anything.
	update([]string{"redis"})
	update([]string{"redis", "mysql"})
	waitFor(c, func() bool { return len(rec.created()) == 4 })

	workers := rec.created()
	c.Assert(workers, tc.HasLen, 4)
	c.Check(workers[0].names, tc.DeepEquals, []string{"redis"})
	c.Check(workers[1].names, tc.DeepEquals, []string{"redis"})
	c.Check(workers[0].stopped.Load(), tc.IsTrue)
	c.Check(workers[1].stopped.Load(), tc.IsTrue)
	c.Check(workers[2].names, tc.DeepEquals, []string{"redis", "mysql"})
	c.Check(workers[3].names, tc.DeepEquals, []string{"redis", "mysql"})
	c.Check(workers[2].stopped.Load(), tc.IsFalse)
	c.Check(workers[3].stopped.Load(), tc.IsFalse)

	workertest.CleanKill(c, monitor)
	c.Check(workers[2].stopped.Load(), tc.IsTrue)
	c.Check(workers[3].stopped.Load(), tc.IsTrue)
}

// TestMonitorEmptySetStartsNothing checks an initially empty container set
// creates no workers until a charm deployment introduces one.
func (s *workloadMonitorSuite) TestMonitorEmptySetStartsNothing(c *tc.C) {
	rec := &recorder{}
	monitor, update := monitorFixture(c, rec, nil)
	defer workertest.DirtyKill(c, monitor)

	c.Check(rec.created(), tc.HasLen, 0)

	update([]string{"redis"})
	waitFor(c, func() bool { return len(rec.created()) == 2 })

	workers := rec.created()
	c.Assert(workers, tc.HasLen, 2)
	c.Check(workers[0].names, tc.DeepEquals, []string{"redis"})
	c.Check(workers[1].names, tc.DeepEquals, []string{"redis"})

	workertest.CleanKill(c, monitor)
	c.Check(workers[0].stopped.Load(), tc.IsTrue)
	c.Check(workers[1].stopped.Load(), tc.IsTrue)
}

// TestMonitorUpdateNeverBlocks checks that a deployment can record a new
// container set even while the monitor is busy restarting its workers,
// because deployments must not stall on monitor progress.
func (s *workloadMonitorSuite) TestMonitorUpdateNeverBlocks(c *tc.C) {
	rec := &recorder{}
	monitor, update := monitorFixture(c, rec, nil)
	defer workertest.DirtyKill(c, monitor)

	done := make(chan struct{})
	go func() {
		defer close(done)
		update([]string{"redis"})
		update([]string{"redis", "mysql"})
	}()
	select {
	case <-done:
	case <-time.After(coretesting.LongWait):
		c.Fatal("update blocked while the monitor was mid-restart")
	}

	// The monitor settles on the latest set regardless of coalescing.
	waitFor(c, func() bool {
		workers := rec.created()
		return len(workers) > 0 && workers[len(workers)-1].watching("mysql")
	})
	workertest.CleanKill(c, monitor)
}

// watching reports whether the stub watches the named container.
func (s *stubWorker) watching(name string) bool {
	for _, n := range s.names {
		if n == name {
			return true
		}
	}
	return false
}
