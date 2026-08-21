// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"

	"github.com/canonical/sqlair"

	coreapplication "github.com/juju/juju/core/application"
	corecharm "github.com/juju/juju/core/charm"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/application"
	"github.com/juju/juju/domain/application/charm"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	"github.com/juju/juju/internal/errors"
)

// GetCharmConfigForApplicationUpdate returns the config schema for the charm
// receiving application config updates. When a branch is active, this is the
// branch-resolved charm; otherwise it is the main charm.
func (st *State) GetCharmConfigForApplicationUpdate(
	ctx context.Context,
	appUUID coreapplication.UUID,
) (corecharm.ID, charm.Config, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return "", charm.Config{}, errors.Capture(err)
	}

	var charmID string
	var config charm.Config
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := st.checkApplicationNotDead(ctx, tx, appUUID); err != nil {
			return errors.Capture(err)
		}
		active, ok, err := st.getGenerationForApplication(ctx, tx, appUUID.String())
		if err != nil {
			return errors.Capture(err)
		}
		if ok {
			charmID, err = st.getGenerationApplicationCharmUUID(ctx, tx, generationApplicationIdent{
				GenerationUUID:  active.UUID,
				ApplicationUUID: appUUID.String(),
			})
			if err != nil {
				return errors.Capture(err)
			}
		} else {
			charmID, err = st.getGenerationApplicationCharmUUID(ctx, tx, generationApplicationIdent{
				ApplicationUUID: appUUID.String(),
			})
			if err != nil {
				return errors.Capture(err)
			}
		}
		resolvedConfig, err := st.getCharmConfig(ctx, tx, entityUUID{UUID: charmID})
		if err != nil {
			return errors.Capture(err)
		}
		config = resolvedConfig
		return nil
	})
	if err != nil {
		return "", charm.Config{}, errors.Capture(err)
	}
	return corecharm.ID(charmID), config, nil
}

// setGenerationApplicationConfig upserts application config deltas in the
// named in-flight branch without changing canonical config or its hash.
func (st *State) setGenerationApplicationConfig(
	ctx context.Context,
	branchName string,
	appUUID coreapplication.UUID,
	config map[string]application.AddApplicationConfig,
) error {
	if len(config) == 0 {
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

		return st.setGenerationApplicationConfigTx(
			ctx, tx, generationUUID, appUUID, config,
		)
	})
	return errors.Capture(err)
}

func (st *State) setGenerationApplicationConfigTx(
	ctx context.Context,
	tx *sqlair.TX,
	generationUUID string,
	appUUID coreapplication.UUID,
	config map[string]application.AddApplicationConfig,
) error {
	if len(config) == 0 {
		return nil
	}
	stmt, err := st.Prepare(`
INSERT INTO generation_application_config (*)
VALUES ($generationApplicationConfig.*)
ON CONFLICT (generation_uuid, application_uuid, "key") DO UPDATE SET
    type_id = excluded.type_id,
    value = excluded.value
`, generationApplicationConfig{})
	if err != nil {
		return errors.Errorf("preparing generation config upsert: %w", err)
	}
	rows := make([]generationApplicationConfig, 0, len(config))
	for key, value := range config {
		typeID, err := encodeConfigType(value.Type)
		if err != nil {
			return errors.Errorf("encoding config type: %w", err)
		}
		rows = append(rows, generationApplicationConfig{
			GenerationUUID:  generationUUID,
			ApplicationUUID: appUUID.String(),
			Key:             key,
			TypeID:          typeID,
			Value:           value.Value,
		})
	}
	if err := tx.Query(ctx, stmt, rows).Run(); err != nil {
		return errors.Errorf("setting generation config: %w", err)
	}
	return nil
}

// unsetGenerationApplicationConfigKeys records tombstones for known charm
// config keys in the named in-flight branch.
func (st *State) unsetGenerationApplicationConfigKeys(
	ctx context.Context,
	branchName string,
	appUUID coreapplication.UUID,
	keys []string,
) error {
	if len(keys) == 0 {
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
		return st.unsetGenerationApplicationConfigKeysTx(
			ctx, tx, generationUUID, appUUID, keys,
		)
	})
	return errors.Capture(err)
}

func (st *State) unsetGenerationApplicationConfigKeysTx(
	ctx context.Context,
	tx *sqlair.TX,
	generationUUID string,
	appUUID coreapplication.UUID,
	keys []string,
) error {
	if len(keys) == 0 {
		return nil
	}
	keyArgs := make(sqlair.S, len(keys))
	for i, key := range keys {
		keyArgs[i] = key
	}
	configStmt, err := st.Prepare(`
SELECT cc."key" AS &configKeyType.key,
       cc.type_id AS &configKeyType.type_id
FROM charm_config AS cc
WHERE cc.charm_uuid = $charmUUID.charm_uuid
AND cc."key" IN ($S[:])
`, configKeyType{}, charmUUID{}, sqlair.S{})
	if err != nil {
		return errors.Errorf("preparing generation config key query: %w", err)
	}
	upsertStmt, err := st.Prepare(`
INSERT INTO generation_application_config (*)
VALUES ($generationApplicationConfig.*)
ON CONFLICT (generation_uuid, application_uuid, "key") DO UPDATE SET
    type_id = excluded.type_id,
    value = NULL
`, generationApplicationConfig{})
	if err != nil {
		return errors.Errorf("preparing generation config tombstone upsert: %w", err)
	}
	resolvedCharm, err := st.getGenerationApplicationCharmUUID(ctx, tx, generationApplicationIdent{
		GenerationUUID:  generationUUID,
		ApplicationUUID: appUUID.String(),
	})
	if err != nil {
		return errors.Capture(err)
	}
	var knownKeys []configKeyType
	if err := tx.Query(ctx, configStmt, charmUUID{UUID: resolvedCharm}, keyArgs).GetAll(&knownKeys); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
		return errors.Errorf("querying generation config keys: %w", err)
	}
	rows := make([]generationApplicationConfig, 0, len(knownKeys))
	for _, key := range knownKeys {
		rows = append(rows, generationApplicationConfig{
			GenerationUUID:  generationUUID,
			ApplicationUUID: appUUID.String(),
			Key:             key.Key,
			TypeID:          key.TypeID,
			Value:           nil,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	if err := tx.Query(ctx, upsertStmt, rows).Run(); err != nil {
		return errors.Errorf("unsetting generation config: %w", err)
	}
	return nil
}

// GetResolvedUnitApplicationConfigWithDefaults returns a unit's effective
// config using its resolved branch charm, branch deltas, main values and charm
// defaults in that order.
func (st *State) GetResolvedUnitApplicationConfigWithDefaults(
	ctx context.Context, unitID coreunit.UUID,
) (map[string]application.ApplicationConfig, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return nil, errors.Capture(err)
	}
	ident := unitUUID{UnitUUID: unitID.String()}
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
resolved_context AS (
    SELECT uc.application_uuid AS application_uuid,
           uc.generation_uuid AS generation_uuid,
           COALESCE(gac.charm_uuid, a.charm_uuid) AS charm_uuid
    FROM   unit_context AS uc
    JOIN   application AS a ON a.uuid = uc.application_uuid
    LEFT JOIN generation_application_charm AS gac
           ON gac.generation_uuid = uc.generation_uuid
           AND gac.application_uuid = uc.application_uuid
)
SELECT cc."key" AS &applicationConfig.key,
       CASE
           WHEN bac."key" IS NOT NULL AND bac.value IS NULL THEN cc.default_value
           WHEN bac."key" IS NOT NULL THEN bac.value
           WHEN ac."key" IS NOT NULL THEN ac.value
           ELSE cc.default_value
       END AS &applicationConfig.value,
       cct.name AS &applicationConfig.type
FROM   resolved_context AS rc
JOIN   charm_config AS cc ON cc.charm_uuid = rc.charm_uuid
JOIN   charm_config_type AS cct ON cct.id = cc.type_id
LEFT JOIN application_config AS ac
       ON ac.application_uuid = rc.application_uuid
       AND ac."key" = cc."key"
       AND ac.type_id = cc.type_id
LEFT JOIN generation_application_config AS bac
       ON bac.generation_uuid = rc.generation_uuid
       AND bac.application_uuid = rc.application_uuid
       AND bac."key" = cc."key"
       AND bac.type_id = cc.type_id
`, applicationConfig{}, ident)
	if err != nil {
		return nil, errors.Errorf("preparing resolved unit config query: %w", err)
	}

	var rows []applicationConfig
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		exists, err := st.checkUnitExists(ctx, tx, unitID.String())
		if err != nil {
			return errors.Capture(err)
		}
		if !exists {
			return applicationerrors.UnitNotFound
		}
		if err := tx.Query(ctx, stmt, ident).GetAll(&rows); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("querying resolved unit config: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Capture(err)
	}

	result := make(map[string]application.ApplicationConfig, len(rows))
	for _, row := range rows {
		typeValue, err := decodeConfigType(row.Type)
		if err != nil {
			return nil, errors.Errorf("decoding config type: %w", err)
		}
		value := application.ApplicationConfig{Type: typeValue}
		if row.ConfigValue.Valid {
			value.Value = &row.ConfigValue.V
		}
		result[row.Key()] = value
	}
	return result, nil
}
