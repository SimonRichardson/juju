// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package holisticuniter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/juju/errors"

	"github.com/juju/juju/core/life"
	"github.com/juju/juju/domain/deployment/charm/hooks"
	"github.com/juju/juju/internal/worker/fortress"
	charm "github.com/juju/juju/internal/worker/uniter/shared/charm"
	"github.com/juju/juju/rpc/params"
)

// EventPlanner selects the distinct lifecycle events to dispatch for a
// snapshot. Implementations must persist their state using the same durability
// guarantees as deltauniter's local operation state.
type EventPlanner interface {
	// Plan returns the next ordered lifecycle events for a snapshot.
	Plan(params.UnitSnapshot) []hooks.Kind
	// Pending returns an event recorded before dispatch, if any.
	Pending() hooks.Kind
	// Begin records an event before it is dispatched.
	Begin(context.Context, hooks.Kind, params.UnitSnapshot) error
	// Retry clears the pending marker without marking the event complete.
	Retry(context.Context) error
	// Complete records a successfully handled lifecycle event.
	Complete(context.Context, hooks.Kind, params.UnitSnapshot) error
	// Terminated reports whether teardown has completed.
	Terminated() bool
}

// LifecycleStore persists lifecycle progress in the controller-backed unit
// state. It deliberately contains execution progress, not a unit snapshot.
type LifecycleStore interface {
	// Load returns durable lifecycle progress for the unit.
	Load(context.Context) (LifecycleState, error)
	// Save records durable lifecycle progress for the unit.
	Save(context.Context, LifecycleState) error
}

// LifecycleState is the holistic equivalent of deltauniter's operation state.
// It records only the completed lifecycle work and compact comparison values
// needed to select the next event after a restart.
type LifecycleState struct {
	Installed            bool       `yaml:"installed"`
	Started              bool       `yaml:"started"`
	Stopped              bool       `yaml:"stopped"`
	Removed              bool       `yaml:"removed"`
	CharmURL             string     `yaml:"charm-url,omitempty"`
	CharmModifiedVersion int        `yaml:"charm-modified-version,omitempty"`
	SnapshotHash         string     `yaml:"snapshot-hash,omitempty"`
	Pending              hooks.Kind `yaml:"pending,omitempty"`
}

// LifecyclePlanner preserves distinct setup and teardown events while using
// reconcile for snapshot-derived steady-state changes.
type LifecyclePlanner struct {
	state LifecycleState
	store LifecycleStore
}

// NewLifecyclePlanner returns a planner for a new unit runtime.
func NewLifecyclePlanner() *LifecyclePlanner { return &LifecyclePlanner{} }

// NewPersistentLifecyclePlanner loads lifecycle progress saved by an earlier
// holistic worker. This is equivalent to deltauniter loading UniterState.
func NewPersistentLifecyclePlanner(ctx context.Context, store LifecycleStore) (*LifecyclePlanner, error) {
	if store == nil {
		return nil, errors.NotValidf("missing lifecycle store")
	}
	state, err := store.Load(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "loading lifecycle state")
	}
	return &LifecyclePlanner{state: state, store: store}, nil
}

// Plan returns one or more distinct events in their required order.
func (p *LifecyclePlanner) Plan(snapshot params.UnitSnapshot) []hooks.Kind {
	if p.state.Pending != "" {
		return []hooks.Kind{p.state.Pending}
	}
	if snapshot.Life == life.Dead {
		return nil
	}
	if snapshot.Life == life.Dying {
		events := make([]hooks.Kind, 0, 2)
		if p.state.Started && !p.state.Stopped {
			events = append(events, hooks.Stop)
		}
		if p.state.Installed && !p.state.Removed {
			events = append(events, hooks.Remove)
		}
		return events
	}
	if !p.state.Installed {
		return []hooks.Kind{hooks.Install, hooks.Start}
	}
	if snapshot.CharmURL != "" && (snapshot.CharmURL != p.state.CharmURL ||
		snapshot.CharmModifiedVersion != p.state.CharmModifiedVersion) {
		return []hooks.Kind{hooks.Reconcile}
	}
	if !p.state.Started {
		return []hooks.Kind{hooks.Start}
	}
	if p.state.SnapshotHash != snapshotReconcileHash(snapshot) {
		return []hooks.Kind{hooks.Reconcile}
	}
	return nil
}

// Pending returns the event recorded before dispatch, if any.
func (p *LifecyclePlanner) Pending() hooks.Kind {
	return p.state.Pending
}

// Terminated reports whether the remove event completed.
func (p *LifecyclePlanner) Terminated() bool {
	return p.state.Removed
}

// Begin records a hook before it is dispatched. If the agent stops before the
// hook completes, the next worker resumes this exact event.
func (p *LifecyclePlanner) Begin(ctx context.Context, event hooks.Kind, _ params.UnitSnapshot) error {
	p.state.Pending = event
	if p.store == nil {
		return nil
	}
	if err := p.store.Save(ctx, p.state); err != nil {
		return errors.Annotatef(err, "recording pending holistic %s event", event)
	}
	return nil
}

// Retry clears the pending marker without recording successful completion.
func (p *LifecyclePlanner) Retry(ctx context.Context) error {
	p.state.Pending = ""
	if p.store == nil {
		return nil
	}
	if err := p.store.Save(ctx, p.state); err != nil {
		return errors.Annotate(err, "recording holistic hook retry")
	}
	return nil
}

// Complete advances lifecycle state after an event succeeds.
func (p *LifecyclePlanner) Complete(ctx context.Context, event hooks.Kind, snapshot params.UnitSnapshot) error {
	p.state.Pending = ""
	switch event {
	case hooks.Install:
		p.state.Installed = true
		p.state.Removed = false
	case hooks.Reconcile:
		p.state.SnapshotHash = snapshotReconcileHash(snapshot)
	case hooks.Start:
		p.state.Started = true
		p.state.Stopped = false
		p.state.SnapshotHash = snapshotReconcileHash(snapshot)
	case hooks.Stop:
		p.state.Stopped = true
	case hooks.Remove:
		p.state.Removed = true
	}
	if snapshot.CharmURL != "" {
		p.state.CharmURL = snapshot.CharmURL
		p.state.CharmModifiedVersion = snapshot.CharmModifiedVersion
	}
	if p.store == nil {
		return nil
	}
	if err := p.store.Save(ctx, p.state); err != nil {
		return errors.Annotate(err, "saving lifecycle state")
	}
	return nil
}

func snapshotReconcileHash(snapshot params.UnitSnapshot) string {
	config, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(config)
	return hex.EncodeToString(hash[:])
}

var _ EventPlanner = (*LifecyclePlanner)(nil)

// PendingEventResolver manages the persisted failed event for a strategy.
type PendingEventResolver interface {
	// PendingEvent returns the event awaiting retry or explicit resolution.
	PendingEvent() hooks.Kind
	// RetryPending makes the failed event eligible for another dispatch.
	RetryPending(context.Context) error
	// SkipPending records the failed event as resolved without dispatching it.
	SkipPending(context.Context, params.UnitSnapshot) error
}

// TerminationStrategy reports that teardown events completed and the worker
// may terminate its unit agent.
type TerminationStrategy interface {
	// Terminated reports whether teardown has completed.
	Terminated() bool
}

// Strategy handles a snapshot after the worker has received a notification.
type Strategy interface {
	// Handle converges the unit for the supplied current snapshot.
	Handle(context.Context, params.UnitSnapshot) error
}

// StrategyConfig contains the dependencies for LifecycleStrategy.
type StrategyConfig struct {
	Planner       EventPlanner
	Charm         CharmProvider
	Deployer      charm.Deployer
	Dispatch      DispatchFunc
	CharmDirGuard fortress.Guard
}

// Validate checks lifecycle strategy dependencies.
func (c StrategyConfig) Validate() error {
	if c.Planner == nil {
		return errors.NotValidf("missing event planner")
	}
	if c.Dispatch == nil {
		return errors.NotValidf("missing dispatch function")
	}
	if (c.Charm == nil) != (c.Deployer == nil) {
		return errors.NotValidf("charm and deployer must be configured together")
	}
	return nil
}

// LifecycleStrategy preserves delta-uniter charm preparation and lifecycle
// ordering while supplying each event with the holistic snapshot.
type LifecycleStrategy struct {
	config                       StrategyConfig
	deployedCharmURL             string
	deployedCharmModifiedVersion int
}

// PendingEvent returns the event awaiting retry or explicit resolution.
func (s *LifecycleStrategy) PendingEvent() hooks.Kind {
	return s.config.Planner.Pending()
}

// Terminated reports whether the planner completed the remove event.
func (s *LifecycleStrategy) Terminated() bool {
	return s.config.Planner.Terminated()
}

// RetryPending makes the failed event eligible for another dispatch.
func (s *LifecycleStrategy) RetryPending(ctx context.Context) error {
	return s.config.Planner.Retry(ctx)
}

// SkipPending records the pending event as resolved without dispatching it.
func (s *LifecycleStrategy) SkipPending(ctx context.Context, snapshot params.UnitSnapshot) error {
	event := s.config.Planner.Pending()
	if event == "" {
		return errors.NotFoundf("pending holistic event")
	}
	return s.config.Planner.Complete(ctx, event, snapshot)
}

// NewLifecycleStrategy constructs a lifecycle strategy.
func NewLifecycleStrategy(config StrategyConfig) (*LifecycleStrategy, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}
	return &LifecycleStrategy{config: config}, nil
}

// Handle stages a changed charm before dispatching its planned events.
func (s *LifecycleStrategy) Handle(ctx context.Context, snapshot params.UnitSnapshot) error {
	if s.config.Charm != nil {
		info, err := s.config.Charm(ctx, snapshot)
		if err != nil {
			return errors.Annotate(err, "getting charm bundle information")
		}
		if info.URL() != s.deployedCharmURL ||
			snapshot.CharmModifiedVersion != s.deployedCharmModifiedVersion {
			if s.config.CharmDirGuard != nil {
				if err := s.config.CharmDirGuard.Lockdown(ctx); err != nil {
					return errors.Annotate(err, "locking down charm directory")
				}
			}
			if err := s.config.Deployer.Stage(ctx, info); err != nil {
				return errors.Annotate(err, "staging charm")
			}
			if err := s.config.Deployer.Deploy(); err != nil {
				return errors.Annotate(err, "deploying charm")
			}
			s.deployedCharmURL = info.URL()
			s.deployedCharmModifiedVersion = snapshot.CharmModifiedVersion
		}
	}
	for _, event := range s.config.Planner.Plan(snapshot) {
		if err := s.config.Planner.Begin(ctx, event, snapshot); err != nil {
			return errors.Annotatef(err, "recording pending holistic %s event", event)
		}
		if err := s.config.Dispatch(ctx, event, snapshot); err != nil {
			return errors.Annotatef(err, "dispatching holistic %s event", event)
		}
		if err := s.config.Planner.Complete(ctx, event, snapshot); err != nil {
			return errors.Annotatef(err, "recording completed holistic %s event", event)
		}
		if s.config.CharmDirGuard != nil && (event == hooks.Start || event == hooks.Reconcile) {
			if err := s.config.CharmDirGuard.Unlock(ctx); err != nil {
				return errors.Annotate(err, "unlocking charm directory")
			}
		}
	}
	return nil
}

var _ Strategy = (*LifecycleStrategy)(nil)
