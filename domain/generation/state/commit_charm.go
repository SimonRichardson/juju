// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"

	"github.com/canonical/sqlair"

	corerelation "github.com/juju/juju/core/relation"
	"github.com/juju/juju/internal/errors"
)

// applyGenerationCharms validates and applies all staged charm overrides.
// Every validation runs before canonical application state is changed.
func (st *State) applyGenerationCharms(
	ctx context.Context, tx *sqlair.TX, generationUUID string,
) error {
	if err := st.validateGenerationCharmRelations(ctx, tx, generationUUID); err != nil {
		return errors.Capture(err)
	}
	if err := st.refreshGenerationApplicationRelationEndpoints(ctx, tx, generationUUID); err != nil {
		return errors.Errorf("refreshing relation endpoints: %w", err)
	}
	if err := st.refreshGenerationApplicationExtraEndpoints(ctx, tx, generationUUID); err != nil {
		return errors.Errorf("refreshing extra endpoints: %w", err)
	}

	ident := generationIdent{UUID: generationUUID}
	stmt, err := st.Prepare(`
UPDATE application
SET charm_uuid = (
    SELECT gac.charm_uuid
    FROM generation_application_charm AS gac
    WHERE gac.generation_uuid = $generationIdent.uuid
    AND gac.application_uuid = application.uuid
)
WHERE uuid IN (
    SELECT gac.application_uuid
    FROM generation_application_charm AS gac
    WHERE gac.generation_uuid = $generationIdent.uuid
)
`, ident)
	if err != nil {
		return errors.Errorf("preparing charm fold: %w", err)
	}
	if err := tx.Query(ctx, stmt, ident).Run(); err != nil {
		return errors.Errorf("applying generation charms: %w", err)
	}
	return nil
}

func (st *State) validateGenerationCharmRelations(
	ctx context.Context, tx *sqlair.TX, generationUUID string,
) error {
	ident := generationIdent{UUID: generationUUID}
	stmt, err := st.Prepare(`
SELECT current.name AS &generationRelationCompatibility.name,
       current.role AS &generationRelationCompatibility.current_role,
       current.interface AS &generationRelationCompatibility.current_interface,
       current.scope AS &generationRelationCompatibility.current_scope,
       COUNT(DISTINCT re.relation_uuid) AS &generationRelationCompatibility.relation_count,
       target.uuid AS &generationRelationCompatibility.target_uuid,
       target.role AS &generationRelationCompatibility.target_role,
       target.interface AS &generationRelationCompatibility.target_interface,
       target.scope AS &generationRelationCompatibility.target_scope,
       target.capacity AS &generationRelationCompatibility.target_capacity
FROM generation_application_charm AS gac
JOIN application_endpoint AS ae ON ae.application_uuid = gac.application_uuid
JOIN v_charm_relation AS current ON current.uuid = ae.charm_relation_uuid
JOIN relation_endpoint AS re ON re.endpoint_uuid = ae.uuid
LEFT JOIN v_charm_relation AS target
       ON target.charm_uuid = gac.charm_uuid
       AND target.name = current.name
WHERE gac.generation_uuid = $generationIdent.uuid
GROUP BY gac.application_uuid,
         current.name,
         current.role,
         current.interface,
         current.scope,
         target.uuid,
         target.role,
         target.interface,
         target.scope,
         target.capacity
ORDER BY gac.application_uuid, current.name
`, generationRelationCompatibility{}, ident)
	if err != nil {
		return errors.Errorf("preparing relation compatibility query: %w", err)
	}

	var relations []generationRelationCompatibility
	if err := tx.Query(ctx, stmt, ident).GetAll(&relations); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
		return errors.Errorf("querying relation compatibility: %w", err)
	}
	for _, relation := range relations {
		if !relation.TargetUUID.Valid {
			if relation.CurrentRole == "peer" {
				return errors.Errorf(
					"charm has no corresponding peer relation %q. Please scale down to 0 units to refresh",
					relation.Name,
				)
			}
			return errors.Errorf("charm has no corresponding relation %q", relation.Name)
		}
		if relation.TargetRole.String != relation.CurrentRole {
			return errors.Errorf(
				"cannot change role of relation %q from %s to %s",
				relation.Name, relation.CurrentRole, relation.TargetRole.String,
			)
		}
		if relation.TargetInterface.String != relation.CurrentInterface.String {
			return errors.Errorf(
				"cannot change interface of relation %q from %s to %s",
				relation.Name, relation.CurrentInterface.String,
				relation.TargetInterface.String,
			)
		}
		if relation.TargetScope.String == "container" && relation.CurrentScope == "global" {
			return errors.Errorf(
				"cannot change scope of relation %q from %s to %s",
				relation.Name, relation.CurrentScope, relation.TargetScope.String,
			)
		}
		if relation.TargetCapacity.Valid && relation.TargetCapacity.V > 0 &&
			relation.RelationCount > relation.TargetCapacity.V {
			return errors.Errorf(
				"new charm version imposes a maximum relation limit of %d for %q which cannot be satisfied by the number of already established relations (%d)",
				relation.TargetCapacity.V, relation.Name, relation.RelationCount,
			)
		}
	}
	return nil
}

func (st *State) refreshGenerationApplicationRelationEndpoints(
	ctx context.Context, tx *sqlair.TX, generationUUID string,
) error {
	ident := generationIdent{UUID: generationUUID}
	mappingsStmt, err := st.Prepare(`
SELECT ae.uuid AS &generationEndpointMapping.endpoint_uuid,
       target.uuid AS &generationEndpointMapping.target_relation_uuid
FROM generation_application_charm AS gac
JOIN application_endpoint AS ae ON ae.application_uuid = gac.application_uuid
JOIN charm_relation AS current ON current.uuid = ae.charm_relation_uuid
LEFT JOIN charm_relation AS target
       ON target.charm_uuid = gac.charm_uuid
       AND target.name = current.name
WHERE gac.generation_uuid = $generationIdent.uuid
ORDER BY ae.uuid
`, generationEndpointMapping{}, ident)
	if err != nil {
		return errors.Errorf("preparing endpoint mapping query: %w", err)
	}
	var mappings []generationEndpointMapping
	if err := tx.Query(ctx, mappingsStmt, ident).GetAll(&mappings); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
		return errors.Errorf("querying endpoint mappings: %w", err)
	}

	updateStmt, err := st.Prepare(`
UPDATE application_endpoint
SET charm_relation_uuid = $generationEndpointMapping.target_relation_uuid
WHERE uuid = $generationEndpointMapping.endpoint_uuid
`, generationEndpointMapping{})
	if err != nil {
		return errors.Errorf("preparing endpoint update: %w", err)
	}
	deleteStmt, err := st.Prepare(`
DELETE FROM application_endpoint
WHERE uuid = $generationEndpointMapping.endpoint_uuid
`, generationEndpointMapping{})
	if err != nil {
		return errors.Errorf("preparing endpoint delete: %w", err)
	}
	for _, mapping := range mappings {
		if mapping.TargetRelationUUID.Valid {
			if err := tx.Query(ctx, updateStmt, mapping).Run(); err != nil {
				return errors.Errorf("updating application endpoint: %w", err)
			}
		} else if err := tx.Query(ctx, deleteStmt, mapping).Run(); err != nil {
			return errors.Errorf("removing application endpoint: %w", err)
		}
	}

	additionalStmt, err := st.Prepare(`
SELECT gac.application_uuid AS &generationAdditionalEndpoint.application_uuid,
       target.uuid AS &generationAdditionalEndpoint.charm_relation_uuid
FROM generation_application_charm AS gac
JOIN charm_relation AS target ON target.charm_uuid = gac.charm_uuid
WHERE gac.generation_uuid = $generationIdent.uuid
AND NOT EXISTS (
    SELECT 1
    FROM application_endpoint AS ae
    JOIN charm_relation AS existing ON existing.uuid = ae.charm_relation_uuid
    WHERE ae.application_uuid = gac.application_uuid
    AND existing.name = target.name
)
ORDER BY gac.application_uuid, target.name
`, generationAdditionalEndpoint{}, ident)
	if err != nil {
		return errors.Errorf("preparing additional endpoint query: %w", err)
	}
	var additional []generationAdditionalEndpoint
	if err := tx.Query(ctx, additionalStmt, ident).GetAll(&additional); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
		return errors.Errorf("querying additional endpoints: %w", err)
	}
	if len(additional) == 0 {
		return nil
	}
	insertStmt, err := st.Prepare(`
INSERT INTO application_endpoint (uuid, application_uuid, charm_relation_uuid)
VALUES ($generationAdditionalEndpoint.*)
`, generationAdditionalEndpoint{})
	if err != nil {
		return errors.Errorf("preparing additional endpoint insert: %w", err)
	}
	for i := range additional {
		uuid, err := corerelation.NewEndpointUUID()
		if err != nil {
			return errors.Capture(err)
		}
		additional[i].UUID = uuid.String()
	}
	if err := tx.Query(ctx, insertStmt, additional).Run(); err != nil {
		return errors.Errorf("inserting additional endpoints: %w", err)
	}
	return nil
}

func (st *State) refreshGenerationApplicationExtraEndpoints(
	ctx context.Context, tx *sqlair.TX, generationUUID string,
) error {
	ident := generationIdent{UUID: generationUUID}
	queryStmt, err := st.Prepare(`
WITH existing_bindings AS (
    SELECT ae.application_uuid AS application_uuid,
           ceb.name AS name,
           ae.space_uuid AS space_uuid
    FROM application_extra_endpoint AS ae
    JOIN charm_extra_binding AS ceb ON ceb.uuid = ae.charm_extra_binding_uuid
)
SELECT gac.application_uuid AS &generationExtraEndpoint.application_uuid,
       target.uuid AS &generationExtraEndpoint.charm_extra_binding_uuid,
       existing.space_uuid AS &generationExtraEndpoint.space_uuid
FROM generation_application_charm AS gac
JOIN charm_extra_binding AS target ON target.charm_uuid = gac.charm_uuid
LEFT JOIN existing_bindings AS existing
       ON existing.application_uuid = gac.application_uuid
       AND existing.name = target.name
WHERE gac.generation_uuid = $generationIdent.uuid
ORDER BY gac.application_uuid, target.name
`, generationExtraEndpoint{}, ident)
	if err != nil {
		return errors.Errorf("preparing extra endpoint query: %w", err)
	}
	var endpoints []generationExtraEndpoint
	if err := tx.Query(ctx, queryStmt, ident).GetAll(&endpoints); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
		return errors.Errorf("querying extra endpoints: %w", err)
	}

	deleteStmt, err := st.Prepare(`
DELETE FROM application_extra_endpoint
WHERE application_uuid IN (
    SELECT gac.application_uuid
    FROM generation_application_charm AS gac
    WHERE gac.generation_uuid = $generationIdent.uuid
)
`, ident)
	if err != nil {
		return errors.Errorf("preparing extra endpoint delete: %w", err)
	}
	if err := tx.Query(ctx, deleteStmt, ident).Run(); err != nil {
		return errors.Errorf("clearing extra endpoints: %w", err)
	}
	if len(endpoints) == 0 {
		return nil
	}
	insertStmt, err := st.Prepare(`
INSERT INTO application_extra_endpoint (
    application_uuid, charm_extra_binding_uuid, space_uuid
)
VALUES ($generationExtraEndpoint.application_uuid,
        $generationExtraEndpoint.charm_extra_binding_uuid,
        $generationExtraEndpoint.space_uuid)
`, generationExtraEndpoint{})
	if err != nil {
		return errors.Errorf("preparing extra endpoint insert: %w", err)
	}
	if err := tx.Query(ctx, insertStmt, endpoints).Run(); err != nil {
		return errors.Errorf("inserting extra endpoints: %w", err)
	}
	return nil
}

type generationRelationCompatibility struct {
	Name             string          `db:"name"`
	CurrentRole      string          `db:"current_role"`
	CurrentInterface sql.NullString  `db:"current_interface"`
	CurrentScope     string          `db:"current_scope"`
	RelationCount    int64           `db:"relation_count"`
	TargetUUID       sql.NullString  `db:"target_uuid"`
	TargetRole       sql.NullString  `db:"target_role"`
	TargetInterface  sql.NullString  `db:"target_interface"`
	TargetScope      sql.NullString  `db:"target_scope"`
	TargetCapacity   sql.Null[int64] `db:"target_capacity"`
}

type generationEndpointMapping struct {
	EndpointUUID       string         `db:"endpoint_uuid"`
	TargetRelationUUID sql.NullString `db:"target_relation_uuid"`
}

type generationAdditionalEndpoint struct {
	UUID              string `db:"uuid"`
	ApplicationUUID   string `db:"application_uuid"`
	CharmRelationUUID string `db:"charm_relation_uuid"`
}

type generationExtraEndpoint struct {
	ApplicationUUID       string         `db:"application_uuid"`
	CharmExtraBindingUUID string         `db:"charm_extra_binding_uuid"`
	SpaceUUID             sql.NullString `db:"space_uuid"`
}
