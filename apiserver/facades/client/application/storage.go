// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package application

import (
	"context"
	"math"

	"github.com/juju/errors"

	applicationservice "github.com/juju/juju/domain/application/service"
	"github.com/juju/juju/internal/storage"
)

// storageDirectives validates storage directives to override an applications
// storage directives.
func storageDirectives(
	ctx context.Context,
	storageService StorageService,
	storage map[string]storage.Directive,
) (map[string]applicationservice.StorageDirectiveOverrides, error) {
	res := map[string]applicationservice.StorageDirectiveOverrides{}
	for storageName, storageDirective := range storage {
		sdo := applicationservice.StorageDirectiveOverrides{}
		if storageDirective.Count != 0 {
			if storageDirective.Count > math.MaxUint32 {
				return nil, errors.NotValidf(
					"storage directive %s count too large", storageName,
				)
			}
			count := uint32(storageDirective.Count)
			sdo.Count = &count
		}
		if storageDirective.Size != 0 {
			sdo.Size = &storageDirective.Size
		}
		if storageDirective.Pool != "" {
			poolUUID, err := storageService.GetStoragePoolUUID(
				ctx,
				storageDirective.Pool,
			)
			if err != nil {
				return nil, errors.Annotatef(err, "storage directive %s pool",
					storageName)
			}
			sdo.PoolUUID = &poolUUID
		}
		res[storageName] = sdo
	}
	return res, nil
}
