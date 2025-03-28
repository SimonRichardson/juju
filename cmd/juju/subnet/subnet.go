// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package subnet

import (
	"context"
	"io"
	"net"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/subnets"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/internal/cmd"
	internallogger "github.com/juju/juju/internal/logger"
	"github.com/juju/juju/rpc/params"
)

// SubnetAPI defines the necessary API methods needed by the subnet
// subcommands.
type SubnetAPI interface {
	io.Closer

	// AddSubnet adds an existing subnet to Juju.
	AddSubnet(ctx context.Context, cidr string, id network.Id, spaceTag names.SpaceTag, zones []string) error

	// ListSubnets returns information about subnets known to Juju,
	// optionally filtered by space and/or zone (both can be empty).
	ListSubnets(ctx context.Context, withSpace *names.SpaceTag, withZone string) ([]params.Subnet, error)
}

// mvpAPIShim forwards SubnetAPI methods to the real API facade for
// implemented methods only. Tested with a feature test only.
type mvpAPIShim struct {
	SubnetAPI

	apiState api.Connection
	facade   *subnets.API
}

func (m *mvpAPIShim) Close() error {
	return m.apiState.Close()
}

func (m *mvpAPIShim) ListSubnets(ctx context.Context, withSpace *names.SpaceTag, withZone string) ([]params.Subnet, error) {
	return m.facade.ListSubnets(ctx, withSpace, withZone)
}

var logger = internallogger.GetLogger("juju.cmd.juju.subnet")

// SubnetCommandBase is the base type embedded into all subnet
// subcommands.
type SubnetCommandBase struct {
	modelcmd.ModelCommandBase
	modelcmd.IAASOnlyCommand
	api SubnetAPI
}

// NewAPI returns a SubnetAPI for the root api endpoint that the
// environment command returns.
func (c *SubnetCommandBase) NewAPI(ctx context.Context) (SubnetAPI, error) {
	if c.api != nil {
		// Already created.
		return c.api, nil
	}
	root, err := c.NewAPIRoot(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// This is tested with a feature test.
	shim := &mvpAPIShim{
		apiState: root,
		facade:   subnets.NewAPI(root),
	}
	return shim, nil
}

type RunOnAPI func(api SubnetAPI, ctx *cmd.Context) error

func (c *SubnetCommandBase) RunWithAPI(ctx *cmd.Context, toRun RunOnAPI) error {
	api, err := c.NewAPI(ctx)
	if err != nil {
		return errors.Annotate(err, "cannot connect to the API server")
	}
	defer api.Close()
	return toRun(api, ctx)
}

// CheckNumArgs is a helper used to validate the number of arguments
// passed to Init(). If the number of arguments is X, errors[X] (if
// set) will be returned, otherwise no error occurs.
func (s *SubnetCommandBase) CheckNumArgs(args []string, errors []error) error {
	for num, err := range errors {
		if len(args) == num {
			return err
		}
	}
	return nil
}

// ValidateCIDR parses given and returns an error if it's not valid.
// If the CIDR is incorrectly specified (e.g. 10.10.10.0/16 instead of
// 10.10.0.0/16) and strict is false, the correctly parsed CIDR in the
// expected format is returned instead without an error. Otherwise,
// when strict is true and given is incorrectly formatted, an error
// will be returned.
func (s *SubnetCommandBase) ValidateCIDR(given string, strict bool) (string, error) {
	_, ipNet, err := net.ParseCIDR(given)
	if err != nil {
		logger.Debugf(context.TODO(), "cannot parse CIDR %q: %v", given, err)
		return "", errors.Errorf("%q is not a valid CIDR", given)
	}
	if strict && given != ipNet.String() {
		expected := ipNet.String()
		return "", errors.Errorf("%q is not correctly specified, expected %q", given, expected)
	}
	// Already validated, so shouldn't error here.
	return ipNet.String(), nil
}

// ValidateSpace parses given and returns an error if it's not a valid
// space name, otherwise returns the parsed tag and no error.
func (s *SubnetCommandBase) ValidateSpace(given string) (names.SpaceTag, error) {
	if !names.IsValidSpace(given) {
		return names.SpaceTag{}, errors.Errorf("%q is not a valid space name", given)
	}
	return names.NewSpaceTag(given), nil
}
