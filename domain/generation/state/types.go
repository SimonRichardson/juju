// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"database/sql"
	"time"
)

// generation_state lookup table ids.
const (
	stateIDInFlight  = 0
	stateIDCommitted = 1
	stateIDAborted   = 2
)

// generationRow is the database representation of a generation (branch) row.
type generationRow struct {
	UUID         string         `db:"uuid"`
	GenerationID uint64         `db:"generation_id"`
	Name         string         `db:"name"`
	StateID      int            `db:"state_id"`
	CreatedBy    string         `db:"created_by"`
	CreatedAt    time.Time      `db:"created_at"`
	CompletedBy  sql.NullString `db:"completed_by"`
	CompletedAt  sql.NullTime   `db:"completed_at"`
}

// branchName is an input used to filter generations by name.
type branchName struct {
	Name string `db:"name"`
}

// generationIdent is an input used to filter by generation uuid.
type generationIdent struct {
	UUID string `db:"uuid"`
}

// unitIdent is an input used to filter by unit uuid.
type unitIdent struct {
	UUID string `db:"uuid"`
}

// generationUnit is the database representation of a generation_unit row.
type generationUnit struct {
	GenerationUUID string `db:"generation_uuid"`
	UnitUUID       string `db:"unit_uuid"`
}

// unitName is used to read a unit's name.
type unitName struct {
	Name string `db:"name"`
}

// applicationIdent is an input used to filter by application uuid.
type applicationIdent struct {
	UUID string `db:"uuid"`
}

// countRow is used to read aggregate counts.
type countRow struct {
	N int `db:"n"`
}

// commitArg binds the arguments shared by the commit statements.
type commitArg struct {
	UUID           string `db:"uuid"`
	GenerationUUID string `db:"generation_uuid"`
	CommittedBy    string `db:"committed_by"`
}

// hashValue is the application_config_hash row.
type hashValue struct {
	ApplicationUUID string `db:"application_uuid"`
	SHA256          string `db:"sha256"`
}

// commitRow is the database representation of a generation_commit row.
type commitRow struct {
	UUID           string    `db:"uuid"`
	GenerationUUID string    `db:"generation_uuid"`
	GenerationID   uint64    `db:"generation_id"`
	Name           string    `db:"name"`
	CreatedBy      string    `db:"created_by"`
	CommittedBy    string    `db:"committed_by"`
	CommittedAt    time.Time `db:"committed_at"`
}

// commitConfig is the database representation of a generation_commit_config
// row, with the application name joined for display.
type commitConfig struct {
	ApplicationUUID string         `db:"application_uuid"`
	ApplicationName sql.NullString `db:"application_name"`
	Key             string         `db:"key"`
	TypeID          int            `db:"type_id"`
	Value           sql.NullString `db:"value"`
}

// configValue is used to read application config key/value pairs for hashing.
type configValue struct {
	Key   string         `db:"key"`
	Value sql.NullString `db:"value"`
}

// trustValue is used to read an application's trust setting for hashing.
type trustValue struct {
	Trust bool `db:"trust"`
}
