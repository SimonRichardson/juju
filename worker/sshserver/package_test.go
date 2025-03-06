// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package sshserver_test

import (
	stdtesting "testing"

	gc "gopkg.in/check.v1"
)

//go:generate go run go.uber.org/mock/mockgen -package sshserver_test -destination facade_client_mock_test.go github.com/juju/juju/worker/sshserver FacadeClient
//go:generate go run go.uber.org/mock/mockgen -package sshserver_test -destination listener_mock_test.go net Listener

func TestPackage(t *stdtesting.T) {
	gc.TestingT(t)
}
