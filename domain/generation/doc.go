// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// Package generation provides the domain for model generations (branches).
//
// A generation is a named in-flight branch that records application-scoped
// changes (charm, config and resources) which are applied selectively to
// units tracking the branch, and rolled out to all units when the branch is
// committed.
//
// This package owns the branch lifecycle: adding, tracking, committing and
// aborting branches, and the committed history. The application-scoped
// deltas themselves (charm, config and resources) are owned by the
// application domain.
package generation
