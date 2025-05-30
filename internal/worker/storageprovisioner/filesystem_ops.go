// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package storageprovisioner

import (
	"context"
	"path/filepath"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/core/status"
	"github.com/juju/juju/internal/storage"
	"github.com/juju/juju/internal/wrench"
	"github.com/juju/juju/rpc/params"
)

// createFilesystems creates filesystems with the specified parameters.
func createFilesystems(ctx context.Context, deps *dependencies, ops map[names.FilesystemTag]*createFilesystemOp) error {
	filesystemParams := make([]storage.FilesystemParams, 0, len(ops))
	for _, op := range ops {
		filesystemParams = append(filesystemParams, op.args)
	}
	paramsBySource, filesystemSources, err := filesystemParamsBySource(
		deps.config.StorageDir,
		filesystemParams,
		deps.managedFilesystemSource,
		deps.config.Registry,
	)
	if err != nil {
		return errors.Trace(err)
	}
	var reschedule []scheduleOp
	var filesystems []storage.Filesystem
	var statuses []params.EntityStatusArgs
	for sourceName, filesystemParams := range paramsBySource {
		deps.config.Logger.Debugf(ctx, "creating filesystems: %v", filesystemParams)
		filesystemSource := filesystemSources[sourceName]
		validFilesystemParams, validationErrors := validateFilesystemParams(
			filesystemSource, filesystemParams,
		)
		for i, err := range validationErrors {
			if err == nil {
				continue
			}
			statuses = append(statuses, params.EntityStatusArgs{
				Tag:    filesystemParams[i].Tag.String(),
				Status: status.Error.String(),
				Info:   err.Error(),
			})
			deps.config.Logger.Debugf(ctx,
				"failed to validate parameters for %s: %v",
				names.ReadableString(filesystemParams[i].Tag), err,
			)
		}
		filesystemParams = validFilesystemParams
		if len(filesystemParams) == 0 {
			continue
		}
		results, err := filesystemSource.CreateFilesystems(ctx, filesystemParams)
		if err != nil {
			return errors.Annotatef(err, "creating filesystems from source %q", sourceName)
		}
		for i, result := range results {
			statuses = append(statuses, params.EntityStatusArgs{
				Tag:    filesystemParams[i].Tag.String(),
				Status: status.Attaching.String(),
			})
			entityStatus := &statuses[len(statuses)-1]
			if result.Error != nil {
				// Reschedule the filesystem creation.
				reschedule = append(reschedule, ops[filesystemParams[i].Tag])

				// Note: we keep the status as "pending" to indicate
				// that we will retry. When we distinguish between
				// transient and permanent errors, we will set the
				// status to "error" for permanent errors.
				entityStatus.Status = status.Pending.String()
				entityStatus.Info = result.Error.Error()
				deps.config.Logger.Debugf(ctx,
					"failed to create %s: %v",
					names.ReadableString(filesystemParams[i].Tag),
					result.Error,
				)
				continue
			}
			filesystems = append(filesystems, *result.Filesystem)
		}
	}
	scheduleOperations(deps, reschedule...)
	setStatus(ctx, deps, statuses)
	if len(filesystems) == 0 {
		return nil
	}
	// TODO(axw) we need to be able to list filesystems in the provider,
	// by environment, so that we can "harvest" them if they're
	// unknown. This will take care of killing filesystems that we fail
	// to record in state.
	errorResults, err := deps.config.Filesystems.SetFilesystemInfo(ctx, filesystemsFromStorage(filesystems))
	if err != nil {
		return errors.Annotate(err, "publishing filesystems to state")
	}
	for i, result := range errorResults {
		if result.Error != nil {
			deps.config.Logger.Errorf(ctx,
				"publishing filesystem %s to state: %v",
				filesystems[i].Tag.Id(),
				result.Error,
			)
		}
	}
	for _, v := range filesystems {
		updateFilesystem(deps, v)
	}
	return nil
}

// attachFilesystems creates filesystem attachments with the specified parameters.
func attachFilesystems(ctx context.Context, deps *dependencies, ops map[params.MachineStorageId]*attachFilesystemOp) error {
	filesystemAttachmentParams := make([]storage.FilesystemAttachmentParams, 0, len(ops))
	for _, op := range ops {
		args := op.args
		if args.Path == "" {
			args.Path = filepath.Join(deps.config.StorageDir, args.Filesystem.Id())
		}
		filesystemAttachmentParams = append(filesystemAttachmentParams, args)
	}
	paramsBySource, filesystemSources, err := filesystemAttachmentParamsBySource(
		deps.config.StorageDir,
		filesystemAttachmentParams,
		deps.filesystems,
		deps.managedFilesystemSource,
		deps.config.Registry,
	)
	if err != nil {
		return errors.Trace(err)
	}
	var reschedule []scheduleOp
	var filesystemAttachments []storage.FilesystemAttachment
	var statuses []params.EntityStatusArgs
	for sourceName, filesystemAttachmentParams := range paramsBySource {
		deps.config.Logger.Debugf(ctx, "attaching filesystems: %+v", filesystemAttachmentParams)
		filesystemSource := filesystemSources[sourceName]
		results, err := filesystemSource.AttachFilesystems(ctx, filesystemAttachmentParams)
		if err != nil {
			return errors.Annotatef(err, "attaching filesystems from source %q", sourceName)
		}
		for i, result := range results {
			p := filesystemAttachmentParams[i]
			statuses = append(statuses, params.EntityStatusArgs{
				Tag:    p.Filesystem.String(),
				Status: status.Attached.String(),
			})
			entityStatus := &statuses[len(statuses)-1]
			if result.Error != nil {
				// Reschedule the filesystem attachment.
				id := params.MachineStorageId{
					MachineTag:    p.Machine.String(),
					AttachmentTag: p.Filesystem.String(),
				}
				reschedule = append(reschedule, ops[id])

				// Note: we keep the status as "attaching" to
				// indicate that we will retry. When we distinguish
				// between transient and permanent errors, we will
				// set the status to "error" for permanent errors.
				entityStatus.Status = status.Attaching.String()
				entityStatus.Info = result.Error.Error()
				deps.config.Logger.Debugf(ctx,
					"failed to attach %s to %s: %v",
					names.ReadableString(p.Filesystem),
					names.ReadableString(p.Machine),
					result.Error,
				)
				continue
			}
			filesystemAttachments = append(filesystemAttachments, *result.FilesystemAttachment)
		}
	}
	scheduleOperations(deps, reschedule...)
	setStatus(ctx, deps, statuses)
	if err := setFilesystemAttachmentInfo(ctx, deps, filesystemAttachments); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// removeFilesystems destroys or releases filesystems with the specified parameters.
func removeFilesystems(ctx context.Context, deps *dependencies, ops map[names.FilesystemTag]*removeFilesystemOp) error {
	tags := make([]names.FilesystemTag, 0, len(ops))
	for tag := range ops {
		tags = append(tags, tag)
	}
	removeFilesystemParams, err := removeFilesystemParams(ctx, deps, tags)
	if err != nil {
		return errors.Trace(err)
	}
	filesystemParams := make([]storage.FilesystemParams, len(tags))
	removeFilesystemParamsByTag := make(map[names.FilesystemTag]params.RemoveFilesystemParams)
	for i, args := range removeFilesystemParams {
		removeFilesystemParamsByTag[tags[i]] = args
		filesystemParams[i] = storage.FilesystemParams{
			Tag:      tags[i],
			Provider: storage.ProviderType(args.Provider),
		}
	}
	paramsBySource, filesystemSources, err := filesystemParamsBySource(
		deps.config.StorageDir,
		filesystemParams,
		deps.managedFilesystemSource,
		deps.config.Registry,
	)
	if err != nil {
		return errors.Trace(err)
	}
	var remove []names.Tag
	var reschedule []scheduleOp
	var statuses []params.EntityStatusArgs
	removeFilesystems := func(tags []names.FilesystemTag, ids []string, f func(context.Context, []string) ([]error, error)) error {
		if len(ids) == 0 {
			return nil
		}
		errs, err := f(ctx, ids)
		if err != nil {
			return errors.Trace(err)
		}
		for i, err := range errs {
			tag := tags[i]
			if wrench.IsActive("storageprovisioner", "RemoveFilesystem") {
				err = errors.New("wrench active")
			}
			if err == nil {
				remove = append(remove, tag)
				continue
			}
			// Failed to destroy or release filesystem; reschedule and update status.
			reschedule = append(reschedule, ops[tag])
			statuses = append(statuses, params.EntityStatusArgs{
				Tag:    tag.String(),
				Status: status.Error.String(),
				Info:   errors.Annotate(err, "removing filesystem").Error(),
			})
		}
		return nil
	}
	for sourceName, filesystemParams := range paramsBySource {
		deps.config.Logger.Debugf(ctx, "removing filesystems from %q: %v", sourceName, filesystemParams)
		filesystemSource := filesystemSources[sourceName]
		removeTags := make([]names.FilesystemTag, len(filesystemParams))
		removeParams := make([]params.RemoveFilesystemParams, len(filesystemParams))
		for i, args := range filesystemParams {
			removeTags[i] = args.Tag
			removeParams[i] = removeFilesystemParamsByTag[args.Tag]
		}
		destroyTags, destroyIds, releaseTags, releaseIds := partitionRemoveFilesystemParams(removeTags, removeParams)
		if err := removeFilesystems(destroyTags, destroyIds, filesystemSource.DestroyFilesystems); err != nil {
			return errors.Trace(err)
		}
		if err := removeFilesystems(releaseTags, releaseIds, filesystemSource.ReleaseFilesystems); err != nil {
			return errors.Trace(err)
		}
	}
	scheduleOperations(deps, reschedule...)
	setStatus(ctx, deps, statuses)
	if err := removeEntities(ctx, deps, remove); err != nil {
		return errors.Annotate(err, "removing filesystems from state")
	}
	return nil
}

func partitionRemoveFilesystemParams(removeTags []names.FilesystemTag, removeParams []params.RemoveFilesystemParams) (
	destroyTags []names.FilesystemTag, destroyIds []string,
	releaseTags []names.FilesystemTag, releaseIds []string,
) {
	destroyTags = make([]names.FilesystemTag, 0, len(removeParams))
	destroyIds = make([]string, 0, len(removeParams))
	releaseTags = make([]names.FilesystemTag, 0, len(removeParams))
	releaseIds = make([]string, 0, len(removeParams))
	for i, args := range removeParams {
		tag := removeTags[i]
		if args.Destroy {
			destroyTags = append(destroyTags, tag)
			destroyIds = append(destroyIds, args.FilesystemId)
		} else {
			releaseTags = append(releaseTags, tag)
			releaseIds = append(releaseIds, args.FilesystemId)
		}
	}
	return
}

// detachFilesystems destroys filesystem attachments with the specified parameters.
func detachFilesystems(ctx context.Context, deps *dependencies, ops map[params.MachineStorageId]*detachFilesystemOp) error {
	filesystemAttachmentParams := make([]storage.FilesystemAttachmentParams, 0, len(ops))
	for _, op := range ops {
		filesystemAttachmentParams = append(filesystemAttachmentParams, op.args)
	}
	paramsBySource, filesystemSources, err := filesystemAttachmentParamsBySource(
		deps.config.StorageDir,
		filesystemAttachmentParams,
		deps.filesystems,
		deps.managedFilesystemSource,
		deps.config.Registry,
	)
	if err != nil {
		return errors.Trace(err)
	}
	var reschedule []scheduleOp
	var statuses []params.EntityStatusArgs
	var remove []params.MachineStorageId
	for sourceName, filesystemAttachmentParams := range paramsBySource {
		deps.config.Logger.Debugf(ctx, "detaching filesystems: %+v", filesystemAttachmentParams)
		filesystemSource, ok := filesystemSources[sourceName]
		if !ok && deps.isApplicationKind() {
			continue
		}
		errs, err := filesystemSource.DetachFilesystems(ctx, filesystemAttachmentParams)
		if err != nil {
			return errors.Annotatef(err, "detaching filesystems from source %q", sourceName)
		}
		for i, err := range errs {
			p := filesystemAttachmentParams[i]
			statuses = append(statuses, params.EntityStatusArgs{
				Tag: p.Filesystem.String(),
				// TODO(axw) when we support multiple
				// attachment, we'll have to check if
				// there are any other attachments
				// before saying the status "detached".
				Status: status.Detached.String(),
			})
			id := params.MachineStorageId{
				MachineTag:    p.Machine.String(),
				AttachmentTag: p.Filesystem.String(),
			}
			entityStatus := &statuses[len(statuses)-1]
			if wrench.IsActive("storageprovisioner", "DetachFilesystem") {
				err = errors.New("wrench active")
			}
			if err != nil {
				reschedule = append(reschedule, ops[id])
				entityStatus.Status = status.Detaching.String()
				entityStatus.Info = err.Error()
				deps.config.Logger.Debugf(ctx,
					"failed to detach %s from %s: %v",
					names.ReadableString(p.Filesystem),
					names.ReadableString(p.Machine),
					err,
				)
				continue
			}
			remove = append(remove, id)
		}
	}
	scheduleOperations(deps, reschedule...)
	setStatus(ctx, deps, statuses)
	if err := removeAttachments(ctx, deps, remove); err != nil {
		return errors.Annotate(err, "removing attachments from state")
	}
	for _, id := range remove {
		delete(deps.filesystemAttachments, id)
	}
	return nil
}

// filesystemParamsBySource separates the filesystem parameters by filesystem source.
func filesystemParamsBySource(
	baseStorageDir string,
	params []storage.FilesystemParams,
	managedFilesystemSource storage.FilesystemSource,
	registry storage.ProviderRegistry,
) (map[string][]storage.FilesystemParams, map[string]storage.FilesystemSource, error) {
	// TODO(axw) later we may have multiple instantiations (sources)
	// for a storage provider, e.g. multiple Ceph installations. For
	// now we assume a single source for each provider type, with no
	// configuration.
	filesystemSources := make(map[string]storage.FilesystemSource)
	for _, params := range params {
		sourceName := string(params.Provider)
		if _, ok := filesystemSources[sourceName]; ok {
			continue
		}
		if params.Volume != (names.VolumeTag{}) {
			filesystemSources[sourceName] = managedFilesystemSource
			continue
		}
		filesystemSource, err := filesystemSource(
			baseStorageDir, sourceName, params.Provider, registry,
		)
		// For k8s models, there may be a not found error as there's only
		// one (model) storage provisioner worker which reacts to all storage,
		// even tmpfs or rootfs which is ostensibly handled by a machine storage
		// provisioner worker. There's no such provisoner for k8s but we still
		// process the detach/destroy so the state model can be updated.
		if errors.Cause(err) == errNonDynamic || errors.Is(err, errors.NotFound) {
			filesystemSource = nil
		} else if err != nil {
			return nil, nil, errors.Annotate(err, "getting filesystem source")
		}
		filesystemSources[sourceName] = filesystemSource
	}
	paramsBySource := make(map[string][]storage.FilesystemParams)
	for _, param := range params {
		sourceName := string(param.Provider)
		filesystemSource := filesystemSources[sourceName]
		if filesystemSource == nil {
			// Ignore nil filesystem sources; this means that the
			// filesystem should be created by the machine-provisioner.
			continue
		}
		paramsBySource[sourceName] = append(paramsBySource[sourceName], param)
	}
	return paramsBySource, filesystemSources, nil
}

// validateFilesystemParams validates a collection of filesystem parameters.
func validateFilesystemParams(
	filesystemSource storage.FilesystemSource,
	filesystemParams []storage.FilesystemParams,
) ([]storage.FilesystemParams, []error) {
	valid := make([]storage.FilesystemParams, 0, len(filesystemParams))
	results := make([]error, len(filesystemParams))
	for i, params := range filesystemParams {
		err := filesystemSource.ValidateFilesystemParams(params)
		if err == nil {
			valid = append(valid, params)
		}
		results[i] = err
	}
	return valid, results
}

// filesystemAttachmentParamsBySource separates the filesystem attachment parameters by filesystem source.
func filesystemAttachmentParamsBySource(
	baseStorageDir string,
	filesystemAttachmentParams []storage.FilesystemAttachmentParams,
	filesystems map[names.FilesystemTag]storage.Filesystem,
	managedFilesystemSource storage.FilesystemSource,
	registry storage.ProviderRegistry,
) (map[string][]storage.FilesystemAttachmentParams, map[string]storage.FilesystemSource, error) {
	// TODO(axw) later we may have multiple instantiations (sources)
	// for a storage provider, e.g. multiple Ceph installations. For
	// now we assume a single source for each provider type, with no
	// configuration.
	filesystemSources := make(map[string]storage.FilesystemSource)
	paramsBySource := make(map[string][]storage.FilesystemAttachmentParams)
	for _, params := range filesystemAttachmentParams {
		sourceName := string(params.Provider)
		paramsBySource[sourceName] = append(paramsBySource[sourceName], params)
		if _, ok := filesystemSources[sourceName]; ok {
			continue
		}
		filesystem, ok := filesystems[params.Filesystem]
		if !ok || filesystem.Volume != (names.VolumeTag{}) {
			filesystemSources[sourceName] = managedFilesystemSource
			continue
		}
		filesystemSource, err := filesystemSource(
			baseStorageDir, sourceName, params.Provider, registry,
		)
		// For k8s models, there may be a not found error as there's only
		// one (model) storage provisioner worker which reacts to all storage,
		// even tmpfs or rootfs which is ostensibly handled by a machine storage
		// provisioner worker. There's no such provisoner for k8s but we still
		// process the detach/destroy so the state model can be updated.
		if err != nil && !errors.Is(err, errors.NotFound) {
			return nil, nil, errors.Annotate(err, "getting filesystem source")
		}
		filesystemSources[sourceName] = filesystemSource
	}
	return paramsBySource, filesystemSources, nil
}

func setFilesystemAttachmentInfo(ctx context.Context, deps *dependencies, filesystemAttachments []storage.FilesystemAttachment) error {
	if len(filesystemAttachments) == 0 {
		return nil
	}
	// TODO(axw) we need to be able to list filesystem attachments in the
	// provider, by environment, so that we can "harvest" them if they're
	// unknown. This will take care of killing filesystems that we fail to
	// record in state.
	errorResults, err := deps.config.Filesystems.SetFilesystemAttachmentInfo(
		ctx,
		filesystemAttachmentsFromStorage(filesystemAttachments),
	)
	if err != nil {
		return errors.Annotate(err, "publishing filesystems to state")
	}
	for i, result := range errorResults {
		if result.Error != nil {
			return errors.Annotatef(
				result.Error, "publishing attachment of %s to %s to state",
				names.ReadableString(filesystemAttachments[i].Filesystem),
				names.ReadableString(filesystemAttachments[i].Machine),
			)
		}
		// Record the filesystem attachment in the context.
		id := params.MachineStorageId{
			MachineTag:    filesystemAttachments[i].Machine.String(),
			AttachmentTag: filesystemAttachments[i].Filesystem.String(),
		}
		deps.filesystemAttachments[id] = filesystemAttachments[i]
		removePendingFilesystemAttachment(deps, id)
	}
	return nil
}

func filesystemsFromStorage(in []storage.Filesystem) []params.Filesystem {
	out := make([]params.Filesystem, len(in))
	for i, f := range in {
		paramsFilesystem := params.Filesystem{
			FilesystemTag: f.Tag.String(),
			VolumeTag:     "",
			Info: params.FilesystemInfo{
				FilesystemId: f.FilesystemId,
				Pool:         "", // pool
				Size:         f.Size,
			},
		}
		if f.Volume != (names.VolumeTag{}) {
			paramsFilesystem.VolumeTag = f.Volume.String()
		}
		out[i] = paramsFilesystem
	}
	return out
}

func filesystemAttachmentsFromStorage(in []storage.FilesystemAttachment) []params.FilesystemAttachment {
	out := make([]params.FilesystemAttachment, len(in))
	for i, f := range in {
		out[i] = params.FilesystemAttachment{
			FilesystemTag: f.Filesystem.String(),
			MachineTag:    f.Machine.String(),
			Info: params.FilesystemAttachmentInfo{
				MountPoint: f.Path,
				ReadOnly:   f.ReadOnly,
			},
		}
	}
	return out
}

type createFilesystemOp struct {
	exponentialBackoff
	args storage.FilesystemParams
}

func (op *createFilesystemOp) key() interface{} {
	return op.args.Tag
}

type removeFilesystemOp struct {
	exponentialBackoff
	tag names.FilesystemTag
}

func (op *removeFilesystemOp) key() interface{} {
	return op.tag
}

type attachFilesystemOp struct {
	exponentialBackoff
	args storage.FilesystemAttachmentParams
}

func (op *attachFilesystemOp) key() interface{} {
	return params.MachineStorageId{
		MachineTag:    op.args.Machine.String(),
		AttachmentTag: op.args.Filesystem.String(),
	}
}

type detachFilesystemOp struct {
	exponentialBackoff
	args storage.FilesystemAttachmentParams
}

func (op *detachFilesystemOp) key() interface{} {
	return params.MachineStorageId{
		MachineTag:    op.args.Machine.String(),
		AttachmentTag: op.args.Filesystem.String(),
	}
}
