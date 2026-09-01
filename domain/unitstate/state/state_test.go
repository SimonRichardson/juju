// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	stdtesting "testing"

	"github.com/canonical/sqlair"
	"github.com/juju/tc"

	coreunit "github.com/juju/juju/core/unit"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	"github.com/juju/juju/domain/unitstate"
)

type stateSuite struct {
	baseSuite
}

func TestStateSuite(t *stdtesting.T) {
	tc.Run(t, &stateSuite{})
}

func (s *stateSuite) TestSetUnitState(c *tc.C) {
	agentState := unitstate.UnitState{
		Name:          s.unitName,
		CharmState:    new(map[string]string{"one-key": "one-value"}),
		UniterState:   new("some-uniter-state-yaml"),
		RelationState: new(map[int]string{1: "one-value"}),
		StorageState:  new("some-storage-state-yaml"),
		SecretState:   new("some-secret-state-yaml"),
	}
	s.state.SetUnitState(c.Context(), agentState)

	expectedAgentState := unitstate.RetrievedUnitState{
		CharmState:    *agentState.CharmState,
		UniterState:   *agentState.UniterState,
		RelationState: *agentState.RelationState,
		StorageState:  *agentState.StorageState,
		SecretState:   *agentState.SecretState,
	}

	state, err := s.state.GetUnitState(c.Context(), s.unitName)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(state, tc.DeepEquals, expectedAgentState)
}

func (s *stateSuite) TestSetUnitStateJustUniterState(c *tc.C) {
	agentState := unitstate.UnitState{
		Name:        s.unitName,
		UniterState: new("some-uniter-state-yaml"),
	}
	s.state.SetUnitState(c.Context(), agentState)

	expectedAgentState := unitstate.RetrievedUnitState{
		UniterState: *agentState.UniterState,
	}

	state, err := s.state.GetUnitState(c.Context(), s.unitName)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(state, tc.DeepEquals, expectedAgentState)
}

func (s *stateSuite) TestGetUnitStateUnitNotFound(c *tc.C) {
	_, err := s.state.GetUnitState(c.Context(), "bad-uuid")
	c.Assert(err, tc.ErrorIs, applicationerrors.UnitNotFound)
}

func (s *stateSuite) TestGetUnitSnapshotWatchIdentifiers(c *tc.C) {
	var applicationUUID, charmUUID, netNodeUUID string
	err := s.DB().QueryRowContext(c.Context(), `
SELECT application_uuid, charm_uuid, net_node_uuid
FROM unit
WHERE uuid = ?`, s.unitUUID).Scan(&applicationUUID, &charmUUID, &netNodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	identifiers, err := s.state.GetUnitSnapshotWatchIdentifiers(c.Context(), coreunit.Name(s.unitName))
	c.Assert(err, tc.ErrorIsNil)
	c.Check(identifiers, tc.DeepEquals, unitstate.SnapshotWatchIdentifiers{
		UnitUUID:               s.unitUUID,
		ApplicationUUID:        applicationUUID,
		CharmUUID:              charmUUID,
		NetNodeUUIDs:           []string{netNodeUUID},
		RelationUUIDs:          []string{},
		RelationUnitUUIDs:      []string{},
		RelationEndpointUUIDs:  []string{},
		StorageAttachmentUUIDs: []string{},
	})
}

func (s *stateSuite) TestGetUnitSnapshotWatchIdentifiersUnitNotFound(c *tc.C) {
	_, err := s.state.GetUnitSnapshotWatchIdentifiers(c.Context(), "unknown-unit")
	c.Assert(err, tc.ErrorIs, applicationerrors.UnitNotFound)
}

func (s *stateSuite) TestGetUnitSnapshot(c *tc.C) {
	s.addUnitStateCharm(c, "snapshot-key", "snapshot-value")
	var applicationUUID, charmUUID, charmName string
	err := s.DB().QueryRowContext(c.Context(), `
SELECT u.application_uuid, u.charm_uuid, c.reference_name
FROM unit AS u
JOIN charm AS c ON c.uuid = u.charm_uuid
WHERE u.uuid = ?`, s.unitUUID).Scan(&applicationUUID, &charmUUID, &charmName)
	c.Assert(err, tc.ErrorIsNil)

	s.query(c, `UPDATE unit SET life_id = 1 WHERE uuid = ?`, s.unitUUID)
	s.query(c, `UPDATE application SET charm_modified_version = 7 WHERE uuid = ?`, applicationUUID)
	s.query(c, `UPDATE application_setting SET trust = TRUE WHERE application_uuid = ?`, applicationUUID)
	s.query(c, `INSERT INTO unit_resolved (unit_uuid, mode_id) VALUES (?, 0)`, s.unitUUID)
	s.query(c, `UPDATE unit_workload_version SET version = ? WHERE unit_uuid = ?`, "8.0", s.unitUUID)
	s.query(c, `INSERT INTO storage_pool (uuid, name, type) VALUES (?, ?, ?)`, "pool-uuid", "pool", "loop")
	s.query(c, `
INSERT INTO storage_instance
    (uuid, charm_name, storage_name, storage_kind_id, storage_id, life_id,
     storage_pool_uuid, requested_size_mib)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"storage-instance-uuid", "app", "data", 1, "data/0", 0, "pool-uuid", 1024)
	s.query(c, `
INSERT INTO storage_attachment (uuid, storage_instance_uuid, unit_uuid, life_id)
VALUES (?, ?, ?, ?)`, "storage-attachment-uuid", "storage-instance-uuid", s.unitUUID, 1)

	snapshot, err := s.state.GetUnitSnapshot(c.Context(), coreunit.Name(s.unitName))
	c.Assert(err, tc.ErrorIsNil)
	c.Check(snapshot, tc.DeepEquals, unitstate.UnitSnapshot{
		UnitName:             s.unitName,
		ApplicationName:      "app",
		ApplicationUUID:      applicationUUID,
		UnitUUID:             s.unitUUID,
		CharmUUID:            charmUUID,
		CharmURL:             charmName,
		LifeID:               1,
		ResolvedMode:         "retry-hooks",
		CharmModifiedVersion: 7,
		Trust:                true,
		WorkloadVersion:      "8.0",
		CharmState: map[string]string{
			"snapshot-key": "snapshot-value",
		},
		Storage: []unitstate.StorageSnapshot{{
			ID:     "data/0",
			KindID: 1,
			LifeID: 1,
		}},
	})
}

func (s *stateSuite) TestGetUnitSnapshotUnitNotFound(c *tc.C) {
	_, err := s.state.GetUnitSnapshot(c.Context(), "unknown-unit")
	c.Assert(err, tc.ErrorIs, applicationerrors.UnitNotFound)
}

func (s *stateSuite) TestEnsureUnitStateRecord(c *tc.C) {
	ctx := c.Context()

	err := s.TxnRunner().Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		return s.state.ensureUnitStateRecord(ctx, tx, entityUUID{UUID: s.unitUUID})
	})
	c.Assert(err, tc.ErrorIsNil)

	s.checkUnitUUID(c, s.unitUUID)

	// Running again makes no change.
	err = s.TxnRunner().Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		return s.state.ensureUnitStateRecord(ctx, tx, entityUUID{UUID: s.unitUUID})
	})
	c.Assert(err, tc.ErrorIsNil)

	s.checkUnitUUID(c, s.unitUUID)
}

func (s *stateSuite) TestUpdateUnitStateUniter(c *tc.C) {
	ctx := c.Context()
	expState := "some uniter state YAML"

	err := s.TxnRunner().Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := s.state.ensureUnitStateRecord(ctx, tx, entityUUID{UUID: s.unitUUID}); err != nil {
			return err
		}
		return s.state.updateUnitStateUniter(ctx, tx, entityUUID{UUID: s.unitUUID}, expState)
	})
	c.Assert(err, tc.ErrorIsNil)

	var gotState string
	err = s.TxnRunner().StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		q := "SELECT uniter_state FROM unit_state where unit_uuid = ?"
		return tx.QueryRowContext(ctx, q, s.unitUUID).Scan(&gotState)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(gotState, tc.Equals, expState)
}

func (s *stateSuite) TestUpdateUnitStateStorage(c *tc.C) {
	ctx := c.Context()
	expState := "some storage state YAML"

	err := s.TxnRunner().Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := s.state.ensureUnitStateRecord(ctx, tx, entityUUID{UUID: s.unitUUID}); err != nil {
			return err
		}
		return s.state.updateUnitStateStorage(ctx, tx, entityUUID{UUID: s.unitUUID}, expState)
	})
	c.Assert(err, tc.ErrorIsNil)

	var gotState string
	err = s.TxnRunner().StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		q := "SELECT storage_state FROM unit_state where unit_uuid = ?"
		return tx.QueryRowContext(ctx, q, s.unitUUID).Scan(&gotState)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(gotState, tc.Equals, expState)
}

func (s *stateSuite) TestUpdateUnitStateSecret(c *tc.C) {
	ctx := c.Context()
	expState := "some secret state YAML"

	err := s.TxnRunner().Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := s.state.ensureUnitStateRecord(ctx, tx, entityUUID{UUID: s.unitUUID}); err != nil {
			return err
		}
		return s.state.updateUnitStateSecret(ctx, tx, entityUUID{UUID: s.unitUUID}, expState)
	})
	c.Assert(err, tc.ErrorIsNil)

	var gotState string
	err = s.TxnRunner().StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		q := "SELECT secret_state FROM unit_state where unit_uuid = ?"
		return tx.QueryRowContext(ctx, q, s.unitUUID).Scan(&gotState)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(gotState, tc.Equals, expState)
}

func (s *stateSuite) TestUpdateUnitStateCharm(c *tc.C) {
	ctx := c.Context()

	// Set some initial state. This should be overwritten.
	s.addUnitStateCharm(c, "one-key", "one-val")

	expState := map[string]string{
		"two-key":   "two-val",
		"three-key": "three-val",
	}

	err := s.TxnRunner().Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		return s.state.setUnitStateCharm(ctx, tx, entityUUID{UUID: s.unitUUID}, expState)
	})
	c.Assert(err, tc.ErrorIsNil)

	gotState := make(map[string]string)
	err = s.TxnRunner().StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		gotState = map[string]string{}

		q := "SELECT key, value FROM unit_state_charm WHERE unit_uuid = ?"
		rows, err := tx.QueryContext(ctx, q, s.unitUUID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err != nil {
				return err
			}
			gotState[k] = v
		}
		return rows.Err()
	})
	c.Assert(err, tc.ErrorIsNil)

	c.Check(gotState, tc.DeepEquals, expState)
}

func (s *stateSuite) TestUpdateUnitStateCharmEmptyMap(c *tc.C) {
	ctx := c.Context()

	// Set some initial state. This should be deleted when we set empty state.
	s.addUnitStateCharm(c, "one-key", "one-val")

	err := s.TxnRunner().Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		return s.state.setUnitStateCharm(ctx, tx, entityUUID{UUID: s.unitUUID}, map[string]string{})
	})
	c.Assert(err, tc.ErrorIsNil)

	var rowCount int
	err = s.TxnRunner().StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		rowCount = 0

		q := "SELECT key, value FROM unit_state_charm WHERE unit_uuid = ?"
		rows, err := tx.QueryContext(ctx, q, s.unitUUID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			rowCount++
		}
		return rows.Err()
	})
	c.Assert(err, tc.ErrorIsNil)

	c.Check(rowCount, tc.DeepEquals, 0)
}

func (s *stateSuite) TestUpdateUnitStateRelation(c *tc.C) {
	ctx := c.Context()

	// Set some initial state. This should be overwritten.
	s.addUnitStateCharm(c, 1, "one-val")

	expState := map[int]string{
		2: "two-val",
		3: "three-val",
	}

	err := s.TxnRunner().Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		return s.state.setUnitStateRelation(ctx, tx, entityUUID{UUID: s.unitUUID}, expState)
	})
	c.Assert(err, tc.ErrorIsNil)

	gotState := make(map[int]string)
	err = s.TxnRunner().StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		gotState = map[int]string{}

		q := "SELECT key, value FROM unit_state_relation WHERE unit_uuid = ?"
		rows, err := tx.QueryContext(ctx, q, s.unitUUID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var k int
			var v string
			if err := rows.Scan(&k, &v); err != nil {
				return err
			}
			gotState[k] = v
		}
		return rows.Err()
	})
	c.Assert(err, tc.ErrorIsNil)

	c.Check(gotState, tc.DeepEquals, expState)
}

func (s *stateSuite) TestUpdateUnitStateRelationEmptyMap(c *tc.C) {
	ctx := c.Context()

	// Set some initial state. This should be overwritten.
	s.addUnitStateCharm(c, 1, "one-val")

	err := s.TxnRunner().Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		return s.state.setUnitStateRelation(ctx, tx, entityUUID{UUID: s.unitUUID}, map[int]string{})
	})
	c.Assert(err, tc.ErrorIsNil)

	var rowCount int
	err = s.TxnRunner().StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		rowCount = 0

		q := "SELECT key, value FROM unit_state_relation WHERE unit_uuid = ?"
		rows, err := tx.QueryContext(ctx, q, s.unitUUID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			rowCount++
		}
		return rows.Err()
	})
	c.Assert(err, tc.ErrorIsNil)

	c.Check(rowCount, tc.DeepEquals, 0)
}
