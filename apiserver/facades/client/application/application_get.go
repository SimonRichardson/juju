// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package application

import (
	"context"

	"github.com/juju/errors"
	"github.com/juju/schema"

	coreapplication "github.com/juju/juju/core/application"
	corecharm "github.com/juju/juju/core/charm"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/network"
	applicationcharm "github.com/juju/juju/domain/application/charm"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	"github.com/juju/juju/internal/charm"
	"github.com/juju/juju/internal/configschema"
	"github.com/juju/juju/rpc/params"
)

// Get returns the charm configuration for an application.
func (api *APIBase) getConfig(
	ctx context.Context,
	args params.ApplicationGet,
	describe func(applicationConfig charm.Config, charmConfig charm.ConfigSpec) map[string]interface{},
) (params.ApplicationGetResults, error) {
	// TODO (stickupkid): This should be one call to the application service.
	// There is no reason to split all these calls into multiple DB calls.
	// Once application service is refactored to return the merged config, this
	// should be a single call.

	appID, err := api.applicationService.GetApplicationUUIDByName(ctx, args.ApplicationName)
	if errors.Is(err, applicationerrors.ApplicationNotFound) {
		return params.ApplicationGetResults{}, errors.NotFoundf("application %s", args.ApplicationName)
	} else if err != nil {
		return params.ApplicationGetResults{}, errors.Trace(err)
	}

	appInfo, err := api.applicationService.GetApplicationAndCharmConfig(ctx, appID)
	if errors.Is(err, applicationerrors.ApplicationNotFound) {
		return params.ApplicationGetResults{}, errors.NotFoundf("application %s", args.ApplicationName)
	} else if err != nil {
		return params.ApplicationGetResults{}, errors.Trace(err)
	}
	mergedCharmConfig := describe(appInfo.ApplicationConfig, appInfo.CharmConfig)

	appSettings := map[string]interface{}{
		coreapplication.TrustConfigOptionName: appInfo.Trust,
	}
	providerSchema, providerDefaults, err := ConfigSchema()
	if err != nil {
		return params.ApplicationGetResults{}, err
	}
	appConfigInfo := describeAppSettings(appSettings, providerSchema, providerDefaults)

	isSubordinate, err := api.applicationService.IsSubordinateApplication(ctx, appID)
	if errors.Is(err, applicationerrors.ApplicationNotFound) {
		return params.ApplicationGetResults{}, errors.NotFoundf("application %s", args.ApplicationName)
	} else if err != nil {
		return params.ApplicationGetResults{}, errors.Trace(err)
	}
	var cons constraints.Value
	if !isSubordinate {
		cons, err = api.applicationService.GetApplicationConstraints(ctx, appID)
		if errors.Is(err, applicationerrors.ApplicationNotFound) {
			return params.ApplicationGetResults{}, errors.NotFoundf("application %s", args.ApplicationName)
		} else if err != nil {
			return params.ApplicationGetResults{}, errors.Trace(err)
		}
	}

	bindings, err := api.applicationService.GetApplicationEndpointBindings(ctx, args.ApplicationName)
	if errors.Is(err, applicationerrors.ApplicationNotFound) {
		return params.ApplicationGetResults{}, errors.NotFoundf("application %q", args.ApplicationName)
	} else if err != nil {
		return params.ApplicationGetResults{}, errors.Trace(err)
	}

	allSpaceInfosLookup, err := api.networkService.GetAllSpaces(ctx)
	if err != nil {
		return params.ApplicationGetResults{}, errors.Trace(err)
	}

	bindingMap, err := network.MapBindingsWithSpaceNames(bindings, allSpaceInfosLookup)
	if err != nil {
		return params.ApplicationGetResults{}, err
	}

	origin, err := api.applicationService.GetApplicationCharmOrigin(ctx, args.ApplicationName)
	if errors.Is(err, applicationerrors.ApplicationNotFound) {
		return params.ApplicationGetResults{}, errors.NotFoundf("application %s", args.ApplicationName)
	} else if err != nil {
		return params.ApplicationGetResults{}, errors.Trace(err)
	}

	// If the applications charm origin is from charm-hub, then build the real
	// channel and send that back.
	var appChannel string
	if corecharm.CharmHub.Matches(origin.Source.String()) && origin.Channel != nil {
		ch := origin.Channel
		appChannel = charm.MakePermissiveChannel(ch.Track, string(ch.Risk), ch.Branch).String()
	}
	osType, err := encodeOSType(origin.Platform.OS)
	if err != nil {
		return params.ApplicationGetResults{}, errors.Trace(err)
	}
	return params.ApplicationGetResults{
		Application:       args.ApplicationName,
		Charm:             appInfo.CharmName,
		CharmConfig:       mergedCharmConfig,
		ApplicationConfig: appConfigInfo,
		Constraints:       cons,
		Base: params.Base{
			Name:    osType,
			Channel: origin.Platform.Channel,
		},
		Channel:          appChannel,
		EndpointBindings: bindingMap,
	}, nil
}

func (api *APIBase) getCharmLocatorByApplicationName(ctx context.Context, name string) (applicationcharm.CharmLocator, error) {
	locator, err := api.applicationService.GetCharmLocatorByApplicationName(ctx, name)
	if errors.Is(err, applicationerrors.ApplicationNotFound) {
		return applicationcharm.CharmLocator{}, errors.NotFoundf("application %q", name)
	} else if errors.Is(err, applicationerrors.CharmNotFound) {
		return applicationcharm.CharmLocator{}, errors.NotFoundf("charm for application %q", name)
	} else if err != nil {
		return applicationcharm.CharmLocator{}, errors.Annotate(err, "getting charm id for application")
	}
	return locator, nil
}

func (api *APIBase) getCharmName(ctx context.Context, locator applicationcharm.CharmLocator) (string, error) {
	name, err := api.applicationService.GetCharmMetadataName(ctx, locator)
	if errors.Is(err, applicationerrors.CharmNotFound) {
		return "", errors.NotFoundf("charm")
	} else if err != nil {
		return "", errors.Annotate(err, "getting charm for application")
	}
	return name, nil
}

func (api *APIBase) getCharm(ctx context.Context, locator applicationcharm.CharmLocator) (*domainCharm, error) {
	charm, resLocator, _, err := api.applicationService.GetCharm(ctx, locator)
	if errors.Is(err, applicationerrors.CharmNotFound) {
		return nil, errors.NotFoundf("charm %q", locator.Name)
	} else if errors.Is(err, applicationerrors.CharmNameNotValid) {
		return nil, errors.NotValidf("charm %q", locator.Name)
	} else if errors.Is(err, applicationerrors.CharmSourceNotValid) {
		return nil, errors.NotValidf("charm %q", locator.Name)
	} else if err != nil {
		return nil, errors.Annotate(err, "getting charm for application")
	}

	available, err := api.applicationService.IsCharmAvailable(ctx, resLocator)
	if errors.Is(err, applicationerrors.CharmNotFound) {
		return nil, errors.NotFoundf("charm")
	} else if err != nil {
		return nil, errors.Annotate(err, "getting charm availability for application")
	}

	return &domainCharm{
		charm:     charm,
		locator:   resLocator,
		available: available,
	}, nil
}

func (api *APIBase) getMergedAppAndCharmConfig(ctx context.Context, appName string) (map[string]interface{}, error) {
	// TODO (stickupkid): This should be one call to the application service.
	// Thee application service should return the merged config, this should
	// not happen at the API server level.
	appID, err := api.applicationService.GetApplicationUUIDByName(ctx, appName)
	if errors.Is(err, applicationerrors.ApplicationNotFound) {
		return nil, errors.NotFoundf("application %s", appName)
	} else if err != nil {
		return nil, errors.Trace(err)
	}

	appInfo, err := api.applicationService.GetApplicationAndCharmConfig(ctx, appID)
	if errors.Is(err, applicationerrors.ApplicationNotFound) {
		return nil, errors.NotFoundf("application %s", appName)
	} else if err != nil {
		return nil, errors.Trace(err)
	}

	return describe(appInfo.ApplicationConfig, appInfo.CharmConfig), nil
}

func describeAppSettings(
	appConfig map[string]interface{},
	schemaFields configschema.Fields,
	defaults schema.Defaults,
) map[string]interface{} {
	results := make(map[string]interface{})
	for name, field := range schemaFields {
		defaultValue := defaults[name]
		info := map[string]interface{}{
			"description": field.Description,
			"type":        field.Type,
			"source":      "unset",
		}
		if defaultValue == schema.Omit {
			results[name] = info
			continue
		}
		set := false
		if value := appConfig[name]; value != nil && defaultValue != value {
			set = true
			info["value"] = value
			info["source"] = "user"
		}
		if defaultValue != nil {
			info["default"] = defaultValue
			if !set {
				info["value"] = defaultValue
				info["source"] = "default"
			}
		}
		results[name] = info
	}
	return results
}

func describe(settings charm.Config, config charm.ConfigSpec) map[string]interface{} {
	results := make(map[string]interface{})
	for name, option := range config.Options {
		info := map[string]interface{}{
			"description": option.Description,
			"type":        option.Type,
			"source":      "unset",
		}
		set := false
		if value := settings[name]; value != nil && option.Default != value {
			set = true
			info["value"] = value
			info["source"] = "user"
		}
		if option.Default != nil {
			info["default"] = option.Default
			if !set {
				info["value"] = option.Default
				info["source"] = "default"
			}
		}
		results[name] = info
	}
	return results
}

type domainCharm struct {
	charm     charm.Charm
	locator   applicationcharm.CharmLocator
	available bool
}

func (c *domainCharm) Manifest() *charm.Manifest {
	return c.charm.Manifest()
}

func (c *domainCharm) Meta() *charm.Meta {
	return c.charm.Meta()
}

func (c *domainCharm) Config() *charm.ConfigSpec {
	return c.charm.Config()
}

func (c *domainCharm) Actions() *charm.Actions {
	return c.charm.Actions()
}

func (c *domainCharm) Revision() int {
	return c.locator.Revision
}

func (c *domainCharm) IsUploaded() bool {
	return c.available
}

func (c *domainCharm) Version() string {
	return c.charm.Version()
}

func ptr[T any](v T) *T {
	return &v
}
