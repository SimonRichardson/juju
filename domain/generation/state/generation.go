// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"

	"github.com/canonical/sqlair"
	"github.com/juju/collections/transform"

	"github.com/juju/juju/domain/generation"
	generationerrors "github.com/juju/juju/domain/generation/errors"
	"github.com/juju/juju/domain/generation/internal"
	sequencestate "github.com/juju/juju/domain/sequence/state"
	internaldatabase "github.com/juju/juju/internal/database"
	"github.com/juju/juju/internal/errors"
)

// AddBranch creates a new in-flight branch with the given name and returns its
// generation identifier.
//
// The following error is returned:
// - [generationerrors.BranchAlreadyExists] if a branch with the given name is
// already in flight.
func (st *State) AddBranch(ctx context.Context, genUUID, name, createdBy string) (uint64, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return 0, errors.Capture(err)
	}

	insertStmt, err := st.Prepare(`
INSERT INTO generation (uuid, generation_id, name, state_id, created_by, created_at)
VALUES ($generationRow.uuid, $generationRow.generation_id, $generationRow.name, $generationRow.state_id, $generationRow.created_by, DATETIME('now', 'utc'))
`, generationRow{})
	if err != nil {
		return 0, errors.Errorf("preparing insert statement: %w", err)
	}

	var generationID uint64
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		id, err := sequencestate.NextValue(ctx, st, tx, generation.GenerationSequenceNamespace)
		if err != nil {
			return errors.Errorf("allocating generation id: %w", err)
		}

		row := generationRow{
			UUID:         genUUID,
			GenerationID: id,
			Name:         name,
			StateID:      stateIDInFlight,
			CreatedBy:    createdBy,
		}
		if err := tx.Query(ctx, insertStmt, row).Run(); internaldatabase.IsErrConstraintUnique(err) {
			return generationerrors.BranchAlreadyExists
		} else if err != nil {
			return errors.Errorf("inserting generation: %w", err)
		}
		generationID = id
		return nil
	})
	if err != nil {
		return 0, errors.Capture(err)
	}
	return generationID, nil
}

// GetBranchByName returns the in-flight branch with the given name.
//
// The following error is returned:
// - [generationerrors.BranchNotFound] if no in-flight branch with the given
// name exists.
func (st *State) GetBranchByName(ctx context.Context, name string) (internal.Generation, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return internal.Generation{}, errors.Capture(err)
	}

	nameArg := branchName{Name: name}
	stmt, err := st.Prepare(`
SELECT &generationRow.*
FROM   generation
WHERE  name = $branchName.name
AND    state_id = $generationRow.state_id
`, generationRow{}, nameArg)
	if err != nil {
		return internal.Generation{}, errors.Errorf("preparing query: %w", err)
	}

	row := generationRow{Name: name, StateID: stateIDInFlight}
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := tx.Query(ctx, stmt, row, nameArg).Get(&row); errors.Is(err, sqlair.ErrNoRows) {
			return generationerrors.BranchNotFound
		} else if err != nil {
			return errors.Errorf("querying generation %q: %w", name, err)
		}
		return nil
	})
	if err != nil {
		return internal.Generation{}, errors.Capture(err)
	}
	return decodeGenerationRow(row)
}

// ListBranches returns all in-flight branches.
func (st *State) ListBranches(ctx context.Context) ([]internal.Generation, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return nil, errors.Capture(err)
	}

	stmt, err := st.Prepare(`
SELECT &generationRow.*
FROM   generation
WHERE  state_id = 0
ORDER BY created_at, generation_id
`, generationRow{})
	if err != nil {
		return nil, errors.Errorf("preparing query: %w", err)
	}

	var rows []generationRow
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := tx.Query(ctx, stmt).GetAll(&rows); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("querying generations: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Capture(err)
	}

	return transform.SliceOrErr(rows, decodeGenerationRow)
}

// TrackUnits records that the given units are tracking the branch identified
// by generationUUID.
//
// The following error is returned:
// - [generationerrors.UnitNotFound] if one of the units does not exist.
// - [generationerrors.ApplicationAlreadyOwned] if one of the units belongs to
// an application owned by another branch.
func (st *State) TrackUnits(ctx context.Context, generationUUID string, unitUUIDs []string) error {
	if len(unitUUIDs) == 0 {
		return nil
	}

	db, err := st.DB(ctx)
	if err != nil {
		return errors.Capture(err)
	}

	insertStmt, err := st.Prepare(`
INSERT INTO generation_unit (generation_uuid, unit_uuid)
VALUES ($generationUnit.*)
ON CONFLICT (generation_uuid, unit_uuid) DO NOTHING
`, generationUnit{})
	if err != nil {
		return errors.Errorf("preparing insert statement: %w", err)
	}

	unitSet := make(map[string]struct{}, len(unitUUIDs))
	for _, unitUUID := range unitUUIDs {
		unitSet[unitUUID] = struct{}{}
	}
	units := make(sqlair.S, 0, len(unitSet))
	rows := make([]generationUnit, 0, len(unitSet))
	for unitUUID := range unitSet {
		units = append(units, unitUUID)
		rows = append(rows, generationUnit{
			GenerationUUID: generationUUID,
			UnitUUID:       unitUUID,
		})
	}

	branchStmt, err := st.Prepare(`
SELECT COUNT(*) AS &countRow.n
FROM generation AS g
WHERE g.uuid = $generationIdent.uuid
AND g.state_id = 0
`, countRow{}, generationIdent{})
	if err != nil {
		return errors.Errorf("preparing branch query: %w", err)
	}
	unitApplicationsStmt, err := st.Prepare(`
SELECT u.uuid AS &generationUnitApplication.unit_uuid,
       u.application_uuid AS &generationUnitApplication.application_uuid
FROM unit AS u
WHERE u.uuid IN ($S[:])
ORDER BY u.uuid
`, generationUnitApplication{}, sqlair.S{})
	if err != nil {
		return errors.Errorf("preparing unit application query: %w", err)
	}
	claimStmt, err := st.Prepare(`
INSERT INTO generation_application (generation_uuid, application_uuid)
VALUES ($generationApplication.*)
ON CONFLICT (application_uuid) DO NOTHING
`, generationApplication{})
	if err != nil {
		return errors.Errorf("preparing application ownership claim: %w", err)
	}
	ownerStmt, err := st.Prepare(`
SELECT &generationApplication.*
FROM generation_application
WHERE application_uuid = $generationApplication.application_uuid
`, generationApplication{})
	if err != nil {
		return errors.Errorf("preparing application ownership query: %w", err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		var branchCount countRow
		if err := tx.Query(ctx, branchStmt, generationIdent{UUID: generationUUID}).Get(&branchCount); err != nil {
			return errors.Errorf("querying branch: %w", err)
		}
		if branchCount.N == 0 {
			return generationerrors.BranchNotFound
		}

		var unitApplications []generationUnitApplication
		if err := tx.Query(ctx, unitApplicationsStmt, units).GetAll(&unitApplications); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("querying unit applications: %w", err)
		}
		if len(unitApplications) != len(unitSet) {
			return generationerrors.UnitNotFound
		}

		applicationSet := make(map[string]struct{})
		for _, unitApplication := range unitApplications {
			applicationSet[unitApplication.ApplicationUUID] = struct{}{}
		}
		claims := make([]generationApplication, 0, len(applicationSet))
		for applicationUUID := range applicationSet {
			claims = append(claims, generationApplication{
				GenerationUUID:  generationUUID,
				ApplicationUUID: applicationUUID,
			})
		}
		if err := tx.Query(ctx, claimStmt, claims).Run(); err != nil {
			return errors.Errorf("claiming applications: %w", err)
		}
		for _, claim := range claims {
			var owner generationApplication
			if err := tx.Query(ctx, ownerStmt, claim).Get(&owner); err != nil {
				return errors.Errorf("querying application owner: %w", err)
			}
			if owner.GenerationUUID != generationUUID {
				return generationerrors.ApplicationAlreadyOwned
			}
		}

		if err := tx.Query(ctx, insertStmt, rows).Run(); internaldatabase.IsErrConstraintForeignKey(err) {
			return generationerrors.UnitNotFound
		} else if err != nil {
			return errors.Errorf("tracking units: %w", err)
		}
		return nil
	})
	return errors.Capture(err)
}

// UntrackUnits removes the given units from tracking the branch identified by
// generationUUID.
func (st *State) UntrackUnits(ctx context.Context, generationUUID string, unitUUIDs []string) error {
	if len(unitUUIDs) == 0 {
		return nil
	}

	db, err := st.DB(ctx)
	if err != nil {
		return errors.Capture(err)
	}

	deleteStmt, err := st.Prepare(`
DELETE FROM generation_unit
WHERE  generation_uuid = $generationUnit.generation_uuid
AND    unit_uuid IN ($S[:])
`, generationUnit{}, sqlair.S{})
	if err != nil {
		return errors.Errorf("preparing delete statement: %w", err)
	}

	uuids := make(sqlair.S, len(unitUUIDs))
	for i, u := range unitUUIDs {
		uuids[i] = u
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := tx.Query(ctx, deleteStmt, generationUnit{GenerationUUID: generationUUID}, uuids).Run(); err != nil {
			return errors.Errorf("untracking units: %w", err)
		}
		return nil
	})
	return errors.Capture(err)
}

// GetTrackedUnitNames returns the names of the units tracking the branch
// identified by generationUUID.
func (st *State) GetTrackedUnitNames(ctx context.Context, generationUUID string) ([]string, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return nil, errors.Capture(err)
	}

	stmt, err := st.Prepare(`
SELECT u.name AS &unitName.name
FROM   generation_unit AS gu
JOIN   unit AS u ON u.uuid = gu.unit_uuid
WHERE  gu.generation_uuid = $generationIdent.uuid
ORDER BY u.name
`, unitName{}, generationIdent{})
	if err != nil {
		return nil, errors.Errorf("preparing query: %w", err)
	}

	var names []unitName
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		err := tx.Query(ctx, stmt, generationIdent{UUID: generationUUID}).GetAll(&names)
		if err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("querying tracked units: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Capture(err)
	}

	return transform.Slice(names, func(n unitName) string { return n.Name }), nil
}

// HasTrackedUnits reports whether any units are tracking the branch identified
// by generationUUID.
func (st *State) HasTrackedUnits(ctx context.Context, generationUUID string) (bool, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return false, errors.Capture(err)
	}

	stmt, err := st.Prepare(`
SELECT COUNT(*) AS &countRow.n
FROM   generation_unit
WHERE  generation_uuid = $generationIdent.uuid
`, countRow{}, generationIdent{})
	if err != nil {
		return false, errors.Errorf("preparing query: %w", err)
	}

	var c countRow
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := tx.Query(ctx, stmt, generationIdent{UUID: generationUUID}).Get(&c); err != nil {
			return errors.Errorf("counting tracked units: %w", err)
		}
		return nil
	})
	if err != nil {
		return false, errors.Capture(err)
	}
	return c.N > 0, nil
}

// GetBranchForUnit returns the in-flight branch that the given unit is
// tracking.
//
// The following error is returned:
// - [generationerrors.BranchNotFound] if the unit is not tracking any
// in-flight branch.
func (st *State) GetBranchForUnit(ctx context.Context, unitUUID string) (internal.Generation, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return internal.Generation{}, errors.Capture(err)
	}

	stmt, err := st.Prepare(`
SELECT g.uuid AS &generationRow.uuid,
       g.generation_id AS &generationRow.generation_id,
       g.name AS &generationRow.name,
       g.state_id AS &generationRow.state_id,
       g.created_by AS &generationRow.created_by,
       g.created_at AS &generationRow.created_at,
       g.completed_by AS &generationRow.completed_by,
       g.completed_at AS &generationRow.completed_at
FROM   generation AS g
JOIN   generation_unit AS gu ON gu.generation_uuid = g.uuid
WHERE  gu.unit_uuid = $unitIdent.uuid
AND    g.state_id = 0
`, generationRow{}, unitIdent{})
	if err != nil {
		return internal.Generation{}, errors.Errorf("preparing query: %w", err)
	}

	var row generationRow
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := tx.Query(ctx, stmt, unitIdent{UUID: unitUUID}).Get(&row); errors.Is(err, sqlair.ErrNoRows) {
			return generationerrors.BranchNotFound
		} else if err != nil {
			return errors.Errorf("querying branch for unit %q: %w", unitUUID, err)
		}
		return nil
	})
	if err != nil {
		return internal.Generation{}, errors.Capture(err)
	}
	return decodeGenerationRow(row)
}

// Abort marks the branch identified by generationUUID as aborted and discards
// any changes made under it.
//
// The following errors are returned:
// - [generationerrors.BranchNotFound] if the branch does not exist or is not
// in flight.
func (st *State) Abort(ctx context.Context, generationUUID, abortedBy string) error {
	db, err := st.DB(ctx)
	if err != nil {
		return errors.Capture(err)
	}

	abortStmt, err := st.Prepare(`
UPDATE generation
SET    state_id = $generationRow.state_id,
       completed_by = $generationRow.completed_by,
       completed_at = DATETIME('now', 'utc')
WHERE  uuid = $generationRow.uuid
AND    state_id = 0
`, generationRow{})
	if err != nil {
		return errors.Errorf("preparing abort statement: %w", err)
	}

	clearTrackingStmt, err := st.Prepare(`
DELETE FROM generation_unit
WHERE  generation_uuid = $generationIdent.uuid
`, generationIdent{})
	if err != nil {
		return errors.Errorf("preparing tracking clear: %w", err)
	}
	clearOwnershipStmt, err := st.Prepare(`
DELETE FROM generation_application
WHERE generation_uuid = $generationIdent.uuid
`, generationIdent{})
	if err != nil {
		return errors.Errorf("preparing application ownership clear: %w", err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		row := generationRow{
			UUID:        generationUUID,
			StateID:     stateIDAborted,
			CompletedBy: sql.NullString{String: abortedBy, Valid: true},
		}
		var outcome sqlair.Outcome
		if err := tx.Query(ctx, abortStmt, row).Get(&outcome); err != nil {
			return errors.Errorf("aborting generation: %w", err)
		}
		affected, err := outcome.Result().RowsAffected()
		if err != nil {
			return errors.Errorf("determining abort result: %w", err)
		}
		if affected == 0 {
			return generationerrors.BranchNotFound
		}
		if err := tx.Query(ctx, clearTrackingStmt, generationIdent{UUID: generationUUID}).Run(); err != nil {
			return errors.Errorf("clearing tracked units: %w", err)
		}
		if err := tx.Query(ctx, clearOwnershipStmt, generationIdent{UUID: generationUUID}).Run(); err != nil {
			return errors.Errorf("clearing application ownership: %w", err)
		}
		return nil
	})
	return errors.Capture(err)
}

// decodeGenerationRow converts a database row into an internal Generation DTO.
func decodeGenerationRow(r generationRow) (internal.Generation, error) {
	state, err := decodeState(r.StateID)
	if err != nil {
		return internal.Generation{}, errors.Errorf("decoding generation %q: %w", r.UUID, err)
	}

	g := internal.Generation{
		UUID:         r.UUID,
		GenerationID: r.GenerationID,
		Name:         r.Name,
		State:        state,
		CreatedBy:    r.CreatedBy,
		CreatedAt:    r.CreatedAt,
	}
	if r.CompletedBy.Valid {
		g.CompletedBy = r.CompletedBy.String
	}
	if r.CompletedAt.Valid {
		g.CompletedAt = r.CompletedAt.Time
	}
	return g, nil
}

// decodeState converts a generation_state id into its string value.
func decodeState(id int) (string, error) {
	switch id {
	case stateIDInFlight:
		return string(generation.StateInFlight), nil
	case stateIDCommitted:
		return string(generation.StateCommitted), nil
	case stateIDAborted:
		return string(generation.StateAborted), nil
	default:
		return "", errors.Errorf("unknown generation state id %d", id)
	}
}
