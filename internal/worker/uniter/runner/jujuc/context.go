// Copyright 2012, 2013, 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/core/application"
	"github.com/juju/juju/core/life"
	corelogger "github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/relation"
	"github.com/juju/juju/core/secrets"
	"github.com/juju/juju/internal/charm"
	"github.com/juju/juju/internal/storage"
	"github.com/juju/juju/rpc/params"
)

// Context is the interface that all hook helper commands
// depend on to interact with the rest of the system.
//
//go:generate go run go.uber.org/mock/mockgen -typed -package mocks -destination mocks/context_mock.go github.com/juju/juju/internal/worker/uniter/runner/jujuc Context
type Context interface {
	HookContext
	relationHookContext
	actionHookContext
	unitCharmStateContext
	workloadHookContext
}

// HookContext represents the information and functionality that is
// common to all charm hooks.
type HookContext interface {
	ContextUnit
	ContextStatus
	ContextInstance
	ContextNetworking
	ContextLeadership
	ContextStorage
	ContextResources
	ContextRelations
	ContextVersion
	ContextSecrets

	// GetLogger returns a juju logger Logger for the supplied module that is
	// correctly wired up for the given context
	GetLoggerByName(module string) corelogger.Logger
}

// UnitHookContext is the context for a unit hook.
type UnitHookContext interface {
	HookContext
}

// RelationHookContext is the context for a relation hook.
type RelationHookContext interface {
	HookContext
	relationHookContext
}

type relationHookContext interface {
	// HookRelation returns the ContextRelation associated with the executing
	// hook if it was found, or an error if it was not found (or is not available).
	HookRelation() (ContextRelation, error)

	// RemoteUnitName returns the name of the remote unit the hook execution
	// is associated with if it was found, and an error if it was not found or is not
	// available.
	RemoteUnitName() (string, error)

	// RemoteApplicationName returns the name of the remote application the hook execution
	// is associated with if it was found, and an error if it was not found or is not
	// available.
	RemoteApplicationName() (string, error)
}

// ActionHookContext is the context for an action hook.
type ActionHookContext interface {
	HookContext
	actionHookContext
}

type actionHookContext interface {
	// ActionParams returns the map of params passed with an Action.
	ActionParams() (map[string]interface{}, error)

	// UpdateActionResults inserts new values for use with action-set.
	// The results struct will be delivered to the controller upon
	// completion of the Action.
	UpdateActionResults(keys []string, value interface{}) error

	// SetActionMessage sets a message for the Action.
	SetActionMessage(string) error

	// SetActionFailed sets a failure state for the Action.
	SetActionFailed() error

	// LogActionMessage records a progress message for the Action.
	LogActionMessage(context.Context, string) error
}

// WorkloadHookContext is the context for a workload hook.
type WorkloadHookContext interface {
	HookContext
	workloadHookContext
}

type workloadHookContext interface {
	// WorkloadName returns the name of the container/workload for workload hooks.
	WorkloadName() (string, error)
}

// unitCharmStateContext provides helper for interacting with the charm state
// that is stored within the context.
type unitCharmStateContext interface {
	// GetCharmState returns a copy of the charm state.
	GetCharmState(context.Context) (map[string]string, error)

	// GetCharmStateValue returns the value of the given key.
	GetCharmStateValue(context.Context, string) (string, error)

	// DeleteCharmStateValue deletes the key/value pair for the given key.
	DeleteCharmStateValue(context.Context, string) error

	// SetCharmStateValue sets the key to the specified value.
	SetCharmStateValue(context.Context, string, string) error
}

// ContextUnit is the part of a hook context related to the unit.
type ContextUnit interface {
	// UnitName returns the executing unit's name.
	UnitName() string

	// ConfigSettings returns the current application
	// configuration of the executing unit.
	ConfigSettings(context.Context) (charm.Settings, error)

	// GoalState returns the goal state for the current unit.
	GoalState(context.Context) (*application.GoalState, error)

	// CloudSpec returns the unit's cloud specification
	CloudSpec(context.Context) (*params.CloudSpec, error)
}

// SecretCreateArgs specifies args used to create a secret.
// Nil values are not included in the create.
type SecretCreateArgs struct {
	SecretUpdateArgs

	Owner secrets.Owner
}

// SecretUpdateArgs specifies args used to update a secret.
// Nil values are not included in the update.
type SecretUpdateArgs struct {
	// Value is the new secret value or nil to not update.
	Value secrets.SecretValue

	RotatePolicy *secrets.RotatePolicy
	ExpireTime   *time.Time

	Description *string
	Label       *string
}

// SecretGrantRevokeArgs specify the args used to grant or revoke access to a secret.
type SecretGrantRevokeArgs struct {
	ApplicationName *string
	UnitName        *string
	RelationKey     *string
	Role            *secrets.SecretRole
}

// SecretMetadata holds a secret's metadata.
type SecretMetadata struct {
	Owner            secrets.Owner
	Description      string
	Label            string
	RotatePolicy     secrets.RotatePolicy
	LatestRevision   int
	LatestExpireTime *time.Time
	LatestChecksum   string
	NextRotateTime   *time.Time
	Revisions        []int
	Access           []secrets.AccessInfo
}

// ContextSecrets is the part of a hook context related to secrets.
type ContextSecrets interface {
	// GetSecret returns the value of the specified secret.
	GetSecret(context.Context, *secrets.URI, string, bool, bool) (secrets.SecretValue, error)

	// CreateSecret creates a secret with the specified data.
	CreateSecret(context.Context, *SecretCreateArgs) (*secrets.URI, error)

	// UpdateSecret creates a secret with the specified data.
	UpdateSecret(*secrets.URI, *SecretUpdateArgs) error

	// RemoveSecret removes a secret with the specified uri.
	RemoveSecret(*secrets.URI, *int) error

	// GrantSecret grants access to the specified secret.
	GrantSecret(*secrets.URI, *SecretGrantRevokeArgs) error

	// RevokeSecret revokes access to the specified secret.
	RevokeSecret(*secrets.URI, *SecretGrantRevokeArgs) error

	// SecretMetadata gets the secret metadata for secrets created by the charm.
	SecretMetadata() (map[string]SecretMetadata, error)
}

// ContextStatus is the part of a hook context related to the unit's status.
type ContextStatus interface {
	// UnitStatus returns the executing unit's current status.
	UnitStatus(context.Context) (*StatusInfo, error)

	// SetUnitStatus updates the unit's status.
	SetUnitStatus(context.Context, StatusInfo) error

	// ApplicationStatus returns the executing unit's application status
	// (including all units).
	ApplicationStatus(context.Context) (ApplicationStatusInfo, error)

	// SetApplicationStatus updates the status for the unit's application.
	SetApplicationStatus(context.Context, StatusInfo) error
}

// RebootPriority is the type used for reboot requests.
type RebootPriority int

// ContextInstance is the part of a hook context related to the unit's instance.
type ContextInstance interface {
	// AvailabilityZone returns the executing unit's availability zone or an error
	// if it was not found (or is not available).
	AvailabilityZone() (string, error)

	// RequestReboot will set the reboot flag to true on the machine agent
	RequestReboot(prio RebootPriority) error
}

// ContextNetworking is the part of a hook context related to network
// interface of the unit's instance.
type ContextNetworking interface {
	// PublicAddress returns the executing unit's public address or an
	// error if it is not available.
	PublicAddress(context.Context) (string, error)

	// PrivateAddress returns the executing unit's private address or an
	// error if it is not available.
	PrivateAddress() (string, error)

	// OpenPortRange marks the supplied port range for opening.
	OpenPortRange(endpointName string, portRange network.PortRange) error

	// ClosePortRange ensures the supplied port range is closed even when
	// the executing unit's application is exposed (unless it is opened
	// separately by a co-located unit).
	ClosePortRange(endpointName string, portRange network.PortRange) error

	// OpenedPortRanges returns all port ranges currently opened by this
	// unit on its assigned machine grouped by endpoint name.
	OpenedPortRanges() network.GroupedPortRanges

	// NetworkInfo returns the network info for the given bindings on the given relation.
	NetworkInfo(ctx context.Context, bindingNames []string, relationId int) (map[string]params.NetworkInfoResult, error)
}

// ContextLeadership is the part of a hook context related to the
// unit leadership.
type ContextLeadership interface {
	// IsLeader returns true if the local unit is known to be leader for at
	// least the next 30s.
	IsLeader() (bool, error)
}

// ContextStorage is the part of a hook context related to storage
// resources associated with the unit.
type ContextStorage interface {
	// StorageTags returns a list of tags for storage instances
	// attached to the unit or an error if they are not available.
	StorageTags(context.Context) ([]names.StorageTag, error)

	// Storage returns the ContextStorageAttachment with the supplied
	// tag if it was found, and an error if it was not found or is not
	// available to the context.
	Storage(context.Context, names.StorageTag) (ContextStorageAttachment, error)

	// HookStorage returns the storage attachment associated
	// the executing hook if it was found, and an error if it
	// was not found or is not available.
	HookStorage(context.Context) (ContextStorageAttachment, error)

	// AddUnitStorage saves storage directives in the context.
	AddUnitStorage(map[string]params.StorageDirectives) error
}

// ContextResources exposes the functionality needed by the
// "resource-*" hook commands.
type ContextResources interface {
	// DownloadResource downloads the named resource and returns
	// the path to which it was downloaded.
	DownloadResource(ctx context.Context, name string) (filePath string, _ error)
}

// ContextRelations exposes the relations associated with the unit.
type ContextRelations interface {
	// Relation returns the relation with the supplied id if it was found, and
	// an error if it was not found or is not available.
	Relation(id int) (ContextRelation, error)

	// RelationIds returns the ids of all relations the executing unit is
	// currently participating in or an error if they are not available.
	RelationIds() ([]int, error)
}

// ContextRelation expresses the capabilities of a hook with respect to a relation.
//
//go:generate go run go.uber.org/mock/mockgen -typed -package mocks -destination mocks/context_relation_mock.go github.com/juju/juju/internal/worker/uniter/runner/jujuc ContextRelation
type ContextRelation interface {

	// Id returns an integer which uniquely identifies the relation.
	Id() int

	// Name returns the name the locally executing charm assigned to this relation.
	Name() string

	// RelationTag returns the relation tag.
	RelationTag() names.RelationTag

	// FakeId returns a string of the form "relation-name:123", which uniquely
	// identifies the relation to the hook. In reality, the identification
	// of the relation is the integer following the colon, but the composed
	// name is useful to humans observing it.
	FakeId() string

	// Settings allows read/write access to the local unit's settings in
	// this relation.
	Settings(context.Context) (Settings, error)

	// ApplicationSettings allows read/write access to the application settings in
	// this relation, but only if the current unit is leader.
	ApplicationSettings(context.Context) (Settings, error)

	// UnitNames returns a list of the remote units in the relation.
	UnitNames() []string

	// ReadSettings returns the settings of any remote unit in the relation.
	ReadSettings(ctx context.Context, unit string) (params.Settings, error)

	// ReadApplicationSettings returns the application settings of any remote unit in the relation.
	ReadApplicationSettings(ctx context.Context, app string) (params.Settings, error)

	// Suspended returns true if the relation is suspended.
	Suspended() bool

	// SetStatus sets the relation's status.
	SetStatus(context.Context, relation.Status) error

	// RemoteApplicationName returns the application on the other end of
	// the relation from the perspective of this unit.
	RemoteApplicationName() string

	// RemoteModelUUID returns the uuid of the model hosting the
	// application on the other end of the relation.
	RemoteModelUUID() string

	// Life returns the relation's current life state.
	Life() life.Value
}

// ContextStorageAttachment expresses the capabilities of a hook with
// respect to a storage attachment.
type ContextStorageAttachment interface {

	// Tag returns a tag which uniquely identifies the storage attachment
	// in the context of the unit.
	Tag() names.StorageTag

	// Kind returns the kind of the storage.
	Kind() storage.StorageKind

	// Location returns the location of the storage: the mount point for
	// filesystem-kind stores, and the device path for block-kind stores.
	Location() string
}

// ContextVersion expresses the parts of a hook context related to
// reporting workload versions.
type ContextVersion interface {

	// UnitWorkloadVersion returns the currently set workload version for
	// the unit.
	UnitWorkloadVersion(context.Context) (string, error)

	// SetUnitWorkloadVersion updates the workload version for the unit.
	SetUnitWorkloadVersion(context.Context, string) error
}

// Settings is implemented by types that manipulate unit settings.
type Settings interface {
	Map() params.Settings
	Set(string, string)
	Delete(string)
}

// NewRelationIdValue returns a gnuflag.Value for convenient parsing of relation
// ids in ctx.
func NewRelationIdValue(ctx Context, result *int) (*relationIdValue, error) {
	v := &relationIdValue{result: result, ctx: ctx}
	id := -1
	if r, err := ctx.HookRelation(); err == nil {
		id = r.Id()
		v.value = r.FakeId()
	} else if !errors.Is(err, errors.NotFound) {
		return nil, errors.Trace(err)
	}
	*result = id
	return v, nil
}

// relationIdValue implements gnuflag.Value for use in relation commands.
type relationIdValue struct {
	result *int
	ctx    Context
	value  string
}

// String returns the current value.
func (v *relationIdValue) String() string {
	return v.value
}

// Set interprets value as a relation id, if possible, and returns an error
// if it is not known to the system. The parsed relation id will be written
// to v.result.
func (v *relationIdValue) Set(value string) error {
	trim := value
	if idx := strings.LastIndex(trim, ":"); idx != -1 {
		trim = trim[idx+1:]
	}
	id, err := strconv.Atoi(trim)
	if err != nil {
		return fmt.Errorf("invalid relation id")
	}
	if _, err := v.ctx.Relation(id); err != nil {
		return errors.Trace(err)
	}
	*v.result = id
	v.value = value
	return nil
}
