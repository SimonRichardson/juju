// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package remoterelationconsumer

import (
	"time"

	"github.com/juju/tc"
	"github.com/juju/worker/v4"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/core/logger"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testhelpers"
)

//go:generate go run go.uber.org/mock/mockgen -typed -package remoterelationconsumer -destination service_mock_test.go -source worker.go
//go:generate go run go.uber.org/mock/mockgen -typed -package remoterelationconsumer -destination worker_mock_test.go github.com/juju/juju/internal/worker/remoterelationconsumer RemoteRelationClientGetter

type baseSuite struct {
	testhelpers.IsolationSuite

	crossModelService          *MockCrossModelService
	remoteModelRelationClient  *MockRemoteModelRelationsClient
	remoteRelationClientGetter *MockRemoteRelationClientGetter

	logger logger.Logger
}

func (s *baseSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.crossModelService = NewMockCrossModelService(ctrl)
	s.remoteModelRelationClient = NewMockRemoteModelRelationsClient(ctrl)
	s.remoteRelationClientGetter = NewMockRemoteRelationClientGetter(ctrl)

	s.logger = loggertesting.WrapCheckLog(c)

	return ctrl
}

type reportableWorker struct {
	worker.Worker
}

func (w reportableWorker) Report() map[string]any {
	return make(map[string]any)
}

func waitForEmptyRunner(c *tc.C, runner *worker.Runner) {
	for {
		select {
		case <-time.After(time.Millisecond * 50):
			if len(runner.WorkerNames()) == 0 {
				return
			}

		case <-c.Context().Done():
			c.Fatalf("timed out waiting for application to be stopped")
		}
	}
}
