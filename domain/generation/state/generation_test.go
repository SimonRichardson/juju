// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"time"

	"github.com/juju/tc"

	"github.com/juju/juju/domain/generation"
	generationerrors "github.com/juju/juju/domain/generation/errors"
)

func (s *stateSuite) TestGenerationTableErrors(c *tc.C) {
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `ALTER TABLE generation RENAME TO generation_unavailable`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.AddBranch(c.Context(), s.newUUID(c), "test", "admin")
	c.Check(err, tc.NotNil)
	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Check(err, tc.NotNil)
	_, err = s.state.ListBranches(c.Context())
	c.Check(err, tc.NotNil)
	_, err = s.state.GetBranchForUnit(c.Context(), s.newUUID(c))
	c.Check(err, tc.NotNil)
	c.Check(s.state.Abort(c.Context(), s.newUUID(c), "admin"), tc.NotNil)
	_, err = s.state.Commit(c.Context(), s.newUUID(c), s.newUUID(c), "admin")
	c.Check(err, tc.NotNil)

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `ALTER TABLE generation_unavailable RENAME TO generation`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestGenerationUnitTableErrors(c *tc.C) {
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `ALTER TABLE generation_unit RENAME TO generation_unit_unavailable`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	c.Check(s.state.TrackUnits(c.Context(), "generation", []string{"unit"}), tc.NotNil)
	c.Check(s.state.UntrackUnits(c.Context(), "generation", []string{"unit"}), tc.NotNil)
	_, err = s.state.GetTrackedUnitNames(c.Context(), "generation")
	c.Check(err, tc.NotNil)
	_, err = s.state.HasTrackedUnits(c.Context(), "generation")
	c.Check(err, tc.NotNil)
	c.Check(s.state.Abort(c.Context(), "generation", "admin"), tc.NotNil)

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `ALTER TABLE generation_unit_unavailable RENAME TO generation_unit`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestGenerationApplicationTableErrors(c *tc.C) {
	_, unitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	generationUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), generationUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
ALTER TABLE generation_application RENAME TO generation_application_unavailable`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)

	c.Check(s.state.TrackUnits(c.Context(), generationUUID, []string{unitUUID}), tc.NotNil)
	c.Check(s.state.Abort(c.Context(), generationUUID, "admin"), tc.NotNil)

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
ALTER TABLE generation_application_unavailable RENAME TO generation_application`)
		return err
	})
	c.Assert(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestAddBranch(c *tc.C) {
	id, err := s.state.AddBranch(c.Context(), s.newUUID(c), "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(id, tc.Equals, uint64(0))

	got, err := s.state.GetBranchByName(c.Context(), "test")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got.Name, tc.Equals, "test")
	c.Check(got.State, tc.Equals, string(generation.StateInFlight))
	c.Check(got.CreatedBy, tc.Equals, "admin")
	c.Check(got.GenerationID, tc.Equals, id)
}

func (s *stateSuite) TestAddBranchDuplicateName(c *tc.C) {
	_, err := s.state.AddBranch(c.Context(), s.newUUID(c), "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.AddBranch(c.Context(), s.newUUID(c), "test", "admin")
	c.Assert(err, tc.ErrorIs, generationerrors.BranchAlreadyExists)
}

func (s *stateSuite) TestGetBranchByNameNotFound(c *tc.C) {
	_, err := s.state.GetBranchByName(c.Context(), "missing")
	c.Assert(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *stateSuite) TestListBranches(c *tc.C) {
	got, err := s.state.ListBranches(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.HasLen, 0)

	_, err = s.state.AddBranch(c.Context(), s.newUUID(c), "one", "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.AddBranch(c.Context(), s.newUUID(c), "two", "admin")
	c.Assert(err, tc.ErrorIsNil)

	got, err = s.state.ListBranches(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got, tc.HasLen, 2)
	c.Check(got[0].Name, tc.Equals, "one")
	c.Check(got[0].GenerationID, tc.Equals, uint64(0))
	c.Check(got[1].Name, tc.Equals, "two")
	c.Check(got[1].GenerationID, tc.Equals, uint64(1))
}

func (s *stateSuite) TestBranchNameCanBeReusedAfterAbort(c *tc.C) {
	firstUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), firstUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(s.state.Abort(c.Context(), firstUUID, "admin"), tc.ErrorIsNil)

	id, err := s.state.AddBranch(c.Context(), s.newUUID(c), "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(id, tc.Equals, uint64(1))
}

func (s *stateSuite) TestTrackUnits(c *tc.C) {
	_, unitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")

	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.TrackUnits(c.Context(), genUUID, []string{unitUUID})
	c.Assert(err, tc.ErrorIsNil)

	names, err := s.state.GetTrackedUnitNames(c.Context(), genUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(names, tc.DeepEquals, []string{"mediawiki/0"})

	tracked, err := s.state.HasTrackedUnits(c.Context(), genUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(tracked, tc.IsTrue)

	got, err := s.state.GetBranchForUnit(c.Context(), unitUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got.Name, tc.Equals, "test")
}

func (s *stateSuite) TestTrackUnitsUnknownUnit(c *tc.C) {
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.TrackUnits(c.Context(), genUUID, []string{s.newUUID(c)})
	c.Assert(err, tc.ErrorIs, generationerrors.UnitNotFound)
}

func (s *stateSuite) TestTrackUnitsIsAtomic(c *tc.C) {
	_, unitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.TrackUnits(c.Context(), genUUID, []string{unitUUID, s.newUUID(c)})
	c.Assert(err, tc.ErrorIs, generationerrors.UnitNotFound)

	names, err := s.state.GetTrackedUnitNames(c.Context(), genUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(names, tc.HasLen, 0)
}

func (s *stateSuite) TestMultipleBranchesCanBeActive(c *tc.C) {
	_, err := s.state.AddBranch(c.Context(), s.newUUID(c), "first", "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.AddBranch(c.Context(), s.newUUID(c), "second", "admin")
	c.Check(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestBranchesCanOwnDifferentApplications(c *tc.C) {
	_, firstUnitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	_, secondUnitUUID := s.createUnit(c, "wordpress", "wordpress/0")
	firstGenerationUUID := s.newUUID(c)
	secondGenerationUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), firstGenerationUUID, "first", "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.AddBranch(c.Context(), secondGenerationUUID, "second", "admin")
	c.Assert(err, tc.ErrorIsNil)

	c.Check(s.state.TrackUnits(c.Context(), firstGenerationUUID, []string{firstUnitUUID}), tc.ErrorIsNil)
	c.Check(s.state.TrackUnits(c.Context(), secondGenerationUUID, []string{secondUnitUUID}), tc.ErrorIsNil)
}

func (s *stateSuite) TestApplicationCannotBeOwnedByTwoBranches(c *tc.C) {
	appUUID, firstUnitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	secondUnitUUID := s.createUnitForApplication(c, appUUID, "mediawiki/1")
	firstGenerationUUID := s.newUUID(c)
	secondGenerationUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), firstGenerationUUID, "first", "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.AddBranch(c.Context(), secondGenerationUUID, "second", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(s.state.TrackUnits(c.Context(), firstGenerationUUID, []string{firstUnitUUID}), tc.ErrorIsNil)

	err = s.state.TrackUnits(c.Context(), secondGenerationUUID, []string{secondUnitUUID})
	c.Check(err, tc.ErrorIs, generationerrors.ApplicationAlreadyOwned)
	names, err := s.state.GetTrackedUnitNames(c.Context(), secondGenerationUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(names, tc.HasLen, 0)
}

func (s *stateSuite) TestApplicationOwnershipSurvivesUntracking(c *tc.C) {
	_, unitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	firstGenerationUUID := s.newUUID(c)
	secondGenerationUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), firstGenerationUUID, "first", "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.AddBranch(c.Context(), secondGenerationUUID, "second", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(s.state.TrackUnits(c.Context(), firstGenerationUUID, []string{unitUUID}), tc.ErrorIsNil)
	c.Assert(s.state.UntrackUnits(c.Context(), firstGenerationUUID, []string{unitUUID}), tc.ErrorIsNil)

	err = s.state.TrackUnits(c.Context(), secondGenerationUUID, []string{unitUUID})
	c.Check(err, tc.ErrorIs, generationerrors.ApplicationAlreadyOwned)
}

func (s *stateSuite) TestAbortReleasesApplicationOwnership(c *tc.C) {
	_, unitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	firstGenerationUUID := s.newUUID(c)
	secondGenerationUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), firstGenerationUUID, "first", "admin")
	c.Assert(err, tc.ErrorIsNil)
	_, err = s.state.AddBranch(c.Context(), secondGenerationUUID, "second", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(s.state.TrackUnits(c.Context(), firstGenerationUUID, []string{unitUUID}), tc.ErrorIsNil)
	c.Assert(s.state.Abort(c.Context(), firstGenerationUUID, "admin"), tc.ErrorIsNil)

	c.Check(s.state.TrackUnits(c.Context(), secondGenerationUUID, []string{unitUUID}), tc.ErrorIsNil)
}

func (s *stateSuite) TestTrackAndUntrackNoUnits(c *tc.C) {
	c.Assert(s.state.TrackUnits(c.Context(), "missing", nil), tc.ErrorIsNil)
	c.Assert(s.state.UntrackUnits(c.Context(), "missing", nil), tc.ErrorIsNil)
}

func (s *stateSuite) TestUntrackUnits(c *tc.C) {
	_, firstUnitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")
	_, secondUnitUUID := s.createUnit(c, "wordpress", "wordpress/0")
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(s.state.TrackUnits(c.Context(), genUUID, []string{secondUnitUUID, firstUnitUUID}), tc.ErrorIsNil)

	err = s.state.UntrackUnits(c.Context(), genUUID, []string{firstUnitUUID})
	c.Assert(err, tc.ErrorIsNil)

	names, err := s.state.GetTrackedUnitNames(c.Context(), genUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(names, tc.DeepEquals, []string{"wordpress/0"})

	tracked, err := s.state.HasTrackedUnits(c.Context(), genUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(tracked, tc.IsTrue)

	c.Assert(s.state.UntrackUnits(c.Context(), genUUID, []string{secondUnitUUID}), tc.ErrorIsNil)
	tracked, err = s.state.HasTrackedUnits(c.Context(), genUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(tracked, tc.IsFalse)
}

func (s *stateSuite) TestGetBranchForUnitUntracked(c *tc.C) {
	_, unitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")

	_, err := s.state.GetBranchForUnit(c.Context(), unitUUID)
	c.Assert(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *stateSuite) TestAbort(c *tc.C) {
	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.Abort(c.Context(), genUUID, "admin")
	c.Assert(err, tc.ErrorIsNil)

	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Assert(err, tc.ErrorIs, generationerrors.BranchNotFound)

	var stateID int
	var completedBy string
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT state_id, completed_by FROM generation WHERE uuid = ?`, genUUID).Scan(&stateID, &completedBy)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(stateID, tc.Equals, stateIDAborted)
	c.Check(completedBy, tc.Equals, "admin")
}

func (s *stateSuite) TestAbortNotFoundOrAlreadyCompleted(c *tc.C) {
	c.Assert(s.state.Abort(c.Context(), s.newUUID(c), "admin"), tc.ErrorIs, generationerrors.BranchNotFound)

	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(s.state.Abort(c.Context(), genUUID, "admin"), tc.ErrorIsNil)
	c.Assert(s.state.Abort(c.Context(), genUUID, "admin"), tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *stateSuite) TestAbortClearsTrackedUnits(c *tc.C) {
	_, unitUUID := s.createUnit(c, "mediawiki", "mediawiki/0")

	genUUID := s.newUUID(c)
	_, err := s.state.AddBranch(c.Context(), genUUID, "test", "admin")
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.TrackUnits(c.Context(), genUUID, []string{unitUUID})
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.Abort(c.Context(), genUUID, "admin")
	c.Assert(err, tc.ErrorIsNil)

	tracked, err := s.state.HasTrackedUnits(c.Context(), genUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(tracked, tc.IsFalse)
	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
}

func (s *stateSuite) TestAbortRollsBackWhenTrackingClearFails(c *tc.C) {
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

	err = s.state.Abort(c.Context(), genUUID, "admin")
	c.Check(err, tc.ErrorMatches, `clearing tracked units: .*tracking unavailable.*`)

	tracked, err := s.state.HasTrackedUnits(c.Context(), genUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(tracked, tc.IsTrue)
	_, err = s.state.GetBranchByName(c.Context(), "test")
	c.Check(err, tc.ErrorIsNil)
}

func (s *stateSuite) TestDecodeGenerationRow(c *tc.C) {
	now := time.Now().UTC()
	got, err := decodeGenerationRow(generationRow{
		UUID:         "uuid",
		GenerationID: 42,
		Name:         "test",
		StateID:      stateIDCommitted,
		CreatedBy:    "creator",
		CreatedAt:    now,
		CompletedBy:  sql.NullString{String: "committer", Valid: true},
		CompletedAt:  sql.NullTime{Time: now.Add(time.Hour), Valid: true},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(got.State, tc.Equals, string(generation.StateCommitted))
	c.Check(got.CompletedBy, tc.Equals, "committer")
	c.Check(got.CompletedAt, tc.Equals, now.Add(time.Hour))

	_, err = decodeGenerationRow(generationRow{UUID: "uuid", StateID: 99})
	c.Check(err, tc.ErrorMatches, `decoding generation "uuid": unknown generation state id 99`)
}

func (s *stateSuite) TestDecodeState(c *tc.C) {
	tests := []struct {
		id   int
		want generation.State
	}{
		{stateIDInFlight, generation.StateInFlight},
		{stateIDCommitted, generation.StateCommitted},
		{stateIDAborted, generation.StateAborted},
	}
	for _, test := range tests {
		got, err := decodeState(test.id)
		c.Assert(err, tc.ErrorIsNil)
		c.Check(got, tc.Equals, string(test.want))
	}

	_, err := decodeState(99)
	c.Check(err, tc.ErrorMatches, `unknown generation state id 99`)
}
