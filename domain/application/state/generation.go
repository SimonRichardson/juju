// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"

	"github.com/canonical/sqlair"

	generationerrors "github.com/juju/juju/domain/generation/errors"
	"github.com/juju/juju/internal/errors"
)

func (st *State) getActiveGeneration(
	ctx context.Context, tx *sqlair.TX,
) (activeGeneration, bool, error) {
	stmt, err := st.Prepare(`
SELECT &activeGeneration.*
FROM   generation
WHERE  state_id = 0
`, activeGeneration{})
	if err != nil {
		return activeGeneration{}, false, errors.Errorf(
			"preparing active generation query: %w", err,
		)
	}

	var result activeGeneration
	if err := tx.Query(ctx, stmt).Get(&result); errors.Is(err, sqlair.ErrNoRows) {
		return activeGeneration{}, false, nil
	} else if err != nil {
		return activeGeneration{}, false, errors.Errorf(
			"querying active generation: %w", err,
		)
	}
	return result, true, nil
}

func (st *State) activeGeneration(ctx context.Context) (activeGeneration, bool, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return activeGeneration{}, false, errors.Capture(err)
	}

	var result activeGeneration
	var ok bool
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		var err error
		result, ok, err = st.getActiveGeneration(ctx, tx)
		return errors.Capture(err)
	})
	return result, ok, errors.Capture(err)
}

// getInFlightGenerationUUID resolves an in-flight branch name to its UUID.
func (st *State) getInFlightGenerationUUID(
	ctx context.Context, tx *sqlair.TX, name generationName,
) (string, error) {
	stmt, err := st.Prepare(`
SELECT g.uuid AS &entityUUID.uuid
FROM   generation AS g
WHERE  g.name = $generationName.name
AND    g.state_id = 0
`, entityUUID{}, name)
	if err != nil {
		return "", errors.Errorf("preparing generation query: %w", err)
	}

	var result entityUUID
	if err := tx.Query(ctx, stmt, name).Get(&result); errors.Is(err, sqlair.ErrNoRows) {
		return "", generationerrors.BranchNotFound
	} else if err != nil {
		return "", errors.Errorf("querying generation: %w", err)
	}
	return result.UUID, nil
}

// getGenerationApplicationCharmUUID returns the branch charm override for an
// application, falling back to its canonical main charm.
func (st *State) getGenerationApplicationCharmUUID(
	ctx context.Context,
	tx *sqlair.TX,
	ident generationApplicationIdent,
) (string, error) {
	stmt, err := st.Prepare(`
SELECT COALESCE(gac.charm_uuid, a.charm_uuid) AS &charmUUID.charm_uuid
FROM   application AS a
LEFT JOIN generation_application_charm AS gac
       ON gac.generation_uuid = $generationApplicationIdent.generation_uuid
       AND gac.application_uuid = a.uuid
WHERE  a.uuid = $generationApplicationIdent.application_uuid
`, charmUUID{}, ident)
	if err != nil {
		return "", errors.Errorf("preparing generation application charm query: %w", err)
	}

	var result charmUUID
	if err := tx.Query(ctx, stmt, ident).Get(&result); errors.Is(err, sqlair.ErrNoRows) {
		return "", errors.Errorf("application not found")
	} else if err != nil {
		return "", errors.Errorf("querying generation application charm: %w", err)
	}
	return result.UUID, nil
}
