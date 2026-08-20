// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"

	"github.com/juju/tc"

	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/application"
	"github.com/juju/juju/domain/application/charm"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	generationerrors "github.com/juju/juju/domain/generation/errors"
)

func (s *applicationRefreshSuite) TestGenerationConfigUsesResolvedCharmAndTombstones(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{
		appName: "mediawiki",
		charmConfig: charm.Config{Options: map[string]charm.Option{
			"title": {Type: charm.OptionString, Default: "main-default"},
			"count": {Type: charm.OptionInt, Default: 1},
		}},
		applicationConfig: map[string]application.AddApplicationConfig{
			"title": {Type: charm.OptionString, Value: "main-value"},
		},
	})
	branchCharm := s.createCharm(c, createCharmArgs{
		name: "mediawiki-branch",
		charmConfig: charm.Config{Options: map[string]charm.Option{
			"title":       {Type: charm.OptionString, Default: "branch-default"},
			"count":       {Type: charm.OptionInt, Default: 2},
			"branch-only": {Type: charm.OptionBool, Default: true},
		}},
	})
	trackedUnit := s.addUnit(c, coreunit.Name("mediawiki/0"), appUUID)
	mainUnit := s.addUnit(c, coreunit.Name("mediawiki/1"), appUUID)
	generationUUID := s.createGeneration(c, "test", 0)
	s.trackUnit(c, generationUUID, trackedUnit)
	c.Assert(s.state.setGenerationCharm(c.Context(), "test", appUUID, branchCharm), tc.ErrorIsNil)
	resolvedCharm, _, err := s.state.GetCharmConfigForApplicationUpdate(c.Context(), appUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(resolvedCharm, tc.Equals, branchCharm)
	c.Assert(s.state.UpdateApplicationConfigAndSettings(c.Context(), appUUID, map[string]application.AddApplicationConfig{
		"title": {Type: charm.OptionString, Value: "branch-value"},
		"count": {Type: charm.OptionInt, Value: "3"},
	}, application.UpdateApplicationSettingsArg{}), tc.ErrorIsNil)
	c.Assert(s.state.UnsetApplicationConfigKeys(c.Context(), appUUID, []string{"title", "unknown"}), tc.ErrorIsNil)

	tracked, err := s.state.GetResolvedUnitApplicationConfigWithDefaults(c.Context(), trackedUnit)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(tracked, tc.DeepEquals, map[string]application.ApplicationConfig{
		"title":       {Type: charm.OptionString, Value: new("branch-default")},
		"count":       {Type: charm.OptionInt, Value: new("3")},
		"branch-only": {Type: charm.OptionBool, Value: new("true")},
	})

	main, err := s.state.GetResolvedUnitApplicationConfigWithDefaults(c.Context(), mainUnit)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(main, tc.DeepEquals, map[string]application.ApplicationConfig{
		"title": {Type: charm.OptionString, Value: new("main-value")},
		"count": {Type: charm.OptionInt, Value: new("1")},
	})

	var canonicalTitle string
	var branchRows, tombstones int
	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `
SELECT value FROM application_config
WHERE application_uuid = ? AND "key" = 'title'
`, appUUID.String()).Scan(&canonicalTitle); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM generation_application_config`).Scan(&branchRows); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM generation_application_config WHERE value IS NULL
`).Scan(&tombstones)
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Check(canonicalTitle, tc.Equals, "main-value")
	c.Check(branchRows, tc.Equals, 2)
	c.Check(tombstones, tc.Equals, 1)
}

func (s *applicationRefreshSuite) TestGenerationConfigApplicationsAreIsolated(c *tc.C) {
	firstApp := s.createApplication(c, createApplicationArgs{
		appName: "mediawiki",
		charmConfig: charm.Config{Options: map[string]charm.Option{
			"title": {Type: charm.OptionString, Default: "main"},
		}},
	})
	secondApp := s.createApplication(c, createApplicationArgs{
		appName: "wordpress",
		charmConfig: charm.Config{Options: map[string]charm.Option{
			"title": {Type: charm.OptionString, Default: "main"},
		}},
	})
	firstUnit := s.addUnit(c, coreunit.Name("mediawiki/0"), firstApp)
	secondUnit := s.addUnit(c, coreunit.Name("wordpress/0"), secondApp)
	generationUUID := s.createGeneration(c, "test", 0)
	s.trackUnit(c, generationUUID, firstUnit)
	s.trackUnit(c, generationUUID, secondUnit)
	c.Assert(s.state.UpdateApplicationConfigAndSettings(c.Context(), firstApp, map[string]application.AddApplicationConfig{
		"title": {Type: charm.OptionString, Value: "one"},
	}, application.UpdateApplicationSettingsArg{}), tc.ErrorIsNil)
	c.Assert(s.state.UpdateApplicationConfigAndSettings(c.Context(), secondApp, map[string]application.AddApplicationConfig{
		"title": {Type: charm.OptionString, Value: "two"},
	}, application.UpdateApplicationSettingsArg{}), tc.ErrorIsNil)

	first, err := s.state.GetResolvedUnitApplicationConfigWithDefaults(c.Context(), firstUnit)
	c.Assert(err, tc.ErrorIsNil)
	second, err := s.state.GetResolvedUnitApplicationConfigWithDefaults(c.Context(), secondUnit)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(*first["title"].Value, tc.Equals, "one")
	c.Check(*second["title"].Value, tc.Equals, "two")
}

func (s *applicationRefreshSuite) TestGenerationConfigBranchAndUnitErrors(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{
		appName: "mediawiki",
		charmConfig: charm.Config{Options: map[string]charm.Option{
			"title": {Type: charm.OptionString},
		}},
	})
	s.createGeneration(c, "committed", 1)
	config := map[string]application.AddApplicationConfig{
		"title": {Type: charm.OptionString, Value: "value"},
	}
	for _, branchName := range []string{"missing", "committed"} {
		err := s.state.setGenerationApplicationConfig(c.Context(), branchName, appUUID, config)
		c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
		err = s.state.unsetGenerationApplicationConfigKeys(c.Context(), branchName, appUUID, []string{"title"})
		c.Check(err, tc.ErrorIs, generationerrors.BranchNotFound)
	}

	_, err := s.state.GetResolvedUnitApplicationConfigWithDefaults(c.Context(), tc.Must(c, coreunit.NewUUID))
	c.Check(err, tc.ErrorIs, applicationerrors.UnitNotFound)
}

func (s *applicationRefreshSuite) TestApplicationConfigHashIncludesActiveBranch(c *tc.C) {
	appUUID := s.createApplication(c, createApplicationArgs{
		appName: "mediawiki",
		charmConfig: charm.Config{Options: map[string]charm.Option{
			"title": {Type: charm.OptionString, Default: "main"},
		}},
	})

	mainHash, err := s.state.GetApplicationConfigHash(c.Context(), appUUID)
	c.Assert(err, tc.ErrorIsNil)
	s.createGeneration(c, "test", 0)
	c.Assert(s.state.UpdateApplicationConfigAndSettings(
		c.Context(), appUUID,
		map[string]application.AddApplicationConfig{
			"title": {Type: charm.OptionString, Value: "branch"},
		},
		application.UpdateApplicationSettingsArg{},
	), tc.ErrorIsNil)

	branchHash, err := s.state.GetApplicationConfigHash(c.Context(), appUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(branchHash, tc.Not(tc.Equals), mainHash)

	branchHashAgain, err := s.state.GetApplicationConfigHash(c.Context(), appUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(branchHashAgain, tc.Equals, branchHash)

	c.Assert(s.state.UnsetApplicationConfigKeys(
		c.Context(), appUUID, []string{"title"},
	), tc.ErrorIsNil)
	tombstoneHash, err := s.state.GetApplicationConfigHash(c.Context(), appUUID)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(tombstoneHash, tc.Not(tc.Equals), branchHash)
}
