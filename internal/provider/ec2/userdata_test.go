// Copyright 2015 Canonical Ltd.
// Copyright 2015 Cloudbase Solutions SRL
// Licensed under the AGPLv3, see LICENCE file for details.

package ec2_test

import (
	stdtesting "testing"

	"github.com/juju/tc"
	"github.com/juju/utils/v4"

	"github.com/juju/juju/core/os/ostype"
	"github.com/juju/juju/internal/cloudconfig/cloudinit/cloudinittest"
	"github.com/juju/juju/internal/provider/ec2"
	"github.com/juju/juju/internal/testing"
)

type UserdataSuite struct {
	testing.BaseSuite
}

func TestUserdataSuite(t *stdtesting.T) {
	tc.Run(t, &UserdataSuite{})
}

func (s *UserdataSuite) TestAmazonUnix(c *tc.C) {
	renderer := ec2.AmazonRenderer{}
	cloudcfg := &cloudinittest.CloudConfig{YAML: []byte("yaml")}

	result, err := renderer.Render(cloudcfg, ostype.Ubuntu)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result, tc.DeepEquals, utils.Gzip(cloudcfg.YAML))
}

func (s *UserdataSuite) TestAmazonUnknownOS(c *tc.C) {
	renderer := ec2.AmazonRenderer{}
	cloudcfg := &cloudinittest.CloudConfig{}
	result, err := renderer.Render(cloudcfg, ostype.GenericLinux)
	c.Assert(result, tc.IsNil)
	c.Assert(err, tc.ErrorMatches, "Cannot encode userdata for OS: GenericLinux")
}
