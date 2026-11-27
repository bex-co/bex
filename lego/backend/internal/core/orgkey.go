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

package core

import (
	"context"
	"errors"
	"log"
	"strings"
)

// orgkey.go is what an organization's audit-log API key may read (w1/m657,
// docs/ADR115-organizations.md D11, docs/ADR012-auth.md §8g). Render: "Audit
// log API keys have read-only access to audit logs for your organization and
// all of its workspaces" (render.com/docs/api-keys).
//
// Such a key is a bex API key with no workspace binding and no OpenFGA tuple,
// so every relation check refuses it. Its grant (KeyGrant.Organization) lets
// the dispatch gate pass it to exactly two operations, the org's audit log and
// a workspace's, and those two verbs authorize through
// AuthorizeOrganizationAuditLog and AuthorizeWorkspaceAuditLog, which decide it
// by its org at request time: a workspace that joins the org after the key was
// minted is read, one that leaves it no longer is.

// OrganizationKeyAuthority is the store slice that decision reads, uncached,
// so a revoke or the org's end stops a key on its next request.
// *store.PGStore implements it.
type OrganizationKeyAuthority interface {
	// LiveOrganizationAPIKey is the org clientID is a live key of, live=false
	// when it is revoked, the org is gone, or it is no org's key.
	LiveOrganizationAPIKey(ctx context.Context, clientID string) (orgID string, live bool, err error)
	// WorkspaceOrganizationID is the org a workspace belongs to now, "" for
	// none.
	WorkspaceOrganizationID(ctx context.Context, workspaceID string) (string, error)
}

// OrganizationKey reports whether the caller is an organization's audit-log
// key, and whose. It reads the caller's grant, so it costs a person nothing
// and a key one uncached lookup.
func (b *Base) OrganizationKey(ctx context.Context) (orgID string, isOrgKey bool, err error) {
	g, err := b.KeyGrant(ctx)
	if err != nil {
		return "", false, err
	}
	return g.Organization, g.Organization != "", nil
}

// AuthorizeWorkspaceAuditLog gates reading workspaceID's audit log ("" =>
// the caller's own workspace, as AuthorizeWorkspace resolves it): a workspace
// Admin, can_manage as before, or an audit-log key of the org the workspace
// belongs to now.
func (b *Base) AuthorizeWorkspaceAuditLog(ctx context.Context, workspaceID string) (string, error) {
	workspaceID, resolveErr := b.workspaceArgument(ctx, workspaceID)
	_, err := b.authorizeAuditLog(ctx, WorkspaceObject(workspaceID), callerVerb(ctx, verbFrameSkip), resolveErr)
	return workspaceID, err
}

// AuthorizeOrganizationAuditLog gates reading orgID's own audit log:
// can_manage on the org (its Owners) as before, or one of the org's
// audit-log keys, which orgKey reports.
func (b *Base) AuthorizeOrganizationAuditLog(ctx context.Context, orgID string) (orgKey bool, err error) {
	return b.authorizeAuditLog(ctx, OrganizationObject(orgID), callerVerb(ctx, verbFrameSkip), nil)
}

// authorizeAuditLog is the one decision behind both: a person or a workspace
// key takes the funnel every verb takes; an org key is decided by its org,
// then held to the login requirements of the object it reads as any key is
// (its creator's second factor and sign-in), and a refusal is audited the
// way the funnel audits one.
func (b *Base) authorizeAuditLog(ctx context.Context, object, verb string, resolveErr error) (orgKey bool, err error) {
	orgID, isOrgKey, err := b.OrganizationKey(ctx)
	if err == nil && !isOrgKey {
		// An org's own log names who is told it was not found, exactly as the
		// org's key management does (apikeys.authorizeOrganizationKeys): a
		// caller who cannot view the org learns nothing from its id, while a
		// Member or Guest the org holds is refused FORBIDDEN. Check the verb's
		// own OAuth capability first so a narrowed token lacking can_manage
		// never reaches OpenFGA or learns whether the org exists, and route
		// every refusal through authorizeAndAudit so denials stay audited.
		if target, isOrg := strings.CutPrefix(object, OrganizationObjectPrefix); isOrg && resolveErr == nil {
			if checkCapability(ctx, RelCanManage) == nil {
				if verr := b.checkAuthz(ctx, RelCanView, object); verr != nil {
					if !IsLoginRefusal(verr) && !IsInsufficientScope(verr) && errors.Is(verr, ErrForbidden) {
						resolveErr = NewOrganizationNotFoundError(target)
					} else {
						resolveErr = verr
					}
				}
			}
		}
		return false, b.authorizeAndAudit(ctx, RelCanManage, object, "", verb, resolveErr)
	}
	if err == nil {
		err = resolveErr
	}
	if err == nil {
		err = b.organizationKeyReads(ctx, orgID, object)
	}
	if err == nil {
		err = b.requireTwoFactor(ctx, object)
	}
	if err != nil && auditsDenial(RelCanManage) {
		b.emit(ctx, verb, RelCanManage, object, "", err)
	}
	return isOrgKey, err
}

// organizationKeyReads decides whether the calling key, a key of orgID, may
// read object's log: the key is live, and object is the org itself or a
// workspace the org holds now. Anything else is ErrForbidden, the refusal any
// key gets where it may not act.
func (b *Base) organizationKeyReads(ctx context.Context, orgID, object string) error {
	if b.OrganizationKeys == nil {
		return ErrForbidden
	}
	id, _ := IdentityFrom(ctx)
	liveOrg, live, err := b.OrganizationKeys.LiveOrganizationAPIKey(ctx, id.KeyClientID())
	if err != nil {
		log.Printf("core: organization key lookup unavailable: %v", err)
		return ErrAuthzUnavailable
	}
	if !live || liveOrg != orgID {
		return ErrForbidden
	}
	if target, isOrg := strings.CutPrefix(object, OrganizationObjectPrefix); isOrg {
		if target != orgID {
			return ErrForbidden
		}
		return nil
	}
	workspaceID, isWorkspace := strings.CutPrefix(object, WorkspaceObjectPrefix)
	if !isWorkspace || workspaceID == "" {
		return ErrForbidden
	}
	holder, err := b.OrganizationKeys.WorkspaceOrganizationID(ctx, workspaceID)
	if err != nil {
		log.Printf("core: workspace organization lookup unavailable: %v", err)
		return ErrAuthzUnavailable
	}
	if holder != orgID {
		return ErrForbidden
	}
	return nil
}

// OrganizationAPIToken is one of an org's audit-log keys as Render's
// organizationAPITokens lists it (w1/m657): no secret and no creator, as
// Render's table shows none. ExpiresAt is always empty: the keys do not
// expire.
type OrganizationAPIToken struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
	Type       string `json:"type"`
}
