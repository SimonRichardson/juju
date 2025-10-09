// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package offer

import (
	coreerrors "github.com/juju/juju/core/errors"
	"github.com/juju/juju/internal/errors"
	"github.com/juju/juju/internal/uuid"
)

// UUID represents a offer unique identifier.
type UUID string

// NewUUID is a convenience function for generating a new offer uuid.
func NewUUID() (UUID, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return UUID(""), err
	}
	return UUID(id.String()), nil
}

// ParseUUID returns a new UUID from the given string. If the string is not a
// valid uuid an error satisfying [errors.NotValid] will be returned.
func ParseUUID(value string) (UUID, error) {
	if !uuid.IsValidUUIDString(value) {
		return "", errors.Errorf("id %q %w", value, coreerrors.NotValid)
	}
	return UUID(value), nil
}

// String implements the stringer interface for UUID.
func (u UUID) String() string {
	return string(u)
}

// Validate ensures the consistency of the UUID. If the uuid is invalid an error
// satisfying [errors.NotValid] will be returned.
func (u UUID) Validate() error {
	if u == "" {
		return errors.Errorf("uuid cannot be empty").Add(coreerrors.NotValid)
	}
	if !uuid.IsValidUUIDString(string(u)) {
		return errors.Errorf("uuid %q %w", u, coreerrors.NotValid)
	}
	return nil
}
