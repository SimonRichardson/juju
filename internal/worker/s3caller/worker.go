// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package s3caller

import (
	context "context"
	"sync"

	"github.com/juju/errors"
	"github.com/juju/worker/v4"
	"github.com/juju/worker/v4/catacomb"

	"github.com/juju/juju/controller"
	"github.com/juju/juju/core/objectstore"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/internal/s3client"
)

// ClientFactory is a function that creates a new object store client.
type ClientFactory interface {
	// ClientFor returns a new object store client for the supplied
	// credentials.
	ClientFor(s3client.Credentials) (objectstore.Session, error)
}

type ControllerConfigService interface {

	// ControllerConfig returns the current controller configuration.
	ControllerConfig() (controller.Config, error)

	// WatchControllerConfig provides a watcher for changes on controller config.
	WatchControllerConfig() (watcher.StringsWatcher, error)
}

type workerConfig struct {
	ClientFactory           ClientFactory
	ControllerConfigService ControllerConfigService
}

type s3ClientWorker struct {
	catacomb catacomb.Catacomb
	cfg      workerConfig

	mutex       sync.RWMutex
	anonClient  objectstore.Session
	credsClient objectstore.Session
}

func newS3ClientWorker(config workerConfig) (worker.Worker, error) {
	// Firstly populate the credentials from the controller config.
	controllerConfig, err := config.ControllerConfigService.ControllerConfig()
	if err != nil {
		return nil, errors.Trace(err)
	}

	anonClient, err := config.ClientFactory.ClientFor(s3client.AnonymousCredentials{})
	if err != nil {
		return nil, errors.Trace(err)
	}

	credsClient, err := createCredentialsClient(config.ClientFactory, controllerConfig)
	if err != nil {
		return nil, errors.Trace(err)
	}

	w := &s3ClientWorker{
		cfg:         config,
		anonClient:  anonClient,
		credsClient: credsClient,
	}

	if err := catacomb.Invoke(catacomb.Plan{
		Site: &w.catacomb,
		Work: w.loop,
	}); err != nil {
		return nil, errors.Trace(err)
	}

	return w, nil
}

// Kill is part of the worker.Worker interface.
func (w *s3ClientWorker) Kill() {
	w.catacomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (w *s3ClientWorker) Wait() error {
	return w.catacomb.Wait()
}

// WithAnonymous uses the current client and calls the supplied function, with
// a limited time context and the supplied anonymous credential client.
func (w *s3ClientWorker) WithAnonymous(fn func(context.Context, objectstore.Session) error) error {
	w.mutex.RLock()
	client := w.anonClient
	w.mutex.RUnlock()

	ctx, cancel := w.scopedContext()
	defer cancel()

	return fn(ctx, client)
}

// WithCredentials uses the current client and calls the supplied function, with
// a limited time context and the supplied credentials client.
func (w *s3ClientWorker) WithCredentials(fn func(context.Context, objectstore.Session) error) error {
	w.mutex.RLock()
	client := w.credsClient
	w.mutex.RUnlock()

	ctx, cancel := w.scopedContext()
	defer cancel()

	return fn(ctx, client)
}

func (w *s3ClientWorker) loop() (err error) {
	watcher, err := w.cfg.ControllerConfigService.WatchControllerConfig()
	if err != nil {
		return errors.Trace(err)
	}

	if err := w.catacomb.Add(watcher); err != nil {
		return errors.Trace(err)
	}

	for {
		select {
		case <-w.catacomb.Dying():
			return w.catacomb.ErrDying()
		case <-watcher.Changes():
			controllerConfig, err := w.cfg.ControllerConfigService.ControllerConfig()
			if err != nil {
				return errors.Trace(err)
			}

			credsClient, err := createCredentialsClient(w.cfg.ClientFactory, controllerConfig)
			if err != nil {
				return errors.Trace(err)
			}

			w.mutex.Lock()
			w.credsClient = credsClient
			w.mutex.Unlock()
		}
	}
}

func (w *s3ClientWorker) scopedContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(w.catacomb.Context(context.Background()))
}

func createCredentialsClient(clientFactory ClientFactory, controllerConfig controller.Config) (objectstore.Session, error) {
	credentials, err := extractCredentials(controllerConfig)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return clientFactory.ClientFor(credentials)
}

func extractCredentials(controllerConfig controller.Config) (s3client.Credentials, error) {
	return s3client.AnonymousCredentials{}, nil
}
