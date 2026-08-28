// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"
	"testing"

	"github.com/juju/clock"
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
	s.st.EXPECT().GetUnitSnapshotWatchIdentifiers(c.Context(), unitName).Return(
		unitstate.SnapshotWatchIdentifiers{UnitUUID: "unit-uuid"}, nil,
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
	c.Check(factory.filters, tc.HasLen, 13)
}

func (s *snapshotSuite) TestUnitSnapshot(c *tc.C) {
	defer s.setupMocks(c).Finish()

	unitName := coreunit.Name("app/0")
	expected := unitstate.UnitSnapshot{
		UnitName:        unitName.String(),
		ApplicationName: "app",
		UnitUUID:        "unit-uuid",
	}
	s.st.EXPECT().GetUnitSnapshot(c.Context(), unitName).Return(expected, nil)

	svc := NewLeadershipService(
		s.st,
		nil,
		nil,
		nil,
		clock.WallClock,
		loggertesting.WrapCheckLog(c),
		nil,
	)
	actual, err := svc.UnitSnapshot(c.Context(), unitName)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(actual, tc.DeepEquals, expected)
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
