// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package scriptlets

import (
	stdtesting "testing"
	"time"

	"github.com/juju/tc"
	"github.com/juju/worker/v5/workertest"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/core/watcher/watchertest"
	"github.com/juju/juju/internal/testing"
)

type workerSuite struct {
	baseSuite

	states chan string
}

func TestWorkerSuite(t *stdtesting.T) {
	defer goleak.VerifyNone(t)
	tc.Run(t, &workerSuite{})
}

func (s *workerSuite) TestValidateConfig(c *tc.C) {
	defer s.setupMocks(c).Finish()

	cfg := s.newConfig()

	c.Check(cfg.Validate(), tc.ErrorIsNil)
}

func (s *workerSuite) TestValidateConfigMissingScriptletService(c *tc.C) {
	defer s.setupMocks(c).Finish()

	cfg := s.newConfig()
	cfg.ScriptletService = nil

	c.Check(cfg.Validate(), tc.ErrorMatches, "nil ScriptletService not valid")
}

func (s *workerSuite) TestValidateConfigMissingClock(c *tc.C) {
	defer s.setupMocks(c).Finish()

	cfg := s.newConfig()
	cfg.Clock = nil

	c.Check(cfg.Validate(), tc.ErrorMatches, "nil Clock not valid")
}

func (s *workerSuite) TestValidateConfigMissingLogger(c *tc.C) {
	defer s.setupMocks(c).Finish()

	cfg := s.newConfig()
	cfg.Logger = nil

	c.Check(cfg.Validate(), tc.ErrorMatches, "nil Logger not valid")
}

func (s *workerSuite) TestStartStop(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.expectClock()

	ch := make(chan []string, 1)
	s.scriptletService.EXPECT().WatchScriptlets(gomock.Any()).Return(
		watchertest.NewMockStringsWatcher(ch), nil,
	)

	w := s.newWorker(c)
	s.ensureStartup(c)

	workertest.CleanKill(c, w)
}

func (s *workerSuite) TestHandlesScriptletChange(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.expectClock()

	ch := make(chan []string, 1)
	s.scriptletService.EXPECT().WatchScriptlets(gomock.Any()).Return(
		watchertest.NewMockStringsWatcher(ch), nil,
	)

	script := `x = 1 + 1`
	s.scriptletService.EXPECT().GetScript(gomock.Any(), "test-scriptlet").Return(script, nil)

	w := s.newWorker(c)
	s.ensureStartup(c)

	// Send a change event.
	select {
	case ch <- []string{"test-scriptlet"}:
	case <-c.Context().Done():
		c.Fatal("timed out sending change")
	}

	// Give the worker time to process the change.
	// The worker should start a child scriptlet worker.
	time.Sleep(testing.ShortWait)

	workertest.CleanKill(c, w)
}

func (s *workerSuite) TestHandlesMultipleScriptletChanges(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.expectClock()

	ch := make(chan []string, 1)
	s.scriptletService.EXPECT().WatchScriptlets(gomock.Any()).Return(
		watchertest.NewMockStringsWatcher(ch), nil,
	)

	script := `x = 1 + 1`
	s.scriptletService.EXPECT().GetScript(gomock.Any(), "scriptlet-a").Return(script, nil)
	s.scriptletService.EXPECT().GetScript(gomock.Any(), "scriptlet-b").Return(script, nil)

	w := s.newWorker(c)
	s.ensureStartup(c)

	// Send two scriptlet IDs in a single change.
	select {
	case ch <- []string{"scriptlet-a", "scriptlet-b"}:
	case <-c.Context().Done():
		c.Fatal("timed out sending changes")
	}

	time.Sleep(testing.ShortWait)

	workertest.CleanKill(c, w)
}

func (s *workerSuite) TestWatcherClosedReturnsError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.expectClock()

	ch := make(chan []string, 1)
	s.scriptletService.EXPECT().WatchScriptlets(gomock.Any()).Return(
		watchertest.NewMockStringsWatcher(ch), nil,
	)

	w := s.newWorker(c)
	s.ensureStartup(c)

	// Close the channel to simulate the watcher being stopped.
	close(ch)

	err := workertest.CheckKilled(c, w)
	c.Assert(err, tc.ErrorMatches, "scriptlet watcher closed")
}

func (s *workerSuite) setupMocks(c *tc.C) *gomock.Controller {
	s.states = make(chan string, 1)
	return s.baseSuite.setupMocks(c)
}

func (s *workerSuite) newConfig() WorkerConfig {
	return WorkerConfig{
		ScriptletService: s.scriptletService,
		Clock:            s.clock,
		Logger:           s.logger,
	}
}

func (s *workerSuite) newWorker(c *tc.C) *scriptletsWorker {
	w, err := newWorker(s.newConfig(), s.states)
	c.Assert(err, tc.ErrorIsNil)
	return w
}

func (s *workerSuite) expectClock() {
	s.clock.EXPECT().Now().Return(time.Now()).AnyTimes()
	s.clock.EXPECT().After(gomock.Any()).AnyTimes()
}

func (s *workerSuite) ensureStartup(c *tc.C) {
	select {
	case state := <-s.states:
		c.Assert(state, tc.Equals, stateStarted)
	case <-c.Context().Done():
		c.Fatal("timed out waiting for startup")
	}
}
