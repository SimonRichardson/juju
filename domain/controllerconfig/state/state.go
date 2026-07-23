// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/canonical/sqlair"

	"github.com/juju/juju/controller"
	"github.com/juju/juju/core/database"
	"github.com/juju/juju/domain"
	"github.com/juju/juju/internal/errors"
)

// State represents a type for interacting with the underlying state.
type State struct {
	*domain.StateBase
}

// NewState returns a new State for interacting with the underlying state.
func NewState(factory database.TxnRunnerFactory) *State {
	return &State{
		StateBase: domain.NewStateBase(factory),
	}
}

// ControllerConfig returns the current configuration in the database.
func (st *State) ControllerConfig(ctx context.Context) (map[string]string, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return nil, errors.Capture(err)
	}

	stmt, err := st.Prepare("SELECT &KeyValue.* FROM v_controller_config", KeyValue{})
	if err != nil {
		return nil, errors.Capture(err)
	}

	result := make(map[string]string)
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		result = map[string]string{}

		var keyValues []KeyValue
		if err := tx.Query(ctx, stmt).GetAll(&keyValues); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return errors.Capture(err)
		}

		for _, kv := range keyValues {
			result[kv.Key] = kv.Value
		}

		return nil
	})

	return result, err
}

// GetControllerConfigValue returns the value for a single controller config
// key. The boolean is false when the key is not set, so callers can fall back
// to the key's default rather than reading the whole controller config.
func (st *State) GetControllerConfigValue(ctx context.Context, key string) (string, bool, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return "", false, errors.Capture(err)
	}

	stmt, err := st.Prepare(`
SELECT &KeyValue.*
FROM v_controller_config
WHERE key = $KeyValue.key`, KeyValue{})
	if err != nil {
		return "", false, errors.Capture(err)
	}

	var (
		kv    KeyValue
		found bool
	)
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		kv = KeyValue{}
		found = false

		err := tx.Query(ctx, stmt, KeyValue{Key: key}).Get(&kv)
		if errors.Is(err, sqlair.ErrNoRows) {
			return nil
		}
		if err != nil {
			return errors.Capture(err)
		}
		found = true
		return nil
	})
	if err != nil {
		return "", false, errors.Capture(err)
	}
	return kv.Value, found, nil
}

// UpdateControllerConfig allows changing some of the configuration
// for the controller. Changes passed in updateAttrs will be applied
// to the current config, and keys in removeAttrs will be unset (and
// so revert to their defaults). Only a subset of keys can be changed
// after bootstrapping.
// ValidateModification is a function that will be called with the current
// config, and should return an error if the modification is not allowed.
func (st *State) UpdateControllerConfig(
	ctx context.Context,
	updateAttrs map[string]string, removeAttrs []string,
) error {
	db, err := st.DB(ctx)
	if err != nil {
		return errors.Capture(err)
	}

	deleteStmt, err := st.Prepare("DELETE FROM controller_config WHERE key IN ($StringSlice[:])", StringSlice{})
	if err != nil {
		return errors.Capture(err)
	}

	updateStmt, err := st.Prepare(`
INSERT INTO controller_config (key, value)
VALUES ($KeyValue.*)
  ON CONFLICT(key) DO UPDATE SET value=excluded.value`, KeyValue{})
	if err != nil {
		return errors.Capture(err)
	}

	desiredApplicationCount := 0
	if value, ok := updateAttrs[controller.ModelDqliteApplicationCount]; ok {
		desiredApplicationCount, err = strconv.Atoi(value)
		if err != nil {
			return errors.Errorf("parsing model Dqlite application count %q: %w", value, err)
		}
	}
	applicationConfigStmt, err := st.Prepare(`
SELECT COUNT(*) AS &dqliteApplicationConfig.count,
       COALESCE(MAX(da.capacity), 0) AS &dqliteApplicationConfig.capacity
FROM dqlite_application AS da
`, dqliteApplicationConfig{})
	if err != nil {
		return errors.Capture(err)
	}
	insertApplicationStmt, err := st.Prepare(`
INSERT INTO dqlite_application (*) VALUES ($dqliteApplication.*)
`, dqliteApplication{})
	if err != nil {
		return errors.Capture(err)
	}
	var (
		updateKeyValues  []KeyValue
		controllerValues controllerValues
	)
	for k, v := range updateAttrs {
		// Although not strictly necessary here, as it's solved in the service
		// layer, we don't want to allow changing the controller UUID or name
		// from the state layer either.
		switch k {
		case controller.ControllerUUIDKey, controller.CACertKey:
			continue
		case controller.APIPort:
			controllerValues.APIPort = sql.Null[string]{V: v, Valid: true}
		default:
			updateKeyValues = append(updateKeyValues, KeyValue{
				Key:   k,
				Value: v,
			})
		}
	}

	// Remove the attributes that have been extracted to the controller
	// table.
	for i, r := range removeAttrs {
		if r == controller.APIPort {
			removeAttrs = append(removeAttrs[:i], removeAttrs[i+1:]...)

			// Force this back to not valid, just in case it was updated and
			// removed at the same time.
			controllerValues.APIPort = sql.Null[string]{}
			break
		}
	}

	updateControllerStmt, err := st.Prepare(`
UPDATE controller
SET api_port = $controllerValues.api_port
`, controllerValues)
	if err != nil {
		return errors.Capture(err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if desiredApplicationCount > 0 {
			var current dqliteApplicationConfig
			if err := tx.Query(ctx, applicationConfigStmt).Get(&current); err != nil {
				return errors.Errorf("reading model Dqlite application configuration: %w", err)
			}
			if desiredApplicationCount < current.Count {
				return errors.Errorf(
					"cannot reduce model Dqlite application count from %d to %d",
					current.Count, desiredApplicationCount,
				)
			}
			applications := make([]dqliteApplication, 0, desiredApplicationCount-current.Count)
			for id := current.Count + 1; id <= desiredApplicationCount; id++ {
				applications = append(applications, dqliteApplication{
					ID:       id,
					State:    "ready",
					Capacity: current.Capacity,
				})
			}
			if len(applications) > 0 {
				if err := tx.Query(ctx, insertApplicationStmt, applications).Run(); err != nil {
					return errors.Errorf("adding model Dqlite applications: %w", err)
				}
			}
		}

		// Update the attributes.
		if len(updateKeyValues) > 0 {
			if err := tx.Query(ctx, updateStmt, updateKeyValues).Run(); err != nil {
				return errors.Capture(err)
			}
		}

		// The controller table needs to be updated with the new API port.
		if err := tx.Query(ctx, updateControllerStmt, controllerValues).Run(); err != nil {
			return errors.Capture(err)
		}

		// Remove the attributes
		if len(removeAttrs) > 0 {
			if err := tx.Query(ctx, deleteStmt, StringSlice(removeAttrs)).Run(); err != nil {
				return errors.Capture(err)
			}
		}

		return nil
	})

	return errors.Capture(err)
}

// AllKeysQuery returns a SQL statement that will return
// all known controller configuration keys.
func (*State) AllKeysQuery() string {
	return "SELECT key FROM v_controller_config"
}

// NamespacesForWatchControllerConfig returns the namespace identifiers
// used for watching controller configuration changes.
func (*State) NamespacesForWatchControllerConfig() []string {
	return []string{"controller", "controller_config"}
}
