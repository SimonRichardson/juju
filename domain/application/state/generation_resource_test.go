// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"

	"github.com/juju/tc"

	coreresource "github.com/juju/juju/core/resource"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/application"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	generationerrors "github.com/juju/juju/domain/generation/errors"
)

func (s *applicationRefreshSuite) createResourcePair(
	c *tc.C, appUUID string, name string,
) (coreresource.UUID, coreresource.UUID) {
	mainUUID := tc.Must(c, coreresource.NewUUID)
	branchUUID := tc.Must(c, coreresource.NewUUID)
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		var charmUUID string
		if err := tx.QueryRowContext(ctx, `SELECT charm_uuid FROM application WHERE uuid = ?`, appUUID).Scan(&charmUUID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO charm_resource (charm_uuid, name, kind_id)
VALUES (?, ?, 0)
`, charmUUID, name); err != nil {
			return err
		}
		for revision, resourceUUID := range []coreresource.UUID{mainUUID, branchUUID} {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO resource (
    uuid, charm_uuid, charm_resource_name, revision,
    origin_type_id, state_id, created_at
)
VALUES (?, ?, ?, ?, 1, 0, DATETIME('now', 'utc'))
`, resourceUUID.String(), charmUUID, name, revision); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO application_resource (resource_uuid, application_uuid)
VALUES (?, ?)
`, mainUUID.String(), appUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	return mainUUID, branchUUID
}

func (s *applicationRefreshSuite) TestGenerationResourcesResolveSelectively(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	mainWebsite, branchWebsite := s.createResourcePair(c, appUUID.String(), "website")
	mainTheme, branchTheme := s.createResourcePair(c, appUUID.String(), "theme")
	potentialWebsite := tc.Must(c, coreresource.NewUUID)
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		var charmUUID string
		if err := tx.QueryRowContext(ctx, `SELECT charm_uuid FROM application WHERE uuid = ?`, appUUID.String()).Scan(&charmUUID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO resource (
    uuid, charm_uuid, charm_resource_name, revision,
    origin_type_id, state_id, created_at
)
VALUES (?, ?, 'website', 2, 1, 1, DATETIME('now', 'utc'))
`, potentialWebsite.String(), charmUUID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO application_resource (resource_uuid, application_uuid)
VALUES (?, ?)
`, potentialWebsite.String(), appUUID.String())
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	trackedUnit := s.addUnit(c, coreunit.Name("mediawiki/0"), appUUID)
	mainUnit := s.addUnit(c, coreunit.Name("mediawiki/1"), appUUID)
	generationUUID := s.createGeneration(c, "test", 0)
	s.trackUnit(c, generationUUID, trackedUnit)

	charmID, err := s.state.GetCharmIDByApplicationName(c.Context(), "mediawiki")
	c.Assert(err, tc.ErrorIsNil)
	err = s.state.SetApplicationCharm(c.Context(), appUUID, charmID, application.SetCharmStateParams{
		Resources: []application.ResourceSelection{
			{Name: "website", ResourceUUID: branchWebsite},
			{Name: "theme", ResourceUUID: branchTheme},
		},
	})
	c.Assert(err, tc.ErrorIsNil)

	for _, test := range []struct {
		unit coreunit.UUID
		name string
		want coreresource.UUID
	}{
		{trackedUnit, "website", branchWebsite},
		{trackedUnit, "theme", branchTheme},
		{mainUnit, "website", mainWebsite},
		{mainUnit, "theme", mainTheme},
	} {
		got, err := s.state.GetResolvedUnitResource(c.Context(), test.unit, test.name)
		c.Assert(err, tc.ErrorIsNil)
		c.Check(got, tc.Equals, test.want)
	}

	var website, theme string
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT r.charm_resource_name, ar.resource_uuid
FROM application_resource AS ar
JOIN resource AS r ON r.uuid = ar.resource_uuid
JOIN resource_state AS rs ON rs.id = r.state_id
WHERE ar.application_uuid = ?
AND rs.name = 'available'
`, appUUID.String())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name, resourceUUID string
			if err := rows.Scan(&name, &resourceUUID); err != nil {
				return err
			}
			switch name {
			case "website":
				website = resourceUUID
			case "theme":
				theme = resourceUUID
			}
		}
		return rows.Err()
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(website, tc.Equals, mainWebsite.String())
	c.Check(theme, tc.Equals, mainTheme.String())
}

func (s *applicationRefreshSuite) TestGenerationResourcesRollbackInvalidSelection(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	_, branchWebsite := s.createResourcePair(c, appUUID.String(), "website")
	branchCharm := s.createCharm(c, createCharmArgs{name: "mediawiki-branch"})
	s.createGeneration(c, "test", 0)

	err := s.state.SetApplicationCharm(c.Context(), appUUID, branchCharm, application.SetCharmStateParams{
		Resources: []application.ResourceSelection{
			{Name: "website", ResourceUUID: branchWebsite},
			{Name: "wrong-name", ResourceUUID: branchWebsite},
		},
	})
	c.Check(err, tc.ErrorIs, applicationerrors.InvalidResourceArgs)

	var resourceCount, charmCount int
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM generation_application_resource`).Scan(&resourceCount); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM generation_application_charm`).Scan(&charmCount)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(resourceCount, tc.Equals, 0)
	c.Check(charmCount, tc.Equals, 0)
}

func (s *applicationRefreshSuite) TestGenerationResourcesBranchAndUnitErrors(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	_, branchWebsite := s.createResourcePair(c, appUUID.String(), "website")
	s.createGeneration(c, "committed", 1)
	selection := []application.ResourceSelection{{Name: "website", ResourceUUID: branchWebsite}}

	for _, branchName := range []string{"missing", "committed"} {
		err := s.state.setGenerationApplicationResources(c.Context(), branchName, appUUID, selection)
		c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
	}
	_, err := s.state.GetResolvedUnitResource(c.Context(), tc.Must(c, coreunit.NewUUID), "website")
	c.Check(err, tc.ErrorIs, applicationerrors.UnitNotFound)
}
