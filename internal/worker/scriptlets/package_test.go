// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package scriptlets

import (
	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/core/logger"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testhelpers"
)

//go:generate go run go.uber.org/mock/mockgen -typed -package scriptlets -destination clock_mock_test.go github.com/juju/clock Clock
//go:generate go run go.uber.org/mock/mockgen -typed -package scriptlets -destination service_mock_test.go github.com/juju/juju/internal/worker/scriptlets ScriptletService
//go:generate go run go.uber.org/mock/mockgen -typed -package scriptlets -destination watcher_mock_test.go github.com/juju/juju/core/watcher StringsWatcher

type baseSuite struct {
	testhelpers.IsolationSuite

	logger logger.Logger

	clock            *MockClock
	scriptletService *MockScriptletService
	watcher          *MockStringsWatcher
}

func (s *baseSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.clock = NewMockClock(ctrl)
	s.scriptletService = NewMockScriptletService(ctrl)
	s.watcher = NewMockStringsWatcher(ctrl)

	s.logger = loggertesting.WrapCheckLog(c)

	c.Cleanup(func() {
		s.clock = nil
		s.scriptletService = nil
		s.watcher = nil
	})

	return ctrl
}
