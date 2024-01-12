// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package downloader_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"

	"github.com/juju/errors"
	jujutesting "github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/internal/downloader"
	"github.com/juju/juju/testing"
)

type DownloadSuite struct {
	testing.BaseSuite
	jujutesting.HTTPSuite
}

func (s *DownloadSuite) SetUpSuite(c *gc.C) {
	s.BaseSuite.SetUpSuite(c)
	s.HTTPSuite.SetUpSuite(c)
}

func (s *DownloadSuite) TearDownSuite(c *gc.C) {
	s.HTTPSuite.TearDownSuite(c)
	s.BaseSuite.TearDownSuite(c)
}

func (s *DownloadSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.HTTPSuite.SetUpTest(c)
}

func (s *DownloadSuite) TearDownTest(c *gc.C) {
	s.HTTPSuite.TearDownTest(c)
	s.BaseSuite.TearDownTest(c)
}

var _ = gc.Suite(&DownloadSuite{})

func (s *DownloadSuite) URL(c *gc.C, path string) *url.URL {
	urlStr := s.HTTPSuite.URL(path)
	URL, err := url.Parse(urlStr)
	c.Assert(err, jc.ErrorIsNil)
	return URL
}

func (s *DownloadSuite) testDownload(c *gc.C, hostnameVerification bool) {
	tmp := c.MkDir()
	jujutesting.Server.Response(200, nil, []byte("archive"))
	d := downloader.StartDownload(
		context.Background(),
		downloader.Request{
			URL:       s.URL(c, "/archive.tgz"),
			TargetDir: tmp,
		},
		downloader.NewHTTPBlobOpener(hostnameVerification),
	)
	err := d.Wait()
	c.Assert(err, jc.ErrorIsNil)

	fileName := d.FileName()
	dir, _ := filepath.Split(fileName)
	c.Assert(filepath.Clean(dir), gc.Equals, tmp)
	assertFileContents(c, fileName, "archive")
}

func (s *DownloadSuite) TestDownloadWithoutDisablingSSLHostnameVerification(c *gc.C) {
	s.testDownload(c, true)
}

func (s *DownloadSuite) TestDownloadWithDisablingSSLHostnameVerification(c *gc.C) {
	s.testDownload(c, false)
}

func (s *DownloadSuite) TestDownloadError(c *gc.C) {
	jujutesting.Server.Response(404, nil, nil)
	tmp := c.MkDir()
	d := downloader.StartDownload(
		context.Background(),
		downloader.Request{
			URL:       s.URL(c, "/archive.tgz"),
			TargetDir: tmp,
		},
		downloader.NewHTTPBlobOpener(true),
	)
	err := d.Wait()

	fileName := d.FileName()
	c.Assert(fileName, gc.Equals, "")
	c.Assert(err, gc.ErrorMatches, `bad http response: 404 Not Found`)
	checkDirEmpty(c, tmp)
}

func (s *DownloadSuite) TestVerifyValid(c *gc.C) {
	stub := &jujutesting.Stub{}
	tmp := c.MkDir()
	jujutesting.Server.Response(200, nil, []byte("archive"))
	d := downloader.StartDownload(
		context.Background(),
		downloader.Request{
			URL:       s.URL(c, "/archive.tgz"),
			TargetDir: tmp,
			Verify: func(f *os.File) error {
				stub.AddCall("Verify", f)
				return nil
			},
		},
		downloader.NewHTTPBlobOpener(true),
	)
	err := d.Wait()
	c.Assert(err, jc.ErrorIsNil)

	fileName := d.FileName()
	c.Check(fileName, gc.Not(gc.Equals), "")
	stub.CheckCallNames(c, "Verify")
}

func (s *DownloadSuite) TestVerifyInvalid(c *gc.C) {
	stub := &jujutesting.Stub{}
	tmp := c.MkDir()
	jujutesting.Server.Response(200, nil, []byte("archive"))
	invalid := errors.NotValidf("oops")
	d := downloader.StartDownload(
		context.Background(),
		downloader.Request{
			URL:       s.URL(c, "/archive.tgz"),
			TargetDir: tmp,
			Verify: func(f *os.File) error {
				stub.AddCall("Verify", f)
				return invalid
			},
		},
		downloader.NewHTTPBlobOpener(true),
	)
	err := d.Wait()
	c.Assert(errors.Cause(err), gc.Equals, invalid)

	fileName := d.FileName()
	c.Check(fileName, gc.Equals, "")
	stub.CheckCallNames(c, "Verify")
	checkDirEmpty(c, tmp)
}

func (s *DownloadSuite) TestAbort(c *gc.C) {
	tmp := c.MkDir()
	jujutesting.Server.Response(200, nil, []byte("archive"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d := downloader.StartDownload(
		ctx,
		downloader.Request{
			URL:       s.URL(c, "/archive.tgz"),
			TargetDir: tmp,
		},
		downloader.NewHTTPBlobOpener(true),
	)
	err := d.Wait()
	c.Assert(err, jc.ErrorIs, context.Canceled)

	fileName := d.FileName()
	c.Check(fileName, gc.Equals, "")
	checkDirEmpty(c, tmp)
}

func assertFileContents(c *gc.C, filename, expect string) {
	got, err := os.ReadFile(filename)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(string(got), gc.Equals, expect)
}

func checkDirEmpty(c *gc.C, dir string) {
	files, err := os.ReadDir(dir)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(files, gc.HasLen, 0)
}
