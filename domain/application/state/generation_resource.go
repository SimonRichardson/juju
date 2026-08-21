// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"

	"github.com/canonical/sqlair"

	coreapplication "github.com/juju/juju/core/application"
	coreresource "github.com/juju/juju/core/resource"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/application"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	"github.com/juju/juju/internal/errors"
)

// setGenerationApplicationResources records non-destructive resource
// selections for an application in the named in-flight branch.
func (st *State) setGenerationApplicationResources(
	ctx context.Context,
	branchName string,
	appUUID coreapplication.UUID,
	resources []application.ResourceSelection,
) error {
	if len(resources) == 0 {
		return nil
	}
	db, err := st.DB(ctx)
	if err != nil {
		return errors.Capture(err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := st.checkApplicationNotDead(ctx, tx, appUUID); err != nil {
			return errors.Capture(err)
		}
		generationUUID, err := st.getInFlightGenerationUUID(
			ctx, tx, generationName{Name: branchName},
		)
		if err != nil {
			return errors.Capture(err)
		}
		return st.upsertGenerationApplicationResources(
			ctx, tx, generationUUID, appUUID, resources,
		)
	})
	return errors.Capture(err)
}

func (st *State) upsertGenerationApplicationResources(
	ctx context.Context,
	tx *sqlair.TX,
	generationUUID string,
	appUUID coreapplication.UUID,
	resources []application.ResourceSelection,
) error {
	stmt, err := st.Prepare(`
INSERT INTO generation_application_resource (
    generation_uuid,
    application_uuid,
    charm_resource_name,
    resource_uuid
)
SELECT $generationApplicationResource.generation_uuid,
       $generationApplicationResource.application_uuid,
       $generationApplicationResource.charm_resource_name,
       $generationApplicationResource.resource_uuid
FROM   resource AS r
JOIN   resource_state AS rs ON rs.id = r.state_id
WHERE  r.uuid = $generationApplicationResource.resource_uuid
AND    r.charm_resource_name = $generationApplicationResource.charm_resource_name
AND    rs.name = 'available'
ON CONFLICT (
    generation_uuid,
    application_uuid,
    charm_resource_name
) DO UPDATE SET
    resource_uuid = excluded.resource_uuid
`, generationApplicationResource{})
	if err != nil {
		return errors.Errorf("preparing generation resource upsert: %w", err)
	}

	for _, resource := range resources {
		row := generationApplicationResource{
			GenerationUUID:  generationUUID,
			ApplicationUUID: appUUID.String(),
			ResourceName:    resource.Name,
			ResourceUUID:    resource.ResourceUUID.String(),
		}
		var outcome sqlair.Outcome
		if err := tx.Query(ctx, stmt, row).Get(&outcome); err != nil {
			return errors.Errorf("setting generation resource: %w", err)
		}
		affected, err := outcome.Result().RowsAffected()
		if err != nil {
			return errors.Errorf("determining generation resource result: %w", err)
		}
		if affected == 0 {
			return errors.Errorf(
				"resource %q is not an available resource named %q",
				resource.ResourceUUID, resource.Name,
			).Add(applicationerrors.InvalidResourceArgs)
		}
	}
	return nil
}

// GetResolvedUnitResource returns the selected resource UUID for a unit and
// charm resource name, preferring its in-flight branch override.
func (st *State) GetResolvedUnitResource(
	ctx context.Context, unitID coreunit.UUID, name string,
) (coreresource.UUID, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return "", errors.Capture(err)
	}
	unitIdent := unitUUID{UnitUUID: unitID.String()}
	nameIdent := resourceName{Name: name}
	stmt, err := st.Prepare(`
WITH unit_context AS (
    SELECT u.application_uuid AS application_uuid,
           g.uuid AS generation_uuid
    FROM   unit AS u
    LEFT JOIN generation_unit AS gu ON gu.unit_uuid = u.uuid
    LEFT JOIN generation AS g ON g.uuid = gu.generation_uuid
                              AND g.state_id = 0
    WHERE  u.uuid = $unitUUID.uuid
),
main_resource AS (
    SELECT ar.application_uuid AS application_uuid,
           r.charm_resource_name AS charm_resource_name,
           ar.resource_uuid AS resource_uuid
    FROM   application_resource AS ar
    JOIN   resource AS r ON r.uuid = ar.resource_uuid
    JOIN   resource_state AS rs ON rs.id = r.state_id
    WHERE  r.charm_resource_name = $resourceName.charm_resource_name
    AND    rs.name = 'available'
)
SELECT COALESCE(gar.resource_uuid, mr.resource_uuid) AS &entityUUID.uuid
FROM   unit_context AS uc
LEFT JOIN generation_application_resource AS gar
       ON gar.generation_uuid = uc.generation_uuid
       AND gar.application_uuid = uc.application_uuid
       AND gar.charm_resource_name = $resourceName.charm_resource_name
LEFT JOIN main_resource AS mr ON mr.application_uuid = uc.application_uuid
WHERE  gar.resource_uuid IS NOT NULL OR mr.resource_uuid IS NOT NULL
`, entityUUID{}, unitIdent, nameIdent)
	if err != nil {
		return "", errors.Errorf("preparing resolved unit resource query: %w", err)
	}

	var result entityUUID
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		exists, err := st.checkUnitExists(ctx, tx, unitID.String())
		if err != nil {
			return errors.Capture(err)
		}
		if !exists {
			return applicationerrors.UnitNotFound
		}
		if err := tx.Query(ctx, stmt, unitIdent, nameIdent).Get(&result); errors.Is(err, sqlair.ErrNoRows) {
			return applicationerrors.InvalidResourceArgs
		} else if err != nil {
			return errors.Errorf("querying resolved unit resource: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", errors.Capture(err)
	}
	return coreresource.UUID(result.UUID), nil
}
