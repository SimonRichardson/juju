// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload_test

import (
	"context"
	"testing"

	"github.com/canonical/gomock/gomock"
	"github.com/juju/clock"
	"github.com/juju/names/v6"
	"github.com/juju/tc"
	"github.com/juju/worker/v5/workertest"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/api"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/core/watcher/watchertest"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	coretesting "github.com/juju/juju/internal/testing"
	"github.com/juju/juju/internal/worker/fortress"
	"github.com/juju/juju/internal/worker/ociworkload"
)

type manifoldSuite struct {
	coretesting.BaseSuite
}

// TestManifoldSuite exercises the unit-scoped manifold dependencies.
func TestManifoldSuite(t *testing.T) {
	tc.Run(t, &manifoldSuite{})
}

// TestDependencies checks that assignment, API and charm-directory access are
// required before starting the OCI workload worker.
func (s *manifoldSuite) TestDependencies(c *tc.C) {
	manifold := ociworkload.Manifold(ociworkload.ManifoldConfig{
		AgentName:     "agent",
		APICallerName: "api-caller",
		CharmDirName:  "charm-dir",
	})
	c.Check(manifold.Inputs, tc.DeepEquals, []string{"agent", "api-caller", "charm-dir"})
}

// TestRejectsInvalidConfigBeforeGettingDependencies checks that the manifold
// rejects incomplete configuration without accessing the dependency engine.
func (s *manifoldSuite) TestRejectsInvalidConfigBeforeGettingDependencies(c *tc.C) {
	manifold := ociworkload.Manifold(ociworkload.ManifoldConfig{
		AgentName:     "agent",
		APICallerName: "api-caller",
		CharmDirName:  "charm-dir",
	})
	_, err := manifold.Start(c.Context(), nil)
	c.Assert(err, tc.ErrorMatches, "OCI workload manifold requires a clock and logger")
}

// TestStartsForAssignedUnit checks that a unit-scoped API is constructed and
// that the worker subscribes to its application's changes.
func (s *manifoldSuite) TestStartsForAssignedUnit(c *tc.C) {
	ctrl := gomock.NewController(c)
	unitTag := names.NewUnitTag("redis/0")
	config := NewMockConfig(ctrl)
	config.EXPECT().Tag().Return(unitTag)
	config.EXPECT().DataDir().Return(c.MkDir())

	a := NewMockAgent(ctrl)
	a.EXPECT().CurrentConfig().Return(config)
	conn := NewMockConnection(ctrl)
	guest := NewMockGuest(ctrl)
	getter := NewMockGetter(ctrl)
	getter.EXPECT().Get("agent", gomock.Any()).DoAndReturn(func(_ string, out any) error {
		*(out.(*agent.Agent)) = a
		return nil
	})
	getter.EXPECT().Get("api-caller", gomock.Any()).DoAndReturn(func(_ string, out any) error {
		*(out.(*api.Connection)) = conn
		return nil
	})
	getter.EXPECT().Get("charm-dir", gomock.Any()).DoAndReturn(func(_ string, out any) error {
		*(out.(*fortress.Guest)) = guest
		return nil
	})

	changes := make(chan struct{})
	watching := make(chan struct{}, 1)
	app := NewMockApplication(ctrl)
	app.EXPECT().Watch(gomock.Any()).DoAndReturn(func(context.Context) (watcher.NotifyWatcher, error) {
		watching <- struct{}{}
		return watchertest.NewMockNotifyWatcher(changes), nil
	})
	resources := NewMockResourceReader(ctrl)
	puller := NewMockImagePuller(ctrl)
	constructed := false
	manifold := ociworkload.Manifold(ociworkload.ManifoldConfig{
		AgentName:     "agent",
		APICallerName: "api-caller",
		CharmDirName:  "charm-dir",
		Clock:         clock.WallClock,
		Logger:        loggertesting.WrapCheckLog(c),
		Puller:        puller,
		NewClients: func(_ context.Context, _ api.Connection, tag names.UnitTag) (ociworkload.Application, ociworkload.ResourceReader, error) {
			c.Check(tag, tc.Equals, unitTag)
			constructed = true
			return app, resources, nil
		},
	})

	w, err := manifold.Start(c.Context(), getter)
	c.Assert(err, tc.ErrorIsNil)
	defer workertest.CleanKill(c, w)
	c.Check(constructed, tc.IsTrue)
	select {
	case <-watching:
	case <-c.Context().Done():
		c.Fatal("application watcher was not started")
	}
}

// TestRejectsMachineIdentity ensures a machine agent cannot obtain a unit's
// image resources by running this manifold outside a nested unit engine.
func (s *manifoldSuite) TestRejectsMachineIdentity(c *tc.C) {
	ctrl := gomock.NewController(c)
	config := NewMockConfig(ctrl)
	config.EXPECT().Tag().Return(names.NewMachineTag("0"))

	a := NewMockAgent(ctrl)
	a.EXPECT().CurrentConfig().Return(config)
	getter := NewMockGetter(ctrl)
	getter.EXPECT().Get("agent", gomock.Any()).DoAndReturn(func(_ string, out any) error {
		*(out.(*agent.Agent)) = a
		return nil
	})
	getter.EXPECT().Get("api-caller", gomock.Any()).DoAndReturn(func(_ string, out any) error {
		*(out.(*api.Connection)) = NewMockConnection(ctrl)
		return nil
	})
	getter.EXPECT().Get("charm-dir", gomock.Any()).DoAndReturn(func(_ string, out any) error {
		*(out.(*fortress.Guest)) = NewMockGuest(ctrl)
		return nil
	})

	manifold := ociworkload.Manifold(ociworkload.ManifoldConfig{
		AgentName:     "agent",
		APICallerName: "api-caller",
		CharmDirName:  "charm-dir",
		Clock:         clock.WallClock,
		Logger:        loggertesting.WrapCheckLog(c),
	})
	_, err := manifold.Start(c.Context(), getter)
	c.Assert(err, tc.ErrorMatches, "OCI workload worker requires a unit agent")
}
