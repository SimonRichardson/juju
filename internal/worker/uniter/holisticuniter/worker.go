// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package holisticuniter

import (
	"context"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/utils/v4"
	"github.com/juju/worker/v5"
	"github.com/juju/worker/v5/catacomb"

	corewatcher "github.com/juju/juju/core/watcher"
	"github.com/juju/juju/domain/deployment/charm/hooks"
	charm "github.com/juju/juju/internal/worker/uniter/shared/charm"
	"github.com/juju/juju/rpc/params"
)

// SnapshotClient obtains the complete state for a holistic unit.
type SnapshotClient interface {
	// Snapshot returns the complete current state for the unit.
	Snapshot(context.Context) (params.UnitSnapshot, error)
}

// Unit is the portion of the unit API needed to run a holistic worker.
type Unit interface {
	SnapshotClient
	// WatchComposite notifies when any snapshot input changes.
	WatchComposite(context.Context) (corewatcher.NotifyWatcher, error)
	// ClearResolved clears the unit's explicit hook-resolution mode.
	ClearResolved(context.Context) error
}

// DispatchFunc runs one named charm event with the snapshot available through
// the dispatch context.
type DispatchFunc func(context.Context, hooks.Kind, params.UnitSnapshot) error

// CharmProvider returns the charm bundle selected by the controller. The
// bundle reader verifies and downloads the archive when it is staged.
type CharmProvider func(context.Context, params.UnitSnapshot) (charm.BundleInfo, error)

// NewForUnit creates a holistic worker using the unit's composite watcher and
// snapshot API.
func NewForUnit(ctx context.Context, unit Unit, planner EventPlanner, dispatch DispatchFunc) (*HolisticUniter, error) {
	strategy, err := NewLifecycleStrategy(StrategyConfig{Planner: planner, Dispatch: dispatch})
	if err != nil {
		return nil, errors.Trace(err)
	}
	return newForUnit(ctx, unit, strategy, params.RetryStrategy{}, nil)
}

// NewForUnitWithCharm is NewForUnit with charm staging and deployment enabled.
// The first snapshot notification stages and deploys the selected charm before
// dispatch; later notifications only deploy when the charm URL changes.
func NewForUnitWithCharm(ctx context.Context, unit Unit, planner EventPlanner, charmProvider CharmProvider, deployer charm.Deployer, dispatch DispatchFunc) (*HolisticUniter, error) {
	if unit == nil {
		return nil, errors.NotValidf("missing unit")
	}
	strategy, err := NewLifecycleStrategy(StrategyConfig{
		Planner: planner, Charm: charmProvider, Deployer: deployer, Dispatch: dispatch,
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	return newForUnit(ctx, unit, strategy, params.RetryStrategy{}, nil)
}

func newForUnit(ctx context.Context, unit Unit, strategy Strategy, retryStrategy params.RetryStrategy, clock clock.Clock) (*HolisticUniter, error) {
	if unit == nil {
		return nil, errors.NotValidf("missing unit")
	}
	if strategy == nil {
		return nil, errors.NotValidf("missing lifecycle strategy")
	}
	watcher, err := unit.WatchComposite(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "watching unit snapshot")
	}
	config := Config{
		Watcher:       watcher,
		Snapshot:      unit,
		Strategy:      strategy,
		RetryStrategy: retryStrategy,
		ClearResolved: unit.ClearResolved,
		Clock:         clock,
	}
	if err := config.Validate(); err != nil {
		watcher.Kill()
		return nil, errors.Trace(err)
	}
	w, err := New(config)
	if err != nil {
		watcher.Kill()
		return nil, errors.Trace(err)
	}
	return w, nil
}

// Config contains the dependencies of a holistic unit worker.
type Config struct {
	Watcher       corewatcher.NotifyWatcher
	Snapshot      SnapshotClient
	Strategy      Strategy
	RetryStrategy params.RetryStrategy
	ClearResolved func(context.Context) error
	Clock         clock.Clock
}

// Validate checks that all worker dependencies are present and consistent.
func (c Config) Validate() error {
	if c.Watcher == nil {
		return errors.NotValidf("missing watcher")
	}
	if c.Snapshot == nil {
		return errors.NotValidf("missing snapshot client")
	}
	if c.Strategy == nil {
		return errors.NotValidf("missing lifecycle strategy")
	}
	if c.RetryStrategy.ShouldRetry && c.Clock == nil {
		return errors.NotValidf("missing clock for retry")
	}
	if c.ClearResolved == nil {
		return errors.NotValidf("missing clear resolved function")
	}
	return nil
}

// HolisticUniter waits for coalesced unit changes, fetches the current
// snapshot, and dispatches the charm against that snapshot.
type HolisticUniter struct {
	catacomb catacomb.Catacomb
	config   Config
}

var _ worker.Worker = (*HolisticUniter)(nil)

// New returns a holistic unit worker. The watcher is owned by the returned
// worker and is stopped when the worker is stopped.
func New(config Config) (*HolisticUniter, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
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
	retryCh := make(chan struct{}, 1)
	retryTimer := utils.NewBackoffTimer(utils.BackoffTimerConfig{
		Min:    w.config.RetryStrategy.MinRetryTime,
		Max:    w.config.RetryStrategy.MaxRetryTime,
		Jitter: w.config.RetryStrategy.JitterRetryTime,
		Factor: w.config.RetryStrategy.RetryTimeFactor,
		Clock:  w.config.Clock,
		Func: func() {
			select {
			case retryCh <- struct{}{}:
			default:
			}
		},
	})
	defer retryTimer.Reset()
	retryStarted := false
	reconcile := func(retryDue bool) error {
		snapshot, err := w.config.Snapshot.Snapshot(ctx)
		if err != nil {
			return errors.Annotate(err, "getting unit snapshot")
		}
		if resolver, ok := w.config.Strategy.(PendingEventResolver); ok && resolver.PendingEvent() != "" {
			switch snapshot.ResolvedMode {
			case params.ResolvedRetryHooks:
				if err := w.config.ClearResolved(ctx); err != nil {
					return errors.Annotate(err, "clearing resolved mode")
				}
				if err := resolver.RetryPending(ctx); err != nil {
					return errors.Annotate(err, "preparing pending event retry")
				}
				retryTimer.Reset()
				retryStarted = false
			case params.ResolvedNoHooks:
				if err := w.config.ClearResolved(ctx); err != nil {
					return errors.Annotate(err, "clearing resolved mode")
				}
				if err := resolver.SkipPending(ctx, snapshot); err != nil {
					return errors.Annotate(err, "skipping pending event")
				}
				retryTimer.Reset()
				retryStarted = false
				return nil
			case params.ResolvedNone:
				if !retryDue {
					if !retryStarted && w.config.RetryStrategy.ShouldRetry {
						retryTimer.Start()
						retryStarted = true
					}
					return nil
				}
				if err := resolver.RetryPending(ctx); err != nil {
					return errors.Annotate(err, "preparing pending event retry")
				}
				retryStarted = false
			}
		}
		err = w.config.Strategy.Handle(ctx, snapshot)
		if err == nil {
			retryTimer.Reset()
			retryStarted = false
		}
		return err
	}
	for {
		retryDue := false
		select {
		case <-w.catacomb.Dying():
			return w.catacomb.ErrDying()
		case _, ok := <-w.config.Watcher.Changes():
			if !ok {
				return nil
			}
		case <-retryCh:
			retryDue = true
		}

		if err := reconcile(retryDue); err != nil {
			if _, ok := w.config.Strategy.(PendingEventResolver); !ok {
				return errors.Trace(err)
			}
		}
	}
}
