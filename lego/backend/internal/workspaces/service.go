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

// Package workspaces is the workspace-lifecycle feature: create / rename /
// delete a workspace and list the caller's workspaces, projected as Render's
// "owner" shape. A workspace is a control-plane tenant row plus its
// tenant_members and its OpenFGA workspace:<id> tuples; this Service is the one
// place those three move together. The rest/graphql/mcp files are thin
// registration fragments over it (REST owners endpoints + MCP tools land in
// w6/m2; m1 ships the core verbs + the dashboard GraphQL).
package workspaces

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// nameRE constrains a workspace name to a DNS-1123 label capped at 30 chars.
// Unlike Render's freeform workspace names, a bex workspace name becomes part
// of every App CR name ("<workspace>-<app>", ≤63 chars), so it must be a DNS
// label — the same rule the internal tenant API enforces (store/api.go). The
// divergence from Render's freeform names is recorded as parity drift (w6/m1/t007).
var nameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$`)

// Service holds the workspace lifecycle logic once. It embeds *core.Base for the
// authorization gate + caller Identity, and writes through the Postgres source
// of truth (Store) with OpenFGA membership kept in lockstep (Granter/Revoker).
type Service struct {
	*core.Base
	// Store is the control-plane source of truth for workspaces. Nil (DB-less
	// mode / the authz sweep) leaves the verbs answering ErrWorkspacesUnavailable
	// after the Authorize gate — never a nil-deref.
	Store WorkspaceStore
	// Granter/Revoker keep OpenFGA membership in step with the tenant_members
	// rows: an owner tuple on create, its removal on delete. Both nil when authz
	// is disabled (the tuples don't exist to write) — the row is still the
	// source of truth, so lifecycle still works.
	Granter WorkspaceGranter
	Revoker WorkspaceRevoker
	// EnterpriseEntitlement is required for Enterprise selection. Nil means the
	// custom tier is unavailable; Pro and Scale remain self-service subject to
	// the normal billing gate.
	EnterpriseEntitlement EnterpriseEntitlement
	// Kick nudges the apps projector after a delete so the deleted workspace's
	// orphaned App CRs are pruned immediately instead of on the next resync. Nil
	// => pruning waits for the resync.
	Kick func()
	// PreCascadePurgers tear down a deleted workspace's out-of-cascade resources
	// BEFORE the tenant row (and its FK cascade) is dropped: OpenBao env-var
	// secrets, env groups, managed Databases/KeyValues, its OpenSandbox
	// sandboxes, and its Stripe subscription. Running before DeleteTenant is what
	// makes Delete retryable — a purger failure leaves the row (and the
	// confirmation phrase GetTenant re-reads) intact, so re-issuing the identical
	// Delete re-runs the whole teardown. Two of them also REQUIRE the pre-cascade
	// slot: the sandbox tenant key and the Stripe subscription id live in rows the
	// cascade is about to drop, and the secrets purger must enumerate the App CRs
	// while they still exist. Each is best-effort and idempotent; injected by the
	// composition root. See WorkspacePurger.
	PreCascadePurgers []WorkspacePurger
	// PostCascadePurgers tear down resources that must wait until AFTER the row
	// cascade: today just the App CRs, which the apps-rows→App-CR projector would
	// immediately re-create if they were purged while their rows still exist.
	// Their second net is the projector's own prune (Kick), so a post-cascade
	// purger failure — unlike a pre-cascade one — is still eventually reconciled
	// even though the Delete call itself can't be retried past the vanished row.
	PostCascadePurgers []WorkspacePurger
	// Identities looks up owner/member email + name + MFA from Kratos (the store
	// carries only subjects). Nil => those fields omitted from the owners/members
	// responses (honest subset; BEX_KRATOS_ADMIN_URL unset).
	Identities IdentityReader
	// KeyOwners resolves an API-key caller to the identity subject that minted
	// the key (w4/m25), so CurrentUser can answer with the owning human's
	// email/name through the same Identities lookup. Nil => machine callers keep
	// the earliest-admin-email fallback alone.
	KeyOwners KeyOwnerReader
}

// WorkspaceStore is the slice of the source of truth this feature writes
// through — narrow, like apps.IntentStore, so the service can't grow into a
// second store client and tests fake exactly these. *store.PGStore satisfies it.
type WorkspaceStore interface {
	CreateWorkspace(ctx context.Context, name, plan, ownerSubject string) (store.Tenant, error)
	GetTenant(ctx context.Context, id string) (store.Tenant, error)
	RenameTenant(ctx context.Context, id, name string) (store.Tenant, error)
	UpdateTenantPlan(ctx context.Context, id, plan string) (store.Tenant, error)
	DeleteTenant(ctx context.Context, id string) error
	ListTenantsForSubject(ctx context.Context, subject string) ([]store.Tenant, error)
	ListTenantMembers(ctx context.Context, tenantID string) ([]store.TenantMember, error)
	// ListInvites returns a workspace's OUTSTANDING invites (unaccepted,
	// unexpired) — the seats ChangePlan's downgrade guards must count alongside
	// the members, since each one becomes a member on its recipient's next login.
	ListInvites(ctx context.Context, tenantID string) ([]store.Invite, error)
	CountWorkspacesForSubjectPlan(ctx context.Context, subject, plan string) (int, error)
	// CountAppsForTenant counts a workspace's services (all of them, including
	// suspended) — ChangePlan's downgrade guard against the target plan's
	// MaxServices, the same count internal/store/api.go's create-time cap uses.
	CountAppsForTenant(ctx context.Context, tenantID string) (int, error)
	// OwnerIDForSubject returns the stable opaque "own-" id for a subject
	// (minted on first sight) — the Render userId the members surface reports
	// instead of the raw subject (w6/m7).
	OwnerIDForSubject(ctx context.Context, subject string) (string, error)
}

// WorkspaceGranter writes a subject's OpenFGA membership on a workspace (the
// authz write side of create). *authz.openfgaChecker satisfies it structurally,
// so this package keeps no dependency on the authz client.
type WorkspaceGranter interface {
	GrantWorkspaceAdmin(ctx context.Context, tenantID, subject string) error
}

// WorkspaceRevoker removes a subject's OpenFGA membership tuple (the authz side
// of delete). Kept separate from Granter so a checker can implement one without
// the other, and so tests can assert revokes independently.
type WorkspaceRevoker interface {
	RevokeWorkspaceMember(ctx context.Context, tenantID, subject, relation string) error
}

// EnterpriseEntitlement is the operator-controlled allow-list for the custom
// Enterprise workspace tier. Enterprise has no public catalog SKU or self-
// service checkout, so a missing provider must fail closed rather than turning
// a syntactically valid plan name into paid capability.
type EnterpriseEntitlement interface {
	CheckEnterpriseEntitlement(ctx context.Context, subject, workspaceID string) error
}

// TenantResolutionInvalidator evicts a subject's cached "current workspace"
// resolution (the api.tenantService cache behind core.Base's WorkspaceResolver)
// — optional and type-asserted from Base.Workspace, structurally like
// WorkspaceGranter/WorkspaceRevoker, since a store-off Base has no such cache to
// invalidate. Delete calls it for every member it revokes so a request racing
// the delete re-resolves instead of riding a stale positive TTL entry that
// still names the just-deleted tenant (core.PositiveTTL, up to 30s) — the
// window that made List (below) 403 immediately after a self-delete even
// though the caller still owned another workspace.
type TenantResolutionInvalidator interface {
	InvalidateTenant(subject string)
}

// WorkspacePurger tears down one class of a deleted workspace's resources that
// the tenant-row FK cascade does NOT reach — OpenBao env-var secrets, managed
// Databases/KeyValues, App CRs, OpenSandbox sandboxes, the Stripe subscription.
// Delete runs most of them BEFORE the cascade (see PreCascadePurgers) and the
// App-CR purger AFTER it (PostCascadePurgers); every implementation must be
// idempotent either way, so a retried delete completes the teardown. Injected
// by the composition root, so this feature imports none of those packages.
type WorkspacePurger interface {
	PurgeWorkspace(ctx context.Context, tenantID string) error
}

// ErrWorkspacesUnavailable lives in core (core.WriteErr maps it to 503, alongside
// the other "…Unavailable" sentinels); the workspace verbs return core.ErrWorkspacesUnavailable
// when the control-plane store isn't wired (bex-api running without BEX_CP_DB_URI).

// IdentityReader looks up a subject's identity attributes from the identity
// provider (Kratos admin API) — the email + name + MFA state the control-plane
// store doesn't carry but Render's owner/member objects require. Nil
// (BEX_KRATOS_ADMIN_URL unset) => attributes omitted (honest subset); the
// endpoints still answer 200.
type IdentityReader interface {
	Lookup(ctx context.Context, subject string) (IdentityAttrs, bool)
}

// IdentityAttrs are the IdP attributes a Render owner/member object needs that the
// store/CRs don't hold. ok=false from Lookup => the caller omits the field. Name
// is the optional Kratos `name` trait (w4/m25); "" when the identity never set it.
type IdentityAttrs struct {
	Email      string
	Name       string
	MFAEnabled bool
}

// KeyOwnerReader resolves a machine caller's API key (Hydra client id) to the
// identity subject that minted it — the key→identity binding stamped as the
// client's bex.co/created-by metadata (w4/m13), read back here so a machine
// caller's GET /v1/users can report its owning human's email/name (w4/m25).
// ok=false for an unknown/non-API-key client or a key minted before the stamp
// existed. Nil => machine callers fall back to the earliest-admin email alone.
type KeyOwnerReader interface {
	KeyOwner(ctx context.Context, clientID string) (subject string, ok bool)
}

// WorkspaceView is the neutral projection of a workspace — the tenant row plus
// the caller's role in it. Each adapter maps this to its wire format (m2's REST
// renders it in Render's owner shape; the GraphQL fragment renders it directly).
type WorkspaceView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Plan      string `json:"plan"`
	Role      string `json:"role,omitempty"`
	CreatedAt string `json:"createdAt"`
}

func view(t store.Tenant, role string) WorkspaceView {
	created := ""
	if !t.CreatedAt.IsZero() {
		created = t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return WorkspaceView{ID: t.ID, Name: t.Name, Plan: t.Plan, Role: role, CreatedAt: created}
}

// List returns the caller's workspaces (the dashboard switcher / owners list).
// The query is membership-scoped to the caller's subject, so this returns only
// workspaces they belong to regardless of the coarse can_view gate.
func (s *Service) List(ctx context.Context) ([]WorkspaceView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrWorkspacesUnavailable
	}
	id, _ := core.IdentityFrom(ctx)
	ts, err := s.Store.ListTenantsForSubject(ctx, id.Subject)
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceView, 0, len(ts))
	for _, t := range ts {
		// Role is left to m2's members endpoint to fill richly; the caller is at
		// least a member (the query guarantees it), and for m1 every membership is
		// admin (create is the only writer).
		out = append(out, view(t, "admin"))
	}
	return out, nil
}

// ownIDPrefix is Render's user-id prefix (docs/render-artifacts/owners-api.md:
// "Workspace IDs start with tea-. If you provide a user ID (starts with own-),
// this endpoint returns the user's default workspace."). bex mints no own- id
// registry — Identity.Subject is a raw Kratos/Hydra string, not a typed id — so
// there is no lookup from an arbitrary own- id to SOME other user's default
// workspace; any own--prefixed ownerId is read as "the caller's own id" and
// resolves to the CALLER's default workspace. Documented parity simplification
// (w6/m2/t006).
const ownIDPrefix = "own-"

// GetWorkspace retrieves one workspace by id — GET /v1/owners/{ownerId}. An
// own--prefixed id resolves to the caller's default workspace (see ownIDPrefix);
// otherwise it authorizes can_view on the exact workspace, so a workspace the
// caller isn't a member of is ErrForbidden (adapters render 403/404) and an
// unknown tea- id is ErrNotFound.
func (s *Service) GetWorkspace(ctx context.Context, ownerID string) (WorkspaceView, error) {
	if strings.HasPrefix(ownerID, ownIDPrefix) {
		return s.defaultWorkspace(ctx)
	}
	if err := s.AuthorizeOn(ctx, core.RelCanView, core.WorkspaceObject(ownerID)); err != nil {
		return WorkspaceView{}, err
	}
	if s.Store == nil {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	t, err := s.Store.GetTenant(ctx, ownerID)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	return view(t, "admin"), nil
}

// defaultWorkspace returns the caller's resolved tenant via core.Base.Tenant —
// the SAME resolver (w1/m9's core.WorkspaceResolver) that apps/postgres List
// use to auto-scope, so an own- id and "my unscoped resource list" always
// agree on what "my workspace" means. ok=false (no resolver wired, or the
// caller has none) is ErrNotFound — nothing to fall back to.
func (s *Service) defaultWorkspace(ctx context.Context) (WorkspaceView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return WorkspaceView{}, err
	}
	if s.Store == nil {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		return WorkspaceView{}, core.ErrNotFound
	}
	t, err := s.Store.GetTenant(ctx, tenantID)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	return view(t, "admin"), nil
}

// UserView is the caller's own account info — GET /v1/users
// (components.schemas.user: {email, name}).
type UserView struct {
	Email string
	Name  string
}

// CurrentUser answers GET /v1/users, Render's "who am I" endpoint (used by e.g.
// the official Render CLI's `render whoami`). A session caller's email + name
// come straight off their Kratos identity (already resolved onto core.Identity
// by the auth gate). A machine (API-key/OAuth) caller carries no traits of its
// own: a user-consented OAuth token's subject is a Kratos identity id, looked
// up directly, while a client_credentials (API-key) token — recognized by the
// gate's ClientID stamp, no probing — resolves through the key's created-by
// binding (KeyOwners, w4/m25) to the human who minted it. A key with no
// resolvable owning human (minted before the created-by stamp, or a
// service-account-style key) degrades to the bound workspace's earliest-admin
// email with no name — the documented honest subset. No GraphQL/MCP
// equivalent: the dashboard authenticates with its Kratos session directly
// rather than this REST-only endpoint, and no MCP tool needs "who am I".
func (s *Service) CurrentUser(ctx context.Context) (UserView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return UserView{}, err
	}
	id, _ := core.IdentityFrom(ctx)
	u := UserView{Email: id.Email, Name: id.Name}
	if u.Email == "" && s.Identities != nil {
		subject := id.Subject
		if id.Subject == id.ClientID {
			// The caller IS an API key — its subject can never be in Kratos.
			subject = ""
			if s.KeyOwners != nil {
				subject, _ = s.KeyOwners.KeyOwner(ctx, id.Subject)
			}
		}
		if subject != "" {
			if attrs, ok := s.Identities.Lookup(ctx, subject); ok {
				u.Email, u.Name = attrs.Email, attrs.Name
			}
		}
	}
	if u.Email == "" {
		if tenantID, ok := s.Tenant(ctx); ok {
			u.Email = s.ownerEmail(ctx, tenantID)
		}
	}
	return u, nil
}

// OwnerFilter narrows ListOwners (GET /v1/owners): Names/Emails are OR'd within
// each field, AND'd across fields (Render's docs: "one of the provided names" /
// "one of the provided email addresses"). Both empty => no filtering.
type OwnerFilter struct {
	Names  []string
	Emails []string
}

// OwnerView is a workspace plus the contact email Render's owner object
// requires — kept separate from WorkspaceView so the dashboard switcher
// (GraphQL `workspaces`) and the lifecycle mutations don't pay for the extra
// Identities lookup they don't need.
type OwnerView struct {
	WorkspaceView
	Email string
}

// ListOwners is List (the caller's workspaces) plus t001's REST-only name/email
// filters and the per-workspace contact email — GET /v1/owners.
func (s *Service) ListOwners(ctx context.Context, f OwnerFilter) ([]OwnerView, error) {
	ws, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]OwnerView, 0, len(ws))
	for _, w := range ws {
		if len(f.Names) > 0 && !slices.Contains(f.Names, w.Name) {
			continue
		}
		email := s.ownerEmail(ctx, w.ID)
		if len(f.Emails) > 0 && !slices.Contains(f.Emails, email) {
			continue
		}
		out = append(out, OwnerView{WorkspaceView: w, Email: email})
	}
	return out, nil
}

// ResolveResourceOwners implements resourcemeta.OwnerResolver for the three
// resource REST adapters. One membership-scoped workspace query resolves every
// unique id in a list response; an id absent from that result is omitted, so a
// resource adapter can never use this seam to reveal another workspace. Email
// remains best-effort and is looked up once per unique workspace, not once per
// resource.
func (s *Service) ResolveResourceOwners(ctx context.Context, ownerIDs []string) map[string]resourcemeta.Owner {
	if s.Store == nil || len(ownerIDs) == 0 {
		return nil
	}
	identity, ok := core.IdentityFrom(ctx)
	if !ok || identity.Subject == "" {
		return nil
	}
	wanted := make(map[string]struct{}, len(ownerIDs))
	for _, id := range ownerIDs {
		if id != "" {
			wanted[id] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	tenants, err := s.Store.ListTenantsForSubject(ctx, identity.Subject)
	if err != nil {
		return nil
	}
	out := make(map[string]resourcemeta.Owner, len(wanted))
	for _, tenant := range tenants {
		if _, ok := wanted[tenant.ID]; !ok {
			continue
		}
		out[tenant.ID] = resourcemeta.Owner{
			ID: tenant.ID, Name: tenant.Name, Email: s.ownerEmail(ctx, tenant.ID), Type: "team",
		}
	}
	return out
}

// ownerEmail resolves the workspace's contact email — the earliest-admin
// member's email via Identities. Best-effort: "" when Identities is nil, the
// membership list can't be read, or the lookup misses (honest subset, matching
// IdentityReader's documented omit-on-miss contract).
func (s *Service) ownerEmail(ctx context.Context, tenantID string) string {
	if s.Identities == nil || s.Store == nil {
		return ""
	}
	members, err := s.Store.ListTenantMembers(ctx, tenantID)
	if err != nil {
		return ""
	}
	for _, m := range members { // oldest first
		if m.Role != "admin" {
			continue
		}
		if attrs, ok := s.Identities.Lookup(ctx, m.Subject); ok {
			return attrs.Email
		}
		return ""
	}
	return ""
}

// MemberView is a workspace member with the identity attributes Render's
// teamMember object needs, layered onto the tenant_members row. OwnerID is the
// member's opaque "own-" id — the Render userId (w6/m7); Subject is the raw
// OpenFGA subject, kept for internal use (revoke/email lookup), never emitted.
type MemberView struct {
	Subject    string
	OwnerID    string
	Role       string
	Email      string
	Name       string
	MFAEnabled bool
}

// ListMembers lists a workspace's members — GET /v1/owners/{ownerId}/members.
// Any member may list members (every m1/m2 role includes can_view). An unknown
// workspace id is ErrNotFound; a workspace the caller isn't a member of is
// ErrForbidden.
func (s *Service) ListMembers(ctx context.Context, ownerID string) ([]MemberView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanView, core.WorkspaceObject(ownerID)); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrWorkspacesUnavailable
	}
	// Confirm the workspace exists — a foreign/unknown id is ErrNotFound, not a
	// silently empty member list.
	if _, err := s.Store.GetTenant(ctx, ownerID); err != nil {
		return nil, mapStoreErr(err)
	}
	rows, err := s.Store.ListTenantMembers(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberView, 0, len(rows))
	for _, m := range rows {
		mv := MemberView{Subject: m.Subject, Role: m.Role}
		// Resolve the opaque own- id (minted on first sight) — the Render userId.
		ownID, err := s.Store.OwnerIDForSubject(ctx, m.Subject)
		if err != nil {
			return nil, err
		}
		mv.OwnerID = ownID
		if s.Identities != nil {
			if attrs, ok := s.Identities.Lookup(ctx, m.Subject); ok {
				mv.Email, mv.Name, mv.MFAEnabled = attrs.Email, attrs.Name, attrs.MFAEnabled
			}
		}
		out = append(out, mv)
	}
	return out, nil
}

// Create makes a new workspace owned by the caller: a tenant row + the caller's
// admin membership (atomic in the store), then the matching OpenFGA admin tuple.
// It refuses a plan's per-user workspace cap (Render: five Hobby workspaces per
// user). Authorization mirrors every other create verb — can_create on the
// default workspace — until w1/m9 grows real per-caller workspace scoping.
func (s *Service) Create(ctx context.Context, name, plan string) (WorkspaceView, error) {
	// Creating a workspace creates a durable administrator authority. A
	// client_credentials API key may be a developer in its existing workspace,
	// but it must not promote itself to the administrator of a new authority
	// domain. Keep this class check ahead of every write, just like API-key and
	// SSH-key minting.
	if err := s.AuthorizeMintClass(ctx); err != nil {
		return WorkspaceView{}, err
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return WorkspaceView{}, err
	}
	if s.Store == nil {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok || id.Subject == "" {
		return WorkspaceView{}, core.ErrForbidden
	}
	if !nameRE.MatchString(name) {
		return WorkspaceView{}, fmt.Errorf("%w: name must be a DNS label of 1-30 chars ([a-z0-9-])", core.ErrBadRequest)
	}
	plan, err := normalizePlan(plan)
	if err != nil {
		return WorkspaceView{}, err
	}
	if err := s.requirePlanEntitlement(ctx, id.Subject, "", plan); err != nil {
		return WorkspaceView{}, err
	}
	if workspacePaidPlan(plan) && (s.Payment != nil || s.Billing != nil) {
		// The new tenant has no billing marker yet. Use the caller's existing
		// workspace as the account-level payment/dunning checkpoint; onboarding
		// creates that Hobby workspace before this mutation is reachable in the
		// production API. If no checkpoint exists, fail closed rather than
		// persisting an authoritative paid plan without proof of eligibility.
		billingWorkspace, ok := s.Tenant(ctx)
		if !ok || billingWorkspace == "" {
			return WorkspaceView{}, core.NewPaymentRequiredError()
		}
		if err := s.RequirePlanBilling(ctx, billingWorkspace, plan); err != nil {
			return WorkspaceView{}, err
		}
	}
	// Per-user plan cap (Render allows five free Hobby workspaces, unlimited
	// paid). Checked before the write; a race past it is bounded and benign
	// (worst case one extra workspace), and the read-modify-write isn't worth a
	// lock here.
	if err := s.guardPerUserWorkspaceCap(ctx, id.Subject, plan); err != nil {
		return WorkspaceView{}, err
	}
	t, err := s.Store.CreateWorkspace(ctx, name, plan, id.Subject)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	// Grow the OpenFGA workspace: the owner becomes admin on workspace:<id>.
	// There's no distributed transaction across Postgres and OpenFGA, so on a
	// grant failure compensate by rolling the row back — a workspace nobody can
	// administer is worse than a reported error the caller can retry. The
	// best-effort delete leaves no partial state in the common failure case.
	if s.Granter != nil {
		if err := s.Granter.GrantWorkspaceAdmin(ctx, t.ID, "user:"+id.Subject); err != nil {
			_ = s.Store.DeleteTenant(ctx, t.ID)
			return WorkspaceView{}, fmt.Errorf("workspace %s: granting admin failed: %w", t.ID, err)
		}
	}
	return view(t, "admin"), nil
}

// Rename changes a workspace's display name. Admin-only (can_manage on the exact
// workspace). The id stays the key, so a rename breaks no references.
func (s *Service) Rename(ctx context.Context, id, name string) (WorkspaceView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(id)); err != nil {
		return WorkspaceView{}, err
	}
	if s.Store == nil {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	if !nameRE.MatchString(name) {
		return WorkspaceView{}, fmt.Errorf("%w: name must be a DNS label of 1-30 chars ([a-z0-9-])", core.ErrBadRequest)
	}
	t, err := s.Store.RenameTenant(ctx, id, name)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	return view(t, "admin"), nil
}

// ChangePlan upgrades or downgrades a workspace's plan (w6/m12,
// docs/render-artifacts/workspace-plan-change.md). Admin-only (can_manage). A
// no-op when the plan is unchanged, mirroring Render's submit-disabled-until-
// different UX. A downgrade is refused when the workspace would violate the
// target plan's caps — member count vs MaxMembers, service count vs
// MaxServices, the caller's own per-plan workspace count vs
// MaxWorkspacesPerUser (e.g. a 6th Hobby workspace via downgrade, the same cap
// Create enforces on creation) — or when any member holds a role the target
// plan's AllowedRoles no longer offers (t004); each guard names the exact reason.
//
// The member-count and role guards count the workspace's OUTSTANDING INVITES as
// well as its members (w6/m15/t003): a pending invite is a member-in-waiting —
// store.AcceptInvitesForEmail enforces the workspace's plan at accept time, so a
// downgrade that ignored them would leave those invites silently un-redeemable
// (the invitee logs in and simply doesn't join) until someone upgrades again.
// Refusing here instead means the admin learns what to revoke while they still
// remember sending it.
func (s *Service) ChangePlan(ctx context.Context, id, plan string) (WorkspaceView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(id)); err != nil {
		return WorkspaceView{}, err
	}
	if s.Store == nil {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	plan, err := normalizePlan(plan)
	if err != nil {
		return WorkspaceView{}, err
	}
	t, err := s.Store.GetTenant(ctx, id)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	if t.Plan == plan {
		return view(t, "admin"), nil // already there — nothing to write
	}
	identity, _ := core.IdentityFrom(ctx)
	if err := s.requirePlanEntitlement(ctx, identity.Subject, id, plan); err != nil {
		return WorkspaceView{}, err
	}
	// A paid upward transition is a billable intent and must be checked before
	// any authoritative plan write. Downgrades remain available during dunning
	// so an administrator can recover the workspace; the normal resource and
	// role-capacity checks below still protect the target plan.
	if workspacePaidPlan(plan) {
		if err := s.RequirePlanBilling(ctx, id, plan); err != nil {
			return WorkspaceView{}, err
		}
	}
	members, err := s.Store.ListTenantMembers(ctx, id)
	if err != nil {
		return WorkspaceView{}, err
	}
	invites, err := s.Store.ListInvites(ctx, id)
	if err != nil {
		return WorkspaceView{}, err
	}
	limits := store.LimitsFor(plan)
	if limits.MaxMembers > 0 && len(members)+len(invites) > limits.MaxMembers {
		if len(invites) == 0 {
			return WorkspaceView{}, fmt.Errorf("%w: workspace has %d member(s), exceeds %s plan's limit of %d",
				core.ErrBadRequest, len(members), plan, limits.MaxMembers)
		}
		// An invite is a seat already promised, so it counts — and naming the
		// invitees is the difference between "you're over the cap" and "revoke these".
		return WorkspaceView{}, fmt.Errorf(
			"%w: workspace has %d member(s) + %d pending invite(s) (%s), exceeds %s plan's limit of %d; revoke the invite(s) first",
			core.ErrBadRequest, len(members), len(invites), strings.Join(inviteEmails(invites), ", "),
			plan, limits.MaxMembers)
	}
	if limits.MaxServices > 0 {
		n, err := s.Store.CountAppsForTenant(ctx, id)
		if err != nil {
			return WorkspaceView{}, err
		}
		if n > limits.MaxServices {
			return WorkspaceView{}, fmt.Errorf("%w: workspace has %d services, exceeds %s plan's limit of %d",
				core.ErrBadRequest, n, plan, limits.MaxServices)
		}
	}
	if caller, ok := core.IdentityFrom(ctx); ok {
		if err := s.guardPerUserWorkspaceCap(ctx, caller.Subject, plan); err != nil {
			return WorkspaceView{}, err
		}
	}
	if blocked := rolesOutsidePlan(members, plan); len(blocked) > 0 {
		return WorkspaceView{}, fmt.Errorf("%w: workspace has members with roles not allowed on %s (%s); downgrade first",
			core.ErrBadRequest, plan, strings.Join(blocked, ", "))
	}
	if blocked := invitedRolesOutsidePlan(invites, plan); len(blocked) > 0 {
		return WorkspaceView{}, fmt.Errorf("%w: workspace has pending invites with roles not allowed on %s (%s); revoke them first",
			core.ErrBadRequest, plan, strings.Join(blocked, ", "))
	}
	nt, err := s.Store.UpdateTenantPlan(ctx, id, plan)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	return view(nt, "admin"), nil
}

// workspacePaidPlan uses the workspace catalog's free tier. Other resource
// families use "free" or their own tier ids, so core.PaidPlan deliberately
// remains neutral to those catalogs.
func workspacePaidPlan(plan string) bool {
	return plan != "" && plan != store.PlanHobby
}

func (s *Service) requirePlanEntitlement(ctx context.Context, subject, workspaceID, plan string) error {
	if plan != store.PlanEnterprise {
		return nil
	}
	if s.EnterpriseEntitlement == nil {
		return core.NewBadRequestError(
			"ENTERPRISE_ENTITLEMENT_REQUIRED",
			"Enterprise workspace plans require explicit operator approval",
			nil,
		)
	}
	return s.EnterpriseEntitlement.CheckEnterpriseEntitlement(ctx, subject, workspaceID)
}

// rolesOutsidePlan returns the "subject:role" pairs that wouldn't be
// assignable on plan — ChangePlan's role-downgrade guard (t004): a downgrade
// must not leave a member holding a role the new plan no longer offers.
func rolesOutsidePlan(members []store.TenantMember, plan string) []string {
	var blocked []string
	for _, m := range members {
		if !store.RoleAllowedOnPlan(plan, m.Role) {
			blocked = append(blocked, m.Subject+":"+m.Role)
		}
	}
	return blocked
}

// invitedRolesOutsidePlan is rolesOutsidePlan for the seats not taken yet
// (w6/m15/t003): the "email:role" pairs of pending invites whose role the
// target plan doesn't offer. Such an invite would be refused at accept time by
// store.AcceptInvitesForEmail's own plan check, so the downgrade would quietly
// strand it — name it here instead. Invites are keyed by email (they have no
// subject until redeemed), which is also the identity the admin revokes by.
func invitedRolesOutsidePlan(invites []store.Invite, plan string) []string {
	var blocked []string
	for _, inv := range invites {
		if !store.RoleAllowedOnPlan(plan, inv.Role) {
			blocked = append(blocked, inv.Email+":"+inv.Role)
		}
	}
	return blocked
}

// inviteEmails names the invitees a refusal is about — the identity an admin
// revokes an invite by (an invite has no subject until it's redeemed).
func inviteEmails(invites []store.Invite) []string {
	emails := make([]string, 0, len(invites))
	for _, inv := range invites {
		emails = append(emails, inv.Email)
	}
	return emails
}

// guardPerUserWorkspaceCap refuses a plan (Create's target, or ChangePlan's)
// that would put subject over Render's per-user workspace cap for that plan
// (Hobby: five free workspaces/user; paid plans unlimited) — the one rule
// both verbs enforce, Create on a new row and ChangePlan on a downgrade into it.
func (s *Service) guardPerUserWorkspaceCap(ctx context.Context, subject, plan string) error {
	limit := store.LimitsFor(plan).MaxWorkspacesPerUser
	if limit == 0 {
		return nil
	}
	n, err := s.Store.CountWorkspacesForSubjectPlan(ctx, subject, plan)
	if err != nil {
		return err
	}
	if n >= limit {
		return fmt.Errorf("%w: at most %d %s workspaces per user", core.ErrBadRequest, limit, plan)
	}
	return nil
}

// DeleteConfirmation is the exact phrase the caller must type to arm a
// workspace delete, cloning Render's live dashboard guard verbatim
// (docs/render-artifacts/workspace-lifecycle.md, captured 2026-07-11): the
// literal "sudo delete workspace " prefixed to the workspace's own name, not
// the bare name. Single source of truth so the service guard, the GraphQL arg
// docs, and the dashboard danger-zone (delete-workspace-card.tsx) agree.
func DeleteConfirmation(name string) string { return "sudo delete workspace " + name }

// Delete tears a workspace down, in an order chosen so a transient failure is
// recoverable: it revokes each member's OpenFGA tuple, runs the pre-cascade
// purgers (OpenBao secrets, env groups, managed Databases/KeyValues, OpenSandbox
// sandboxes, the Stripe subscription) while the tenant row still exists, deletes
// the tenant row (the FK cascade drops its apps, domains, memberships, sandbox
// key, and billing rows in the same statement), nudges the projector to prune
// the orphaned App CRs, then runs the post-cascade purger (the App CRs). Running
// the bulk of the teardown BEFORE the row is dropped is what makes it retryable:
// a pre-cascade purger failure leaves the row (and its confirmation phrase)
// intact, so re-issuing the identical Delete re-runs the whole sweep. Admin-only,
// and guarded by a confirmation that must equal DeleteConfirmation(name) —
// Render's "sudo delete workspace <name>" phrase (a typo is a no-op, not a
// destroyed workspace). Every purger and the revoke tolerate already-gone
// tuples/resources so the retry converges.
func (s *Service) Delete(ctx context.Context, id, confirmName string) error {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(id)); err != nil {
		return err
	}
	// codex-security round-6 #16: destroying a workspace is the most
	// destructive verb the API has, so re-assert can_manage against the source
	// of truth (uncached) after the audited check — a just-revoked admin must
	// not ride a ≤PositiveTTL cached positive into it. Same pattern as the
	// membership issuance verbs (round-5 finding 4).
	if err := s.AuthorizeFreshOn(ctx, core.RelCanManage, core.WorkspaceObject(id)); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrWorkspacesUnavailable
	}
	t, err := s.Store.GetTenant(ctx, id)
	if err != nil {
		return mapStoreErr(err)
	}
	if want := DeleteConfirmation(t.Name); confirmName != want {
		return fmt.Errorf("%w: confirmation must be %q", core.ErrBadRequest, want)
	}
	return s.deleteWorkspace(ctx, id)
}

// AccountTeardown is the trusted account-lifecycle adapter over the same
// workspace teardown used by the public Delete verb. It intentionally is not a
// Service verb: only the accounts worker receives it from the composition root,
// and public callers must still pass Delete's authorization and confirmation.
type AccountTeardown struct{ Service *Service }

func (a AccountTeardown) Delete(ctx context.Context, id string) error {
	if a.Service == nil || a.Service.Store == nil {
		return core.ErrWorkspacesUnavailable
	}
	err := a.Service.deleteWorkspace(ctx, id)
	if errors.Is(err, core.ErrNotFound) {
		return nil
	}
	return err
}

func (s *Service) deleteWorkspace(ctx context.Context, id string) error {
	// Re-read so a background retry that starts after a prior cascade converges
	// as not-found at AccountTeardown's boundary.
	if _, err := s.Store.GetTenant(ctx, id); err != nil {
		return mapStoreErr(err)
	}
	// Revoke authz tuples before dropping the rows: the tenant_members rows name
	// exactly the subjects to revoke, and reading them after the cascade would be
	// too late. Every m1 membership is admin (create is the only writer); the
	// member's role is carried through so w4/m12's future roles revoke cleanly.
	if s.Revoker != nil {
		members, err := s.Store.ListTenantMembers(ctx, id)
		if err != nil {
			return err
		}
		inv, _ := s.Workspace.(TenantResolutionInvalidator)
		for _, m := range members {
			if err := s.Revoker.RevokeWorkspaceMember(ctx, id, "user:"+m.Subject, m.Role); err != nil {
				return fmt.Errorf("revoke %s from workspace %s: %w", m.Subject, id, err)
			}
			if inv != nil {
				inv.InvalidateTenant(m.Subject)
			}
		}
	}
	// Pre-cascade teardown: everything except the App CRs runs while the tenant
	// row still exists. That is what makes Delete retryable — a failure here
	// leaves the row (and the confirmation phrase GetTenant re-reads) intact, so
	// re-issuing the identical Delete re-runs the whole sweep — and it is what
	// lets the sandbox/Stripe purgers still read the ids the cascade is about to
	// drop and the secrets purger still enumerate the workspace's App CRs.
	if err := runPurgers(ctx, s.PreCascadePurgers, id); err != nil {
		return err
	}
	if err := s.Store.DeleteTenant(ctx, id); err != nil {
		return mapStoreErr(err)
	}
	if s.Kick != nil {
		s.Kick()
	}
	// Post-cascade teardown: the App CRs, now that their backing rows are gone so
	// the projector won't resurrect them. Best-effort — the projector's own prune
	// (Kick, above) is the second net if this fails, and the row is already gone
	// so the workspace is unusable regardless.
	if err := runPurgers(ctx, s.PostCascadePurgers, id); err != nil {
		return err
	}
	return nil
}

// runPurgers invokes each purger in order, wrapping the first failure so Delete
// surfaces it (a pre-cascade failure then leaves the row intact for a retry).
func runPurgers(ctx context.Context, purgers []WorkspacePurger, id string) error {
	for _, p := range purgers {
		if err := p.PurgeWorkspace(ctx, id); err != nil {
			return fmt.Errorf("purge workspace %s: %w", id, err)
		}
	}
	return nil
}

// mapStoreErr translates the store's error taxonomy into the kernel sentinels the
// adapters map to status codes (the store's ErrInvalid/NotFound/Conflict are
// package-private to store's HTTP surface; features speak core's).
func mapStoreErr(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return fmt.Errorf("%w: %v", core.ErrNotFound, err)
	case errors.Is(err, store.ErrConflict):
		return fmt.Errorf("%w: %v", core.ErrBadRequest, err)
	case errors.Is(err, store.ErrInvalid):
		return fmt.Errorf("%w: %v", core.ErrBadRequest, err)
	default:
		return err
	}
}

// normalizePlan validates a workspace plan against the store catalog, mapping its
// ErrInvalid to the kernel's ErrBadRequest.
func normalizePlan(plan string) (string, error) {
	p, err := store.NormalizePlan(plan)
	if err != nil {
		return "", fmt.Errorf("%w: %v", core.ErrBadRequest, err)
	}
	return p, nil
}

// ResourceCapView is one resource slot in a workspace's limit report (w7/m9).
// Used is every CR the workspace owns (what the ResourceQuota gates a create on);
// Terminating is the subset of Used finishing deletion, which the resource list
// drops but whose quota it still holds. See ResourceLimits for why the two are
// reported together rather than filtered (w6/m129).
type ResourceCapView struct {
	Used        int `json:"used"`
	Terminating int `json:"terminating"`
	Limit       int `json:"limit"` // 0 = unlimited
}

// ResourceLimitsView is the per-workspace resource usage vs. cap (w7/m9):
// how many of each resource kind the workspace currently owns (Used) and the
// maximum it may create (Limit; 0 = unlimited). Returned by ResourceLimits.
type ResourceLimitsView struct {
	Services  ResourceCapView `json:"services"`
	Postgres  ResourceCapView `json:"postgres"`
	KeyValues ResourceCapView `json:"keyValues"`
}

// ResourceLimits returns the workspace's current resource usage vs. its
// plan's caps (w7/m9) — the "3/5 services" visibility surface. It authorizes
// can_view on the named workspace (same gate as GetWorkspace), then counts
// App/Database/KeyValue CRs labelled with the workspace's tenant id. Limits
// come from store.QuotaCapsForPlan(ws.Plan) — the same per-workspace object
// counts the per-namespace ResourceQuota enforces at the API server (ADR043
// D3; the app-code MaxServices/MaxPostgres/MaxKeyValues caps this used to
// read were retired in w3/m34). When the k8s client is nil (authz-sweep /
// store-off tests), counts are zero but limits are still returned — the call
// never 500s.
//
// Used counts EVERY CR, including ones mid-deletion, because a k8s object-count
// ResourceQuota holds quota until an object's finalizers clear and it leaves
// etcd — so a still-terminating CR genuinely consumes the cap, and Used is the
// truthful report of what a create will be gated against (w6/m129). We do NOT
// filter terminating CRs out of Used: that would make the number smaller than
// enforcement and refuse a create at "6/100" with no explanation. Instead we
// report Terminating alongside Used so the usage surface stays reconcilable with
// the resource list (which drops terminating rows, apps.Service.List / w3/m46):
// the list shows Used - Terminating, and the difference is the quota held by
// deletes still finishing. The rejected alternative — replacing the ResourceQuota
// with a mechanism that ignores terminating objects — is infeasible: k8s cannot
// be told to discount an object it still holds in etcd.
func (s *Service) ResourceLimits(ctx context.Context, ownerID string) (ResourceLimitsView, error) {
	ws, err := s.GetWorkspace(ctx, ownerID)
	if err != nil {
		return ResourceLimitsView{}, err
	}
	tenantID := ws.ID

	caps := store.QuotaCapsForPlan(ws.Plan)
	out := ResourceLimitsView{
		Services:  ResourceCapView{Limit: int(caps.Services)},
		Postgres:  ResourceCapView{Limit: int(caps.Postgres)},
		KeyValues: ResourceCapView{Limit: int(caps.KeyValues)},
	}
	if s.Client == nil {
		return out, nil
	}
	var apps appv1alpha1.AppList
	if listErr := s.ListByTenant(ctx, &apps, tenantID); listErr != nil {
		return ResourceLimitsView{}, fmt.Errorf("counting services: %w", listErr)
	}
	var dbs appv1alpha1.DatabaseList
	if listErr := s.ListByTenant(ctx, &dbs, tenantID); listErr != nil {
		return ResourceLimitsView{}, fmt.Errorf("counting databases: %w", listErr)
	}
	var kvs appv1alpha1.KeyValueList
	if listErr := s.ListByTenant(ctx, &kvs, tenantID); listErr != nil {
		return ResourceLimitsView{}, fmt.Errorf("counting key-values: %w", listErr)
	}
	out.Services.Used, out.Services.Terminating = usageCount(apps.Items)
	out.Postgres.Used, out.Postgres.Terminating = usageCount(dbs.Items)
	out.KeyValues.Used, out.KeyValues.Terminating = usageCount(kvs.Items)
	return out, nil
}

// deletable is satisfied by *App / *Database / *KeyValue: the pointer carries
// metav1.Object (GetDeletionTimestamp et al.) promoted from the embedded
// ObjectMeta, whereas a value T does not — which is why the constraint is on *T.
type deletable[T any] interface {
	*T
	metav1.Object
}

// usageCount reports total CRs (used) and how many are finishing deletion
// (terminating — a non-zero DeletionTimestamp), one rule for all three resource
// kinds so services, Postgres and Key Value cannot drift apart. See ResourceLimits
// for why terminating CRs count toward used.
func usageCount[T any, PT deletable[T]](items []T) (used, terminating int) {
	used = len(items)
	for i := range items {
		if !PT(&items[i]).GetDeletionTimestamp().IsZero() {
			terminating++
		}
	}
	return used, terminating
}
