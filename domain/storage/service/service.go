// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"

	corestorage "github.com/juju/juju/core/storage"
	"github.com/juju/juju/core/trace"
	"github.com/juju/juju/internal/errors"
	internalstorage "github.com/juju/juju/internal/storage"
)

// State defines an interface for interacting with the underlying state.
type State interface {
	StoragePoolState
	StorageState
}

// Service defines a service for interacting with the underlying state.
type Service struct {
	*StoragePoolService
	*StorageService
}

// NewService returns a new Service for interacting with the underlying state.
func NewService(st State, registryGetter corestorage.ModelStorageRegistryGetter) *Service {
	return &Service{
		StoragePoolService: &StoragePoolService{
			st:             st,
			registryGetter: registryGetter,
		},
		StorageService: &StorageService{
			st:             st,
			registryGetter: registryGetter,
		},
	}
}

// GetStorageRegistry returns the storage registry for the model.
//
// Deprecated: This method will be removed once the storage registry is fully
// implemented in each service.
func (s *Service) GetStorageRegistry(ctx context.Context) (internalstorage.ProviderRegistry, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	registry, err := s.StorageService.registryGetter.GetStorageRegistry(ctx)
	if err != nil {
		return nil, errors.Errorf("getting storage registry: %w", err)
	}
	return registry, nil
}
