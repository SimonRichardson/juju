// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"

	"github.com/juju/tc"

	generationerrors "github.com/juju/juju/domain/generation/errors"
	"github.com/juju/juju/domain/generation/internal"
)

func (s *stateSuite) TestCommit(c *tc.C) {
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	commitUUID := s.newUUID(c)
	id, err := s.state.Commit(c.Context(), genUUID, commitUUID, "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(id, tc.Equals, uint64(0))

	// The branch is no longer in flight.
	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Assert(err, tc.ErrorIs, generationerrors.BranchNotFound)

	commits, err := s.state.ListCommits(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(commits, tc.HasLen, 1)

	commit, err := s.state.GetCommit(c.Context(), id)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(commit.Name, tc.Equals, "test")
	c.Check(commit.CreatedBy, tc.Equals, "admin")
	c.Check(commit.CommittedBy, tc.Equals, "admin")
	c.Check(commit.UUID, tc.Equals, commitUUID)
	c.Check(commit.CommittedAt.IsZero(), tc.IsFalse)
}

func (s *stateSuite) TestCommitClearsTrackingAndNameCanBeReused(c *tc.C) {
	_, unitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "creator")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(s.state.TrackUnits(c.Context(), genUUID, []string{unitUUID}), tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "committer")
	c.Assert(err, tc.ErrorIsNil)

	tracked, err := s.state.HasTrackedUnits(c.Context(), genUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(tracked, tc.IsFalse)
	_, err = s.state.GetBranchForUnit(c.Context(), unitUUID)
	c.Assert(err, tc.ErrorIs, generationerrors.BranchNotFound)

	newGenerationUUID := s.newUUID(c)
	id, err := s.state.AddBranch(c.Context(), newGenerationUUID, "test", "creator")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(id, tc.Equals, uint64(1))
	c.Check(s.state.TrackUnits(c.Context(), newGenerationUUID, []string{unitUUID}), tc.ErrorIsNil)
}

func (s *stateSuite) TestCommitDoesNotAffectOtherActiveBranch(c *tc.C) {
	_, firstUnitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	_, secondUnitUUID := s.createUnit(c, "wordpress", "wordpress/0")
	firstGenerationUUID := s.newUUID(c)
	secondGenerationUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), firstGenerationUUID, "first", "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.AddBranch(c.Context(), secondGenerationUUID, "second", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(s.state.TrackUnits(c.Context(), firstGenerationUUID, []string{firstUnitUUID}), tc.ErrorIsNil)
	c.Assert(s.state.TrackUnits(c.Context(), secondGenerationUUID, []string{secondUnitUUID}), tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), firstGenerationUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)
	second, err := s.state.GetBranchByName(c.Context(), "second")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(second.UUID, tc.Equals, secondGenerationUUID)
	unitBranch, err := s.state.GetBranchForUnit(c.Context(), secondUnitUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(unitBranch.Name, tc.Equals, "second")
}

func (s *stateSuite) TestCommitBranchNotFoundOrAlreadyCompleted(c *tc.C) {
	_, err := s.state.Commit(c.Context(), s.newUUID(c), s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIs, generationerrors.BranchNotFound)

	genUUID := s.newUUID(c)
	_, err = s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *stateSuite) TestCommitRollsBackWhenCommitInsertFails(c *tc.C) {
	commitUUID := s.newUUID(c)
	firstGenerationUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), firstGenerationUUID, "first", "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.Commit(c.Context(), firstGenerationUUID, commitUUID, "admin")
	c.Assert(err, tc.ErrorIsNil)

	appUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	secondGenerationUUID := s.newUUID(c)
	_, err = s.state.AddBranch(c.Context(), secondGenerationUUID, "second", "admin")
	c.Assert(err, tc.ErrorIsNil)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value)
VALUES (?, ?, 'value', 0, 'changed')`, secondGenerationUUID, appUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), secondGenerationUUID, commitUUID, "admin")
	c.Check(err, tc.NotNil)

	var configCount int
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM application_config WHERE application_uuid = ?`, appUUID).Scan(&configCount)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(configCount, tc.Equals, 0)
	_, err = s.state.GetBranchByName(c.Context(), "second")
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestCommitRollsBackWhenConfigHistoryInsertFails(c *tc.C) {
	appUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value)
VALUES (?, ?, 'value', 0, 'changed')`, genUUID, appUUID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
CREATE TRIGGER fail_generation_commit_config
BEFORE INSERT ON generation_commit_config
BEGIN
    SELECT RAISE(ABORT, 'config history unavailable');
END`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Check(err, tc.ErrorMatches, `inserting commit config: .*config history unavailable.*`)

	var configCount, commitCount int
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM application_config WHERE application_uuid = ?`, appUUID).Scan(&configCount); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM generation_commit`).Scan(&commitCount)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(configCount, tc.Equals, 0)
	c.Check(commitCount, tc.Equals, 0)
	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestCommitRollsBackWhenTrackingClearFails(c *tc.C) {
	_, unitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(s.state.TrackUnits(c.Context(), genUUID, []string{unitUUID}), tc.ErrorIsNil)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
CREATE TRIGGER fail_generation_unit_delete
BEFORE DELETE ON generation_unit
BEGIN
    SELECT RAISE(ABORT, 'tracking unavailable');
END`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Check(err, tc.ErrorMatches, `clearing tracked units: .*tracking unavailable.*`)

	tracked, err := s.state.HasTrackedUnits(c.Context(), genUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(tracked, tc.IsTrue)
	commits, err := s.state.ListCommits(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(commits, tc.HasLen, 0)
}

func (s *stateSuite) TestCommitRollsBackWhenApplicationOwnershipClearFails(c *tc.C) {
	_, unitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	generationUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), generationUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(s.state.TrackUnits(c.Context(), generationUUID, []string{unitUUID}), tc.ErrorIsNil)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
CREATE TRIGGER fail_generation_application_delete
BEFORE DELETE ON generation_application
BEGIN
    SELECT RAISE(ABORT, 'ownership unavailable');
END`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), generationUUID, s.newUUID(c), "admin")
	c.Check(err, tc.ErrorMatches, `clearing application ownership: .*ownership unavailable.*`)
	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Check(err, tc.ErrorIsNil)
	tracked, err := s.state.HasTrackedUnits(c.Context(), generationUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(tracked, tc.IsTrue)
}

func (s *stateSuite) TestCommitRollsBackWhenConfigHashFails(c *tc.C) {
	appUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value)
VALUES (?, ?, 'value', 0, 'changed')`, genUUID, appUUID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
CREATE TRIGGER fail_application_config_hash
BEFORE INSERT ON application_config_hash
BEGIN
    SELECT RAISE(ABORT, 'hash unavailable');
END`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Check(err, tc.ErrorMatches, `refreshing config hash: .*storing config hash: .*hash unavailable.*`)

	var configCount int
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM application_config WHERE application_uuid = ?`, appUUID).Scan(&configCount)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(configCount, tc.Equals, 0)
	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestCommitRollsBackWhenMarkCommittedFails(c *tc.C) {
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
CREATE TRIGGER fail_generation_mark_committed
BEFORE UPDATE OF state_id ON generation
WHEN NEW.state_id = 1
BEGIN
    SELECT RAISE(ABORT, 'mark unavailable');
END`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Check(err, tc.ErrorMatches, `marking committed: .*mark unavailable.*`)

	commits, err := s.state.ListCommits(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(commits, tc.HasLen, 0)
	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestCommitFoldTableErrors(c *tc.C) {
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	tests := []struct {
		unavailable string
		restore     string
	}{
		{
			unavailable: `ALTER TABLE generation_application_charm RENAME TO generation_application_charm_unavailable`,
			restore:     `ALTER TABLE generation_application_charm_unavailable RENAME TO generation_application_charm`,
		},
		{
			unavailable: `ALTER TABLE application_config RENAME TO application_config_unavailable`,
			restore:     `ALTER TABLE application_config_unavailable RENAME TO application_config`,
		},
		{
			unavailable: `ALTER TABLE generation_application_resource RENAME TO generation_application_resource_unavailable`,
			restore:     `ALTER TABLE generation_application_resource_unavailable RENAME TO generation_application_resource`,
		},
	}
	for _, test := range tests {
		err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, test.unavailable)
			return err
		})
		c.Assert(err, tc.ErrorIsNil)

		_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
		c.Check(err, tc.NotNil)

		err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, test.restore)
			return err
		})
		c.Assert(err, tc.ErrorIsNil)
	}

	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestCommitFoldsConfig(c *tc.C) {
	appUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")

	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value)
VALUES (?, ?, 'use_suffix', 0, 'false')`, genUUID, appUUID)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)

	var value string
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT value FROM application_config
WHERE application_uuid = ? AND "key" = 'use_suffix'`, appUUID).Scan(&value)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(value, tc.Equals, "false")

	// The config is also recorded in the commit history.
	commit, err := s.state.GetCommit(c.Context(), 0)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(commit.Applications, tc.HasLen, 1)
	c.Check(commit.Applications[0].ApplicationName, tc.Equals, "mediawiki")
	c.Check(commit.Applications[0].Config, tc.HasLen, 1)
	c.Check(commit.Applications[0].Config[0].Key, tc.Equals, "use_suffix")
}

func (s *stateSuite) TestCommitFoldsConfigUpdatesDeletesAndHashes(c *tc.C) {
	appUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		for _, query := range []struct {
			stmt string
			args []any
		}{
			{`INSERT INTO application_config (application_uuid, "key", type_id, value) VALUES (?, 'remove-me', 0, 'old')`, []any{appUUID}},
			{`INSERT INTO application_config (application_uuid, "key", type_id, value) VALUES (?, 'update-me', 0, 'old')`, []any{appUUID}},
			{`INSERT INTO application_setting (application_uuid, trust) VALUES (?, true)`, []any{appUUID}},
			{`INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value) VALUES (?, ?, 'remove-me', 0, NULL)`, []any{genUUID, appUUID}},
			{`INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value) VALUES (?, ?, 'update-me', 0, 'new')`, []any{genUUID, appUUID}},
			{`INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value) VALUES (?, ?, 'count', 1, '7')`, []any{genUUID, appUUID}},
			{`INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value) VALUES (?, ?, 'ratio', 2, '1.5')`, []any{genUUID, appUUID}},
			{`INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value) VALUES (?, ?, 'enabled', 3, 'true')`, []any{genUUID, appUUID}},
			{`INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value) VALUES (?, ?, 'secret', 4, 'secret://value')`, []any{genUUID, appUUID}},
		} {
			if _, err := tx.ExecContext(ctx, query.stmt, query.args...); err != nil {
				return err
			}
		}
		return nil
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)

	var removed int
	var hash string
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM application_config
WHERE application_uuid = ? AND "key" = 'remove-me'`, appUUID).Scan(&removed); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `
SELECT sha256 FROM application_config_hash WHERE application_uuid = ?`, appUUID).Scan(&hash)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(removed, tc.Equals, 0)
	c.Check(hash, tc.HasLen, 64)

	commit, err := s.state.GetCommit(c.Context(), 0)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(commit.Applications, tc.HasLen, 1)
	c.Check(commit.Applications[0].Config, tc.DeepEquals, []internal.ConfigChange{
		{Key: "count", Value: 7},
		{Key: "enabled", Value: true},
		{Key: "ratio", Value: 1.5},
		{Key: "remove-me", Value: nil},
		{Key: "secret", Value: "secret://value"},
		{Key: "update-me", Value: "new"},
	})
}

func (s *stateSuite) TestCommitFoldsConfigForMultipleApplications(c *tc.C) {
	firstAppUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	secondAppUUID, _ := s.createUnit(c, "wordpress", "wordpress/0")
	unaffectedAppUUID, _ := s.createUnit(c, "mysql", "mysql/0")
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		for _, query := range []struct {
			stmt string
			args []any
		}{
			{`INSERT INTO application_config (application_uuid, "key", type_id, value) VALUES (?, 'remove-me', 0, 'old')`, []any{secondAppUUID}},
			{`INSERT INTO application_config (application_uuid, "key", type_id, value) VALUES (?, 'untouched', 0, 'original')`, []any{unaffectedAppUUID}},
			{`INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value) VALUES (?, ?, 'enabled', 3, 'true')`, []any{genUUID, firstAppUUID}},
			{`INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value) VALUES (?, ?, 'title', 0, 'wiki')`, []any{genUUID, firstAppUUID}},
			{`INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value) VALUES (?, ?, 'count', 1, '3')`, []any{genUUID, secondAppUUID}},
			{`INSERT INTO generation_application_config (generation_uuid, application_uuid, "key", type_id, value) VALUES (?, ?, 'remove-me', 0, NULL)`, []any{genUUID, secondAppUUID}},
		} {
			if _, err := tx.ExecContext(ctx, query.stmt, query.args...); err != nil {
				return err
			}
		}
		return nil
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)

	commit, err := s.state.GetCommit(c.Context(), 0)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(commit.Applications, tc.HasLen, 2)
	byApplication := make(map[string]internal.ApplicationConfigChange)
	for _, application := range commit.Applications {
		byApplication[application.ApplicationUUID] = application
	}
	c.Check(byApplication[firstAppUUID], tc.DeepEquals, internal.ApplicationConfigChange{
		ApplicationUUID: firstAppUUID,
		ApplicationName: "mediawiki",
		Config: []internal.ConfigChange{
			{Key: "enabled", Value: true},
			{Key: "title", Value: "wiki"},
		},
	})
	c.Check(byApplication[secondAppUUID], tc.DeepEquals, internal.ApplicationConfigChange{
		ApplicationUUID: secondAppUUID,
		ApplicationName: "wordpress",
		Config: []internal.ConfigChange{
			{Key: "count", Value: 3},
			{Key: "remove-me", Value: nil},
		},
	})

	var affectedHashes, unaffectedHashes, untouchedConfig int
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM application_config_hash
WHERE application_uuid IN (?, ?)`, firstAppUUID, secondAppUUID).Scan(&affectedHashes); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM application_config_hash
WHERE application_uuid = ?`, unaffectedAppUUID).Scan(&unaffectedHashes); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM application_config
WHERE application_uuid = ? AND "key" = 'untouched' AND value = 'original'`, unaffectedAppUUID).Scan(&untouchedConfig)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(affectedHashes, tc.Equals, 2)
	c.Check(unaffectedHashes, tc.Equals, 0)
	c.Check(untouchedConfig, tc.Equals, 1)
}

func (s *stateSuite) TestCommitFoldsResource(c *tc.C) {
	appUUID, _ := s.createUnit(c, "mediawiki", "mediawiki/0")
	genUUID := s.newUUID(c)
	oldWebsiteUUID := s.newUUID(c)
	newWebsiteUUID := s.newUUID(c)
	oldThemeUUID := s.newUUID(c)
	newThemeUUID := s.newUUID(c)
	unchangedDatabaseUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		var charmUUID string
		if err := tx.QueryRowContext(ctx, `SELECT charm_uuid FROM application WHERE uuid = ?`, appUUID).Scan(&charmUUID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO charm_resource (charm_uuid, name, kind_id)
VALUES (?, 'website', 0), (?, 'theme', 0), (?, 'database', 0)`, charmUUID, charmUUID, charmUUID); err != nil {
			return err
		}
		resources := []struct {
			uuid     string
			name     string
			revision int
		}{
			{oldWebsiteUUID, "website", 0},
			{newWebsiteUUID, "website", 1},
			{oldThemeUUID, "theme", 0},
			{newThemeUUID, "theme", 1},
			{unchangedDatabaseUUID, "database", 0},
		}
		for _, resource := range resources {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO resource (uuid, charm_uuid, charm_resource_name, revision, origin_type_id, state_id, created_at)
VALUES (?, ?, ?, ?, 1, 0, DATETIME('now', 'utc'))`, resource.uuid, charmUUID, resource.name, resource.revision); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO application_resource (resource_uuid, application_uuid)
VALUES (?, ?), (?, ?), (?, ?)`,
			oldWebsiteUUID, appUUID,
			oldThemeUUID, appUUID,
			unchangedDatabaseUUID, appUUID,
		); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO generation_application_resource (generation_uuid, application_uuid, charm_resource_name, resource_uuid)
VALUES (?, ?, 'website', ?), (?, ?, 'theme', ?)`,
			genUUID, appUUID, newWebsiteUUID,
			genUUID, appUUID, newThemeUUID,
		)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)

	got := make(map[string]string)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT r.charm_resource_name, ar.resource_uuid
FROM application_resource AS ar
JOIN resource AS r ON r.uuid = ar.resource_uuid
WHERE ar.application_uuid = ?`, appUUID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name, resourceUUID string
			if err := rows.Scan(&name, &resourceUUID); err != nil {
				return err
			}
			got[name] = resourceUUID
		}
		return rows.Err()
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.DeepEquals, map[string]string{
		"website":  newWebsiteUUID,
		"theme":    newThemeUUID,
		"database": unchangedDatabaseUUID,
	})
}

func (s *stateSuite) TestComputeConfigHashIsStable(c *tc.C) {
	first, err := computeConfigHash([]configValue{
		{Key: "b", Value: sql.NullString{String: "2", Valid: true}},
		{Key: "a", Value: sql.NullString{String: "1", Valid: true}},
	}, false)
	c.Assert(err, tc.ErrorIsNil)
	second, err := computeConfigHash([]configValue{
		{Key: "a", Value: sql.NullString{String: "1", Valid: true}},
		{Key: "b", Value: sql.NullString{String: "2", Valid: true}},
	}, false)
	c.Assert(err, tc.ErrorIsNil)
	trusted, err := computeConfigHash([]configValue{
		{Key: "a", Value: sql.NullString{String: "1", Valid: true}},
		{Key: "b", Value: sql.NullString{String: "2", Valid: true}},
	}, true)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(first, tc.Equals, second)
	c.Check(first == trusted, tc.IsFalse)
	c.Check(first, tc.HasLen, 64)
}
