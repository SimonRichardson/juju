// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package holisticuniter

import (
	"context"

	"github.com/juju/errors"
	"github.com/juju/worker/v5"
	"github.com/juju/worker/v5/catacomb"

	corewatcher "github.com/juju/juju/core/watcher"
	charm "github.com/juju/juju/internal/worker/uniter/shared/charm"
	"github.com/juju/juju/rpc/params"
)

// SnapshotClient obtains the complete state for a holistic unit.
type SnapshotClient interface {
	Snapshot(context.Context) (params.UnitSnapshot, error)
}

// Unit is the portion of the unit API needed to run a holistic worker.
type Unit interface {
	SnapshotClient
	WatchComposite(context.Context) (corewatcher.NotifyWatcher, error)
}

// DispatchFunc runs the charm's dispatch entry point with the snapshot
// available through the dispatch context.
type DispatchFunc func(context.Context, params.UnitSnapshot) error

// CharmProvider returns the charm bundle selected by the controller. The
// bundle reader verifies and downloads the archive when it is staged.
type CharmProvider func(context.Context, params.UnitSnapshot) (charm.BundleInfo, error)

// NewForUnit creates a holistic worker using the unit's composite watcher and
// snapshot API.
func NewForUnit(ctx context.Context, unit Unit, dispatch DispatchFunc) (*HolisticUniter, error) {
	return NewForUnitWithCharm(ctx, unit, nil, nil, dispatch)
}

// NewForUnitWithCharm is NewForUnit with charm staging and deployment enabled.
// The first snapshot notification stages and deploys the selected charm before
// dispatch; later notifications only deploy when the charm URL changes.
func NewForUnitWithCharm(ctx context.Context, unit Unit, charmProvider CharmProvider, deployer charm.Deployer, dispatch DispatchFunc) (*HolisticUniter, error) {
	if unit == nil {
		return nil, errors.NotValidf("missing unit")
	}
	watcher, err := unit.WatchComposite(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "watching unit snapshot")
	}
	w, err := New(Config{
		Watcher:  watcher,
		Snapshot: unit,
		Charm:    charmProvider,
		Deployer: deployer,
		Dispatch: dispatch,
	})
	if err != nil {
		watcher.Kill()
		return nil, errors.Trace(err)
	}
	return w, nil
}

// Config contains the dependencies of a holistic unit worker.
type Config struct {
	Watcher  corewatcher.NotifyWatcher
	Snapshot SnapshotClient
	Charm    CharmProvider
	Deployer charm.Deployer
	Dispatch DispatchFunc
}

// HolisticUniter waits for coalesced unit changes, fetches the current
// snapshot, and dispatches the charm against that snapshot.
type HolisticUniter struct {
	catacomb         catacomb.Catacomb
	config           Config
	deployedCharmURL string
}

var _ worker.Worker = (*HolisticUniter)(nil)

// New returns a holistic unit worker. The watcher is owned by the returned
// worker and is stopped when the worker is stopped.
func New(config Config) (*HolisticUniter, error) {
	if config.Watcher == nil {
		return nil, errors.NotValidf("missing watcher")
	}
	if config.Snapshot == nil {
		return nil, errors.NotValidf("missing snapshot client")
	}
	if config.Dispatch == nil {
		return nil, errors.NotValidf("missing dispatch function")
	}
	if (config.Charm == nil) != (config.Deployer == nil) {
		return nil, errors.NotValidf("charm and deployer must be configured together")
	}

	w := &HolisticUniter{config: config}
	if err := catacomb.Invoke(catacomb.Plan{
		Name: "holistic-uniter",
		Site: &w.catacomb,
		Work: w.loop,
		Init: []worker.Worker{config.Watcher},
	}); err != nil {
		return nil, errors.Trace(err)
	}
	return w, nil
}

// Kill stops the worker and its composite watcher.
func (w *HolisticUniter) Kill() {
	w.catacomb.Kill(nil)
}

// Wait waits for the worker and its composite watcher to stop.
func (w *HolisticUniter) Wait() error {
	return w.catacomb.Wait()
}

func (w *HolisticUniter) loop() error {
	ctx := w.catacomb.Context(context.Background())
	for {
		select {
		case <-w.catacomb.Dying():
			return w.catacomb.ErrDying()
		case _, ok := <-w.config.Watcher.Changes():
			if !ok {
				return nil
			}
			snapshot, err := w.config.Snapshot.Snapshot(ctx)
			if err != nil {
				return errors.Annotate(err, "getting unit snapshot")
			}
			if w.config.Charm != nil {
				info, err := w.config.Charm(ctx, snapshot)
				if err != nil {
					return errors.Annotate(err, "getting charm bundle information")
				}
				if info.URL() != w.deployedCharmURL {
					if err := w.config.Deployer.Stage(ctx, info); err != nil {
						return errors.Annotate(err, "staging charm")
					}
					if err := w.config.Deployer.Deploy(); err != nil {
						return errors.Annotate(err, "deploying charm")
					}
					w.deployedCharmURL = info.URL()
				}
			}
			if err := w.config.Dispatch(ctx, snapshot); err != nil {
				return errors.Annotate(err, "dispatching holistic unit")
			}
		}
	}
}
