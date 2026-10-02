// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package dbrepl

import (
	"bytes"
	"context"
	"database/sql"

	"github.com/juju/tc"
)

func (s *dbReplSuite) TestWatchTableChanges(c *tc.C) {
	db := s.TxnRunner()
	err := db.StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `CREATE TABLE "watch table" (id TEXT PRIMARY KEY, value TEXT)`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	before, err := readTableSnapshot(c.Context(), db, "watch table")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(before.columns, tc.DeepEquals, []string{"id", "value"})

	err = db.StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO "watch table" (id, value) VALUES ('a', 'first'), ('b', 'second')`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	afterCreate, err := readTableSnapshot(c.Context(), db, "watch table")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(diffTableSnapshots(before, afterCreate), tc.DeepEquals, []tableChange{
		{kind: "create", row: []any{"a", "first"}},
		{kind: "create", row: []any{"b", "second"}},
	})

	err = db.StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE "watch table" SET value = 'changed' WHERE id = 'a'`)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM "watch table" WHERE id = 'b'`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	afterChange, err := readTableSnapshot(c.Context(), db, "watch table")
	c.Assert(err, tc.ErrorIsNil)
	changes := diffTableSnapshots(afterCreate, afterChange)
	c.Check(changes, tc.DeepEquals, []tableChange{
		{kind: "delete", row: []any{"b", "second"}},
		{kind: "update", row: []any{"a", "changed"}},
	})

	var output bytes.Buffer
	worker := dbReplWorker{cfg: WorkerConfig{Stdout: &output}}
	worker.printTableChanges(afterChange.columns, changes)
	c.Check(output.String(), tc.Contains, "change")
	c.Check(output.String(), tc.Contains, "value")
	c.Check(output.String(), tc.Contains, "changed")
}

func (s *dbReplSuite) TestWatchTableWithoutPrimaryKey(c *tc.C) {
	db := s.TxnRunner()
	err := db.StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `CREATE TABLE watch_unkeyed (value TEXT)`)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO watch_unkeyed (value) VALUES ('first')`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	before, err := readTableSnapshot(c.Context(), db, "watch_unkeyed")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(before.columns, tc.DeepEquals, []string{"value"})
	err = db.StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE watch_unkeyed SET value = 'second'`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	after, err := readTableSnapshot(c.Context(), db, "watch_unkeyed")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(diffTableSnapshots(before, after), tc.DeepEquals, []tableChange{
		{kind: "update", row: []any{"second"}},
	})
}

func (s *dbReplSuite) TestWatchMissingTable(c *tc.C) {
	_, err := readTableSnapshot(c.Context(), s.TxnRunner(), "missing_table")
	c.Check(err, tc.ErrorMatches, "table does not exist")
}
