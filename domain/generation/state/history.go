// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/canonical/sqlair"
	"github.com/juju/collections/transform"

	generationerrors "github.com/juju/juju/domain/generation/errors"
	"github.com/juju/juju/domain/generation/internal"
	"github.com/juju/juju/internal/errors"
)

// ListCommits returns the committed generation history, oldest first.
func (st *State) ListCommits(ctx context.Context) ([]internal.Commit, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return nil, errors.Capture(err)
	}

	stmt, err := st.Prepare(`
SELECT &commitRow.*
FROM   generation_commit
ORDER BY generation_id
`, commitRow{})
	if err != nil {
		return nil, errors.Errorf("preparing query: %w", err)
	}

	var rows []commitRow
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := tx.Query(ctx, stmt).GetAll(&rows); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("querying commits: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Capture(err)
	}

	return transform.SliceOrErr(rows, decodeCommitRow)
}

// GetCommit returns the commit identified by generation id.
//
// The following error is returned:
// - [generationerrors.CommitNotFound] if no commit with the given generation
// id exists.
func (st *State) GetCommit(ctx context.Context, generationID uint64) (internal.Commit, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return internal.Commit{}, errors.Capture(err)
	}

	commitStmt, err := st.Prepare(`
SELECT &commitRow.*
FROM   generation_commit
WHERE  generation_id = $commitRow.generation_id
`, commitRow{})
	if err != nil {
		return internal.Commit{}, errors.Errorf("preparing commit query: %w", err)
	}

	configStmt, err := st.Prepare(`
SELECT gc.application_uuid AS &commitConfig.application_uuid,
       a.name AS &commitConfig.application_name,
       gc."key" AS &commitConfig.key,
       gc.type_id AS &commitConfig.type_id,
       gc.value AS &commitConfig.value
FROM   generation_commit_config AS gc
LEFT JOIN application AS a ON a.uuid = gc.application_uuid
WHERE  gc.commit_uuid = $commitRow.uuid
ORDER BY gc.application_uuid, gc."key"
`, commitConfig{}, commitRow{})
	if err != nil {
		return internal.Commit{}, errors.Errorf("preparing commit config query: %w", err)
	}

	var commit internal.Commit
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		row := commitRow{GenerationID: generationID}
		if err := tx.Query(ctx, commitStmt, row).Get(&row); errors.Is(err, sqlair.ErrNoRows) {
			return generationerrors.CommitNotFound
		} else if err != nil {
			return errors.Errorf("querying commit %d: %w", generationID, err)
		}

		decoded, err := decodeCommitRow(row)
		if err != nil {
			return err
		}

		var configs []commitConfig
		if err := tx.Query(ctx, configStmt, row).GetAll(&configs); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("querying commit config: %w", err)
		}

		decoded.Applications = decodeCommitConfigs(configs)
		commit = decoded
		return nil
	})
	if err != nil {
		return internal.Commit{}, errors.Capture(err)
	}
	return commit, nil
}

// decodeCommitRow converts a commit database row into an internal Commit DTO.
func decodeCommitRow(r commitRow) (internal.Commit, error) {
	return internal.Commit{
		UUID:         r.UUID,
		GenerationID: r.GenerationID,
		Name:         r.Name,
		CreatedBy:    r.CreatedBy,
		CommittedBy:  r.CommittedBy,
		CommittedAt:  r.CommittedAt,
	}, nil
}

// decodeCommitConfigs groups commit config rows by application.
func decodeCommitConfigs(rows []commitConfig) []internal.ApplicationConfigChange {
	byApp := make(map[string]*internal.ApplicationConfigChange)
	var order []string

	for _, row := range rows {
		app, ok := byApp[row.ApplicationUUID]
		if !ok {
			name := ""
			if row.ApplicationName.Valid {
				name = row.ApplicationName.String
			}
			app = &internal.ApplicationConfigChange{
				ApplicationUUID: row.ApplicationUUID,
				ApplicationName: name,
			}
			byApp[row.ApplicationUUID] = app
			order = append(order, row.ApplicationUUID)
		}
		app.Config = append(app.Config, internal.ConfigChange{
			Key:   row.Key,
			Value: decodeConfigValue(row.TypeID, row.Value),
		})
	}

	result := make([]internal.ApplicationConfigChange, 0, len(order))
	for _, uuid := range order {
		result = append(result, *byApp[uuid])
	}
	return result
}

// decodeConfigValue decodes a stored config value into its Go representation.
// A NULL value represents an explicit unset (tombstone).
func decodeConfigValue(typeID int, value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	s := value.String
	switch typeID {
	case 1: // int
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	case 2: // float
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
	case 3: // boolean
		if v, err := strconv.ParseBool(s); err == nil {
			return v
		}
	}
	// string (0) and secret (4) are returned as-is.
	return s
}
