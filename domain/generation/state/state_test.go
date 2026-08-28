// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/juju/tc"

	coredatabase "github.com/juju/juju/core/database"
	schematesting "github.com/juju/juju/domain/schema/testing"
	internaluuid "github.com/juju/juju/internal/uuid"
)

type stateSuite struct {
	schematesting.ModelSuite

	state *State
}

func TestStateSuite(t *testing.T) {
	tc.Run(t, &stateSuite{})
}

func (s *stateSuite) SetUpTest(c *tc.C) {
	s.ModelSuite.SetUpTest(c)
	s.state = NewState(s.TxnRunnerFactory())
}

func (s *stateSuite) newUUID(c *tc.C) string {
	return tc.Must(c, internaluuid.NewUUID).String()
}

// createUnit inserts a minimal charm, application and unit, returning the
// application and unit UUIDs.
func (s *stateSuite) createUnit(c *tc.C, appName, unitName string) (string, string) {
	appUUID := s.newUUID(c)
	unitUUID := s.newUUID(c)
	charmUUID := s.newUUID(c)
	netNodeUUID := s.newUUID(c)

	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO charm (uuid, reference_name, architecture_id, revision)
VALUES (?, ?, 0, 1)`, charmUUID, appName); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO application (uuid, name, life_id, charm_uuid, space_uuid)
VALUES (?, ?, 0, ?, '656b4a82-e28c-53d6-a014-f0dd53417eb6')`, appUUID, appName, charmUUID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO net_node (uuid)
VALUES (?)`, netNodeUUID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO unit (uuid, name, life_id, net_node_uuid, application_uuid, charm_uuid)
VALUES (?, ?, 0, ?, ?, ?)`, unitUUID, unitName, netNodeUUID, appUUID, charmUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	return appUUID, unitUUID
}

func (s *stateSuite) createUnitForApplication(
	c *tc.C, appUUID, unitName string,
) string {
	unitUUID := s.newUUID(c)
	netNodeUUID := s.newUUID(c)
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		var charmUUID string
		if err := tx.QueryRowContext(ctx, `
SELECT charm_uuid FROM application WHERE uuid = ?`, appUUID).Scan(&charmUUID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO net_node (uuid) VALUES (?)`, netNodeUUID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO unit (uuid, name, life_id, net_node_uuid, application_uuid, charm_uuid)
VALUES (?, ?, 0, ?, ?, ?)`, unitUUID, unitName, netNodeUUID, appUUID, charmUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	return unitUUID
}

func (s *stateSuite) TestDatabaseError(c *tc.C) {
	boom := errors.New("boom")
	state := NewState(func(context.Context) (coredatabase.TxnRunner, error) {
		return nil, boom
	})

	_, err := state.AddBranch(c.Context(), "generation", "test", "admin")
	c.Check(err, tc.ErrorIs, boom)
	_, err = state.GetBranchByName(c.Context(), "test")
	c.Check(err, tc.ErrorIs, boom)
	_, err = state.ListBranches(c.Context())
	c.Check(err, tc.ErrorIs, boom)
	c.Check(state.TrackUnits(c.Context(), "generation", []string{"unit"}), tc.ErrorIs, boom)
	c.Check(state.UntrackUnits(c.Context(), "generation", []string{"unit"}), tc.ErrorIs, boom)
	_, err = state.GetTrackedUnitNames(c.Context(), "generation")
	c.Check(err, tc.ErrorIs, boom)
	_, err = state.HasTrackedUnits(c.Context(), "generation")
	c.Check(err, tc.ErrorIs, boom)
	_, err = state.GetBranchForUnit(c.Context(), "unit")
	c.Check(err, tc.ErrorIs, boom)
	c.Check(state.Abort(c.Context(), "generation", "admin"), tc.ErrorIs, boom)
	_, err = state.Commit(c.Context(), "generation", "commit", "admin")
	c.Check(err, tc.ErrorIs, boom)
	_, err = state.ListCommits(c.Context())
	c.Check(err, tc.ErrorIs, boom)
	_, err = state.GetCommit(c.Context(), 0)
	c.Check(err, tc.ErrorIs, boom)
}
