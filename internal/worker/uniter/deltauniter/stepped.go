// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package deltauniter

//go:generate go run github.com/canonical/gomock/mockgen -package deltauniter_test -destination stepped_mock_test.go github.com/juju/juju/internal/worker/uniter/deltauniter Stepped

// Stepped is used by tests only.
type Stepped interface {
	Stepped(s any)
}
