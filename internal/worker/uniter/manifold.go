// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter

import (
	stdcontext "context"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"github.com/juju/worker/v5"
	"github.com/juju/worker/v5/dependency"
	"gopkg.in/yaml.v2"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/agent/tools"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/agent/secretsmanager"
	"github.com/juju/juju/api/agent/uniter"
	"github.com/juju/juju/api/client/charms"
	"github.com/juju/juju/core/leadership"
	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/machinelock"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/objectstore"
	coretrace "github.com/juju/juju/core/trace"
	"github.com/juju/juju/domain/deployment/charm/hooks"
	"github.com/juju/juju/internal/observability/probe"
	"github.com/juju/juju/internal/s3client"
	"github.com/juju/juju/internal/secrets"
	"github.com/juju/juju/internal/worker/common/reboot"
	"github.com/juju/juju/internal/worker/fortress"
	"github.com/juju/juju/internal/worker/secretexpire"
	"github.com/juju/juju/internal/worker/secretrotate"
	"github.com/juju/juju/internal/worker/trace"
	"github.com/juju/juju/internal/worker/uniter/holisticuniter"
	"github.com/juju/juju/internal/worker/uniter/shared"
	uniterapi "github.com/juju/juju/internal/worker/uniter/shared/api"
	"github.com/juju/juju/internal/worker/uniter/shared/charm"
	unitercontext "github.com/juju/juju/internal/worker/uniter/shared/context"
	"github.com/juju/juju/internal/worker/uniter/shared/hook"
	"github.com/juju/juju/internal/worker/uniter/shared/jujuc"
	"github.com/juju/juju/internal/worker/uniter/shared/operation"
	"github.com/juju/juju/internal/worker/uniter/shared/relation"
	"github.com/juju/juju/internal/worker/uniter/shared/resolver"
	"github.com/juju/juju/internal/worker/uniter/shared/runner"
	"github.com/juju/juju/rpc/params"
)

// ManifoldConfig defines the names of the manifolds on which a
// Manifold will depend.
type ManifoldConfig struct {
	AgentName             string
	APICallerName         string
	S3CallerName          string
	LeadershipTrackerName string
	CharmDirName          string
	HookRetryStrategyName string
	TraceName             string

	ModelType                    model.ModelType
	MachineLock                  machinelock.Lock
	Clock                        clock.Clock
	TranslateResolverErr         func(error) error
	Logger                       logger.Logger
	Sidecar                      bool
	EnforcedCharmModifiedVersion int
	ContainerNames               []string
}

// unitWorkerFactory selects the implementation that converges a unit. Both
// implementations receive their dependencies explicitly from the manifold.
type unitWorkerFactory struct {
	newDelta    func() (worker.Worker, error)
	newHolistic func(stdcontext.Context) (worker.Worker, error)
}

func (f unitWorkerFactory) New(ctx stdcontext.Context, runtimeType string) (worker.Worker, error) {
	switch runtimeType {
	case "", "delta":
		if f.newDelta == nil {
			return nil, errors.NotValidf("missing delta uniter constructor")
		}
		return f.newDelta()
	case "holistic":
		if f.newHolistic == nil {
			return nil, errors.NotValidf("missing holistic uniter constructor")
		}
		return f.newHolistic(ctx)
	default:
		return nil, errors.NotValidf("unknown unit runtime type %q", runtimeType)
	}
}

type deltaUniterFactoryConfig struct {
	manifoldConfig       ManifoldConfig
	dataDir              string
	transientDataDir     string
	unitTag              names.UnitTag
	unitClient           *uniter.Client
	resourcesClient      *uniter.ResourcesFacadeClient
	secretsClient        *secretsmanager.Client
	secretsBackendGetter unitercontext.SecretsBackendGetter
	leadershipTracker    leadership.Tracker
	charmDirGuard        fortress.Guard
	hookRetryStrategy    params.RetryStrategy
	downloader           charm.Downloader
	tracer               coretrace.Tracer
}

type holisticUniterFactoryConfig struct {
	manifoldConfig       ManifoldConfig
	dataDir              string
	unitTag              names.UnitTag
	unit                 *uniter.Unit
	unitClient           *uniter.Client
	resourcesClient      *uniter.ResourcesFacadeClient
	secretsClient        *secretsmanager.Client
	secretsBackendGetter unitercontext.SecretsBackendGetter
	leadershipTracker    leadership.Tracker
	charmDirGuard        fortress.Guard
	downloader           charm.Downloader
	hookRetryStrategy    params.RetryStrategy
}

// unitLifecycleStore keeps holistic lifecycle progress in the same
// controller-backed UniterState field used by deltauniter. Snapshots are never
// persisted here.
type unitLifecycleStore struct {
	unit    *uniter.Unit
	unitTag names.UnitTag
}

func (s unitLifecycleStore) Load(ctx stdcontext.Context) (holisticuniter.LifecycleState, error) {
	unitState, err := s.unit.State(ctx)
	if err != nil {
		return holisticuniter.LifecycleState{}, errors.Trace(err)
	}
	if unitState.UniterState == "" {
		return holisticuniter.LifecycleState{}, nil
	}
	var state holisticuniter.LifecycleState
	if err := yaml.Unmarshal([]byte(unitState.UniterState), &state); err != nil {
		return holisticuniter.LifecycleState{}, errors.Annotate(err, "decoding holistic lifecycle state")
	}
	return state, nil
}

func (s unitLifecycleStore) Save(ctx stdcontext.Context, state holisticuniter.LifecycleState) error {
	encoded, err := yaml.Marshal(state)
	if err != nil {
		return errors.Annotate(err, "encoding holistic lifecycle state")
	}
	value := string(encoded)
	return s.unit.SetState(ctx, params.SetUnitStateArg{
		Tag:         s.unitTag.String(),
		UniterState: &value,
	})
}

// Validate ensures all the required values for the config are set.
func (config *ManifoldConfig) Validate() error {
	if config.Clock == nil {
		return errors.NotValidf("missing Clock")
	}
	if len(config.ModelType) == 0 {
		return errors.NotValidf("missing model type")
	}
	if config.MachineLock == nil {
		return errors.NotValidf("missing MachineLock")
	}
	if config.Logger == nil {
		return errors.NotValidf("missing Logger")
	}
	return nil
}

// Manifold returns a dependency manifold that runs a uniter worker,
// using the resource names defined in the supplied config.
func Manifold(config ManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: []string{
			config.AgentName,
			config.APICallerName,
			config.S3CallerName,
			config.LeadershipTrackerName,
			config.CharmDirName,
			config.HookRetryStrategyName,
			config.TraceName,
		},
		Start: func(ctx stdcontext.Context, getter dependency.Getter) (worker.Worker, error) {
			if err := config.Validate(); err != nil {
				return nil, errors.Trace(err)
			}
			// Collect all required resources.
			var agent agent.Agent
			if err := getter.Get(config.AgentName, &agent); err != nil {
				return nil, errors.Trace(err)
			}
			var apiConn api.Connection
			if err := getter.Get(config.APICallerName, &apiConn); err != nil {
				// TODO(fwereade): absence of an APICaller shouldn't be the end
				// of the world -- we ought to return a type that can at least
				// run the leader-deposed hook -- but that's not done yet.
				return nil, errors.Trace(err)
			}
			var leadershipTracker leadership.Tracker
			if err := getter.Get(config.LeadershipTrackerName, &leadershipTracker); err != nil {
				return nil, errors.Trace(err)
			}
			var charmDirGuard fortress.Guard
			if err := getter.Get(config.CharmDirName, &charmDirGuard); err != nil {
				return nil, errors.Trace(err)
			}

			var hookRetryStrategy params.RetryStrategy
			if err := getter.Get(config.HookRetryStrategyName, &hookRetryStrategy); err != nil {
				return nil, errors.Trace(err)
			}

			// Ensure the agent is correctly configured with a unit tag.
			agentConfig := agent.CurrentConfig()
			tag := agentConfig.Tag()
			unitTag, ok := tag.(names.UnitTag)
			if !ok {
				return nil, errors.Errorf("expected a unit tag, got %v", tag)
			}

			// Get the tracer from the context.
			var tracerGetter trace.TracerGetter
			if err := getter.Get(config.TraceName, &tracerGetter); err != nil {
				return nil, errors.Trace(err)
			}

			tracer, err := tracerGetter.GetTracer(ctx, coretrace.Namespace("uniter", agentConfig.Model().Id()))
			if err != nil {
				tracer = coretrace.NoopTracer{}
			}

			var objectStoreCaller objectstore.Session
			if err := getter.Get(config.S3CallerName, &objectStoreCaller); err != nil {
				return nil, errors.Trace(err)
			}

			s3Downloader := charms.NewS3CharmDownloader(s3client.NewBlobsS3Client(objectStoreCaller), apiConn)

			jujuSecretsAPI := secretsmanager.NewClient(apiConn, uniter.WithTracer(tracer))
			resourcesClient, err := uniter.NewResourcesFacadeClient(apiConn, unitTag)
			if err != nil {
				return nil, err
			}

			secretsBackendGetter := func() (uniterapi.SecretsBackend, error) {
				return secrets.NewClient(jujuSecretsAPI)
			}

			unitClient := uniter.NewClient(apiConn, unitTag, uniter.WithTracer(tracer))
			unit, err := unitClient.Unit(ctx, unitTag)
			if err != nil {
				return nil, errors.Trace(err)
			}
			deltaConfig := deltaUniterFactoryConfig{
				manifoldConfig:       config,
				dataDir:              agentConfig.DataDir(),
				transientDataDir:     agentConfig.TransientDataDir(),
				unitTag:              unitTag,
				unitClient:           unitClient,
				resourcesClient:      resourcesClient,
				secretsClient:        jujuSecretsAPI,
				secretsBackendGetter: secretsBackendGetter,
				leadershipTracker:    leadershipTracker,
				charmDirGuard:        charmDirGuard,
				hookRetryStrategy:    hookRetryStrategy,
				downloader:           s3Downloader,
				tracer:               tracer,
			}
			holisticConfig := holisticUniterFactoryConfig{
				manifoldConfig:       config,
				dataDir:              agentConfig.DataDir(),
				unitTag:              unitTag,
				unit:                 unit,
				unitClient:           unitClient,
				resourcesClient:      resourcesClient,
				secretsClient:        jujuSecretsAPI,
				secretsBackendGetter: secretsBackendGetter,
				leadershipTracker:    leadershipTracker,
				charmDirGuard:        charmDirGuard,
				downloader:           s3Downloader,
				hookRetryStrategy:    hookRetryStrategy,
			}
			factory := unitWorkerFactory{
				newDelta: func() (worker.Worker, error) {
					return newDeltaUniter(deltaConfig)
				},
				newHolistic: func(ctx stdcontext.Context) (worker.Worker, error) {
					return newHolisticUniter(ctx, holisticConfig)
				},
			}
			return factory.New(ctx, unit.RuntimeType())
		},
		Output: output,
	}
}

func newDeltaUniter(config deltaUniterFactoryConfig) (worker.Worker, error) {
	leadershipTrackerFunc := func(_ names.UnitTag) leadership.Tracker {
		return config.leadershipTracker
	}
	secretRotateWatcherFunc := func(unitTag names.UnitTag, isLeader bool, rotateSecrets chan []string) (worker.Worker, error) {
		owners := []names.Tag{unitTag}
		if isLeader {
			appName, _ := names.UnitApplication(unitTag.Id())
			owners = append(owners, names.NewApplicationTag(appName))
		}
		return secretrotate.New(secretrotate.Config{
			SecretManagerFacade: config.secretsClient,
			Clock:               config.manifoldConfig.Clock,
			Logger:              config.manifoldConfig.Logger.Child("secretsrotate"),
			SecretOwners:        owners,
			RotateSecrets:       rotateSecrets,
		})
	}
	secretExpiryWatcherFunc := func(unitTag names.UnitTag, isLeader bool, expireRevisions chan []string) (worker.Worker, error) {
		owners := []names.Tag{unitTag}
		if isLeader {
			appName, _ := names.UnitApplication(unitTag.Id())
			owners = append(owners, names.NewApplicationTag(appName))
		}
		return secretexpire.New(secretexpire.Config{
			SecretManagerFacade: config.secretsClient,
			Clock:               config.manifoldConfig.Clock,
			Logger:              config.manifoldConfig.Logger.Child("secretrevisionsexpire"),
			SecretOwners:        owners,
			ExpireRevisions:     expireRevisions,
		})
	}

	worker, err := NewUniter(&UniterParams{
		UniterClient: uniterapi.UniterClientShim{
			Client: config.unitClient,
		},
		ResourcesClient:              config.resourcesClient,
		SecretsClient:                config.secretsClient,
		SecretsBackendGetter:         config.secretsBackendGetter,
		UnitTag:                      config.unitTag,
		ModelType:                    config.manifoldConfig.ModelType,
		LeadershipTrackerFunc:        leadershipTrackerFunc,
		SecretRotateWatcherFunc:      secretRotateWatcherFunc,
		SecretExpiryWatcherFunc:      secretExpiryWatcherFunc,
		DataDir:                      config.dataDir,
		Downloader:                   config.downloader,
		MachineLock:                  config.manifoldConfig.MachineLock,
		CharmDirGuard:                config.charmDirGuard,
		UpdateStatusSignal:           NewUpdateStatusTimer(),
		HookRetryStrategy:            config.hookRetryStrategy,
		NewOperationExecutor:         operation.NewExecutor,
		NewDeployer:                  charm.NewDeployer,
		NewProcessRunner:             runner.NewRunner,
		TranslateResolverErr:         config.manifoldConfig.TranslateResolverErr,
		Clock:                        config.manifoldConfig.Clock,
		RebootQuerier:                reboot.NewMonitor(config.transientDataDir),
		Logger:                       config.manifoldConfig.Logger,
		Sidecar:                      config.manifoldConfig.Sidecar,
		EnforcedCharmModifiedVersion: config.manifoldConfig.EnforcedCharmModifiedVersion,
		ContainerNames:               config.manifoldConfig.ContainerNames,
		Tracer:                       config.tracer,
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	return worker, nil
}

func newHolisticUniter(ctx stdcontext.Context, config holisticUniterFactoryConfig) (worker.Worker, error) {
	planner, err := holisticuniter.NewPersistentLifecyclePlanner(ctx, unitLifecycleStore{
		unit:    config.unit,
		unitTag: config.unitTag,
	})
	if err != nil {
		return nil, errors.Annotate(err, "creating holistic lifecycle planner")
	}
	paths := shared.NewPaths(config.dataDir, config.unitTag, nil)
	if err := tools.EnsureSymlinks(paths.ToolsDir, paths.ToolsDir, jujuc.CommandNames()); err != nil {
		return nil, errors.Trace(err)
	}

	unitShim := uniterapi.UnitShim{Unit: config.unit}
	relationStateTracker, err := relation.NewRelationStateTracker(ctx, relation.RelationStateTrackerConfig{
		Client:            uniterapi.UniterClientShim{Client: config.unitClient},
		Unit:              unitShim,
		LeadershipContext: unitercontext.NewLeadershipContext(config.leadershipTracker),
		CharmDir:          paths.State.CharmDir,
		Abort:             ctx.Done(),
		Logger:            config.manifoldConfig.Logger.Child("relation"),
	})
	if err != nil {
		return nil, errors.Annotate(err, "creating relation state tracker")
	}

	contextFactory, err := unitercontext.NewContextFactory(ctx, unitercontext.FactoryConfig{
		Uniter:               uniterapi.UniterClientShim{Client: config.unitClient},
		SecretsClient:        config.secretsClient,
		SecretsBackendGetter: config.secretsBackendGetter,
		Unit:                 unitShim,
		Resources:            config.resourcesClient,
		Tracker:              config.leadershipTracker,
		GetRelationInfos:     relationStateTracker.GetInfo,
		Paths:                paths,
		Clock:                config.manifoldConfig.Clock,
		Logger:               config.manifoldConfig.Logger.Child("context"),
	})
	if err != nil {
		return nil, errors.Annotate(err, "creating hook context factory")
	}
	if err := charm.ClearDownloads(paths.State.BundlesDir); err != nil {
		config.manifoldConfig.Logger.Warningf(ctx, "clearing charm downloads: %v", err)
	}
	charmLogger := config.manifoldConfig.Logger.Child("charm")
	deployer, err := charm.NewDeployer(
		paths.State.CharmDir,
		paths.State.DeployerDir,
		charm.NewBundlesDir(paths.State.BundlesDir, config.downloader, charmLogger),
		charmLogger,
	)
	if err != nil {
		return nil, errors.Annotate(err, "creating charm deployer")
	}

	return holisticuniter.NewRuntime(ctx, holisticuniter.RuntimeConfig{
		Unit:          config.unit,
		Planner:       planner,
		Deployer:      deployer,
		CharmDirGuard: config.charmDirGuard,
		RetryStrategy: config.hookRetryStrategy,
		Clock:         config.manifoldConfig.Clock,
		Charm: func(_ stdcontext.Context, snapshot params.UnitSnapshot) (charm.BundleInfo, error) {
			return config.unitClient.Charm(snapshot.CharmURL)
		},
		NewHookRunner: func(ctx stdcontext.Context, info hook.Info, snapshot params.UnitSnapshot) (runner.Runner, error) {
			hookContext, err := contextFactory.HookContext(ctx, info)
			if err != nil {
				return nil, errors.Trace(err)
			}
			hookContext.SetUnitSnapshot(snapshot)
			return runner.NewRunner(hookContext, paths), nil
		},
		Lock: func(ctx stdcontext.Context, event hooks.Kind) (func(), error) {
			return config.manifoldConfig.MachineLock.Acquire(machinelock.Spec{
				Cancel:  ctx.Done(),
				Worker:  "uniter",
				Comment: "run " + string(event) + " hook",
			})
		},
	})
}

type probeProvider interface {
	ProbeProvider() probe.ProbeProvider
}

var (
	_ probeProvider = (*Uniter)(nil)
	_ probeProvider = (*holisticuniter.HolisticUniter)(nil)
)

func output(in worker.Worker, out any) error {
	switch outPtr := out.(type) {
	case *probe.ProbeProvider:
		provider, ok := in.(probeProvider)
		if !ok {
			return errors.Errorf("expected uniter worker in")
		}
		*outPtr = provider.ProbeProvider()
	case **Uniter:
		uniter, ok := in.(*Uniter)
		if !ok {
			return errors.Errorf("expected delta Uniter in")
		}
		*outPtr = uniter
	default:
		return errors.Errorf("unknown out type")
	}
	return nil
}

// TranslateFortressErrors turns errors returned by dependent
// manifolds due to fortress lockdown (i.e. model migration) into an
// error which causes the resolver loop to be restarted. When this
// happens the uniter is about to be shut down anyway.
func TranslateFortressErrors(err error) error {
	if fortress.IsFortressError(err) {
		return resolver.ErrRestart
	}
	return err
}
