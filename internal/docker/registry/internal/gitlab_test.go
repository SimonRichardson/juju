// Copyright 2021 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package internal_test

import (
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/core/semversion"
	"github.com/juju/juju/internal/docker"
	"github.com/juju/juju/internal/docker/registry"
	"github.com/juju/juju/internal/docker/registry/image"
	"github.com/juju/juju/internal/docker/registry/internal"
	"github.com/juju/juju/internal/docker/registry/mocks"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/internal/tools"
)

type gitlabSuite struct {
	testhelpers.IsolationSuite

	mockRoundTripper *mocks.MockRoundTripper
	imageRepoDetails docker.ImageRepoDetails
	isPrivate        bool
	authToken        string
}

func TestGitlabSuite(t *testing.T) {
	tc.Run(t, &gitlabSuite{})
}

func (s *gitlabSuite) getRegistry(c *tc.C) (registry.Registry, *gomock.Controller) {
	ctrl := gomock.NewController(c)

	s.imageRepoDetails = docker.ImageRepoDetails{
		Repository: "registry.gitlab.com/jujuqa",
	}
	s.authToken = base64.StdEncoding.EncodeToString([]byte("username:pwd"))
	if s.isPrivate {
		s.imageRepoDetails.BasicAuthConfig = docker.BasicAuthConfig{
			Auth: docker.NewToken(s.authToken),
		}
	}

	s.mockRoundTripper = mocks.NewMockRoundTripper(ctrl)
	if s.isPrivate {
		gomock.InOrder(
			// registry.Ping() 1st try failed - bearer token was missing.
			s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(
				func(req *http.Request) (*http.Response, error) {
					c.Assert(req.Header, tc.DeepEquals, http.Header{})
					c.Assert(req.Method, tc.Equals, `GET`)
					c.Assert(req.URL.String(), tc.Equals, `https://registry.gitlab.com/v2`)
					return &http.Response{
						Request:    req,
						StatusCode: http.StatusUnauthorized,
						Body:       io.NopCloser(nil),
						Header: http.Header{
							http.CanonicalHeaderKey("WWW-Authenticate"): []string{
								`Bearer realm="https://gitlab.com/jwt/auth",service="container_registry",scope="repository:jujuqa/jujud-operator:pull",error="invalid_token"`,
							},
						},
					}, nil
				},
			),
			// Refresh OAuth Token
			s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(
				func(req *http.Request) (*http.Response, error) {
					c.Assert(req.Header, tc.DeepEquals, http.Header{"Authorization": []string{"Basic " + s.authToken}})
					c.Assert(req.Method, tc.Equals, `GET`)
					c.Assert(req.URL.String(), tc.Equals, `https://gitlab.com/jwt/auth?scope=repository%3Ajujuqa%2Fjujud-operator%3Apull&service=container_registry`)
					return &http.Response{
						Request:    req,
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"token": "jwt-token", "access_token": "jwt-token","expires_in": 300}`)),
					}, nil
				},
			),
			// registry.Ping()
			s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(
				func(req *http.Request) (*http.Response, error) {
					c.Assert(req.Header, tc.DeepEquals, http.Header{"Authorization": []string{"Bearer " + `jwt-token`}})
					c.Assert(req.Method, tc.Equals, `GET`)
					c.Assert(req.URL.String(), tc.Equals, `https://registry.gitlab.com/v2`)
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(nil)}, nil
				},
			),
		)
	}
	s.PatchValue(&registry.DefaultTransport, s.mockRoundTripper)

	reg, err := registry.New(s.imageRepoDetails)
	c.Assert(err, tc.ErrorIsNil)
	_, ok := reg.(*internal.GitlabContainerRegistry)
	c.Assert(ok, tc.IsTrue)
	err = reg.Ping()
	c.Assert(err, tc.ErrorIsNil)
	return reg, ctrl
}

func (s *gitlabSuite) TestPingPublicRepository(c *tc.C) {
	s.isPrivate = false
	_, ctrl := s.getRegistry(c)
	ctrl.Finish()
}

func (s *gitlabSuite) TestPingPrivateRepository(c *tc.C) {
	s.isPrivate = true
	_, ctrl := s.getRegistry(c)
	ctrl.Finish()
}

func (s *gitlabSuite) TestTagsPublicRegistry(c *tc.C) {
	// Use anonymous login for public repository.
	s.isPrivate = false
	reg, ctrl := s.getRegistry(c)
	defer ctrl.Finish()

	data := `
{"name":"jujuqa/jujud-operator","tags":["2.9.10.1","2.9.10.2","2.9.10"]}
`[1:]

	gomock.InOrder(
		s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(
			func(req *http.Request) (*http.Response, error) {
				c.Assert(req.Header, tc.DeepEquals, http.Header{})
				c.Assert(req.Method, tc.Equals, `GET`)
				c.Assert(req.URL.String(), tc.Equals, `https://registry.gitlab.com/v2/jujuqa/jujud-operator/tags/list`)
				return &http.Response{
					Request:    req,
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(nil),
					Header: http.Header{
						http.CanonicalHeaderKey("WWW-Authenticate"): []string{
							`Bearer realm="https://gitlab.com/jwt/auth",service="container_registry",scope="repository:jujuqa/jujud-operator:pull",error="invalid_token"`,
						},
					},
				}, nil
			},
		),
		// Refresh OAuth Token without credential.
		s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(
			func(req *http.Request) (*http.Response, error) {
				c.Assert(req.Header, tc.DeepEquals, http.Header{})
				c.Assert(req.Method, tc.Equals, `GET`)
				c.Assert(req.URL.String(), tc.Equals, `https://gitlab.com/jwt/auth?scope=repository%3Ajujuqa%2Fjujud-operator%3Apull&service=container_registry`)
				return &http.Response{
					Request:    req,
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"token": "jwt-token", "access_token": "jwt-token","expires_in": 300}`)),
				}, nil
			},
		),
		s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
			c.Assert(req.Header, tc.DeepEquals, http.Header{"Authorization": []string{"Bearer jwt-token"}})
			c.Assert(req.Method, tc.Equals, `GET`)
			c.Assert(req.URL.String(), tc.Equals, `https://registry.gitlab.com/v2/jujuqa/jujud-operator/tags/list`)
			resps := &http.Response{
				Request:    req,
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(data)),
			}
			return resps, nil
		}),
	)
	vers, err := reg.Tags("jujud-operator")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(vers, tc.DeepEquals, tools.Versions{
		image.NewImageInfo(semversion.MustParse("2.9.10.1")),
		image.NewImageInfo(semversion.MustParse("2.9.10.2")),
		image.NewImageInfo(semversion.MustParse("2.9.10")),
	})
}

func (s *gitlabSuite) TestTagsPrivateRegistry(c *tc.C) {
	// Use v2 for private repository.
	s.isPrivate = true
	reg, ctrl := s.getRegistry(c)
	defer ctrl.Finish()

	data := `
{"name":"jujuqa/jujud-operator","tags":["2.9.10.1","2.9.10.2","2.9.10"]}
`[1:]

	gomock.InOrder(
		s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(
			func(req *http.Request) (*http.Response, error) {
				c.Assert(req.Header, tc.DeepEquals, http.Header{})
				c.Assert(req.Method, tc.Equals, `GET`)
				c.Assert(req.URL.String(), tc.Equals, `https://registry.gitlab.com/v2/jujuqa/jujud-operator/tags/list`)
				return &http.Response{
					Request:    req,
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(nil),
					Header: http.Header{
						http.CanonicalHeaderKey("WWW-Authenticate"): []string{
							`Bearer realm="https://gitlab.com/jwt/auth",service="container_registry",scope="repository:jujuqa/jujud-operator:pull",error="invalid_token"`,
						},
					},
				}, nil
			},
		),
		// Refresh OAuth Token.
		s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(
			func(req *http.Request) (*http.Response, error) {
				c.Assert(req.Header, tc.DeepEquals, http.Header{"Authorization": []string{"Basic " + s.authToken}})
				c.Assert(req.Method, tc.Equals, `GET`)
				c.Assert(req.URL.String(), tc.Equals, `https://gitlab.com/jwt/auth?scope=repository%3Ajujuqa%2Fjujud-operator%3Apull&service=container_registry`)
				return &http.Response{
					Request:    req,
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"token": "jwt-token", "access_token": "jwt-token","expires_in": 300}`)),
				}, nil
			},
		),
		s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
			c.Assert(req.Header, tc.DeepEquals, http.Header{"Authorization": []string{"Bearer jwt-token"}})
			c.Assert(req.Method, tc.Equals, `GET`)
			c.Assert(req.URL.String(), tc.Equals, `https://registry.gitlab.com/v2/jujuqa/jujud-operator/tags/list`)
			resps := &http.Response{
				Request:    req,
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(data)),
			}
			return resps, nil
		}),
	)
	vers, err := reg.Tags("jujud-operator")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(vers, tc.DeepEquals, tools.Versions{
		image.NewImageInfo(semversion.MustParse("2.9.10.1")),
		image.NewImageInfo(semversion.MustParse("2.9.10.2")),
		image.NewImageInfo(semversion.MustParse("2.9.10")),
	})
}

func (s *gitlabSuite) TestTagsErrorResponse(c *tc.C) {
	s.isPrivate = true
	reg, ctrl := s.getRegistry(c)
	defer ctrl.Finish()

	data := `
{"errors":[{"code":"UNAUTHORIZED","message":"authentication required"}]}
`[1:]

	gomock.InOrder(
		s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
			c.Assert(req.Header, tc.DeepEquals, http.Header{})
			c.Assert(req.Method, tc.Equals, `GET`)
			c.Assert(req.URL.String(), tc.Equals, `https://registry.gitlab.com/v2/jujuqa/jujud-operator/tags/list`)
			return &http.Response{
				Request:    req,
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(nil),
				Header: http.Header{
					http.CanonicalHeaderKey("WWW-Authenticate"): []string{
						`Bearer realm="https://gitlab.com/jwt/auth",service="container_registry",scope="repository:jujuqa/jujud-operator:pull",error="invalid_token"`,
					},
				},
			}, nil
		}),
		// Refresh OAuth Token.
		s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(
			func(req *http.Request) (*http.Response, error) {
				c.Assert(req.Header, tc.DeepEquals, http.Header{"Authorization": []string{"Basic " + s.authToken}})
				c.Assert(req.Method, tc.Equals, `GET`)
				c.Assert(req.URL.String(), tc.Equals, `https://gitlab.com/jwt/auth?scope=repository%3Ajujuqa%2Fjujud-operator%3Apull&service=container_registry`)
				return &http.Response{
					Request:    req,
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"token": "jwt-token", "access_token": "jwt-token","expires_in": 300}`)),
				}, nil
			},
		),

		s.mockRoundTripper.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
			c.Assert(req.Header, tc.DeepEquals, http.Header{"Authorization": []string{"Bearer jwt-token"}})
			c.Assert(req.Method, tc.Equals, `GET`)
			c.Assert(req.URL.String(), tc.Equals, `https://registry.gitlab.com/v2/jujuqa/jujud-operator/tags/list`)
			resps := &http.Response{
				Request:    req,
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader(data)),
			}
			return resps, nil
		}),
	)
	_, err := reg.Tags("jujud-operator")
	c.Assert(err, tc.ErrorMatches, `Get "https://registry.gitlab.com/v2/jujuqa/jujud-operator/tags/list": non-successful response status=403`)
}
