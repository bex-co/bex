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

package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// tenancy.go is the store-backed bridge between the auth gate and core.Base's
// workspace resolver (w1/m9). The auth gate mints a personal tenant on a human's
// first login (Onboarding); core.Base.Authorize resolves every caller to its
// workspace (core.WorkspaceResolver); the api-keys service binds a minted key to
// its tenant (apikeys.KeyBinder). One tenantService implements all three — it
// sits here, in the composition root, rather than a feature package, so core
// stays free of the store dependency; one TTL cache deduplicates the
// identity→tenant lookups a single request fans out (Authorize + List/Get).

// TenantStore is the slice of the source of truth the resolver + onboarding +
// key-binder read/write. *store.PGStore (and the in-memory fake) satisfy it.
// TenantForIdentity serves both human (Kratos identity id) and machine (Hydra
// client id) subjects — tenant_members.subject covers both, so there is no
// separate machine-caller lookup.
type TenantStore interface {
	TenantForIdentity(ctx context.Context, subject string) (store.Tenant, error)
	CreateTenantWithMember(ctx context.Context, identityID, plan string) (store.Tenant, error)
	BindClient(ctx context.Context, clientID, tenantID string) error
	UnbindClient(ctx context.Context, clientID string) error
}

// Onboarding mints a personal tenant for a human identity on first login. The
// auth gate calls it for session callers only — machine (API-key) callers never
// mint (they resolve via their key's tenant binding instead).
type Onboarding interface {
	EnsureTenant(ctx context.Context, identityID string) (tenantID string, err error)
}

// tenantService implements both core.WorkspaceResolver (the read path
// Authorize/List/Get use) and Onboarding (the mint the auth gate drives), over
// one store + one TTL cache. The granter is nil when OpenFGA is off (store on,
// authz off): tenants are still minted and keys still bound, just without the
// FGA membership tuples — a correct intermediate where the store isolates
// tenants by filtering and authz isn't yet enforcing roles.
type tenantService struct {
	store   TenantStore
	granter store.MembershipGranter // nil => skip FGA tuple writes
	cache   *core.TTLCache[string]
}

// NewTenantService wires the store-backed resolver + onboarding mint. granter
// may be nil (authz off). Returns nil when the store is nil (store off — no
// resolver, no mint, the legacy default-workspace mode).
func NewTenantService(s TenantStore, granter store.MembershipGranter) *tenantService {
	if s == nil {
		return nil
	}
	return &tenantService{store: s, granter: granter, cache: core.NewTTLCache[string]()}
}

// cacheKey namespaces the resolver cache by auth method so a Kratos identity id
// and a Hydra client_id can never shadow each other.
func cacheKey(method, subject string) string { return method + ":" + subject }

// EnsureTenant mints a personal tenant for a human identity on first login and
// returns its id. Idempotent: a returning caller finds its tenant and mints
// nothing; concurrent first logins yield one tenant (the store's unique
// owner_identity_id gate). The owner is granted admin on workspace:tea-<id> so
// it can authorize its own resources — verified, not just attempted, on every
// cache-miss call (ensureGranted), not only a fresh mint: OpenFGA's /write
// errors on a tuple that already exists, so a naive "grant only on fresh mint"
// would permanently skip the grant if the very first attempt failed after the
// tenant row had already committed — the caller's every retry would then find
// "already onboarded" and never regrant, a silent lockout from its own
// workspace. ensureGranted checks first, so both a returning caller (grant
// already succeeded) and a genuine retry (grant still missing) are handled by
// the same path.
func (t *tenantService) EnsureTenant(ctx context.Context, identityID string) (string, error) {
	key := cacheKey("session", identityID)
	if tid, ok := t.cache.Get(key); ok {
		return tid, nil
	}
	tenant, err := t.store.TenantForIdentity(ctx, identityID)
	switch {
	case err == nil:
		// returning caller — tenant already exists
	case errors.Is(err, store.ErrNotFound):
		tenant, err = t.store.CreateTenantWithMember(ctx, identityID, store.PlanHobby)
		if err != nil {
			return "", err
		}
	default:
		return "", err
	}
	if err := t.ensureGranted(ctx, tenant.ID, identityID); err != nil {
		return "", err
	}
	t.cache.Put(key, tenant.ID, time.Now().Add(core.PositiveTTL))
	return tenant.ID, nil
}

// ensureGranted makes identityID admin of workspace:tenantID unless it already
// is. The check is what makes this safe to call on every cache-miss (not just
// a fresh mint) — OpenFGA errors on a duplicate write, so re-granting blindly
// would fail every call after the first success. A Check error (e.g. OpenFGA
// unreachable) falls through to attempting the grant, which fails the same
// way — fail closed either way, never a silent skip.
func (t *tenantService) ensureGranted(ctx context.Context, tenantID, identityID string) error {
	if t.granter == nil {
		return nil
	}
	subject := "user:" + identityID
	if checker, ok := t.granter.(core.Checker); ok {
		if allowed, err := checker.Check(ctx, subject, "admin", "workspace:"+tenantID); err == nil && allowed {
			return nil
		}
	}
	if err := t.granter.GrantWorkspaceAdmin(ctx, tenantID, subject); err != nil {
		return fmt.Errorf("grant workspace admin: %w", err)
	}
	return nil
}

// Tenant resolves a caller to its tenant id via tenant_members — one lookup
// serves both a session caller (Kratos identity id) and a machine caller
// (Hydra client id), since tenant_members.subject covers both. ok=false (store
// on, no tenant — an unbound machine key, or the platform bootstrap) leaves the
// caller on the default workspace; Authorize then 403s a tuple-less subject and
// admits the seeded bootstrap. Negatives are never cached so a fresh binding or
// grant takes effect immediately (mirroring the authz checker's contract).
func (t *tenantService) Tenant(ctx context.Context, id core.Identity) (string, bool) {
	key := cacheKey(id.Method, id.Subject)
	if tid, ok := t.cache.Get(key); ok {
		return tid, true
	}
	tenant, err := t.store.TenantForIdentity(ctx, id.Subject)
	if err != nil {
		return "", false
	}
	t.cache.Put(key, tenant.ID, time.Now().Add(core.PositiveTTL))
	return tenant.ID, true
}

// BindKey implements apikeys.KeyBinder: records the client's tenant_members
// row (role "developer") + writes the key's FGA developer membership. A failed
// FGA write rolls the binding back so no half-bound key lingers (the api-keys
// service separately deletes the Hydra client — no orphaned credential).
func (t *tenantService) BindKey(ctx context.Context, clientID, tenantID string) error {
	if err := t.store.BindClient(ctx, clientID, tenantID); err != nil {
		return err
	}
	if t.granter != nil {
		if err := t.granter.GrantWorkspaceMember(ctx, tenantID, "user:"+clientID); err != nil {
			// Roll back the mapping row; the caller deletes the Hydra client too.
			_ = t.store.UnbindClient(ctx, clientID)
			return fmt.Errorf("grant key membership: %w", err)
		}
	}
	return nil
}

// UnbindKey implements apikeys.KeyBinder: removes the client's tenant_members
// row + its FGA developer membership.
func (t *tenantService) UnbindKey(ctx context.Context, clientID string) error {
	// Remember the tenant before unbinding so the FGA tuple can be removed.
	tenant, foundErr := t.store.TenantForIdentity(ctx, clientID)
	if err := t.store.UnbindClient(ctx, clientID); err != nil {
		return err
	}
	// Best-effort tuple removal: a never-bound key has no tuple (foundErr != nil,
	// nothing to revoke), and deleting an absent tuple is a no-op — don't let a
	// stale tuple block revoking a key the Hydra side has already killed.
	if t.granter != nil && foundErr == nil {
		_ = t.granter.RevokeWorkspaceMember(ctx, tenant.ID, "user:"+clientID, "developer")
	}
	return nil
}
