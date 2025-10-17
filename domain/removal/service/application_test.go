// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"testing"
	"time"

	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	coreapplication "github.com/juju/juju/core/application"
	applicationtesting "github.com/juju/juju/core/application/testing"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	"github.com/juju/juju/domain/life"
	removal "github.com/juju/juju/domain/removal"
	removalerrors "github.com/juju/juju/domain/removal/errors"
	"github.com/juju/juju/domain/removal/internal"
	"github.com/juju/juju/internal/errors"
)

type applicationSuite struct {
	baseSuite
}

func TestApplicationSuite(t *testing.T) {
	tc.Run(t, &applicationSuite{})
}

func (s *applicationSuite) TestRemoveApplicationDestroyStorageNoForceSuccess(c *tc.C) {
	defer s.setupMocks(c).Finish()

	appUUID := tc.Must(c, coreapplication.NewID)

	when := time.Now()
	s.clock.EXPECT().Now().Return(when)

	exp := s.modelState.EXPECT()
	exp.ApplicationExists(gomock.Any(), appUUID.String()).Return(true, nil)
	exp.EnsureApplicationNotAliveCascade(gomock.Any(), appUUID.String(), true).Return(internal.CascadedApplicationLives{}, nil)
	exp.ApplicationScheduleRemoval(gomock.Any(), gomock.Any(), appUUID.String(), false, when.UTC()).Return(nil)

	jobUUID, err := s.newService(c).RemoveApplication(c.Context(), appUUID, true, false, 0)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(jobUUID.Validate(), tc.ErrorIsNil)
}

func (s *applicationSuite) TestRemoveApplicationForceNoWaitSuccess(c *tc.C) {
	defer s.setupMocks(c).Finish()

	appUUID := applicationtesting.GenApplicationUUID(c)

	when := time.Now()
	s.clock.EXPECT().Now().Return(when)

	exp := s.modelState.EXPECT()
	exp.ApplicationExists(gomock.Any(), appUUID.String()).Return(true, nil)
	exp.EnsureApplicationNotAliveCascade(gomock.Any(), appUUID.String(), false).Return(internal.CascadedApplicationLives{}, nil)
	exp.ApplicationScheduleRemoval(gomock.Any(), gomock.Any(), appUUID.String(), true, when.UTC()).Return(nil)

	jobUUID, err := s.newService(c).RemoveApplication(c.Context(), appUUID, false, true, 0)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(jobUUID.Validate(), tc.ErrorIsNil)
}

func (s *applicationSuite) TestRemoveApplicationForceWaitSuccess(c *tc.C) {
	defer s.setupMocks(c).Finish()

	appUUID := tc.Must(c, coreapplication.NewID)

	when := time.Now()
	s.clock.EXPECT().Now().Return(when).MinTimes(1)

	exp := s.modelState.EXPECT()
	exp.ApplicationExists(gomock.Any(), appUUID.String()).Return(true, nil)
	exp.EnsureApplicationNotAliveCascade(gomock.Any(), appUUID.String(), false).Return(internal.CascadedApplicationLives{}, nil)

	// The first normal removal scheduled immediately.
	exp.ApplicationScheduleRemoval(gomock.Any(), gomock.Any(), appUUID.String(), false, when.UTC()).Return(nil)

	// The forced removal scheduled after the wait duration.
	exp.ApplicationScheduleRemoval(gomock.Any(), gomock.Any(), appUUID.String(), true, when.UTC().Add(time.Minute)).Return(nil)

	jobUUID, err := s.newService(c).RemoveApplication(c.Context(), appUUID, false, true, time.Minute)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(jobUUID.Validate(), tc.ErrorIsNil)
}

func (s *applicationSuite) TestRemoveApplicationNotFound(c *tc.C) {
	defer s.setupMocks(c).Finish()

	appUUID := tc.Must(c, coreapplication.NewID)

	s.modelState.EXPECT().ApplicationExists(gomock.Any(), appUUID.String()).Return(false, nil)

	_, err := s.newService(c).RemoveApplication(c.Context(), appUUID, false, false, 0)
	c.Assert(err, tc.ErrorIs, applicationerrors.ApplicationNotFound)
}

func (s *applicationSuite) TestRemoveApplicationNoForceSuccessWithUnitsAndStorage(c *tc.C) {
	defer s.setupMocks(c).Finish()

	appUUID := tc.Must(c, coreapplication.NewID)

	when := time.Now()
	s.clock.EXPECT().Now().Return(when).MinTimes(1)

	exp := s.modelState.EXPECT()
	exp.ApplicationExists(gomock.Any(), appUUID.String()).Return(true, nil)
	exp.EnsureApplicationNotAliveCascade(gomock.Any(), appUUID.String(), false).Return(internal.CascadedApplicationLives{
		UnitUUIDs:              []string{"unit-1", "unit-2"},
		StorageAttachmentUUIDs: []string{"st-att-unit-1", "st-att-unit-2"},
	}, nil)
	exp.ApplicationScheduleRemoval(gomock.Any(), gomock.Any(), appUUID.String(), false, when.UTC()).Return(nil)

	exp.UnitScheduleRemoval(gomock.Any(), gomock.Any(), "unit-1", false, when.UTC()).Return(nil)
	exp.UnitScheduleRemoval(gomock.Any(), gomock.Any(), "unit-2", false, when.UTC()).Return(nil)
	exp.StorageAttachmentScheduleRemoval(gomock.Any(), gomock.Any(), "st-att-unit-1", false, when.UTC()).Return(nil)
	exp.StorageAttachmentScheduleRemoval(gomock.Any(), gomock.Any(), "st-att-unit-2", false, when.UTC()).Return(nil)

	jobUUID, err := s.newService(c).RemoveApplication(c.Context(), appUUID, false, false, 0)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(jobUUID.Validate(), tc.ErrorIsNil)
}

func (s *applicationSuite) TestRemoveApplicationNoForceSuccessWithMachines(c *tc.C) {
	defer s.setupMocks(c).Finish()

	appUUID := tc.Must(c, coreapplication.NewID)

	when := time.Now()
	s.clock.EXPECT().Now().Return(when).MinTimes(1)

	exp := s.modelState.EXPECT()
	exp.ApplicationExists(gomock.Any(), appUUID.String()).Return(true, nil)
	exp.EnsureApplicationNotAliveCascade(gomock.Any(), appUUID.String(), false).Return(internal.CascadedApplicationLives{
		MachineUUIDs: []string{"machine-1", "machine-2"},
	}, nil)
	exp.ApplicationScheduleRemoval(gomock.Any(), gomock.Any(), appUUID.String(), false, when.UTC()).Return(nil)

	exp.MachineScheduleRemoval(gomock.Any(), gomock.Any(), "machine-1", false, when.UTC()).Return(nil)
	exp.MachineScheduleRemoval(gomock.Any(), gomock.Any(), "machine-2", false, when.UTC()).Return(nil)

	jobUUID, err := s.newService(c).RemoveApplication(c.Context(), appUUID, false, false, 0)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(jobUUID.Validate(), tc.ErrorIsNil)
}

func (s *applicationSuite) TestRemoveApplicationNoForceSuccessWithRelations(c *tc.C) {
	defer s.setupMocks(c).Finish()

	appUUID := tc.Must(c, coreapplication.NewID)

	when := time.Now()
	s.clock.EXPECT().Now().Return(when).MinTimes(1)

	exp := s.modelState.EXPECT()
	exp.ApplicationExists(gomock.Any(), appUUID.String()).Return(true, nil)
	exp.EnsureApplicationNotAliveCascade(gomock.Any(), appUUID.String(), false).Return(internal.CascadedApplicationLives{
		RelationUUIDs: []string{"relation-1", "relation-2"},
	}, nil)
	exp.ApplicationScheduleRemoval(gomock.Any(), gomock.Any(), appUUID.String(), false, when.UTC()).Return(nil)

	exp.RelationScheduleRemoval(gomock.Any(), gomock.Any(), "relation-1", false, when.UTC()).Return(nil)
	exp.RelationScheduleRemoval(gomock.Any(), gomock.Any(), "relation-2", false, when.UTC()).Return(nil)

	jobUUID, err := s.newService(c).RemoveApplication(c.Context(), appUUID, false, false, 0)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(jobUUID.Validate(), tc.ErrorIsNil)
}

func (s *applicationSuite) TestProcessRemovalJobInvalidJobType(c *tc.C) {
	var invalidJobType removal.JobType = 500

	job := removal.Job{
		RemovalType: invalidJobType,
	}

	err := s.newService(c).processApplicationRemovalJob(c.Context(), job)
	c.Check(err, tc.ErrorIs, removalerrors.RemovalJobTypeNotValid)
}

func (s *applicationSuite) TestExecuteJobForApplicationNotFound(c *tc.C) {
	defer s.setupMocks(c).Finish()

	j := newApplicationJob(c)

	exp := s.modelState.EXPECT()
	exp.GetApplicationLife(gomock.Any(), j.EntityUUID).Return(-1, applicationerrors.ApplicationNotFound)
	exp.DeleteJob(gomock.Any(), j.UUID.String()).Return(nil)

	err := s.newService(c).ExecuteJob(c.Context(), j)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *applicationSuite) TestExecuteJobForApplicationError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	j := newApplicationJob(c)

	exp := s.modelState.EXPECT()
	exp.GetApplicationLife(gomock.Any(), j.EntityUUID).Return(-1, errors.Errorf("the front fell off"))

	err := s.newService(c).ExecuteJob(c.Context(), j)
	c.Assert(err, tc.ErrorMatches, ".*the front fell off")
}

func (s *applicationSuite) TestExecuteJobForApplicationStillAlive(c *tc.C) {
	defer s.setupMocks(c).Finish()

	j := newApplicationJob(c)

	exp := s.modelState.EXPECT()
	exp.GetApplicationLife(gomock.Any(), j.EntityUUID).Return(life.Alive, nil)

	err := s.newService(c).ExecuteJob(c.Context(), j)
	c.Assert(err, tc.ErrorIs, removalerrors.EntityStillAlive)
}

func (s *applicationSuite) TestExecuteJobForApplicationDyingDeleteApplication(c *tc.C) {
	defer s.setupMocks(c).Finish()

	j := newApplicationJob(c)

	exp := s.modelState.EXPECT()
	exp.GetApplicationLife(gomock.Any(), j.EntityUUID).Return(life.Dying, nil)
	exp.DeleteApplication(gomock.Any(), j.EntityUUID).Return(nil)
	exp.DeleteJob(gomock.Any(), j.UUID.String()).Return(nil)

	err := s.newService(c).ExecuteJob(c.Context(), j)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *applicationSuite) TestExecuteJobForApplicationDyingDeleteApplicationError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	j := newApplicationJob(c)

	exp := s.modelState.EXPECT()
	exp.GetApplicationLife(gomock.Any(), j.EntityUUID).Return(life.Dying, nil)
	exp.DeleteApplication(gomock.Any(), j.EntityUUID).Return(errors.Errorf("the front fell off"))

	err := s.newService(c).ExecuteJob(c.Context(), j)
	c.Assert(err, tc.ErrorMatches, ".*the front fell off")
}

func newApplicationJob(c *tc.C) removal.Job {
	jUUID, err := removal.NewUUID()
	c.Assert(err, tc.ErrorIsNil)

	return removal.Job{
		UUID:        jUUID,
		RemovalType: removal.ApplicationJob,
		EntityUUID:  tc.Must(c, coreapplication.NewID).String(),
	}
}
