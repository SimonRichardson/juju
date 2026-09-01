// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter

import (
	"context"
	"testing"

	"github.com/juju/errors"
	"github.com/juju/tc"
	"github.com/juju/worker/v5"

	"github.com/juju/juju/internal/observability/probe"
	"github.com/juju/juju/internal/worker/uniter/holisticuniter"
)

type ManifoldFactorySuite struct{}

func TestManifoldFactorySuite(t *testing.T) {
	tc.Run(t, &ManifoldFactorySuite{})
}

func (s *ManifoldFactorySuite) TestNewDelta(c *tc.C) {
	wantErr := errors.New("delta constructor called")
	factory := unitWorkerFactory{
		newDelta: func() (worker.Worker, error) {
			return nil, wantErr
		},
	}

	_, err := factory.New(c.Context(), "delta")

	c.Check(err, tc.ErrorIs, wantErr)
}

func (s *ManifoldFactorySuite) TestNewEmptyRuntimeUsesDelta(c *tc.C) {
	wantErr := errors.New("delta constructor called")
	factory := unitWorkerFactory{
		newDelta: func() (worker.Worker, error) {
			return nil, wantErr
		},
	}

	_, err := factory.New(c.Context(), "")

	c.Check(err, tc.ErrorIs, wantErr)
}

func (s *ManifoldFactorySuite) TestNewHolistic(c *tc.C) {
	wantErr := errors.New("holistic constructor called")
	factory := unitWorkerFactory{
		newHolistic: func(_ context.Context) (worker.Worker, error) {
			return nil, wantErr
		},
	}

	_, err := factory.New(c.Context(), "holistic")

	c.Check(err, tc.ErrorIs, wantErr)
}

func (s *ManifoldFactorySuite) TestNewUnknownRuntime(c *tc.C) {
	factory := unitWorkerFactory{}

	_, err := factory.New(c.Context(), "unknown")

	c.Check(err, tc.ErrorIs, errors.NotValid)
	c.Check(err, tc.ErrorMatches, `unknown unit runtime type "unknown" not valid`)
}

func (s *ManifoldFactorySuite) TestNewRequiresRuntimeConstructor(c *tc.C) {
	factory := unitWorkerFactory{}

	_, err := factory.New(c.Context(), "holistic")

	c.Check(err, tc.ErrorIs, errors.NotValid)
	c.Check(err, tc.ErrorMatches, "missing holistic uniter constructor not valid")
}

func (s *ManifoldFactorySuite) TestHolisticOutputProvidesProbes(c *tc.C) {
	worker := &holisticuniter.HolisticUniter{}
	var provider probe.ProbeProvider

	err := output(worker, &provider)

	c.Assert(err, tc.ErrorIsNil)
	c.Check(provider, tc.NotNil)
	probes := provider.SupportedProbes()
	c.Check(probes[probe.ProbeLiveness], tc.NotNil)
	c.Check(probes[probe.ProbeReadiness], tc.NotNil)
}

func (s *ManifoldFactorySuite) TestDeltaOutputProvidesProbes(c *tc.C) {
	worker := &Uniter{}
	var provider probe.ProbeProvider

	err := output(worker, &provider)

	c.Assert(err, tc.ErrorIsNil)
	c.Check(provider, tc.NotNil)
	probes := provider.SupportedProbes()
	c.Check(probes[probe.ProbeLiveness], tc.NotNil)
	c.Check(probes[probe.ProbeReadiness], tc.NotNil)
}
