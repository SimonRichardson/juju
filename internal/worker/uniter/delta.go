// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter

import "github.com/juju/juju/internal/worker/uniter/deltauniter"

// The delta uniter remains the active implementation while the holistic
// uniter is introduced. These aliases preserve the existing worker API for
// manifolds and consumers during that transition.
type (
	ChannelCommandRunner       = deltauniter.ChannelCommandRunner
	ChannelCommandRunnerConfig = deltauniter.ChannelCommandRunnerConfig
	CommandRunner              = deltauniter.CommandRunner
	JujuExecServer             = deltauniter.JujuExecServer
	NewOperationExecutorFunc   = deltauniter.NewOperationExecutorFunc
	NewPebbleClientFunc        = deltauniter.NewPebbleClientFunc
	NewRunnerExecutorFunc      = deltauniter.NewRunnerExecutorFunc
	PebbleClient               = deltauniter.PebbleClient
	Probe                      = deltauniter.Probe
	RebootQuerier              = deltauniter.RebootQuerier
	ResolverConfig             = deltauniter.ResolverConfig
	RunCommandsArgs            = deltauniter.RunCommandsArgs
	RunListener                = deltauniter.RunListener
	Stepped                    = deltauniter.Stepped
	Uniter                     = deltauniter.Uniter
	UniterExecutionObserver    = deltauniter.UniterExecutionObserver
	UniterParams               = deltauniter.UniterParams
)

const (
	ErrCAASUnitDead  = deltauniter.ErrCAASUnitDead
	JujuExecEndpoint = deltauniter.JujuExecEndpoint
)

var (
	NewChannelCommandRunner = deltauniter.NewChannelCommandRunner
	NewPebbleNoticer        = deltauniter.NewPebbleNoticer
	NewPebblePoller         = deltauniter.NewPebblePoller
	NewRunListener          = deltauniter.NewRunListener
	NewRunListenerWrapper   = deltauniter.NewRunListenerWrapper
	NewUniter               = deltauniter.NewUniter
	NewUniterResolver       = deltauniter.NewUniterResolver
	NewUpdateStatusTimer    = deltauniter.NewUpdateStatusTimer
)
