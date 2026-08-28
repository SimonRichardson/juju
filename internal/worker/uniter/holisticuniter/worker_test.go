// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package holisticuniter

import (
	"context"
	stdtesting "testing"
	"time"

	"github.com/juju/clock/testclock"
	"github.com/juju/errors"
	"github.com/juju/tc"
	"github.com/juju/worker/v5"

	corewatcher "github.com/juju/juju/core/watcher"
	"github.com/juju/juju/domain/deployment/charm/hooks"
	"github.com/juju/juju/internal/worker/uniter/shared/charm"
	"github.com/juju/juju/internal/worker/uniter/shared/hook"
	"github.com/juju/juju/internal/worker/uniter/shared/runner"
	"github.com/juju/juju/rpc/params"
)

type WorkerSuite struct{}

func TestWorkerSuite(t *stdtesting.T) {
	tc.Run(t, &WorkerSuite{})
}

func (s *WorkerSuite) TestDispatchesSnapshotForEachNotification(c *tc.C) {
	watch := newTestWatcher()
	client := &testSnapshotClient{snapshot: params.UnitSnapshot{UnitName: "app/0"}}
	dispatched := make(chan params.UnitSnapshot, 3)
	events := make(chan hooks.Kind, 3)
	strategy, err := NewLifecycleStrategy(StrategyConfig{
		Planner: NewLifecyclePlanner(),
		Dispatch: func(_ context.Context, event hooks.Kind, snapshot params.UnitSnapshot) error {
			events <- event
			dispatched <- snapshot
			return nil
		},
	})
	c.Assert(err, tc.ErrorIsNil)

	w, err := New(Config{
		Watcher:  watch,
		Snapshot: client,
		Strategy: strategy,
	})
	c.Assert(err, tc.ErrorIsNil)
	defer worker.Stop(w)

	watch.changes <- struct{}{}
	select {
	case snapshot := <-dispatched:
		c.Check(snapshot.UnitName, tc.Equals, "app/0")
	case <-c.Context().Done():
		c.Fatalf("timed out waiting for dispatch")
	}
	c.Check(<-events, tc.Equals, hooks.Install)
	c.Check(<-events, tc.Equals, hooks.ConfigChanged)
	c.Check(<-events, tc.Equals, hooks.Start)
	c.Check(client.calls, tc.Equals, 1)
}

func (s *WorkerSuite) TestSnapshotErrorStopsWorker(c *tc.C) {
	watch := newTestWatcher()
	client := &testSnapshotClient{err: errSnapshot}
	strategy, err := NewLifecycleStrategy(StrategyConfig{
		Planner:  NewLifecyclePlanner(),
		Dispatch: func(context.Context, hooks.Kind, params.UnitSnapshot) error { return nil },
	})
	c.Assert(err, tc.ErrorIsNil)
	w, err := New(Config{
		Watcher:  watch,
		Snapshot: client,
		Strategy: strategy,
	})
	c.Assert(err, tc.ErrorIsNil)
	watch.changes <- struct{}{}
	err = w.Wait()
	c.Check(err, tc.ErrorMatches, "getting unit snapshot: snapshot failed")
}

func (s *WorkerSuite) TestFailedDispatchRetriesWithFreshSnapshot(c *tc.C) {
	watch := newTestWatcher()
	client := &testSnapshotClient{snapshot: params.UnitSnapshot{UnitName: "app/0"}}
	clock := testclock.NewClock(time.Now())
	strategy := &retryStrategy{failed: make(chan struct{}), done: make(chan struct{})}
	w, err := New(Config{
		Watcher:    watch,
		Snapshot:   client,
		Strategy:   strategy,
		RetryDelay: time.Second,
		Clock:      clock,
	})
	c.Assert(err, tc.ErrorIsNil)
	defer worker.Stop(w)

	watch.changes <- struct{}{}
	<-strategy.failed
	clock.Advance(time.Second)
	select {
	case <-strategy.done:
	case <-c.Context().Done():
		c.Fatalf("timed out waiting for retry")
	}

	c.Check(client.calls, tc.Equals, 2)
}

func (s *WorkerSuite) TestNewForUnitUsesUnitAPIs(c *tc.C) {
	unit := &testUnit{watcher: newTestWatcher()}
	dispatched := make(chan struct{}, 3)
	w, err := NewForUnit(c.Context(), unit, NewLifecyclePlanner(), func(context.Context, hooks.Kind, params.UnitSnapshot) error {
		dispatched <- struct{}{}
		return nil
	})
	c.Assert(err, tc.ErrorIsNil)
	defer worker.Stop(w)

	unit.watcher.changes <- struct{}{}
	select {
	case <-dispatched:
	case <-c.Context().Done():
		c.Fatalf("timed out waiting for dispatch")
	}
}

func (s *WorkerSuite) TestStagesAndDeploysCharmBeforeDispatch(c *tc.C) {
	watch := newTestWatcher()
	deployer := &testDeployer{done: make(chan struct{})}
	strategy, err := NewLifecycleStrategy(StrategyConfig{
		Planner: NewLifecyclePlanner(),
		Charm: func(context.Context, params.UnitSnapshot) (charm.BundleInfo, error) {
			return testBundleInfo{url: "charmhub/ubuntu-0"}, nil
		},
		Deployer: deployer,
		Dispatch: func(context.Context, hooks.Kind, params.UnitSnapshot) error { return nil },
	})
	c.Assert(err, tc.ErrorIsNil)
	w, err := New(Config{
		Watcher:  watch,
		Snapshot: &testSnapshotClient{snapshot: params.UnitSnapshot{CharmURL: "charmhub/ubuntu-0"}},
		Strategy: strategy,
	})
	c.Assert(err, tc.ErrorIsNil)
	defer worker.Stop(w)

	watch.changes <- struct{}{}
	select {
	case <-deployer.done:
	case <-c.Context().Done():
		c.Fatalf("timed out waiting for charm deployment")
	}
	c.Check(deployer.staged, tc.Equals, 1)
}

func (s *WorkerSuite) TestRuntimeStagesAndDispatchesWithStandardRunner(c *tc.C) {
	unit := &testUnit{
		testSnapshotClient: testSnapshotClient{snapshot: params.UnitSnapshot{CharmURL: "charmhub/ubuntu-0"}},
		watcher:            newTestWatcher(),
	}
	deployer := &testDeployer{done: make(chan struct{})}
	testRunner := &testRunner{events: make(chan string, 3)}
	guard := &testCharmDirGuard{unlocked: make(chan struct{})}
	w, err := NewRuntime(c.Context(), RuntimeConfig{
		Unit:     unit,
		Planner:  NewLifecyclePlanner(),
		Deployer: deployer,
		Charm: func(context.Context, params.UnitSnapshot) (charm.BundleInfo, error) {
			return testBundleInfo{url: "charmhub/ubuntu-0"}, nil
		},
		NewHookRunner: func(_ context.Context, info hook.Info, snapshot params.UnitSnapshot) (runner.Runner, error) {
			c.Check(info.Kind, tc.Not(tc.Equals), hooks.Kind(""))
			c.Check(snapshot.CharmURL, tc.Equals, "charmhub/ubuntu-0")
			return testRunner, nil
		},
		Lock: func(context.Context, hooks.Kind) (func(), error) {
			return func() {}, nil
		},
		CharmDirGuard: guard,
	})
	c.Assert(err, tc.ErrorIsNil)
	defer worker.Stop(w)

	unit.watcher.changes <- struct{}{}
	select {
	case <-deployer.done:
	case <-c.Context().Done():
		c.Fatalf("timed out waiting for charm deployment")
	}
	c.Check(<-testRunner.events, tc.Equals, "install")
	c.Check(<-testRunner.events, tc.Equals, "config-changed")
	c.Check(<-testRunner.events, tc.Equals, "start")
	<-guard.unlocked
	c.Check(guard.lockdowns, tc.Equals, 1)
	c.Check(guard.unlocks, tc.Equals, 1)
}

func (s *WorkerSuite) TestNewValidation(c *tc.C) {
	err := (Config{}).Validate()
	c.Check(err, tc.ErrorMatches, "missing watcher.*")
	_, err = New(Config{})
	c.Check(err, tc.ErrorMatches, "missing watcher.*")
}

var errSnapshot = errorString("snapshot failed")

type errorString string

func (e errorString) Error() string { return string(e) }

type testSnapshotClient struct {
	snapshot params.UnitSnapshot
	err      error
	calls    int
}

type retryStrategy struct {
	failed chan struct{}
	done   chan struct{}
	calls  int
}

func (s *retryStrategy) Handle(context.Context, params.UnitSnapshot) error {
	s.calls++
	if s.calls == 1 {
		close(s.failed)
		return errors.New("hook failed")
	}
	close(s.done)
	return nil
}

type testUnit struct {
	testSnapshotClient
	watcher *testWatcher
}

type testBundleInfo struct{ url string }

func (i testBundleInfo) URL() string { return i.url }

func (i testBundleInfo) ArchiveSha256(context.Context) (string, error) { return "", nil }

type testDeployer struct {
	staged   int
	deployed bool
	done     chan struct{}
}

type testRunner struct {
	runner.Runner
	events chan string
}

type testCharmDirGuard struct {
	lockdowns int
	unlocks   int
	unlocked  chan struct{}
}

func (g *testCharmDirGuard) Unlock(context.Context) error {
	g.unlocks++
	if g.unlocked != nil {
		close(g.unlocked)
	}
	return nil
}

func (g *testCharmDirGuard) Lockdown(context.Context) error {
	g.lockdowns++
	return nil
}

func (r *testRunner) RunHook(_ context.Context, name string) (runner.HookHandlerType, error) {
	r.events <- name
	return runner.DispatchingHookHandler, nil
}

func (d *testDeployer) Stage(context.Context, charm.BundleInfo) error {
	d.staged++
	return nil
}

func (d *testDeployer) Deploy() error {
	d.deployed = true
	close(d.done)
	return nil
}

func (u *testUnit) WatchComposite(context.Context) (corewatcher.NotifyWatcher, error) {
	return u.watcher, nil
}

func (c *testSnapshotClient) Snapshot(context.Context) (params.UnitSnapshot, error) {
	c.calls++
	return c.snapshot, c.err
}

type testWatcher struct {
	changes chan struct{}
	done    chan struct{}
}

func newTestWatcher() *testWatcher {
	return &testWatcher{changes: make(chan struct{}, 1), done: make(chan struct{})}
}

func (w *testWatcher) Kill() {
	select {
	case <-w.done:
	default:
		close(w.done)
	}
}

func (w *testWatcher) Wait() error { return nil }

func (w *testWatcher) Changes() <-chan struct{} { return w.changes }

var _ corewatcher.NotifyWatcher = (*testWatcher)(nil)
