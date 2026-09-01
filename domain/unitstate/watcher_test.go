// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package unitstate_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/juju/clock"
	"github.com/juju/tc"

	"github.com/juju/juju/core/changestream"
	"github.com/juju/juju/core/database"
	"github.com/juju/juju/core/model"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/core/watcher/watchertest"
	"github.com/juju/juju/domain"
	"github.com/juju/juju/domain/application"
	"github.com/juju/juju/domain/application/architecture"
	"github.com/juju/juju/domain/application/charm"
	applicationstate "github.com/juju/juju/domain/application/state"
	"github.com/juju/juju/domain/deployment"
	domainnetwork "github.com/juju/juju/domain/network"
	unitstateservice "github.com/juju/juju/domain/unitstate/service"
	unitstatestate "github.com/juju/juju/domain/unitstate/state"
	changestreamtesting "github.com/juju/juju/internal/changestream/testing"
	"github.com/juju/juju/internal/errors"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	internaltesting "github.com/juju/juju/internal/testing"
	"github.com/juju/juju/internal/uuid"
)

type snapshotWatcherSuite struct {
	changestreamtesting.ModelSuite

	modelUUID             model.UUID
	applicationUUID       string
	otherApplicationUUID  string
	unitUUID              string
	otherUnitUUID         string
	charmUUID             string
	otherCharmUUID        string
	netNodeUUID           string
	relationUUID          string
	relationEndpointUUID  string
	relationUnitUUID      string
	storageAttachmentUUID string
}

func TestSnapshotWatcherSuite(t *testing.T) {
	tc.Run(t, &snapshotWatcherSuite{})
}

func (s *snapshotWatcherSuite) SetUpTest(c *tc.C) {
	s.ModelSuite.SetUpTest(c)

	s.modelUUID = tc.Must(c, model.NewUUID)
	s.exec(c, `
INSERT INTO model (uuid, controller_uuid, name, qualifier, type, cloud, cloud_type)
VALUES (?, ?, 'test', 'prod', 'caas', 'test-model', 'kubernetes')`,
		s.modelUUID, internaltesting.ControllerTag.Id())

	appState := applicationstate.NewState(
		s.TxnRunnerFactory(), s.modelUUID, clock.WallClock,
		loggertesting.WrapCheckLog(c),
	)
	s.applicationUUID, s.unitUUID = s.createApplication(c, appState, "foo")
	s.otherApplicationUUID, s.otherUnitUUID = s.createApplication(c, appState, "bar")

	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx,
			"SELECT charm_uuid, net_node_uuid FROM unit WHERE uuid = ?", s.unitUUID,
		).Scan(&s.charmUUID, &s.netNodeUUID); err != nil {
			return errors.Capture(err)
		}
		if err := tx.QueryRowContext(ctx,
			"SELECT charm_uuid FROM unit WHERE uuid = ?", s.otherUnitUUID,
		).Scan(&s.otherCharmUUID); err != nil {
			return errors.Capture(err)
		}
		return nil
	})
	c.Assert(err, tc.ErrorIsNil)

	s.setupRelation(c)
	s.setupAddress(c)
	s.setupStorage(c)
	s.exec(c, "INSERT INTO application_status (application_uuid, status_id, message) VALUES (?, 0, 'initial')",
		s.applicationUUID)
}

func (s *snapshotWatcherSuite) TestWatchUnitSnapshot(c *tc.C) {
	modelDB := func(context.Context) (database.TxnRunner, error) {
		return s.ModelTxnRunner(), nil
	}
	factory := changestream.NewWatchableDBFactoryForNamespace(
		s.GetWatchableDB, "unitstate-snapshot",
	)
	svc := unitstateservice.NewLeadershipService(
		unitstatestate.NewState(modelDB, clock.WallClock, loggertesting.WrapCheckLog(c)),
		nil, nil, nil, clock.WallClock, loggertesting.WrapCheckLog(c),
		domain.NewWatcherFactory(factory, loggertesting.WrapCheckLog(c)),
	)

	s.AssertChangeStreamIdle(c, "before watcher start")
	w, err := svc.WatchUnitSnapshot(c.Context(), coreunit.Name("foo/0"))
	c.Assert(err, tc.ErrorIsNil)

	harness := watchertest.NewHarness(s, watchertest.NewWatcherC(c, w))
	assertChange := func(query string, args ...any) {
		harness.AddTest(c, func(c *tc.C) {
			s.exec(c, query, args...)
		}, func(w watchertest.WatcherC[struct{}]) {
			w.AssertChange()
		})
	}

	assertChange("UPDATE unit SET life_id = 1 WHERE uuid = ?", s.unitUUID)
	assertChange("INSERT INTO unit_principal (unit_uuid, principal_uuid) VALUES (?, ?)",
		s.otherUnitUUID, s.unitUUID)
	assertChange("INSERT INTO unit_resolved (unit_uuid, mode_id) VALUES (?, 0)", s.unitUUID)
	assertChange("UPDATE application SET charm_modified_version = 1 WHERE uuid = ?",
		s.applicationUUID)
	assertChange("UPDATE application_config_hash SET sha256 = 'config-hash' WHERE application_uuid = ?",
		s.applicationUUID)
	assertChange("UPDATE application_setting SET trust = TRUE WHERE application_uuid = ?",
		s.applicationUUID)
	assertChange("UPDATE application_scale SET scale_target = 2 WHERE application_uuid = ?",
		s.applicationUUID)
	assertChange("UPDATE charm SET version = '2.0' WHERE uuid = ?", s.charmUUID)
	assertChange("UPDATE ip_address SET address_value = '10.0.0.2/24' WHERE net_node_uuid = ?",
		s.netNodeUUID)
	assertChange("UPDATE relation SET life_id = 1 WHERE uuid = ?", s.relationUUID)
	assertChange("DELETE FROM relation_unit WHERE uuid = ?", s.relationUnitUUID)
	assertChange(`INSERT INTO relation_unit (uuid, relation_endpoint_uuid, unit_uuid)
VALUES (?, ?, ?)`, s.relationUnitUUID, s.relationEndpointUUID, s.unitUUID)
	assertChange(`INSERT INTO relation_unit_settings_hash (relation_unit_uuid, sha256)
VALUES (?, 'unit-settings-hash')`, s.relationUnitUUID)
	assertChange(`INSERT INTO relation_application_settings_hash (relation_endpoint_uuid, sha256)
VALUES (?, 'application-settings-hash')`, s.relationEndpointUUID)
	assertChange("INSERT INTO unit_state_charm (unit_uuid, key, value) VALUES (?, 'key', 'value')",
		s.unitUUID)
	assertChange("UPDATE unit_workload_version SET version = '2.0' WHERE unit_uuid = ?",
		s.unitUUID)
	assertChange(`INSERT INTO unit_workload_status (unit_uuid, status_id, message)
VALUES (?, 0, 'changed')`, s.unitUUID)
	assertChange("UPDATE application_status SET message = 'changed' WHERE application_uuid = ?",
		s.applicationUUID)
	assertChange(`INSERT INTO port_range (uuid, protocol_id, from_port, to_port, unit_uuid)
VALUES (?, 1, 8080, 8080, ?)`, uuid.MustNewUUID().String(), s.unitUUID)
	assertChange("UPDATE storage_attachment SET life_id = 1 WHERE uuid = ?",
		s.storageAttachmentUUID)
	assertChange(`INSERT INTO storage_attachment
    (uuid, storage_instance_uuid, unit_uuid, life_id)
VALUES ('storage-attachment-2', 'storage-instance-2', ?, 0)`, s.unitUUID)

	harness.AddTest(c, func(c *tc.C) {
		s.exec(c, "UPDATE application_scale SET scale_target = 2 WHERE application_uuid = ?",
			s.otherApplicationUUID)
	}, func(w watchertest.WatcherC[struct{}]) {
		w.AssertNoChange()
	})
	harness.AddTest(c, func(c *tc.C) {
		s.exec(c, "UPDATE charm SET version = '2.0' WHERE uuid = ?", s.otherCharmUUID)
	}, func(w watchertest.WatcherC[struct{}]) {
		w.AssertNoChange()
	})

	harness.Run(c, struct{}{})
}

func (s *snapshotWatcherSuite) createApplication(
	c *tc.C, appState *applicationstate.State, name string,
) (string, string) {
	unitUUID := tc.Must(c, coreunit.NewUUID)
	netNodeUUID := tc.Must(c, domainnetwork.NewNetNodeUUID)
	appUUID, err := appState.CreateCAASApplication(c.Context(), name,
		application.AddCAASApplicationArg{
			BaseAddApplicationArg: application.BaseAddApplicationArg{
				Platform: deployment.Platform{
					Channel:      "22.04/stable",
					OSType:       deployment.Ubuntu,
					Architecture: architecture.AMD64,
				},
				Charm: charm.Charm{
					Metadata: charm.Metadata{
						Name: name,
						Provides: map[string]charm.Relation{
							"endpoint": {
								Name:  "endpoint",
								Role:  charm.RoleProvider,
								Scope: charm.ScopeGlobal,
							},
						},
					},
					Manifest: charm.Manifest{Bases: []charm.Base{{
						Name:          "ubuntu",
						Channel:       charm.Channel{Risk: charm.RiskStable},
						Architectures: []string{"amd64"},
					}}},
					ReferenceName: name,
					Source:        charm.CharmHubSource,
					Revision:      1,
					Hash:          name + "-hash",
				},
			},
			Scale: 1,
		}, []application.AddCAASUnitArg{{AddUnitArg: application.AddUnitArg{
			UnitUUID:    unitUUID,
			NetNodeUUID: netNodeUUID,
		}}})
	c.Assert(err, tc.ErrorIsNil)
	return appUUID.String(), unitUUID.String()
}

func (s *snapshotWatcherSuite) setupRelation(c *tc.C) {
	s.relationUUID = "relation-uuid"
	s.relationEndpointUUID = "relation-endpoint-uuid"
	s.relationUnitUUID = "relation-unit-uuid"

	var endpointUUID, otherEndpointUUID string
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx,
			"SELECT uuid FROM application_endpoint WHERE application_uuid = ?",
			s.applicationUUID,
		).Scan(&endpointUUID); err != nil {
			return errors.Capture(err)
		}
		if err := tx.QueryRowContext(ctx,
			"SELECT uuid FROM application_endpoint WHERE application_uuid = ?",
			s.otherApplicationUUID,
		).Scan(&otherEndpointUUID); err != nil {
			return errors.Capture(err)
		}
		return nil
	})
	c.Assert(err, tc.ErrorIsNil)

	s.exec(c, "INSERT INTO relation (uuid, life_id, relation_id, scope_id) VALUES (?, 0, 0, 0)",
		s.relationUUID)
	s.exec(c, `INSERT INTO relation_endpoint (uuid, relation_uuid, endpoint_uuid)
VALUES (?, ?, ?)`, s.relationEndpointUUID, s.relationUUID, endpointUUID)
	s.exec(c, `INSERT INTO relation_endpoint (uuid, relation_uuid, endpoint_uuid)
VALUES ('other-relation-endpoint-uuid', ?, ?)`, s.relationUUID, otherEndpointUUID)
	s.exec(c, `INSERT INTO relation_unit (uuid, relation_endpoint_uuid, unit_uuid)
VALUES (?, ?, ?)`, s.relationUnitUUID, s.relationEndpointUUID, s.unitUUID)
}

func (s *snapshotWatcherSuite) setupAddress(c *tc.C) {
	s.exec(c, `INSERT INTO link_layer_device
    (uuid, net_node_uuid, name, device_type_id, virtual_port_type_id)
VALUES ('device-uuid', ?, 'eth0', 0, 0)`, s.netNodeUUID)
	s.exec(c, `INSERT INTO ip_address
    (uuid, net_node_uuid, device_uuid, address_value, type_id, config_type_id,
     origin_id, scope_id)
VALUES ('address-uuid', ?, 'device-uuid', '10.0.0.1/24', 0, 4, 1, 2)`,
		s.netNodeUUID)
}

func (s *snapshotWatcherSuite) setupStorage(c *tc.C) {
	s.storageAttachmentUUID = "storage-attachment-uuid"
	s.exec(c, "INSERT INTO storage_pool (uuid, name, type) VALUES ('pool-uuid', 'pool', 'loop')")
	s.exec(c, `
INSERT INTO storage_instance
    (uuid, charm_name, storage_name, storage_kind_id, storage_id, life_id,
     storage_pool_uuid, requested_size_mib)
VALUES ('storage-instance-uuid', 'foo', 'data', 1, 'data/0', 0, 'pool-uuid', 1024)`)
	s.exec(c, "INSERT INTO storage_instance (uuid, charm_name, storage_name, storage_kind_id, storage_id, life_id, storage_pool_uuid, requested_size_mib) VALUES ('storage-instance-2', 'foo', 'data', 1, 'data/1', 0, 'pool-uuid', 1024)")
	s.exec(c, `INSERT INTO storage_attachment
    (uuid, storage_instance_uuid, unit_uuid, life_id)
VALUES (?, 'storage-instance-uuid', ?, 0)`, s.storageAttachmentUUID, s.unitUUID)
}

func (s *snapshotWatcherSuite) exec(c *tc.C, query string, args ...any) {
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, query, args...)
		return errors.Capture(err)
	})
	c.Assert(err, tc.ErrorIsNil, tc.Commentf("query: %s", query))
}
