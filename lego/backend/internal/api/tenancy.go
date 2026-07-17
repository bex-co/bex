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
	"log"
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
	TenantForOwner(ctx context.Context, identityID string) (store.Tenant, error)
	// IsMember backs the resolver's membership gate: may this caller act in a
	// workspace it NAMED (w6/m14), or one an App it named belongs to?
	IsMember(ctx context.Context, subject, tenantID string) (bool, error)
	CreateTenantWithMember(ctx context.Context, identityID, plan string) (store.Tenant, error)
	BindClient(ctx context.Context, clientID, tenantID string) error
	UnbindClient(ctx context.Context, clientID string) error
	// AcceptInvitesForEmail redeems every outstanding invite addressed to email
	// into a tenant_members row for subject and returns them, so the caller can
	// write the matching OpenFGA role tuples (w4/m12).
	AcceptInvitesForEmail(ctx context.Context, email, subject string) ([]store.Invite, error)
}

// Onboarding mints a personal tenant for a human identity on first login and
// redeems any workspace invites addressed to its email. The auth gate calls it
// for session callers only — machine (API-key) callers never mint (they resolve
// via their key's tenant binding instead) and carry no email to redeem against.
type Onboarding interface {
	EnsureTenant(ctx context.Context, identityID, email string) (tenantID string, err error)
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
	// members caches "subject X belongs to workspace Y" separately from cache's
	// "subject X's default workspace is Z" (w6/m14). Two maps, not one: a
	// TTLCache resets WHOLESALE at core.CacheMax, and membership multiplies the
	// key space by the number of workspaces each caller touches — sharing one
	// map would let membership churn evict every caller's default-workspace
	// resolution, sending them all back to Postgres.
	members *core.TTLCache[string]
	// Audit records the members.AcceptInvite row for each login-time invite
	// redemption (w1/m33) — the one membership mutation that happens outside a
	// feature verb's authorize interception. Nil => unrecorded (store-off /
	// ssh-gateway), the same degrade as core.Base's own sink.
	Audit core.AuditSink
}

// NewTenantService wires the store-backed resolver + onboarding mint. granter
// may be nil (authz off). Returns nil when the store is nil (store off — no
// resolver, no mint, the legacy default-workspace mode).
func NewTenantService(s TenantStore, granter store.MembershipGranter) *tenantService {
	if s == nil {
		return nil
	}
	return &tenantService{store: s, granter: granter, cache: core.NewTTLCache[string](), members: core.NewTTLCache[string]()}
}

// cacheKey namespaces the resolver cache by auth method so a Kratos identity id
// and a Hydra client_id can never shadow each other.
func cacheKey(method, subject string) string { return method + ":" + subject }

// The two core.Identity.Method values this file's cache keys off of (see
// core/identity.go's Method field). Named here (rather than repeating the
// literals at every cacheKey call site in this file) so InvalidateTenant's
// two-key eviction can't drift from EnsureTenant's mint.
const (
	methodSession = "session"
	methodOAuth2  = "oauth2"
)

// EnsureTenant resolves a human identity's PERSONAL tenant on login (minting it
// on first login) and redeems any workspace invites addressed to its email.
// Returns the personal tenant's id. Idempotent: concurrent first logins yield
// one tenant (the store's unique owner_identity_id gate). The owner is granted
// admin on workspace:tea-<id> so it can authorize its own resources — verified,
// not just attempted, on every cache-miss call (ensureGranted), not only a fresh
// mint: OpenFGA's /write errors on a tuple that already exists, so a naive "grant
// only on fresh mint" would permanently skip the grant if the very first attempt
// failed after the tenant row had already committed — the caller's every retry
// would then find "already onboarded" and never regrant, a silent lockout from
// its own workspace. ensureGranted checks first, so both a returning caller
// (grant already succeeded) and a genuine retry (grant still missing) are handled
// by the same path.
//
// The personal tenant is resolved by OWNER identity (owner_identity_id), NOT by
// membership: once a caller can be a member of workspaces it does not own (w4/m12
// invites), a membership lookup could return an INVITED workspace and
// ensureGranted would wrongly grant the caller admin there. Owner-keyed
// resolution keeps the admin grant pinned to the workspace the caller actually
// owns. A returning caller costs one owner-keyed SELECT (TenantForOwner); only a
// first login falls to the idempotent CreateTenantWithMember upsert.
func (t *tenantService) EnsureTenant(ctx context.Context, identityID, email string) (string, error) {
	key := cacheKey(methodSession, identityID)
	if tid, ok := t.cache.Get(key); ok {
		return tid, nil
	}
	// Redeem pending invites on the cache-miss path (a fresh signup is always a
	// miss, so a new teammate lands in the workspace immediately; a returning
	// caller redeems within the cache TTL). Done before the personal-tenant
	// resolution so the two are independent; acceptInvites no-ops without an email.
	t.acceptInvites(ctx, identityID, email)

	tenant, err := t.store.TenantForOwner(ctx, identityID)
	switch {
	case err == nil:
		// returning caller — personal tenant already minted
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

// acceptInvites redeems every pending invite addressed to the caller's email
// into a membership + its OpenFGA role tuple — how an invited teammate lands in
// the workspace on first login (w4/m12). The membership row is the source of
// truth and is written transactionally; the FGA tuple is a best-effort follow-up
// (Postgres and OpenFGA aren't one transaction), logged on failure rather than
// surfaced so a flaky OpenFGA never blocks a login. A redeemed invite is
// terminal (accepted_at set), so a failed tuple write is a rare, narrow window
// consistent with the rest of the system's row-is-truth/tuple-best-effort model.
// No email (a machine caller, or an identity without the trait) means nothing to
// redeem.
func (t *tenantService) acceptInvites(ctx context.Context, identityID, email string) {
	if email == "" {
		return
	}
	accepted, err := t.store.AcceptInvitesForEmail(ctx, email, identityID)
	if err != nil {
		log.Printf("tenancy: redeeming invites for %s: %v", identityID, err)
		return
	}
	for _, inv := range accepted {
		// The caller is the ACCEPTING identity — the teammate joining on login,
		// method "session" by construction (only session callers reach EnsureTenant).
		core.RecordInviteAccepted(ctx, t.Audit, time.Now(),
			inv.TenantID, inv.ID, inv.Email, inv.Role, identityID, methodSession)
	}
	if t.granter == nil {
		return
	}
	for _, inv := range accepted {
		if err := t.granter.GrantWorkspaceRole(ctx, inv.TenantID, "user:"+identityID, inv.Role); err != nil {
			log.Printf("tenancy: granting %s on workspace %s to %s: %v", inv.Role, inv.TenantID, identityID, err)
		}
	}
	// The caller's cached resolution (their personal workspace) stays valid — a
	// new membership elsewhere doesn't unseat it, and the switcher's workspace
	// list (ListTenantsForSubject) is uncached, so the invited workspace shows
	// immediately. Which of several workspaces is "active" is a w1/m9 concern
	// (single-tenant resolution), not this feature's.
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

// IsMember reports whether the caller belongs to a NAMED workspace — the gate
// core.Base runs before honoring an explicit workspace (REST/GraphQL ownerId, an
// MCP session's select_workspace) or reaching an App that lives in another of
// the caller's workspaces (w6/m14). It answers from tenant_members, the source
// of truth, NOT from the resolver cache: the cache holds the caller's ONE
// default workspace, which says nothing about the others they belong to.
//
// Positives are cached (a membership is stable, and a request can consult this
// several times — Authorize, then GetApp); negatives never are, so a fresh
// invite or binding takes effect immediately, matching Tenant's own contract. A
// store error is surfaced, not swallowed as "not a member": core.Base fails the
// request closed (ErrAuthzUnavailable) rather than mistaking an outage for a
// refusal — and, critically, rather than silently falling back to the caller's
// own workspace, which would land a write in the wrong one.
func (t *tenantService) IsMember(ctx context.Context, id core.Identity, tenantID string) (bool, error) {
	key := cacheKey(id.Method, id.Subject) + ":" + tenantID
	if _, ok := t.members.Get(key); ok {
		return true, nil
	}
	member, err := t.store.IsMember(ctx, id.Subject, tenantID)
	if err != nil {
		return false, err
	}
	if member {
		t.members.Put(key, tenantID, time.Now().Add(core.PositiveTTL))
	}
	return member, nil
}

// InvalidateTenant evicts subject's cached tenant resolution under both
// possible core.Identity.Method values (a given subject is only ever active
// under one — "session" for a Kratos identity, "oauth2" for a bound API key —
// so evicting both is always safe and lets the caller invalidate by subject
// alone, without knowing which method minted the cached entry). Called by
// workspaces.Service.Delete for every member it revokes, so a request racing
// the delete re-resolves instead of riding the stale positive answer for the
// rest of its core.PositiveTTL window: without this, Tenant kept resolving a
// revoked member (including the deleter) to the just-deleted tenant for up to
// 30s, and core.Base.Authorize then 403'd every verb against it — including
// workspaces.Service.List, which is supposed to be membership-scoped and
// return the caller's OTHER workspaces regardless (m13 live-verify finding:
// the dashboard's workspace switcher went blank for up to 30s after a
// self-delete even though the account still owned a second workspace).
func (t *tenantService) InvalidateTenant(subject string) {
	t.cache.Delete(cacheKey(methodSession, subject))
	t.cache.Delete(cacheKey(methodOAuth2, subject))
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

// TenantForKey implements apikeys.KeyBinder: resolves the workspace a bound
// key belongs to, via the same cache-backed lookup Tenant uses (a bound key's
// tenant is as stable as any caller's, w6/m18) — keyed under methodOAuth2 so
// it can never collide with a human identity's cached resolution even if the
// two ids ever coincided.
func (t *tenantService) TenantForKey(ctx context.Context, clientID string) (string, bool) {
	return t.Tenant(ctx, core.Identity{Subject: clientID, Method: methodOAuth2})
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
