/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apikeys

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// organizations.go is an organization's audit-log API keys (w1/m657,
// docs/ADR115-organizations.md D11): Render's provisionOrganizationAuditLogToken,
// organizationAPITokens and revokeOrganizationAPIToken. "Org Owners can create
// an audit log API key"; "Audit log API keys have read-only access to audit
// logs for your organization and all of its workspaces"; "It's shown in full
// only once" (render.com/docs/api-keys).
//
// The key is a bex API key like any other, a Hydra client stamped with its
// creator and sign-in, sent as `Authorization: Bearer bex_<id>_<secret>`. Its
// organization_api_keys row makes it the org's: it is bound to no workspace,
// holds no OpenFGA tuple, and its grant (core.OrganizationKeyGrant) passes the
// dispatch gate only to the two audit-log reads, which decide it by its org.
// It belongs to the org, not to its creator: it outlives its creator's
// demotion or departure from the org, any Owner revokes it, and the org's
// end revokes it.

// OrganizationKeyStore is the store slice of an org's audit-log keys;
// *store.PGStore implements it.
type OrganizationKeyStore interface {
	OrganizationRole(ctx context.Context, orgID, subject string) (store.Organization, string, error)
	InsertOrganizationAPIKey(ctx context.Context, k store.OrganizationAPIKey, limit int) error
	ListOrganizationAPIKeys(ctx context.Context, orgID string) ([]store.OrganizationAPIKey, error)
	RevokeOrganizationAPIKey(ctx context.Context, orgID, clientID string) error
	RevokeOrganizationAPIKeys(ctx context.Context, orgID string) ([]string, error)
}

// defaultMaxOrganizationKeys is an org's live-key quota, the workspace's.
const defaultMaxOrganizationKeys = defaultMaxActiveKeys

// OrganizationKeyLimitCode refuses a key past the org's quota or its creation
// rate; the dashboard answers both with Render's "You have created too many
// keys. Please try again later.".
const OrganizationKeyLimitCode = "ORGANIZATION_API_KEY_LIMIT"

// withoutOrganizationEvent makes minting and revoking an org key write no row
// to the org's audit log, because Render's org event list
// (render.com/docs/audit-logs, renderOrganizationEvents) has no event for
// them and the org log answers only Render's events: the allowed row is
// deferred and never recorded. A refusal is still recorded.
func withoutOrganizationEvent(ctx context.Context) context.Context {
	return core.WithDeferredAllowedWriteAudit(ctx)
}

func (s *Service) organizationKeys() (OrganizationKeyStore, error) {
	if s.Organizations == nil || s.APIKeys == nil {
		return nil, core.ErrAPIKeysUnavailable
	}
	return s.Organizations, nil
}

// authorizeOrganizationKeys holds the caller to the org's Owners, Render's
// manageOrganization. Anyone the org does not hold (no can_view on it) is told
// the org was not found, so an org id reveals nothing. A Member or Guest is
// refused 403 FORBIDDEN, as orgUserAuthStatus refuses them (w1/m657 t007: they
// were told the org was not found, which is not true for them). Then
// can_manage, a write relation that OpenFGA is asked uncached, and the
// caller's role in the source of truth.
func (s *Service) authorizeOrganizationKeys(ctx context.Context, orgID string) (OrganizationKeyStore, error) {
	// Check the verb's own OAuth capability before OpenFGA can_view so a
	// narrowed token lacking can_manage gets insufficient_scope without
	// probing whether the org exists, matching core.authorizeAuditLog.
	if id, ok := core.IdentityFrom(ctx); ok {
		if err := id.RequireCapability(core.RelCanManage); err != nil {
			return nil, s.AuthorizeOn(ctx, core.RelCanManage, core.OrganizationObject(orgID))
		}
	}
	if err := s.AuthorizeOn(ctx, core.RelCanView, core.OrganizationObject(orgID)); err != nil {
		if core.IsLoginRefusal(err) || core.IsInsufficientScope(err) || !errors.Is(err, core.ErrForbidden) {
			return nil, err
		}
		return nil, core.NewOrganizationNotFoundError(orgID)
	}
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.OrganizationObject(orgID)); err != nil {
		return nil, err
	}
	orgs, err := s.organizationKeys()
	if err != nil {
		return nil, err
	}
	id, _ := core.IdentityFrom(ctx)
	_, role, err := orgs.OrganizationRole(ctx, orgID, id.Subject)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, core.NewOrganizationNotFoundError(orgID)
	case err != nil:
		return nil, err
	case role != core.OrgRoleOwner:
		return nil, core.ErrForbidden
	}
	return orgs, nil
}

// ProvisionOrganizationAuditLogToken mints one of orgID's audit-log keys
// labelled label and returns its bearer form, the only time it is shown
// (Render: "It's shown in full only once"). Minting is a person's own act
// (AuthorizeMintClass), as every key's is.
func (s *Service) ProvisionOrganizationAuditLogToken(ctx context.Context, orgID, label string) (string, error) {
	ctx = withoutOrganizationEvent(ctx)
	if err := s.AuthorizeMintClass(ctx); err != nil {
		return "", err
	}
	orgs, err := s.authorizeOrganizationKeys(ctx, orgID)
	if err != nil {
		return "", err
	}
	label, err = core.TrimmedLabel(label)
	if err != nil {
		return "", err
	}
	if label == "" {
		return "", fmt.Errorf("%w: label: must not be blank", core.ErrBadRequest)
	}
	createdBy := core.SubjectFrom(ctx)
	if err := s.refuseDeletingCreator(ctx, createdBy); err != nil {
		return "", err
	}
	if s.CreationLimiter != nil && !s.CreationLimiter.Bucket(orgID).Allow() {
		return "", core.NewConflictError(OrganizationKeyLimitCode,
			"API key creation rate exceeded for this organization", map[string]any{"organizationId": orgID})
	}
	key, err := s.APIKeys.Create(ctx, label, createdBy, core.KeySignInStamp(ctx))
	if err != nil {
		return "", err
	}
	// The secret has not left this function, so no one holds the client
	// before its row makes it the org's; a refused row takes it back.
	err = orgs.InsertOrganizationAPIKey(ctx, store.OrganizationAPIKey{
		ClientID: key.ID, OrganizationID: orgID, Label: label, CreatedBy: createdBy,
	}, defaultMaxOrganizationKeys)
	if err != nil {
		_ = s.APIKeys.Delete(ctx, key.ID)
		if errors.Is(err, store.ErrOrganizationAPIKeyLimit) {
			return "", core.NewConflictError(OrganizationKeyLimitCode,
				"organization has the maximum number of API keys",
				map[string]any{"organizationId": orgID, "limit": defaultMaxOrganizationKeys})
		}
		return "", fmt.Errorf("record organization api key: %w", err)
	}
	// Account deletion may have started while the key was minted (see
	// createBoundAPIKey): take it back rather than outlive the account.
	if err := s.refuseDeletingCreator(ctx, createdBy); err != nil {
		_ = orgs.RevokeOrganizationAPIKey(ctx, orgID, key.ID)
		_ = s.APIKeys.Delete(ctx, key.ID)
		return "", err
	}
	return BearerKey(key.ID, key.Secret), nil
}

// ListOrganizationAPITokens lists orgID's live audit-log keys, oldest first,
// with no secret, reading each key's client (at most defaultMaxOrganizationKeys)
// for its last use. A key whose Hydra client is gone (its creator's account was
// deleted, which revokes every key they made) is not listed.
func (s *Service) ListOrganizationAPITokens(ctx context.Context, orgID string) ([]core.OrganizationAPIToken, error) {
	orgs, err := s.authorizeOrganizationKeys(ctx, orgID)
	if err != nil {
		return nil, err
	}
	rows, err := orgs.ListOrganizationAPIKeys(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]core.OrganizationAPIToken, 0, len(rows))
	for _, row := range rows {
		client, err := s.boundKey(ctx, row.ClientID)
		switch {
		case errors.Is(err, core.ErrNotFound):
			continue
		case err != nil:
			return nil, err
		}
		out = append(out, core.OrganizationAPIToken{
			ID: row.ClientID, Label: row.Label, Type: row.Type,
			CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339),
			LastUsedAt: client.LastUsedAt,
		})
	}
	return out, nil
}

// RevokeOrganizationAPIToken revokes orgID's key tokenID. Its row stops it at
// once on every replica, since the audit-log reads check it uncached; then
// its Hydra client is deleted. A key that is not a live key of orgID, another
// org's included, is not found.
func (s *Service) RevokeOrganizationAPIToken(ctx context.Context, orgID, tokenID string) error {
	ctx = withoutOrganizationEvent(ctx)
	orgs, err := s.authorizeOrganizationKeys(ctx, orgID)
	if err != nil {
		return err
	}
	if err := orgs.RevokeOrganizationAPIKey(ctx, orgID, tokenID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return core.ErrNotFound
		}
		return err
	}
	deleteOrganizationClient(ctx, s.APIKeys, tokenID)
	return nil
}

// deleteOrganizationClient deletes a revoked org key's Hydra client. The
// revoked row already refuses the key everywhere, so a failure here leaves an
// inert client, which is logged rather than reported as a failed revoke.
func deleteOrganizationClient(ctx context.Context, keys APIKeyStore, clientID string) {
	if err := keys.Delete(ctx, clientID); err != nil && !errors.Is(err, core.ErrNotFound) {
		log.Printf("apikeys: deleting revoked organization key %s: %v", clientID, err)
	}
}

// OrganizationPurger revokes every audit-log key of an org that has ended
// (ADR115 D11): a downgrade dissolved it, or its primary was deleted. The
// keys already read nothing once the org is gone; this retires their rows
// and deletes their clients. It is the platform's own cleanup, not a verb.
type OrganizationPurger struct {
	Store APIKeyStore
	Keys  OrganizationKeyStore
}

// PurgeOrganization revokes orgID's live keys.
func (p OrganizationPurger) PurgeOrganization(ctx context.Context, orgID string) error {
	ids, err := p.Keys.RevokeOrganizationAPIKeys(ctx, orgID)
	if err != nil {
		return fmt.Errorf("revoke organization api keys: %w", err)
	}
	for _, id := range ids {
		deleteOrganizationClient(ctx, p.Store, id)
	}
	return nil
}
