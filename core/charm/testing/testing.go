// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package testing

import (
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	corecharm "github.com/juju/juju/core/charm"
)

// GenCharmID can be used in testing for generating a charm ID that is
// checked for subsequent errors using the test suit's go check instance.
func GenCharmID(c *gc.C) corecharm.ID {
	id, err := corecharm.NewID()
	c.Assert(err, jc.ErrorIsNil)
	return id
}
