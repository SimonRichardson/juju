// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"

	"github.com/juju/tc"

	generationerrors "github.com/juju/juju/domain/generation/errors"
)

func (s *stateSuite) TestGenerationCommitTableErrors(c *tc.C) {
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `ALTER TABLE generation_commit RENAME TO generation_commit_unavailable`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.ListCommits(c.Context())
	c.Check(err, tc.NotNil)
	_, err = s.state.GetCommit(c.Context(), 0)
	c.Check(err, tc.NotNil)

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `ALTER TABLE generation_commit_unavailable RENAME TO generation_commit`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestGenerationCommitConfigTableError(c *tc.C) {
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "admin")
	c.Assert(err, tc.ErrorIsNil)

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `ALTER TABLE generation_commit_config RENAME TO generation_commit_config_unavailable`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.GetCommit(c.Context(), 0)
	c.Check(err, tc.NotNil)

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `ALTER TABLE generation_commit_config_unavailable RENAME TO generation_commit_config`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestListCommitsOldestFirst(c *tc.C) {
	for _, name := range []string{"one", "two"} {
		genUUID := s.newUUID(c)
		_, err := s.state.AddBranch(c.Context(), genUUID, name, "creator")
		c.Assert(err, tc.ErrorIsNil)
		_, err = s.state.Commit(c.Context(), genUUID, s.newUUID(c), "committer")
		c.Assert(err, tc.ErrorIsNil)
	}

	commits, err := s.state.ListCommits(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(commits, tc.HasLen, 2)
	c.Check(commits[0].Name, tc.Equals, "one")
	c.Check(commits[0].GenerationID, tc.Equals, uint64(0))
	c.Check(commits[1].Name, tc.Equals, "two")
	c.Check(commits[1].GenerationID, tc.Equals, uint64(1))
}

func (s *stateSuite) TestCommitNotFound(c *tc.C) {
	_, err := s.state.GetCommit(c.Context(), 42)
	c.Assert(err, tc.ErrorIs, generationerrors.CommitNotFound)
}

func (s *stateSuite) TestDecodeConfigValueMalformedValuesRemainStrings(c *tc.C) {
	c.Check(decodeConfigValue(1, sql.NullString{String: "invalid", Valid: true}), tc.Equals, any("invalid"))
	c.Check(decodeConfigValue(2, sql.NullString{String: "invalid", Valid: true}), tc.Equals, any("invalid"))
	c.Check(decodeConfigValue(3, sql.NullString{String: "invalid", Valid: true}), tc.Equals, any("invalid"))
	c.Check(decodeConfigValue(0, sql.NullString{String: "value", Valid: true}), tc.Equals, any("value"))
	c.Check(decodeConfigValue(0, sql.NullString{}), tc.IsNil)
}
