// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package crossmodel

import (
	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/core/network"
)

// ControllerInfo holds the details required to connect to a controller.
type ControllerInfo struct {
	// ControllerUUID holds the UUID of the controller.
	ControllerUUID string

	// Alias holds a (human friendly) alias for the controller.
	Alias string

	// Addrs holds the addresses and ports of the controller's API servers.
	Addrs []string

	// CACert holds the CA certificate that will be used to validate
	// the API server's certificate, in PEM format.
	CACert string

	// ModelUUIDs holds the UUIDs of the models hosted on this controller.
	ModelUUIDs []string
}

// Validate returns an error if the ControllerInfo contains bad data.
func (info *ControllerInfo) Validate() error {
	if !names.IsValidController(info.ControllerUUID) {
		return errors.NotValidf("ControllerTag")
	}

	if len(info.Addrs) < 1 {
		return errors.NotValidf("empty controller api addresses")
	}
	for _, addr := range info.Addrs {
		_, err := network.ParseMachineHostPort(addr)
		if err != nil {
			return errors.NotValidf("controller api address %q", addr)
		}
	}
	return nil
}
