// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package maas

import (
	"context"

	"github.com/juju/schema"

	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/internal/configschema"
)

var configSchema = configschema.Fields{}

var configFields = func() schema.Fields {
	fs, _, err := configSchema.ValidationSchema()
	if err != nil {
		panic(err)
	}
	return fs
}()

var configDefaults = schema.Defaults{}

type maasModelConfig struct {
	*config.Config
	attrs map[string]interface{}
}

func (p EnvironProvider) newConfig(ctx context.Context, cfg *config.Config) (*maasModelConfig, error) {
	validCfg, err := p.Validate(ctx, cfg, nil)
	if err != nil {
		return nil, err
	}
	result := new(maasModelConfig)
	result.Config = validCfg
	result.attrs = validCfg.UnknownAttrs()
	return result, nil
}

// Schema returns the configuration schema for an environment.
func (EnvironProvider) Schema() configschema.Fields {
	fields, err := config.Schema(configSchema)
	if err != nil {
		panic(err)
	}
	return fields
}

// ModelConfigDefaults provides a set of default model config attributes that
// should be set on a models config if they have not been specified by the user.
func (p EnvironProvider) ModelConfigDefaults(_ context.Context) (map[string]any, error) {
	return map[string]any{}, nil
}

// ConfigSchema returns extra config attributes specific
// to this provider only.
func (p EnvironProvider) ConfigSchema() schema.Fields {
	return configFields
}

// ConfigDefaults returns the default values for the
// provider specific config attributes.
func (p EnvironProvider) ConfigDefaults() schema.Defaults {
	return configDefaults
}

func (p EnvironProvider) Validate(ctx context.Context, cfg, oldCfg *config.Config) (*config.Config, error) {
	// Validate base configuration change before validating MAAS specifics.
	err := config.Validate(ctx, cfg, oldCfg)
	if err != nil {
		return nil, err
	}
	validated, err := cfg.ValidateUnknownAttrs(configFields, configDefaults)
	if err != nil {
		return nil, err
	}
	envCfg := &maasModelConfig{
		Config: cfg,
		attrs:  validated,
	}
	return cfg.Apply(envCfg.attrs)
}
