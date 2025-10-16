// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package objects

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	stdtesting "testing"

	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/apiserver/apiserverhttp"
	"github.com/juju/juju/domain/application/architecture"
	applicationcharm "github.com/juju/juju/domain/application/charm"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	"github.com/juju/juju/internal/testing"
	"github.com/juju/juju/rpc/params"
)

const (
	charmsObjectsRoutePrefix = "/model-:modeluuid/charms/:object"
)

type objectsCharmHandlerSuite struct {
	applicationsServiceGetter *MockApplicationServiceGetter
	applicationsService       *MockApplicationService

	mux *apiserverhttp.Mux
	srv *httptest.Server
}

func TestObjectsCharmHandlerSuite(t *stdtesting.T) {
	tc.Run(t, &objectsCharmHandlerSuite{})
}
func (s *objectsCharmHandlerSuite) SetUpTest(c *tc.C) {
	s.mux = apiserverhttp.NewMux()
	s.srv = httptest.NewServer(s.mux)
}

func (s *objectsCharmHandlerSuite) TearDownTest(c *tc.C) {
	s.srv.Close()
}

func (s *objectsCharmHandlerSuite) TestServeMethodNotSupported(c *tc.C) {
	defer s.setupMocks(c).Finish()

	handlers := &ObjectsCharmHTTPHandler{
		applicationServiceGetter: s.applicationsServiceGetter,
	}

	// This is a bit pathological, but we want to make sure that the handler
	// logic only actions on POST requests.
	s.mux.AddHandler("POST", charmsObjectsRoutePrefix, handlers)
	defer s.mux.RemoveHandler("POST", charmsObjectsRoutePrefix)

	modelUUID := testing.ModelTag.Id()
	hashPrefix := "0abcdef"

	url := fmt.Sprintf("%s/model-%s/charms/testcharm-%s", s.srv.URL, modelUUID, hashPrefix)
	resp, err := http.Post(url, "application/octet-stream", strings.NewReader("charm-content"))
	c.Assert(err, tc.ErrorIsNil)
	defer resp.Body.Close()

	c.Assert(resp.StatusCode, tc.Equals, http.StatusNotImplemented)
}

func (s *objectsCharmHandlerSuite) TestServeGet(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.expectApplicationService()

	s.applicationsService.EXPECT().GetCharmArchiveBySHA256Prefix(gomock.Any(), "01abcdef").Return(io.NopCloser(strings.NewReader("charm-content")), nil)

	handlers := &ObjectsCharmHTTPHandler{
		applicationServiceGetter: s.applicationsServiceGetter,
	}

	s.mux.AddHandler("GET", charmsObjectsRoutePrefix, handlers)
	defer s.mux.RemoveHandler("GET", charmsObjectsRoutePrefix)

	modelUUID := testing.ModelTag.Id()
	hashPrefix := "01abcdef"

	url := fmt.Sprintf("%s/model-%s/charms/testcharm-%s", s.srv.URL, modelUUID, hashPrefix)
	resp, err := http.Get(url)
	c.Assert(err, tc.ErrorIsNil)
	defer resp.Body.Close()

	c.Assert(resp.StatusCode, tc.Equals, http.StatusOK)
	body, err := io.ReadAll(resp.Body)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(string(body), tc.Equals, "charm-content")
}

func (s *objectsCharmHandlerSuite) TestServeGetNotFound(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.expectApplicationService()

	s.applicationsService.EXPECT().GetCharmArchiveBySHA256Prefix(gomock.Any(), "01abcdef").Return(nil, applicationerrors.CharmNotFound)

	handlers := &ObjectsCharmHTTPHandler{
		applicationServiceGetter: s.applicationsServiceGetter,
	}

	s.mux.AddHandler("GET", charmsObjectsRoutePrefix, handlers)
	defer s.mux.RemoveHandler("GET", charmsObjectsRoutePrefix)

	modelUUID := testing.ModelTag.Id()
	hashPrefix := "01abcdef"

	url := fmt.Sprintf("%s/model-%s/charms/testcharm-%s", s.srv.URL, modelUUID, hashPrefix)
	resp, err := http.Get(url)
	c.Assert(err, tc.ErrorIsNil)
	defer resp.Body.Close()

	c.Assert(resp.StatusCode, tc.Equals, http.StatusNotFound)
}

func (s *objectsCharmHandlerSuite) TestServePutIncorrectEncoding(c *tc.C) {
	defer s.setupMocks(c).Finish()

	handlers := &ObjectsCharmHTTPHandler{
		applicationServiceGetter: s.applicationsServiceGetter,
	}

	s.mux.AddHandler("PUT", charmsObjectsRoutePrefix, handlers)
	defer s.mux.RemoveHandler("PUT", charmsObjectsRoutePrefix)

	modelUUID := testing.ModelTag.Id()
	hashPrefix := "01abcdef"

	url := fmt.Sprintf("%s/model-%s/charms/testcharm-%s", s.srv.URL, modelUUID, hashPrefix)
	req, err := http.NewRequest("PUT", url, strings.NewReader("charm-content"))
	c.Assert(err, tc.ErrorIsNil)

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	c.Assert(err, tc.ErrorIsNil)
	defer resp.Body.Close()

	c.Assert(resp.StatusCode, tc.Equals, http.StatusBadRequest)
}

func (s *objectsCharmHandlerSuite) TestServePutNoJujuCharmURL(c *tc.C) {
	defer s.setupMocks(c).Finish()

	handlers := &ObjectsCharmHTTPHandler{
		applicationServiceGetter: s.applicationsServiceGetter,
		makeCharmURL:             CharmURLFromLocator,
	}

	s.expectApplicationService()

	s.mux.AddHandler("PUT", charmsObjectsRoutePrefix, handlers)
	defer s.mux.RemoveHandler("PUT", charmsObjectsRoutePrefix)

	modelUUID := testing.ModelTag.Id()
	hashPrefix := "01abcdef"

	url := fmt.Sprintf("%s/model-%s/charms/testcharm-%s", s.srv.URL, modelUUID, hashPrefix)
	req, err := http.NewRequest("PUT", url, strings.NewReader("charm-content"))
	c.Assert(err, tc.ErrorIsNil)

	req.Header.Set("Content-Type", "application/zip")

	resp, err := http.DefaultClient.Do(req)
	c.Assert(err, tc.ErrorIsNil)
	defer resp.Body.Close()

	c.Check(resp.StatusCode, tc.Equals, http.StatusBadRequest)
}

func (s *objectsCharmHandlerSuite) TestServePutInvalidSHA256Prefix(c *tc.C) {
	defer s.setupMocks(c).Finish()

	handlers := &ObjectsCharmHTTPHandler{
		applicationServiceGetter: s.applicationsServiceGetter,
		makeCharmURL:             CharmURLFromLocator,
	}

	s.expectApplicationService()

	s.mux.AddHandler("PUT", charmsObjectsRoutePrefix, handlers)
	defer s.mux.RemoveHandler("PUT", charmsObjectsRoutePrefix)

	modelUUID := testing.ModelTag.Id()
	hashPrefix := "cdef"

	url := fmt.Sprintf("%s/model-%s/charms/testcharm-%s", s.srv.URL, modelUUID, hashPrefix)
	req, err := http.NewRequest("PUT", url, strings.NewReader("charm-content"))
	c.Assert(err, tc.ErrorIsNil)

	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set(params.JujuCharmURLHeader, "ch:testcharm-1")

	resp, err := http.DefaultClient.Do(req)
	c.Assert(err, tc.ErrorIsNil)
	defer resp.Body.Close()

	c.Check(resp.StatusCode, tc.Equals, http.StatusBadRequest)
}

func (s *objectsCharmHandlerSuite) TestServePutInvalidCharmURL(c *tc.C) {
	defer s.setupMocks(c).Finish()

	handlers := &ObjectsCharmHTTPHandler{
		applicationServiceGetter: s.applicationsServiceGetter,
		makeCharmURL:             CharmURLFromLocator,
	}

	s.expectApplicationService()

	s.mux.AddHandler("PUT", charmsObjectsRoutePrefix, handlers)
	defer s.mux.RemoveHandler("PUT", charmsObjectsRoutePrefix)

	modelUUID := testing.ModelTag.Id()
	hashPrefix := "01abcdef"

	url := fmt.Sprintf("%s/model-%s/charms/testcharm_%s", s.srv.URL, modelUUID, hashPrefix)
	req, err := http.NewRequest("PUT", url, strings.NewReader("charm-content"))
	c.Assert(err, tc.ErrorIsNil)

	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set(params.JujuCharmURLHeader, "ch:testcharm-1")

	resp, err := http.DefaultClient.Do(req)
	c.Assert(err, tc.ErrorIsNil)
	defer resp.Body.Close()

	c.Check(resp.StatusCode, tc.Equals, http.StatusBadRequest)
}

func (s *objectsCharmHandlerSuite) TestServePut(c *tc.C) {
	defer s.setupMocks(c).Finish()

	handlers := &ObjectsCharmHTTPHandler{
		applicationServiceGetter: s.applicationsServiceGetter,
		makeCharmURL:             CharmURLFromLocator,
	}

	s.expectApplicationService()

	s.mux.AddHandler("PUT", charmsObjectsRoutePrefix, handlers)
	defer s.mux.RemoveHandler("PUT", charmsObjectsRoutePrefix)

	modelUUID := testing.ModelTag.Id()
	hashPrefix := "01abcdef"

	url := fmt.Sprintf("%s/model-%s/charms/testcharm-%s", s.srv.URL, modelUUID, hashPrefix)
	req, err := http.NewRequest("PUT", url, strings.NewReader("charm-content"))
	c.Assert(err, tc.ErrorIsNil)

	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set(params.JujuCharmURLHeader, "ch:testcharm-1")

	s.applicationsService.EXPECT().ResolveUploadCharm(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, args applicationcharm.ResolveUploadCharm) (applicationcharm.CharmLocator, error) {
		c.Check(args.Name, tc.Equals, "testcharm")
		c.Check(args.Revision, tc.Equals, 1)
		c.Check(args.Architecture, tc.Equals, "")

		return applicationcharm.CharmLocator{
			Name:         "testcharm",
			Revision:     2,
			Source:       applicationcharm.CharmHubSource,
			Architecture: architecture.AMD64,
		}, nil

	})

	resp, err := http.DefaultClient.Do(req)
	c.Assert(err, tc.ErrorIsNil)
	defer resp.Body.Close()

	c.Check(resp.StatusCode, tc.Equals, http.StatusOK)
	c.Check(resp.Header.Get(params.JujuCharmURLHeader), tc.Equals, "ch:amd64/testcharm-2")
}

func (s *objectsCharmHandlerSuite) TestCharmURLFromLocator(c *tc.C) {
	locator := applicationcharm.CharmLocator{
		Name:         "testcharm",
		Revision:     1,
		Source:       applicationcharm.CharmHubSource,
		Architecture: architecture.AMD64,
	}

	for _, includeArch := range []bool{true, false} {
		url, err := CharmURLFromLocator(locator, includeArch)
		c.Assert(err, tc.ErrorIsNil)
		c.Check(url.String(), tc.Equals, "ch:amd64/testcharm-1")
	}
}

func (s *objectsCharmHandlerSuite) TestCharmURLFromLocatorDuringMigration(c *tc.C) {
	locator := applicationcharm.CharmLocator{
		Name:         "testcharm",
		Revision:     1,
		Source:       applicationcharm.CharmHubSource,
		Architecture: architecture.AMD64,
	}

	tests := []struct {
		includeArch bool
		result      string
	}{
		{
			includeArch: true,
			result:      "ch:amd64/testcharm-1",
		},
		{
			includeArch: false,
			result:      "ch:testcharm-1",
		},
	}

	for _, test := range tests {
		url, err := CharmURLFromLocatorDuringMigration(locator, test.includeArch)
		c.Assert(err, tc.ErrorIsNil)
		c.Check(url.String(), tc.Equals, test.result)
	}
}

func (s *objectsCharmHandlerSuite) expectApplicationService() {
	s.applicationsServiceGetter.EXPECT().Application(gomock.Any()).Return(s.applicationsService, nil)
}

func (s *objectsCharmHandlerSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.applicationsServiceGetter = NewMockApplicationServiceGetter(ctrl)
	s.applicationsService = NewMockApplicationService(ctrl)

	return ctrl
}
