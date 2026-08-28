// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelgeneration

import (
	"context"
	"fmt"
	"sort"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/apiserver/authentication"
	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/apiserver/facade"
	coremodel "github.com/juju/juju/core/model"
	"github.com/juju/juju/core/permission"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/generation"
	generationerrors "github.com/juju/juju/domain/generation/errors"
	internalerrors "github.com/juju/juju/internal/errors"
	"github.com/juju/juju/rpc/params"
)

// API implements the ModelGeneration facade.
type API struct {
	authorizer         facade.Authorizer
	controllerUUID     string
	modelUUID          coremodel.UUID
	apiUser            names.UserTag
	generationService  GenerationService
	applicationService ApplicationService
}

// NewAPI returns a model generation facade.
func NewAPI(
	authorizer facade.Authorizer,
	controllerUUID string,
	modelUUID coremodel.UUID,
	generationService GenerationService,
	applicationService ApplicationService,
) (*API, error) {
	if !authorizer.AuthClient() {
		return nil, apiservererrors.ErrPerm
	}
	apiUser, ok := authorizer.GetAuthTag().(names.UserTag)
	if !ok {
		return nil, internalerrors.Errorf(
			"expected authenticated entity to be a user, got %T",
			authorizer.GetAuthTag(),
		)
	}
	return &API{
		authorizer:         authorizer,
		controllerUUID:     controllerUUID,
		modelUUID:          modelUUID,
		apiUser:            apiUser,
		generationService:  generationService,
		applicationService: applicationService,
	}, nil
}

func (api *API) checkAccess(ctx context.Context, access permission.Access) error {
	err := api.authorizer.HasPermission(
		ctx, permission.SuperuserAccess,
		names.NewControllerTag(api.controllerUUID),
	)
	if err == nil {
		return nil
	}
	if !errors.Is(err, authentication.ErrorEntityMissingPermission) {
		return errors.Trace(err)
	}
	return api.authorizer.HasPermission(
		ctx, access, names.NewModelTag(api.modelUUID.String()),
	)
}

func (api *API) checkCanRead(ctx context.Context) error {
	return api.checkAccess(ctx, permission.ReadAccess)
}

func (api *API) checkCanAdmin(ctx context.Context) error {
	return api.checkAccess(ctx, permission.AdminAccess)
}

// AddBranch creates the model's active branch.
func (api *API) AddBranch(ctx context.Context, arg params.BranchArg) (params.ErrorResult, error) {
	if err := api.checkCanAdmin(ctx); err != nil {
		return params.ErrorResult{}, errors.Trace(err)
	}
	_, err := api.generationService.AddBranch(ctx, arg.BranchName, api.apiUser.Name())
	return params.ErrorResult{Error: apiservererrors.ServerError(err)}, nil
}

// TrackBranch enrolls units or units of applications in a branch.
func (api *API) TrackBranch(ctx context.Context, arg params.BranchTrackArg) (params.ErrorResults, error) {
	if err := api.checkCanAdmin(ctx); err != nil {
		return params.ErrorResults{}, errors.Trace(err)
	}
	if arg.NumUnits > 0 && len(arg.Entities) > 1 {
		return params.ErrorResults{}, internalerrors.Errorf(
			"number of units and unit IDs can not be specified at the same time",
		)
	}
	return api.updateTrackedUnits(ctx, arg, true), nil
}

// UntrackBranch removes units or units of applications from a branch.
func (api *API) UntrackBranch(ctx context.Context, arg params.BranchTrackArg) (params.ErrorResults, error) {
	if err := api.checkCanAdmin(ctx); err != nil {
		return params.ErrorResults{}, errors.Trace(err)
	}
	if arg.NumUnits > 0 && len(arg.Entities) > 1 {
		return params.ErrorResults{}, internalerrors.Errorf(
			"number of units and unit IDs can not be specified at the same time",
		)
	}
	return api.updateTrackedUnits(ctx, arg, false), nil
}

func (api *API) updateTrackedUnits(
	ctx context.Context, arg params.BranchTrackArg, track bool,
) params.ErrorResults {
	result := params.ErrorResults{Results: make([]params.ErrorResult, len(arg.Entities))}
	for i, entity := range arg.Entities {
		unitUUIDs, err := api.resolveUnitUUIDs(ctx, entity.Tag, arg.NumUnits)
		if err == nil {
			if track {
				err = api.generationService.TrackBranch(ctx, arg.BranchName, unitUUIDs)
			} else {
				err = api.generationService.UntrackBranch(ctx, arg.BranchName, unitUUIDs)
			}
		}
		result.Results[i].Error = apiservererrors.ServerError(err)
	}
	return result
}

func (api *API) resolveUnitUUIDs(
	ctx context.Context, entity string, numUnits int,
) ([]coreunit.UUID, error) {
	tag, err := names.ParseTag(entity)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var unitNames []coreunit.Name
	switch tag.Kind() {
	case names.UnitTagKind:
		if numUnits > 0 {
			return nil, internalerrors.Errorf(
				"number of units can only be specified for an application",
			)
		}
		unitNames = []coreunit.Name{coreunit.Name(tag.Id())}
	case names.ApplicationTagKind:
		unitNames, err = api.applicationService.GetUnitNamesForApplication(ctx, tag.Id())
		if err != nil {
			return nil, internalerrors.Capture(err)
		}
		sort.Slice(unitNames, func(i, j int) bool {
			return unitNames[i].String() < unitNames[j].String()
		})
		if numUnits > len(unitNames) {
			return nil, internalerrors.Errorf(
				"application %q has %d units, cannot select %d",
				tag.Id(), len(unitNames), numUnits,
			)
		}
		if numUnits > 0 {
			unitNames = unitNames[:numUnits]
		}
	default:
		return nil, internalerrors.Errorf(
			"expected unit or application tag, got %q", tag.Kind(),
		)
	}

	unitUUIDs := make([]coreunit.UUID, len(unitNames))
	for i, unitName := range unitNames {
		unitUUIDs[i], err = api.applicationService.GetUnitUUID(ctx, unitName)
		if err != nil {
			return nil, internalerrors.Capture(err)
		}
	}
	return unitUUIDs, nil
}

// CommitBranch commits a branch to main.
func (api *API) CommitBranch(ctx context.Context, arg params.BranchArg) (params.IntResult, error) {
	if err := api.checkCanAdmin(ctx); err != nil {
		return params.IntResult{}, errors.Trace(err)
	}
	id, err := api.generationService.CommitBranch(ctx, arg.BranchName, api.apiUser.Name())
	if err != nil {
		return params.IntResult{Error: apiservererrors.ServerError(err)}, nil
	}
	result, err := generationIDToInt(id)
	if err != nil {
		return params.IntResult{Error: apiservererrors.ServerError(err)}, nil
	}
	return params.IntResult{Result: result}, nil
}

// AbortBranch aborts a branch without applying its changes.
func (api *API) AbortBranch(ctx context.Context, arg params.BranchArg) (params.ErrorResult, error) {
	if err := api.checkCanAdmin(ctx); err != nil {
		return params.ErrorResult{}, errors.Trace(err)
	}
	err := api.generationService.AbortBranch(ctx, arg.BranchName, api.apiUser.Name())
	return params.ErrorResult{Error: apiservererrors.ServerError(err)}, nil
}

// HasActiveBranch reports whether the named branch is active.
func (api *API) HasActiveBranch(ctx context.Context, arg params.BranchArg) (params.BoolResult, error) {
	if err := api.checkCanRead(ctx); err != nil {
		return params.BoolResult{}, errors.Trace(err)
	}
	_, err := api.generationService.GetBranchByName(ctx, arg.BranchName)
	if internalerrors.Is(err, generationerrors.BranchNotFound) {
		return params.BoolResult{Result: false}, nil
	}
	return params.BoolResult{
		Result: err == nil,
		Error:  apiservererrors.ServerError(err),
	}, nil
}

// BranchInfo returns active branch details.
func (api *API) BranchInfo(ctx context.Context, args params.BranchInfoArgs) (params.BranchResults, error) {
	if err := api.checkCanRead(ctx); err != nil {
		return params.BranchResults{}, errors.Trace(err)
	}

	var branches []generation.Generation
	var err error
	if len(args.BranchNames) == 0 {
		branches, err = api.generationService.ListBranches(ctx)
	} else {
		branches = make([]generation.Generation, len(args.BranchNames))
		for i, name := range args.BranchNames {
			branches[i], err = api.generationService.GetBranchByName(ctx, name)
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		return params.BranchResults{Error: apiservererrors.ServerError(err)}, nil
	}

	result := params.BranchResults{Generations: make([]params.Generation, len(branches))}
	for i, branch := range branches {
		result.Generations[i], err = api.encodeBranch(ctx, branch, args.Detailed)
		if err != nil {
			return params.BranchResults{Error: apiservererrors.ServerError(err)}, nil
		}
	}
	return result, nil
}

func (api *API) encodeBranch(
	ctx context.Context, branch generation.Generation, detailed bool,
) (params.Generation, error) {
	result, err := encodeGeneration(branch)
	if err != nil {
		return params.Generation{}, internalerrors.Capture(err)
	}

	tracked, err := api.generationService.GetTrackedUnits(ctx, branch.Name)
	if err != nil {
		return params.Generation{}, internalerrors.Capture(err)
	}
	byApplication := make(map[string][]coreunit.Name)
	for _, unitName := range tracked {
		byApplication[unitName.Application()] = append(
			byApplication[unitName.Application()], unitName,
		)
	}
	applicationNames := make([]string, 0, len(byApplication))
	for appName := range byApplication {
		applicationNames = append(applicationNames, appName)
	}
	sort.Strings(applicationNames)

	result.Applications = make([]params.GenerationApplication, 0, len(applicationNames))
	for _, appName := range applicationNames {
		allUnits, err := api.applicationService.GetUnitNamesForApplication(ctx, appName)
		if err != nil {
			return params.Generation{}, internalerrors.Capture(err)
		}
		tracking := byApplication[appName]
		sort.Slice(tracking, func(i, j int) bool {
			return tracking[i].String() < tracking[j].String()
		})
		app := params.GenerationApplication{
			ApplicationName: appName,
			UnitProgress:    fmt.Sprintf("%d/%d", len(tracking), len(allUnits)),
		}
		if detailed {
			trackedSet := make(map[coreunit.Name]struct{}, len(tracking))
			for _, unitName := range tracking {
				trackedSet[unitName] = struct{}{}
				app.UnitsTracking = append(app.UnitsTracking, unitName.String())
			}
			for _, unitName := range allUnits {
				if _, ok := trackedSet[unitName]; !ok {
					app.UnitsPending = append(app.UnitsPending, unitName.String())
				}
			}
			sort.Strings(app.UnitsPending)
		}
		result.Applications = append(result.Applications, app)
	}
	return result, nil
}

// ListCommits returns committed generation history.
func (api *API) ListCommits(ctx context.Context) (params.BranchResults, error) {
	if err := api.checkCanRead(ctx); err != nil {
		return params.BranchResults{}, errors.Trace(err)
	}
	commits, err := api.generationService.ListCommits(ctx)
	if err != nil {
		return params.BranchResults{Error: apiservererrors.ServerError(err)}, nil
	}
	result := params.BranchResults{Generations: make([]params.Generation, len(commits))}
	for i, commit := range commits {
		result.Generations[i], err = encodeCommit(commit)
		if err != nil {
			return params.BranchResults{Error: apiservererrors.ServerError(err)}, nil
		}
	}
	return result, nil
}

// ShowCommit returns a committed generation by id.
func (api *API) ShowCommit(ctx context.Context, arg params.GenerationId) (params.GenerationResult, error) {
	if err := api.checkCanRead(ctx); err != nil {
		return params.GenerationResult{}, errors.Trace(err)
	}
	if arg.GenerationId < 0 {
		return params.GenerationResult{
			Error: apiservererrors.ServerError(
				internalerrors.Errorf("generation id cannot be negative"),
			),
		}, nil
	}
	commit, err := api.generationService.GetCommit(ctx, uint64(arg.GenerationId))
	if err != nil {
		return params.GenerationResult{Error: apiservererrors.ServerError(err)}, nil
	}
	result, err := encodeCommit(commit)
	return params.GenerationResult{
		Generation: result,
		Error:      apiservererrors.ServerError(err),
	}, nil
}

func encodeGeneration(value generation.Generation) (params.Generation, error) {
	id, err := generationIDToInt(value.GenerationID)
	if err != nil {
		return params.Generation{}, internalerrors.Capture(err)
	}
	result := params.Generation{
		BranchName:   value.Name,
		Created:      value.CreatedAt.Unix(),
		CreatedBy:    value.CreatedBy,
		GenerationId: id,
	}
	if !value.CompletedAt.IsZero() {
		result.Completed = value.CompletedAt.Unix()
		result.CompletedBy = value.CompletedBy
	}
	return result, nil
}

func encodeCommit(value generation.Commit) (params.Generation, error) {
	id, err := generationIDToInt(value.GenerationID)
	if err != nil {
		return params.Generation{}, internalerrors.Capture(err)
	}
	result := params.Generation{
		BranchName:   value.Name,
		Completed:    value.CommittedAt.Unix(),
		CompletedBy:  value.CommittedBy,
		GenerationId: id,
		CreatedBy:    value.CreatedBy,
		Applications: make([]params.GenerationApplication, len(value.Applications)),
	}
	for i, app := range value.Applications {
		changes := make(map[string]any, len(app.Config))
		for _, change := range app.Config {
			changes[change.Key] = change.Value
		}
		result.Applications[i] = params.GenerationApplication{
			ApplicationName: app.ApplicationName,
			ConfigChanges:   changes,
		}
	}
	return result, nil
}

func generationIDToInt(id uint64) (int, error) {
	maxInt := uint64(^uint(0) >> 1)
	if id > maxInt {
		return 0, internalerrors.Errorf("generation id %d overflows int", id)
	}
	return int(id), nil
}
