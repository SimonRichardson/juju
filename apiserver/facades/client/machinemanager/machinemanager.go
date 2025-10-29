// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package machinemanager

import (
	"context"
	"fmt"
	"time"

	"github.com/juju/clock"
	"github.com/juju/collections/set"
	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/apiserver/common"
	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/apiserver/facade"
	corebase "github.com/juju/juju/core/base"
	"github.com/juju/juju/core/instance"
	corelogger "github.com/juju/juju/core/logger"
	coremachine "github.com/juju/juju/core/machine"
	coremodel "github.com/juju/juju/core/model"
	"github.com/juju/juju/core/objectstore"
	"github.com/juju/juju/core/os/ostype"
	"github.com/juju/juju/core/permission"
	"github.com/juju/juju/core/status"
	coreunit "github.com/juju/juju/core/unit"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	"github.com/juju/juju/domain/constraints"
	"github.com/juju/juju/domain/deployment"
	domainmachine "github.com/juju/juju/domain/machine"
	machineerrors "github.com/juju/juju/domain/machine/errors"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/environs/manual/sshprovisioner"
	internalerrors "github.com/juju/juju/internal/errors"
	"github.com/juju/juju/rpc/params"
)

// MachineManagerAPI provides access to the MachineManager API facade.
type MachineManagerAPI struct {
	controllerUUID  string
	modelUUID       coremodel.UUID
	authorizer      Authorizer
	check           *common.BlockChecker
	controllerStore objectstore.ObjectStore
	clock           clock.Clock

	agentBinaryService      AgentBinaryService
	agentPasswordService    AgentPasswordService
	applicationService      ApplicationService
	cloudService            CloudService
	controllerConfigService ControllerConfigService
	controllerNodeService   ControllerNodeService
	keyUpdaterService       KeyUpdaterService
	machineService          MachineService
	statusService           StatusService
	modelConfigService      ModelConfigService
	networkService          NetworkService
	removalService          RemovalService

	logger corelogger.Logger
}

// NewMachineManagerAPI creates a new server-side MachineManager API facade.
func NewMachineManagerAPI(
	controllerUUID string,
	modelUUID coremodel.UUID,
	controllerStore objectstore.ObjectStore,
	auth Authorizer,
	logger corelogger.Logger,
	clock clock.Clock,
	services Services,
) *MachineManagerAPI {
	api := &MachineManagerAPI{
		controllerUUID:  controllerUUID,
		modelUUID:       modelUUID,
		controllerStore: controllerStore,
		authorizer:      auth,
		check:           common.NewBlockChecker(services.BlockCommandService),
		clock:           clock,
		logger:          logger,

		agentBinaryService:      services.AgentBinaryService,
		agentPasswordService:    services.AgentPasswordService,
		applicationService:      services.ApplicationService,
		controllerConfigService: services.ControllerConfigService,
		controllerNodeService:   services.ControllerNodeService,
		cloudService:            services.CloudService,
		keyUpdaterService:       services.KeyUpdaterService,
		machineService:          services.MachineService,
		statusService:           services.StatusService,
		modelConfigService:      services.ModelConfigService,
		networkService:          services.NetworkService,
		removalService:          services.RemovalService,
	}
	return api
}

// AddMachines adds new machines with the supplied parameters.
// The args will contain Base info.
func (mm *MachineManagerAPI) AddMachines(ctx context.Context, args params.AddMachines) (params.AddMachinesResults, error) {
	results := params.AddMachinesResults{
		Machines: make([]params.AddMachinesResult, len(args.MachineParams)),
	}
	if err := mm.authorizer.CanWrite(ctx); err != nil {
		return results, err
	}
	if err := mm.check.ChangeAllowed(ctx); err != nil {
		return results, errors.Trace(err)
	}

	for i, p := range args.MachineParams {
		machineName, err := mm.addOneMachine(ctx, p)
		results.Machines[i].Error = apiservererrors.ServerError(err)
		if err == nil {
			results.Machines[i].Machine = machineName.String()
		}
	}
	return results, nil
}

func (mm *MachineManagerAPI) addOneMachine(ctx context.Context, p params.AddMachineParams) (coremachine.Name, error) {
	if p.ParentId != "" && p.ContainerType == "" {
		return "", internalerrors.New("parent machine specified without container type")
	}
	if p.ContainerType != "" && p.Placement != nil {
		return "", internalerrors.New("container type and placement are mutually exclusive")
	}
	if p.ContainerType != "" || p.Placement != nil {
		// Guard against dubious client by making sure that
		// the following attributes can only be set when we're
		// not using placement.
		p.InstanceId = ""
		p.Nonce = ""
		p.HardwareCharacteristics = instance.HardwareCharacteristics{}
		p.Addrs = nil
	}

	var base corebase.Base
	if p.Base == nil {
		conf, err := mm.modelConfigService.ModelConfig(ctx)
		if err != nil {
			return "", errors.Trace(err)
		}
		base = config.PreferredBase(conf)
	} else {
		var err error
		base, err = corebase.ParseBase(p.Base.Name, p.Base.Channel)
		if err != nil {
			return "", errors.Trace(err)
		}
	}

	// Check if the model exists in case of model scope placement.
	parsedPlacement, err := deployment.ParsePlacement(p.Placement)
	if err != nil {
		return "", internalerrors.Errorf("invalid placement: %w", err)
	}
	if parsedPlacement.Type == deployment.PlacementTypeProvider && parsedPlacement.Directive != mm.modelUUID.String() {
		return "", internalerrors.Errorf("invalid model id %q", parsedPlacement.Directive)
	}

	var n *string
	if p.Nonce != "" {
		n = &p.Nonce
	}
	osType, err := encodeOSType(base.OS)
	if err != nil {
		return "", internalerrors.Errorf("invalid placement: %w", err)
	}
	addedMachine, err := mm.machineService.AddMachine(ctx, domainmachine.AddMachineArgs{
		Nonce:       n,
		Constraints: constraints.DecodeConstraints(p.Constraints),
		Platform: deployment.Platform{
			Channel: base.Channel.String(),
			OSType:  osType,
		},
		Directive:               parsedPlacement,
		HardwareCharacteristics: p.HardwareCharacteristics,
	})

	return addedMachine.MachineName, err
}

func encodeOSType(os string) (deployment.OSType, error) {
	switch ostype.OSTypeForName(os) {
	case ostype.Ubuntu:
		return deployment.Ubuntu, nil
	default:
		return 0, errors.Errorf("unknown os type %q, expected ubuntu", os)
	}
}

// ProvisioningScript returns a shell script that, when run,
// provisions a machine agent on the machine executing the script.
func (mm *MachineManagerAPI) ProvisioningScript(ctx context.Context, args params.ProvisioningScriptParams) (params.ProvisioningScriptResult, error) {
	if err := mm.authorizer.CanWrite(ctx); err != nil {
		return params.ProvisioningScriptResult{}, err
	}

	var result params.ProvisioningScriptResult

	services := InstanceConfigServices{
		CloudService:            mm.cloudService,
		ControllerConfigService: mm.controllerConfigService,
		ControllerNodeService:   mm.controllerNodeService,
		ObjectStore:             mm.controllerStore,
		KeyUpdaterService:       mm.keyUpdaterService,
		ModelConfigService:      mm.modelConfigService,
		MachineService:          mm.machineService,
		AgentBinaryService:      mm.agentBinaryService,
		AgentPasswordService:    mm.agentPasswordService,
	}

	machineName := coremachine.Name(args.MachineId)
	icfg, err := InstanceConfig(
		ctx,
		mm.controllerUUID,
		mm.modelUUID,
		services, machineName, args.Nonce, args.DataDir)
	if err != nil {
		return result, apiservererrors.ServerError(errors.Annotate(
			err, "getting instance config",
		))
	}

	// Until DisablePackageCommands is retired, for backwards
	// compatibility, we must respect the client's request and
	// override any model settings the user may have specified.
	// If the client does specify this setting, it will only ever be
	// true. False indicates the client doesn't care and we should use
	// what's specified in the environment config.
	if args.DisablePackageCommands {
		icfg.EnableOSRefreshUpdate = false
		icfg.EnableOSUpgrade = false
	} else {
		config, err := mm.modelConfigService.ModelConfig(ctx)
		if err != nil {
			mm.logger.Errorf(ctx,
				"cannot getting model config for provisioning machine %q: %v",
				args.MachineId, err,
			)
			return result, errors.New("controller failed to get model config for machine")
		}

		icfg.EnableOSUpgrade = config.EnableOSUpgrade()
		icfg.EnableOSRefreshUpdate = config.EnableOSRefreshUpdate()
	}

	getProvisioningScript := sshprovisioner.ProvisioningScript
	result.Script, err = getProvisioningScript(icfg)
	if err != nil {
		return result, apiservererrors.ServerError(errors.Annotate(
			err, "getting provisioning script",
		))
	}

	return result, nil
}

// RetryProvisioning marks a provisioning error as transient on the machines.
func (mm *MachineManagerAPI) RetryProvisioning(ctx context.Context, p params.RetryProvisioningArgs) (params.ErrorResults, error) {
	if err := mm.authorizer.CanWrite(ctx); err != nil {
		return params.ErrorResults{}, err
	}

	if err := mm.check.ChangeAllowed(ctx); err != nil {
		return params.ErrorResults{}, errors.Trace(err)
	}
	result := params.ErrorResults{}
	machineNames, err := mm.machineService.AllMachineNames(ctx)
	if err != nil {
		return params.ErrorResults{}, errors.Trace(err)
	}
	wanted := set.NewStrings()
	for _, tagStr := range p.Machines {
		tag, err := names.ParseMachineTag(tagStr)
		if err != nil {
			result.Results = append(result.Results, params.ErrorResult{Error: apiservererrors.ServerError(err)})
			continue
		}
		wanted.Add(tag.Id())
	}
	for _, machineName := range machineNames {
		if !p.All && !wanted.Contains(machineName.String()) {
			continue
		}
		if err := mm.maybeUpdateInstanceStatus(ctx, p.All, machineName, map[string]interface{}{"transient": true}); err != nil {
			result.Results = append(result.Results, params.ErrorResult{Error: apiservererrors.ServerError(err)})
		}
	}
	return result, nil
}

func (mm *MachineManagerAPI) maybeUpdateInstanceStatus(ctx context.Context, all bool, machineName coremachine.Name, data map[string]interface{}) error {
	existingStatusInfo, err := mm.statusService.GetInstanceStatus(ctx, machineName)
	if errors.Is(err, machineerrors.MachineNotFound) {
		return errors.NotFoundf("machine %q", machineName)
	} else if err != nil {
		return err
	}

	newData := existingStatusInfo.Data
	if newData == nil {
		newData = data
	} else {
		for k, v := range data {
			newData[k] = v
		}
	}
	if len(newData) > 0 && existingStatusInfo.Status != status.Error && existingStatusInfo.Status != status.ProvisioningError {
		// If a specifc machine has been asked for and it's not in error, that's a problem.
		if !all {
			return fmt.Errorf("machine %s is not in an error state (%v)", machineName, existingStatusInfo.Status)
		}
		// Otherwise just skip it.
		return nil
	}
	now := mm.clock.Now()
	sInfo := status.StatusInfo{
		Status:  existingStatusInfo.Status,
		Message: existingStatusInfo.Message,
		Data:    newData,
		Since:   &now,
	}
	err = mm.statusService.SetInstanceStatus(ctx, machineName, sInfo)
	if errors.Is(err, machineerrors.MachineNotFound) {
		return errors.NotFoundf("machine %q", machineName)
	} else if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// DestroyMachineWithParams removes a set of machines from the model.
func (mm *MachineManagerAPI) DestroyMachineWithParams(ctx context.Context, args params.DestroyMachinesParams) (params.DestroyMachineResults, error) {
	entities := params.Entities{Entities: make([]params.Entity, len(args.MachineTags))}
	for i, tag := range args.MachineTags {
		entities.Entities[i].Tag = tag
	}
	return mm.destroyMachine(ctx, entities, args.Force, args.Keep, args.DryRun, common.MaxWait(args.MaxWait))
}

func (mm *MachineManagerAPI) destroyMachine(ctx context.Context, args params.Entities, force, keep, dryRun bool, maxWait time.Duration) (params.DestroyMachineResults, error) {
	if err := mm.authorizer.CanWrite(ctx); err != nil {
		return params.DestroyMachineResults{}, err
	}
	if err := mm.check.RemoveAllowed(ctx); err != nil {
		return params.DestroyMachineResults{}, err
	}
	results := make([]params.DestroyMachineResult, len(args.Entities))
	for i, entity := range args.Entities {
		result := params.DestroyMachineResult{}
		fail := func(e error) {
			result.Error = apiservererrors.ServerError(e)
			results[i] = result
		}

		machineTag, err := names.ParseMachineTag(entity.Tag)
		if err != nil {
			fail(err)
			continue
		}
		machineName := coremachine.Name(machineTag.Id())
		machineUUID, err := mm.machineService.GetMachineUUID(ctx, machineName)
		if err != nil {
			fail(internalerrors.Errorf("getting machine UUID: %w", err))
			continue
		}

		containers, err := mm.machineService.GetMachineContainers(ctx, machineUUID)
		if err != nil {
			fail(internalerrors.Errorf("getting machine containers: %w", err))
			continue
		}
		info, err := mm.calculateDestroyResult(ctx, machineName, containers)
		if err != nil {
			fail(err)
			continue
		}

		if dryRun {
			result.Info = &info
			results[i] = result
			continue
		}

		if keep {
			mm.logger.Infof(ctx, "destroy machine %v but keep instance", machineName)
			if err := mm.machineService.SetKeepInstance(ctx, machineName, keep); err != nil {
				if !force {
					fail(err)
					continue
				}
				mm.logger.Warningf(ctx, "could not keep instance for machine %v: %v", machineName, err)
			}
		}

		if _, err := mm.removalService.RemoveMachine(ctx, machineUUID, force, maxWait); err != nil {
			fail(internalerrors.Errorf("removing machine: %w", err))
			continue
		}

		result.Info = &info
		results[i] = result
	}
	return params.DestroyMachineResults{Results: results}, nil
}

func (mm *MachineManagerAPI) calculateDestroyResult(ctx context.Context, machineName coremachine.Name, containers []coremachine.Name) (params.DestroyMachineInfo, error) {
	info, err := mm.destroyResultForMachine(ctx, machineName)
	if err != nil {
		return info, errors.Trace(err)
	}

	if len(containers) > 0 {
		info.DestroyedContainers = make([]params.DestroyMachineResult, len(containers))
		for i, container := range containers {
			containerInfo, err := mm.destroyResultForMachine(ctx, container)
			if err != nil {
				return info, errors.Trace(err)
			}
			info.DestroyedContainers[i].Info = &containerInfo
		}
	}

	return info, nil
}

func (mm *MachineManagerAPI) destroyResultForMachine(ctx context.Context, machineName coremachine.Name) (params.DestroyMachineInfo, error) {
	info := params.DestroyMachineInfo{
		MachineId: machineName.String(),
	}

	unitNames, err := mm.applicationService.GetUnitNamesOnMachine(ctx, machineName)
	if errors.Is(err, applicationerrors.MachineNotFound) {
		return info, errors.NotFoundf("machine %s", machineName)
	} else if err != nil {
		return info, errors.Trace(err)
	}
	for _, unitName := range unitNames {
		unitTag := names.NewUnitTag(unitName.String())
		info.DestroyedUnits = append(info.DestroyedUnits, params.Entity{Tag: unitTag.String()})
	}

	info.DestroyedStorage, info.DetachedStorage, err = mm.classifyDetachedStorage(unitNames)
	if err != nil {
		return info, internalerrors.Errorf("classifying storage for machine %q: %w", machineName, err)
	}

	return info, nil
}

func (mm *MachineManagerAPI) classifyDetachedStorage(unitNames []coreunit.Name) (destroyed, detached []params.Entity, _ error) {
	var storageErrors []params.ErrorResult

	// TODO(storage): classify detached storage

	err := params.ErrorResults{Results: storageErrors}.Combine()
	return destroyed, detached, err
}

// ModelAuthorizer defines if a given operation can be performed based on a
// model tag.
type ModelAuthorizer struct {
	ModelTag   names.ModelTag
	Authorizer facade.Authorizer
}

// CanRead checks to see if a read is possible. Returns an error if a read is
// not possible.
func (a ModelAuthorizer) CanRead(ctx context.Context) error {
	return a.checkAccess(ctx, permission.ReadAccess)
}

// CanWrite checks to see if a write is possible. Returns an error if a write
// is not possible.
func (a ModelAuthorizer) CanWrite(ctx context.Context) error {
	return a.checkAccess(ctx, permission.WriteAccess)
}

// AuthClient returns true if the entity is an external user.
func (a ModelAuthorizer) AuthClient() bool {
	return a.Authorizer.AuthClient()
}

func (a ModelAuthorizer) checkAccess(ctx context.Context, access permission.Access) error {
	return a.Authorizer.HasPermission(ctx, access, a.ModelTag)
}
