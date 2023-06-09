// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package stateauthenticator_test

import (
	"testing"

	coretesting "github.com/juju/juju/testing"
)

//go:generate go run github.com/golang/mock/mockgen -package stateauthenticator_test -destination domain_mock_test.go github.com/juju/juju/apiserver/stateauthenticator ControllerConfigGetter

func TestPackage(t *testing.T) {
	coretesting.MgoTestPackage(t)
}
