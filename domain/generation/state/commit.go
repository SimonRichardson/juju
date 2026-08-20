// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"sort"
	"strconv"

	"github.com/canonical/sqlair"

	generationerrors "github.com/juju/juju/domain/generation/errors"
	"github.com/juju/juju/internal/errors"
)

// Commit applies the branch's changes to the canonical application tables,
// archives the deltas to the commit history, stops all units tracking the
// branch and marks it committed. It returns the branch's generation id.
//
// The following error is returned:
// - [generationerrors.BranchNotFound] if the branch does not exist or is not
// in flight.
func (st *State) Commit(ctx context.Context, generationUUID, commitUUID, committedBy string) (uint64, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return 0, errors.Capture(err)
	}

	args := commitArg{
		UUID:           commitUUID,
		GenerationUUID: generationUUID,
		CommittedBy:    committedBy,
	}

	// Read the in-flight branch, folding its state into the returned row.
	selectBranchStmt, err := st.Prepare(`
SELECT &generationRow.*
FROM   generation
WHERE  uuid = $generationIdent.uuid
AND    state_id = 0
`, generationRow{}, generationIdent{})
	if err != nil {
		return 0, errors.Errorf("preparing branch select: %w", err)
	}

	foldConfigUpsertStmt, err := st.Prepare(`
INSERT INTO application_config (application_uuid, "key", type_id, value)
SELECT gac.application_uuid, gac."key", gac.type_id, gac.value
FROM generation_application_config AS gac
WHERE gac.generation_uuid = $commitArg.generation_uuid
AND gac.value IS NOT NULL
ON CONFLICT (application_uuid, "key") DO UPDATE SET
    type_id = excluded.type_id,
    value = excluded.value
`, commitArg{})
	if err != nil {
		return 0, errors.Errorf("preparing config fold: %w", err)
	}

	foldConfigDeleteStmt, err := st.Prepare(`
DELETE FROM application_config
WHERE (application_uuid, "key") IN (
    SELECT application_uuid, "key"
    FROM generation_application_config
    WHERE generation_uuid = $commitArg.generation_uuid
    AND value IS NULL
)
`, commitArg{})
	if err != nil {
		return 0, errors.Errorf("preparing config tombstone fold: %w", err)
	}

	foldResourceDeleteStmt, err := st.Prepare(`
DELETE FROM application_resource
WHERE resource_uuid IN (
    SELECT ar.resource_uuid
    FROM application_resource AS ar
    JOIN resource AS r ON r.uuid = ar.resource_uuid
    JOIN generation_application_resource AS gar
        ON gar.application_uuid = ar.application_uuid
        AND gar.charm_resource_name = r.charm_resource_name
    WHERE gar.generation_uuid = $commitArg.generation_uuid
)
`, commitArg{})
	if err != nil {
		return 0, errors.Errorf("preparing resource fold delete: %w", err)
	}

	foldResourceInsertStmt, err := st.Prepare(`
INSERT INTO application_resource (resource_uuid, application_uuid)
SELECT gar.resource_uuid, gar.application_uuid
FROM generation_application_resource AS gar
WHERE gar.generation_uuid = $commitArg.generation_uuid
`, commitArg{})
	if err != nil {
		return 0, errors.Errorf("preparing resource fold insert: %w", err)
	}

	insertCommitStmt, err := st.Prepare(`
INSERT INTO generation_commit (uuid, generation_uuid, generation_id, name, created_by, committed_by, committed_at)
SELECT $commitArg.uuid, g.uuid, g.generation_id, g.name, g.created_by, $commitArg.committed_by, DATETIME('now', 'utc')
FROM generation AS g
WHERE g.uuid = $commitArg.generation_uuid
`, commitArg{})
	if err != nil {
		return 0, errors.Errorf("preparing commit insert: %w", err)
	}

	insertCommitConfigStmt, err := st.Prepare(`
INSERT INTO generation_commit_config (commit_uuid, application_uuid, "key", type_id, value)
SELECT $commitArg.uuid, gac.application_uuid, gac."key", gac.type_id, gac.value
FROM generation_application_config AS gac
WHERE gac.generation_uuid = $commitArg.generation_uuid
`, commitArg{})
	if err != nil {
		return 0, errors.Errorf("preparing commit config insert: %w", err)
	}

	clearTrackingStmt, err := st.Prepare(`
DELETE FROM generation_unit
WHERE generation_uuid = $commitArg.generation_uuid
`, commitArg{})
	if err != nil {
		return 0, errors.Errorf("preparing tracking clear: %w", err)
	}

	markCommittedStmt, err := st.Prepare(`
UPDATE generation
SET state_id = $generationRow.state_id,
    completed_by = $generationRow.completed_by,
    completed_at = DATETIME('now', 'utc')
WHERE uuid = $generationRow.uuid
AND state_id = 0
`, generationRow{})
	if err != nil {
		return 0, errors.Errorf("preparing commit mark: %w", err)
	}

	var generationID uint64
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		var branch generationRow
		if err := tx.Query(ctx, selectBranchStmt, generationIdent{UUID: generationUUID}).Get(&branch); errors.Is(err, sqlair.ErrNoRows) {
			return generationerrors.BranchNotFound
		} else if err != nil {
			return errors.Errorf("querying branch: %w", err)
		}

		if err := st.applyGenerationCharms(ctx, tx, generationUUID); err != nil {
			return errors.Errorf("folding charm changes: %w", err)
		}
		if err := tx.Query(ctx, foldConfigUpsertStmt, args).Run(); err != nil {
			return errors.Errorf("folding config changes: %w", err)
		}
		if err := tx.Query(ctx, foldConfigDeleteStmt, args).Run(); err != nil {
			return errors.Errorf("folding config unset: %w", err)
		}
		if err := tx.Query(ctx, foldResourceDeleteStmt, args).Run(); err != nil {
			return errors.Errorf("folding resource changes: %w", err)
		}
		if err := tx.Query(ctx, foldResourceInsertStmt, args).Run(); err != nil {
			return errors.Errorf("folding resource changes: %w", err)
		}

		if err := st.refreshConfigHash(ctx, tx, generationUUID); err != nil {
			return errors.Errorf("refreshing config hash: %w", err)
		}

		if err := tx.Query(ctx, insertCommitStmt, args).Run(); err != nil {
			return errors.Errorf("inserting commit: %w", err)
		}
		if err := tx.Query(ctx, insertCommitConfigStmt, args).Run(); err != nil {
			return errors.Errorf("inserting commit config: %w", err)
		}
		if err := tx.Query(ctx, clearTrackingStmt, args).Run(); err != nil {
			return errors.Errorf("clearing tracked units: %w", err)
		}

		row := generationRow{
			UUID:        generationUUID,
			StateID:     stateIDCommitted,
			CompletedBy: sql.NullString{String: committedBy, Valid: true},
		}
		var outcome sqlair.Outcome
		if err := tx.Query(ctx, markCommittedStmt, row).Get(&outcome); err != nil {
			return errors.Errorf("marking committed: %w", err)
		}
		affected, err := outcome.Result().RowsAffected()
		if err != nil {
			return errors.Errorf("determining commit result: %w", err)
		}
		if affected == 0 {
			return generationerrors.BranchNotFound
		}

		generationID = branch.GenerationID
		return nil
	})
	if err != nil {
		return 0, errors.Capture(err)
	}
	return generationID, nil
}

// refreshConfigHash recomputes the application_config_hash for every
// application that has config deltas under the branch identified by
// generationUUID.
func (st *State) refreshConfigHash(ctx context.Context, tx *sqlair.TX, generationUUID string) error {
	appsStmt, err := st.Prepare(`
SELECT DISTINCT application_uuid AS &applicationIdent.uuid
FROM generation_application_config
WHERE generation_uuid = $generationIdent.uuid
`, applicationIdent{}, generationIdent{})
	if err != nil {
		return errors.Errorf("preparing affected applications query: %w", err)
	}

	var apps []applicationIdent
	if err := tx.Query(ctx, appsStmt, generationIdent{UUID: generationUUID}).GetAll(&apps); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
		return errors.Errorf("querying affected applications: %w", err)
	}

	for _, app := range apps {
		if err := st.refreshApplicationConfigHash(ctx, tx, app.UUID); err != nil {
			return errors.Errorf("refreshing config hash for application %q: %w", app.UUID, err)
		}
	}
	return nil
}

// refreshApplicationConfigHash recomputes and stores the config hash for the
// given application.
func (st *State) refreshApplicationConfigHash(ctx context.Context, tx *sqlair.TX, applicationUUID string) error {
	configStmt, err := st.Prepare(`
SELECT ac."key" AS &configValue.key, ac.value AS &configValue.value
FROM application_config AS ac
WHERE ac.application_uuid = $applicationIdent.uuid
`, configValue{}, applicationIdent{})
	if err != nil {
		return errors.Errorf("preparing config read: %w", err)
	}

	trustStmt, err := st.Prepare(`
SELECT trust AS &trustValue.trust
FROM application_setting
WHERE application_uuid = $applicationIdent.uuid
`, trustValue{}, applicationIdent{})
	if err != nil {
		return errors.Errorf("preparing settings read: %w", err)
	}

	setHashStmt, err := st.Prepare(`
INSERT INTO application_config_hash (application_uuid, sha256)
VALUES ($hashValue.application_uuid, $hashValue.sha256)
ON CONFLICT (application_uuid) DO UPDATE SET sha256 = excluded.sha256
`, hashValue{})
	if err != nil {
		return errors.Errorf("preparing hash upsert: %w", err)
	}

	ident := applicationIdent{UUID: applicationUUID}

	var config []configValue
	if err := tx.Query(ctx, configStmt, ident).GetAll(&config); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
		return errors.Errorf("reading config: %w", err)
	}

	trust := false
	var t trustValue
	if err := tx.Query(ctx, trustStmt, ident).Get(&t); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
		return errors.Errorf("reading settings: %w", err)
	} else if err == nil {
		trust = t.Trust
	}

	hash, err := computeConfigHash(config, trust)
	if err != nil {
		return errors.Errorf("computing config hash: %w", err)
	}

	if err := tx.Query(ctx, setHashStmt, hashValue{
		ApplicationUUID: applicationUUID,
		SHA256:          hash,
	}).Run(); err != nil {
		return errors.Errorf("storing config hash: %w", err)
	}
	return nil
}

// computeConfigHash reproduces the application config hash: a SHA-256 over the
// sorted config key/value pairs followed by the trust setting. This must stay
// consistent with the application domain's hash so that watchers observe a
// stable value.
func computeConfigHash(config []configValue, trust bool) (string, error) {
	sort.Slice(config, func(i, j int) bool {
		return config[i].Key < config[j].Key
	})

	h := sha256.New()
	for _, c := range config {
		if _, err := h.Write([]byte(c.Key)); err != nil {
			return "", errors.Errorf("writing config key: %w", err)
		}
		value := ""
		if c.Value.Valid {
			value = c.Value.String
		}
		if _, err := h.Write([]byte(value)); err != nil {
			return "", errors.Errorf("writing config value: %w", err)
		}
	}
	if _, err := h.Write([]byte(strconv.FormatBool(trust))); err != nil {
		return "", errors.Errorf("writing settings: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
