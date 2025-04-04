// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charmrevisioner

import (
	"time"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/uuid"
)

type ManifoldConfigSuite struct {
	testing.IsolationSuite
	config ManifoldConfig
}

var _ = gc.Suite(&ManifoldConfigSuite{})

func (s *ManifoldConfigSuite) SetUpTest(c *gc.C) {
	s.IsolationSuite.SetUpTest(c)
	s.config = validConfig(c)
}

func validConfig(c *gc.C) ManifoldConfig {
	return ManifoldConfig{
		DomainServicesName: "domain-services",
		HTTPClientName:     "http-client",
		NewHTTPClient:      NewHTTPClient,
		NewCharmhubClient:  NewCharmhubClient,
		NewWorker:          NewWorker,
		Period:             time.Second,
		ModelTag:           names.NewModelTag(uuid.MustNewUUID().String()),
		Logger:             loggertesting.WrapCheckLog(c),
		Clock:              clock.WallClock,
	}
}

func (s *ManifoldConfigSuite) TestValid(c *gc.C) {
	c.Check(s.config.Validate(), jc.ErrorIsNil)
}

func (s *ManifoldConfigSuite) TestMissingDomainServicesName(c *gc.C) {
	s.config.DomainServicesName = ""
	s.checkNotValid(c, "empty DomainServicesName not valid")
}

func (s *ManifoldConfigSuite) TestMissingHTTPClientName(c *gc.C) {
	s.config.HTTPClientName = ""
	s.checkNotValid(c, "empty HTTPClientName not valid")
}

func (s *ManifoldConfigSuite) TestMissingNewHTTPClient(c *gc.C) {
	s.config.NewHTTPClient = nil
	s.checkNotValid(c, "nil NewHTTPClient not valid")
}

func (s *ManifoldConfigSuite) TestMissingNewCharmhubClient(c *gc.C) {
	s.config.NewCharmhubClient = nil
	s.checkNotValid(c, "nil NewCharmhubClient not valid")
}

func (s *ManifoldConfigSuite) TestMissingNewWorker(c *gc.C) {
	s.config.NewWorker = nil
	s.checkNotValid(c, "nil NewWorker not valid")
}

func (s *ManifoldConfigSuite) TestInvalidPeriod(c *gc.C) {
	s.config.Period = 0
	s.checkNotValid(c, "invalid Period not valid")
}

func (s *ManifoldConfigSuite) TestInvalidModelTag(c *gc.C) {
	s.config.ModelTag = names.ModelTag{}
	s.checkNotValid(c, "invalid ModelTag not valid")
}

func (s *ManifoldConfigSuite) TestMissingLogger(c *gc.C) {
	s.config.Logger = nil
	s.checkNotValid(c, "nil Logger not valid")
}

func (s *ManifoldConfigSuite) TestMissingClock(c *gc.C) {
	s.config.Clock = nil
	s.checkNotValid(c, "nil Clock not valid")
}

func (s *ManifoldConfigSuite) checkNotValid(c *gc.C, expect string) {
	err := s.config.Validate()
	c.Check(err, gc.ErrorMatches, expect)
	c.Check(err, jc.ErrorIs, errors.NotValid)
}
