// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"strings"

	"github.com/juju/errors"
	"github.com/juju/juju/core/database"
	coreschema "github.com/juju/juju/core/database/schema"
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
		return errors.Annotatef(err, "applying model schema")
	}
	s.logger.Infof("applied model schema changes from: %d to: %d", changeSet.Post, changeSet.Current)
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

	operations := schema.ModelDestroyOperations()
	pre := new(strings.Builder)
	post := new(strings.Builder)
	for _, op := range operations {
		switch op.Kind {
		case coreschema.Table:
			pre.WriteString("DELETE FROM ")
			pre.WriteString(op.Name)
			pre.WriteString(";\n")

			post.WriteString("DROP TABLE IF EXISTS ")
			post.WriteString(op.Name)
			post.WriteString(";\n")
		case coreschema.Trigger:
			if (op.TriggerModes & coreschema.Insert) == 1 {
				post.WriteString("DROP TRIGGER IF EXISTS trg_log_")
				post.WriteString(op.Name)
				post.WriteString("_insert")
				post.WriteString(";\n")
			}
			if (op.TriggerModes & coreschema.Update) == 1 {
				post.WriteString("DROP TRIGGER IF EXISTS trg_log_")
				post.WriteString(op.Name)
				post.WriteString("_update")
				post.WriteString(";\n")
			}
			if (op.TriggerModes & coreschema.Delete) == 1 {
				post.WriteString("DROP TRIGGER IF EXISTS trg_log_")
				post.WriteString(op.Name)
				post.WriteString("_delete")
				post.WriteString(";\n")
			}
		case coreschema.View:
			post.WriteString("DROP VIEW IF EXISTS ")
			post.WriteString(op.Name)
			post.WriteString(";\n")
		case coreschema.Index:
			post.WriteString("DROP INDEX IF EXISTS ")
			post.WriteString(op.Name)
			post.WriteString(";\n")
		}
	}

	err = db.StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, pre.String()); err != nil {
			return errors.Annotate(err, "dropping pre model schema")
		}
		if _, err := tx.ExecContext(ctx, post.String()); err != nil {
			return errors.Annotate(err, "dropping post model schema")
		}

		if err := finalizeCleanup(ctx, tx); err != nil {
			return errors.Annotate(err, "finalizing cleanup")
		}
		return nil
	})

	return errors.Trace(err)
}

type sqliteSchema struct {
	Type    string
	Name    string
	TblName string
}

func finalizeCleanup(ctx context.Context, tx *sql.Tx) error {
	var (
		indexes  = make(map[string]struct{})
		triggers = make(map[string]struct{})
		views    = make(map[string]struct{})
		tables   = make(map[string]struct{})
	)

	rows, err := tx.QueryContext(ctx, `SELECT type, name, tbl_name FROM sqlite_master;`)
	if err != nil {
		return errors.Trace(err)
	}
	defer rows.Close()

	for rows.Next() {
		var schema sqliteSchema
		err := rows.Scan(&schema.Type, &schema.Name, &schema.TblName)
		if err != nil {
			return errors.Trace(err)
		}

		name := schema.Name

		// If the name is prefixed with sqlite_, then we should ignore it.
		if strings.HasPrefix(name, "sqlite_") {
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
	// 1. Remove all triggers.
	// 2. Remove all indexes.
	// 3. Drop all views.
	// 3. Drop all tables (ignoring sqlite_master)
	for name := range tables {
		stmts := new(strings.Builder)
		stmts.WriteString("DELETE FROM ")
		stmts.WriteString(name)
		stmts.WriteString(";\n")
		if _, err := tx.ExecContext(ctx, stmts.String()); err != nil {
			return errors.Annotatef(err, "truncate table %q", name)
		}
	}
	for name := range indexes {
		stmts := new(strings.Builder)
		stmts.WriteString("DROP INDEX IF EXISTS ")
		stmts.WriteString(name)
		stmts.WriteString(";\n")
		if _, err := tx.ExecContext(ctx, stmts.String()); err != nil {
			return errors.Annotatef(err, "dropping index %q", name)
		}
	}
	for name := range triggers {
		stmts := new(strings.Builder)
		stmts.WriteString("DROP TRIGGER IF EXISTS ")
		stmts.WriteString(name)
		stmts.WriteString(";\n")
		if _, err := tx.ExecContext(ctx, stmts.String()); err != nil {
			return errors.Annotatef(err, "dropping trigger %q", name)
		}
	}
	for name := range views {
		stmts := new(strings.Builder)
		stmts.WriteString("DROP VIEW IF EXISTS ")
		stmts.WriteString(name)
		stmts.WriteString(";\n")
		if _, err := tx.ExecContext(ctx, stmts.String()); err != nil {
			return errors.Annotatef(err, "dropping view %q", name)
		}
	}
	for name := range tables {
		stmts := new(strings.Builder)
		stmts.WriteString("DROP TABLE IF EXISTS ")
		stmts.WriteString(name)
		stmts.WriteString(";\n")
		if _, err := tx.ExecContext(ctx, stmts.String()); err != nil {
			return errors.Annotatef(err, "dropping table %q", name)
		}
	}

	return errors.Trace(err)
}
