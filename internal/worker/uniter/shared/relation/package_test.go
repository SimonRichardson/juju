// Copyright 2014, 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package relation_test

//go:generate go run github.com/canonical/gomock/mockgen -package mocks -destination mocks/mock_statetracker.go github.com/juju/juju/internal/worker/uniter/shared/relation RelationStateTracker
//go:generate go run github.com/canonical/gomock/mockgen -package mocks -destination mocks/mock_relationer.go github.com/juju/juju/internal/worker/uniter/shared/relation Relationer
//go:generate go run github.com/canonical/gomock/mockgen -package mocks -destination mocks/mock_subordinate_destroyer.go github.com/juju/juju/internal/worker/uniter/shared/relation SubordinateDestroyer
//go:generate go run github.com/canonical/gomock/mockgen -package mocks -destination mocks/mock_state_tracker.go github.com/juju/juju/internal/worker/uniter/shared/relation StateTrackerClient
//go:generate go run github.com/canonical/gomock/mockgen -package mocks -destination mocks/mock_state_manager.go github.com/juju/juju/internal/worker/uniter/shared/relation StateManager
//go:generate go run github.com/canonical/gomock/mockgen -package mocks -destination mocks/mock_unit_getter.go github.com/juju/juju/internal/worker/uniter/shared/relation UnitGetter
