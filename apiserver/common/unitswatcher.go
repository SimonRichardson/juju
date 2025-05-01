// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package common

import (
	"context"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/apiserver/internal"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
)

// ApplicationService represents the application service for interacting
// with applications and units in a model.
type ApplicationService interface {
	// WatchApplicationUnits starts a watcher for the specified
	// application. The watcher will notify when the application
	// changes its units.
	WatchApplicationUnits(ctx context.Context, appName string) (watcher.Watcher[[]string], error)
}

// UnitsWatcher implements a common WatchUnits method for use by
// various facades.
type UnitsWatcher struct {
	applicationService ApplicationService
	watcherRegistry    facade.WatcherRegistry
	getCanWatch        GetAuthFunc

	// TODO: Remove me when we do machines.
	st state.EntityFinder
}

// NewUnitsWatcher returns a new UnitsWatcher. The GetAuthFunc will be
// used on each invocation of WatchUnits to determine current
// permissions.
func NewUnitsWatcher(
	applicationService ApplicationService,
	state state.EntityFinder,
	watcherRegistry facade.WatcherRegistry,
	getCanWatch GetAuthFunc,
) *UnitsWatcher {
	return &UnitsWatcher{
		applicationService: applicationService,
		watcherRegistry:    watcherRegistry,
		getCanWatch:        getCanWatch,
		st:                 state,
	}
}

// WatchUnits starts a StringsWatcher to watch all units belonging to
// to any entity (machine or application) passed in args.
func (u *UnitsWatcher) WatchUnits(ctx context.Context, args params.Entities) (params.StringsWatchResults, error) {
	result := params.StringsWatchResults{
		Results: make([]params.StringsWatchResult, len(args.Entities)),
	}
	if len(args.Entities) == 0 {
		return result, nil
	}
	canWatch, err := u.getCanWatch(ctx)
	if err != nil {
		return params.StringsWatchResults{}, errors.Trace(err)
	}
	for i, entity := range args.Entities {
		tag, err := names.ParseTag(entity.Tag)
		if err != nil {
			result.Results[i].Error = apiservererrors.ServerError(apiservererrors.ErrPerm)
			continue
		}

		var w watcher.Watcher[[]string]
		switch tag.Kind() {
		case names.ApplicationTagKind:
			w, err = u.watchApplicationUnits(ctx, canWatch, tag)
		case names.MachineTagKind:
			w, err = u.legacyWatchMachineUnits(canWatch, tag)
		default:
			result.Results[i].Error = apiservererrors.ServerError(apiservererrors.ErrPerm)
		}
		if err != nil {
			result.Results[i].Error = apiservererrors.ServerError(err)
			continue
		}

		id, changes, err := internal.EnsureRegisterWatcher(ctx, u.watcherRegistry, w)
		if err != nil {
			result.Results[i].Error = apiservererrors.ServerError(err)
			continue
		}

		result.Results[i] = params.StringsWatchResult{
			StringsWatcherId: id,
			Changes:          changes,
		}
	}
	return result, nil
}

func (u *UnitsWatcher) watchApplicationUnits(ctx context.Context, canWatch AuthFunc, tag names.Tag) (watcher.Watcher[[]string], error) {
	if !canWatch(tag) {
		return nil, apiservererrors.ErrPerm
	}

	return u.applicationService.WatchApplicationUnits(ctx, tag.Id())
}

func (u *UnitsWatcher) legacyWatchMachineUnits(canWatch AuthFunc, tag names.Tag) (watcher.Watcher[[]string], error) {
	if !canWatch(tag) {
		return nil, apiservererrors.ErrPerm
	}
	entity0, err := u.st.FindEntity(tag)
	if err != nil {
		return nil, err
	}
	entity, ok := entity0.(state.UnitsWatcher)
	if !ok {
		return nil, apiservererrors.NotSupportedError(tag, "watching units")
	}
	watch := entity.WatchUnits()
	return watch, nil
}
