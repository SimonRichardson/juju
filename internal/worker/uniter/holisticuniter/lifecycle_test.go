// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package holisticuniter

import (
	"context"
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/core/life"
	"github.com/juju/juju/domain/deployment/charm/hooks"
	charm "github.com/juju/juju/internal/worker/uniter/shared/charm"
	"github.com/juju/juju/rpc/params"
)

type LifecycleSuite struct{}

func TestLifecycleSuite(t *stdtesting.T) {
	tc.Run(t, &LifecycleSuite{})
}

func (s *LifecycleSuite) TestInitialSetupIsDistinctAndOrdered(c *tc.C) {
	planner := NewLifecyclePlanner()
	snapshot := params.UnitSnapshot{CharmURL: "charmhub/example"}

	c.Check(planner.Plan(snapshot), tc.DeepEquals, []hooks.Kind{
		hooks.Install, hooks.Start,
	})
	c.Assert(planner.Complete(c.Context(), hooks.Install, snapshot), tc.ErrorIsNil)
	c.Assert(planner.Complete(c.Context(), hooks.Start, snapshot), tc.ErrorIsNil)
	c.Check(planner.Plan(snapshot), tc.IsNil)
}

func (s *LifecycleSuite) TestCharmChangeDispatchesReconcile(c *tc.C) {
	planner := NewLifecyclePlanner()
	initial := params.UnitSnapshot{CharmURL: "charmhub/example"}
	for _, event := range planner.Plan(initial) {
		c.Assert(planner.Complete(c.Context(), event, initial), tc.ErrorIsNil)
	}

	upgrade := params.UnitSnapshot{CharmURL: "charmhub/example-revision-2"}
	c.Check(planner.Plan(upgrade), tc.DeepEquals, []hooks.Kind{hooks.Reconcile})
	c.Assert(planner.Complete(c.Context(), hooks.Reconcile, upgrade), tc.ErrorIsNil)
	c.Check(planner.Plan(upgrade), tc.IsNil)
}

func (s *LifecycleSuite) TestCharmModifiedVersionDispatchesReconcile(c *tc.C) {
	planner := NewLifecyclePlanner()
	initial := params.UnitSnapshot{CharmURL: "charmhub/example", CharmModifiedVersion: 1}
	for _, event := range planner.Plan(initial) {
		c.Assert(planner.Complete(c.Context(), event, initial), tc.ErrorIsNil)
	}

	upgrade := initial
	upgrade.CharmModifiedVersion = 2
	c.Check(planner.Plan(upgrade), tc.DeepEquals, []hooks.Kind{hooks.Reconcile})
}

func (s *LifecycleSuite) TestFailedEventDoesNotAdvanceState(c *tc.C) {
	planner := NewLifecyclePlanner()
	snapshot := params.UnitSnapshot{CharmURL: "charmhub/example"}

	c.Check(planner.Plan(snapshot), tc.DeepEquals, []hooks.Kind{
		hooks.Install, hooks.Start,
	})
}

func (s *LifecycleSuite) TestPersistentPlannerResumesCompletedSetup(c *tc.C) {
	store := &testLifecycleStore{}
	snapshot := params.UnitSnapshot{CharmURL: "charmhub/example"}
	planner, err := NewPersistentLifecyclePlanner(c.Context(), store)
	c.Assert(err, tc.ErrorIsNil)
	for _, event := range planner.Plan(snapshot) {
		c.Assert(planner.Complete(c.Context(), event, snapshot), tc.ErrorIsNil)
	}

	restarted, err := NewPersistentLifecyclePlanner(c.Context(), store)
	c.Assert(err, tc.ErrorIsNil)

	c.Check(store.saves, tc.Equals, 2)
	c.Check(restarted.Plan(snapshot), tc.IsNil)
}

func (s *LifecycleSuite) TestPersistentPlannerResumesPendingEvent(c *tc.C) {
	store := &testLifecycleStore{}
	snapshot := params.UnitSnapshot{CharmURL: "charmhub/example"}
	planner, err := NewPersistentLifecyclePlanner(c.Context(), store)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(planner.Begin(c.Context(), hooks.Start, snapshot), tc.ErrorIsNil)

	restarted, err := NewPersistentLifecyclePlanner(c.Context(), store)
	c.Assert(err, tc.ErrorIsNil)

	c.Check(restarted.Plan(snapshot), tc.DeepEquals, []hooks.Kind{hooks.Start})
}

func (s *LifecycleSuite) TestPlannerDispatchesReconcileWhenSnapshotChanges(c *tc.C) {
	planner := NewLifecyclePlanner()
	snapshot := params.UnitSnapshot{
		CharmURL: "charmhub/example",
		Config:   map[string]any{"message": "one"},
	}
	for _, event := range planner.Plan(snapshot) {
		c.Assert(planner.Complete(c.Context(), event, snapshot), tc.ErrorIsNil)
	}

	c.Check(planner.Plan(snapshot), tc.IsNil)
	snapshot.Config["message"] = "two"
	c.Check(planner.Plan(snapshot), tc.DeepEquals, []hooks.Kind{hooks.Reconcile})
}

func (s *LifecycleSuite) TestDyingUnitStopsThenRemovesBeforeTermination(c *tc.C) {
	store := &testLifecycleStore{state: LifecycleState{Installed: true, Started: true}}
	planner, err := NewPersistentLifecyclePlanner(c.Context(), store)
	c.Assert(err, tc.ErrorIsNil)
	snapshot := params.UnitSnapshot{Life: life.Dying}

	c.Check(planner.Plan(snapshot), tc.DeepEquals, []hooks.Kind{hooks.Stop, hooks.Remove})
	c.Assert(planner.Complete(c.Context(), hooks.Stop, snapshot), tc.ErrorIsNil)
	c.Assert(planner.Complete(c.Context(), hooks.Remove, snapshot), tc.ErrorIsNil)

	c.Check(planner.Terminated(), tc.IsTrue)
}

func (s *LifecycleSuite) TestHandleStagesBeforeDistinctDispatches(c *tc.C) {
	deployer := &testDeployer{done: make(chan struct{})}
	events := make(chan hooks.Kind, 2)
	strategy, err := NewLifecycleStrategy(StrategyConfig{
		Planner: NewLifecyclePlanner(),
		Charm: func(context.Context, params.UnitSnapshot) (charm.BundleInfo, error) {
			return testBundleInfo{url: "charmhub/example"}, nil
		},
		Deployer: deployer,
		Dispatch: func(_ context.Context, event hooks.Kind, _ params.UnitSnapshot) error {
			events <- event
			return nil
		},
	})
	c.Assert(err, tc.ErrorIsNil)

	err = strategy.Handle(c.Context(), params.UnitSnapshot{CharmURL: "charmhub/example"})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(deployer.staged, tc.Equals, 1)
	c.Check(deployer.deployed, tc.IsTrue)
	c.Check(<-events, tc.Equals, hooks.Install)
	c.Check(<-events, tc.Equals, hooks.Start)
}

func (s *LifecycleSuite) TestHandleDoesNotRedeployUnchangedCharm(c *tc.C) {
	deployer := &testDeployer{done: make(chan struct{})}
	strategy, err := NewLifecycleStrategy(StrategyConfig{
		Planner: NewLifecyclePlanner(),
		Charm: func(context.Context, params.UnitSnapshot) (charm.BundleInfo, error) {
			return testBundleInfo{url: "charmhub/example"}, nil
		},
		Deployer: deployer,
		Dispatch: func(context.Context, hooks.Kind, params.UnitSnapshot) error { return nil },
	})
	c.Assert(err, tc.ErrorIsNil)

	snapshot := params.UnitSnapshot{CharmURL: "charmhub/example"}
	err = strategy.Handle(c.Context(), snapshot)
	c.Assert(err, tc.ErrorIsNil)
	err = strategy.Handle(c.Context(), snapshot)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(deployer.staged, tc.Equals, 1)
}

func (s *LifecycleSuite) TestHandleRedeploysModifiedCharm(c *tc.C) {
	deployer := &testDeployer{done: make(chan struct{})}
	strategy, err := NewLifecycleStrategy(StrategyConfig{
		Planner: NewLifecyclePlanner(),
		Charm: func(context.Context, params.UnitSnapshot) (charm.BundleInfo, error) {
			return testBundleInfo{url: "charmhub/example"}, nil
		},
		Deployer: deployer,
		Dispatch: func(context.Context, hooks.Kind, params.UnitSnapshot) error { return nil },
	})
	c.Assert(err, tc.ErrorIsNil)

	snapshot := params.UnitSnapshot{CharmURL: "charmhub/example", CharmModifiedVersion: 1}
	c.Assert(strategy.Handle(c.Context(), snapshot), tc.ErrorIsNil)
	snapshot.CharmModifiedVersion = 2
	c.Assert(strategy.Handle(c.Context(), snapshot), tc.ErrorIsNil)
	c.Check(deployer.staged, tc.Equals, 2)
}

func (s *LifecycleSuite) TestHandleDoesNotStageForSnapshotReconcile(c *tc.C) {
	deployer := &testDeployer{done: make(chan struct{})}
	strategy, err := NewLifecycleStrategy(StrategyConfig{
		Planner: NewLifecyclePlanner(),
		Charm: func(context.Context, params.UnitSnapshot) (charm.BundleInfo, error) {
			return testBundleInfo{url: "charmhub/example"}, nil
		},
		Deployer: deployer,
		Dispatch: func(context.Context, hooks.Kind, params.UnitSnapshot) error { return nil },
	})
	c.Assert(err, tc.ErrorIsNil)

	snapshot := params.UnitSnapshot{CharmURL: "charmhub/example", Config: map[string]any{"key": "one"}}
	err = strategy.Handle(c.Context(), snapshot)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(deployer.staged, tc.Equals, 1)

	snapshot.Config["key"] = "two"
	err = strategy.Handle(c.Context(), snapshot)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(deployer.staged, tc.Equals, 1)
}

func (s *LifecycleSuite) TestHandleDoesNotStageForStopRemove(c *tc.C) {
	deployer := &testDeployer{done: make(chan struct{})}
	store := &testLifecycleStore{state: LifecycleState{Installed: true, Started: true}}
	planner, err := NewPersistentLifecyclePlanner(c.Context(), store)
	c.Assert(err, tc.ErrorIsNil)

	strategy, err := NewLifecycleStrategy(StrategyConfig{
		Planner: planner,
		Charm: func(context.Context, params.UnitSnapshot) (charm.BundleInfo, error) {
			return testBundleInfo{url: "charmhub/example"}, nil
		},
		Deployer: deployer,
		Dispatch: func(context.Context, hooks.Kind, params.UnitSnapshot) error { return nil },
	})
	c.Assert(err, tc.ErrorIsNil)

	err = strategy.Handle(c.Context(), params.UnitSnapshot{Life: life.Dying})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(deployer.staged, tc.Equals, 0)
}

type testLifecycleStore struct {
	state LifecycleState
	saves int
}

func (s *testLifecycleStore) Load(context.Context) (LifecycleState, error) {
	return s.state, nil
}

func (s *testLifecycleStore) Save(_ context.Context, state LifecycleState) error {
	s.state = state
	s.saves++
	return nil
}
