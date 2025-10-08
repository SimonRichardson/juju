// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package crossmodel

import (
	"context"

	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery"
	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery/checkers"
	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"gopkg.in/macaroon.v2"

	"github.com/juju/juju/apiserver/authentication"
	"github.com/juju/juju/apiserver/common"
	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/apiserver/facade"
	crossmodelbakery "github.com/juju/juju/apiserver/internal/crossmodel/bakery"
	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/permission"
	"github.com/juju/juju/core/user"
)

// AccessService validates access for user permissions.
type AccessService interface {
	// ReadUserAccessLevelForTarget returns the user access level for the
	// given user on the given target.
	ReadUserAccessLevelForTarget(ctx context.Context, subject user.Name, target permission.ID) (permission.Access, error)
}

// OfferBakery is used to create and inspect macaroons used to access
// application offers.
type OfferBakery interface {
	// ParseCaveat parses the specified caveat and returns the offer access details
	// it contains.
	ParseCaveat(caveat string) (crossmodelbakery.OfferAccessDetails, error)
	// GetConsumeOfferCaveats returns the caveats for consuming an offer.
	GetConsumeOfferCaveats(offerUUID, sourceModelUUID, username, relation string) []checkers.Caveat
	// GetRemoteRelationCaveats returns the caveats for accessing a remote relation.
	GetRemoteRelationCaveats(offerUUID, sourceModelUUID, username, relation string) []checkers.Caveat
	// InferDeclared retrieves any declared information from
	// the given macaroons and returns it as a key-value map.
	InferDeclaredFromMacaroon(macaroon.Slice, map[string]string) crossmodelbakery.DeclaredValues
	// NewMacaroon creates a new macaroon for the given version, caveats and ops.
	NewMacaroon(context.Context, bakery.Version, []checkers.Caveat, ...bakery.Op) (*bakery.Macaroon, error)
	// CreateDischargeMacaroon creates a discharge macaroon.
	CreateDischargeMacaroon(
		ctx context.Context,
		username string,
		requiredValues map[string]string,
		declaredValues crossmodelbakery.DeclaredValues,
		op bakery.Op,
		version bakery.Version,
	) (*bakery.Macaroon, error)
	// GetOfferRequiredValues returns the required values for the specified
	// offer access.
	GetOfferRequiredValues(sourceModelUUID, offerUUID string) (map[string]string, error)
	// GetRelationRequiredValues returns the required values for the specified
	// relation access.
	GetRelationRequiredValues(sourceModelUUID, offerUUID, relation string) (map[string]string, error)
	// AllowedMacaroonAuth checks the specified macaroon is valid for the operation
	// and returns the associated AuthInfo.
	AllowedAuth(ctx context.Context, op bakery.Op, mac macaroon.Slice) ([]string, error)
}

// AuthContext is used to validate macaroons used to access
// application offers.
type AuthContext struct {
	accessService AccessService
	bakery        OfferBakery

	offerThirdPartyKey *bakery.KeyPair

	controllerTag names.ControllerTag
	modelTag      names.ModelTag

	clock  clock.Clock
	logger logger.Logger
}

// NewAuthContext creates a new authentication context for checking
// macaroons used with application offer requests.
func NewAuthContext(
	accessService AccessService,
	bakery OfferBakery,
	offerThirdPartyKey *bakery.KeyPair,
	controllerUUID string,
	modelUUID model.UUID,
	clock clock.Clock,
	logger logger.Logger,
) *AuthContext {
	return &AuthContext{
		accessService:      accessService,
		bakery:             bakery,
		offerThirdPartyKey: offerThirdPartyKey,
		controllerTag:      names.NewControllerTag(controllerUUID),
		modelTag:           names.NewModelTag(modelUUID.String()),
		clock:              clock,
		logger:             logger,
	}
}

// OfferThirdPartyKey returns the key used to discharge offer macaroons.
func (a *AuthContext) OfferThirdPartyKey() *bakery.KeyPair {
	return a.offerThirdPartyKey
}

// CheckOfferAccessCaveat checks that the specified caveat required to be satisfied
// to gain access to an offer is valid, and returns the attributes return to check
// that the caveat is satisfied.
func (a *AuthContext) CheckOfferAccessCaveat(ctx context.Context, caveat string) (crossmodelbakery.OfferAccessDetails, error) {
	details, err := a.bakery.ParseCaveat(caveat)
	if err != nil {
		return crossmodelbakery.OfferAccessDetails{}, errors.Annotatef(err, "parsing caveat %q", caveat)
	}

	a.logger.Debugf(ctx, "offer access caveat details: %+v", details)
	if !names.IsValidModel(details.SourceModelUUID) {
		return crossmodelbakery.OfferAccessDetails{}, errors.NotValidf("source-model-uuid %q", details.SourceModelUUID)
	}
	if !names.IsValidUser(details.User) {
		return crossmodelbakery.OfferAccessDetails{}, errors.NotValidf("username %q", details.User)
	}
	if err := permission.ValidateOfferAccess(permission.Access(details.Permission)); err != nil {
		return crossmodelbakery.OfferAccessDetails{}, errors.NotValidf("permission %q", details.Permission)
	}
	return details, nil
}

// CheckLocalAccessRequest checks that the user in the specified permission
// check details has consume access to the offer in the details.
// It returns an error with a *bakery.VerificationError cause if the macaroon
// verification failed. If the macaroon is valid, CheckLocalAccessRequest
// returns a list of caveats to add to the discharge macaroon.
func (a *AuthContext) CheckLocalAccessRequest(ctx context.Context, details crossmodelbakery.OfferAccessDetails) ([]checkers.Caveat, error) {
	a.logger.Debugf(ctx, "authenticate local offer access: %+v", details)
	if err := a.checkOfferAccess(ctx, a.hasUserOfferPermission, details.User, details.OfferUUID); err != nil {
		return nil, errors.Trace(err)
	}

	return a.bakery.GetConsumeOfferCaveats(details.OfferUUID, details.SourceModelUUID, details.User, details.Relation), nil
}

// CreateConsumeOfferMacaroon creates a macaroon that authorizes access to the
// specified offer.
func (a *AuthContext) CreateConsumeOfferMacaroon(
	ctx context.Context,
	modelUUID model.UUID,
	offerUUID, username string,
	version bakery.Version,
) (*bakery.Macaroon, error) {
	return a.bakery.NewMacaroon(
		ctx, version,
		a.bakery.GetConsumeOfferCaveats(offerUUID, modelUUID.String(), username, ""),
		crossModelConsumeOp(offerUUID),
	)
}

// CreateRemoteRelationMacaroon creates a macaroon that authorizes access to the
// specified relation.
func (a *AuthContext) CreateRemoteRelationMacaroon(
	ctx context.Context,
	modelUUID model.UUID,
	offerUUID, username string,
	rel names.RelationTag,
	version bakery.Version,
) (*bakery.Macaroon, error) {
	return a.bakery.NewMacaroon(
		ctx, version,
		a.bakery.GetRemoteRelationCaveats(offerUUID, modelUUID.String(), username, rel.String()),
		crossModelRelateOp(rel.String()),
	)
}

// Authenticator returns an instance used to authenticate macaroons used to
// access offers.
func (a *AuthContext) Authenticator() facade.MacaroonAuthenticator {
	return &Authenticator{
		bakery: a.bakery,
		logger: a.logger.Child("authenticator"),
	}
}

func (a *AuthContext) hasUserOfferPermission(ctx context.Context, userName user.Name, target permission.ID) (permission.Access, error) {
	if target.ObjectType == permission.Cloud {
		return "", errors.NotValidf("target %q", target.ObjectType)
	}

	access, err := a.accessService.ReadUserAccessLevelForTarget(ctx, userName, target)
	return access, errors.Trace(err)
}

func (a *AuthContext) checkOfferAccess(ctx context.Context, userAccess common.UserAccessFunc, username, offerUUID string) error {
	userTag := names.NewUserTag(username)

	isAdmin, err := hasAccess(ctx, userAccess, userTag, permission.SuperuserAccess, a.controllerTag)
	if is := errors.Is(err, authentication.ErrorEntityMissingPermission); err != nil && !is {
		return apiservererrors.ErrPerm
	} else if isAdmin {
		return nil
	}

	isAdmin, err = hasAccess(ctx, userAccess, userTag, permission.AdminAccess, a.modelTag)
	if is := errors.Is(err, authentication.ErrorEntityMissingPermission); err != nil && !is {
		return apiservererrors.ErrPerm
	} else if isAdmin {
		return nil
	}

	isConsume, err := hasAccess(ctx, userAccess, userTag, permission.ConsumeAccess, names.NewApplicationOfferTag(offerUUID))
	if err != nil {
		return err
	} else if !isConsume {
		return apiservererrors.ErrPerm
	}
	return nil
}

func hasAccess(ctx context.Context, userAccess common.UserAccessFunc, userTag names.UserTag, access permission.Access, target names.Tag) (bool, error) {
	has, err := common.HasPermission(ctx, userAccess, userTag, access, target)
	if errors.Is(err, errors.NotFound) {
		return false, nil
	}
	return has, err
}

const (
	// consumeOp is the action used to consume an offer.
	consumeOp = "consume"

	// relateOp is the action used to relate to a remote relation.
	relateOp = "relate"
)

// NewCMRAuthorizer returns a bakery.OpsAuthorizer that authorizes
// cross model related operations.
func NewCMRAuthorizer(logger logger.Logger) bakery.OpsAuthorizer {
	return crossModelAuthorizer{logger: logger}
}

type crossModelAuthorizer struct {
	logger logger.Logger
}

// AuthorizeOps implements OpsAuthorizer.AuthorizeOps.
func (a crossModelAuthorizer) AuthorizeOps(ctx context.Context, authorizedOp bakery.Op, queryOps []bakery.Op) ([]bool, []checkers.Caveat, error) {
	a.logger.Debugf(ctx, "authorize cmr query ops check for %#v: %#v", authorizedOp, queryOps)
	allowed := make([]bool, len(queryOps))
	for i := range allowed {
		allowed[i] = queryOps[i].Action == consumeOp || queryOps[i].Action == relateOp
	}
	return allowed, nil, nil
}

func crossModelConsumeOp(offerUUID string) bakery.Op {
	return bakery.Op{
		Entity: offerUUID,
		Action: consumeOp,
	}
}

func crossModelRelateOp(relationID string) bakery.Op {
	return bakery.Op{
		Entity: relationID,
		Action: relateOp,
	}
}
