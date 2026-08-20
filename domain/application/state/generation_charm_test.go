// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"

	"github.com/juju/tc"

	coreapplication "github.com/juju/juju/core/application"
	corecharm "github.com/juju/juju/core/charm"
	coreunit "github.com/juju/juju/core/unit"
	applicationcharm "github.com/juju/juju/domain/application/charm"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	generationerrors "github.com/juju/juju/domain/generation/errors"
	"github.com/juju/juju/internal/uuid"
)

func (s *applicationRefreshSuite) createGeneration(c *tc.C, name string, stateID int) string {
	generationUUID := uuid.MustNewUUID().String()
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO generation (
    uuid, generation_id, name, state_id, created_by, created_at
)
VALUES (
    ?,
    (SELECT COALESCE(MAX(generation_id), -1) + 1 FROM generation),
    ?,
    ?,
    'admin',
    DATETIME('now', 'utc')
)
`, generationUUID, name, stateID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	return generationUUID
}

func (s *applicationRefreshSuite) trackUnit(
	c *tc.C, generationUUID string, unitUUID coreunit.UUID,
) {
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO generation_unit (generation_uuid, unit_uuid)
VALUES (?, ?)
`, generationUUID, unitUUID.String())
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
}

func (s *applicationRefreshSuite) applicationCharm(
	c *tc.C, applicationUUID coreapplication.UUID,
) (corecharm.ID, int) {
	var charmUUID string
	var modifiedVersion int
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT charm_uuid, charm_modified_version
FROM application
WHERE uuid = ?
`, applicationUUID.String()).Scan(&charmUUID, &modifiedVersion)
	})
	c.Assert(err, tc.ErrorIsNil)
	return corecharm.ID(charmUUID), modifiedVersion
}

func (s *applicationRefreshSuite) TestSetGenerationCharm(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	mainCharm, mainVersion := s.applicationCharm(c, appUUID)
	branchCharm := s.createCharm(c, createCharmArgs{name: "mediawiki"})
	s.createGeneration(c, "test", 0)

	err := s.state.SetGenerationCharm(c.Context(), "test", appUUID, branchCharm)
	c.Assert(err, tc.ErrorIsNil)

	var gotApplicationUUID, gotCharmUUID string
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT application_uuid, charm_uuid
FROM generation_application_charm AS gac
JOIN generation AS g ON g.uuid = gac.generation_uuid
WHERE g.name = 'test'
`).Scan(&gotApplicationUUID, &gotCharmUUID)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(gotApplicationUUID, tc.Equals, appUUID.String())
	c.Check(gotCharmUUID, tc.Equals, branchCharm.String())

	gotMainCharm, gotMainVersion := s.applicationCharm(c, appUUID)
	c.Check(gotMainCharm, tc.Equals, mainCharm)
	c.Check(gotMainVersion, tc.Equals, mainVersion)
}

func (s *applicationRefreshSuite) TestSetGenerationCharmUpdatesOverride(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	firstCharm := s.createCharm(c, createCharmArgs{name: "mediawiki-first"})
	secondCharm := s.createCharm(c, createCharmArgs{name: "mediawiki-second"})
	s.createGeneration(c, "test", 0)

	c.Assert(s.state.SetGenerationCharm(c.Context(), "test", appUUID, firstCharm), tc.ErrorIsNil)
	c.Assert(s.state.SetGenerationCharm(c.Context(), "test", appUUID, secondCharm), tc.ErrorIsNil)

	var count int
	var gotCharmUUID string
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT COUNT(*), MAX(charm_uuid)
FROM generation_application_charm
`).Scan(&count, &gotCharmUUID)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(count, tc.Equals, 1)
	c.Check(gotCharmUUID, tc.Equals, secondCharm.String())
}

func (s *applicationRefreshSuite) TestSetGenerationCharmBranchErrors(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	branchCharm := s.createCharm(c, createCharmArgs{name: "mediawiki"})
	s.createGeneration(c, "committed", 1)
	s.createGeneration(c, "aborted", 2)

	for _, name := range []string{"missing", "committed", "aborted"} {
		err := s.state.SetGenerationCharm(c.Context(), name, appUUID, branchCharm)
		c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
	}
}

func (s *applicationRefreshSuite) TestSetGenerationCharmEntityErrors(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	branchCharm := s.createCharm(c, createCharmArgs{name: "mediawiki"})
	s.createGeneration(c, "test", 0)

	err := s.state.SetGenerationCharm(
		c.Context(), "test", tc.Must(c, coreapplication.NewUUID), branchCharm,
	)
	c.Check(err, tc.ErrorIs, applicationerrors.ApplicationNotFound)

	err = s.state.SetGenerationCharm(
		c.Context(), "test", appUUID, tc.Must(c, corecharm.NewID),
	)
	c.Check(err, tc.ErrorIs, applicationerrors.CharmNotFound)
}

func (s *applicationRefreshSuite) TestSetGenerationCharmDeadApplication(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	branchCharm := s.createCharm(c, createCharmArgs{name: "mediawiki"})
	s.createGeneration(c, "test", 0)
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE application SET life_id = 2 WHERE uuid = ?`, appUUID.String())
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.SetGenerationCharm(c.Context(), "test", appUUID, branchCharm)
	c.Check(err, tc.ErrorIs, applicationerrors.ApplicationIsDead)
}

func (s *applicationRefreshSuite) TestSetGenerationCharmRejectsIncompatibleRelation(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{
		appName: "mediawiki",
		relations: []applicationcharm.Relation{{
			Name:      "database",
			Role:      applicationcharm.RoleProvider,
			Interface: "database",
			Limit:     1,
		}},
	})
	s.establishRelationWith(c, appUUID, "database", applicationcharm.RoleRequirer)
	branchCharm := s.createCharm(c, createCharmArgs{name: "mediawiki-new"})
	s.createGeneration(c, "test", 0)

	err := s.state.SetGenerationCharm(c.Context(), "test", appUUID, branchCharm)
	c.Check(err, tc.ErrorMatches, `.*charm has no corresponding relation "database"`)

	var count int
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM generation_application_charm`).Scan(&count)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(count, tc.Equals, 0)
}

func (s *applicationRefreshSuite) TestCreateApplicationDoesNotJoinGeneration(c *tc.C) {
	s.createGeneration(c, "test", 0)
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	s.addUnit(c, coreunit.Name("mediawiki/0"), appUUID)

	var charmOverrides, trackedUnits int
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM generation_application_charm`).Scan(&charmOverrides); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM generation_unit`).Scan(&trackedUnits)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(charmOverrides, tc.Equals, 0)
	c.Check(trackedUnits, tc.Equals, 0)
}

func (s *applicationRefreshSuite) TestGetResolvedUnitCharmSelectiveTracking(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	mainCharm, _ := s.applicationCharm(c, appUUID)
	branchCharm := s.createCharm(c, createCharmArgs{name: "mediawiki"})
	trackedUnit := s.addUnit(c, coreunit.Name("mediawiki/0"), appUUID)
	mainUnit := s.addUnit(c, coreunit.Name("mediawiki/1"), appUUID)
	generationUUID := s.createGeneration(c, "test", 0)
	s.trackUnit(c, generationUUID, trackedUnit)
	c.Assert(s.state.SetGenerationCharm(c.Context(), "test", appUUID, branchCharm), tc.ErrorIsNil)

	got, err := s.state.GetResolvedUnitCharm(c.Context(), trackedUnit)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.Equals, branchCharm)

	got, err = s.state.GetResolvedUnitCharm(c.Context(), mainUnit)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.Equals, mainCharm)
}

func (s *applicationRefreshSuite) TestGetResolvedUnitCharmFallsBackToMain(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	mainCharm, _ := s.applicationCharm(c, appUUID)
	unitUUID := s.addUnit(c, coreunit.Name("mediawiki/0"), appUUID)
	generationUUID := s.createGeneration(c, "test", 0)
	s.trackUnit(c, generationUUID, unitUUID)

	got, err := s.state.GetResolvedUnitCharm(c.Context(), unitUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.Equals, mainCharm)
}

func (s *applicationRefreshSuite) TestGetResolvedUnitCharmIgnoresOtherApplication(c *tc.C) {
	firstAppUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	secondAppUUID := s.createApplication(c, createApplicationArgs{appName: "wordpress"})
	secondMainCharm, _ := s.applicationCharm(c, secondAppUUID)
	branchCharm := s.createCharm(c, createCharmArgs{name: "mediawiki"})
	secondUnit := s.addUnit(c, coreunit.Name("wordpress/0"), secondAppUUID)
	generationUUID := s.createGeneration(c, "test", 0)
	s.trackUnit(c, generationUUID, secondUnit)
	c.Assert(s.state.SetGenerationCharm(c.Context(), "test", firstAppUUID, branchCharm), tc.ErrorIsNil)

	got, err := s.state.GetResolvedUnitCharm(c.Context(), secondUnit)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.Equals, secondMainCharm)
}

func (s *applicationRefreshSuite) TestGetResolvedUnitCharmIgnoresCompletedBranch(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{appName: "mediawiki"})
	mainCharm, _ := s.applicationCharm(c, appUUID)
	branchCharm := s.createCharm(c, createCharmArgs{name: "mediawiki"})
	unitUUID := s.addUnit(c, coreunit.Name("mediawiki/0"), appUUID)
	generationUUID := s.createGeneration(c, "test", 0)
	s.trackUnit(c, generationUUID, unitUUID)
	c.Assert(s.state.SetGenerationCharm(c.Context(), "test", appUUID, branchCharm), tc.ErrorIsNil)

	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE generation SET state_id = 1 WHERE uuid = ?`, generationUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	got, err := s.state.GetResolvedUnitCharm(c.Context(), unitUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.Equals, mainCharm)
}

func (s *applicationRefreshSuite) TestGetResolvedUnitCharmUnitNotFound(c *tc.C) {
	_, err := s.state.GetResolvedUnitCharm(c.Context(), tc.Must(c, coreunit.NewUUID))
	c.Check(err, tc.ErrorIs, applicationerrors.UnitNotFound)
}
