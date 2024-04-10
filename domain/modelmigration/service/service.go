// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"
)

// State describes retrieval and persistence methods for storage.
type State interface {
	EnsureSchema(context.Context) error
	DestroySchema(context.Context) error
}

// Service provides the API for working with external controllers.
type Service struct {
	st State
}

// NewService returns a new service reference wrapping the input state.
func NewService(st State) *Service {
	return &Service{
		st: st,
	}
}

// EnsureModelSchema ensures that the model schema is created for a given model.
// If the model schema already exists, it is a no-op.
// We've intentionally left out providing a model DDL to ensure that you can't
// accidentally apply the wrong schema to the wrong model.
func (s *Service) EnsureModelSchema(ctx context.Context) error {
	return s.st.EnsureSchema(ctx)
}

// DestroyModelSchema destroys the model schema. This includes all data in the
// model. Removing constraints, triggers and tables.
// If the model schema has already been destroyed, it is a no-op.
func (s *Service) DestroyModelSchema(ctx context.Context) error {
	return s.st.DestroySchema(ctx)
}
