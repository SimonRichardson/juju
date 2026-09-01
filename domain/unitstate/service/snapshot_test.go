// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"
	"testing"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/tc"

	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/core/watcher/eventsource"
	"github.com/juju/juju/domain/unitstate"
	loggertesting "github.com/juju/juju/internal/logger/testing"
)

type snapshotSuite struct {
	serviceSuite
}

func TestSnapshotSuite(t *testing.T) {
	tc.Run(t, &snapshotSuite{})
}

func (s *snapshotSuite) TestWatchUnitSnapshot(c *tc.C) {
	defer s.setupMocks(c).Finish()

	unitName := coreunit.Name("app/0")
	identifiers := unitstate.SnapshotWatchIdentifiers{
		UnitUUID:               "unit-uuid",
		ApplicationUUID:        "application-uuid",
		CharmUUID:              "charm-uuid",
		NetNodeUUIDs:           []string{"net-node-uuid"},
		RelationUUIDs:          []string{"relation-uuid"},
		RelationUnitUUIDs:      []string{"relation-unit-uuid"},
		RelationEndpointUUIDs:  []string{"relation-endpoint-uuid"},
		StorageAttachmentUUIDs: []string{"storage-attachment-uuid"},
	}
	s.st.EXPECT().GetUnitSnapshotWatchIdentifiers(c.Context(), unitName).Return(
		identifiers, nil,
	)
	factory := &snapshotWatcherFactory{}
	svc := NewLeadershipService(
		s.st,
		nil,
		nil,
		nil,
		clock.WallClock,
		loggertesting.WrapCheckLog(c),
		factory,
	)

	got, err := svc.WatchUnitSnapshot(c.Context(), unitName)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.Equals, factory.watcher)
	c.Check(factory.summary, tc.Equals, `unit snapshot watcher for "app/0"`)
	c.Assert(factory.filters, tc.HasLen, 20)

	filters := make(map[string]eventsource.FilterOption, len(factory.filters))
	for _, filter := range factory.filters {
		filters[filter.Namespace()] = filter
	}
	assertPredicate := func(namespace, matching, nonMatching string) {
		filter, ok := filters[namespace]
		c.Assert(ok, tc.IsTrue, tc.Commentf("missing %q filter", namespace))
		c.Check(filter.ChangePredicate()(matching), tc.IsTrue,
			tc.Commentf("%q should match %q", namespace, matching))
		c.Check(filter.ChangePredicate()(nonMatching), tc.IsFalse,
			tc.Commentf("%q should not match %q", namespace, nonMatching))
	}
	assertPredicate("application_config_hash", identifiers.ApplicationUUID, "other")
	assertPredicate("ip_address", identifiers.NetNodeUUIDs[0], "other")
	assertPredicate("relation_unit", identifiers.UnitUUID, identifiers.RelationUnitUUIDs[0])
	assertPredicate("unit_state_charm", identifiers.UnitUUID, "other")
	assertPredicate("unit_workload_version", identifiers.UnitUUID, "other")
	assertPredicate("custom_unit_workload_status", identifiers.ApplicationUUID, identifiers.UnitUUID)
	assertPredicate("custom_storage_attachment_unit_uuid_lifecycle", identifiers.UnitUUID, "other")
	assertPredicate("custom_storage_attachment_entities_storage_attachment_uuid",
		identifiers.StorageAttachmentUUIDs[0], "other")
}

func (s *snapshotSuite) TestUnitSnapshot(c *tc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()

	unitName := coreunit.Name("app/0")
	ensurer := NewMockEnsurer(ctrl)
	expected := unitstate.UnitSnapshot{
		UnitName:        unitName.String(),
		ApplicationName: "app",
		UnitUUID:        "unit-uuid",
		Leader:          true,
	}
	stateSnapshot := expected
	stateSnapshot.Leader = false
	s.st.EXPECT().GetUnitSnapshot(c.Context(), unitName).Return(stateSnapshot, nil)
	ensurer.EXPECT().LeadershipCheck("app", "app/0").Return(snapshotToken{})

	svc := NewLeadershipService(
		s.st,
		nil,
		nil,
		ensurer,
		clock.WallClock,
		loggertesting.WrapCheckLog(c),
		nil,
	)
	actual, err := svc.UnitSnapshot(c.Context(), unitName)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(actual, tc.DeepEquals, expected)
}

func (s *snapshotSuite) TestUnitSnapshotNotLeader(c *tc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()

	unitName := coreunit.Name("app/0")
	ensurer := NewMockEnsurer(ctrl)
	s.st.EXPECT().GetUnitSnapshot(c.Context(), unitName).Return(unitstate.UnitSnapshot{
		UnitName:        unitName.String(),
		ApplicationName: "app",
	}, nil)
	ensurer.EXPECT().LeadershipCheck("app", "app/0").Return(snapshotToken{err: errors.New("not leader")})

	svc := NewLeadershipService(
		s.st, nil, nil, ensurer, clock.WallClock,
		loggertesting.WrapCheckLog(c), nil,
	)
	actual, err := svc.UnitSnapshot(c.Context(), unitName)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(actual.Leader, tc.IsFalse)
}

type snapshotToken struct {
	err error
}

func (t snapshotToken) Check() error {
	return t.err
}

type snapshotWatcherFactory struct {
	watcher watcher.NotifyWatcher
	summary string
	filters []eventsource.FilterOption
}

func (f *snapshotWatcherFactory) NewNotifyWatcher(
	_ context.Context,
	summary string,
	filter eventsource.FilterOption,
	filters ...eventsource.FilterOption,
) (watcher.NotifyWatcher, error) {
	f.summary = summary
	f.filters = append([]eventsource.FilterOption{filter}, filters...)
	f.watcher = watcher.TODO[struct{}]()
	return f.watcher, nil
}
