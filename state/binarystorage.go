// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"io"

	"github.com/juju/blobstore/v3"
	"github.com/juju/errors"
	jujutxn "github.com/juju/txn/v3"

	"github.com/juju/juju/core/objectstore"
	"github.com/juju/juju/internal/mongo"
	"github.com/juju/juju/state/binarystorage"
)

var binarystorageNew = binarystorage.New

// ToolsStorage returns a new binarystorage.StorageCloser that stores tools
// metadata in the "juju" database "toolsmetadata" collection.
func (st *State) ToolsStorage(storeFactory objectstore.ObjectStoreFactory) (binarystorage.StorageCloser, error) {
	modelStorage := newBinaryStorageCloser(st.database, storeFactory.ModelObjectStore(), toolsmetadataC, st.ModelUUID())
	if st.IsController() {
		return modelStorage, nil
	}
	// This is a hosted model. Hosted models have their own tools
	// catalogue, which we combine with the controller's.
	controllerStorage := newBinaryStorageCloser(
		st.database,
		storeFactory.ControllerObjectStore(),
		toolsmetadataC, st.ControllerModelUUID(),
	)
	storage, err := binarystorage.NewLayeredStorage(modelStorage, controllerStorage)
	if err != nil {
		modelStorage.Close()
		controllerStorage.Close()
		return nil, errors.Trace(err)
	}
	return &storageCloser{Storage: storage, closer: func() {
		modelStorage.Close()
		controllerStorage.Close()
	}}, nil
}

func newBinaryStorageCloser(db Database, store objectstore.ObjectStore, collectionName, uuid string) binarystorage.StorageCloser {
	db, closer1 := db.CopyForModel(uuid)
	metadataCollection, closer2 := db.GetCollection(collectionName)
	txnRunner, closer3 := db.TransactionRunner()
	closer := func() {
		closer3()
		closer2()
		closer1()
	}
	storage := newBinaryStorage(uuid, metadataCollection, txnRunner, store)
	return &storageCloser{Storage: storage, closer: closer}
}

func newBinaryStorage(uuid string, metadataCollection mongo.Collection, txnRunner jujutxn.Runner, store objectstore.ObjectStore) binarystorage.Storage {
	db := metadataCollection.Writeable().Underlying().Database
	managedStorage := blobstore.NewManagedStorage(db, managedStorage{
		store: store,
	})
	return binarystorageNew(uuid, managedStorage, metadataCollection, txnRunner)
}

type storageCloser struct {
	binarystorage.Storage
	closer func()
}

func (sc *storageCloser) Close() error {
	sc.closer()
	return nil
}

type managedStorage struct {
	store objectstore.ObjectStore
}

// Get returns a reader for the resource located at path.
func (s managedStorage) Get(path string) (io.ReadCloser, error) {
	r, _, err := s.store.Get(context.TODO(), path)
	return r, errors.Trace(err)
}

// Put writes data from the specified reader to path and returns a checksum of
// the data written.
func (s managedStorage) Put(path string, r io.Reader, length int64) (string, error) {
	if err := s.store.Put(context.TODO(), path, r, length); err != nil {
		return "", err
	}
	// Interestingly, the blobstore doesn't actually use the checksum here.
	return "", nil
}

// Remove deletes the data at the specified path.
func (s managedStorage) Remove(path string) error {
	return s.store.Remove(context.TODO(), path)
}
