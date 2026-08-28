// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc

import (
	stdtesting "testing"

	"github.com/juju/errors"
	"github.com/juju/tc"

	"github.com/juju/juju/rpc/params"
)

type UnitSnapshotSuite struct{}

func TestUnitSnapshotSuite(t *stdtesting.T) {
	tc.Run(t, &UnitSnapshotSuite{})
}

func (s *UnitSnapshotSuite) TestRunFailsWithoutHolisticSnapshot(c *tc.C) {
	command, err := NewUnitSnapshotCommand(unavailableSnapshotContext{})
	c.Assert(err, tc.ErrorIsNil)

	err = command.(*UnitSnapshotCommand).Run(nil)

	c.Check(err, tc.ErrorIs, errors.NotFound)
	c.Check(err, tc.ErrorMatches, "unit snapshot not found")
}

type unavailableSnapshotContext struct{ Context }

func (unavailableSnapshotContext) UnitSnapshot() (params.UnitSnapshot, error) {
	return params.UnitSnapshot{}, errors.NotFoundf("unit snapshot")
}
