// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"

	"github.com/juju/collections/transform"

	"github.com/juju/juju/core/crossmodel"
	"github.com/juju/juju/core/offer"
	corerelation "github.com/juju/juju/core/relation"
	"github.com/juju/juju/core/trace"
	"github.com/juju/juju/domain/crossmodelrelation"
	crossmodelrelationerrors "github.com/juju/juju/domain/crossmodelrelation/errors"
	"github.com/juju/juju/internal/errors"
	"github.com/juju/juju/internal/uuid"
)

// ModelOfferState describes retrieval and persistence methods for cross model
// relations in the model database.
type ModelOfferState interface {
	// CreateOffer creates an offer and links the endpoints to it.
	CreateOffer(
		context.Context,
		crossmodelrelation.CreateOfferArgs,
	) error

	// DeleteFailedOffer deletes the provided offer, used after adding
	// permissions failed. Assumes that the offer is never used, no
	// checking of relations is required.
	DeleteFailedOffer(
		context.Context,
		offer.UUID,
	) error

	// GetOfferDetails returns the OfferDetail of every offer in the model.
	// No error is returned if offers are found.
	GetOfferDetails(context.Context, crossmodelrelation.OfferFilter) ([]*crossmodelrelation.OfferDetail, error)

	// GetOfferUUID returns the offer uuid for provided name.
	// Returns crossmodelrelationerrors.OfferNotFound of the offer is not found.
	GetOfferUUID(ctx context.Context, name string) (string, error)

	// GetOfferUUIDByRelationUUID returns the offer UUID corresponding to
	// the cross model relation UUID, returning an error satisfying
	// [crossmodelrelationerrors.OfferNotFound] if the relation is not found.
	GetOfferUUIDByRelationUUID(ctx context.Context, relationUUID string) (string, error)
}

// GetOfferUUID returns the uuid for the provided offer URL.
// Returns crossmodelrelationerrors.OfferNotFound of the offer is not found.
func (s *Service) GetOfferUUID(ctx context.Context, offerURL crossmodel.OfferURL) (offer.UUID, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	offerUUID, err := s.modelState.GetOfferUUID(ctx, offerURL.Name)
	if err != nil {
		return "", errors.Capture(err)
	}
	res, err := offer.ParseUUID(offerUUID)
	if err != nil {
		return "", errors.Errorf("parsing offer UUID: %w", err)
	}
	return res, nil
}

// GetOfferUUIDByRelationUUID returns the offer UUID corresponding to
// the cross model relation UUID, returning an error satisfying
// [crossmodelrelationerrors.OfferNotFound] if the relation is not found.
func (s *Service) GetOfferUUIDByRelationUUID(ctx context.Context, relationUUID corerelation.UUID) (offer.UUID, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	if err := relationUUID.Validate(); err != nil {
		return "", errors.Errorf("validating relation UUID: %w", err)
	}

	offerUUID, err := s.modelState.GetOfferUUIDByRelationUUID(ctx, relationUUID.String())
	if err != nil {
		return "", errors.Capture(err)
	}
	res, err := offer.ParseUUID(offerUUID)
	if err != nil {
		return "", errors.Errorf("parsing offer UUID: %w", err)
	}
	return res, nil
}

// Offer updates an existing offer, or creates a new offer if it does not exist.
// Permissions are created for a new offer only.
func (s *Service) Offer(
	ctx context.Context,
	args crossmodelrelation.ApplicationOfferArgs,
) error {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	if err := args.Validate(); err != nil {
		return errors.Capture(err)
	}

	if args.OfferName == "" {
		args.OfferName = args.ApplicationName
	}

	offerUUID, err := offer.NewUUID()
	if err != nil {
		return errors.Capture(err)
	}
	permissionUUID, err := uuid.NewUUID()
	if err != nil {
		return errors.Capture(err)
	}
	createArgs := crossmodelrelation.MakeCreateOfferArgs(args, offerUUID)

	// Check if the offer already exists.
	existingOfferUUID, err := s.modelState.GetOfferUUID(ctx, args.OfferName)
	if err != nil && !errors.Is(err, crossmodelrelationerrors.OfferNotFound) {
		return errors.Errorf("create offer: %w", err)
	} else if err == nil {
		// The offer exists, this means that we have to return an error since we
		// don't support updating offers.
		return errors.Errorf("create offer: offer %q already exists with UUID %q",
			args.OfferName, existingOfferUUID).Add(crossmodelrelationerrors.OfferAlreadyExists)
	}

	// Verify the owner exists, has not been removed, and
	// is not disabled before creating. Other users can
	// update an offer, such an admin.
	ownerUUID, err := s.controllerState.GetUserUUIDByName(ctx, args.OwnerName)
	if err != nil {
		return errors.Errorf("create offer: %w", err)
	}

	// The offer does not exist, create it.
	err = s.modelState.CreateOffer(ctx, createArgs)
	if err != nil {
		return errors.Errorf("create offer: %w", err)
	}

	err = s.controllerState.CreateOfferAccess(ctx, permissionUUID, offerUUID, ownerUUID)
	if err == nil {
		return nil
	}

	// If we fail to create offer access rows, delete the offer.
	deleteErr := s.modelState.DeleteFailedOffer(ctx, offerUUID)
	if deleteErr != nil {
		err = errors.Join(err, deleteErr)
	}
	err = errors.Errorf("creating access for offer %q: %w", args.OfferName, err)
	return errors.Capture(err)
}

// GetOffers returns offer details for all offers satisfying any of the
// provided filters.
func (s *Service) GetOffers(
	ctx context.Context,
	filters []OfferFilter,
) ([]*crossmodelrelation.OfferDetail, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	details := make([]*crossmodelrelation.OfferDetail, 0)
	var err error
	for _, filter := range filters {
		offerFilterArg := encodeInternalOfferFilter(filter)

		var offerUUIDs []string
		if len(filter.AllowedConsumers) > 0 {
			offerUUIDs, err = s.controllerState.GetOfferUUIDsForUsersWithConsume(ctx, filter.AllowedConsumers)
			if err != nil {
				return nil, errors.Errorf("getting offer UUIDs for allowed consumers: %w", err)
			}
			// If there are no offerUUIDs and nothing else in the filter,
			// move on to the next. The filter has been satisfied.
			if len(offerUUIDs) == 0 && offerFilterArg.Empty() {
				continue
			}
			offerFilterArg.OfferUUIDs = offerUUIDs
		}

		output, err := s.modelState.GetOfferDetails(ctx, offerFilterArg)
		if err != nil {
			return nil, errors.Errorf("getting offer details: %w", err)
		}

		outputWithUsers, err := s.addOfferUsers(ctx, output)
		if err != nil {
			return nil, errors.Errorf("adding allowed consumers: %w", err)
		}

		details = append(details, outputWithUsers...)
	}
	return details, nil
}

func (s *Service) addOfferUsers(
	ctx context.Context,
	input []*crossmodelrelation.OfferDetail,
) ([]*crossmodelrelation.OfferDetail, error) {
	if len(input) == 0 {
		return input, nil
	}

	output := make([]*crossmodelrelation.OfferDetail, 0)

	offerUUIDsForConsumers := transform.Slice(
		input,
		func(in *crossmodelrelation.OfferDetail) string { return in.OfferUUID },
	)

	usersWithConsume, err := s.controllerState.GetUsersForOfferUUIDs(ctx, offerUUIDsForConsumers)
	if err != nil {
		return nil, errors.Errorf("getting offer consumers: %w", err)
	}
	if len(usersWithConsume) == 0 {
		return input, nil
	}

	for _, in := range input {
		users, ok := usersWithConsume[in.OfferUUID]
		if !ok {
			// There are no allowed consumers of the offer.
			continue
		}
		out := in
		out.OfferUsers = users
		output = append(output, out)
	}

	return output, nil
}

func encodeInternalOfferFilter(
	filter OfferFilter,
) crossmodelrelation.OfferFilter {
	res := crossmodelrelation.OfferFilter{
		OfferName:              filter.OfferName,
		ApplicationName:        filter.ApplicationName,
		ApplicationDescription: filter.ApplicationDescription,
	}
	if len(filter.Endpoints) > 0 {
		res.Endpoints = transform.Slice(filter.Endpoints, encodeOfferFilterEndpoints)
	}
	return res
}

func encodeOfferFilterEndpoints(in EndpointFilterTerm) crossmodelrelation.EndpointFilterTerm {
	return crossmodelrelation.EndpointFilterTerm{
		Name:      in.Name,
		Interface: in.Interface,
		Role:      in.Role,
	}
}
