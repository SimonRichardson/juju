// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"cmp"
	"context"
	"slices"

	coreerrors "github.com/juju/juju/core/errors"
	"github.com/juju/juju/core/secrets"
	"github.com/juju/juju/core/trace"
	"github.com/juju/juju/core/unit"
	domainsecret "github.com/juju/juju/domain/secret"
	secreterrors "github.com/juju/juju/domain/secret/errors"
	"github.com/juju/juju/internal/errors"
)

// GetSecretConsumerAndLatest returns the secret consumer info for the specified unit and secret, along with
// the latest revision for the secret.
// If the unit does not exist, an error satisfying [applicationerrors.UnitNotFound] is returned.
// If the secret does not exist, an error satisfying [secreterrors.SecretNotFound] is returned.
// If there's not currently a consumer record for the secret, the latest revision is still returned,
// along with an error satisfying [secreterrors.SecretConsumerNotFound].
func (s *SecretService) GetSecretConsumerAndLatest(ctx context.Context, uri *secrets.URI, unitName unit.Name) (*secrets.SecretConsumerMetadata, int, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	consumerMetadata, latestRevision, err := s.secretState.GetSecretConsumer(ctx, uri, unitName)
	if err != nil {
		return nil, latestRevision, errors.Capture(err)
	}
	if consumerMetadata.Label != "" {
		return consumerMetadata, latestRevision, nil
	}
	// We allow units to access the application owned secrets using the application owner label,
	// so we copy the owner label to consumer metadata.
	md, err := s.getAppOwnedOrUnitOwnedSecretMetadata(ctx, uri, unitName, "")
	if errors.Is(err, secreterrors.SecretNotFound) {
		// The secret is owned by a different application; the named unit is the consumer.
		return consumerMetadata, latestRevision, nil
	}
	if err != nil {
		return nil, 0, errors.Errorf("cannot get secret metadata for %q: %w", uri, err)
	}
	consumerMetadata.Label = md.Label
	return consumerMetadata, latestRevision, nil
}

// GetSecretConsumer returns the secret consumer info for the specified unit and secret.
// If the unit does not exist, an error satisfying [applicationerrors.UnitNotFound] is returned.
// If the secret does not exist, an error satisfying [secreterrors.SecretNotFound] is returned.
// If there's not currently a consumer record for the secret, an error satisfying [secreterrors.SecretConsumerNotFound]
// is returned.
func (s *SecretService) GetSecretConsumer(ctx context.Context, uri *secrets.URI, unitName unit.Name) (*secrets.SecretConsumerMetadata, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	result, _, err := s.GetSecretConsumerAndLatest(ctx, uri, unitName)
	return result, err
}

// ListUnitSecretMetadata returns only the metadata for secret revisions that
// the unit owns or currently consumes. Secret content is never returned.
func (s *SecretService) ListUnitSecretMetadata(
	ctx context.Context, unitName unit.Name,
) ([]domainsecret.UnitSecretMetadata, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	appName := unitName.Application()
	owned, _, err := s.secretState.ListCharmSecrets(
		ctx,
		domainsecret.ApplicationOwners{appName},
		domainsecret.UnitOwners{unitName.String()},
	)
	if err != nil {
		return nil, errors.Errorf("listing secrets owned by unit %q: %w", unitName, err)
	}

	result := make(map[string]domainsecret.UnitSecretMetadata, len(owned))
	for _, metadata := range owned {
		result[metadata.URI.ID] = domainsecret.UnitSecretMetadata{
			URI:      metadata.URI,
			Label:    metadata.Label,
			Revision: metadata.LatestRevision,
		}
	}
	consumers, err := s.secretState.AllSecretConsumers(ctx)
	if err != nil {
		return nil, errors.Errorf("listing secret consumers for unit %q: %w", unitName, err)
	}
	for secretID, infos := range consumers {
		for _, info := range infos {
			if info.SubjectID != unitName.String() {
				continue
			}
			metadata, err := s.secretState.GetSecret(ctx, &secrets.URI{ID: secretID})
			if err != nil {
				return nil, errors.Errorf("getting visible secret %q: %w", secretID, err)
			}
			label := info.Label
			if label == "" {
				label = metadata.Label
			}
			result[secretID] = domainsecret.UnitSecretMetadata{
				URI:      metadata.URI,
				Label:    label,
				Revision: info.CurrentRevision,
			}
		}
	}
	remoteSecrets, err := s.secretState.AllRemoteSecrets(ctx)
	if err != nil {
		return nil, errors.Errorf("listing remote secrets for unit %q: %w", unitName, err)
	}
	for _, secret := range remoteSecrets {
		if secret.SubjectID != unitName.String() {
			continue
		}
		result[secret.URI.String()] = domainsecret.UnitSecretMetadata{
			URI:      secret.URI,
			Label:    secret.Label,
			Revision: secret.CurrentRevision,
		}
	}

	metadata := make([]domainsecret.UnitSecretMetadata, 0, len(result))
	for _, value := range result {
		metadata = append(metadata, value)
	}
	slices.SortFunc(metadata, func(a, b domainsecret.UnitSecretMetadata) int {
		return cmp.Compare(a.URI.String(), b.URI.String())
	})
	return metadata, nil
}

// SaveSecretConsumer saves the consumer metadata for the given secret and unit.
// If the unit does not exist, an error satisfying [applicationerrors.UnitNotFound] is returned.
// If the secret does not exist, an error satisfying [secreterrors.SecretNotFound] is returned.
func (s *SecretService) SaveSecretConsumer(ctx context.Context, uri *secrets.URI, unitName unit.Name, md secrets.SecretConsumerMetadata) error {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	return s.secretState.SaveSecretConsumer(ctx, uri, unitName, md)
}

// GetURIByConsumerLabel looks up the secret URI using the label previously registered by the specified unit,
// returning an error satisfying [secreterrors.SecretNotFound] if there's no corresponding URI.
// If the unit does not exist, an error satisfying [applicationerrors.UnitNotFound] is returned.
func (s *SecretService) GetURIByConsumerLabel(ctx context.Context, label string, unitName unit.Name) (*secrets.URI, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	return s.secretState.GetURIByConsumerLabel(ctx, label, unitName)
}

// GetConsumedRevision returns the secret revision number for the specified consumer, possibly updating
// the label associated with the secret for the consumer.
func (s *SecretService) GetConsumedRevision(ctx context.Context, uri *secrets.URI, unitName unit.Name, refresh, peek bool, labelToUpdate *string) (int, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	consumerInfo, latestRevision, err := s.GetSecretConsumerAndLatest(ctx, uri, unitName)
	if err != nil && !errors.Is(err, secreterrors.SecretConsumerNotFound) {
		return 0, errors.Capture(err)
	}
	refresh = refresh ||
		err != nil // Not found, so need to create one.

	var wantRevision int
	if err == nil {
		wantRevision = consumerInfo.CurrentRevision
	}

	// Use the latest revision as the current one if --refresh or --peek.
	if refresh || peek {
		if consumerInfo == nil {
			consumerInfo = &secrets.SecretConsumerMetadata{}
		}
		if refresh {
			consumerInfo.CurrentRevision = latestRevision
		}
		wantRevision = latestRevision
	}
	// Save the latest consumer info if required.
	if refresh || labelToUpdate != nil {
		if labelToUpdate != nil {
			consumerInfo.Label = *labelToUpdate
		}
		if err := s.SaveSecretConsumer(ctx, uri, unitName, *consumerInfo); err != nil {
			return 0, errors.Capture(err)
		}
	}
	return wantRevision, nil
}

// ListGrantedSecretsForBackend returns the secret revision info for any
// secrets from the specified backend for which the specified consumers
// have been granted the specified access.
func (s *SecretService) ListGrantedSecretsForBackend(
	ctx context.Context, backendID string, role secrets.SecretRole, consumers ...domainsecret.SecretAccessor,
) ([]*secrets.SecretRevisionRef, error) {
	ctx, span := trace.Start(ctx, trace.NameFromFunc())
	defer span.End()

	accessors := make([]domainsecret.AccessParams, len(consumers))
	for i, consumer := range consumers {
		accessor := domainsecret.AccessParams{
			SubjectID: consumer.ID,
		}
		switch consumer.Kind {
		case domainsecret.UnitAccessor:
			accessor.SubjectTypeID = domainsecret.SubjectUnit
		case domainsecret.ApplicationAccessor:
			accessor.SubjectTypeID = domainsecret.SubjectApplication
		case domainsecret.ModelAccessor:
			accessor.SubjectTypeID = domainsecret.SubjectModel
		default:
			return nil, errors.Errorf("consumer kind %q %w", consumer.Kind, coreerrors.NotValid)
		}
		accessors[i] = accessor
	}

	// Expand the requested role to include all roles that satisfy it.
	roles := expandRolesToMatch(role)

	return s.secretState.ListGrantedSecretsForBackend(ctx, backendID, accessors, roles)
}

// expandRolesToMatch returns a slice of roles that satisfy the requested role.
// RoleManage implies RoleView.
func expandRolesToMatch(role secrets.SecretRole) []domainsecret.Role {
	switch role {
	case secrets.RoleView:
		// Manage implies view, so include both.
		return []domainsecret.Role{domainsecret.RoleView, domainsecret.RoleManage}
	case secrets.RoleManage:
		return []domainsecret.Role{domainsecret.RoleManage}
	case secrets.RoleNone:
		return nil
	default:
		return nil
	}
}
