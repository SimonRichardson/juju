// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package dbrepl

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/juju/ansiterm"

	"github.com/juju/juju/core/database"
)

type tableSnapshot struct {
	columns []string
	rows    map[string][]any
}

type tableChange struct {
	kind string
	row  []any
}

func (w *dbReplWorker) execWatch(ctx context.Context, line *readline.Instance, args []string) {
	if len(args) != 1 || args[0] == "" {
		_, _ = fmt.Fprintln(w.cfg.Stderr, "usage: .watch <table>")
		return
	}

	db := w.currentDB
	table := args[0]
	previous, err := readTableSnapshot(ctx, db, table)
	if err != nil {
		_, _ = fmt.Fprintf(w.cfg.Stderr, "cannot watch %q: %v\n", table, err)
		return
	}

	watchCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-watchCtx.Done():
				return
			case <-db.Dying():
				return
			case <-ticker.C:
				current, err := readTableSnapshot(watchCtx, db, table)
				if err != nil {
					if watchCtx.Err() == nil {
						_, _ = fmt.Fprintf(w.cfg.Stderr, "watch %q: %v\n", table, err)
					}
					return
				}
				changes := diffTableSnapshots(previous, current)
				if len(changes) > 0 {
					w.printTableChanges(current.columns, changes)
				}
				previous = current
			}
		}
	}()

	_, _ = fmt.Fprintf(w.cfg.Stdout, "Watching %s (press Enter or Ctrl-C to stop)\n", table)
	line.SetPrompt("")
	_, _ = line.Readline()
	cancel()
	<-done
}

func readTableSnapshot(ctx context.Context, db database.TxnRunner, table string) (tableSnapshot, error) {
	var result tableSnapshot
	err := db.StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		result = tableSnapshot{rows: make(map[string][]any)}

		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&exists); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("table does not exist")
			}
			return err
		}

		quoted := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
		info, err := tx.QueryContext(ctx, "PRAGMA table_info("+quoted+")")
		if err != nil {
			return err
		}
		defer func() { _ = info.Close() }()
		var primaryKeys []struct {
			order int
			name  string
		}
		for info.Next() {
			var cid, pk int
			var name, columnType string
			var notNull int
			var defaultValue any
			if err := info.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
				return err
			}
			if pk > 0 {
				primaryKeys = append(primaryKeys, struct {
					order int
					name  string
				}{order: pk, name: name})
			}
		}
		if err := info.Err(); err != nil {
			return err
		}

		sort.Slice(primaryKeys, func(i, j int) bool { return primaryKeys[i].order < primaryKeys[j].order })

		query := "SELECT * FROM " + quoted
		if len(primaryKeys) == 0 {
			query = "SELECT _rowid_, * FROM " + quoted
		}
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		columns, err := rows.Columns()
		if err != nil {
			return err
		}
		keyIndexes := make([]int, 0, len(primaryKeys))
		for _, key := range primaryKeys {
			for i, column := range columns {
				if column == key.name {
					keyIndexes = append(keyIndexes, i)
					break
				}
			}
		}
		if len(primaryKeys) == 0 {
			result.columns = columns[1:]
		} else {
			result.columns = columns
		}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				return err
			}
			var keyValues []any
			if len(primaryKeys) == 0 {
				keyValues = values[:1]
				values = values[1:]
			} else {
				for _, i := range keyIndexes {
					keyValues = append(keyValues, values[i])
				}
			}
			key, err := json.Marshal(keyValues)
			if err != nil {
				return err
			}
			result.rows[string(key)] = values
		}
		return rows.Err()
	})
	return result, err
}

func diffTableSnapshots(before, after tableSnapshot) []tableChange {
	var changes []tableChange
	for key, row := range after.rows {
		old, exists := before.rows[key]
		switch {
		case !exists:
			changes = append(changes, tableChange{kind: "create", row: row})
		case !reflect.DeepEqual(old, row):
			changes = append(changes, tableChange{kind: "update", row: row})
		}
	}
	for key, row := range before.rows {
		if _, exists := after.rows[key]; !exists {
			changes = append(changes, tableChange{kind: "delete", row: row})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].kind != changes[j].kind {
			return changes[i].kind < changes[j].kind
		}
		return fmt.Sprint(changes[i].row) < fmt.Sprint(changes[j].row)
	})
	return changes
}

func (w *dbReplWorker) printTableChanges(columns []string, changes []tableChange) {
	var output strings.Builder
	writer := ansiterm.NewTabWriter(&output, 0, 8, 1, '\t', 0)
	_, _ = fmt.Fprint(writer, "change\t")
	for _, column := range columns {
		_, _ = fmt.Fprintf(writer, "%s\t", column)
	}
	_, _ = fmt.Fprintln(writer)
	for _, change := range changes {
		_, _ = fmt.Fprintf(writer, "%s\t", change.kind)
		for _, value := range change.row {
			_, _ = fmt.Fprintf(writer, "%v\t", value)
		}
		_, _ = fmt.Fprintln(writer)
	}
	_ = writer.Flush()
	_, _ = fmt.Fprint(w.cfg.Stdout, output.String())
}
