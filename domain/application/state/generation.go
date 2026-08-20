// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"

	"github.com/canonical/sqlair"

	generationerrors "github.com/juju/juju/domain/generation/errors"
	"github.com/juju/juju/internal/errors"
)

func (st *State) getGenerationForApplication(
	ctx context.Context, tx *sqlair.TX, applicationUUID string,
) (applicationGeneration, bool, error) {
	ident := generationApplicationIdent{ApplicationUUID: applicationUUID}
	stmt, err := st.Prepare(`
SELECT g.uuid AS &applicationGeneration.uuid,
       g.name AS &applicationGeneration.name
FROM generation_application AS ga
JOIN generation AS g ON g.uuid = ga.generation_uuid
WHERE ga.application_uuid = $generationApplicationIdent.application_uuid
AND g.state_id = 0
`, applicationGeneration{}, ident)
	if err != nil {
		return applicationGeneration{}, false, errors.Errorf(
			"preparing application generation query: %w", err,
		)
	}

	var result applicationGeneration
	if err := tx.Query(ctx, stmt, ident).Get(&result); errors.Is(err, sqlair.ErrNoRows) {
		return applicationGeneration{}, false, nil
	} else if err != nil {
		return applicationGeneration{}, false, errors.Errorf(
			"querying application generation: %w", err,
		)
	}
	return result, true, nil
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
