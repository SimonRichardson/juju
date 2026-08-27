// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package holisticuniter

import (
	"context"
	stdtesting "testing"

	"github.com/juju/tc"
	"github.com/juju/worker/v5"

	corewatcher "github.com/juju/juju/core/watcher"
	"github.com/juju/juju/internal/worker/uniter/shared/charm"
	"github.com/juju/juju/rpc/params"
)

type WorkerSuite struct{}

func TestWorkerSuite(t *stdtesting.T) {
	tc.Run(t, &WorkerSuite{})
}

func (s *WorkerSuite) TestDispatchesSnapshotForEachNotification(c *tc.C) {
	watch := newTestWatcher()
	client := &testSnapshotClient{snapshot: params.UnitSnapshot{UnitName: "app/0"}}
	dispatched := make(chan params.UnitSnapshot, 1)

	w, err := New(Config{
		Watcher:  watch,
		Snapshot: client,
		Dispatch: func(_ context.Context, snapshot params.UnitSnapshot) error {
			dispatched <- snapshot
			return nil
		},
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
	c.Check(client.calls, tc.Equals, 1)
}

func (s *WorkerSuite) TestSnapshotErrorStopsWorker(c *tc.C) {
	watch := newTestWatcher()
	client := &testSnapshotClient{err: errSnapshot}
	w, err := New(Config{
		Watcher:  watch,
		Snapshot: client,
		Dispatch: func(context.Context, params.UnitSnapshot) error { return nil },
	})
	c.Assert(err, tc.ErrorIsNil)
	watch.changes <- struct{}{}
	err = w.Wait()
	c.Check(err, tc.ErrorMatches, "getting unit snapshot: snapshot failed")
}

func (s *WorkerSuite) TestNewForUnitUsesUnitAPIs(c *tc.C) {
	unit := &testUnit{watcher: newTestWatcher()}
	dispatched := make(chan struct{}, 1)
	w, err := NewForUnit(c.Context(), unit, func(context.Context, params.UnitSnapshot) error {
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
	w, err := New(Config{
		Watcher:  watch,
		Snapshot: &testSnapshotClient{snapshot: params.UnitSnapshot{CharmURL: "charmhub/ubuntu-0"}},
		Charm: func(context.Context, params.UnitSnapshot) (charm.BundleInfo, error) {
			return testBundleInfo{url: "charmhub/ubuntu-0"}, nil
		},
		Deployer: deployer,
		Dispatch: func(context.Context, params.UnitSnapshot) error {
			return nil
		},
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

func (s *WorkerSuite) TestNewValidation(c *tc.C) {
	_, err := New(Config{})
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
