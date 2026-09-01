// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package holisticuniter

import (
	"sync/atomic"

	"github.com/juju/juju/internal/observability/probe"
)

// Probe reports the liveness and readiness of a holistic uniter.
type Probe struct {
	started atomic.Bool
}

// SetHasStarted records whether the unit has completed its startup lifecycle.
func (p *Probe) SetHasStarted(started bool) {
	p.started.Store(started)
}

// SupportedProbes implements [probe.ProbeProvider].
func (p *Probe) SupportedProbes() probe.SupportedProbes {
	return probe.SupportedProbes{
		probe.ProbeLiveness: probe.ProberFn(func() (bool, error) {
			return true, nil
		}),
		probe.ProbeReadiness: probe.ProberFn(func() (bool, error) {
			return p.started.Load(), nil
		}),
	}
}
