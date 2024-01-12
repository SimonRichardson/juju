// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charms_test

import (
	"context"
	"net/url"

	"github.com/juju/names/v5"
	"github.com/juju/testing"
	"go.uber.org/mock/gomock"
	gc "gopkg.in/check.v1"

	basemocks "github.com/juju/juju/api/base/mocks"
	"github.com/juju/juju/api/client/charms"
	"github.com/juju/juju/api/client/charms/mocks"
	"github.com/juju/juju/internal/downloader"
)

type charmS3DownloaderSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&charmS3DownloaderSuite{})

func (s *charmS3DownloaderSuite) TestCharmOpener(c *gc.C) {
	correctURL, err := url.Parse("ch:mycharm")
	c.Assert(err, gc.IsNil)

	tests := []struct {
		name               string
		req                downloader.Request
		mocks              func(*mocks.MockCharmGetter, *basemocks.MockAPICaller)
		expectedErrPattern string
	}{
		{
			name: "invalid ArchiveSha256 in request",
			req: downloader.Request{
				ArchiveSha256: "abcd012",
			},
			expectedErrPattern: "download request with archiveSha256 length 7 not valid",
		},
		{
			name: "invalid URL in request",
			req: downloader.Request{
				ArchiveSha256: "abcd0123",
				URL: &url.URL{
					Scheme: "badscheme",
					Host:   "badhost",
				},
			},
			expectedErrPattern: "did not receive a valid charm URL.*",
		},
		{
			name: "open charm OK",
			req: downloader.Request{
				ArchiveSha256: "abcd0123",
				URL:           correctURL,
			},
			mocks: func(mockGetter *mocks.MockCharmGetter, mockCaller *basemocks.MockAPICaller) {

				modelTag := names.NewModelTag("testmodel")
				mockCaller.EXPECT().ModelTag().Return(modelTag, true)
				mockGetter.EXPECT().GetCharm(gomock.Any(), "testmodel", "mycharm-abcd0123").MinTimes(1).Return(nil, "", nil)
			},
		},
	}

	for i, tt := range tests {
		c.Logf("test %d - %s", i, tt.name)

		ctrl := gomock.NewController(c)
		defer ctrl.Finish()

		mockCaller := basemocks.NewMockAPICaller(ctrl)
		mockGetter := mocks.NewMockCharmGetter(ctrl)
		if tt.mocks != nil {
			tt.mocks(mockGetter, mockCaller)
		}

		charmOpener := charms.NewS3CharmOpener(mockGetter, mockCaller)
		r, hash, err := charmOpener.OpenCharm(context.Background(), tt.req)

		if tt.expectedErrPattern != "" {
			c.Assert(r, gc.IsNil)
			c.Check(err, gc.ErrorMatches, tt.expectedErrPattern)
			c.Check(hash, gc.Equals, "")
		} else {
			c.Assert(err, gc.IsNil)
		}
	}
}
