// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package cache

import (
	"sync"
	"sync/atomic"

	"github.com/juju/errors"
	worker "gopkg.in/juju/worker.v1"
)

type identifier struct {
	counter uint64
}

func newIdentifier() *identifier {
	return &identifier{counter: 0}
}

func (id *identifier) Inc() uint64 {
	return atomic.AddUint64(&id.counter, 1)
}

type Resources struct {
	identifier   *identifier
	subResources map[string]*Resources
	watchers     map[uint64]Watcher
	mu           sync.Mutex
}

func newResources(identifier *identifier) *Resources {
	return &Resources{
		identifier:   identifier,
		subResources: make(map[string]*Resources),
		watchers:     make(map[uint64]Watcher),
	}
}

func (r *Resources) Namespace(ns string) *Resources {
	if _, ok := r.subResources[ns]; !ok {
		r.subResources[ns] = newResources(r.identifier)
	}
	return r.subResources[ns]
}

// Register a watcher to the watcher resource, which returns a uint64 for
// identifying the watcher uniquely.
func (r *Resources) Register(w Watcher) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	value := r.identifier.Inc()
	r.watchers[value] = w
	return value
}

// Unregister removes the watcher identifier from the watcher resource, but it
// makes no effort to clean up the watcher itself
func (r *Resources) Unregister(identifier uint64) {
	r.mu.Lock()
	delete(r.watchers, identifier)
	r.mu.Unlock()
}

// Stop attempts to stop a watcher with in the resources.
func (r *Resources) Stop(identifier uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// attempt to search over your own resources before going through the
	// subresources
	for id, watcher := range r.watchers {
		if id == identifier {
			if err := worker.Stop(watcher); err != nil {
				return errors.Trace(err)
			}
			return nil
		}
	}
	for _, subResource := range r.subResources {
		err := subResource.Stop(identifier)
		// we stopped the resource, so no need to walk any further
		if err == nil {
			return nil
		}
		// we didn't find anything, so we need to continue walking
		if errors.IsNotFound(err) {
			continue
		}
		// we found an error attempting to stop the resource, so we should
		// bubble the error up.
		return errors.Trace(err)
	}
	return errors.NotFoundf("watcher for identifier %d", identifier)
}

// StopAll attempts to stop all the watchers with in the resources, which
// includes all the sub-resources with in the namespaces doing a bottom cleanup
// effectively.
// StopAll will also unregister the watcher at the same time.
func (r *Resources) StopAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for namespace, subResource := range r.subResources {
		if err := subResource.StopAll(); err != nil {
			return errors.Trace(err)
		}
		delete(r.subResources, namespace)
	}
	for identifier, watcher := range r.watchers {
		if err := worker.Stop(watcher); err != nil {
			return errors.Trace(err)
		}
		delete(r.watchers, identifier)
	}
	return nil
}
