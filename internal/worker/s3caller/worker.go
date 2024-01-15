// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package s3caller

import (
	"github.com/juju/worker/v4"
	"gopkg.in/tomb.v2"

	"github.com/juju/juju/core/objectstore"
	"github.com/juju/juju/internal/s3client"
)

// ClientFactory is a function that creates a new object store client.
type ClientFactory interface {
	// ClientFor returns a new object store client for the supplied
	// credentials.
	ClientFor(s3client.Credentials) (objectstore.Session, error)
}

type workerConfig struct {
	ClientFactory ClientFactory
}

type s3ClientWorker struct {
	tomb tomb.Tomb
	cfg  workerConfig
}

func newS3ClientWorker(config workerConfig) worker.Worker {
	w := &s3ClientWorker{cfg: config}
	w.tomb.Go(w.loop)
	return w
}

// Kill is part of the worker.Worker interface.
func (w *s3ClientWorker) Kill() {
	w.tomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (w *s3ClientWorker) Wait() error {
	return w.tomb.Wait()
}

// Anonymous returns a session that can be used to access the object store
// anonymously. No credentials are used to create the session.
func (w *s3ClientWorker) Anonymous() (objectstore.Session, error) {
	return w.cfg.ClientFactory.ClientFor(s3client.AnonymousCredentials{})
}

func (w *s3ClientWorker) loop() (err error) {
	<-w.tomb.Dying()
	return tomb.ErrDying
}
