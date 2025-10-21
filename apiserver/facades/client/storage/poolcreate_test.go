// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package storage

import (
	"testing"

	"github.com/juju/errors"
	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/internal/storage/provider"
	"github.com/juju/juju/rpc/params"
)

type poolCreateSuite struct {
	baseStorageSuite
}

func TestPoolCreateSuite(t *testing.T) {
	tc.Run(t, &poolCreateSuite{})
}

func (s *poolCreateSuite) TestCreatePool(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.storageService.EXPECT().CreateStoragePool(gomock.Any(), "pname", provider.LoopProviderType, nil).Return(nil)

	args := params.StoragePoolArgs{
		Pools: []params.StoragePool{{
			Name:     "pname",
			Provider: "loop",
			Attrs:    nil,
		}},
	}
	results, err := s.api.CreatePool(c.Context(), args)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(results.Results, tc.HasLen, 1)
	c.Assert(results.Results[0].Error, tc.IsNil)
}

func (s *poolCreateSuite) TestCreatePoolError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.storageService.EXPECT().CreateStoragePool(gomock.Any(), "doesnt-matter", gomock.Any(), gomock.Any()).Return(errors.New("as expected"))

	args := params.StoragePoolArgs{
		Pools: []params.StoragePool{{
			Name: "doesnt-matter",
		}},
	}
	results, err := s.api.CreatePool(c.Context(), args)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(results.Results, tc.HasLen, 1)
	c.Assert(results.Results[0].Error, tc.DeepEquals, &params.Error{
		Message: "as expected",
	})
}
