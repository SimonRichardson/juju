// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package google_test

import (
	jujuhttp "github.com/juju/http/v2"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/internal/provider/gce/internal/google"
	"github.com/juju/juju/testing"
)

type connConfigSuite struct {
	testing.BaseSuite
}

var _ = gc.Suite(&connConfigSuite{})

func (*connConfigSuite) TestValidateValid(c *gc.C) {
	cfg := google.ConnectionConfig{
		Region:     "spam",
		HTTPClient: jujuhttp.NewClient(),
	}
	err := cfg.Validate()

	c.Check(err, jc.ErrorIsNil)
}

func (*connConfigSuite) TestValidateMissingRegion(c *gc.C) {
	cfg := google.ConnectionConfig{}
	err := cfg.Validate()

	c.Assert(err, gc.FitsTypeOf, &google.InvalidConfigValueError{})
	c.Check(err.(*google.InvalidConfigValueError).Key, gc.Equals, "GCE_REGION")
}
