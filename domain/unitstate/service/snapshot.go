// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"
	"fmt"

	"github.com/juju/juju/core/changestream"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/core/watcher/eventsource"
	"github.com/juju/juju/domain/unitstate"
	"github.com/juju/juju/internal/errors"
)

// UnitSnapshot returns the model-database projection used to construct a
// holistic unit snapshot.
func (s *LeadershipService) UnitSnapshot(ctx context.Context, unitName coreunit.Name) (unitstate.UnitSnapshot, error) {
	if err := unitName.Validate(); err != nil {
		return unitstate.UnitSnapshot{}, errors.Capture(err)
	}
	return s.st.GetUnitSnapshot(ctx, unitName)
}

// WatchUnitSnapshot watches every model row that contributes to the named
// unit's snapshot.
func (s *LeadershipService) WatchUnitSnapshot(ctx context.Context, unitName coreunit.Name) (watcher.NotifyWatcher, error) {
	identifiers, err := s.st.GetUnitSnapshotWatchIdentifiers(ctx, unitName)
	if err != nil {
		return nil, errors.Errorf("getting snapshot watch identifiers for unit %q: %w", unitName, err)
	}

	return s.watcherFactory.NewNotifyWatcher(
		ctx,
		fmt.Sprintf("unit snapshot watcher for %q", unitName),
		eventsource.PredicateFilter("unit", changestream.All, eventsource.EqualsPredicate(identifiers.UnitUUID)),
		eventsource.PredicateFilter("unit_principal", changestream.All, eventsource.EqualsPredicate(identifiers.UnitUUID)),
		eventsource.PredicateFilter("unit_resolved", changestream.All, eventsource.EqualsPredicate(identifiers.UnitUUID)),
		eventsource.PredicateFilter("application", changestream.All, eventsource.EqualsPredicate(identifiers.ApplicationUUID)),
		eventsource.PredicateFilter("application_config", changestream.All, eventsource.EqualsPredicate(identifiers.ApplicationUUID)),
		eventsource.PredicateFilter("application_setting", changestream.All, eventsource.EqualsPredicate(identifiers.ApplicationUUID)),
		eventsource.PredicateFilter("application_scale", changestream.All, eventsource.EqualsPredicate(identifiers.ApplicationUUID)),
		eventsource.PredicateFilter("charm", changestream.All, eventsource.EqualsPredicate(identifiers.CharmUUID)),
		eventsource.PredicateFilter("net_node_address", changestream.All, eventsource.ContainsPredicate(identifiers.NetNodeUUIDs)),
		eventsource.PredicateFilter("relation", changestream.All, eventsource.ContainsPredicate(identifiers.RelationUUIDs)),
		eventsource.PredicateFilter("relation_unit", changestream.All, eventsource.ContainsPredicate(identifiers.RelationUnitUUIDs)),
		eventsource.PredicateFilter("relation_unit_settings_hash", changestream.All, eventsource.ContainsPredicate(identifiers.RelationUnitUUIDs)),
		eventsource.PredicateFilter("relation_application_settings_hash", changestream.All, eventsource.ContainsPredicate(identifiers.RelationEndpointUUIDs)),
	)
}
