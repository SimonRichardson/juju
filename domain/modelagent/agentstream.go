// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelagent

import (
	coreagentbinary "github.com/juju/juju/core/agentbinary"
	coreerrors "github.com/juju/juju/core/errors"
	"github.com/juju/juju/internal/errors"
)

// AgentStream represents the agent stream that is currently being used by a
// model agent.
type AgentStream int

const (
	// AgentStreamReleased represents the released stream for agent binaries.
	AgentStreamReleased AgentStream = iota
	// AgentStreamProposed represents the proposed stream for agent binaries.
	AgentStreamProposed
	// AgentStreamTesting represents the testing stream for agent binaries.
	AgentStreamTesting
	// AgentStreamDevel represents the devel stream for agent binaries.
	AgentStreamDevel
)

// AgentStreamFromCoreAgentStream converts a [coreagentbinary.AgentStream] to a
// corresponding [AgentStream]. It returns an error if the value is not
// recognised or supported satisfying [coreerrors.NotValid].
func AgentStreamFromCoreAgentStream(
	agentStream coreagentbinary.AgentStream,
) (AgentStream, error) {
	switch agentStream {
	case coreagentbinary.AgentStreamReleased:
		return AgentStreamReleased, nil
	case coreagentbinary.AgentStreamTesting:
		return AgentStreamTesting, nil
	case coreagentbinary.AgentStreamProposed:
		return AgentStreamProposed, nil
	case coreagentbinary.AgentStreamDevel:
		return AgentStreamDevel, nil
	}

	return AgentStream(-1), errors.Errorf(
		"agent stream %q is not recognised as a valid value", agentStream,
	).Add(coreerrors.NotValid)
}

// IsValid checks if the [AgentStream] is a valid value.
func (s AgentStream) IsValid() bool {
	switch s {
	case AgentStreamReleased, AgentStreamProposed, AgentStreamTesting, AgentStreamDevel:
		return true
	default:
		return false
	}
}

// String returns the primitive string values of [AgentStream].
// It is the client's responsibility to validate that [AgentStream]
// is indeed valid before calling [AgentStream.String].
func (s AgentStream) String() string {
	switch s {
	case AgentStreamReleased:
		return "released"
	case AgentStreamProposed:
		return "proposed"
	case AgentStreamTesting:
		return "testing"
	case AgentStreamDevel:
		return "devel"
	}

	return ""
}
