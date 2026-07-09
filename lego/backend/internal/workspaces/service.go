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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
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
	// Kick nudges the apps projector after a delete so the deleted workspace's
	// orphaned App CRs are pruned immediately instead of on the next resync. Nil
	// => pruning waits for the resync.
	Kick func()
	// Purgers tear down a deleted workspace's out-of-cascade resources (its
	// OpenBao env-var secrets, its managed Databases). Each is best-effort and
	// injected by the composition root; see WorkspacePurger.
	Purgers []WorkspacePurger
	// Identities looks up owner/member email + MFA from Kratos (the store carries
	// only subjects). Nil => those fields omitted from the owners/members responses
	// (honest subset; BEX_KRATOS_ADMIN_URL unset).
	Identities IdentityReader
	// Selections is the shared MCP per-session workspace selection (w6/m2/t005):
	// select_workspace writes it, get_selected_workspace reads it, and the apps/
	// postgres list tools read it as their default ownerId filter. Nil (should
	// not happen in practice — the composition root always wires one) leaves the
	// MCP workspace tools unregistered rather than panicking; see RegisterMCP.
	Selections *core.WorkspaceSelections
}

// WorkspaceStore is the slice of the source of truth this feature writes
// through — narrow, like apps.IntentStore, so the service can't grow into a
// second store client and tests fake exactly these. *store.PGStore satisfies it.
type WorkspaceStore interface {
	CreateWorkspace(ctx context.Context, name, plan, ownerSubject string) (store.Tenant, error)
	GetTenant(ctx context.Context, id string) (store.Tenant, error)
	RenameTenant(ctx context.Context, id, name string) (store.Tenant, error)
	DeleteTenant(ctx context.Context, id string) error
	ListTenantsForSubject(ctx context.Context, subject string) ([]store.Tenant, error)
	ListTenantMembers(ctx context.Context, tenantID string) ([]store.TenantMember, error)
	CountWorkspacesForSubjectPlan(ctx context.Context, subject, plan string) (int, error)
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

// WorkspacePurger tears down one class of a deleted workspace's resources that
// the tenant-row FK cascade does NOT reach — its OpenBao env-var secrets, its
// managed Databases. Purge is called after the row (and its cascade) is gone;
// it must be idempotent so a retried delete completes the teardown. Injected by
// the composition root, so this feature imports neither secrets nor postgres.
type WorkspacePurger interface {
	PurgeWorkspace(ctx context.Context, tenantID string) error
}

// ErrWorkspacesUnavailable lives in core (core.WriteErr maps it to 503, alongside
// the other "…Unavailable" sentinels); the workspace verbs return core.ErrWorkspacesUnavailable
// when the control-plane store isn't wired (bex-api running without BEX_CP_DB_URI).

// IdentityReader looks up a subject's identity attributes from the identity
// provider (Kratos admin API) — the email + MFA state the control-plane store
// doesn't carry but Render's owner/member objects require. Nil (BEX_KRATOS_ADMIN_URL
// unset) => attributes omitted (honest subset); the endpoints still answer 200.
// bex's Kratos schema defines only an email trait (no name) — see
// docs/render-artifacts/owners-api.md — so IdentityAttrs carries no Name.
type IdentityReader interface {
	Lookup(ctx context.Context, subject string) (IdentityAttrs, bool)
}

// IdentityAttrs are the IdP attributes a Render owner/member object needs that the
// store/CRs don't hold. ok=false from Lookup => the caller omits the field.
type IdentityAttrs struct {
	Email      string
	MFAEnabled bool
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
// teamMember object needs, layered onto the tenant_members row.
type MemberView struct {
	Subject    string
	Role       string
	Email      string
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
		if s.Identities != nil {
			if attrs, ok := s.Identities.Lookup(ctx, m.Subject); ok {
				mv.Email, mv.MFAEnabled = attrs.Email, attrs.MFAEnabled
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
	// Per-user plan cap (Render allows five free Hobby workspaces, unlimited
	// paid). Checked before the write; a race past it is bounded and benign
	// (worst case one extra workspace), and the read-modify-write isn't worth a
	// lock here.
	if limit := store.LimitsFor(plan).MaxWorkspacesPerUser; limit > 0 {
		n, err := s.Store.CountWorkspacesForSubjectPlan(ctx, id.Subject, plan)
		if err != nil {
			return WorkspaceView{}, err
		}
		if n >= limit {
			return WorkspaceView{}, fmt.Errorf("%w: at most %d %s workspaces per user", core.ErrBadRequest, limit, plan)
		}
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

// Delete tears a workspace down: it revokes each member's OpenFGA tuple, deletes
// the tenant row (the FK cascade drops its apps, domains, and memberships in the
// same statement), nudges the projector to prune the orphaned App CRs, and runs
// the injected purgers (OpenBao secrets, managed Databases). Admin-only, and
// guarded by a confirmation that must equal the workspace name (a typo is a
// no-op, not a destroyed workspace). Idempotent enough to re-run after a partial
// failure: revoke/purge tolerate already-gone tuples/resources.
func (s *Service) Delete(ctx context.Context, id, confirmName string) error {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(id)); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrWorkspacesUnavailable
	}
	t, err := s.Store.GetTenant(ctx, id)
	if err != nil {
		return mapStoreErr(err)
	}
	if confirmName != t.Name {
		return fmt.Errorf("%w: confirmation must equal the workspace name %q", core.ErrBadRequest, t.Name)
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
		for _, m := range members {
			if err := s.Revoker.RevokeWorkspaceMember(ctx, id, "user:"+m.Subject, m.Role); err != nil {
				return fmt.Errorf("revoke %s from workspace %s: %w", m.Subject, id, err)
			}
		}
	}
	if err := s.Store.DeleteTenant(ctx, id); err != nil {
		return mapStoreErr(err)
	}
	if s.Kick != nil {
		s.Kick()
	}
	// Out-of-cascade teardown (OpenBao secrets, Databases). Best-effort and
	// idempotent; a purger failure is surfaced so a retry completes it, but the
	// row is already gone so the workspace is unusable regardless.
	for _, p := range s.Purgers {
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
