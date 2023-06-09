// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package peergrouper_test

import (
	stdtesting "testing"

	"github.com/juju/juju/testing"
)

//go:generate go run github.com/golang/mock/mockgen -package mocks -destination mocks/changestream_mock.go github.com/juju/juju/core/changestream WatchableDBGetter

func Test(t *stdtesting.T) {
	testing.MgoTestPackage(t)
}
