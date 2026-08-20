// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/juju/tc"
)

func (s *stateSuite) createCharm(c *tc.C, name string) string {
	charmUUID := s.newUUID(c)
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO charm (uuid, reference_name, architecture_id, revision)
VALUES (?, ?, 0, 1)`, charmUUID, name)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	return charmUUID
}

func (s *stateSuite) addCharmRelation(
	c *tc.C, charmUUID, name, relationInterface string,
) string {
	return s.addCharmRelationWith(c, charmUUID, testCharmRelation{
		name: name, relationInterface: relationInterface,
	})
}

type testCharmRelation struct {
	name              string
	relationInterface string
	roleID            int
	scopeID           int
	capacity          int
}

func (s *stateSuite) addCharmRelationWith(
	c *tc.C, charmUUID string, relation testCharmRelation,
) string {
	relationUUID := s.newUUID(c)
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO charm_relation (
    uuid, charm_uuid, name, role_id, scope_id, interface, optional, capacity
)
VALUES (?, ?, ?, ?, ?, ?, FALSE, ?)
`, relationUUID, charmUUID, relation.name, relation.roleID,
			relation.scopeID, relation.relationInterface, relation.capacity)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	return relationUUID
}

func (s *stateSuite) addApplicationEndpoint(
	c *tc.C, appUUID, charmRelationUUID string,
) string {
	endpointUUID := s.newUUID(c)
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO application_endpoint (uuid, application_uuid, charm_relation_uuid)
VALUES (?, ?, ?)
`, endpointUUID, appUUID, charmRelationUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	return endpointUUID
}

func (s *stateSuite) establishRelation(c *tc.C, endpointUUID string, relationID int) {
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		relationUUID := s.newUUID(c)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO relation (uuid, life_id, relation_id, scope_id)
VALUES (?, 0, ?, 0)
`, relationUUID, relationID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO relation_endpoint (uuid, relation_uuid, endpoint_uuid)
VALUES (?, ?, ?)
`, s.newUUID(c), relationUUID, endpointUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) applicationCharmUUID(c *tc.C, appUUID string) string {
	var charmUUID string
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT charm_uuid FROM application WHERE uuid = ?`, appUUID).Scan(&charmUUID)
	})
	c.Assert(err, tc.ErrorIsNil)
	return charmUUID
}

func (s *stateSuite) stageGenerationCharm(
	c *tc.C, generationUUID, appUUID, charmUUID string,
) {
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO generation_application_charm (generation_uuid, application_uuid, charm_uuid)
VALUES (?, ?, ?)`, generationUUID, appUUID, charmUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) addCharmExtraBinding(c *tc.C, charmUUID, name string) string {
	uuid := s.newUUID(c)
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO charm_extra_binding (uuid, charm_uuid, name)
VALUES (?, ?, ?)`, uuid, charmUUID, name)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
	return uuid
}

func (s *stateSuite) TestCommitFoldsCharm(c *tc.C) {
	firstAppUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	secondAppUUID, _ := s.createUnit(c, "wordpress", "wordpress/0")
	unaffectedAppUUID, _ := s.createUnit(c, "mysql", "mysql/0")
	firstNewCharmUUID := s.createCharm(c, "mediawiki-new")
	secondNewCharmUUID := s.createCharm(c, "wordpress-new")
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	var unaffectedCharmUUID string
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `
SELECT charm_uuid FROM application WHERE uuid = ?`, unaffectedAppUUID).Scan(&unaffectedCharmUUID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO generation_application_charm (generation_uuid, application_uuid, charm_uuid)
VALUES (?, ?, ?), (?, ?, ?)`,
			genUUID, firstAppUUID, firstNewCharmUUID,
			genUUID, secondAppUUID, secondNewCharmUUID,
		)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)

	got := make(map[string]string)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT uuid, charm_uuid FROM application WHERE uuid IN (?, ?, ?)`, firstAppUUID, secondAppUUID, unaffectedAppUUID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var appUUID, charmUUID string
			if err := rows.Scan(&appUUID, &charmUUID); err != nil {
				return err
			}
			got[appUUID] = charmUUID
		}
		return rows.Err()
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.DeepEquals, map[string]string{
		firstAppUUID:      firstNewCharmUUID,
		secondAppUUID:     secondNewCharmUUID,
		unaffectedAppUUID: unaffectedCharmUUID,
	})
}

func (s *stateSuite) TestCommitRejectsCharmThatBreaksRelationAddedAfterStaging(c *tc.C) {
	appUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	var originalCharmUUID string
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT charm_uuid FROM application WHERE uuid = ?`, appUUID).Scan(&originalCharmUUID)
	})
	c.Assert(err, tc.ErrorIsNil)
	originalRelationUUID := s.addCharmRelation(c, originalCharmUUID, "database", "database")
	endpointUUID := s.addApplicationEndpoint(c, appUUID, originalRelationUUID)

	newCharmUUID := s.createCharm(c, "mediawiki-new")
	genUUID := s.newUUID(c)
	_, err = s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO generation_application_charm (generation_uuid, application_uuid, charm_uuid)
VALUES (?, ?, ?)`, genUUID, appUUID, newCharmUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	// The relation is established after the branch charm was staged, so commit
	// must repeat the compatibility check in its transaction.
	s.establishRelation(c, endpointUUID, 42)
	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Check(err, tc.ErrorMatches, `folding charm changes: .*charm has no corresponding relation "database"`)

	var gotCharmUUID string
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT charm_uuid FROM application WHERE uuid = ?`, appUUID).Scan(&gotCharmUUID)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(gotCharmUUID, tc.Equals, originalCharmUUID)
	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Check(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestCommitReconcilesCompatibleRelationEndpoint(c *tc.C) {
	appUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	var originalCharmUUID string
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT charm_uuid FROM application WHERE uuid = ?`, appUUID).Scan(&originalCharmUUID)
	})
	c.Assert(err, tc.ErrorIsNil)
	originalRelationUUID := s.addCharmRelation(c, originalCharmUUID, "database", "database")
	endpointUUID := s.addApplicationEndpoint(c, appUUID, originalRelationUUID)
	s.establishRelation(c, endpointUUID, 42)

	newCharmUUID := s.createCharm(c, "mediawiki-new")
	newRelationUUID := s.addCharmRelation(c, newCharmUUID, "database", "database")
	genUUID := s.newUUID(c)
	_, err = s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO generation_application_charm (generation_uuid, application_uuid, charm_uuid)
VALUES (?, ?, ?)`, genUUID, appUUID, newCharmUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)

	var gotCharmUUID, gotRelationUUID string
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `
SELECT charm_uuid FROM application WHERE uuid = ?`, appUUID).Scan(&gotCharmUUID); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `
SELECT charm_relation_uuid FROM application_endpoint WHERE uuid = ?`, endpointUUID).Scan(&gotRelationUUID)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(gotCharmUUID, tc.Equals, newCharmUUID)
	c.Check(gotRelationUUID, tc.Equals, newRelationUUID)
}

func (s *stateSuite) TestCommitRejectsIncompatibleRelationChanges(c *tc.C) {
	tests := []struct {
		name       string
		target     testCharmRelation
		relations  int
		errorMatch string
	}{
		{
			name: "role",
			target: testCharmRelation{
				name: "database", relationInterface: "database", roleID: 1,
			},
			relations:  1,
			errorMatch: `.*cannot change role of relation "database" from provider to requirer`,
		},
		{
			name: "interface",
			target: testCharmRelation{
				name: "database", relationInterface: "different",
			},
			relations:  1,
			errorMatch: `.*cannot change interface of relation "database" from database to different`,
		},
		{
			name: "scope",
			target: testCharmRelation{
				name: "database", relationInterface: "database", scopeID: 1,
			},
			relations:  1,
			errorMatch: `.*cannot change scope of relation "database" from global to container`,
		},
		{
			name: "capacity",
			target: testCharmRelation{
				name: "database", relationInterface: "database", capacity: 1,
			},
			relations:  2,
			errorMatch: `.*new charm version imposes a maximum relation limit of 1 for "database" which cannot be satisfied by the number of already established relations \(2\)`,
		},
	}

	for i, test := range tests {
		c.Logf("test %q", test.name)
		appName := fmt.Sprintf("mediawiki-%d", i)
		appUUID, _ := s.createUnit(c, appName, appName+"/0")
		originalCharmUUID := s.applicationCharmUUID(c, appUUID)
		originalRelationUUID := s.addCharmRelation(
			c, originalCharmUUID, "database", "database",
		)
		endpointUUID := s.addApplicationEndpoint(c, appUUID, originalRelationUUID)
		for relation := 0; relation < test.relations; relation++ {
			s.establishRelation(c, endpointUUID, 100+i*10+relation)
		}

		newCharmUUID := s.createCharm(c, appName+"-new")
		s.addCharmRelationWith(c, newCharmUUID, test.target)
		genUUID := s.newUUID(c)
		branchName := fmt.Sprintf("test-%d", i)
		_, err := s.state.AddBranch(c.Context(), genUUID, branchName, "admin")
		c.Assert(err, tc.ErrorIsNil)
		s.stageGenerationCharm(c, genUUID, appUUID, newCharmUUID)

		_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
		c.Check(err, tc.ErrorMatches, test.errorMatch)
		c.Check(s.applicationCharmUUID(c, appUUID), tc.Equals, originalCharmUUID)

		var gotRelationUUID string
		err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
			return tx.QueryRowContext(ctx, `
SELECT charm_relation_uuid FROM application_endpoint WHERE uuid = ?`, endpointUUID).Scan(&gotRelationUUID)
		})
		c.Assert(err, tc.ErrorIsNil)
		c.Check(gotRelationUUID, tc.Equals, originalRelationUUID)
		c.Assert(s.state.Abort(c.Context(), genUUID, "admin"), tc.ErrorIsNil)
	}
}

func (s *stateSuite) TestCommitCharmValidationIsAtomicAcrossApplications(c *tc.C) {
	firstAppUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	firstOriginalCharmUUID := s.applicationCharmUUID(c, firstAppUUID)
	firstOriginalRelationUUID := s.addCharmRelation(
		c, firstOriginalCharmUUID, "database", "database",
	)
	firstEndpointUUID := s.addApplicationEndpoint(c, firstAppUUID, firstOriginalRelationUUID)
	s.establishRelation(c, firstEndpointUUID, 200)
	firstNewCharmUUID := s.createCharm(c, "mediawiki-new")
	s.addCharmRelation(c, firstNewCharmUUID, "database", "database")

	secondAppUUID, _ := s.createUnit(c, "wordpress", "wordpress/0")
	secondOriginalCharmUUID := s.applicationCharmUUID(c, secondAppUUID)
	secondOriginalRelationUUID := s.addCharmRelation(
		c, secondOriginalCharmUUID, "cache", "cache",
	)
	secondEndpointUUID := s.addApplicationEndpoint(c, secondAppUUID, secondOriginalRelationUUID)
	s.establishRelation(c, secondEndpointUUID, 201)
	secondNewCharmUUID := s.createCharm(c, "wordpress-new")

	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	s.stageGenerationCharm(c, genUUID, firstAppUUID, firstNewCharmUUID)
	s.stageGenerationCharm(c, genUUID, secondAppUUID, secondNewCharmUUID)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Check(err, tc.ErrorMatches, `folding charm changes: .*charm has no corresponding relation "cache"`)
	c.Check(s.applicationCharmUUID(c, firstAppUUID), tc.Equals, firstOriginalCharmUUID)
	c.Check(s.applicationCharmUUID(c, secondAppUUID), tc.Equals, secondOriginalCharmUUID)
	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Check(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestCommitReconcilesAddedAndRemovedRelationEndpoints(c *tc.C) {
	appUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	originalCharmUUID := s.applicationCharmUUID(c, appUUID)
	databaseRelationUUID := s.addCharmRelation(
		c, originalCharmUUID, "database", "database",
	)
	legacyRelationUUID := s.addCharmRelation(c, originalCharmUUID, "legacy", "legacy")
	databaseEndpointUUID := s.addApplicationEndpoint(c, appUUID, databaseRelationUUID)
	legacyEndpointUUID := s.addApplicationEndpoint(c, appUUID, legacyRelationUUID)
	s.establishRelation(c, databaseEndpointUUID, 300)

	newCharmUUID := s.createCharm(c, "mediawiki-new")
	newDatabaseRelationUUID := s.addCharmRelation(c, newCharmUUID, "database", "database")
	s.addCharmRelation(c, newCharmUUID, "metrics", "metrics")
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	s.stageGenerationCharm(c, genUUID, appUUID, newCharmUUID)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)

	endpoints := make(map[string]string)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT cr.name, ae.uuid
FROM application_endpoint AS ae
JOIN charm_relation AS cr ON cr.uuid = ae.charm_relation_uuid
WHERE ae.application_uuid = ?`, appUUID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name, uuid string
			if err := rows.Scan(&name, &uuid); err != nil {
				return err
			}
			endpoints[name] = uuid
		}
		return rows.Err()
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(endpoints, tc.HasLen, 2)
	c.Check(endpoints["database"], tc.Equals, databaseEndpointUUID)
	c.Check(endpoints["metrics"] == "", tc.IsFalse)
	_, found := endpoints["legacy"]
	c.Check(found, tc.IsFalse)

	var gotRelationUUID string
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT charm_relation_uuid FROM application_endpoint WHERE uuid = ?`, databaseEndpointUUID).Scan(&gotRelationUUID)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(gotRelationUUID, tc.Equals, newDatabaseRelationUUID)

	var legacyCount int
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM application_endpoint WHERE uuid = ?`, legacyEndpointUUID).Scan(&legacyCount)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(legacyCount, tc.Equals, 0)
}

func (s *stateSuite) TestCommitReconcilesExtraEndpointBindings(c *tc.C) {
	appUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	originalCharmUUID := s.applicationCharmUUID(c, appUUID)
	originalMetricsUUID := s.addCharmExtraBinding(c, originalCharmUUID, "metrics")
	originalLegacyUUID := s.addCharmExtraBinding(c, originalCharmUUID, "legacy")
	const defaultSpaceUUID = "656b4a82-e28c-53d6-a014-f0dd53417eb6"
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO application_extra_endpoint (
    application_uuid, charm_extra_binding_uuid, space_uuid
)
VALUES (?, ?, ?), (?, ?, NULL)
`, appUUID, originalMetricsUUID, defaultSpaceUUID,
			appUUID, originalLegacyUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	newCharmUUID := s.createCharm(c, "mediawiki-new")
	newMetricsUUID := s.addCharmExtraBinding(c, newCharmUUID, "metrics")
	newTracingUUID := s.addCharmExtraBinding(c, newCharmUUID, "tracing")
	genUUID := s.newUUID(c)
	_, err = s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	s.stageGenerationCharm(c, genUUID, appUUID, newCharmUUID)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)

	type binding struct {
		uuid  string
		space sql.NullString
	}
	bindings := make(map[string]binding)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT ceb.name, aee.charm_extra_binding_uuid, aee.space_uuid
FROM application_extra_endpoint AS aee
JOIN charm_extra_binding AS ceb ON ceb.uuid = aee.charm_extra_binding_uuid
WHERE aee.application_uuid = ?`, appUUID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var value binding
			if err := rows.Scan(&name, &value.uuid, &value.space); err != nil {
				return err
			}
			bindings[name] = value
		}
		return rows.Err()
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(bindings, tc.HasLen, 2)
	c.Check(bindings["metrics"], tc.DeepEquals, binding{
		uuid: newMetricsUUID, space: sql.NullString{String: defaultSpaceUUID, Valid: true},
	})
	c.Check(bindings["tracing"], tc.DeepEquals, binding{uuid: newTracingUUID})
	_, found := bindings["legacy"]
	c.Check(found, tc.IsFalse)
}
