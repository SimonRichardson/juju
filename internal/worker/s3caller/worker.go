// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package s3caller

import (
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

	mutex       sync.Mutex
	credentials s3client.Credentials
}

func newS3ClientWorker(config workerConfig) (worker.Worker, error) {
	// Firstly populate the credentials from the controller config.
	controllerConfig, err := config.ControllerConfigService.ControllerConfig()
	if err != nil {
		return nil, errors.Trace(err)
	}

	credentials, err := extractCredentials(controllerConfig)
	if err != nil {
		return nil, errors.Trace(err)
	}

	w := &s3ClientWorker{
		cfg:         config,
		credentials: credentials,
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

// Anonymous returns a session that can be used to access the object store
// anonymously. No credentials are used to create the session.
func (w *s3ClientWorker) Anonymous() (objectstore.Session, error) {
	return w.cfg.ClientFactory.ClientFor(s3client.AnonymousCredentials{})
}

// Credentials returns a session that can be used to access the object store
// using the supplied credentials.
func (w *s3ClientWorker) Credentials() (objectstore.Session, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	return w.cfg.ClientFactory.ClientFor(w.credentials)
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
			if err := w.updateCredentials(); err != nil {
				return errors.Trace(err)
			}
		}
	}
}

func (w *s3ClientWorker) updateCredentials() error {
	controllerConfig, err := w.cfg.ControllerConfigService.ControllerConfig()
	if err != nil {
		return errors.Trace(err)
	}

	credentials, err := extractCredentials(controllerConfig)
	if err != nil {
		return errors.Trace(err)
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.credentials = credentials
	return nil
}

func extractCredentials(controllerConfig controller.Config) (s3client.Credentials, error) {
	return s3client.AnonymousCredentials{}, nil
}
