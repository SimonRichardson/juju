// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"

	"github.com/juju/collections/set"

	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/network/firewall"
	"github.com/juju/juju/core/trace"
	"github.com/juju/juju/domain/application"
	"github.com/juju/juju/internal/errors"
)

// IsApplicationExposed returns whether the provided application is exposed or not.
//
// If no application is found, an error satisfying
// [applicationerrors.ApplicationNotFound] is returned.
func (s *Service) IsApplicationExposed(ctx context.Context, appName string) (bool, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	appID, err := s.st.GetApplicationUUIDByName(ctx, appName)
	if err != nil {
		return false, errors.Capture(err)
	}

	return s.st.IsApplicationExposed(ctx, appID)
}

// GetExposedEndpoints returns map where keys are endpoint names (or the ""
// value which represents all endpoints) and values are ExposedEndpoint
// instances that specify which sources (spaces or CIDRs) can access the
// opened ports for each endpoint once the application is exposed.
//
// If no application is found, an error satisfying
// [applicationerrors.ApplicationNotFound] is returned.
func (s *Service) GetExposedEndpoints(ctx context.Context, appName string) (map[string]application.ExposedEndpoint, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	appID, err := s.st.GetApplicationUUIDByName(ctx, appName)
	if err != nil {
		return nil, errors.Capture(err)
	}

	return s.st.GetExposedEndpoints(ctx, appID)
}

// UnsetExposeSettings removes the expose settings for the provided list of
// endpoint names. If the resulting exposed endpoints map for the application
// becomes empty after the settings are removed, the application will be
// automatically unexposed.
//
// If no application is found, an error satisfying
// [applicationerrors.ApplicationNotFound] is returned.
func (s *Service) UnsetExposeSettings(ctx context.Context, appName string, exposedEndpoints set.Strings) error {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	appID, err := s.st.GetApplicationUUIDByName(ctx, appName)
	if err != nil {
		return errors.Capture(err)
	}

	return s.st.UnsetExposeSettings(ctx, appID, exposedEndpoints)
}

// MergeExposeSettings marks the application as exposed and merges the provided
// ExposedEndpoint details into the current set of expose settings. The merge
// operation will overwrite expose settings for each existing endpoint name.
//
// If no application is found, an error satisfying
// [applicationerrors.ApplicationNotFound] is returned.
func (s *Service) MergeExposeSettings(ctx context.Context, appName string, exposedEndpoints map[string]application.ExposedEndpoint) error {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	appID, err := s.st.GetApplicationUUIDByName(ctx, appName)
	if err != nil {
		return errors.Capture(err)
	}

	// First check that the endpoints actually exist.
	endpointNames := set.NewStrings()
	for endpoint := range exposedEndpoints {
		if endpoint != network.WildcardEndpoint {
			endpointNames.Add(endpoint)
		}
	}
	if err := s.st.EndpointsExist(ctx, appID, endpointNames); err != nil {
		return errors.Capture(err)
	}
	// Then we need to make sure that the spaces that endpoints are exposed
	// to (if any) actually exist.
	spaceUUIDStr := make([]string, 0)
	for _, exposedEndpoint := range exposedEndpoints {
		spaceUUIDStr = append(spaceUUIDStr, exposedEndpoint.ExposeToSpaceIDs.Values()...)
	}
	if err := s.st.SpacesExist(ctx, set.NewStrings(spaceUUIDStr...)); err != nil {
		return errors.Errorf("validating exposed endpoints to spaces %+v: %w", set.NewStrings(spaceUUIDStr...).Values(), err)
	}

	validatedExposedEndpoints := make(map[string]application.ExposedEndpoint)
	if len(exposedEndpoints) == 0 {
		// If an empty exposedEndpoints list is provided, all endpoints should
		// be exposed.
		validatedExposedEndpoints[network.WildcardEndpoint] = application.ExposedEndpoint{
			ExposeToCIDRs: set.NewStrings(firewall.AllNetworksIPV4CIDR, firewall.AllNetworksIPV6CIDR),
		}
	} else {
		for endpoint, exposedEndpoint := range exposedEndpoints {
			// If no spaces and CIDRs are provided, assume an implicit
			// 0.0.0.0/0 CIDR. This matches the "expose to the entire
			// world" behavior in juju controllers prior to 2.9.
			if len(exposedEndpoint.ExposeToSpaceIDs)+len(exposedEndpoint.ExposeToCIDRs) == 0 {
				exposedEndpoint.ExposeToCIDRs = set.NewStrings(firewall.AllNetworksIPV4CIDR, firewall.AllNetworksIPV6CIDR)
			}

			validatedExposedEndpoints[endpoint] = exposedEndpoint
		}
	}

	return s.st.MergeExposeSettings(ctx, appID, validatedExposedEndpoints)
}
