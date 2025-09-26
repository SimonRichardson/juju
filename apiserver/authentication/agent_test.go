// Copyright 2014 Canonical Ltd. All rights reserved.
// Licensed under the AGPLv3, see LICENCE file for details.

package authentication_test

import (
	"testing"

	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/apiserver/authentication"
	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/core/machine"
	"github.com/juju/juju/core/unit"
	agentpassworderrors "github.com/juju/juju/domain/agentpassword/errors"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	controllernodeerrors "github.com/juju/juju/domain/controllernode/errors"
	machineerrors "github.com/juju/juju/domain/machine/errors"
	loggertesting "github.com/juju/juju/internal/logger/testing"
)

type agentAuthenticatorSuite struct {
	agentPasswordService *MockAgentPasswordService
}

func TestAgentAuthenticatorSuite(t *testing.T) {
	tc.Run(t, &agentAuthenticatorSuite{})
}

func (s *agentAuthenticatorSuite) TestStub(c *tc.C) {
	c.Skip(`This suite is missing tests for the following scenarios:

- Login for invalid relation entity.
`)
}

func (s *agentAuthenticatorSuite) TestUserLogin(c *tc.C) {
	defer s.setupMocks(c).Finish()

	authTag := names.NewUserTag("joeblogs")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag: authTag,
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrBadRequest)
}

func (s *agentAuthenticatorSuite) TestUnitLogin(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesUnitPasswordHash(gomock.Any(), unit.Name("foo/0"), "password").Return(true, nil)

	authTag := names.NewUnitTag("foo/0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	authenticatedTag, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "password",
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(authenticatedTag, tc.DeepEquals, authTag)
}

func (s *agentAuthenticatorSuite) TestUnitLoginEmptyCredentials(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesUnitPasswordHash(gomock.Any(), unit.Name("foo/0"), "").Return(false, agentpassworderrors.EmptyPassword)

	authTag := names.NewUnitTag("foo/0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrBadRequest)
}

func (s *agentAuthenticatorSuite) TestUnitLoginInvalidCredentials(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesUnitPasswordHash(gomock.Any(), unit.Name("foo/0"), "").Return(false, agentpassworderrors.InvalidPassword)

	authTag := names.NewUnitTag("foo/0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrUnauthorized)
}

func (s *agentAuthenticatorSuite) TestUnitLoginUnitNotFound(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesUnitPasswordHash(gomock.Any(), unit.Name("foo/0"), "").Return(false, applicationerrors.UnitNotFound)

	authTag := names.NewUnitTag("foo/0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrUnauthorized)
}

func (s *agentAuthenticatorSuite) TestUnitLoginUnitError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesUnitPasswordHash(gomock.Any(), unit.Name("foo/0"), "").Return(false, errors.Errorf("boom"))

	authTag := names.NewUnitTag("foo/0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorMatches, "boom")
}

func (s *agentAuthenticatorSuite) TestMachineLogin(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesMachinePasswordHashWithNonce(gomock.Any(), machine.Name("0"), "password", "nonce").Return(true, nil)

	authTag := names.NewMachineTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	authenticatedTag, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "password",
		Nonce:       "nonce",
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(authenticatedTag, tc.DeepEquals, authTag)
}

func (s *agentAuthenticatorSuite) TestMachineLoginEmptyCredentials(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesMachinePasswordHashWithNonce(gomock.Any(), machine.Name("0"), "", "").Return(false, agentpassworderrors.EmptyPassword)

	authTag := names.NewMachineTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
		Nonce:       "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrBadRequest)
}

func (s *agentAuthenticatorSuite) TestMachineLoginEmptyNonce(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesMachinePasswordHashWithNonce(gomock.Any(), machine.Name("0"), "password", "").Return(false, agentpassworderrors.EmptyNonce)

	authTag := names.NewMachineTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "password",
		Nonce:       "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrBadRequest)
}

func (s *agentAuthenticatorSuite) TestMachineLoginInvalidCredentials(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesMachinePasswordHashWithNonce(gomock.Any(), machine.Name("0"), "", "").Return(false, agentpassworderrors.InvalidPassword)

	authTag := names.NewMachineTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
		Nonce:       "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrUnauthorized)
}

func (s *agentAuthenticatorSuite) TestMachineLoginMachineNotFound(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesMachinePasswordHashWithNonce(gomock.Any(), machine.Name("0"), "", "").Return(false, applicationerrors.MachineNotFound)

	authTag := names.NewMachineTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
		Nonce:       "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrUnauthorized)
}

func (s *agentAuthenticatorSuite) TestMachineLoginMachineNotProvisioned(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesMachinePasswordHashWithNonce(gomock.Any(), machine.Name("0"), "", "").Return(false, machineerrors.NotProvisioned)

	authTag := names.NewMachineTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
		Nonce:       "",
	})
	c.Assert(err, tc.ErrorIs, errors.NotProvisioned)
}

func (s *agentAuthenticatorSuite) TestMachineLoginMachineError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesMachinePasswordHashWithNonce(gomock.Any(), machine.Name("0"), "", "").Return(false, errors.Errorf("boom"))

	authTag := names.NewMachineTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorMatches, "boom")
}

func (s *agentAuthenticatorSuite) TestControllerNodeLogin(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesControllerNodePasswordHash(gomock.Any(), "0", "password").Return(true, nil)

	authTag := names.NewControllerAgentTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	authenticatedTag, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "password",
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(authenticatedTag, tc.DeepEquals, authTag)
}

func (s *agentAuthenticatorSuite) TestControllerNodeLoginEmptyCredentials(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesControllerNodePasswordHash(gomock.Any(), "0", "").Return(false, agentpassworderrors.EmptyPassword)

	authTag := names.NewControllerAgentTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrBadRequest)
}

func (s *agentAuthenticatorSuite) TestControllerNodeLoginInvalidCredentials(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesControllerNodePasswordHash(gomock.Any(), "0", "").Return(false, agentpassworderrors.InvalidPassword)

	authTag := names.NewControllerAgentTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrUnauthorized)
}

func (s *agentAuthenticatorSuite) TestControllerNodeLoginControllerNodeNotFound(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesControllerNodePasswordHash(gomock.Any(), "0", "").Return(false, controllernodeerrors.NotFound)

	authTag := names.NewControllerAgentTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrUnauthorized)
}

func (s *agentAuthenticatorSuite) TestControllerNodeLoginControllerNodeError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesControllerNodePasswordHash(gomock.Any(), "0", "").Return(false, errors.Errorf("boom"))

	authTag := names.NewControllerAgentTag("0")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorMatches, "boom")
}

func (s *agentAuthenticatorSuite) TestApplicationLogin(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesApplicationPasswordHash(gomock.Any(), "foo", "password").Return(true, nil)

	authTag := names.NewApplicationTag("foo")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	authenticatedTag, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "password",
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(authenticatedTag, tc.DeepEquals, authTag)
}

func (s *agentAuthenticatorSuite) TestApplicationLoginEmptyCredentials(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesApplicationPasswordHash(gomock.Any(), "foo", "").Return(false, agentpassworderrors.EmptyPassword)

	authTag := names.NewApplicationTag("foo")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrBadRequest)
}

func (s *agentAuthenticatorSuite) TestApplicationLoginInvalidCredentials(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesApplicationPasswordHash(gomock.Any(), "foo", "123").Return(false, agentpassworderrors.InvalidPassword)

	authTag := names.NewApplicationTag("foo")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "123",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrUnauthorized)
}

func (s *agentAuthenticatorSuite) TestApplicationLoginApplicationNotFound(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesApplicationPasswordHash(gomock.Any(), "foo", "").Return(false, applicationerrors.ApplicationNotFound)

	authTag := names.NewApplicationTag("foo")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrUnauthorized)
}

func (s *agentAuthenticatorSuite) TestApplicationLoginOtherError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesApplicationPasswordHash(gomock.Any(), "foo", "").Return(false, errors.Errorf("boom"))

	authTag := names.NewApplicationTag("foo")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorMatches, "boom")
}

func (s *agentAuthenticatorSuite) TestModelLogin(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesModelPasswordHash(gomock.Any(), "password").Return(true, nil)

	authTag := names.NewModelTag("test-model")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	authenticatedTag, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "password",
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(authenticatedTag, tc.DeepEquals, authTag)
}

func (s *agentAuthenticatorSuite) TestModelLoginEmptyCredentials(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesModelPasswordHash(gomock.Any(), "").Return(false, agentpassworderrors.EmptyPassword)

	authTag := names.NewModelTag("test-model")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrBadRequest)
}

func (s *agentAuthenticatorSuite) TestModelLoginInvalidCredentials(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesModelPasswordHash(gomock.Any(), "wrongpass").Return(false, agentpassworderrors.InvalidPassword)

	authTag := names.NewModelTag("test-model")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "wrongpass",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrUnauthorized)
}

func (s *agentAuthenticatorSuite) TestModelLoginOtherError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesModelPasswordHash(gomock.Any(), "password").Return(false, errors.Errorf("boom"))

	authTag := names.NewModelTag("test-model")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "password",
	})
	c.Assert(err, tc.ErrorMatches, "boom")
}

func (s *agentAuthenticatorSuite) TestModelLoginNotValid(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.agentPasswordService.EXPECT().MatchesModelPasswordHash(gomock.Any(), "password").Return(false, nil)

	authTag := names.NewModelTag("test-model")

	authenticatorGetter := authentication.NewAgentAuthenticatorGetter(s.agentPasswordService, loggertesting.WrapCheckLog(c))
	_, err := authenticatorGetter.Authenticator().Authenticate(c.Context(), authentication.AuthParams{
		AuthTag:     authTag,
		Credentials: "password",
	})
	c.Assert(err, tc.ErrorIs, apiservererrors.ErrUnauthorized)
}

func (s *agentAuthenticatorSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.agentPasswordService = NewMockAgentPasswordService(ctrl)

	return ctrl
}
