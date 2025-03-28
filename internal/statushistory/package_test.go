// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package statushistory

import (
	"testing"

	gc "gopkg.in/check.v1"
)

//go:generate go run go.uber.org/mock/mockgen -typed -package statushistory -destination package_mock_test.go github.com/juju/juju/internal/statushistory Recorder
//go:generate go run go.uber.org/mock/mockgen -typed -package statushistory -destination logger_mock_test.go github.com/juju/juju/core/logger Logger

func TestPackage(t *testing.T) {
	gc.TestingT(t)
}
