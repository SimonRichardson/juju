// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package secrets_test

import (
	"context"

	"github.com/juju/collections/set"
	"github.com/juju/errors"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"go.uber.org/mock/gomock"
	gc "gopkg.in/check.v1"

	coresecrets "github.com/juju/juju/core/secrets"
	secreterrors "github.com/juju/juju/domain/secret/errors"
	"github.com/juju/juju/internal/secrets"
	"github.com/juju/juju/internal/secrets/mocks"
	"github.com/juju/juju/internal/secrets/provider"
)

type deleteBackendSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&deleteBackendSuite{})

func (s *deleteBackendSuite) TestGetContent(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	state := mocks.NewMockSecretsState(ctrl)
	backendConfigForDeleteGetter := func(backendID string) (*provider.ModelBackendConfigInfo, error) {
		c.Fail()
		return nil, nil
	}

	uri := coresecrets.NewURI()
	client := secrets.NewClientForContentDeletion(state, backendConfigForDeleteGetter)
	_, err := client.GetContent(context.Background(), uri, "", false, false)
	c.Assert(err, jc.ErrorIs, errors.NotSupported)
}

func (s *deleteBackendSuite) TestSaveContent(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	state := mocks.NewMockSecretsState(ctrl)
	backendConfigForDeleteGetter := func(backendID string) (*provider.ModelBackendConfigInfo, error) {
		c.Fail()
		return nil, nil
	}

	uri := coresecrets.NewURI()
	client := secrets.NewClientForContentDeletion(state, backendConfigForDeleteGetter)
	_, err := client.SaveContent(context.Background(), uri, 666, coresecrets.NewSecretValue(nil))
	c.Assert(err, jc.ErrorIs, errors.NotSupported)
}

func (s *deleteBackendSuite) TestDeleteExternalContent(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	state := mocks.NewMockSecretsState(ctrl)
	backendConfigForDeleteGetter := func(backendID string) (*provider.ModelBackendConfigInfo, error) {
		c.Fail()
		return nil, nil
	}

	client := secrets.NewClientForContentDeletion(state, backendConfigForDeleteGetter)
	err := client.DeleteExternalContent(context.Background(), coresecrets.ValueRef{})
	c.Assert(err, jc.ErrorIs, errors.NotSupported)
}

func (s *deleteBackendSuite) TestGetBackend(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	state := mocks.NewMockSecretsState(ctrl)
	backendConfigForDeleteGetter := func(backendID string) (*provider.ModelBackendConfigInfo, error) {
		c.Fail()
		return nil, nil
	}

	client := secrets.NewClientForContentDeletion(state, backendConfigForDeleteGetter)
	_, _, err := client.GetBackend(context.Background(), ptr("someid"), false)
	c.Assert(err, jc.ErrorIs, errors.NotSupported)
}

func (s *deleteBackendSuite) TestGetRevisionContent(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	state := mocks.NewMockSecretsState(ctrl)
	backendConfigForDeleteGetter := func(backendID string) (*provider.ModelBackendConfigInfo, error) {
		c.Fail()
		return nil, nil
	}

	uri := coresecrets.NewURI()
	client := secrets.NewClientForContentDeletion(state, backendConfigForDeleteGetter)
	_, err := client.GetRevisionContent(context.Background(), uri, 666)
	c.Assert(err, jc.ErrorIs, errors.NotSupported)
}

func (s *deleteBackendSuite) TestDeleteContent(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	backend := mocks.NewMockSecretsBackend(ctrl)
	state := mocks.NewMockSecretsState(ctrl)

	backends := set.NewStrings("somebackend1", "somebackend2")
	s.PatchValue(&secrets.GetBackend, func(cfg *provider.ModelBackendConfig) (provider.SecretsBackend, error) {
		c.Assert(backends.Contains(cfg.BackendType), jc.IsTrue)
		return backend, nil
	})

	backendConfigForDeleteGetter := func(backendID string) (*provider.ModelBackendConfigInfo, error) {
		return &provider.ModelBackendConfigInfo{
			ActiveID: "somebackend1",
			Configs: map[string]provider.ModelBackendConfig{
				backendID: {
					ControllerUUID: "controller-uuid2",
					ModelUUID:      "model-uuid2",
					ModelName:      "model2",
					BackendConfig:  provider.BackendConfig{BackendType: "somebackend1"},
				},
			},
		}, nil
	}

	client := secrets.NewClientForContentDeletion(state, backendConfigForDeleteGetter)

	uri := coresecrets.NewURI()
	state.EXPECT().GetSecretValue(uri, 666).Return(nil, &coresecrets.ValueRef{
		BackendID:  "somebackend1",
		RevisionID: "rev-id",
	}, nil)
	backend.EXPECT().DeleteContent(gomock.Any(), "rev-id").Return(nil)

	err := client.DeleteContent(context.Background(), uri, 666)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *deleteBackendSuite) TestDeleteContentDraining(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	backend := mocks.NewMockSecretsBackend(ctrl)
	state := mocks.NewMockSecretsState(ctrl)

	backends := set.NewStrings("somebackend1", "somebackend2")
	s.PatchValue(&secrets.GetBackend, func(cfg *provider.ModelBackendConfig) (provider.SecretsBackend, error) {
		c.Assert(backends.Contains(cfg.BackendType), jc.IsTrue)
		return backend, nil
	})

	count := 0
	backendConfigForDeleteGetter := func(backendID string) (*provider.ModelBackendConfigInfo, error) {
		activeID := "somebackend2"
		if count > 0 {
			activeID = backendID
		}
		count++
		return &provider.ModelBackendConfigInfo{
			ActiveID: activeID,
			Configs: map[string]provider.ModelBackendConfig{
				backendID: {
					ControllerUUID: "controller-uuid2",
					ModelUUID:      "model-uuid2",
					ModelName:      "model2",
					BackendConfig:  provider.BackendConfig{BackendType: "somebackend1"},
				},
			},
		}, nil
	}

	client := secrets.NewClientForContentDeletion(state, backendConfigForDeleteGetter)

	uri := coresecrets.NewURI()
	state.EXPECT().GetSecretValue(uri, 666).Return(nil, &coresecrets.ValueRef{
		BackendID:  "somebackend1",
		RevisionID: "rev-id",
	}, nil).Times(2)
	backend.EXPECT().DeleteContent(gomock.Any(), "rev-id").Return(secreterrors.SecretRevisionNotFound)
	backend.EXPECT().DeleteContent(gomock.Any(), "rev-id").Return(nil)

	err := client.DeleteContent(context.Background(), uri, 666)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *deleteBackendSuite) TestDeleteInternalContentNoop(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	state := mocks.NewMockSecretsState(ctrl)

	backendConfigForDeleteGetter := func(backendID string) (*provider.ModelBackendConfigInfo, error) {
		c.Fail()
		return nil, nil
	}

	client := secrets.NewClientForContentDeletion(state, backendConfigForDeleteGetter)

	uri := coresecrets.NewURI()
	state.EXPECT().GetSecretValue(uri, 666).Return(coresecrets.NewSecretValue(nil), nil, nil)

	err := client.DeleteContent(context.Background(), uri, 666)
	c.Assert(err, jc.ErrorIsNil)
}
