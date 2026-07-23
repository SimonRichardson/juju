// Copyright 2022 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package database

import (
	"context"
	"database/sql"

	"github.com/canonical/sqlair"

	coredatabase "github.com/juju/juju/core/database"
	coreerrors "github.com/juju/juju/core/errors"
	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/domain/schema"
	"github.com/juju/juju/internal/database/app"
	"github.com/juju/juju/internal/database/pragma"
	"github.com/juju/juju/internal/errors"
)

// BootstrapNodeManager is an interface for managing the bootstrap of a Dqlite
// node.
type BootstrapNodeManager interface {
	// ForDqliteApplication returns an isolated node manager for a model
	// Dqlite application.
	ForDqliteApplication(int) BootstrapNodeManager

	// EnsureDataDir ensures that a directory for Dqlite data exists at
	// a path determined by the agent config, then returns that path.
	EnsureDataDir() (string, error)

	// PrepareBootstrapNode gives a new standalone application a unique node
	// identity before it is started.
	PrepareBootstrapNode(context.Context) error

	// IsLoopbackPreferred returns true if the Dqlite application should
	// be bound to the loopback address.
	IsLoopbackPreferred() bool

	// WithLoopbackAddressOption returns a Dqlite application
	// Option that will bind Dqlite to the loopback IP.
	WithLoopbackAddressOption() app.Option

	// WithPreferredCloudLocalAddressOption uses the input network config
	// source to return a local-cloud address to which to bind Dqlite,
	// provided that a unique one can be determined.
	WithPreferredCloudLocalAddressOption(network.ConfigSource) (app.Option, error)

	// WithTLSOption returns a Dqlite application Option for TLS encryption
	// of traffic between clients and clustered application nodes.
	WithTLSOption() (app.Option, error)

	// WithLogFuncOption returns a Dqlite application Option
	// that will proxy Dqlite log output via this factory's
	// logger where the level is recognised.
	WithLogFuncOption() app.Option

	// WithTracingOption returns a Dqlite application Option
	// that will enable tracing of Dqlite operations.
	WithTracingOption() app.Option
}

// BootstrapOpt is a function run when bootstrapping a database,
// used to insert initial data into the model.
type BootstrapOpt func(
	ctx context.Context,
	controller, model coredatabase.TxnRunner,
) error

// BootstrapDqlite opens a new database for the controller, and runs the
// DDL to create its schema.
//
// It accepts an optional list of functions to perform operations on the
// controller database.
func BootstrapDqlite(
	ctx context.Context,
	mgr BootstrapNodeManager,
	uuid model.UUID,
	modelApplicationCount int,
	logger logger.Logger,
	opts ...BootstrapOpt,
) error {
	dir, err := mgr.EnsureDataDir()
	if err != nil {
		return errors.Capture(err)
	}

	options := []app.Option{mgr.WithLogFuncOption()}
	if mgr.IsLoopbackPreferred() {
		options = append(options, mgr.WithLoopbackAddressOption())
	} else {
		addrOpt, err := mgr.WithPreferredCloudLocalAddressOption(network.DefaultConfigSource())
		if err != nil {
			return errors.Errorf("generating bind address option: %w", err)
		}

		tlsOpt, err := mgr.WithTLSOption()
		if err != nil {
			return errors.Errorf("generating TLS option: %w", err)
		}

		options = append(options, addrOpt, tlsOpt)
	}

	dqlite, err := app.New(dir, options...)
	if err != nil {
		return errors.Errorf("creating Dqlite app: %w", err)
	}
	defer func() {
		if err := dqlite.Close(); err != nil {
			logger.Errorf(ctx, "closing Dqlite: %v", err)
		}
	}()

	if err := dqlite.Ready(ctx); err != nil {
		return errors.Errorf("waiting for Dqlite readiness: %w", err)
	}

	controller, err := runMigration(ctx, dqlite, coredatabase.ControllerNS, schema.ControllerDDL(), controllerBootstrapInit, logger)
	if err != nil {
		return errors.Errorf("running controller migration: %w", err)
	}

	if modelApplicationCount < 1 {
		return errors.Errorf(
			"model Dqlite application count %d: %w",
			modelApplicationCount, coreerrors.NotValid,
		)
	}

	modelApps := make([]*app.App, 0, modelApplicationCount)
	defer func() {
		for _, modelApp := range modelApps {
			if err := modelApp.Close(); err != nil {
				logger.Errorf(ctx, "closing model Dqlite application: %v", err)
			}
		}
	}()
	for applicationID := 1; applicationID <= modelApplicationCount; applicationID++ {
		applicationManager := mgr.ForDqliteApplication(applicationID)
		applicationDir, err := applicationManager.EnsureDataDir()
		if err != nil {
			return errors.Errorf("ensuring model Dqlite application %d data directory: %w", applicationID, err)
		}

		applicationOptions := []app.Option{applicationManager.WithLogFuncOption()}
		if applicationManager.IsLoopbackPreferred() {
			applicationOptions = append(applicationOptions, applicationManager.WithLoopbackAddressOption())
		} else {
			addrOpt, err := applicationManager.WithPreferredCloudLocalAddressOption(network.DefaultConfigSource())
			if err != nil {
				return errors.Errorf("generating model Dqlite application %d bind address: %w", applicationID, err)
			}
			tlsOpt, err := applicationManager.WithTLSOption()
			if err != nil {
				return errors.Errorf("generating model Dqlite application %d TLS option: %w", applicationID, err)
			}
			applicationOptions = append(applicationOptions, addrOpt, tlsOpt)
		}
		if err := applicationManager.PrepareBootstrapNode(ctx); err != nil {
			return errors.Errorf("preparing model Dqlite application %d node: %w", applicationID, err)
		}

		modelApp, err := app.New(applicationDir, applicationOptions...)
		if err != nil {
			return errors.Errorf("creating model Dqlite application %d: %w", applicationID, err)
		}
		if err := modelApp.Ready(ctx); err != nil {
			_ = modelApp.Close()
			return errors.Errorf("waiting for model Dqlite application %d readiness: %w", applicationID, err)
		}
		modelApps = append(modelApps, modelApp)
	}

	// The controller model is allocated by the same least-filled policy as
	// every later model, so on an empty controller it belongs to application 1.
	model, err := runMigration(ctx, modelApps[0], uuid.String(), schema.ModelDDL(), emptyInit, logger)
	if err != nil {
		return errors.Errorf("running model migration: %w", err)
	}

	for i, op := range opts {
		if err := op(ctx, controller, model); err != nil {
			return errors.Errorf("running bootstrap operation at index %d: %w", i, err)
		}
	}

	return nil
}

func runMigration(ctx context.Context, dqlite *app.App, namespace string, schema Schema, init bootstrapInit, logger logger.Logger) (coredatabase.TxnRunner, error) {
	db, err := dqlite.Open(ctx, namespace)
	if err != nil {
		return nil, errors.Errorf("opening database for namespace %q: %w", namespace, err)
	}

	if err := pragma.SetPragma(ctx, db, pragma.ForeignKeysPragma, true); err != nil {
		return nil, errors.Errorf("setting foreign keys pragma for namespace %q: %w", namespace, err)
	}

	runner := &txnRunner{db: db}

	migration := NewDBMigration(runner, logger, schema)
	if err := migration.Apply(ctx); err != nil {
		return nil, errors.Errorf("creating database with namespace %q schema: %w", namespace, err)
	}

	if err := init(ctx, runner, dqlite); err != nil {
		return nil, errors.Errorf("running init for database with namespace %q: %w", namespace, err)
	}

	return runner, nil
}

// InsertControllerNodeID inserts the node ID of the controller node
// into the controller_node table.
func InsertControllerNodeID(ctx context.Context, runner coredatabase.TxnRunner, nodeID uint64) error {
	q := `
-- TODO (manadart 2023-06-06): At the time of writing, 
-- we have not yet modelled machines. 
-- Accordingly, the controller ID remains the ID of the machine, 
-- but it should probably become a UUID once machines have one.
-- While HA is not supported in K8s, this doesn't matter.
INSERT INTO controller_node (controller_id, dqlite_node_id, dqlite_bind_address)
VALUES ('0', ?, '127.0.0.1');`
	return runner.StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, q, nodeID)
		if err != nil {
			return errors.Capture(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return errors.Capture(err)
		}
		if affected != 1 {
			return errors.Errorf("expected 1 row affected, got %d", affected)
		}
		return nil
	})
}

// txnRunner is the simplest implementation of TxnRunner, wrapping a
// sql.DB reference. It is recruited to run the bootstrap DB migration,
// where we do not yet have access to a transaction runner sourced from
// dbaccessor worker.
type txnRunner struct {
	db *sql.DB
}

func (r *txnRunner) Txn(ctx context.Context, f func(context.Context, *sqlair.TX) error) error {
	return errors.Capture(Txn(ctx, sqlair.NewDB(r.db), f))
}

func (r *txnRunner) StdTxn(ctx context.Context, f func(context.Context, *sql.Tx) error) error {
	return errors.Capture(StdTxn(ctx, r.db, f))
}

func (r *txnRunner) Dying() <-chan struct{} {
	return make(<-chan struct{})
}

// bootstrapInit is a type for describing a bootstrap operation that
// initialises a database.
type bootstrapInit = func(ctx context.Context, runner coredatabase.TxnRunner, dqlite *app.App) error

// controllerBootstrapInit is used to initialise the controller database with
// a controller node ID. The controller node ID is required to be present in
// the controller_node table as this is used for referential integrity.
func controllerBootstrapInit(ctx context.Context, runner coredatabase.TxnRunner, dqlite *app.App) error {
	if err := InsertControllerNodeID(ctx, runner, dqlite.ID()); err != nil {
		// If the controller node ID already exists, we assume that
		// the database has already been bootstrapped. Mask the unique
		// constraint error with a more user-friendly error.
		if IsErrConstraintUnique(err) {
			return errors.Errorf("controller node ID: %w", coreerrors.AlreadyExists)
		}
		return errors.Errorf("inserting controller node ID: %w", err)
	}
	return nil
}

// emptyInit is a BootstrapInit type that does nothing.
func emptyInit(context.Context, coredatabase.TxnRunner, *app.App) error {
	return nil
}
