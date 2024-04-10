// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"strings"

	"github.com/juju/errors"
	"github.com/juju/juju/core/database"
	"github.com/juju/juju/domain"
	"github.com/juju/juju/domain/schema"
)

// Logger is an interface for logging messages.
type Logger interface {
	Debugf(string, ...any)
	Infof(string, ...any)
}

// State represents a type for interacting with the underlying model migration
// domain.
type State struct {
	*domain.StateBase
	logger Logger
}

// NewState returns a new state instance.
// NewState returns a new State for interacting with the underlying model
// defaults.
func NewState(factory database.TxnRunnerFactory, logger Logger) *State {
	return &State{
		StateBase: domain.NewStateBase(factory),
		logger:    logger,
	}
}

// EnsureSchema ensures that the model schema is created for a given model.
// If the model schema already exists, it is a no-op.
func (s *State) EnsureSchema(ctx context.Context) error {
	db, err := s.DB()
	if err != nil {
		return errors.Annotate(err, "getting database")
	}

	ddl := schema.ModelDDL()
	changeSet, err := ddl.Ensure(ctx, db)
	if err != nil {
		return errors.Annotatef(err, "applying model schema %s")
	}
	s.logger.Infof("applied model schema changes from: %d to: %d for model %s", changeSet.Post, changeSet.Current)
	return nil
}

// DestroySchema destroys the model schema. This includes all data in the
// model. Removing constraints, triggers and tables.
// If the model schema has already been destroyed, it is a no-op.
func (s *State) DestroySchema(ctx context.Context) error {
	db, err := s.DB()
	if err != nil {
		return errors.Annotate(err, "getting database")
	}

	schemaStmt := `SELECT type, name, tbl_name FROM sqlite_master;`

	var (
		indexes  = make(map[string]struct{})
		triggers = make(map[string]struct{})
		views    = make(map[string]struct{})
		tables   = make(map[string]struct{})
	)
	err = db.StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, schemaStmt)
		if err != nil {
			return errors.Trace(err)
		}
		defer rows.Close()

		for rows.Next() {
			var schema sqliteSchema
			err := rows.Scan(&schema.Type, &schema.Name)
			if err != nil {
				return errors.Trace(err)
			}

			name := schema.Name

			// If the name is prefixed with sqlite_, then we should ignore it.
			if strings.HasPrefix(name, "sqlite_") {
				s.logger.Debugf("ignoring sqlite_ prefixed schema object %q", name)
				continue
			}

			switch schema.Type {
			case "index":
				indexes[name] = struct{}{}
			case "trigger":
				triggers[name] = struct{}{}
			case "view":
				views[name] = struct{}{}
			case "table":
				tables[name] = struct{}{}
			}
		}
		if err := rows.Err(); err != nil {
			return errors.Trace(err)
		}

		// Ensure that we remove everything in the correct order.
		// 1. Remove all indexes.
		// 2. Remove all triggers.
		// 3. Drop all views.
		// 3. Drop all tables (ignoring sqlite_master)
		stmts := new(strings.Builder)
		for name := range indexes {
			stmts.WriteString("DROP INDEX IF EXISTS ")
			stmts.WriteString(name)
			stmts.WriteString("; ")
		}
		for name := range triggers {
			stmts.WriteString("DROP TRIGGER IF EXISTS ")
			stmts.WriteString(name)
			stmts.WriteString("; ")
		}
		for name := range views {
			stmts.WriteString("DROP VIEW IF EXISTS ")
			stmts.WriteString(name)
			stmts.WriteString("; ")
		}
		for name := range tables {
			stmts.WriteString("DROP TABLE IF EXISTS ")
			stmts.WriteString(name)
			stmts.WriteString("; ")
		}

		_, err = tx.ExecContext(ctx, stmts.String())
		return errors.Trace(err)
	})

	return errors.Trace(err)
}

type sqliteSchema struct {
	Type string
	Name string
}
