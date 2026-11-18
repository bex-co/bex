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

package tenancy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Package tenancy is the store-backed bridge between the auth gate and
// core.Base's workspace resolver (w1/m9). The auth gate mints a personal tenant
// on a human's first login (Onboarding); core.Base.Authorize resolves every
// caller to its workspace (core.WorkspaceResolver); the api-keys service binds
// a minted key to its tenant (apikeys.KeyBinder). One Service implements all
// three — it sits here, in its own feature package, rather than the api
// composition root, so the ssh-gateway (which needs only tenant resolution,
// never the HTTP surface) does not import internal/api; one TTL cache
// deduplicates the identity→tenant lookups a single request fans out
// (Authorize + List/Get).

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
	// SetTenantDisplayNameIfUnset names a signup workspace once (w3/m246).
	SetTenantDisplayNameIfUnset(ctx context.Context, id, displayName string) error
	BindClient(ctx context.Context, clientID, tenantID string) error
	UnbindClient(ctx context.Context, clientID string) error
	// ClientBindingsOfTenants and BumpOAuthRevocation serve a deleted
	// workspace's key teardown (w9/120): the keys bound there, and the marker
	// that refuses a key's tokens on every replica while its row still exists.
	ClientBindingsOfTenants(ctx context.Context, tenantIDs []string) (map[string]string, error)
	BumpOAuthRevocation(ctx context.Context, subject, clientID string) error
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
	EnsureTenant(ctx context.Context, identityID, email string, emailVerified bool) (tenantID string, err error)
}

type accountDeletionReader interface {
	AccountDeletionTombstoned(context.Context, string) (bool, error)
}

// AccountDeletionTombstoned implements apikeys.AccountDeletionReader over the
// same durable tombstone used by onboarding. Stores predating account deletion
// preserve their legacy behavior.
func (t *Service) AccountDeletionTombstoned(ctx context.Context, subject string) (bool, error) {
	if deletions, ok := t.store.(accountDeletionReader); ok {
		return deletions.AccountDeletionTombstoned(ctx, subject)
	}
	return false, nil
}

// Service implements both core.WorkspaceResolver (the read path
// Authorize/List/Get use) and Onboarding (the mint the auth gate drives), over
// one store + one TTL cache. The granter is nil when OpenFGA is off (store on,
// authz off): tenants are still minted and keys still bound, just without the
// FGA membership tuples — a correct intermediate where the store isolates
// tenants by filtering and authz isn't yet enforcing roles.
type Service struct {
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
	// recent remembers each person's personal workspace for recentTenantTTL
	// after onboarding last resolved it, so a transient store or OpenFGA fault
	// on a cache miss serves that workspace instead of a 503 (w32/m446).
	recent *core.TTLCache[string]
	// Audit records the members.AcceptInvite row for each login-time invite
	// redemption (w1/m33) — the one membership mutation that happens outside a
	// feature verb's authorize interception. Nil => unrecorded (store-off /
	// ssh-gateway), the same degrade as core.Base's own sink.
	Audit core.AuditSink
	// RequireVerifiedInviteEmail gates login-time invite redemption on the caller's
	// email being Kratos-verified (BEX_REQUIRE_VERIFIED_INVITE_EMAIL, w1/m53).
	// Secure by DEFAULT (w1/m65 F13): true unless explicitly set to 0/false, so an
	// unverified address can't claim another user's invite. The emailed
	// ?invite=<token> bearer link redeems regardless of this flag.
	RequireVerifiedInviteEmail bool
	// OrgTuples writes the org role tuple of someone a login-time invite
	// redemption made an org Member (w7/m464). Nil when authz is off.
	OrgTuples core.OrganizationTupleWriter
}

// NewTenantService wires the store-backed resolver + onboarding mint. granter
// may be nil (authz off). Returns nil when the store is nil (store off — no
// resolver, no mint, the legacy default-workspace mode).
func NewTenantService(s TenantStore, granter store.MembershipGranter) *Service {
	if s == nil {
		return nil
	}
	return &Service{store: s, granter: granter, cache: core.NewTTLCache[string](), members: core.NewTTLCache[string](), recent: core.NewTTLCache[string]()}
}

// recentTenantTTL bounds how long after a successful onboarding a person's
// personal workspace can stand in for a cache-miss resolution that failed.
const recentTenantTTL = 10 * time.Minute

// The kinds of dependency an onboarding step reads or writes, named in the 503
// a caller gets when the step fails.
const (
	DepStore = "workspace store"
	DepAuthz = "authorization service"
)

// The onboarding steps that can fail a request. Each names what it was doing,
// in the log line and in the 503 body. Exported: the steps and dependencies
// are the user-visible 503 contract (see UnavailableMessage), pinned by the api
// gate tests, so they are spelled once here.
const (
	StepTombstone = "account deletion check"
	StepOwnerRead = "personal workspace lookup"
	StepCreate    = "personal workspace creation"
	StepGrant     = "workspace admin grant"
	StepReread    = "personal workspace re-read"
	StepRevoke    = "admin revoke after the workspace binding ended"
)

// onboardingStepError is an EnsureTenant failure that names its step and the
// dependency behind it (w32/m446). The auth gate logs the whole error and
// answers UnavailableMessage, which carries the step and dependency
// but never the cause's own text: a store error can carry SQL or a connection
// string.
type onboardingStepError struct {
	step, dependency string
	err              error
}

func (e *onboardingStepError) Error() string {
	return e.step + " failed (" + e.dependency + "): " + e.err.Error()
}

func (e *onboardingStepError) Unwrap() error { return e.err }

// StepFailed tags err with the step that produced it. A coded refusal (account
// deletion pending) is a public answer of its own and passes through as is.
func StepFailed(step, dependency string, err error) error {
	var coded *core.CodedError
	if err == nil || errors.As(err, &coded) {
		return err
	}
	return &onboardingStepError{step: step, dependency: dependency, err: err}
}

// UnavailableMessage is the 503 body for an onboarding failure that is
// not a coded refusal: which step failed against which dependency, when the
// error says.
func UnavailableMessage(err error) string {
	const msg = "tenant onboarding unavailable"
	var step *onboardingStepError
	if errors.As(err, &step) {
		return msg + ": " + step.step + " failed (" + step.dependency + ")"
	}
	return msg
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
// mint: a naive "grant only on fresh mint" would permanently skip the grant if
// the very first attempt failed after the tenant row had already committed — the
// caller's every retry would then find "already onboarded" and never regrant, a
// silent lockout from its own workspace. ensureGranted checks first, so both a
// returning caller (grant already succeeded) and a genuine retry (grant still
// missing) are handled by the same path.
//
// The personal tenant is resolved by OWNER identity (owner_identity_id), NOT by
// membership: once a caller can be a member of workspaces it does not own (w4/m12
// invites), a membership lookup could return an INVITED workspace and
// ensureGranted would wrongly grant the caller admin there. Owner-keyed
// resolution keeps the admin grant pinned to the workspace the caller actually
// owns. A returning caller costs one owner-keyed SELECT (TenantForOwner); only a
// first login falls to the idempotent CreateTenantWithMember upsert.
func (t *Service) EnsureTenant(ctx context.Context, identityID, email string, emailVerified bool) (string, error) {
	// ADR086: the tombstone wins over even a positive tenant cache entry. It is
	// checked before invite redemption and tenant creation so cleanup can never
	// race a request into reminting the personal workspace.
	//
	// A tombstone that cannot be read fails the request closed, cache or no
	// cache: an unreadable tombstone could be a deletion in progress, and only
	// the tombstone keeps a deleted account from being served (w32/m446 t003).
	key := cacheKey(methodSession, identityID)
	if deletions, ok := t.store.(accountDeletionReader); ok {
		pending, err := deletions.AccountDeletionTombstoned(ctx, identityID)
		if err != nil {
			return "", StepFailed(StepTombstone, DepStore, err)
		}
		if pending {
			t.cache.Delete(key)
			t.recent.Delete(key)
			return "", core.NewAccountDeletionPendingError()
		}
	}
	if tid, ok := t.cache.Get(key); ok {
		return tid, nil
	}
	// Redeem pending invites on the cache-miss path (a fresh signup is always a
	// miss, so a new teammate lands in the workspace immediately; a returning
	// caller redeems within the cache TTL). Done before the personal-tenant
	// resolution so the two are independent; acceptInvites no-ops without an email.
	t.acceptInvites(ctx, identityID, email, emailVerified)

	tenant, err := t.personalTenant(ctx, identityID)
	if errors.Is(err, errOwnerBindingEnded) {
		// The binding ended while this request granted admin; the grant is taken
		// back, and this resolves (or mints) the caller's new personal workspace.
		tenant, err = t.personalTenant(ctx, identityID)
	}
	if err != nil {
		return t.servedThroughFault(identityID, tenant, err)
	}
	t.nameSignupWorkspace(ctx, tenant, email)
	now := time.Now()
	t.cache.Put(key, tenant.ID, now.Add(core.PositiveTTL))
	t.recent.Put(key, tenant.ID, now.Add(recentTenantTTL))
	return tenant.ID, nil
}

// servedThroughFault answers a cache-miss onboarding failure. A person whose
// personal workspace onboarding resolved within recentTenantTTL keeps that
// workspace through a transient fault, instead of a 503, when nothing says the
// workspace stopped being theirs: the owner lookup itself failed, or the admin
// grant failed on that same workspace. The workspace goes back in the cache for
// core.PositiveTTL, as a hit would have served it, and the fault is logged.
//
// Serving it grants nothing: every verb still authorizes against OpenFGA. A
// first-ever caller has no recent workspace and still gets the error, and so
// does a failure after a grant this request wrote (the re-read that decides
// whether to take the grant back), a creation (the old binding is gone) or a
// tombstone read, which runs before this.
func (t *Service) servedThroughFault(identityID string, tenant store.Tenant, err error) (string, error) {
	var step *onboardingStepError
	if !errors.As(err, &step) {
		return "", err
	}
	key := cacheKey(methodSession, identityID)
	recent, ok := t.recent.Get(key)
	if !ok || !(step.step == StepOwnerRead || step.step == StepGrant && tenant.ID == recent) {
		return "", err
	}
	log.Printf("tenancy: serving %s its personal workspace %s, resolved within %s, through a failed onboarding: %v",
		identityID, recent, recentTenantTTL, err)
	t.cache.Put(key, recent, time.Now().Add(core.PositiveTTL))
	return recent, nil
}

// errOwnerBindingEnded reports that the caller's personal-workspace binding
// ended between its read and the admin grant that followed.
var errOwnerBindingEnded = errors.New("personal workspace binding ended during onboarding")

// personalTenant resolves (or mints) the caller's personal workspace and makes
// sure they are its admin.
//
// Since w5/m372 another admin may remove or demote a workspace's creator, and
// the store ends the binding in that transaction. A request that read the
// binding just before can grant admin just after, which would leave a removed
// creator an admin tuple, or a demoted one admin on top of their new role. So a
// grant this call actually wrote is followed by a second read of the binding:
// if it ended, the admin tuple is revoked and errOwnerBindingEnded returned.
// The remover covers the other order (a grant that lands before its commit) by
// revoking the admin tuple again after it commits (members.endMembership; a
// demotion already revokes after its commit).
//
// The revoke cannot tell a creator removed or demoted from one demoted and then
// promoted back to admin in the same instant; that member is left with no
// admin tuple (fail closed) until their role is set again.
func (t *Service) personalTenant(ctx context.Context, identityID string) (store.Tenant, error) {
	tenant, err := t.store.TenantForOwner(ctx, identityID)
	switch {
	case err == nil:
		// returning caller — personal tenant already minted
	case errors.Is(err, store.ErrNotFound):
		tenant, err = t.store.CreateTenantWithMember(ctx, identityID, store.PlanHobby)
		if errors.Is(err, store.ErrAccountDeletionPending) {
			key := cacheKey(methodSession, identityID)
			t.cache.Delete(key)
			t.recent.Delete(key)
			return store.Tenant{}, core.NewAccountDeletionPendingError()
		}
		if err != nil {
			return store.Tenant{}, StepFailed(StepCreate, DepStore, err)
		}
	default:
		return store.Tenant{}, StepFailed(StepOwnerRead, DepStore, err)
	}
	granted, err := t.ensureGranted(ctx, tenant.ID, identityID)
	if err != nil || !granted {
		return tenant, StepFailed(StepGrant, DepAuthz, err)
	}
	again, err := t.store.TenantForOwner(ctx, identityID)
	switch {
	case err == nil && again.ID == tenant.ID:
		return tenant, nil
	case err != nil && !errors.Is(err, store.ErrNotFound):
		return store.Tenant{}, StepFailed(StepReread, DepStore, err)
	}
	if err := t.granter.RevokeWorkspaceMember(ctx, tenant.ID, "user:"+identityID, "admin"); err != nil {
		return store.Tenant{}, StepFailed(StepRevoke, DepAuthz, err)
	}
	return store.Tenant{}, errOwnerBindingEnded
}

// acceptInvites redeems every pending invite addressed to the caller's email
// into a membership + its OpenFGA role tuple — how an invited teammate lands in
// the workspace on first login (w4/m12). The membership row and durable
// reconciliation intent commit in one Postgres transaction. OpenFGA remains an
// asynchronous side effect, but a failed write is retained for the members
// worker instead of being silently stranded after accepted_at becomes terminal.
// No email (a machine caller, or an identity without the trait) means nothing to
// redeem.
func (t *Service) acceptInvites(ctx context.Context, identityID, email string, emailVerified bool) {
	if email == "" {
		return
	}
	// w1/m53: an invite addresses an email, and Kratos issues a session before the
	// address is verified — so an attacker who registers with a victim's
	// not-yet-signed-up invited address would otherwise redeem their invite on
	// first login. When RequireVerifiedInviteEmail is set, refuse redemption for an
	// unverified email. Default off preserves the current behavior on deployments
	// whose email-verification UX isn't complete yet (docs/ADR024-members.md).
	if t.RequireVerifiedInviteEmail && !emailVerified {
		log.Printf("tenancy: skipping invite redemption for %s: email %s not verified", identityID, email)
		return
	}
	t.acceptOrganizationInvites(ctx, email, identityID)
	accepted, err := t.store.AcceptInvitesForEmail(ctx, email, identityID)
	if err != nil {
		if errors.Is(err, store.ErrAccountDeletionPending) {
			return
		}
		log.Printf("tenancy: redeeming invites for %s: %v", identityID, err)
		return
	}
	for _, inv := range accepted {
		// The caller is the ACCEPTING identity — the teammate joining on login,
		// method "session" by construction (only session callers reach EnsureTenant).
		core.RecordInviteAccepted(ctx, t.Audit, time.Now(),
			inv.TenantID, inv.ID, inv.Email, inv.Role, identityID, methodSession)
		// An org workspace's invite made a new person a Member of its org
		// (w7/m464, ADR115 D4): their org tuple and AddOrgMemberEvent.
		if inv.JoinedOrganizationID != "" {
			core.JoinedOrganization(ctx, t.OrgTuples, t.Audit, time.Now(), inv.JoinedOrganizationID, identityID, methodSession)
		}
	}
	if t.granter == nil {
		for _, inv := range accepted {
			t.completeInviteRoleReconciliation(ctx, inv, identityID)
		}
		return
	}
	for _, inv := range accepted {
		if err := t.reconcileInviteRole(ctx, inv.TenantID, "user:"+identityID, inv.Role); err != nil {
			log.Printf("tenancy: reconciling %s on workspace %s to %s: %v", inv.Role, inv.TenantID, identityID, err)
			t.failInviteRoleReconciliation(ctx, inv, identityID, err)
			continue
		}
		t.completeInviteRoleReconciliation(ctx, inv, identityID)
	}
	// The caller's cached resolution (their personal workspace) stays valid — a
	// new membership elsewhere doesn't unseat it, and the switcher's workspace
	// list (ListTenantsForSubject) is uncached, so the invited workspace shows
	// immediately. Which of several workspaces is "active" is a w1/m9 concern
	// (single-tenant resolution), not this feature's.
}

func (t *Service) completeInviteRoleReconciliation(ctx context.Context, inv store.Invite, subject string) {
	if queue, ok := t.store.(members.RoleReconciliationStore); ok {
		if err := queue.CompleteRoleReconciliation(ctx, inv.TenantID, subject, inv.Role); err != nil {
			log.Printf("tenancy: acknowledging role reconciliation for %s/%s: %v", inv.TenantID, subject, err)
		}
	}
}

func (t *Service) failInviteRoleReconciliation(ctx context.Context, inv store.Invite, subject string, reconcileErr error) {
	if queue, ok := t.store.(members.RoleReconciliationStore); ok {
		if err := queue.FailRoleReconciliation(ctx, inv.TenantID, subject, inv.Role, reconcileErr.Error()); err != nil {
			log.Printf("tenancy: retaining failed role reconciliation for %s/%s: %v", inv.TenantID, subject, err)
		}
	}
}

// reconcileInviteRole revokes every OTHER role tuple the subject already holds
// on the workspace and grants the invited role, so redeeming an invite for
// someone who is already a member at a different role leaves EXACTLY the invited
// role (w1/m65 F7) — the OR-based model would otherwise keep the higher of the
// two effective. round-19 #3: the revokes run BEFORE the grant so a partial
// failure strands the subject under-privileged (fail-closed, repaired by the
// role-reconciliation outbox) instead of riding a union of old+new. Best-effort
// like the grant it replaces; check-before-revoke skips absent tuples, and with
// no checker it degrades to grant-only (prior behavior).
func (t *Service) reconcileInviteRole(ctx context.Context, tenantID, subject, role string) error {
	if t.granter == nil {
		return nil
	}
	checker, ok := t.granter.(core.Checker)
	if ok {
		if err := t.revokeOtherRoles(ctx, checker, tenantID, subject, role); err != nil {
			return err
		}
	}
	return t.granter.GrantWorkspaceRole(ctx, tenantID, subject, role)
}

// revokeOtherRoles removes every role tuple except `role`, check-before-revoke.
func (t *Service) revokeOtherRoles(ctx context.Context, checker core.Checker, tenantID, subject, role string) error {
	for _, other := range members.Roles {
		if other == role {
			continue
		}
		var has bool
		var err error
		if fresh, ok := t.granter.(core.FreshChecker); ok {
			has, err = fresh.CheckFresh(ctx, subject, other, "workspace:"+tenantID)
		} else {
			has, err = checker.Check(ctx, subject, other, "workspace:"+tenantID)
		}
		if err == nil && !has {
			continue // definitively absent — nothing to revoke
		}
		// round-5 finding 16: a Check ERROR must NOT be folded into "role absent".
		// The old `err != nil || !has` skipped the revoke on a transient OpenFGA
		// failure, leaving a higher stale role tuple the OR-based model keeps
		// effective — and this login path has no retry driver. On an indeterminate
		// check, attempt the revoke anyway: deleting an absent tuple is a no-op
		// (authz.RevokeWorkspaceMember is idempotent), and a real error surfaces
		// to the caller instead of being silently swallowed.
		if err := t.granter.RevokeWorkspaceMember(ctx, tenantID, subject, other); err != nil {
			return err
		}
	}
	return nil
}

// ensureGranted makes identityID admin of workspace:tenantID unless it already
// is. The check keeps every cache-miss from writing a tuple that is already
// there, and tells granted apart from "was already admin". A Check error (e.g.
// OpenFGA unreachable) is logged and falls through to the grant: a write of a
// tuple that is already there is a no-op (authz.writeTuple), so an existing
// admin is still served, and an OpenFGA that cannot take the write either fails
// the request closed, never a silent skip. granted reports whether it wrote the
// tuple, which a no-op write also reports, so personalTenant's re-read runs.
//
// The Check gets no retry of its own (w32/m446 t002): the transport already
// resends one that lands on a dropped keep-alive connection (w4/695), and the
// grant answers for any other Check error.
func (t *Service) ensureGranted(ctx context.Context, tenantID, identityID string) (granted bool, err error) {
	if t.granter == nil {
		return false, nil
	}
	subject := "user:" + identityID
	if checker, ok := t.granter.(core.Checker); ok {
		allowed, err := checker.Check(ctx, subject, "admin", "workspace:"+tenantID)
		if err == nil && allowed {
			return false, nil
		}
		if err != nil {
			log.Printf("tenancy: admin check of %s on workspace %s failed, so the grant is written again: %v", identityID, tenantID, err)
		}
	}
	if err := t.granter.GrantWorkspaceAdmin(ctx, tenantID, subject); err != nil {
		return false, err
	}
	return true, nil
}

// Tenant resolves a caller to its tenant id via tenant_members — one lookup
// serves both a session caller (Kratos identity id) and a machine caller
// (Hydra client id), since tenant_members.subject covers both. ok=false (store
// on, no tenant — an unbound machine key, or the platform bootstrap) leaves the
// caller on the default workspace; Authorize then 403s a tuple-less subject and
// admits the seeded bootstrap. Negatives are never cached so a fresh binding or
// grant takes effect immediately (mirroring the authz checker's contract).
func (t *Service) Tenant(ctx context.Context, id core.Identity) (string, bool) {
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
// MCP's per-call workspaceId) or reaching an App that lives in another of
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
func (t *Service) IsMember(ctx context.Context, id core.Identity, tenantID string) (bool, error) {
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
//
// It also forgets the person's recently resolved workspace, so a store fault
// right after the membership changed never serves the workspace they just left.
func (t *Service) InvalidateTenant(subject string) {
	t.cache.Delete(cacheKey(methodSession, subject))
	t.cache.Delete(cacheKey(methodOAuth2, subject))
	t.recent.Delete(cacheKey(methodSession, subject))
}

// InvalidateMembership evicts subject's cached POSITIVE membership in one
// workspace — the second cache, which InvalidateTenant does not touch (w5/m102).
// IsMember caches only positives for core.PositiveTTL, so after a member leaves,
// a request that explicitly NAMES the workspace they just left would keep
// resolving into it for the rest of that window. OpenFGA is still the
// authorization gate (the role tuple is revoked, so every verb 403s), which is
// why this was never an access bypass — but resolving into a workspace the
// caller no longer belongs to produces exactly the m13 class of symptom, so the
// leave path evicts it. Both methods, for the same reason InvalidateTenant
// evicts both.
func (t *Service) InvalidateMembership(subject, tenantID string) {
	t.members.Delete(cacheKey(methodSession, subject) + ":" + tenantID)
	t.members.Delete(cacheKey(methodOAuth2, subject) + ":" + tenantID)
}

// creatorKeyBinder seats a key at its creator's role (w9/m481).
// *store.PGStore implements it; a store without it binds every key as a
// Developer, the binding before.
type creatorKeyBinder interface {
	BindClientForCreator(ctx context.Context, clientID, tenantID, creator string) (string, error)
}

// BindKey implements apikeys.KeyBinder: records the client's tenant_members
// row at its creator's current role in tenantID (w9/m481, docs/ADR024-members.md
// § An API key acts with its creator's role) and writes the key's matching FGA
// role tuple. With no recorded creator (the store-off and authz-off mints) the
// key is bound as a Developer and never moves from there (invariant I3). A
// failed FGA write rolls the binding back so no half-bound key lingers (the
// api-keys service separately deletes the Hydra client — no orphaned
// credential).
func (t *Service) BindKey(ctx context.Context, clientID, tenantID, createdBy string) error {
	role := "developer"
	if binder, ok := t.store.(creatorKeyBinder); ok && createdBy != "" {
		seated, err := binder.BindClientForCreator(ctx, clientID, tenantID, createdBy)
		switch {
		case errors.Is(err, store.ErrRoleUnsettled):
			return fmt.Errorf("%w: your workspace role change is still being applied; retry shortly", core.ErrForbidden)
		case errors.Is(err, store.ErrNotFound):
			return fmt.Errorf("%w: you no longer hold a role that may create API keys in this workspace", core.ErrForbidden)
		case err != nil:
			return err
		}
		role = seated
	} else if err := t.store.BindClient(ctx, clientID, tenantID); err != nil {
		return err
	}
	if t.granter != nil {
		if err := t.granter.GrantWorkspaceRole(ctx, tenantID, "user:"+clientID, role); err != nil {
			// Roll back the mapping row; the caller deletes the Hydra client too.
			_ = t.store.UnbindClient(ctx, clientID)
			return fmt.Errorf("grant key membership: %w", err)
		}
	}
	return nil
}

// WithTenantAdvisoryLock implements apikeys.KeyQuotaLocker. Production's
// *store.PGStore holds the lock across the API-key count→Hydra-create→bind
// sequence; non-Postgres stores preserve the legacy in-process behavior.
func (t *Service) WithTenantAdvisoryLock(ctx context.Context, tenantID string, fn func() error) error {
	if locker, ok := t.store.(interface {
		WithTenantAdvisoryLock(context.Context, string, func() error) error
	}); ok {
		return locker.WithTenantAdvisoryLock(ctx, tenantID, fn)
	}
	return fn()
}

// TenantForKey implements apikeys.KeyBinder: resolves the workspace a bound
// key belongs to, via the same cache-backed lookup Tenant uses (a bound key's
// tenant is as stable as any caller's, w6/m18) — keyed under methodOAuth2 so
// it can never collide with a human identity's cached resolution even if the
// two ids ever coincided.
func (t *Service) TenantForKey(ctx context.Context, clientID string) (string, bool) {
	return t.Tenant(ctx, core.Identity{Subject: clientID, Method: methodOAuth2})
}

// UnbindKey implements apikeys.KeyBinder: removes every FGA role tuple the
// client may hold in its workspace (a key follows its creator's role since
// w9/m481, so it is not always a Developer) and its tenant_members row.
//
// SECURITY: the OpenFGA tuple is revoked BEFORE the local binding is deleted,
// and neither its failure nor a failed tenant lookup is discarded. The tuple is
// what still authorizes an already-minted token (a client-credentials token's
// subject is the client id itself, so it needs no local row to be an FGA
// subject); the tenant_members row is the retry's only way back to the tenant.
// The old order deleted the row first, so a failed revoke stranded a live
// developer tuple whose owner row was already gone — a renewable credential the
// normal retry could no longer finish revoking (codex finding-4, CWE-613). Now a
// failed revoke leaves BOTH intact: the caller (RevokeAPIKey) withholds the
// Hydra delete and a retry re-resolves the tenant and completes. Revoke is
// idempotent (authz.writeTuple), so once the tuple is gone the subsequent local
// delete — or its retry — converges cleanly with no orphaned authority.
func (t *Service) UnbindKey(ctx context.Context, clientID string) error {
	// Resolve the tenant while its binding still exists, then revoke the FGA
	// tuple, then drop the local row.
	tenant, foundErr := t.store.TenantForIdentity(ctx, clientID)
	switch {
	case foundErr == nil:
		if t.granter != nil {
			if err := t.revokeKeyRoles(ctx, tenant.ID, "user:"+clientID); err != nil {
				return fmt.Errorf("revoke key membership: %w", err)
			}
		}
	case errors.Is(foundErr, store.ErrNotFound):
		// No binding row — nothing to revoke in FGA either; fall through to the
		// idempotent local delete (e.g. a retry after a prior partial revoke).
	default:
		// A transient lookup failure must not lead to deleting the local row
		// while a developer tuple may still be live. Fail closed; the caller
		// retries once the store is reachable again.
		return foundErr
	}
	if err := t.store.UnbindClient(ctx, clientID); err != nil {
		return err
	}
	return nil
}

// WorkspaceKeys implements apikeys.WorkspaceKeyBinding: the client ids of the
// API keys bound to tenantID, sorted (w9/120).
func (t *Service) WorkspaceKeys(ctx context.Context, tenantID string) ([]string, error) {
	bound, err := t.store.ClientBindingsOfTenants(ctx, []string{tenantID})
	if err != nil {
		return nil, err
	}
	return slices.Sorted(maps.Keys(bound)), nil
}

// RevokeWorkspaceKey implements apikeys.WorkspaceKeyBinding: it takes every
// role tuple the key holds in tenantID and refuses its tokens on every replica
// (the permanent machine marker UnbindKey also writes), but keeps the key's
// binding row (w9/120). A deleted workspace's teardown deletes the Hydra client
// next and leaves the row to the tenant cascade, so a delete that fails in
// between still finds the key on its retry. Idempotent.
func (t *Service) RevokeWorkspaceKey(ctx context.Context, tenantID, clientID string) error {
	if t.granter != nil {
		if err := t.revokeKeyRoles(ctx, tenantID, "user:"+clientID); err != nil {
			return fmt.Errorf("revoke key membership: %w", err)
		}
	}
	return t.store.BumpOAuthRevocation(ctx, clientID, clientID)
}

// revokeKeyRoles removes every role tuple a key holds in tenantID, not only
// the one its row names: a role change still converging can leave it a tuple
// the row no longer names (w9/m481). Check-before-revoke, as
// revokeOtherRoles does, so no revoke asks OpenFGA to delete a tuple that is
// not there; a granter that cannot check revokes every role blindly.
func (t *Service) revokeKeyRoles(ctx context.Context, tenantID, subject string) error {
	if checker, ok := t.granter.(core.Checker); ok {
		return t.revokeOtherRoles(ctx, checker, tenantID, subject, "")
	}
	for _, role := range members.Roles {
		if err := t.granter.RevokeWorkspaceMember(ctx, tenantID, subject, role); err != nil {
			return err
		}
	}
	return nil
}

// nameSignupWorkspace names a signup workspace (slug = id, no display name)
// "<Name>'s Workspace", as Render does (w3/m246). The person's name is only
// known here, so workspaces created before display names are named on the
// owner's next sign-in. Best effort: a failure never blocks the caller.
func (t *Service) nameSignupWorkspace(ctx context.Context, tenant store.Tenant, email string) {
	if tenant.DisplayName != "" || tenant.Name != tenant.ID {
		return
	}
	var name string
	if id, ok := core.IdentityFrom(ctx); ok {
		name = id.Name
	}
	display := signupWorkspaceName(name, email)
	if display == "" {
		return
	}
	if err := t.store.SetTenantDisplayNameIfUnset(ctx, tenant.ID, display); err != nil {
		log.Printf("tenancy: name workspace %s: %v", tenant.ID, err)
	}
}

// signupWorkspaceName is Render's default workspace name: the person's name,
// else their email's local part, as "<who>'s Workspace". Empty when neither
// is known.
func signupWorkspaceName(name, email string) string {
	who := strings.TrimSpace(name)
	if who == "" {
		who, _, _ = strings.Cut(strings.TrimSpace(email), "@")
	}
	if who == "" {
		return ""
	}
	return who + "'s Workspace"
}

// organizationInviteStore is the org invites' login-time redemption; the
// production store has it, a TenantStore fake need not.
type organizationInviteStore interface {
	AcceptOrganizationInvitesForEmail(ctx context.Context, email, subject string) ([]store.OrganizationInvite, error)
}

// acceptOrganizationInvites redeems the org invites addressed to the caller's
// verified email on sign-in (w7/m464, ADR115 D4), as workspace invites are:
// the new member's org tuple and Render's AcceptOrgInviteEvent.
func (t *Service) acceptOrganizationInvites(ctx context.Context, email, identityID string) {
	invites, ok := t.store.(organizationInviteStore)
	if !ok {
		return
	}
	joined, err := invites.AcceptOrganizationInvitesForEmail(ctx, email, identityID)
	if err != nil {
		if !errors.Is(err, store.ErrAccountDeletionPending) {
			log.Printf("tenancy: redeeming organization invites for %s: %v", identityID, err)
		}
		return
	}
	for _, inv := range joined {
		if err := core.WriteOrgTuples(ctx, t.OrgTuples, []core.Tuple{core.OrgRoleTuple(inv.OrganizationID, identityID, inv.Role)}, nil); err != nil {
			log.Printf("tenancy: writing %s's membership of organization %s: %v", identityID, inv.OrganizationID, err)
		}
		core.RecordOrganizationEventAs(ctx, t.Audit, time.Now(), core.AuditVerbOrgInviteAccepted, inv.OrganizationID, identityID, methodSession,
			core.OrganizationAudit{Target: core.InviteTarget(inv.ID), TargetName: inv.Email, RoleTo: inv.Role})
	}
}
