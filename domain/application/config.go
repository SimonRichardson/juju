// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package application

import (
	"strconv"

	"github.com/juju/juju/domain/application/charm"
	internalcharm "github.com/juju/juju/internal/charm"
	"github.com/juju/juju/internal/errors"
)

// TODO(dqlite) - we don't want to reference environs/config here but need a place for config key names.
const (
	// StorageDefaultBlockSourceKey is the key for the default block storage source.
	StorageDefaultBlockSourceKey = "storage-default-block-source"

	// StorageDefaultFilesystemSourceKey is the key for the default filesystem storage source.
	StorageDefaultFilesystemSourceKey = "storage-default-filesystem-source"
)

// DecodeConfig decodes the domain charm config representation into the internal
// charm config representation.
func DecodeConfig(options charm.Config) (internalcharm.ConfigSpec, error) {
	if len(options.Options) == 0 {
		return internalcharm.ConfigSpec{}, nil
	}

	result := make(map[string]internalcharm.Option)
	for name, option := range options.Options {
		opt, err := decodeConfigOption(option)
		if err != nil {
			return internalcharm.ConfigSpec{}, errors.Errorf("decode config option: %w", err)
		}

		result[name] = opt
	}
	return internalcharm.ConfigSpec{
		Options: result,
	}, nil
}

func decodeConfigOption(option charm.Option) (internalcharm.Option, error) {
	t, err := decodeOptionType(option.Type)
	if err != nil {
		return internalcharm.Option{}, errors.Errorf("decode option type: %w", err)
	}

	return internalcharm.Option{
		Type:        t,
		Description: option.Description,
		Default:     option.Default,
	}, nil
}

func decodeOptionType(t charm.OptionType) (string, error) {
	switch t {
	case charm.OptionString:
		return "string", nil
	case charm.OptionInt:
		return "int", nil
	case charm.OptionFloat:
		return "float", nil
	case charm.OptionBool:
		return "boolean", nil
	case charm.OptionSecret:
		return "secret", nil
	default:
		return "", errors.Errorf("unknown option type %q", t)
	}
}

// EncodeConfig encodes the internal charm config representation into the domain
// charm config representation.
func EncodeConfig(config *internalcharm.ConfigSpec) (charm.Config, error) {
	if config == nil || len(config.Options) == 0 {
		return charm.Config{}, nil
	}

	result := make(map[string]charm.Option)
	for name, option := range config.Options {
		opt, err := encodeConfigOption(option)
		if err != nil {
			return charm.Config{}, errors.Errorf("encode config option: %w", err)
		}

		result[name] = opt
	}
	return charm.Config{
		Options: result,
	}, nil
}

func encodeConfigOption(option internalcharm.Option) (charm.Option, error) {
	t, err := encodeOptionType(option.Type)
	if err != nil {
		return charm.Option{}, errors.Errorf("encode option type: %w", err)
	}

	return charm.Option{
		Type:        t,
		Description: option.Description,
		Default:     option.Default,
	}, nil
}

func encodeOptionType(t string) (charm.OptionType, error) {
	switch t {
	case "string":
		return charm.OptionString, nil
	case "int":
		return charm.OptionInt, nil
	case "float":
		return charm.OptionFloat, nil
	case "boolean":
		return charm.OptionBool, nil
	case "secret":
		return charm.OptionSecret, nil
	default:
		return "", errors.Errorf("unknown option type %q", t)
	}
}

// DecodeApplicationConfig decodes the application config from the domain
// representation into the internal charm config representation.
func DecodeApplicationConfig(cfg map[string]ApplicationConfig) (internalcharm.Config, error) {
	if len(cfg) == 0 {
		return nil, nil
	}

	result := make(internalcharm.Config)
	for k, v := range cfg {
		if v.Value == nil {
			result[k] = nil
			continue
		}
		coercedV, err := coerceValue(v.Type, *v.Value)
		if err != nil {
			return nil, errors.Errorf("coerce config value for key %q: %w", k, err)
		}
		result[k] = coercedV
	}
	return result, nil
}

func coerceValue(t charm.OptionType, value string) (interface{}, error) {
	switch t {
	case charm.OptionString, charm.OptionSecret:
		return value, nil
	case charm.OptionInt:
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return nil, errors.Errorf("cannot convert string %q to int: %w", value, err)
		}
		return intValue, nil
	case charm.OptionFloat:
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, errors.Errorf("cannot convert string %q to float: %w", value, err)
		}
		return floatValue, nil
	case charm.OptionBool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return nil, errors.Errorf("cannot convert string %q to bool: %w", value, err)
		}
		return boolValue, nil
	default:
		return nil, errors.Errorf("unknown config type %q", t)
	}

}
