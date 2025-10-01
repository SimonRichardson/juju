// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"testing"

	"github.com/juju/clock"
	"github.com/juju/tc"

	coreapplication "github.com/juju/juju/core/application"
	machinetesting "github.com/juju/juju/core/machine/testing"
	"github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/application"
	"github.com/juju/juju/domain/application/architecture"
	"github.com/juju/juju/domain/application/charm"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	machineerrors "github.com/juju/juju/domain/machine/errors"
	domainnetwork "github.com/juju/juju/domain/network"
	schematesting "github.com/juju/juju/domain/schema/testing"
	loggertesting "github.com/juju/juju/internal/logger/testing"
)

type machinePlacementSuite struct {
	schematesting.ModelSuite

	state *State
}

func TestMachinePlacementSuite(t *testing.T) {
	tc.Run(t, &machinePlacementSuite{})
}

func (s *machinePlacementSuite) SetUpTest(c *tc.C) {
	s.ModelSuite.SetUpTest(c)

	s.state = NewState(s.TxnRunnerFactory(), clock.WallClock, loggertesting.WrapCheckLog(c))
}

func (s *machinePlacementSuite) TestIsMachineControllerApplicationController(c *tc.C) {
	s.createApplication(c, true)

	st := NewState(s.TxnRunnerFactory(), clock.WallClock, loggertesting.WrapCheckLog(c))

	machineName := s.createMachine(c)

	isController, err := st.IsMachineController(c.Context(), machineName)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(isController, tc.IsTrue)
}

func (s *machinePlacementSuite) TestIsMachineControllerApplicationNonController(c *tc.C) {
	s.createApplication(c, false)

	st := NewState(s.TxnRunnerFactory(), clock.WallClock, loggertesting.WrapCheckLog(c))

	machineName := s.createMachine(c)

	isController, err := st.IsMachineController(c.Context(), machineName)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(isController, tc.IsFalse)
}

func (s *machinePlacementSuite) TestIsMachineControllerFailure(c *tc.C) {
	s.createApplication(c, false)

	st := NewState(s.TxnRunnerFactory(), clock.WallClock, loggertesting.WrapCheckLog(c))

	machineName := s.createMachine(c)

	isController, err := st.IsMachineController(c.Context(), machineName)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(isController, tc.IsFalse)
}

// TestIsMachineControllerNotFound asserts that a NotFound error is returned when the
// machine is not found.
func (s *machinePlacementSuite) TestIsMachineControllerNotFound(c *tc.C) {
	st := NewState(s.TxnRunnerFactory(), clock.WallClock, loggertesting.WrapCheckLog(c))

	_, err := st.IsMachineController(c.Context(), "666")
	c.Assert(err, tc.ErrorIs, machineerrors.MachineNotFound)
}

func (s *machinePlacementSuite) TestGetMachinesForApplicationEmptyApp(c *tc.C) {
	// Arrange
	appUUID := s.createApplication(c, true)

	// Act
	machines, err := s.state.GetMachinesForApplication(c.Context(), appUUID.String())

	// Assert
	c.Assert(err, tc.ErrorIsNil)
	c.Check(machines, tc.HasLen, 0)
}

func (s *machinePlacementSuite) TestGetMachinesForApplication(c *tc.C) {
	// Arrange
	appUUID := s.createApplication(c, true)
	machineName0 := s.createMachine(c)
	machineName1 := s.createMachine(c)

	// Act
	machines, err := s.state.GetMachinesForApplication(c.Context(), appUUID.String())

	// Assert
	c.Assert(err, tc.ErrorIsNil)
	c.Check(machines, tc.SameContents, []string{machineName0, machineName1})
}

func (s *machinePlacementSuite) TestGetMachinesForApplicationNotFound(c *tc.C) {
	// Act
	_, err := s.state.GetMachinesForApplication(c.Context(), "666")

	// Assert
	c.Assert(err, tc.ErrorIs, applicationerrors.ApplicationNotFound)
}

func (s *machinePlacementSuite) createApplication(c *tc.C, controller bool) coreapplication.UUID {
	appID, _, err := s.state.CreateIAASApplication(c.Context(), "foo", application.AddIAASApplicationArg{
		BaseAddApplicationArg: application.BaseAddApplicationArg{
			Charm: charm.Charm{
				Metadata: charm.Metadata{
					Name: "foo",
				},
				Manifest: charm.Manifest{
					Bases: []charm.Base{{
						Name:          "ubuntu",
						Channel:       charm.Channel{Risk: charm.RiskStable},
						Architectures: []string{"amd64"},
					}},
				},
				ReferenceName: "foo",
				Architecture:  architecture.AMD64,
				Revision:      1,
				Source:        charm.LocalSource,
			},
			IsController: controller,
		},
	}, nil)
	c.Assert(err, tc.ErrorIsNil)
	return appID
}

func (s *machinePlacementSuite) createUnit(c *tc.C) unit.Name {
	appID, err := s.state.GetApplicationUUIDByName(c.Context(), "foo")
	c.Assert(err, tc.ErrorIsNil)

	netNodeUUID := tc.Must(c, domainnetwork.NewNetNodeUUID)
	unitNames, _, err := s.state.AddIAASUnits(c.Context(), appID, application.AddIAASUnitArg{
		AddUnitArg: application.AddUnitArg{
			NetNodeUUID: netNodeUUID,
		},
		MachineNetNodeUUID: netNodeUUID,
		MachineUUID:        machinetesting.GenUUID(c),
		Nonce:              ptr("foo"),
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(unitNames, tc.HasLen, 1)
	unitName := unitNames[0]

	return unitName
}

func (s *machinePlacementSuite) createMachine(c *tc.C) string {
	unitName := s.createUnit(c)

	var machineName string
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
SELECT m.name
FROM machine m
JOIN net_node nn ON m.net_node_uuid = nn.uuid
JOIN unit u ON u.net_node_uuid = nn.uuid
WHERE u.name = ?
`, unitName).Scan(&machineName)
		if err != nil {
			return err
		}

		return nil
	})
	c.Assert(err, tc.ErrorIsNil)

	return machineName
}
