// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package holisticuniter

import (
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/internal/observability/probe"
)

type ProbeSuite struct{}

func TestProbeSuite(t *stdtesting.T) {
	tc.Run(t, &ProbeSuite{})
}

func (s *ProbeSuite) TestLiveness(c *tc.C) {
	prober := (&Probe{}).SupportedProbes()[probe.ProbeLiveness]

	alive, err := prober.Probe()
	c.Assert(err, tc.ErrorIsNil)
	c.Check(alive, tc.IsTrue)
}

func (s *ProbeSuite) TestReadiness(c *tc.C) {
	provider := &Probe{}
	prober := provider.SupportedProbes()[probe.ProbeReadiness]

	ready, err := prober.Probe()
	c.Assert(err, tc.ErrorIsNil)
	c.Check(ready, tc.IsFalse)

	provider.SetHasStarted(true)
	ready, err = prober.Probe()
	c.Assert(err, tc.ErrorIsNil)
	c.Check(ready, tc.IsTrue)
}
