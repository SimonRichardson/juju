// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"fmt"
	"testing"

	"github.com/juju/clock"
	"github.com/juju/tc"

	"github.com/juju/juju/core/instance"
	coremodel "github.com/juju/juju/core/model"
	modeltesting "github.com/juju/juju/core/model/testing"
	usertesting "github.com/juju/juju/core/user/testing"
	jujuversion "github.com/juju/juju/core/version"
	domainagentbinary "github.com/juju/juju/domain/agentbinary"
	"github.com/juju/juju/domain/deployment"
	domainmachine "github.com/juju/juju/domain/machine"
	machinestate "github.com/juju/juju/domain/machine/state"
	"github.com/juju/juju/domain/model"
	statemodel "github.com/juju/juju/domain/model/state/model"
	schematesting "github.com/juju/juju/domain/schema/testing"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/uuid"
)

type migrationSuite struct {
	schematesting.ModelSuite

	controllerUUID uuid.UUID
}

func TestMigrationSuite(t *testing.T) {
	tc.Run(t, &migrationSuite{})
}

func (s *migrationSuite) SetUpTest(c *tc.C) {
	s.ModelSuite.SetUpTest(c)
	s.controllerUUID = uuid.MustNewUUID()

	runner := s.TxnRunnerFactory()
	state := statemodel.NewState(runner, loggertesting.WrapCheckLog(c))

	id := modeltesting.GenModelUUID(c)
	args := model.ModelDetailArgs{
		UUID:               id,
		AgentStream:        domainagentbinary.AgentStreamReleased,
		AgentVersion:       jujuversion.Current,
		LatestAgentVersion: jujuversion.Current,
		ControllerUUID:     s.controllerUUID,
		Name:               "my-awesome-model",
		Qualifier:          "prod",
		Type:               coremodel.IAAS,
		Cloud:              "aws",
		CloudType:          "ec2",
		CloudRegion:        "myregion",
		CredentialOwner:    usertesting.GenNewName(c, "myowner"),
		CredentialName:     "mycredential",
	}
	err := state.Create(c.Context(), args)
	c.Assert(err, tc.ErrorIsNil)
}

// TestGetControllerUUID is asserting the happy path of getting the controller
// uuid from the database.
func (s *migrationSuite) TestGetControllerUUID(c *tc.C) {
	controllerId, err := New(s.TxnRunnerFactory()).GetControllerUUID(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(controllerId, tc.Equals, s.controllerUUID.String())
}

// TestGetAllInstanceIDs is asserting the happy path of getting all instance
// IDs for the model.
func (s *migrationSuite) TestGetAllInstanceIDs(c *tc.C) {
	// Add two different instances.
	db := s.DB()
	machineState := machinestate.NewState(s.TxnRunnerFactory(), clock.WallClock, loggertesting.WrapCheckLog(c))

	_, machineNames0, err := machineState.AddMachine(c.Context(), domainmachine.AddMachineArgs{
		Platform: deployment.Platform{
			Channel: "24.04",
			OSType:  deployment.Ubuntu,
		},
	})
	c.Assert(err, tc.ErrorIsNil)
	machineUUID0, err := machineState.GetMachineUUID(c.Context(), machineNames0[0])
	c.Assert(err, tc.ErrorIsNil)

	// Add a reference AZ.
	_, err = db.ExecContext(c.Context(), fmt.Sprintf("INSERT INTO availability_zone VALUES(%q, 'az-1')", machineUUID0.String()))
	c.Assert(err, tc.ErrorIsNil)
	arch := "arm64"
	err = machineState.SetMachineCloudInstance(
		c.Context(),
		machineUUID0.String(),
		instance.Id("instance-0"),
		"",
		"nonce",
		&instance.HardwareCharacteristics{
			Arch: &arch,
		},
	)
	c.Assert(err, tc.ErrorIsNil)

	_, machineNames1, err := machineState.AddMachine(c.Context(), domainmachine.AddMachineArgs{
		Platform: deployment.Platform{
			Channel: "24.04",
			OSType:  deployment.Ubuntu,
		},
	})
	c.Assert(err, tc.ErrorIsNil)
	machineUUID1, err := machineState.GetMachineUUID(c.Context(), machineNames1[0])
	c.Assert(err, tc.ErrorIsNil)

	err = machineState.SetMachineCloudInstance(
		c.Context(),
		machineUUID1.String(),
		instance.Id("instance-1"),
		"",
		"nonce",
		&instance.HardwareCharacteristics{
			Arch: &arch,
		},
	)
	c.Assert(err, tc.ErrorIsNil)

	instanceIDs, err := New(s.TxnRunnerFactory()).GetAllInstanceIDs(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(instanceIDs, tc.HasLen, 2)
	c.Check(instanceIDs.Values(), tc.SameContents, []string{"instance-0", "instance-1"})
}

// TestEmptyInstanceIDs tests that no error is returned when there are no
// instances in the model.
func (s *migrationSuite) TestEmptyInstanceIDs(c *tc.C) {
	instanceIDs, err := New(s.TxnRunnerFactory()).GetAllInstanceIDs(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(instanceIDs, tc.HasLen, 0)
}

// TestClearModelImportingStatusSuccess tests that clearing an existing
// model_migrating entry succeeds and actually removes the entry from the
// database.
func (s *migrationSuite) TestClearModelImportingStatusSuccess(c *tc.C) {
	db := s.DB()
	st := New(s.TxnRunnerFactory())

	// Get the model UUID from the database.
	var modelUUID string
	err := db.QueryRowContext(c.Context(), "SELECT uuid FROM model").Scan(&modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Insert a model_migrating entry.
	migratingUUID := uuid.MustNewUUID().String()
	_, err = db.ExecContext(c.Context(),
		"INSERT INTO model_migrating (uuid, model_uuid) VALUES (?, ?)",
		migratingUUID, modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Verify the entry exists.
	var count int
	err = db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM model_migrating WHERE model_uuid = ?",
		modelUUID).Scan(&count)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(count, tc.Equals, 1)

	// Clear the importing status.
	err = st.ClearModelImportingStatus(c.Context(), modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Verify the entry has been deleted.
	err = db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM model_migrating WHERE model_uuid = ?",
		modelUUID).Scan(&count)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(count, tc.Equals, 0)
}

// TestClearModelImportingStatusNoEntry tests that clearing a non-existent
// model_migrating entry succeeds without error (idempotent behavior).
func (s *migrationSuite) TestClearModelImportingStatusNoEntry(c *tc.C) {
	db := s.DB()
	st := New(s.TxnRunnerFactory())

	// Get the model UUID from the database.
	var modelUUID string
	err := db.QueryRowContext(c.Context(), "SELECT uuid FROM model").Scan(&modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Verify no entry exists.
	var count int
	err = db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM model_migrating WHERE model_uuid = ?",
		modelUUID).Scan(&count)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(count, tc.Equals, 0)

	// Clear should succeed even when there's nothing to delete.
	err = st.ClearModelImportingStatus(c.Context(), modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Verify still no entries.
	err = db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM model_migrating WHERE model_uuid = ?",
		modelUUID).Scan(&count)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(count, tc.Equals, 0)
}

// TestClearModelImportingStatusVerifyCorrectEntry tests that clearing
// deletes the correct entry and verifies by UUID.
func (s *migrationSuite) TestClearModelImportingStatusVerifyCorrectEntry(c *tc.C) {
	db := s.DB()
	st := New(s.TxnRunnerFactory())

	// Get the model UUID from the database.
	var modelUUID string
	err := db.QueryRowContext(c.Context(), "SELECT uuid FROM model").Scan(&modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Insert a model_migrating entry with a specific UUID.
	migratingUUID := uuid.MustNewUUID().String()
	_, err = db.ExecContext(c.Context(),
		"INSERT INTO model_migrating (uuid, model_uuid) VALUES (?, ?)",
		migratingUUID, modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Verify we can query the specific entry by its UUID.
	var retrievedModelUUID string
	err = db.QueryRowContext(c.Context(),
		"SELECT model_uuid FROM model_migrating WHERE uuid = ?",
		migratingUUID).Scan(&retrievedModelUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(retrievedModelUUID, tc.Equals, modelUUID)

	// Clear the importing status.
	err = st.ClearModelImportingStatus(c.Context(), modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Verify the entry no longer exists.
	var count int
	err = db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM model_migrating WHERE uuid = ?",
		migratingUUID).Scan(&count)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(count, tc.Equals, 0)
}

// TestClearModelImportingStatusWrongModelUUID tests that clearing with a
// non-existent model UUID succeeds without error and doesn't affect other
// entries.
func (s *migrationSuite) TestClearModelImportingStatusWrongModelUUID(c *tc.C) {
	db := s.DB()
	st := New(s.TxnRunnerFactory())

	// Get the actual model UUID from the database.
	var modelUUID string
	err := db.QueryRowContext(c.Context(), "SELECT uuid FROM model").Scan(&modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Insert a model_migrating entry.
	migratingUUID := uuid.MustNewUUID().String()
	_, err = db.ExecContext(c.Context(),
		"INSERT INTO model_migrating (uuid, model_uuid) VALUES (?, ?)",
		migratingUUID, modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Try to clear with a different (non-existent) model UUID.
	differentModelUUID := uuid.MustNewUUID().String()
	err = st.ClearModelImportingStatus(c.Context(), differentModelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Verify the original entry still exists.
	var count int
	err = db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM model_migrating WHERE model_uuid = ?",
		modelUUID).Scan(&count)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(count, tc.Equals, 1)
}

// TestClearModelImportingStatusIdempotent tests that calling
// ClearModelImportingStatus multiple times is safe and idempotent.
func (s *migrationSuite) TestClearModelImportingStatusIdempotent(c *tc.C) {
	db := s.DB()
	st := New(s.TxnRunnerFactory())

	// Get the model UUID from the database.
	var modelUUID string
	err := db.QueryRowContext(c.Context(), "SELECT uuid FROM model").Scan(&modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Insert a model_migrating entry.
	migratingUUID := uuid.MustNewUUID().String()
	_, err = db.ExecContext(c.Context(),
		"INSERT INTO model_migrating (uuid, model_uuid) VALUES (?, ?)",
		migratingUUID, modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Clear the importing status multiple times.
	err = st.ClearModelImportingStatus(c.Context(), modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	err = st.ClearModelImportingStatus(c.Context(), modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	err = st.ClearModelImportingStatus(c.Context(), modelUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Verify no entries exist.
	var count int
	err = db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM model_migrating WHERE model_uuid = ?",
		modelUUID).Scan(&count)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(count, tc.Equals, 0)
}
