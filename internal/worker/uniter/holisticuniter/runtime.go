// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package holisticuniter

import (
	"context"
	"time"

	"github.com/juju/clock"
	"github.com/juju/errors"

	"github.com/juju/juju/domain/deployment/charm/hooks"
	"github.com/juju/juju/internal/worker/fortress"
	charm "github.com/juju/juju/internal/worker/uniter/shared/charm"
	"github.com/juju/juju/internal/worker/uniter/shared/hook"
	"github.com/juju/juju/internal/worker/uniter/shared/runner"
	"github.com/juju/juju/rpc/params"
)

// HookRunnerFactory creates the standard uniter runner for a lifecycle hook.
// The runner supplies the same dispatch script, hook environment, and jujuc
// server used by the delta uniter, with the current snapshot attached to its
// hook context.
type HookRunnerFactory func(context.Context, hook.Info, params.UnitSnapshot) (runner.Runner, error)

// LockFunc acquires the global machine lock required for charm hook execution.
type LockFunc func(context.Context, hooks.Kind) (func(), error)

// RuntimeConfig contains the fully constructed dependencies required to run a
// holistic unit. Construction belongs in the uniter manifold so both runtimes
// receive the same agent-scoped services.
type RuntimeConfig struct {
	Unit          Unit
	Planner       EventPlanner
	Charm         CharmProvider
	Deployer      charm.Deployer
	NewHookRunner HookRunnerFactory
	Lock          LockFunc
	CharmDirGuard fortress.Guard
	RetryDelay    time.Duration
	Clock         clock.Clock
}

// Validate checks that the complete charm lifecycle runtime is configured.
func (c RuntimeConfig) Validate() error {
	if c.Unit == nil {
		return errors.NotValidf("missing unit")
	}
	if c.Planner == nil {
		return errors.NotValidf("missing event planner")
	}
	if c.Charm == nil {
		return errors.NotValidf("missing charm provider")
	}
	if c.Deployer == nil {
		return errors.NotValidf("missing charm deployer")
	}
	if c.NewHookRunner == nil {
		return errors.NotValidf("missing hook runner factory")
	}
	if c.Lock == nil {
		return errors.NotValidf("missing hook lock")
	}
	if c.CharmDirGuard == nil {
		return errors.NotValidf("missing charm directory guard")
	}
	if c.RetryDelay > 0 && c.Clock == nil {
		return errors.NotValidf("missing clock for retry")
	}
	return nil
}

// NewRuntime creates a holistic worker which stages charms through the
// verified manifest deployer and dispatches hooks through the standard runner.
func NewRuntime(ctx context.Context, config RuntimeConfig) (*HolisticUniter, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}
	strategy, err := NewLifecycleStrategy(StrategyConfig{
		Planner:       config.Planner,
		Charm:         config.Charm,
		Deployer:      config.Deployer,
		CharmDirGuard: config.CharmDirGuard,
		Dispatch: func(ctx context.Context, event hooks.Kind, snapshot params.UnitSnapshot) error {
			release, err := config.Lock(ctx, event)
			if err != nil {
				return errors.Annotatef(err, "acquiring lock for %s", event)
			}
			defer release()

			runner, err := config.NewHookRunner(ctx, hook.Info{Kind: event}, snapshot)
			if err != nil {
				return errors.Annotatef(err, "creating runner for %s", event)
			}
			_, err = runner.RunHook(ctx, string(event))
			return errors.Trace(err)
		},
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	return newForUnit(ctx, config.Unit, strategy, config.RetryDelay, config.Clock)
}
