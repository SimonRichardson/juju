// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package application_test

import (
	"context"
	"database/sql"

	"github.com/juju/tc"

	"github.com/juju/juju/core/changestream"
	"github.com/juju/juju/domain/application/service"
)

func (s *watcherSuite) TestGetUnitWatchIdentifiersIntegration(c *tc.C) {
	factory := changestream.NewWatchableDBFactoryForNamespace(s.GetWatchableDB, "unit")
	svc := s.setupService(c, factory)
	appUUID := s.createIAASApplication(c, svc, "foo", service.AddIAASUnitArg{})
	otherAppUUID := s.createIAASApplication(c, svc, "bar", service.AddIAASUnitArg{})

	var (
		unitUUID  string
		charmUUID string
	)
	const (
		relationUUID              = "relation-uuid"
		relationEndpointUUID      = "relation-endpoint-uuid"
		otherRelationEndpointUUID = "other-relation-endpoint-uuid"
		relationUnitUUID          = "relation-unit-uuid"
	)
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, "SELECT uuid, charm_uuid FROM unit WHERE name = ?", "foo/0").Scan(&unitUUID, &charmUUID); err != nil {
			return err
		}

		var endpointUUID, otherEndpointUUID string
		if err := tx.QueryRowContext(ctx, "SELECT uuid FROM application_endpoint WHERE application_uuid = ?", appUUID.String()).Scan(&endpointUUID); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, "SELECT uuid FROM application_endpoint WHERE application_uuid = ?", otherAppUUID.String()).Scan(&otherEndpointUUID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO relation (uuid, life_id, relation_id, scope_id)
VALUES (?, 0, 0, 0)`, relationUUID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO relation_endpoint (uuid, relation_uuid, endpoint_uuid)
VALUES (?, ?, ?)`, relationEndpointUUID, relationUUID, endpointUUID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO relation_endpoint (uuid, relation_uuid, endpoint_uuid)
VALUES (?, ?, ?)`, otherRelationEndpointUUID, relationUUID, otherEndpointUUID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO relation_unit (uuid, relation_endpoint_uuid, unit_uuid)
VALUES (?, ?, ?)`, relationUnitUUID, relationEndpointUUID, unitUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	identifiers, err := svc.GetUnitWatchIdentifiers(c.Context(), "foo/0")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(identifiers.UnitUUID, tc.Equals, unitUUID)
	c.Check(identifiers.ApplicationUUID, tc.Equals, appUUID.String())
	c.Check(identifiers.CharmUUID, tc.Equals, charmUUID)
	c.Check(identifiers.NetNodeUUIDs, tc.HasLen, 1)
	for _, identifier := range append(
		[]string{identifiers.UnitUUID, identifiers.ApplicationUUID, identifiers.CharmUUID},
		identifiers.NetNodeUUIDs...,
	) {
		c.Check(identifier != "", tc.IsTrue)
	}
	c.Check(identifiers.RelationUUIDs, tc.DeepEquals, []string{relationUUID})
	c.Check(identifiers.RelationUnitUUIDs, tc.DeepEquals, []string{relationUnitUUID})
	c.Check(identifiers.RelationEndpointUUIDs, tc.SameContents, []string{
		relationEndpointUUID,
		otherRelationEndpointUUID,
	})
}
