// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ociworkload

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/juju/tc"

	"github.com/juju/juju/internal/docker"
	coretesting "github.com/juju/juju/internal/testing"
)

type pullerSuite struct {
	coretesting.BaseSuite
}

// TestPullerSuite runs OCI image puller tests.
func TestPullerSuite(t *testing.T) {
	tc.Run(t, &pullerSuite{})
}

// TestCredentials checks the basic and token representations supported by
// Juju's OCI image resources, including malformed basic credentials.
func (s *pullerSuite) TestCredentials(c *tc.C) {
	cases := []struct {
		name     string
		image    docker.DockerImageDetails
		username string
		secret   string
		wantErr  bool
	}{
		{name: "public"},
		{
			name: "username and password",
			image: docker.DockerImageDetails{ImageRepoDetails: docker.ImageRepoDetails{
				BasicAuthConfig: docker.BasicAuthConfig{Username: "user", Password: "secret"},
			}},
			username: "user", secret: "secret",
		},
		{
			name: "encoded basic auth",
			image: docker.DockerImageDetails{ImageRepoDetails: docker.ImageRepoDetails{
				BasicAuthConfig: docker.BasicAuthConfig{Auth: docker.NewToken(base64.StdEncoding.EncodeToString([]byte("user:secret")))},
			}},
			username: "user", secret: "secret",
		},
		{
			name: "registry token",
			image: docker.DockerImageDetails{ImageRepoDetails: docker.ImageRepoDetails{
				TokenAuthConfig: docker.TokenAuthConfig{RegistryToken: docker.NewToken("secret")},
			}},
			secret: "secret",
		},
		{
			name: "malformed basic auth",
			image: docker.DockerImageDetails{ImageRepoDetails: docker.ImageRepoDetails{
				BasicAuthConfig: docker.BasicAuthConfig{Auth: docker.NewToken("not-base64")},
			}},
			wantErr: true,
		},
	}

	for _, test := range cases {
		c.Logf("credentials case: %s", test.name)
		username, secret, err := imageCredentials(test.image)
		if test.wantErr {
			c.Check(err, tc.NotNil)
			continue
		}

		c.Assert(err, tc.ErrorIsNil)
		c.Check(username, tc.Equals, test.username)
		c.Check(secret, tc.Equals, test.secret)
	}
}

// TestInvalidImageDoesNotConnect ensures a malformed image reference fails
// before opening a containerd connection or attempting any image pull.
func (s *pullerSuite) TestInvalidImageDoesNotConnect(c *tc.C) {
	called := false
	puller := Puller{NewClient: func() (ContainerdClient, error) {
		called = true
		return nil, errors.New("unexpected connection")
	}}
	err := puller.Pull(c.Context(), docker.DockerImageDetails{
		RegistryPath: "bad image ref",
	})
	c.Check(err, tc.ErrorMatches, `.*docker image path.*not valid.*`)
	c.Check(called, tc.IsFalse)
}
