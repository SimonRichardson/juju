// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package errors

import "github.com/juju/juju/internal/errors"

const (
	// BranchNotFound is returned when an in-flight branch cannot be found.
	BranchNotFound = errors.ConstError("branch not found")

	// BranchAlreadyExists is returned when a branch with the given name is
	// already in flight.
	BranchAlreadyExists = errors.ConstError("branch already exists")

	// BranchInProgress is returned when an operation that requires a branch
	// with no tracked units is attempted while units are still tracking it.
	BranchInProgress = errors.ConstError("branch is in progress")

	// BranchCommitted is returned when an operation is attempted on a branch
	// that has already been committed.
	BranchCommitted = errors.ConstError("branch already committed")

	// BranchAborted is returned when an operation is attempted on a branch
	// that has already been aborted.
	BranchAborted = errors.ConstError("branch already aborted")

	// CommitNotFound is returned when a commit cannot be found by its
	// generation id.
	CommitNotFound = errors.ConstError("commit not found")

	// UnitNotFound is returned when a unit cannot be found.
	UnitNotFound = errors.ConstError("unit not found")
)
