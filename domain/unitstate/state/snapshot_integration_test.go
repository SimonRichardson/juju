// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"testing"

	"github.com/juju/tc"

	coreunit "github.com/juju/juju/core/unit"
)

type snapshotIntegrationSuite struct {
	commitHookBaseSuite
}

func TestSnapshotIntegrationSuite(t *testing.T) {
	tc.Run(t, &snapshotIntegrationSuite{})
}

func (s *snapshotIntegrationSuite) TestGetUnitSnapshotWatchIdentifiers(c *tc.C) {
	var applicationUUID, charmUUID, netNodeUUID string
	err := s.DB().QueryRowContext(c.Context(), `
SELECT application_uuid, charm_uuid, net_node_uuid
FROM unit
WHERE uuid = ?`, s.unitUUID).Scan(&applicationUUID, &charmUUID, &netNodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	localCharmRelationUUID := s.addCharmRelationWithDefaults(c, charmUUID)
	localApplicationEndpointUUID := s.addApplicationEndpoint(c, applicationUUID, localCharmRelationUUID)
	remoteApplicationEndpointUUID := s.addApplicationEndpoint(
		c, s.fakeApplicationUUID1, s.fakeCharmRelationProvidesUUID,
	)
	relationUUID := s.addRelation(c)
	localRelationEndpointUUID := s.addRelationEndpoint(c, relationUUID, localApplicationEndpointUUID)
	remoteRelationEndpointUUID := s.addRelationEndpoint(c, relationUUID, remoteApplicationEndpointUUID)
	relationUnitUUID := s.addRelationUnit(c, coreunit.UUID(s.unitUUID), localRelationEndpointUUID)

	identifiers, err := s.state.GetUnitSnapshotWatchIdentifiers(
		c.Context(), coreunit.Name(s.unitName),
	)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(identifiers.UnitUUID, tc.Equals, s.unitUUID)
	c.Check(identifiers.ApplicationUUID, tc.Equals, applicationUUID)
	c.Check(identifiers.CharmUUID, tc.Equals, charmUUID)
	c.Check(identifiers.NetNodeUUIDs, tc.DeepEquals, []string{netNodeUUID})
	c.Check(identifiers.RelationUUIDs, tc.DeepEquals, []string{relationUUID.String()})
	c.Check(identifiers.RelationUnitUUIDs, tc.DeepEquals, []string{relationUnitUUID.String()})
	c.Check(identifiers.RelationEndpointUUIDs, tc.SameContents, []string{
		localRelationEndpointUUID,
		remoteRelationEndpointUUID,
	})
}
