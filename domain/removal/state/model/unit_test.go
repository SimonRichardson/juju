// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/juju/tc"

	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/unit"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	applicationservice "github.com/juju/juju/domain/application/service"
	"github.com/juju/juju/domain/life"
	removalerrors "github.com/juju/juju/domain/removal/errors"
	loggertesting "github.com/juju/juju/internal/logger/testing"
)

type unitSuite struct {
	baseSuite
}

func TestUnitSuite(t *testing.T) {
	tc.Run(t, &unitSuite{})
}

func (s *unitSuite) TestUnitExists(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app", applicationservice.AddIAASUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	exists, err := st.UnitExists(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(exists, tc.Equals, true)

	exists, err = st.UnitExists(c.Context(), "not-today-henry")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(exists, tc.Equals, false)
}

func (s *unitSuite) TestEnsureUnitNotAliveCascadeNormalSuccessLastUnit(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app", applicationservice.AddIAASUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	unitMachineUUID := s.getUnitMachineUUID(c, unitUUID)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	cascade, err := st.EnsureUnitNotAliveCascade(c.Context(), unitUUID.String(), false)
	c.Assert(err, tc.ErrorIsNil)

	c.Assert(*cascade.MachineUUID, tc.Equals, unitMachineUUID.String())

	// Unit had life "alive" and should now be "dying".
	s.checkUnitLife(c, unitUUID.String(), life.Dying)

	// The last machine had life "alive" and should now be "dying".
	s.checkMachineLife(c, unitMachineUUID.String(), life.Dying)
}

func (s *unitSuite) TestEnsureUnitNotAliveCascadeStorageAttachmentsDying(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app", applicationservice.AddIAASUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	ctx := c.Context()

	// Create a storage pool and a storage instance attached to the app's unit.
	err := s.TxnRunner().StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(
			ctx, "INSERT INTO storage_pool (uuid, name, type) VALUES ('pool-uuid', 'pool', 'whatever')",
		); err != nil {
			return err
		}

		inst := `
INSERT INTO storage_instance (
    uuid, storage_id, storage_pool_uuid, storage_kind_id, requested_size_mib,
    charm_name, storage_name, life_id
)
VALUES ('instance-uuid', 'does-not-matter', 'pool-uuid', 1, 100, 'charm-name', 'storage-name', 0)`
		if _, err := tx.ExecContext(ctx, inst); err != nil {
			return err
		}

		attach := `
INSERT INTO storage_attachment (uuid, storage_instance_uuid, unit_uuid, life_id)
VALUES ('storage-attachment-uuid', 'instance-uuid', ?, 0)`
		if _, err := tx.ExecContext(ctx, attach, unitUUID); err != nil {
			return err
		}

		return nil
	})
	c.Assert(err, tc.ErrorIsNil)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	cascade, err := st.EnsureUnitNotAliveCascade(ctx, unitUUID.String(), false)
	c.Assert(err, tc.ErrorIsNil)

	// Unit had life "alive" and should now be "dying".
	s.checkUnitLife(c, unitUUID.String(), life.Dying)

	// Storage attachment should be "dying".
	row := s.DB().QueryRow("SELECT life_id FROM storage_attachment WHERE uuid = 'storage-attachment-uuid'")
	var lifeID int
	err = row.Scan(&lifeID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(lifeID, tc.Equals, 1)

	c.Check(cascade.StorageAttachmentUUIDs, tc.DeepEquals, []string{"storage-attachment-uuid"})
}

func (s *unitSuite) TestEnsureUnitNotAliveDestroyStorage(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app", applicationservice.AddIAASUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	ctx := c.Context()

	// Create a storage pool and a storage instance attached to the app's unit.
	err := s.TxnRunner().StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(
			ctx, "INSERT INTO storage_pool (uuid, name, type) VALUES ('pool-uuid', 'pool', 'whatever')",
		); err != nil {
			return err
		}

		inst := `
INSERT INTO storage_instance (
	uuid, storage_id, storage_pool_uuid, requested_size_mib, charm_name, storage_name, life_id, storage_kind_id
)
VALUES ('instance-uuid', 'does-not-matter', 'pool-uuid', 100, 'charm-name', 'storage-name', 0, 0)`
		if _, err := tx.ExecContext(ctx, inst); err != nil {
			return err
		}

		attach := `
INSERT INTO storage_attachment (uuid, storage_instance_uuid, unit_uuid, life_id)
VALUES ('storage-attachment-uuid', 'instance-uuid', ?, 0)`
		if _, err := tx.ExecContext(ctx, attach, unitUUID); err != nil {
			return err
		}

		owned := "INSERT INTO storage_unit_owner (storage_instance_uuid, unit_uuid) VALUES ('instance-uuid', ?)"
		if _, err := tx.ExecContext(ctx, owned, unitUUID); err != nil {
			return err
		}

		return nil
	})
	c.Assert(err, tc.ErrorIsNil)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	cascade, err := st.EnsureUnitNotAliveCascade(ctx, unitUUID.String(), true)
	c.Assert(err, tc.ErrorIsNil)

	// Unit had life "alive" and should now be "dying".
	s.checkUnitLife(c, unitUUID.String(), life.Dying)

	// Storage attachment should be "dying".
	row := s.DB().QueryRow("SELECT life_id FROM storage_attachment WHERE uuid = 'storage-attachment-uuid'")
	var lifeID int
	err = row.Scan(&lifeID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(lifeID, tc.Equals, 1)

	// Storage instance should be "dying".
	row = s.DB().QueryRow("SELECT life_id FROM storage_instance WHERE uuid = 'instance-uuid'")
	err = row.Scan(&lifeID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(lifeID, tc.Equals, 1)

	c.Check(cascade.StorageAttachmentUUIDs, tc.DeepEquals, []string{"storage-attachment-uuid"})
	c.Check(cascade.StorageInstanceUUIDs, tc.DeepEquals, []string{"instance-uuid"})
}

func (s *unitSuite) TestEnsureUnitNotAliveCascadeNormalSuccessLastUnitParentMachine(c *tc.C) {
	svc := s.setupApplicationService(c)
	app1UUID := s.createIAASApplication(c, svc, "foo",
		applicationservice.AddIAASUnitArg{},
	)
	app2UUID := s.createIAASApplication(c, svc, "bar",
		applicationservice.AddIAASUnitArg{},
	)

	app1UnitUUIDs := s.getAllUnitUUIDs(c, app1UUID)
	c.Assert(len(app1UnitUUIDs), tc.Equals, 1)
	app1UnitUUID := app1UnitUUIDs[0]

	app2UnitUUIDs := s.getAllUnitUUIDs(c, app2UUID)
	c.Assert(len(app2UnitUUIDs), tc.Equals, 1)
	app2UnitUUID := app2UnitUUIDs[0]

	app1UnitMachineUUID := s.getUnitMachineUUID(c, app1UnitUUID)
	app2UnitMachineUUID := s.getUnitMachineUUID(c, app2UnitUUID)

	_, err := s.DB().Exec(`
INSERT INTO machine_parent (machine_uuid, parent_uuid) VALUES (?, ?)
	`, app2UnitMachineUUID.String(), app1UnitMachineUUID.String())
	c.Assert(err, tc.ErrorIsNil)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	cascade, err := st.EnsureUnitNotAliveCascade(c.Context(), app1UnitUUID.String(), false)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(cascade.MachineUUID, tc.IsNil)

	// Unit had life "alive" and should now be "dying".
	s.checkUnitLife(c, app1UnitUUID.String(), life.Dying)

	// The last machine had life "alive" and should be still alive, because
	// it is a parent machine.
	s.checkMachineLife(c, app1UnitMachineUUID.String(), life.Alive)
}

// Test to ensure that we don't prevent a unit from being set to "dying"
// if the machine is already in the "dying" state. This shouldn't happen,
// but we want to ensure that the state machine is resilient to this
// situation.
func (s *unitSuite) TestEnsureUnitNotAliveCascadeNormalSuccessLastUnitMachineAlreadyDying(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app",
		applicationservice.AddIAASUnitArg{},
	)

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	unitMachineUUID := s.getUnitMachineUUID(c, unitUUID)
	// Set the machine to "dying" manually.
	_, err := s.DB().Exec("UPDATE machine SET life_id = 1 WHERE uuid = ?", unitMachineUUID.String())
	c.Assert(err, tc.ErrorIsNil)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	cascade, err := st.EnsureUnitNotAliveCascade(c.Context(), unitUUID.String(), false)
	c.Assert(err, tc.ErrorIsNil)

	// The machine was already "dying", so we don't expect a machine UUID.
	c.Assert(cascade.MachineUUID, tc.IsNil)

	// Unit had life "alive" and should now be "dying".
	s.checkUnitLife(c, unitUUID.String(), life.Dying)
}

func (s *unitSuite) TestEnsureUnitNotAliveCascadeNormalSuccess(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app",
		applicationservice.AddIAASUnitArg{},
	)

	_, _, err := svc.AddIAASUnits(
		c.Context(),
		"some-app",
		applicationservice.AddIAASUnitArg{
			AddUnitArg: applicationservice.AddUnitArg{
				// Place this unit on the same machine as the first one.
				Placement: instance.MustParsePlacement("0"),
			},
		},
	)
	c.Assert(err, tc.ErrorIsNil)

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 2)
	unitUUID := unitUUIDs[0]

	unitMachineUUID := s.getUnitMachineUUID(c, unitUUID)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	cascade, err := st.EnsureUnitNotAliveCascade(c.Context(), unitUUID.String(), false)
	c.Assert(err, tc.ErrorIsNil)

	// This isn't the last unit on the machine, so we don't expect a machine
	// UUID.
	c.Assert(cascade.MachineUUID, tc.IsNil)

	// Unit had life "alive" and should now be "dying".
	s.checkUnitLife(c, unitUUID.String(), life.Dying)

	// Don't set the machine life to "dying" if there are other units on it.
	s.checkMachineLife(c, unitMachineUUID.String(), life.Alive)
}

func (s *unitSuite) TestEnsureUnitNotAliveCascadeDyingSuccess(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app", applicationservice.AddIAASUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	_, err := st.EnsureUnitNotAliveCascade(c.Context(), unitUUID.String(), false)
	c.Assert(err, tc.ErrorIsNil)

	// Unit was already "dying" and should be unchanged.
	s.checkUnitLife(c, unitUUID.String(), life.Dying)
}

func (s *unitSuite) TestEnsureUnitNotAliveCascadeNotExistsSuccess(c *tc.C) {
	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	// We don't care if it's already gone.
	_, err := st.EnsureUnitNotAliveCascade(c.Context(), "some-unit-uuid", false)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *unitSuite) TestUnitRemovalNormalSuccess(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app", applicationservice.AddIAASUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	when := time.Now().UTC()
	err := st.UnitScheduleRemoval(
		c.Context(), "removal-uuid", unitUUID.String(), false, when,
	)
	c.Assert(err, tc.ErrorIsNil)

	// We should have a removal job scheduled immediately.
	row := s.DB().QueryRow(
		"SELECT removal_type_id, entity_uuid, force, scheduled_for FROM removal where uuid = ?",
		"removal-uuid",
	)
	var (
		removalTypeID int
		rUUID         string
		force         bool
		scheduledFor  time.Time
	)
	err = row.Scan(&removalTypeID, &rUUID, &force, &scheduledFor)
	c.Assert(err, tc.ErrorIsNil)

	c.Check(removalTypeID, tc.Equals, 1)
	c.Check(rUUID, tc.Equals, unitUUID.String())
	c.Check(force, tc.Equals, false)
	c.Check(scheduledFor, tc.Equals, when)
}

func (s *unitSuite) TestUnitRemovalNotExistsSuccess(c *tc.C) {
	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	when := time.Now().UTC()
	err := st.UnitScheduleRemoval(
		c.Context(), "removal-uuid", "some-unit-uuid", true, when,
	)
	c.Assert(err, tc.ErrorIsNil)

	// We should have a removal job scheduled immediately.
	// It doesn't matter that the unit does not exist.
	// We rely on the worker to handle that fact.
	row := s.DB().QueryRow(`
SELECT t.name, r.entity_uuid, r.force, r.scheduled_for
FROM   removal r JOIN removal_type t ON r.removal_type_id = t.id
where  r.uuid = ?`, "removal-uuid",
	)

	var (
		removalType  string
		rUUID        string
		force        bool
		scheduledFor time.Time
	)
	err = row.Scan(&removalType, &rUUID, &force, &scheduledFor)
	c.Assert(err, tc.ErrorIsNil)

	c.Check(removalType, tc.Equals, "unit")
	c.Check(rUUID, tc.Equals, "some-unit-uuid")
	c.Check(force, tc.Equals, true)
	c.Check(scheduledFor, tc.Equals, when)
}

func (s *unitSuite) TestGetUnitLifeSuccess(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app", applicationservice.AddIAASUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	l, err := st.GetUnitLife(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(l, tc.Equals, life.Alive)

	// Set the unit to "dying" manually.
	s.advanceUnitLife(c, unitUUID, life.Dying)

	l, err = st.GetUnitLife(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(l, tc.Equals, life.Dying)
}

func (s *unitSuite) TestGetUnitLifeNotFound(c *tc.C) {
	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	_, err := st.GetUnitLife(c.Context(), "some-unit-uuid")
	c.Assert(err, tc.ErrorIs, applicationerrors.UnitNotFound)
}

func (s *unitSuite) TestMarkUnitAsDead(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app", applicationservice.AddIAASUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	err := st.MarkUnitAsDead(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIs, removalerrors.EntityStillAlive)

	_, err = s.DB().Exec("UPDATE unit SET life_id = 1 WHERE uuid = ?", unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)

	err = st.MarkUnitAsDead(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)

	// The unit should now be dead.
	s.checkUnitLife(c, unitUUID.String(), life.Dead)
}

func (s *unitSuite) TestMarkUnitAsDeadNotFound(c *tc.C) {
	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	err := st.MarkUnitAsDead(c.Context(), "abc")
	c.Assert(err, tc.ErrorIs, applicationerrors.UnitNotFound)
}

func (s *unitSuite) TestDeleteUnitNotFound(c *tc.C) {
	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	err := st.DeleteUnit(c.Context(), "blah")
	c.Assert(err, tc.ErrorIs, applicationerrors.UnitNotFound)
}

func (s *unitSuite) TestDeleteIAASUnit(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app", applicationservice.AddIAASUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	s.advanceUnitLife(c, unitUUID, life.Dead)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	err := st.DeleteUnit(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)

	// The unit should be gone.
	exists, err := st.UnitExists(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(exists, tc.Equals, false)

	// The charm isn't removed because the application still references it.
	s.checkCharmsCount(c, 1)
}

func (s *unitSuite) TestDeleteSubordinateUnit(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID1 := s.createIAASApplication(c, svc, "foo", applicationservice.AddIAASUnitArg{})
	appUUID2 := s.createIAASSubordinateApplication(c, svc, "baz", applicationservice.AddIAASUnitArg{})

	// Force the second application to be a subordinate of the first.

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID1)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	subUnitUUIDs := s.getAllUnitUUIDs(c, appUUID2)
	c.Assert(len(subUnitUUIDs), tc.Equals, 1)
	subUnitUUID := subUnitUUIDs[0]

	s.advanceUnitLife(c, subUnitUUID, life.Dead)

	_, err := s.DB().Exec(`INSERT INTO unit_principal (unit_uuid, principal_uuid) VALUES (?, ?)`,
		subUnitUUID.String(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	err = st.DeleteUnit(c.Context(), subUnitUUID.String())
	c.Assert(err, tc.ErrorIsNil)
}

func (s *unitSuite) TestDeleteIAASUnitWithSubordinates(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID1 := s.createIAASApplication(c, svc, "foo", applicationservice.AddIAASUnitArg{})
	appUUID2 := s.createIAASSubordinateApplication(c, svc, "baz", applicationservice.AddIAASUnitArg{})

	// Force the second application to be a subordinate of the first.

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID1)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	subUnitUUIDs := s.getAllUnitUUIDs(c, appUUID2)
	c.Assert(len(subUnitUUIDs), tc.Equals, 1)
	subUnitUUID := subUnitUUIDs[0]

	s.advanceUnitLife(c, unitUUID, life.Dead)

	_, err := s.DB().Exec(`INSERT INTO unit_principal (unit_uuid, principal_uuid) VALUES (?, ?)`,
		subUnitUUID.String(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	err = st.DeleteUnit(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIs, removalerrors.RemovalJobIncomplete)

	_, err = s.DB().Exec(`DELETE FROM unit_principal`)
	c.Assert(err, tc.ErrorIsNil)

	err = st.DeleteUnit(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)

	// The unit should be gone.
	exists, err := st.UnitExists(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(exists, tc.Equals, false)

	// The charm isn't removed because the application still references it.
	s.checkCharmsCount(c, 2)
}

func (s *unitSuite) TestDeleteIAASUnitWithSubordinatesNotDying(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID1 := s.createIAASApplication(c, svc, "foo", applicationservice.AddIAASUnitArg{})
	appUUID2 := s.createIAASSubordinateApplication(c, svc, "baz", applicationservice.AddIAASUnitArg{})

	// Force the second application to be a subordinate of the first.

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID1)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	subUnitUUIDs := s.getAllUnitUUIDs(c, appUUID2)
	c.Assert(len(subUnitUUIDs), tc.Equals, 1)
	subUnitUUID := subUnitUUIDs[0]

	_, err := s.DB().Exec(`INSERT INTO unit_principal (unit_uuid, principal_uuid) VALUES (?, ?)`,
		subUnitUUID.String(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	err = st.DeleteUnit(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorMatches, `.*still alive.*`)
}

func (s *unitSuite) TestDeleteCAASUnit(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createCAASApplication(c, svc, "some-app", applicationservice.AddUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	s.advanceUnitLife(c, unitUUID, life.Dead)

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	s.expectK8sPodCount(c, unitUUID, 1)

	err := st.DeleteUnit(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)

	// The unit should be gone.
	exists, err := st.UnitExists(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(exists, tc.Equals, false)

	s.expectK8sPodCount(c, unitUUID, 0)

	// The charm isn't removed because the application still references it.
	s.checkCharmsCount(c, 1)
}

func (s *unitSuite) TestGetApplicationNameAndUnitNameByUnitUUID(c *tc.C) {
	svc := s.setupApplicationService(c)
	appUUID := s.createIAASApplication(c, svc, "some-app", applicationservice.AddIAASUnitArg{})

	unitUUIDs := s.getAllUnitUUIDs(c, appUUID)
	c.Assert(len(unitUUIDs), tc.Equals, 1)
	unitUUID := unitUUIDs[0]

	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	appName, unitName, err := st.GetApplicationNameAndUnitNameByUnitUUID(c.Context(), unitUUID.String())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(appName, tc.Equals, "some-app")
	c.Check(unitName, tc.Equals, "some-app/0")
}

func (s *unitSuite) TestGetApplicationNameAndUnitNameByUnitUUIDNotFound(c *tc.C) {
	st := NewState(s.TxnRunnerFactory(), loggertesting.WrapCheckLog(c))

	_, _, err := st.GetApplicationNameAndUnitNameByUnitUUID(c.Context(), "foo")
	c.Assert(err, tc.ErrorIs, applicationerrors.UnitNotFound)
}

func (s *unitSuite) expectK8sPodCount(c *tc.C, unitUUID unit.UUID, expected int) {
	var count int
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM k8s_pod WHERE unit_uuid = ?`, unitUUID.String())
		if err := row.Scan(&count); err != nil {
			return err
		}
		return row.Err()
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(count, tc.Equals, expected)
}
