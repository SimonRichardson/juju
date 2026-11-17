// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter

import (
	"syscall"
	"testing"

	"github.com/juju/tc"

	coretesting "github.com/juju/juju/internal/testing"
)

type hookUserSuite struct {
	coretesting.BaseSuite
}

func TestHookUserSuite(t *testing.T) {
	tc.Run(t, &hookUserSuite{})
}

func (s *hookUserSuite) TestCharmDirHookCredential(c *tc.C) {
	credential := func(uid, gid uint32) *syscall.Credential {
		return &syscall.Credential{Uid: uid, Gid: gid}
	}
	for _, test := range []struct {
		summary  string
		charmDir string
		expected *syscall.Credential
		err      string
	}{{
		summary:  "no charm deployed yet",
		charmDir: c.MkDir() + "/no/such/charm",
	}, {
		summary:  "no containers is not governed by charm-user",
		charmDir: s.charmDir(c, metadataWithoutContainers+"\ncharm-user: non-root\n"),
	}, {
		summary:  "containers without charm-user",
		charmDir: s.charmDir(c, metadataWithContainers),
	}, {
		summary:  "root",
		charmDir: s.charmDir(c, metadataWithContainers+"\ncharm-user: root\n"),
		expected: credential(0, 0),
	}, {
		summary:  "sudoer",
		charmDir: s.charmDir(c, metadataWithContainers+"\ncharm-user: sudoer\n"),
		expected: credential(171, 171),
	}, {
		summary:  "non-root",
		charmDir: s.charmDir(c, metadataWithContainers+"\ncharm-user: non-root\n"),
		expected: credential(170, 170),
	}, {
		summary:  "invalid charm-user",
		charmDir: s.charmDir(c, metadataWithContainers+"\ncharm-user: barry\n"),
		err:      `reading deployed charm metadata: parsing "metadata.yaml" file: parsing charm-user: invalid charm-user "barry" expected one of root, sudoer or non-root`,
	}} {
		c.Logf("%s", test.summary)
		result, err := CharmDirHookCredential(test.charmDir)
		if test.err == "" {
			c.Assert(err, tc.ErrorIsNil)
			c.Check(result, tc.DeepEquals, test.expected)
		} else {
			c.Assert(err, tc.ErrorMatches, test.err)
			c.Check(result, tc.IsNil)
		}
	}
}

func (s *hookUserSuite) charmDir(c *tc.C, metadata string) string {
	charmDir := c.MkDir()
	writeCharmMetadata(c, charmDir, metadata)
	return charmDir
}
