// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter_test

import (
	"path/filepath"

	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/v4/exec"
	gc "gopkg.in/check.v1"

	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testing"
	"github.com/juju/juju/internal/worker/uniter"
	"github.com/juju/juju/internal/worker/uniter/runcommands"
	"github.com/juju/juju/juju/sockets"
)

type ListenerSuite struct {
	testing.BaseSuite
	socketPath sockets.Socket
}

var _ = gc.Suite(&ListenerSuite{})

func sockPath(c *gc.C) sockets.Socket {
	sockPath := filepath.Join(c.MkDir(), "test.listener")
	return sockets.Socket{Address: sockPath, Network: "unix"}
}

func (s *ListenerSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.socketPath = sockPath(c)
}

// Mirror the params to uniter.NewRunListener, but add cleanup to close it.
func (s *ListenerSuite) NewRunListener(c *gc.C) *uniter.RunListener {
	listener, err := uniter.NewRunListener(s.socketPath, loggertesting.WrapCheckLog(c))
	c.Assert(err, jc.ErrorIsNil)
	listener.RegisterRunner("test/0", &mockCommandRunner{
		c: c,
	})
	s.AddCleanup(func(c *gc.C) {
		c.Assert(listener.Close(), jc.ErrorIsNil)
	})
	return listener
}

func (s *ListenerSuite) TestNewRunListenerOnExistingSocketRemovesItAndSucceeds(c *gc.C) {
	s.NewRunListener(c)
	s.NewRunListener(c)
}

func (s *ListenerSuite) TestClientCall(c *gc.C) {
	s.NewRunListener(c)

	client, err := sockets.Dial(s.socketPath)
	c.Assert(err, jc.ErrorIsNil)
	defer client.Close()

	var result exec.ExecResponse
	args := uniter.RunCommandsArgs{
		Commands:        "some-command",
		RelationId:      -1,
		RemoteUnitName:  "",
		ForceRemoteUnit: false,
		UnitName:        "test/0",
	}
	err = client.Call(uniter.JujuExecEndpoint, args, &result)
	c.Assert(err, jc.ErrorIsNil)

	c.Assert(string(result.Stdout), gc.Equals, "some-command stdout")
	c.Assert(string(result.Stderr), gc.Equals, "some-command stderr")
	c.Assert(result.Code, gc.Equals, 42)
}

func (s *ListenerSuite) TestUnregisterRunner(c *gc.C) {
	listener := s.NewRunListener(c)
	listener.UnregisterRunner("test/0")

	client, err := sockets.Dial(s.socketPath)
	c.Assert(err, jc.ErrorIsNil)
	defer client.Close()

	var result exec.ExecResponse
	args := uniter.RunCommandsArgs{
		Commands:        "some-command",
		RelationId:      -1,
		RemoteUnitName:  "",
		ForceRemoteUnit: false,
		UnitName:        "test/0",
	}
	err = client.Call(uniter.JujuExecEndpoint, args, &result)
	c.Assert(err, gc.ErrorMatches, ".*no runner is registered for unit test/0")
}

type ChannelCommandRunnerSuite struct {
	testing.BaseSuite
	abort          chan struct{}
	commands       runcommands.Commands
	commandChannel chan string
	runner         *uniter.ChannelCommandRunner
}

var _ = gc.Suite(&ChannelCommandRunnerSuite{})

func (s *ChannelCommandRunnerSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.abort = make(chan struct{}, 1)
	s.commands = runcommands.NewCommands()
	s.commandChannel = make(chan string, 1)
	runner, err := uniter.NewChannelCommandRunner(uniter.ChannelCommandRunnerConfig{
		Abort:          s.abort,
		Commands:       s.commands,
		CommandChannel: s.commandChannel,
	})
	c.Assert(err, jc.ErrorIsNil)
	s.runner = runner
}

func (s *ChannelCommandRunnerSuite) TestCommandsAborted(c *gc.C) {
	close(s.abort)
	_, err := s.runner.RunCommands(uniter.RunCommandsArgs{
		Commands: "some-command",
	})
	c.Assert(err, gc.ErrorMatches, "command execution aborted")
}

type mockCommandRunner struct {
	c *gc.C
}

var _ uniter.CommandRunner = (*mockCommandRunner)(nil)

func (r *mockCommandRunner) RunCommands(args uniter.RunCommandsArgs) (results *exec.ExecResponse, err error) {
	r.c.Log("mock runner: " + args.Commands)
	return &exec.ExecResponse{
		Code:   42,
		Stdout: []byte(args.Commands + " stdout"),
		Stderr: []byte(args.Commands + " stderr"),
	}, nil
}
