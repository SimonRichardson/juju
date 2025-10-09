// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package facade

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery"
	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery/checkers"
	"github.com/juju/clock"
	"github.com/juju/description/v10"
	"github.com/juju/names/v6"
	"github.com/juju/worker/v4"
	"gopkg.in/macaroon.v2"

	crossmodelbakery "github.com/juju/juju/apiserver/internal/crossmodel/bakery"
	corehttp "github.com/juju/juju/core/http"
	"github.com/juju/juju/core/leadership"
	"github.com/juju/juju/core/lease"
	corelogger "github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/objectstore"
	"github.com/juju/juju/core/permission"
	"github.com/juju/juju/internal/services"
	"github.com/juju/juju/internal/worker/watcherregistry"
)

// Facade could be anything; it will be interpreted by the apiserver
// machinery such that certain exported methods will be made available
// as facade methods to connected clients.
type Facade interface{}

// Factory is a callback used to create a Facade.
type Factory func(stdCtx context.Context, modelCtx ModelContext) (Facade, error)

// MultiModelFactory is a callback used to create a Facade.
type MultiModelFactory func(stdCtx context.Context, modelCtx MultiModelContext) (Facade, error)

// LeadershipModelContext
type LeadershipModelContext interface {
	// LeadershipClaimer returns a leadership.Claimer for this
	// context's model.
	LeadershipClaimer() (leadership.Claimer, error)

	// LeadershipRevoker returns a leadership.Revoker for this
	// context's model.
	LeadershipRevoker() (leadership.Revoker, error)

	// LeadershipPinner returns a leadership.Pinner for this
	// context's model.
	LeadershipPinner() (leadership.Pinner, error)

	// LeadershipReader returns a leadership.Reader for this
	// context's model.
	LeadershipReader() (leadership.Reader, error)

	// LeadershipChecker returns a leadership.Checker for this
	// context's model.
	LeadershipChecker() (leadership.Checker, error)

	// SingularClaimer returns a lease.Claimer for singular leases for
	// this context's model.
	SingularClaimer() (lease.Claimer, error)
}

// MultiModelContext is a context that can operate on multiple models at once.
type MultiModelContext interface {
	ModelContext

	// DomainServicesForModel returns the services factory for a given model
	// uuid.
	DomainServicesForModel(context.Context, model.UUID) (services.DomainServices, error)

	// ObjectStoreForModel returns the object store for a given model uuid.
	ObjectStoreForModel(ctx context.Context, modelUUID string) (objectstore.ObjectStore, error)
}

// ModelContext exposes useful capabilities to a Facade for a given model.
type ModelContext interface {
	// TODO (stickupkid): This shouldn't be embedded, instead this should be
	// in the form of `context.Leadership() Leadership`, which returns the
	// contents of the LeadershipContext.
	// Context should have a single responsibility, and that's access to other
	// types/objects.
	LeadershipModelContext
	ModelMigrationFactory
	DomainServices
	ObjectStoreFactory
	Logger

	// Auth represents information about the connected client. You
	// should always be checking individual requests against Auth:
	// both state changes *and* data retrieval should be blocked
	// with apiservererrors.ErrPerm for any targets for which the client is
	// not *known* to have a responsibility or requirement.
	Auth() Authorizer

	// CrossModelAuthContext provides methods to create and authorize macaroons
	// for cross model operations.
	CrossModelAuthContext() CrossModelAuthContext

	// Dispose disposes the context and any resources related to
	// the API server facade object. Normally the context will not
	// be disposed until the API connection is closed. This is OK
	// except when contexts are dynamically generated, such as in
	// the case of watchers. When a facade context is no longer
	// needed, e.g. when a watcher is closed, then the context may
	// be disposed by calling this method.
	Dispose()

	// WatcherRegistry returns the watcher registry for this context. The
	// watchers are per-connection, and are cleaned up when the connection
	// is closed.
	WatcherRegistry() watcherregistry.WatcherRegistry

	// ID returns a string that should almost always be "", unless
	// this is a watcher facade, in which case it exists in lieu of
	// actual arguments in the Next() call, and is used as a key
	// into Resources to get the watcher in play. This is not really
	// a good idea; see Resources.
	ID() string

	// ControllerUUID returns the controller's unique identifier.
	ControllerUUID() string

	// ControllerModelUUID returns the controller's model unique identifier.
	// This is the model that the controller is running in, which may be
	// different from the model that the facade is running in.
	ControllerModelUUID() model.UUID

	// ModelUUID returns the model's unique identifier. All facade requests
	// are in the scope of a model. There are some exceptions to the rule, but
	// they are exceptions that prove the rule.
	ModelUUID() model.UUID

	// IsControllerModelScoped returns true if the context is scoped to the
	// controller model. Is the controller model uuid and then model uuid
	// are the same.
	IsControllerModelScoped() bool

	// RequestRecorder defines a metrics collector for outbound requests.
	RequestRecorder() RequestRecorder

	// HTTPClient returns an HTTP client to use for the given purpose. The
	// following errors can be expected:
	// - [ErrorHTTPClientPurposeInvalid] when the requested purpose is not
	// understood by the context.
	// - [ErrorHTTPClientForPurposeNotFound] when no http client can be found
	// for the requested [HTTPClientPurpose].
	HTTPClient(corehttp.Purpose) (HTTPClient, error)

	// MachineTag returns the current machine tag.
	MachineTag() names.Tag

	// DataDir returns the data directory.
	DataDir() string

	// LogDir returns the log directory.
	LogDir() string

	// Clock returns a instance of the clock.
	Clock() clock.Clock
}

// ModelExporter defines a interface for exporting models.
type ModelExporter interface {
	// ExportModel exports the current model into a description model. This
	// can be serialized into yaml and then imported.
	ExportModel(context.Context, objectstore.ObjectStore) (description.Model, error)
}

// LegacyStateExporter describes interface on state required to export a
// model.
// Deprecated: This is being replaced with the ModelExporter.
type LegacyStateExporter interface {
	// Export generates an abstract representation of a model.
	Export(objectstore.ObjectStore) (description.Model, error)
}

// ModelImporter defines an interface for importing models.
type ModelImporter interface {
	// ImportModel takes a serialized description model (yaml bytes) and returns
	// a state model and state state.
	ImportModel(ctx context.Context, bytes []byte) error
}

// ModelMigrationFactory defines an interface for getting a model migrator.
type ModelMigrationFactory interface {
	// ModelExporter returns a model exporter for the current model.
	ModelExporter(context.Context, model.UUID) (ModelExporter, error)

	// ModelImporter returns a model importer.
	ModelImporter() ModelImporter
}

// DomainServices defines an interface for accessing all the services.
type DomainServices interface {
	// DomainServices returns the services factory for the current model.
	DomainServices() services.DomainServices
}

// ObjectStoreFactory defines an interface for accessing the object store.
type ObjectStoreFactory interface {
	// ObjectStore returns the object store for the current model.
	ObjectStore() objectstore.ObjectStore

	// ControllerObjectStore returns the object store for the controller.
	ControllerObjectStore() objectstore.ObjectStore
}

// Logger defines an interface for getting the apiserver logger instance.
type Logger interface {
	// Logger returns the apiserver logger instance.
	Logger() corelogger.Logger
}

// RequestRecorder is implemented by types that can record information about
// successful and unsuccessful http requests.
type RequestRecorder interface {
	// Record an outgoing request that produced a http.Response.
	Record(method string, url *url.URL, res *http.Response, rtt time.Duration)

	// RecordError records an outgoing request that returned back an error.
	RecordError(method string, url *url.URL, err error)
}

// Authorizer represents the authenticated entity using the API server.
type Authorizer interface {

	// GetAuthTag returns the entity's tag.
	GetAuthTag() names.Tag

	// AuthController returns whether the authenticated entity is
	// a machine acting as a controller. Can't be removed from this
	// interface without introducing a dependency on something else
	// to look up that property: it's not inherent in the result of
	// GetAuthTag, as the other methods all are.
	AuthController() bool

	// TODO(wallyworld - bug 1733759) - the following auth methods should not be on this interface
	// eg introduce a utility func or something.

	// AuthMachineAgent returns true if the entity is a machine agent.
	AuthMachineAgent() bool

	// AuthApplicationAgent returns true if the entity is an application operator.
	AuthApplicationAgent() bool

	// AuthModelAgent returns true if the entity is a model operator.
	AuthModelAgent() bool

	// AuthUnitAgent returns true if the entity is a unit agent.
	AuthUnitAgent() bool

	// AuthOwner returns true if tag == .GetAuthTag().
	AuthOwner(tag names.Tag) bool

	// AuthClient returns true if the entity is an external user.
	AuthClient() bool

	// HasPermission reports whether the given access is allowed for the given
	// target by the authenticated entity.
	HasPermission(ctx context.Context, operation permission.Access, target names.Tag) error

	// EntityHasPermission reports whether the given access is allowed for the given
	// target by the given entity.
	EntityHasPermission(ctx context.Context, entity names.Tag, operation permission.Access, target names.Tag) error
}

// MacaroonAuthenticator provides methods to authenticate macaroons for cross
// model operations.
type MacaroonAuthenticator interface {
	// CheckOfferMacaroons verifies that the specified macaroons allow access to the
	// offer.
	CheckOfferMacaroons(ctx context.Context, offeringModelUUID, offerUUID string, mac macaroon.Slice, version bakery.Version) (map[string]string, error)
}

// CrossModelAuthContext provides methods to create macaroons for cross model
// operations.
type CrossModelAuthContext interface {
	// Authenticator returns an instance used to authenticate macaroons used to
	// access offers.
	Authenticator() MacaroonAuthenticator

	// CreateConsumeOfferMacaroon creates a macaroon that authorizes access to the
	// specified offer.
	CreateConsumeOfferMacaroon(
		ctx context.Context,
		modelUUID model.UUID,
		offerUUID, username string,
		version bakery.Version,
	) (*bakery.Macaroon, error)

	// CreateRemoteRelationMacaroon creates a macaroon that authorizes access to the
	// specified relation.
	CreateRemoteRelationMacaroon(
		ctx context.Context,
		modelUUID model.UUID,
		offerUUID, username string,
		rel names.RelationTag,
		version bakery.Version,
	) (*bakery.Macaroon, error)

	// CheckLocalAccessRequest checks that the user in the specified permission
	// check details has consume access to the offer in the details.
	// It returns an error with a *bakery.VerificationError cause if the macaroon
	// verification failed. If the macaroon is valid, CheckLocalAccessRequest
	// returns a list of caveats to add to the discharge macaroon.
	CheckLocalAccessRequest(ctx context.Context, details crossmodelbakery.OfferAccessDetails) ([]checkers.Caveat, error)

	// CheckOfferAccessCaveat checks that the specified caveat required to be satisfied
	// to gain access to an offer is valid, and returns the attributes return to check
	// that the caveat is satisfied.
	CheckOfferAccessCaveat(ctx context.Context, caveat string) (crossmodelbakery.OfferAccessDetails, error)

	// OfferThirdPartyKey returns the key used to discharge offer macaroons.
	OfferThirdPartyKey() *bakery.KeyPair
}

// Hub represents the central hub that the API server has.
type Hub interface {
	Publish(topic string, data interface{}) (func(), error)
}

// HTTPClient represents an HTTP client, for example, an *http.Client.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// WatcherRegistry defines an interface for managing watchers
// for a connection.
type WatcherRegistry interface {
	// Get returns the watcher for the given id, or nil if there is no such
	// watcher.
	Get(string) (worker.Worker, error)
	// Register registers the given watcher. It returns a unique identifier for the
	// watcher which can then be used in subsequent API requests to refer to the
	// watcher.
	Register(context.Context, worker.Worker) (string, error)

	// RegisterNamed registers the given watcher. Callers must supply a unique
	// name for the given watcher. It is an error to try to register another
	// watcher with the same name as an already registered name.
	// It is also an error to supply a name that is an integer string, since that
	// collides with the auto-naming from Register.
	RegisterNamed(context.Context, string, worker.Worker) error

	// Stop stops the resource with the given id and unregisters it.
	// It returns any error from the underlying Stop call.
	// It does not return an error if the resource has already
	// been unregistered.
	Stop(id string) error
}
