// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"sort"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/core/database"
	schematesting "github.com/juju/juju/domain/schema/testing"
	"github.com/juju/juju/testing"
)

type stateSuite struct {
	schematesting.ModelSuite
}

var _ = gc.Suite(&stateSuite{})

func (s *stateSuite) TestEnsureSchema(c *gc.C) {
	factory, _ := s.OpenDBForNamespace(c, "foo")

	state := NewState(func() (database.TxnRunner, error) {
		return factory, nil
	}, testing.NewCheckLogger(c))

	rows := s.countSchemaContent(c, factory)
	c.Assert(rows, gc.Equals, 0)

	err := state.EnsureSchema(context.Background())
	c.Assert(err, jc.ErrorIsNil)

	// Ensure we've got at least some tables. We don't care about the
	// implementations, just that something has been applied.
	rows = s.countSchemaContent(c, factory)
	c.Assert(rows > 0, jc.IsTrue)
}

func (s *stateSuite) TestEnsureSchemaIsIdempotent(c *gc.C) {
	factory, _ := s.OpenDBForNamespace(c, "foo")

	state := NewState(func() (database.TxnRunner, error) {
		return factory, nil
	}, testing.NewCheckLogger(c))

	err := state.EnsureSchema(context.Background())
	c.Assert(err, jc.ErrorIsNil)

	err = state.EnsureSchema(context.Background())
	c.Assert(err, jc.ErrorIsNil)
}

func (s *stateSuite) TestDestroySchema(c *gc.C) {
	factory, _ := s.OpenDBForNamespace(c, "foo")

	state := NewState(func() (database.TxnRunner, error) {
		return factory, nil
	}, testing.NewCheckLogger(c))

	err := state.EnsureSchema(context.Background())
	c.Assert(err, jc.ErrorIsNil)

	err = state.DestroySchema(context.Background())
	c.Assert(err, jc.ErrorIsNil)

	rows := s.countSchemaContent(c, factory)
	if rows != 0 {
		c.Logf("expected no rows, got %d %v", rows, s.showSchemaContent(c, factory))
	}
	c.Assert(rows, gc.Equals, 0)
}

func (s *stateSuite) TestDestroyThenEnsureSchema(c *gc.C) {
	factory, _ := s.OpenDBForNamespace(c, "foo")

	state := NewState(func() (database.TxnRunner, error) {
		return factory, nil
	}, testing.NewCheckLogger(c))

	err := state.EnsureSchema(context.Background())
	c.Assert(err, jc.ErrorIsNil)

	err = state.DestroySchema(context.Background())
	c.Assert(err, jc.ErrorIsNil)

	err = state.EnsureSchema(context.Background())
	c.Assert(err, jc.ErrorIsNil)

	// Ensure we've got at least some tables. We don't care about the
	// implementations, just that something has been applied.
	rows := s.countSchemaContent(c, factory)
	c.Assert(rows > 0, jc.IsTrue)
}

func (s *stateSuite) countSchemaContent(c *gc.C, runner database.TxnRunner) int {
	stmt := `SELECT COUNT(*) FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`

	var count int
	err := runner.StdTxn(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, stmt)
		return row.Scan(&count)
	})
	c.Assert(err, jc.ErrorIsNil)
	return count
}

func (s *stateSuite) showSchemaContent(c *gc.C, runner database.TxnRunner) []sqliteSchema {
	stmt := `SELECT type, name FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`

	var schemas []sqliteSchema
	err := runner.StdTxn(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, stmt)
		c.Assert(err, jc.ErrorIsNil)
		defer rows.Close()

		for rows.Next() {
			var schema sqliteSchema
			err := rows.Scan(&schema.Type, &schema.Name)
			c.Assert(err, jc.ErrorIsNil)

			schemas = append(schemas, schema)
		}

		return nil

	})
	c.Assert(err, jc.ErrorIsNil)

	sort.Slice(schemas, func(i, j int) bool {
		return schemas[i].Name < schemas[j].Name
	})

	return schemas
}
