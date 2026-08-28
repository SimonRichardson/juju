// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"

	"github.com/canonical/sqlair"
	"github.com/juju/clock"

	"github.com/juju/juju/core/database"
	"github.com/juju/juju/core/logger"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	"github.com/juju/juju/domain/unitstate"
	"github.com/juju/juju/internal/errors"
)

// State implements persistence for unit state.
type State struct {
	*domain.StateBase

	clock  clock.Clock
	logger logger.Logger
}

// NewState returns a new state reference.
func NewState(factory database.TxnRunnerFactory, clock clock.Clock, logger logger.Logger) *State {
	return &State{
		StateBase: domain.NewStateBase(factory),
		clock:     clock,
		logger:    logger,
	}
}

// GetUnitSnapshotWatchIdentifiers returns stable identifiers for every model
// entity currently represented by the named unit's snapshot.
func (st *State) GetUnitSnapshotWatchIdentifiers(ctx context.Context, name coreunit.Name) (unitstate.SnapshotWatchIdentifiers, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return unitstate.SnapshotWatchIdentifiers{}, errors.Capture(err)
	}

	ident := unitName{Name: name.String()}
	unitStmt, err := st.Prepare(`
SELECT u.uuid AS &unitSnapshotWatchIdentifier.unit_uuid,
       u.application_uuid AS &unitSnapshotWatchIdentifier.application_uuid,
       u.charm_uuid AS &unitSnapshotWatchIdentifier.charm_uuid
FROM unit AS u
WHERE u.name = $unitName.name
`, unitSnapshotWatchIdentifier{}, ident)
	if err != nil {
		return unitstate.SnapshotWatchIdentifiers{}, errors.Capture(err)
	}
	netNodesStmt, err := st.Prepare(`
SELECT &unitNetNodeUUID.*
FROM (
    SELECT s.net_node_uuid, u.name
    FROM unit AS u
    JOIN k8s_service AS s ON s.application_uuid = u.application_uuid
    UNION
    SELECT net_node_uuid, name FROM unit
) AS n
WHERE n.name = $unitName.name
`, unitNetNodeUUID{}, ident)
	if err != nil {
		return unitstate.SnapshotWatchIdentifiers{}, errors.Capture(err)
	}
	relationsStmt, err := st.Prepare(`
SELECT DISTINCT re.relation_uuid AS &relationSnapshotWatchIdentifier.relation_uuid,
       ru.uuid AS &relationSnapshotWatchIdentifier.relation_unit_uuid
FROM relation_unit AS ru
JOIN relation_endpoint AS re ON ru.relation_endpoint_uuid = re.uuid
JOIN unit AS u ON ru.unit_uuid = u.uuid
WHERE u.name = $unitName.name
`, relationSnapshotWatchIdentifier{}, ident)
	if err != nil {
		return unitstate.SnapshotWatchIdentifiers{}, errors.Capture(err)
	}
	relationEndpointsStmt, err := st.Prepare(`
SELECT DISTINCT related.uuid AS &entityUUID.*
FROM relation_unit AS ru
JOIN relation_endpoint AS local ON ru.relation_endpoint_uuid = local.uuid
JOIN relation_endpoint AS related ON related.relation_uuid = local.relation_uuid
JOIN unit AS u ON ru.unit_uuid = u.uuid
WHERE u.name = $unitName.name
`, entityUUID{}, ident)
	if err != nil {
		return unitstate.SnapshotWatchIdentifiers{}, errors.Capture(err)
	}

	var unit unitSnapshotWatchIdentifier
	var netNodes []unitNetNodeUUID
	var relations []relationSnapshotWatchIdentifier
	var relationEndpoints []entityUUID
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := tx.Query(ctx, unitStmt, ident).Get(&unit); err != nil {
			if errors.Is(err, sqlair.ErrNoRows) {
				return errors.Errorf("%w: %s", applicationerrors.UnitNotFound, name)
			}
			return errors.Capture(err)
		}
		if err := tx.Query(ctx, netNodesStmt, ident).GetAll(&netNodes); err != nil {
			return errors.Capture(err)
		}
		if err := tx.Query(ctx, relationsStmt, ident).GetAll(&relations); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Capture(err)
		}
		if err := tx.Query(ctx, relationEndpointsStmt, ident).GetAll(&relationEndpoints); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Capture(err)
		}
		return nil
	})
	if err != nil {
		return unitstate.SnapshotWatchIdentifiers{}, errors.Capture(err)
	}

	result := unitstate.SnapshotWatchIdentifiers{
		UnitUUID:              unit.UnitUUID,
		ApplicationUUID:       unit.ApplicationUUID,
		CharmUUID:             unit.CharmUUID,
		NetNodeUUIDs:          make([]string, len(netNodes)),
		RelationUUIDs:         make([]string, len(relations)),
		RelationUnitUUIDs:     make([]string, len(relations)),
		RelationEndpointUUIDs: make([]string, len(relationEndpoints)),
	}
	for i, netNode := range netNodes {
		result.NetNodeUUIDs[i] = netNode.NetNodeUUID
	}
	for i, relation := range relations {
		result.RelationUUIDs[i] = relation.RelationUUID
		result.RelationUnitUUIDs[i] = relation.RelationUnitUUID
	}
	for i, endpoint := range relationEndpoints {
		result.RelationEndpointUUIDs[i] = endpoint.UUID
	}
	return result, nil
}

// GetUnitSnapshot returns the stable model projection for a unit snapshot.
// Additional snapshot collections are loaded by dedicated projection queries in
// the same transaction as this base row.
func (st *State) GetUnitSnapshot(ctx context.Context, name coreunit.Name) (unitstate.UnitSnapshot, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return unitstate.UnitSnapshot{}, errors.Capture(err)
	}
	storageStmt, err := st.Prepare(`
SELECT si.storage_id AS &unitSnapshotStorageRow.storage_id,
       si.storage_kind_id AS &unitSnapshotStorageRow.storage_kind_id,
       sa.life_id AS &unitSnapshotStorageRow.life_id,
       COALESCE(sfa.mount_point, bdld.name) AS &unitSnapshotStorageRow.location
FROM storage_attachment AS sa
JOIN storage_instance AS si ON si.uuid = sa.storage_instance_uuid
JOIN unit AS u ON u.uuid = sa.unit_uuid
LEFT JOIN storage_instance_filesystem AS sif ON sif.storage_instance_uuid = si.uuid
LEFT JOIN storage_filesystem_attachment AS sfa ON sfa.storage_filesystem_uuid = sif.storage_filesystem_uuid AND sfa.net_node_uuid = u.net_node_uuid
LEFT JOIN storage_instance_volume AS siv ON siv.storage_instance_uuid = si.uuid
LEFT JOIN storage_volume_attachment AS sva ON sva.storage_volume_uuid = siv.storage_volume_uuid AND sva.net_node_uuid = u.net_node_uuid
LEFT JOIN block_device_link_device AS bdld ON bdld.block_device_uuid = sva.block_device_uuid
WHERE sa.unit_uuid = $entityUUID.uuid
`, unitSnapshotStorageRow{}, entityUUID{})
	if err != nil {
		return unitstate.UnitSnapshot{}, errors.Capture(err)
	}

	ident := unitName{Name: name.String()}
	stmt, err := st.Prepare(`
SELECT u.uuid AS &unitSnapshotRow.unit_uuid,
       u.name AS &unitSnapshotRow.unit_name,
       a.uuid AS &unitSnapshotRow.application_uuid,
       a.name AS &unitSnapshotRow.application_name,
       c.uuid AS &unitSnapshotRow.charm_uuid,
       c.reference_name AS &unitSnapshotRow.charm_url,
       u.life_id AS &unitSnapshotRow.life_id,
       rm.name AS &unitSnapshotRow.resolved_mode,
       a.charm_modified_version AS &unitSnapshotRow.charm_modified_version,
       COALESCE(aps.trust, FALSE) AS &unitSnapshotRow.trust,
       uwv.version AS &unitSnapshotRow.workload_version
FROM unit AS u
JOIN application AS a ON a.uuid = u.application_uuid
JOIN charm AS c ON c.uuid = u.charm_uuid
LEFT JOIN application_setting AS aps ON aps.application_uuid = a.uuid
LEFT JOIN unit_workload_version AS uwv ON uwv.unit_uuid = u.uuid
LEFT JOIN unit_resolved AS ur ON ur.unit_uuid = u.uuid
LEFT JOIN resolve_mode AS rm ON rm.id = ur.mode_id
WHERE u.name = $unitName.name
`, unitSnapshotRow{}, ident)
	if err != nil {
		return unitstate.UnitSnapshot{}, errors.Capture(err)
	}
	charmStateStmt, err := st.Prepare(`
SELECT &unitCharmStateKeyVal.*
FROM unit_state_charm
WHERE unit_uuid = $entityUUID.uuid
`, unitCharmStateKeyVal{}, entityUUID{})
	if err != nil {
		return unitstate.UnitSnapshot{}, errors.Capture(err)
	}

	var row unitSnapshotRow
	var charmState []unitCharmStateKeyVal
	var storageRows []unitSnapshotStorageRow
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := tx.Query(ctx, stmt, ident).Get(&row); err != nil {
			if errors.Is(err, sqlair.ErrNoRows) {
				return errors.Errorf("%w: %s", applicationerrors.UnitNotFound, name)
			}
			return errors.Capture(err)
		}
		if err := tx.Query(ctx, charmStateStmt, entityUUID{UUID: row.UnitUUID}).GetAll(&charmState); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Capture(err)
		}
		if err := tx.Query(ctx, storageStmt, entityUUID{UUID: row.UnitUUID}).GetAll(&storageRows); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Capture(err)
		}
		return nil
	})
	if err != nil {
		return unitstate.UnitSnapshot{}, errors.Capture(err)
	}
	snapshot := unitstate.UnitSnapshot{
		UnitName:             row.UnitName,
		ApplicationName:      row.ApplicationName,
		ApplicationUUID:      row.ApplicationUUID,
		UnitUUID:             row.UnitUUID,
		CharmUUID:            row.CharmUUID,
		CharmURL:             row.CharmURL,
		LifeID:               row.LifeID,
		ResolvedMode:         row.ResolvedMode.String,
		CharmModifiedVersion: row.CharmModifiedVersion,
		Trust:                row.Trust,
		WorkloadVersion:      row.WorkloadVersion.String,
	}
	if len(charmState) > 0 {
		snapshot.CharmState = make(map[string]string, len(charmState))
		for _, entry := range charmState {
			snapshot.CharmState[entry.Key] = entry.Value
		}
	}
	if len(storageRows) > 0 {
		snapshot.Storage = make([]unitstate.StorageSnapshot, len(storageRows))
		for i, storage := range storageRows {
			snapshot.Storage[i] = unitstate.StorageSnapshot{
				ID:       storage.ID,
				KindID:   storage.KindID,
				LifeID:   storage.LifeID,
				Location: storage.Location.String,
			}
		}
	}
	return snapshot, nil
}

// GetUnitState returns the full unit state. The state may be
// empty.
// If no unit with the name exists, a [errors.UnitNotFound] error is returned.
func (st *State) GetUnitState(ctx context.Context, name string) (unitstate.RetrievedUnitState, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return unitstate.RetrievedUnitState{}, errors.Capture(err)
	}

	var state unitState
	q := "SELECT &unitState.* FROM unit_state WHERE unit_uuid = $entityUUID.uuid"
	unitStateStmt, err := st.Prepare(q, state, entityUUID{})
	if err != nil {
		return unitstate.RetrievedUnitState{}, errors.Errorf("preparing select unit state statement: %w", err)
	}

	var charmKVs []unitCharmStateKeyVal
	q = `
SELECT &unitCharmStateKeyVal.*
FROM unit_state_charm
WHERE unit_uuid = $entityUUID.uuid`
	charmStateStmt, err := st.Prepare(q, unitCharmStateKeyVal{}, entityUUID{})
	if err != nil {
		return unitstate.RetrievedUnitState{}, errors.Errorf("preparing select unit charm state statement: %w", err)
	}

	var relationKVs []unitRelationStateKeyVal
	q = `
SELECT &unitRelationStateKeyVal.*
FROM unit_state_relation
WHERE unit_uuid = $entityUUID.uuid`
	relationStateStmt, err := st.Prepare(q, unitRelationStateKeyVal{}, entityUUID{})
	if err != nil {
		return unitstate.RetrievedUnitState{}, errors.Errorf("preparing select unit relation state statement: %w", err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		id, err := st.getUnitUUIDForName(ctx, tx, name)
		if err != nil {
			return errors.Errorf("getting unit UUID for %q: %w", name, err)
		}

		err = tx.Query(ctx, unitStateStmt, id).Get(&state)
		if err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("getting unit state: %w", err)
		}

		err = tx.Query(ctx, charmStateStmt, id).GetAll(&charmKVs)
		if err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("getting unit charm state: %w", err)
		}

		err = tx.Query(ctx, relationStateStmt, id).GetAll(&relationKVs)
		if err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("getting unit relation state: %w", err)
		}

		return nil
	})
	if err != nil {
		return unitstate.RetrievedUnitState{}, err
	}

	unitState := unitstate.RetrievedUnitState{
		UniterState:  state.UniterState,
		StorageState: state.StorageState,
		SecretState:  state.SecretState,
	}
	if len(charmKVs) > 0 {
		unitState.CharmState = makeMapFromCharmUnitStateKeyVals(charmKVs)
	}
	if len(relationKVs) > 0 {
		unitState.RelationState = makeMapFromRelationUnitStateKeyVals(relationKVs)
	}

	return unitState, nil
}

func (st *State) SetUnitState(ctx context.Context, as unitstate.UnitState) error {
	db, err := st.DB(ctx)
	if err != nil {
		return errors.Capture(err)
	}

	return db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		uuid, err := st.getUnitUUIDForName(ctx, tx, as.Name)
		if err != nil {
			return errors.Errorf("getting unit UUID for %q: %w", as.Name, err)
		}

		if err = st.ensureUnitStateRecord(ctx, tx, uuid); err != nil {
			return errors.Errorf("ensuring state record for %q: %w", as.Name, err)
		}

		if as.UniterState != nil {
			if err = st.updateUnitStateUniter(ctx, tx, uuid, *as.UniterState); err != nil {
				return errors.Errorf("setting uniter state for %q: %w", as.Name, err)
			}
		}

		if as.StorageState != nil {
			if err = st.updateUnitStateStorage(ctx, tx, uuid, *as.StorageState); err != nil {
				return errors.Errorf("setting storage state for %q: %w", as.Name, err)
			}
		}

		if as.SecretState != nil {
			if err = st.updateUnitStateSecret(ctx, tx, uuid, *as.SecretState); err != nil {
				return errors.Errorf("setting secret state for %q: %w", as.Name, err)
			}
		}

		if as.CharmState != nil {
			if err = st.setUnitStateCharm(ctx, tx, uuid, *as.CharmState); err != nil {
				return errors.Errorf("setting charm state for %q: %w", as.Name, err)
			}
		}

		if as.RelationState != nil {
			if err = st.setUnitStateRelation(ctx, tx, uuid, *as.RelationState); err != nil {
				return errors.Errorf("setting relation state for %q: %w", as.Name, err)
			}
		}

		return nil
	})
}

// ensureUnitStateRecord ensures that there is a row in the unit_state table
// for the input unit UUID. This eliminates the need for upsert statements
// when updating state for uniter, storage and secrets.
func (st *State) ensureUnitStateRecord(ctx context.Context, tx *sqlair.TX, id entityUUID) error {
	q := "SELECT unit_uuid AS &entityUUID.uuid FROM unit_state WHERE unit_uuid = $entityUUID.uuid"
	rowStmt, err := st.Prepare(q, id)
	if err != nil {
		return errors.Errorf("preparing state row query: %w", err)
	}

	q = "INSERT INTO unit_state(unit_uuid) values ($entityUUID.uuid)"
	insertStmt, err := st.Prepare(q, id)
	if err != nil {
		return errors.Errorf("preparing state insert query: %w", err)
	}

	err = tx.Query(ctx, rowStmt, id).Get(&id)
	if err == nil {
		return nil
	} else if !errors.Is(err, sqlair.ErrNoRows) {
		return errors.Errorf("checking for state row: %w", err)
	}

	err = tx.Query(ctx, insertStmt, id).Run()
	if err != nil {
		return errors.Errorf("adding state row: %w", err)
	}
	return nil
}

// updateUnitStateUniter sets the input uniter
// state against the input unit UUID.
func (st *State) updateUnitStateUniter(ctx context.Context, tx *sqlair.TX, id entityUUID, state string) error {
	uSt := unitState{UniterState: state}

	q := "UPDATE unit_state SET uniter_state = $unitState.uniter_state WHERE unit_uuid = $entityUUID.uuid"
	stmt, err := st.Prepare(q, id, uSt)
	if err != nil {
		return errors.Errorf("preparing uniter state update query: %w", err)
	}

	return tx.Query(ctx, stmt, id, uSt).Run()
}

// updateUnitStateStorage sets the input storage
// state against the input unit UUID.
func (st *State) updateUnitStateStorage(ctx context.Context, tx *sqlair.TX, id entityUUID, state string) error {
	uSt := unitState{StorageState: state}

	q := "UPDATE unit_state SET storage_state = $unitState.storage_state WHERE unit_uuid = $entityUUID.uuid"
	stmt, err := st.Prepare(q, id, uSt)
	if err != nil {
		return errors.Errorf("preparing storage state update query: %w", err)
	}

	return tx.Query(ctx, stmt, id, uSt).Run()
}

// updateUnitStateSecret sets the input secret
// state against the input unit UUID.
func (st *State) updateUnitStateSecret(ctx context.Context, tx *sqlair.TX, id entityUUID, state string) error {
	uSt := unitState{SecretState: state}

	q := "UPDATE unit_state SET secret_state = $unitState.secret_state WHERE unit_uuid = $entityUUID.uuid"
	stmt, err := st.Prepare(q, id, uSt)
	if err != nil {
		return errors.Errorf("preparing secret state update query: %w", err)
	}

	return tx.Query(ctx, stmt, id, uSt).Run()
}

// setUnitStateCharm sets the input key/value pairs
// as the charm state for the input unit UUID.
func (st *State) setUnitStateCharm(ctx context.Context, tx *sqlair.TX, id entityUUID, state map[string]string) error {
	q := "DELETE from unit_state_charm WHERE unit_uuid = $entityUUID.uuid"
	dStmt, err := st.Prepare(q, id)
	if err != nil {
		return errors.Errorf("preparing charm state delete query: %w", err)
	}

	if err := tx.Query(ctx, dStmt, id).Run(); err != nil {
		return errors.Errorf("deleting unit charm state: %w", err)
	}

	keyVals := makeUnitCharmStateKeyVals(id, state)
	if len(keyVals) != 0 {
		q = "INSERT INTO unit_state_charm(*) VALUES ($unitCharmStateKeyVal.*)"
		iStmt, err := st.Prepare(q, keyVals[0])
		if err != nil {
			return errors.Errorf("preparing charm state insert query: %w", err)
		}

		if err := tx.Query(ctx, iStmt, keyVals).Run(); err != nil {
			return errors.Errorf("setting unit charm state: %w", err)
		}
	}
	return nil
}

// SetUnitStateRelation sets the input key/value pairs
// as the relation state for the input unit UUID.
func (st *State) setUnitStateRelation(ctx context.Context, tx *sqlair.TX, id entityUUID, state map[int]string) error {
	q := "DELETE from unit_state_relation WHERE unit_uuid = $entityUUID.uuid"
	dStmt, err := st.Prepare(q, id)
	if err != nil {
		return errors.Errorf("preparing relation state delete query: %w", err)
	}

	keyVals := makeUnitRelationStateKeyVals(id, state)

	if err := tx.Query(ctx, dStmt, id).Run(); err != nil {
		return errors.Errorf("deleting unit relation state: %w", err)
	}

	if len(keyVals) != 0 {
		q = "INSERT INTO unit_state_relation(*) VALUES ($unitRelationStateKeyVal.*)"
		iStmt, err := st.Prepare(q, keyVals[0])
		if err != nil {
			return errors.Errorf("preparing relation state insert query: %w", err)
		}

		if err := tx.Query(ctx, iStmt, keyVals).Run(); err != nil {
			return errors.Errorf("setting unit relation state: %w", err)
		}
	}
	return nil
}

func (st *State) getUnitUUIDForName(ctx context.Context, tx *sqlair.TX, name string) (entityUUID, error) {
	uName := unitName{Name: name}
	uuid := entityUUID{}

	q := "SELECT &entityUUID.uuid FROM unit WHERE name = $unitName.name"
	stmt, err := st.Prepare(q, uName, uuid)
	if err != nil {
		return entityUUID{}, errors.Errorf("preparing UUID query: %w", err)
	}

	err = tx.Query(ctx, stmt, uName).Get(&uuid)
	if errors.Is(err, sqlair.ErrNoRows) {
		return entityUUID{}, applicationerrors.UnitNotFound
	} else if err != nil {
		return entityUUID{}, errors.Errorf("getting unit UUID for %q: %w", name, err)
	}

	return uuid, nil
}

// GetModelUUID returns the UUID of the model for the unit state domain.
func (st *State) GetModelUUID(ctx context.Context) (string, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return "", errors.Capture(err)
	}

	var result entityUUID
	stmt, err := st.Prepare("SELECT &entityUUID.uuid FROM model", result)
	if err != nil {
		return "", errors.Capture(err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		return tx.Query(ctx, stmt).Get(&result)
	})
	if err != nil {
		return "", errors.Errorf("querying model UUID: %w", err)
	}
	return result.UUID, nil
}
