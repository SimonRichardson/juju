// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package controller

// entityUUID represents a generic uuid column from a given table in the
// model's database.
type entityUUID struct {
	UUID string `db:"uuid"`
}
