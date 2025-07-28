// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package objectstore

import (
	"context"
	"io"

	"github.com/juju/errors"
	"github.com/juju/worker/v4"
	"github.com/juju/worker/v4/catacomb"

	corelife "github.com/juju/juju/core/life"
	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/objectstore"
	coretrace "github.com/juju/juju/core/trace"
	"github.com/juju/juju/core/watcher"
	modelerrors "github.com/juju/juju/domain/model/errors"
)

// ModelService is the interface that provides model information.
type ModelService interface {
	// WatchModel returns a watcher that notifies when the model changes.
	WatchModel(ctx context.Context) (watcher.NotifyWatcher, error)
	// Model returns the model for the current context.
	Model(ctx context.Context) (model.Model, error)
}

// trackerWorker is a wrapper around a ObjectStore that adds tracing, without
// exposing the underlying ObjectStore.
type trackerWorker struct {
	catacomb catacomb.Catacomb

	modelUUID    model.UUID
	modelService ModelService
	objectStore  TrackedObjectStore
	tracer       coretrace.Tracer

	logger logger.Logger
}

func newTrackerWorker(
	modelUUID model.UUID,
	modelService ModelService,
	objectStore TrackedObjectStore,
	tracer coretrace.Tracer,
	logger logger.Logger,
) (*trackerWorker, error) {
	w := &trackerWorker{
		modelUUID:   modelUUID,
		objectStore: objectStore,
		tracer:      tracer,
		logger:      logger,
	}

	if err := catacomb.Invoke(catacomb.Plan{
		Name: "tracked-object-store",
		Site: &w.catacomb,
		Work: w.loop,
		Init: []worker.Worker{w.objectStore},
	}); err != nil {
		return nil, errors.Trace(err)
	}

	return w, nil
}

// Kill stops the worker.
func (t *trackerWorker) Kill() {
	t.catacomb.Kill(nil)
}

// Wait blocks until the worker has completed.
func (t *trackerWorker) Wait() error {
	return t.catacomb.Wait()
}

// Get returns an io.ReadCloser for data at path, namespaced to the
// model.
func (t *trackerWorker) Get(ctx context.Context, path string) (_ io.ReadCloser, _ int64, err error) {
	ctx, span := coretrace.Start(coretrace.WithTracer(ctx, t.tracer), coretrace.NameFromFunc(),
		coretrace.WithAttributes(coretrace.StringAttr("objectstore.path", path)),
	)
	defer func() {
		span.RecordError(err)
		span.End()
	}()

	return t.objectStore.Get(ctx, path)
}

// Put stores data from reader at path, namespaced to the model.
func (t *trackerWorker) Put(ctx context.Context, path string, r io.Reader, length int64) (uuid objectstore.UUID, err error) {
	ctx, span := coretrace.Start(coretrace.WithTracer(ctx, t.tracer), coretrace.NameFromFunc(),
		coretrace.WithAttributes(
			coretrace.StringAttr("objectstore.path", path),
			coretrace.Int64Attr("objectstore.size", length),
		),
	)
	defer func() {
		span.RecordError(err)
		span.End()
	}()

	return t.objectStore.Put(ctx, path, r, length)
}

func (t *trackerWorker) Report() map[string]any {
	return t.objectStore.Report()
}

func (t *trackerWorker) loop() error {
	ctx := t.catacomb.Context(context.Background())

	modelWatcher, err := t.modelService.WatchModel(ctx)
	if err != nil {
		return errors.Annotate(err, "watching model")
	}

	for {
		select {
		case <-t.catacomb.Dying():
			return t.catacomb.ErrDying()

		case <-modelWatcher.Changes():
			model, err := t.modelService.Model(ctx)
			if errors.Is(err, modelerrors.NotFound) {
				// The model has been removed, we can stop the worker.
				t.logger.Infof(ctx, "model %q (%s) has been removed, stopping tracker worker", t.modelUUID)
				return nil
			} else if err != nil {
				return errors.Annotate(err, "reading model")
			}
			if corelife.IsDead(model.Life) {
				// The model is dead, we can stop the worker.
				t.logger.Infof(ctx, "model %q (%s) is dead, stopping tracker worker", model.Name, model.UUID)
				return nil
			}
		}
	}
}
