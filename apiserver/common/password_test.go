// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package common_test

import (
	"context"
	"testing"

	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/common/mocks"
	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/core/machine"
	"github.com/juju/juju/core/unit"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	controllernodeerrors "github.com/juju/juju/domain/controllernode/errors"
	internalerrors "github.com/juju/juju/internal/errors"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/rpc/params"
)

type passwordSuite struct {
	testhelpers.IsolationSuite

	agentPasswordService *mocks.MockAgentPasswordService
}

func TestPasswordSuite(t *testing.T) {
	tc.Run(t, &passwordSuite{})
}

func (s *passwordSuite) TestSetPasswordsForUnit(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().
		SetUnitPassword(gomock.Any(), unit.Name("foo/1"), "password").
		Return(nil)

	changer := common.NewPasswordChanger(s.agentPasswordService, alwaysAllow)
	results, err := changer.SetPasswords(c.Context(), params.EntityPasswords{
		Changes: []params.EntityPassword{{
			Tag:      "unit-foo/1",
			Password: "password",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{{
			Error: nil,
		}},
	})
}

func (s *passwordSuite) TestSetPasswordsForUnitError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().
		SetUnitPassword(gomock.Any(), unit.Name("foo/1"), "password").
		Return(internalerrors.Errorf("boom"))

	changer := common.NewPasswordChanger(s.agentPasswordService, alwaysAllow)
	results, err := changer.SetPasswords(c.Context(), params.EntityPasswords{
		Changes: []params.EntityPassword{{
			Tag:      "unit-foo/1",
			Password: "password",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{{
			Error: apiservererrors.ServerError(internalerrors.Errorf(`setting password for "unit-foo-1": boom`)),
		}},
	})
}

func (s *passwordSuite) TestSetPasswordsForUnitNotFoundError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().
		SetUnitPassword(gomock.Any(), unit.Name("foo/1"), "password").
		Return(applicationerrors.UnitNotFound)

	changer := common.NewPasswordChanger(s.agentPasswordService, alwaysAllow)
	results, err := changer.SetPasswords(c.Context(), params.EntityPasswords{
		Changes: []params.EntityPassword{{
			Tag:      "unit-foo/1",
			Password: "password",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{{
			Error: apiservererrors.ServerError(errors.NotFoundf(`unit "foo/1"`)),
		}},
	})
}

func (s *passwordSuite) TestSetPasswordsForMachine(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().
		SetMachinePassword(gomock.Any(), machine.Name("1"), "password").
		Return(nil)

	changer := common.NewPasswordChanger(s.agentPasswordService, alwaysAllow)
	results, err := changer.SetPasswords(c.Context(), params.EntityPasswords{
		Changes: []params.EntityPassword{{
			Tag:      "machine-1",
			Password: "password",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{{
			Error: nil,
		}},
	})
}

func (s *passwordSuite) TestSetPasswordsForMachineError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().
		SetMachinePassword(gomock.Any(), machine.Name("1"), "password").
		Return(internalerrors.Errorf("boom"))

	changer := common.NewPasswordChanger(s.agentPasswordService, alwaysAllow)
	results, err := changer.SetPasswords(c.Context(), params.EntityPasswords{
		Changes: []params.EntityPassword{{
			Tag:      "machine-1",
			Password: "password",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{{
			Error: apiservererrors.ServerError(internalerrors.Errorf(`setting password for "machine-1": boom`)),
		}},
	})
}

func (s *passwordSuite) TestSetPasswordsForMachineNotFoundError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().
		SetMachinePassword(gomock.Any(), machine.Name("1"), "password").
		Return(applicationerrors.MachineNotFound)

	changer := common.NewPasswordChanger(s.agentPasswordService, alwaysAllow)
	results, err := changer.SetPasswords(c.Context(), params.EntityPasswords{
		Changes: []params.EntityPassword{{
			Tag:      "machine-1",
			Password: "password",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{{
			Error: apiservererrors.ServerError(errors.NotFoundf(`machine "1"`)),
		}},
	})
}

func (s *passwordSuite) TestSetPasswordsForControllerNode(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().
		SetControllerNodePassword(gomock.Any(), "1", "password").
		Return(nil)

	changer := common.NewPasswordChanger(s.agentPasswordService, alwaysAllow)
	results, err := changer.SetPasswords(c.Context(), params.EntityPasswords{
		Changes: []params.EntityPassword{{
			Tag:      "controller-1",
			Password: "password",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{{
			Error: nil,
		}},
	})
}

func (s *passwordSuite) TestSetPasswordsForControllerNodeError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().
		SetControllerNodePassword(gomock.Any(), "1", "password").
		Return(internalerrors.Errorf("boom"))

	changer := common.NewPasswordChanger(s.agentPasswordService, alwaysAllow)
	results, err := changer.SetPasswords(c.Context(), params.EntityPasswords{
		Changes: []params.EntityPassword{{
			Tag:      "controller-1",
			Password: "password",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{{
			Error: apiservererrors.ServerError(internalerrors.Errorf(`setting password for "controller-1": boom`)),
		}},
	})
}

func (s *passwordSuite) TestSetPasswordsForControllerNodeNotFoundError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().
		SetControllerNodePassword(gomock.Any(), "1", "password").
		Return(controllernodeerrors.NotFound)

	changer := common.NewPasswordChanger(s.agentPasswordService, alwaysAllow)
	results, err := changer.SetPasswords(c.Context(), params.EntityPasswords{
		Changes: []params.EntityPassword{{
			Tag:      "controller-1",
			Password: "password",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{{
			Error: apiservererrors.ServerError(errors.NotFoundf(`controller node "1"`)),
		}},
	})
}

func (s *passwordSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.agentPasswordService = mocks.NewMockAgentPasswordService(ctrl)

	return ctrl
}

func alwaysAllow(ctx context.Context) (common.AuthFunc, error) {
	return func(tag names.Tag) bool {
		return true
	}, nil
}
