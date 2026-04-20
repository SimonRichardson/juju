// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package scriptlets

import (
	stdtesting "testing"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/tc"
	"github.com/juju/worker/v5"
	"github.com/juju/worker/v5/dependency"
	"go.uber.org/goleak"

	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testhelpers"
)

type manifoldSuite struct {
	testhelpers.IsolationSuite
}

func TestManifoldSuite(t *stdtesting.T) {
	defer goleak.VerifyNone(t)
	tc.Run(t, &manifoldSuite{})
}

func (s *manifoldSuite) TestValidateConfig(c *tc.C) {
	cfg := s.newManifoldConfig()
	c.Check(cfg.Validate(), tc.ErrorIsNil)
}

func (s *manifoldSuite) TestValidateConfigMissingDomainServicesName(c *tc.C) {
	cfg := s.newManifoldConfig()
	cfg.DomainServicesName = ""
	c.Check(cfg.Validate(), tc.ErrorMatches, "empty DomainServicesName not valid")
}

func (s *manifoldSuite) TestValidateConfigMissingClock(c *tc.C) {
	cfg := s.newManifoldConfig()
	cfg.Clock = nil
	c.Check(cfg.Validate(), tc.ErrorMatches, "nil Clock not valid")
}

func (s *manifoldSuite) TestValidateConfigMissingLogger(c *tc.C) {
	cfg := s.newManifoldConfig()
	cfg.Logger = nil
	c.Check(cfg.Validate(), tc.ErrorMatches, "nil Logger not valid")
}

func (s *manifoldSuite) TestValidateConfigMissingNewWorker(c *tc.C) {
	cfg := s.newManifoldConfig()
	cfg.NewWorker = nil
	c.Check(cfg.Validate(), tc.ErrorMatches, "nil NewWorker not valid")
}

func (s *manifoldSuite) TestValidateConfigMissingGetScriptletService(c *tc.C) {
	cfg := s.newManifoldConfig()
	cfg.GetScriptletService = nil
	c.Check(cfg.Validate(), tc.ErrorMatches, "nil GetScriptletService not valid")
}

func (s *manifoldSuite) TestManifoldInputs(c *tc.C) {
	cfg := s.newManifoldConfig()
	manifold := Manifold(cfg)
	c.Check(manifold.Inputs, tc.DeepEquals, []string{"domain-services"})
}

func (s *manifoldSuite) newManifoldConfig() ManifoldConfig {
	return ManifoldConfig{
		DomainServicesName: "domain-services",
		Clock:              clock.WallClock,
		Logger:             loggertesting.WrapCheckLogWithLevel(nil, 0),
		NewWorker: func(cfg WorkerConfig) (worker.Worker, error) {
			return nil, nil
		},
		GetScriptletService: func(getter dependency.Getter, name string) (ScriptletService, error) {
			return nil, errors.NotFoundf("not found")
		},
	}
}
