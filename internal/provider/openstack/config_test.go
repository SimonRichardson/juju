// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package openstack

import (
	"context"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/internal/testing"
)

type ConfigSuite struct {
	testing.BaseSuite
}

var _ = gc.Suite(&ConfigSuite{})

// configTest specifies a config parsing test, checking that env when
// parsed as the openstack section of a config file matches
// baseConfigResult when mutated by the mutate function, or that the
// parse matches the given error.
type configTest struct {
	summary                 string
	config                  testing.Attrs
	change                  map[string]interface{}
	expect                  map[string]interface{}
	useDefaultSecurityGroup bool
	network                 string
	externalNetwork         string
	firewallMode            string
	err                     string
	sslHostnameVerification bool
	sslHostnameSet          bool
	blockStorageSource      string
}

var requiredConfig = testing.Attrs{}

func (t configTest) check(c *gc.C) {
	attrs := testing.FakeConfig().Merge(testing.Attrs{
		"type": "openstack",
	}).Merge(t.config)

	cfg, err := config.New(config.NoDefaults, attrs)
	c.Assert(err, jc.ErrorIsNil)

	e := &Environ{}
	err = e.SetConfig(context.Background(), cfg)

	if t.change != nil {
		c.Assert(err, jc.ErrorIsNil)

		// Testing a change in configuration.
		var old, changed, valid *config.Config
		osenv := e
		old = osenv.ecfg().Config
		changed, err = old.Apply(t.change)
		c.Assert(err, jc.ErrorIsNil)

		// Keep err for validation below.
		valid, err = providerInstance.Validate(context.Background(), changed, old)
		if err == nil {
			err = osenv.SetConfig(context.Background(), valid)
		}
	}
	if t.err != "" {
		c.Check(err, gc.ErrorMatches, t.err)
		return
	}
	c.Assert(err, jc.ErrorIsNil)

	ecfg := e.ecfg()
	c.Check(ecfg.Name(), gc.Equals, "testmodel")
	if t.firewallMode != "" {
		c.Check(ecfg.FirewallMode(), gc.Equals, t.firewallMode)
	}
	c.Check(ecfg.useDefaultSecurityGroup(), gc.Equals, t.useDefaultSecurityGroup)
	c.Check(ecfg.networks(), gc.DeepEquals, []string{t.network})
	c.Check(ecfg.externalNetwork(), gc.Equals, t.externalNetwork)
	// Default should be true
	expectedHostnameVerification := true
	if t.sslHostnameSet {
		expectedHostnameVerification = t.sslHostnameVerification
	}
	c.Check(ecfg.SSLHostnameVerification(), gc.Equals, expectedHostnameVerification)
	for name, expect := range t.expect {
		actual, found := ecfg.UnknownAttrs()[name]
		c.Check(found, jc.IsTrue)
		c.Check(actual, gc.Equals, expect)
	}
	if t.blockStorageSource != "" {
		storage, ok := ecfg.StorageDefaultBlockSource()
		c.Assert(ok, jc.IsTrue)
		c.Check(storage, gc.Equals, t.blockStorageSource)
	}
}

func (s *ConfigSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.PatchValue(&authenticateClient, func(context.Context, authenticator) error { return nil })
}

var configTests = []configTest{
	{
		summary: "default use default security group",
		config:  requiredConfig,
		// Do not use default security group by default.
		useDefaultSecurityGroup: false,
	}, {
		summary: "use default security group",
		config: requiredConfig.Merge(testing.Attrs{
			"use-default-secgroup": true,
		}),
		useDefaultSecurityGroup: true,
	}, {
		summary: "admin-secret given",
		config: requiredConfig.Merge(testing.Attrs{
			"admin-secret": "Futumpsh",
		}),
	}, {
		summary:      "default firewall-mode",
		config:       requiredConfig,
		firewallMode: config.FwInstance,
	}, {
		summary: "instance firewall-mode",
		config: requiredConfig.Merge(testing.Attrs{
			"firewall-mode": "instance",
		}),
		firewallMode: config.FwInstance,
	}, {
		summary: "global firewall-mode",
		config: requiredConfig.Merge(testing.Attrs{
			"firewall-mode": "global",
		}),
		firewallMode: config.FwGlobal,
	}, {
		summary: "none firewall-mode",
		config: requiredConfig.Merge(testing.Attrs{
			"firewall-mode": "none",
		}),
		firewallMode: config.FwNone,
	}, {
		config: requiredConfig.Merge(testing.Attrs{
			"future": "hammerstein",
		}),
		expect: testing.Attrs{
			"future": "hammerstein",
		},
	}, {
		config: requiredConfig,
		change: testing.Attrs{
			"future": "hammerstein",
		},
		expect: testing.Attrs{
			"future": "hammerstein",
		},
	}, {
		config: requiredConfig,
		change: testing.Attrs{
			"ssl-hostname-verification": false,
		},
		sslHostnameVerification: false,
		sslHostnameSet:          true,
	}, {
		config: requiredConfig,
		change: testing.Attrs{
			"ssl-hostname-verification": true,
		},
		sslHostnameVerification: true,
		sslHostnameSet:          true,
	}, {
		summary: "default network",
		config:  requiredConfig,
		network: "",
	}, {
		summary: "network",
		config: requiredConfig.Merge(testing.Attrs{
			NetworkKey: "a-network-label",
		}),
		network: "a-network-label",
	}, {}, {
		summary:         "default external network",
		config:          requiredConfig,
		externalNetwork: "",
	}, {
		summary: "external network",
		config: requiredConfig.Merge(testing.Attrs{
			"external-network": "a-external-network-label",
		}),
		externalNetwork: "a-external-network-label",
	}, {
		summary: "block storage specified",
		config: requiredConfig.Merge(testing.Attrs{
			"storage-default-block-source": "my-cinder",
		}),
		blockStorageSource: "my-cinder",
	}, {
		summary: "use gbp set, ptg not set",
		config: requiredConfig.Merge(testing.Attrs{
			"use-openstack-gbp": true,
		}),
		err: "policy-target-group must be set when use-openstack-gbp is set",
	}, {
		summary: "use gbp set, ptg set",
		config: requiredConfig.Merge(testing.Attrs{
			"use-openstack-gbp":   true,
			"policy-target-group": "fb19cd79-a25c-4357-9271-b071c5cb726c",
		}),
	}, {
		summary: "use gbp not set, ptg set",
		config: requiredConfig.Merge(testing.Attrs{
			"policy-target-group": "fb19cd79-a25c-4357-9271-b071c5cb726c",
		}),
	}, {
		summary: "use gbp set, ptg set, network set",
		config: requiredConfig.Merge(testing.Attrs{
			"use-openstack-gbp":   true,
			"policy-target-group": "fb19cd79-a25c-4357-9271-b071c5cb726c",
			"network":             "fb19cd79-a25c-4357-9271-b071c5cb726c",
		}),
		err: "cannot use 'network' config setting when use-openstack-gbp is set",
	}, {
		summary: "use gbp set, ptg not an UUID",
		config: requiredConfig.Merge(testing.Attrs{
			"use-openstack-gbp":   true,
			"policy-target-group": "groundcontroltomajortom",
		}),
		err: "policy-target-group has invalid UUID: .*",
	},
}

func (s *ConfigSuite) TestConfig(c *gc.C) {
	for i, t := range configTests {
		c.Logf("test %d: %s (%v)", i, t.summary, t.config)
		t.check(c)
	}
}

func (s *ConfigSuite) TestDeprecatedAttributesRemoved(c *gc.C) {
	attrs := testing.FakeConfig().Merge(testing.Attrs{
		"type":                  "openstack",
		"default-image-id":      "id-1234",
		"default-instance-type": "big",
	})

	cfg, err := config.New(config.NoDefaults, attrs)
	c.Assert(err, jc.ErrorIsNil)
	// Keep err for validation below.
	valid, err := providerInstance.Validate(context.Background(), cfg, nil)
	c.Assert(err, jc.ErrorIsNil)
	// Check deprecated attributes removed.
	allAttrs := valid.AllAttrs()
	for _, attr := range []string{"default-image-id", "default-instance-type"} {
		_, ok := allAttrs[attr]
		c.Assert(ok, jc.IsFalse)
	}
}

func (*ConfigSuite) TestSchema(c *gc.C) {
	fields := providerInstance.Schema()
	// Check that all the fields defined in environs/config
	// are in the returned schema.
	globalFields, err := config.Schema(nil)
	c.Assert(err, gc.IsNil)
	for name, field := range globalFields {
		c.Check(fields[name], jc.DeepEquals, field)
	}
}
