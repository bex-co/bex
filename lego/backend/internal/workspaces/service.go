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
	"log"
	"maps"
	"net/mail"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// validWorkspaceName holds a workspace slug (`tenants.name`) to the
// resource-name rule (appv1alpha1.ValidResourceName): it is a unique, typable
// key (`BEX_WORKSPACE`, `X-Bex-Workspace`, `?name=`), the same rule the internal
// tenant API enforces (store/api.go). It is not part of any resource name — App
// CR names, namespaces and labels use the tenant id (store.CRName). Render's
// freeform workspace name is `tenants.display_name` (w3/m246).
func validWorkspaceName(name string) error {
	return core.CheckResourceName("name", name)
}

// Service holds the workspace lifecycle logic once. It embeds *core.Base for the
// authorization gate + caller Identity, and writes through the Postgres source
// of truth (Store) with OpenFGA membership kept in lockstep (Granter/Revoker).
type Service struct {
	*core.Base
	// Store is the control-plane source of truth for workspaces. Nil (DB-less
	// mode / the authz sweep) leaves the verbs answering ErrWorkspacesUnavailable
	// after the Authorize gate — never a nil-deref.
	Store WorkspaceStore
	// CreationStore and CreationBilling form the GraphQL-only, pre-tenant
	// billing workflow. Kept separate from WorkspaceStore so read-only and
	// legacy test stores do not accidentally gain provisional-financial verbs.
	CreationStore   WorkspaceCreationStore
	CreationBilling WorkspaceCreationBilling
	// CustomerEmail sets the Stripe Customer's email when a workspace's billing
	// email changes, and when a move to Hobby resets it (w6/m627). Nil (Stripe
	// off) changes bex's own record alone.
	CustomerEmail CustomerEmailSetter
	// CustomerName renames the workspace's Stripe Customer, its invoices'
	// Bill-to, when the workspace is renamed (w6/m628). Nil (Stripe off)
	// renames the workspace alone.
	CustomerName CustomerNameFollower
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
	// BEFORE the tenant row (and its FK cascade) is dropped: the API keys bound
	// there (w9/120), OpenBao env-var secrets, env groups, managed
	// Databases/KeyValues, its OpenSandbox sandboxes, and its Stripe
	// subscription. Running before DeleteTenant is what
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
	// Settler charges the workspace's unbilled usage before anything is torn
	// down (ADR091 D10.4, ADR086): a declined charge refuses the delete with
	// BILLING_BALANCE_DUE and leaves the workspace, its members and its
	// Stripe Subscription intact. Nil (Stripe disabled) skips it.
	Settler WorkspaceSettler
	// Identities looks up owner/member email + name + MFA from Kratos (the store
	// carries only subjects). Nil => those fields omitted from the owners/members
	// responses (honest subset; BEX_KRATOS_ADMIN_URL unset).
	Identities IdentityReader
	// KeyOwners resolves an API-key caller to the identity subject that minted
	// the key (w4/m25), so CurrentUser can answer with the owning human's
	// email/name through the same Identities lookup. Nil => machine callers keep
	// the earliest-admin-email fallback alone.
	KeyOwners KeyOwnerReader
	// OpsWorkspaceID pins the platform ops workspace (BEX_OPS_WORKSPACE,
	// docs/ADR088-platform-observability-ui.md §4), whose membership is the
	// observability-UI ACL: Delete — the public verb AND the account-deletion
	// teardown path — refuses it with core.CodeOpsWorkspaceProtected, because
	// destroying it would lock every operator out at once. Empty (unset) leaves
	// every workspace deletable — zero behavior change.
	OpsWorkspaceID string
	// Members runs Render's member mutations through the members service
	// (authorization, self and last-admin safeguards, audit); nil → ErrWorkspacesUnavailable.
	Members MemberMutator
	// OrgTuples keeps OpenFGA's org tuples in step with the organization rows
	// (w7/m464, ADR115 D5). Nil when authz is disabled; the rows are still the
	// source of truth.
	OrgTuples core.OrganizationTupleWriter
	// IPLayerTargets project the workspace inbound-IP layer onto the
	// workspace's datastores (w9/m388): the Postgres and Key Value services.
	// Its web services and static sites are patched here. Empty leaves
	// datastores untouched (test fakes).
	IPLayerTargets []IPLayerTarget

	gqlWorkspaceOnce sync.Once
	gqlWorkspace     *graphql.Object
	// TwoFactorChanged is told which workspace's "require two-factor" setting
	// just changed, so the enforcement point (internal/twofactor) drops this
	// replica's cached copy at once instead of within its TTL. Nil when nothing
	// enforces. It is told about the Google-login setting too: the gate caches
	// both settings as one. An org's change (w7/m466) tells it the org's object,
	// core.OrganizationObject(orgID), and each org workspace.
	TwoFactorChanged func(key string)
	// GoogleLogin answers the Google-login setting's lock-out guard: would the
	// acting admin meet the requirement right now (*twofactor.Gate). Nil when
	// the deployment cannot look up Google accounts, which refuses turning the
	// setting on (409 WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE).
	GoogleLogin GoogleLoginChecker
	// LoginAssessor and LoginImpactKeys answer who turning a login
	// requirement on would block (w7/m471): the gate's assessment of each
	// member and key (*twofactor.Gate), and the API keys and SSH keys to
	// assess. Either nil answers that read with core.ErrAuthzUnavailable.
	LoginAssessor   LoginAssessor
	LoginImpactKeys LoginImpactKeys
	// Mailer and Reminders send the 2FA reminders (w1/m647): the email, and
	// the day of each workspace's last batch (*store.PGStore), which allows
	// one batch a UTC day. Either nil (no SMTP, no store) refuses Send
	// reminders with WORKSPACE_2FA_REMINDERS_UNAVAILABLE, and turning 2FA on
	// then sends none. DashboardURL is the dashboard origin the reminder
	// links to account security on; empty leaves the link out.
	Mailer       Mailer
	Reminders    TwoFactorReminderDays
	DashboardURL string
}

// GoogleLoginChecker is the Google-login gate's own check for one caller,
// run without a workspace (twofactor.Gate.SignedInWithGoogle). An error means
// the answer could not be established.
type GoogleLoginChecker interface {
	SignedInWithGoogle(ctx context.Context, id core.Identity) (bool, error)
}

// MemberMutator is the members service's role change and removal, keyed by
// the member's identity subject.
type MemberMutator interface {
	ChangeRole(ctx context.Context, workspaceID, subject, role string) error
	Remove(ctx context.Context, workspaceID, subject string) error
}

type WorkspaceCreationStore interface {
	CreateWorkspaceCreationAttempt(context.Context, string, string, string, string, bool, time.Time) (store.WorkspaceCreationAttempt, error)
	GetWorkspaceCreationAttempt(context.Context, string, string) (store.WorkspaceCreationAttempt, error)
	SetWorkspaceCreationSetup(context.Context, string, string, string, string, bool) (store.WorkspaceCreationAttempt, error)
	MarkWorkspaceCreationSetupSucceeded(context.Context, string, string, string) (store.WorkspaceCreationAttempt, error)
	SetWorkspaceCreationSubscription(context.Context, string, string, string) error
	FinalizeWorkspaceCreation(context.Context, string, string, time.Time) (store.Tenant, error)
	CancelWorkspaceCreationAttempt(context.Context, string, string) error
}

type WorkspaceCreationBilling interface {
	PrepareWorkspaceSetup(ctx context.Context, attemptID, workspaceID, workspaceName, billingEmail, customerID, setupIntentID string) (billing.WorkspaceSetup, error)
	VerifyWorkspaceSetup(context.Context, string, string, string, string) (billing.VerifiedWorkspaceSetup, error)
	PrepareWorkspaceContract(context.Context, string, string, string, string) (string, error)
}

type WorkspaceCreationPolicy struct {
	Mode              string `json:"mode"`
	PaymentRequired   bool   `json:"paymentRequired"`
	ProviderAvailable bool   `json:"providerAvailable"`
}

type WorkspaceCreationAttemptView struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Plan            string `json:"plan"`
	BillingEmail    string `json:"billingEmail"`
	PaymentRequired bool   `json:"paymentRequired"`
	State           string `json:"state"`
	ClientSecret    string `json:"clientSecret,omitempty"`
	PublishableKey  string `json:"publishableKey,omitempty"`
}

func creationAttemptView(a store.WorkspaceCreationAttempt) WorkspaceCreationAttemptView {
	return WorkspaceCreationAttemptView{
		ID: a.ID, Name: a.Name, Plan: a.Plan,
		BillingEmail: a.BillingEmail, PaymentRequired: a.PaymentRequired,
		State: a.State,
	}
}

// WorkspaceStore is the slice of the source of truth this feature writes
// through — narrow, like apps.IntentStore, so the service can't grow into a
// second store client and tests fake exactly these. *store.PGStore satisfies it.
type WorkspaceStore interface {
	CreateWorkspace(ctx context.Context, name, plan, ownerSubject string) (store.Tenant, error)
	GetTenant(ctx context.Context, id string) (store.Tenant, error)
	SetTenantDisplayName(ctx context.Context, id, name string) (store.Tenant, error)
	// SetTenantRequireTwoFactor turns the workspace's "require two-factor
	// authentication" setting on or off. changed=false means it already held
	// enabled, so of two admins toggling at once only one changes it.
	SetTenantRequireTwoFactor(ctx context.Context, id string, enabled bool) (t store.Tenant, changed bool, err error)
	// SetTenantRequireGoogleLogin is the same for "require Google login"
	// (w7/m465).
	SetTenantRequireGoogleLogin(ctx context.Context, id string, enabled bool) (t store.Tenant, changed bool, err error)
	UpdateTenantPlan(ctx context.Context, id, plan string) (store.Tenant, error)
	DeleteTenant(ctx context.Context, id string) error
	// StartTenantDelete marks the workspace's delete started, before the delete
	// revokes its members (w9/121); DeletingTenantAdmin is then the store's
	// answer to whether subject is a recorded admin of it, which Delete accepts
	// once that revoke has left OpenFGA nothing to say.
	StartTenantDelete(ctx context.Context, id string) error
	DeletingTenantAdmin(ctx context.Context, tenantID, subject string) (bool, error)
	ListTenantsForSubject(ctx context.Context, subject string) ([]store.Tenant, error)
	ListTenantMembers(ctx context.Context, tenantID string) ([]store.TenantMember, error)
	// TenantContactSubjects answers each workspace's contact subject: its
	// owner binding, else its oldest user admin (w5/m103, w5/126).
	TenantContactSubjects(ctx context.Context, tenantIDs []string) (map[string]string, error)
	// ListInvites returns a workspace's OUTSTANDING invites (unaccepted,
	// unexpired) — the seats ChangePlan's downgrade guards must count alongside
	// the members, since each one becomes a member on its recipient's next login.
	ListInvites(ctx context.Context, tenantID string) ([]store.Invite, error)
	CountWorkspacesForSubjectPlan(ctx context.Context, subject, plan string) (int, error)
	// CountAppsForTenant counts a workspace's services (all of them, including
	// suspended) — ChangePlan's downgrade guard against the target plan's
	// MaxServices, the same count internal/store/api.go's create-time cap uses.
	CountAppsForTenant(ctx context.Context, tenantID string) (int, error)
	// OwnerIDsForSubjects returns each subject's stable opaque "own-" id
	// (minted on first sight) — the Render userId the members surface reports
	// instead of the raw subject (w6/m7).
	OwnerIDsForSubjects(ctx context.Context, subjects []string) (map[string]string, error)
	// SubjectForOwnerID is its inverse, for the Render member mutations.
	SubjectForOwnerID(ctx context.Context, ownID string) (string, error)
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
// resolution (the tenancy.Service cache behind core.Base's WorkspaceResolver)
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

// WorkspaceSettler settles a workspace's balance before it is deleted;
// *billing.Settler satisfies it.
type WorkspaceSettler interface {
	SettleWorkspace(ctx context.Context, workspaceID string) error
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
	// LookupMany resolves a list's subjects at once (w5/116). A subject that
	// did not resolve is absent, and the caller omits its fields.
	LookupMany(ctx context.Context, subjects []string) map[string]IdentityAttrs
	// LookupEmails is LookupMany for a caller that reads only the email: it
	// asks the IdP for no credentials, which a batch read otherwise loads
	// (w5/133). Its error, core.ErrIdentityLookupFailed, says some subjects
	// could not be read; the map still holds every email that resolved.
	LookupEmails(ctx context.Context, subjects []string) (map[string]string, error)
	// MFAEnrolled reports whether a Kratos identity has an enrolled second
	// factor (a TOTP credential, or a WebAuthn one with a registered key) —
	// the fact a workspace's "require two-factor" setting turns on
	// (docs/ADR012-auth.md §9a). human=false means the subject is
	// not a Kratos identity (a platform principal that can never enroll); an
	// error means the IdP could not answer and the caller fails closed.
	MFAEnrolled(ctx context.Context, subject string) (enrolled, human bool, err error)
}

// IdentityAttrs are the IdP attributes a Render owner/member object needs that the
// store/CRs don't hold. ok=false from Lookup => the caller omits the field. Name
// is the optional Kratos `name` trait (w4/m25); "" when the identity never set it.
type IdentityAttrs struct {
	Email string
	// EmailVerified is Kratos' verification state for Email (its matching
	// verifiable address). Read by the identity-claims verb and the auth
	// gate's human-OAuth verification lookup (w2/m168).
	EmailVerified bool
	Name          string
	MFAEnabled    bool
	// GitHubUserID is the numeric GitHub user id of the member's GitHub
	// sign-in, when the oidc credential was read and one is linked.
	GitHubUserID string
}

// KeyOwnerReader resolves a machine caller's API key (Hydra client id) to the
// identity subject that minted it — the key→identity binding stamped as the
// client's bex.co/created-by metadata (w4/m13), read back here so a machine
// caller's GET /v1/users can report its owning human's id/email/name (w4/m25,
// w4/062). ok=false for an unknown/non-API-key client or a key minted before
// the stamp existed. Nil => machine callers fall back to the earliest-admin
// email alone (no fabricated id).
type KeyOwnerReader interface {
	KeyOwner(ctx context.Context, clientID string) (subject string, ok bool)
}

// WorkspaceView is the neutral projection of a workspace — the tenant row plus
// the caller's role in it. Each adapter maps this to its wire format (m2's REST
// renders it in Render's owner shape; the GraphQL fragment renders it directly).
type WorkspaceView struct {
	ID string `json:"id"`
	// Name is what Render calls the workspace's name: its freeform display
	// name, else its slug (w3/m246).
	Name string `json:"name"`
	// Slug is the unique DNS-label key (CLI `BEX_WORKSPACE`, `X-Bex-Workspace`,
	// `?name=`) — a bex extension.
	Slug      string `json:"slug"`
	Plan      string `json:"plan"`
	Role      string `json:"role,omitempty"`
	CreatedAt string `json:"createdAt"`
	// IPAllowList is the workspace's inbound IP rules (w9/m388), Render's
	// owner.ipAllowList: what it saved, or the seeded allow-all when it never
	// saved any. Only the surfaces that return it read it (withIPAllowLists:
	// the owners REST reads, MCP list_workspaces, SetIPAllowList); GraphQL
	// reads it when the field is selected.
	IPAllowList []core.IPAllowListEntry `json:"ipAllowList,omitempty"`
	// RequireTwoFactor is the workspace's "require two-factor authentication"
	// setting, readable by every member: a member the setting blocks
	// must be able to see why. A bex extension (Render has no workspace
	// settings API), so the Render owner shape does not carry it.
	RequireTwoFactor bool `json:"requireTwoFactor"`
	// RequireGoogleLogin is the workspace's "require Google login" setting
	// (w7/m465), readable by every member like RequireTwoFactor, and like it
	// a bex extension the Render owner shape does not carry.
	RequireGoogleLogin bool `json:"requireGoogleLogin"`
	// OverlappingDeployPolicy is the workspace's Overlapping Deploy Policy
	// (w4/653), "wait" or "override": GraphQL reads it when the field is
	// selected. Render's REST owner has no such field, so it is never encoded.
	OverlappingDeployPolicy string `json:"-"`
	// OrganizationID and Privacy are the workspace's org and its access
	// setting there (w7/m464, ADR115). Render's owner object has neither, so
	// they are never encoded on REST; GraphQL reads them.
	OrganizationID string `json:"-"`
	Privacy        string `json:"-"`
	// Organization is the org List read for the workspace in one query, so
	// the GraphQL organization and primary fields cost none per row; nil when
	// it was not read.
	Organization *store.Organization `json:"-"`
}

// requiresLogin reports whether the workspace may have a login requirement, so
// a read of its configuration must ask the gate whether the caller may see it.
// The workspace's own settings are not the whole answer: its org's bind it too
// (w7/m466), and when the org was not read with the view the gate decides.
func (w WorkspaceView) requiresLogin() bool {
	if w.RequireTwoFactor || w.RequireGoogleLogin {
		return true
	}
	if w.OrganizationID == "" {
		return false
	}
	return w.Organization == nil || w.Organization.RequireTwoFactor || w.Organization.RequireGoogleLogin
}

func view(t store.Tenant, role string) WorkspaceView {
	created := ""
	if !t.CreatedAt.IsZero() {
		created = t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	v := WorkspaceView{ID: t.ID, Name: t.Label(), Slug: t.Name, Plan: t.Plan, Role: role, CreatedAt: created, RequireTwoFactor: t.RequireTwoFactor,
		RequireGoogleLogin: t.RequireGoogleLogin, OrganizationID: t.OrganizationID}
	if t.OrganizationID != "" {
		v.Privacy = t.Privacy // a standalone workspace has no privacy setting
	}
	return v
}

func (s *Service) WorkspaceCreationPolicy(ctx context.Context, plan string) (WorkspaceCreationPolicy, error) {
	if err := s.AuthorizeMintClass(ctx); err != nil {
		return WorkspaceCreationPolicy{}, err
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return WorkspaceCreationPolicy{}, err
	}
	normalized, err := normalizePlan(plan)
	if err != nil {
		return WorkspaceCreationPolicy{}, err
	}
	return s.workspaceCreationPolicy(normalized), nil
}

func (s *Service) workspaceCreationPolicy(normalizedPlan string) WorkspaceCreationPolicy {
	mode := "off"
	required := false
	if s.Payment != nil {
		mode = "paid"
		required = workspacePaidPlan(normalizedPlan)
		if s.PaymentAllPlans {
			mode = "all"
			required = true
		}
	}
	return WorkspaceCreationPolicy{Mode: mode, PaymentRequired: required, ProviderAvailable: s.CreationBilling != nil}
}

func normalizeBillingEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	if email == "" || len(email) > 254 {
		return "", fmt.Errorf("%w: billing email is required", core.ErrBadRequest)
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", fmt.Errorf("%w: billing email is invalid", core.ErrBadRequest)
	}
	return email, nil
}

func (s *Service) authorizeWorkspaceCreation(ctx context.Context) (core.Identity, error) {
	if err := s.AuthorizeMintClass(ctx); err != nil {
		return core.Identity{}, err
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return core.Identity{}, err
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok || id.Subject == "" {
		return core.Identity{}, core.ErrForbidden
	}
	return id, nil
}

func (s *Service) validateWorkspaceCreation(ctx context.Context, id core.Identity, name, plan, billingEmail string) (string, string, error) {
	if err := validWorkspaceName(name); err != nil {
		return "", "", err
	}
	normalizedPlan, err := normalizePlan(plan)
	if err != nil {
		return "", "", err
	}
	email, err := normalizeBillingEmail(billingEmail)
	if err != nil {
		return "", "", err
	}
	if normalizedPlan == store.PlanHobby && s.Identities != nil {
		emails, err := s.Identities.LookupEmails(ctx, []string{id.Subject})
		address := emails[id.Subject]
		if address == "" && err != nil {
			// An unread account email cannot be matched; refuse rather than
			// accept any address (w5/160). An account with no email trait
			// still skips the match.
			return "", "", err
		}
		if address != "" {
			accountEmail, normalizeErr := normalizeBillingEmail(address)
			if normalizeErr != nil || email != accountEmail {
				return "", "", fmt.Errorf("%w: Hobby billing email must match the account email", core.ErrBadRequest)
			}
		}
	}
	if err := s.requirePlanEntitlement(ctx, id.Subject, "", normalizedPlan); err != nil {
		return "", "", err
	}
	if err := s.guardPerUserWorkspaceCap(ctx, id.Subject, normalizedPlan); err != nil {
		return "", "", err
	}
	return normalizedPlan, email, nil
}

func (s *Service) prepareWorkspaceCreationGuards(ctx context.Context, name, plan, billingEmail string) (core.Identity, string, string, error) {
	id, err := s.authorizeWorkspaceCreation(ctx)
	if err != nil {
		return core.Identity{}, "", "", err
	}
	normalizedPlan, email, err := s.validateWorkspaceCreation(ctx, id, name, plan, billingEmail)
	return id, normalizedPlan, email, err
}

// PrepareWorkspaceCreation reserves an invisible workspace id and, when the
// policy requires it (or the user elects to add one), creates that workspace's
// own Stripe Customer + SetupIntent. No current-workspace billing state is read.
func (s *Service) PrepareWorkspaceCreation(ctx context.Context, name, plan, billingEmail, resumeID string, collectPaymentMethod bool) (WorkspaceCreationAttemptView, error) {
	id, normalizedPlan, email, err := s.prepareWorkspaceCreationGuards(ctx, name, plan, billingEmail)
	if err != nil {
		return WorkspaceCreationAttemptView{}, err
	}
	if s.CreationStore == nil {
		return WorkspaceCreationAttemptView{}, core.ErrWorkspacesUnavailable
	}
	policy := s.workspaceCreationPolicy(normalizedPlan)
	var attempt store.WorkspaceCreationAttempt
	if resumeID != "" {
		attempt, err = s.CreationStore.GetWorkspaceCreationAttempt(ctx, resumeID, id.Subject)
		if err != nil {
			return WorkspaceCreationAttemptView{}, mapWorkspaceCreationStoreErr(err)
		}
		if attempt.Name != name || attempt.Plan != normalizedPlan || attempt.BillingEmail != email || attempt.PaymentRequired != policy.PaymentRequired {
			return WorkspaceCreationAttemptView{}, core.ErrConflict
		}
	} else {
		attempt, err = s.CreationStore.CreateWorkspaceCreationAttempt(ctx, id.Subject, name, normalizedPlan, email, policy.PaymentRequired, time.Now().Add(24*time.Hour))
		if err != nil {
			return WorkspaceCreationAttemptView{}, mapWorkspaceCreationStoreErr(err)
		}
	}
	result := creationAttemptView(attempt)
	if !policy.PaymentRequired && !collectPaymentMethod {
		return result, nil
	}
	if s.CreationBilling == nil {
		return WorkspaceCreationAttemptView{}, fmt.Errorf("%w: workspace payment setup is unavailable", core.ErrBillingUnavailable)
	}
	setup, err := s.CreationBilling.PrepareWorkspaceSetup(ctx, attempt.ID, attempt.WorkspaceID, attempt.Name, attempt.BillingEmail, attempt.ProviderCustomerID, attempt.ProviderSetupIntentID)
	if err != nil {
		return WorkspaceCreationAttemptView{}, core.HideCause(core.ErrBillingUnavailable, fmt.Errorf("workspace %s payment setup: %w", attempt.WorkspaceID, err))
	}
	attempt, err = s.CreationStore.SetWorkspaceCreationSetup(ctx, attempt.ID, id.Subject, setup.CustomerID, setup.SetupIntentID, setup.Livemode)
	if err != nil {
		return WorkspaceCreationAttemptView{}, mapWorkspaceCreationStoreErr(err)
	}
	result = creationAttemptView(attempt)
	result.ClientSecret = setup.ClientSecret
	result.PublishableKey = setup.PublishableKey
	return result, nil
}

func (s *Service) ResumeWorkspaceCreation(ctx context.Context, attemptID string) (WorkspaceCreationAttemptView, error) {
	id, err := s.authorizeWorkspaceCreation(ctx)
	if err != nil {
		return WorkspaceCreationAttemptView{}, err
	}
	if s.CreationStore == nil {
		return WorkspaceCreationAttemptView{}, core.ErrNotFound
	}
	attempt, err := s.CreationStore.GetWorkspaceCreationAttempt(ctx, attemptID, id.Subject)
	if err != nil {
		return WorkspaceCreationAttemptView{}, mapWorkspaceCreationStoreErr(err)
	}
	result := creationAttemptView(attempt)
	if attempt.State == store.WorkspaceCreationSetupPending && s.CreationBilling != nil {
		setup, setupErr := s.CreationBilling.PrepareWorkspaceSetup(ctx, attempt.ID, attempt.WorkspaceID, attempt.Name, attempt.BillingEmail, attempt.ProviderCustomerID, attempt.ProviderSetupIntentID)
		if setupErr != nil {
			return WorkspaceCreationAttemptView{}, core.HideCause(core.ErrBillingUnavailable, fmt.Errorf("workspace %s resumed payment setup: %w", attempt.WorkspaceID, setupErr))
		}
		result.ClientSecret = setup.ClientSecret
		result.PublishableKey = setup.PublishableKey
	}
	return result, nil
}

func (s *Service) FinalizeWorkspaceCreation(ctx context.Context, attemptID string) (WorkspaceView, error) {
	id, err := s.authorizeWorkspaceCreation(ctx)
	if err != nil {
		return WorkspaceView{}, err
	}
	if s.CreationStore == nil {
		return WorkspaceView{}, core.ErrNotFound
	}
	attempt, err := s.CreationStore.GetWorkspaceCreationAttempt(ctx, attemptID, id.Subject)
	if err != nil {
		return WorkspaceView{}, mapWorkspaceCreationStoreErr(err)
	}
	if attempt.State == store.WorkspaceCreationFinalized {
		tenant, getErr := s.Store.GetTenant(ctx, attempt.WorkspaceID)
		if getErr != nil {
			return WorkspaceView{}, getErr
		}
		return view(tenant, "admin"), nil
	}
	if _, _, err := s.validateWorkspaceCreation(ctx, id, attempt.Name, attempt.Plan, attempt.BillingEmail); err != nil {
		return WorkspaceView{}, err
	}

	if attempt.State == store.WorkspaceCreationSetupPending {
		if s.CreationBilling == nil {
			return WorkspaceView{}, core.ErrBillingUnavailable
		}
		verified, verifyErr := s.CreationBilling.VerifyWorkspaceSetup(ctx, attempt.ID, attempt.WorkspaceID, attempt.ProviderCustomerID, attempt.ProviderSetupIntentID)
		if verifyErr != nil {
			if billing.IsPaymentMethodRejected(verifyErr) {
				return WorkspaceView{}, verifyErr
			}
			if billing.IsInputError(verifyErr) {
				return WorkspaceView{}, core.NewPaymentRequiredError()
			}
			return WorkspaceView{}, core.HideCause(core.ErrBillingUnavailable, fmt.Errorf("workspace %s payment verification: %w", attempt.WorkspaceID, verifyErr))
		}
		attempt, err = s.CreationStore.MarkWorkspaceCreationSetupSucceeded(ctx, attempt.ID, id.Subject, verified.PaymentMethodID)
		if err != nil {
			return WorkspaceView{}, mapWorkspaceCreationStoreErr(err)
		}
	}
	if attempt.PaymentRequired && attempt.State != store.WorkspaceCreationSetupSucceeded {
		return WorkspaceView{}, core.NewPaymentRequiredError()
	}
	if attempt.State == store.WorkspaceCreationSetupSucceeded && attempt.ProviderSubscriptionID == "" {
		if s.CreationBilling == nil {
			return WorkspaceView{}, core.ErrBillingUnavailable
		}
		subscriptionID, contractErr := s.CreationBilling.PrepareWorkspaceContract(ctx, attempt.ID, attempt.WorkspaceID, attempt.ProviderCustomerID, attempt.ProviderPaymentMethodID)
		if contractErr != nil {
			return WorkspaceView{}, core.HideCause(core.ErrBillingUnavailable, fmt.Errorf("workspace %s contract: %w", attempt.WorkspaceID, contractErr))
		}
		if err := s.CreationStore.SetWorkspaceCreationSubscription(ctx, attempt.ID, id.Subject, subscriptionID); err != nil {
			return WorkspaceView{}, mapWorkspaceCreationStoreErr(err)
		}
	}

	// Establish the tuple before the tenant becomes visible. If the SQL
	// transaction fails, revoke the orphan tuple; the production OpenFGA client
	// supplies both sides. A nil revoker retains the legacy test/store behavior.
	preGranted := s.Granter != nil && s.Revoker != nil
	if preGranted {
		if err := s.Granter.GrantWorkspaceAdmin(ctx, attempt.WorkspaceID, "user:"+id.Subject); err != nil {
			return WorkspaceView{}, fmt.Errorf("workspace %s: granting admin failed: %w", attempt.WorkspaceID, err)
		}
	}
	tenant, err := s.CreationStore.FinalizeWorkspaceCreation(ctx, attempt.ID, id.Subject, time.Now())
	if err != nil {
		if preGranted {
			_ = s.Revoker.RevokeWorkspaceMember(ctx, attempt.WorkspaceID, "user:"+id.Subject, "admin")
		}
		return WorkspaceView{}, mapWorkspaceCreationStoreErr(err)
	}
	if s.Granter != nil && !preGranted {
		if err := s.Granter.GrantWorkspaceAdmin(ctx, tenant.ID, "user:"+id.Subject); err != nil {
			_ = s.Store.DeleteTenant(ctx, tenant.ID)
			return WorkspaceView{}, fmt.Errorf("workspace %s: granting admin failed: %w", tenant.ID, err)
		}
	}
	return view(tenant, "admin"), nil
}

func (s *Service) CancelWorkspaceCreation(ctx context.Context, attemptID string) error {
	id, err := s.authorizeWorkspaceCreation(ctx)
	if err != nil {
		return err
	}
	if s.CreationStore == nil {
		return core.ErrNotFound
	}
	return mapWorkspaceCreationStoreErr(s.CreationStore.CancelWorkspaceCreationAttempt(ctx, attemptID, id.Subject))
}

func mapWorkspaceCreationStoreErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return core.NewNotFoundError("WORKSPACE_CREATION_NOT_FOUND", "workspace creation attempt not found", nil)
	case errors.Is(err, store.ErrConflict):
		return core.NewConflictError("WORKSPACE_CREATION_STALE", "workspace creation attempt is stale or no longer resumable", nil)
	default:
		return err
	}
}

// List returns the caller's workspaces (the dashboard switcher / owners list).
// The query is membership-scoped to the caller's subject, so this returns only
// workspaces they belong to regardless of the coarse can_view gate.
func (s *Service) List(ctx context.Context) ([]WorkspaceView, error) {
	// Exempt from the two-factor requirement: a blocked member lists
	// their workspaces to learn which one requires 2FA, from the list's
	// requireTwoFactor.
	ctx = core.WithTwoFactorExempt(ctx)
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
	return s.withOrganizations(ctx, id.Subject, out)
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
// otherwise it authorizes can_view on the exact workspace, and an unknown id
// and a workspace the caller is not in both answer errWorkspaceNotFound
// (authorizeWorkspaceRead).
func (s *Service) GetWorkspace(ctx context.Context, ownerID string) (WorkspaceView, error) {
	if strings.HasPrefix(ownerID, ownIDPrefix) {
		return s.defaultWorkspace(ctx)
	}
	ownerID, err := s.authorizeWorkspaceRead(ctx, ownerID)
	if err != nil {
		return WorkspaceView{}, err
	}
	if s.Store == nil {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	t, err := s.Store.GetTenant(ctx, ownerID)
	if err != nil {
		return WorkspaceView{}, workspaceReadErr(err)
	}
	return view(t, "admin"), nil
}

// errWorkspaceNotFound is what a read of a workspace by its own id answers
// for an id that names no workspace and for one the caller is not in.
var errWorkspaceNotFound = core.NotFound("workspace")

// authorizeWorkspaceRead is AuthorizeWorkspace(can_view) for a verb that reads
// the workspace ownerID names: Render's retrieve-workspace and
// list-workspace-members document 404 and no 403 (w8/349). The plain refusal
// (no membership, or no such workspace) answers errWorkspaceNotFound, so the
// answer never tells a non-member that the workspace exists. A member's named
// refusal (a 2FA or Google-login requirement) and a narrowed token's
// insufficient scope keep their 403. An empty ownerID names no workspace: it
// resolves the caller's own, whose refusal stays a 403.
func (s *Service) authorizeWorkspaceRead(ctx context.Context, ownerID string) (string, error) {
	resolved, err := s.AuthorizeWorkspace(ctx, core.RelCanView, ownerID)
	if ownerID != "" && err == core.ErrForbidden {
		return resolved, errWorkspaceNotFound
	}
	return resolved, err
}

// workspaceReadErr is mapStoreErr for authorizeWorkspaceRead's verbs: a
// missing row answers what a refused read does.
func workspaceReadErr(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return errWorkspaceNotFound
	}
	return mapStoreErr(err)
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
// (components.schemas.user: {id, email, name}).
type UserView struct {
	ID    string
	Email string
	Name  string
}

// CurrentUser answers GET /v1/users, Render's "who am I" endpoint (used by e.g.
// the official Render CLI's `render whoami`). A session caller's id/email/name
// come straight off their Kratos identity (already resolved onto core.Identity
// by the auth gate). A machine (API-key/OAuth) caller carries no traits of its
// own: a user-consented OAuth token's subject is a Kratos identity id, looked
// up directly, while a client_credentials (API-key) token — recognized by the
// gate's ClientID stamp, no probing — resolves through the key's created-by
// binding (KeyOwners, w4/m25) to the human who minted it. A key with no
// resolvable owning human (minted before the created-by stamp, or a
// service-account-style key) degrades to the bound workspace's contact email
// (ownerEmails) with no name and no fabricated id — the documented honest subset. No
// GraphQL/MCP equivalent: the dashboard authenticates with its Kratos session
// directly rather than this REST-only endpoint, and no MCP tool needs "who am I".
func (s *Service) CurrentUser(ctx context.Context) (UserView, error) {
	// Exempt from the two-factor requirement: a blocked member can
	// still ask who they are.
	ctx = core.WithTwoFactorExempt(ctx)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return UserView{}, err
	}
	id, _ := core.IdentityFrom(ctx)
	u := UserView{Email: id.Email, Name: id.Name}
	human := id.Subject
	if id.Subject == id.ClientID {
		// The caller IS an API key — its subject can never be a human id.
		human = ""
		if s.KeyOwners != nil {
			human, _ = s.KeyOwners.KeyOwner(ctx, id.Subject)
		}
	}
	// Render's whoami id is the same usr- id the members list answers for
	// this person (w5/m277), never the raw Kratos subject.
	// A store failure degrades to no id rather than failing whoami.
	if human != "" && s.Store != nil {
		if ownIDs, err := s.Store.OwnerIDsForSubjects(ctx, []string{human}); err == nil {
			u.ID = ids.UserPublicID(ownIDs[human])
		}
	}
	if u.Email == "" && s.Identities != nil && human != "" {
		if attrs, ok := s.Identities.Lookup(ctx, human); ok {
			u.Email, u.Name = attrs.Email, attrs.Name
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
// filters, the per-workspace contact email and inbound IP rules (w9/m388) —
// GET /v1/owners.
func (s *Service) ListOwners(ctx context.Context, f OwnerFilter) ([]OwnerView, error) {
	ws, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(f.Names) > 0 {
		ws = matchNames(ws, f.Names)
	}
	ids := make([]string, 0, len(ws))
	for _, w := range ws {
		ids = append(ids, w.ID)
	}
	emails := s.ownerEmails(ctx, ids)
	matched := slices.DeleteFunc(ws, func(w WorkspaceView) bool {
		return len(f.Emails) > 0 && !slices.Contains(f.Emails, emails[w.ID])
	})
	if matched, err = s.withIPAllowLists(ctx, matched); err != nil {
		return nil, err
	}
	out := make([]OwnerView, 0, len(matched))
	for _, w := range matched {
		out = append(out, OwnerView{WorkspaceView: w, Email: emails[w.ID]})
	}
	return out, nil
}

// matchNames is the owners name filter (w3/m246). Render filters by its
// freeform name; bex also accepts the slug, and an exact slug match wins
// outright because slugs are unique: renaming another workspace to someone's
// slug must not redirect a typed key. Otherwise every display-name match is
// kept, so a caller can report ambiguity.
func matchNames(ws []WorkspaceView, names []string) []WorkspaceView {
	out := make([]WorkspaceView, 0, len(ws))
	for _, w := range ws {
		for _, n := range names {
			if w.Slug == n || (w.Name == n && !slices.ContainsFunc(ws, func(o WorkspaceView) bool { return o.Slug == n })) {
				out = append(out, w)
				break
			}
		}
	}
	return out
}

// ResolveResourceOwners implements resourcemeta.OwnerResolver for the four
// resource REST adapters (apps, postgres, keyvalue, projects). One
// membership-scoped workspace query resolves every unique id in a list
// response; an id absent from that result is omitted, so a resource adapter
// can never use this seam to reveal another workspace. Email remains
// best-effort, resolved for every workspace in the response at once.
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
	tenants = slices.DeleteFunc(tenants, func(tenant store.Tenant) bool {
		_, ok := wanted[tenant.ID]
		return !ok
	})
	ids := make([]string, len(tenants))
	for i, tenant := range tenants {
		ids[i] = tenant.ID
	}
	emails := s.ownerEmails(ctx, ids)
	out := make(map[string]resourcemeta.Owner, len(tenants))
	for _, tenant := range tenants {
		out[tenant.ID] = resourcemeta.Owner{ID: tenant.ID, Name: tenant.Label(), Email: emails[tenant.ID], Type: "team"}
	}
	return out
}

// ownerEmail is ownerEmails for one workspace.
func (s *Service) ownerEmail(ctx context.Context, tenantID string) string {
	return s.ownerEmails(ctx, []string{tenantID})[tenantID]
}

// ownerEmails resolves each workspace's contact email, with one store read and
// one identity lookup for the whole list (w5/126).
//
// It follows the OWNER BINDING first (w5/m103): tenants.owner_identity_id is
// the subject onboarding minted the workspace for, and it is the only thing in
// the model that means "whose workspace this is". Before this it asked only
// "who is the oldest admin", so removing a founding admin silently reassigned
// who the workspace appeared to belong to — an attribution change nobody
// performed, on a field billing and support read.
//
// The oldest-admin answer remains, as an explicit FALLBACK for workspaces that
// have no binding: anything created through CreateWorkspace rather than
// first-login onboarding, and, since w5/m372, a workspace whose creator was
// removed, demoted or left (the store ends the binding then). That is a contact
// address, not a claim of ownership, and it is the only case where one is
// guessed.
//
// Best-effort throughout: a workspace is absent when Identities is nil, the
// read fails, or the lookup misses (the honest subset IdentityReader
// documents). A bound owner whose identity does not resolve is absent too:
// naming the oldest admin instead would claim somebody else owns it.
func (s *Service) ownerEmails(ctx context.Context, tenantIDs []string) map[string]string {
	if s.Identities == nil || s.Store == nil || len(tenantIDs) == 0 {
		return nil
	}
	contacts, err := s.Store.TenantContactSubjects(ctx, tenantIDs)
	if err != nil || len(contacts) == 0 {
		return nil
	}
	// An unread contact is omitted, as an unknown one is.
	found, _ := s.Identities.LookupEmails(ctx, slices.Collect(maps.Values(contacts)))
	emails := make(map[string]string, len(contacts))
	for tenantID, subject := range contacts {
		if email, ok := found[subject]; ok {
			emails[tenantID] = email
		}
	}
	return emails
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

// UpdateMember is Render's update-workspace-member: userId is the member's
// usr- id (or the own- id handed out before w5/m277) and role is Render's
// teamMemberRole.
func (s *Service) UpdateMember(ctx context.Context, ownerID, userID, role string) (MemberView, error) {
	ownerID, subject, err := s.memberSubject(ctx, ownerID, userID)
	if err != nil {
		return MemberView{}, err
	}
	bexRole, ok := bexMemberRole(role)
	if !ok {
		return MemberView{}, fmt.Errorf("%w: role must be one of %s", core.ErrBadRequest, strings.Join(renderMemberRoles(), ", "))
	}
	if err := s.Members.ChangeRole(ctx, ownerID, subject, bexRole); err != nil {
		return MemberView{}, err
	}
	m := MemberView{Subject: subject, OwnerID: ids.UserStorageID(userID), Role: bexRole}
	if s.Identities != nil {
		if attrs, ok := s.Identities.LookupMany(ctx, []string{subject})[subject]; ok {
			m.Email, m.Name, m.MFAEnabled = attrs.Email, attrs.Name, attrs.MFAEnabled
		}
	}
	return m, nil
}

// RemoveMember is Render's remove-workspace-member; userId is the own- id.
func (s *Service) RemoveMember(ctx context.Context, ownerID, userID string) error {
	ownerID, subject, err := s.memberSubject(ctx, ownerID, userID)
	if err != nil {
		return err
	}
	return s.Members.Remove(ctx, ownerID, subject)
}

// memberSubject authorizes a member mutation, then resolves userId to the
// identity subject. Whether the subject belongs to this workspace is the
// members service's check.
func (s *Service) memberSubject(ctx context.Context, ownerID, userID string) (string, string, error) {
	ownerID, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, ownerID)
	if err != nil {
		return "", "", err
	}
	if s.Store == nil || s.Members == nil {
		return "", "", core.ErrWorkspacesUnavailable
	}
	subject, err := s.Store.SubjectForOwnerID(ctx, ids.UserStorageID(userID))
	if errors.Is(err, store.ErrNotFound) {
		return "", "", core.NotFound("member")
	}
	return ownerID, subject, err
}

// ListMembers lists a workspace's members — GET /v1/owners/{ownerId}/members.
// Any member may list members (every m1/m2 role includes can_view). An unknown
// workspace id and one the caller isn't a member of both answer
// errWorkspaceNotFound (authorizeWorkspaceRead).
func (s *Service) ListMembers(ctx context.Context, ownerID string) ([]MemberView, error) {
	ownerID, err := s.authorizeWorkspaceRead(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrWorkspacesUnavailable
	}
	// Confirm the workspace exists — an unknown id is not found, not a
	// silently empty member list.
	if _, err := s.Store.GetTenant(ctx, ownerID); err != nil {
		return nil, workspaceReadErr(err)
	}
	rows, err := s.Store.ListTenantMembers(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	// Resolve the opaque own- ids (minted on first sight), the Render userId, in
	// one read.
	subjects := make([]string, 0, len(rows))
	for _, m := range rows {
		subjects = append(subjects, m.Subject)
	}
	ownIDs, err := s.Store.OwnerIDsForSubjects(ctx, subjects)
	if err != nil {
		return nil, err
	}
	var identities map[string]IdentityAttrs
	if s.Identities != nil {
		identities = s.Identities.LookupMany(ctx, subjects)
	}
	out := make([]MemberView, 0, len(rows))
	for _, m := range rows {
		mv := MemberView{Subject: m.Subject, Role: m.Role}
		mv.OwnerID = ownIDs[m.Subject]
		if attrs, ok := identities[m.Subject]; ok {
			mv.Email, mv.Name, mv.MFAEnabled = attrs.Email, attrs.Name, attrs.MFAEnabled
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
	// The one CreateWorkspaceEvent is written once the workspace exists, in
	// the new workspace's own log: a create refused by validation, the plan
	// cap, the payment gate or the store records none, while a denial is
	// still recorded here (w4/679).
	if err := s.Authorize(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate); err != nil {
		return WorkspaceView{}, err
	}
	if s.Store == nil {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok || id.Subject == "" {
		return WorkspaceView{}, core.ErrForbidden
	}
	if err := validWorkspaceName(name); err != nil {
		return WorkspaceView{}, err
	}
	plan, err := normalizePlan(plan)
	if err != nil {
		return WorkspaceView{}, err
	}
	if err := s.requirePlanEntitlement(ctx, id.Subject, "", plan); err != nil {
		return WorkspaceView{}, err
	}
	policy := s.workspaceCreationPolicy(plan)
	if policy.PaymentRequired {
		// The legacy plan-only mutation cannot prove a payment method for the
		// workspace it has not created. Never borrow another workspace's marker;
		// interactive clients must use prepareWorkspaceCreation/finalize.
		return WorkspaceView{}, core.NewPaymentRequiredError()
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
	s.RecordAllowedWrite(ctx, "workspaces.Create", core.RelCanCreate, t.ID, "", t.Label())
	return view(t, "admin"), nil
}

// Rename changes a workspace's display name — freeform, as on Render (w3/m246).
// Admin-only (can_manage on the exact workspace). The id and slug stay the
// keys, so a rename breaks no references.
func (s *Service) Rename(ctx context.Context, id, name string) (WorkspaceView, error) {
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, id)
	if err != nil {
		return WorkspaceView{}, err
	}
	if s.Store == nil {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	name, err = core.TrimmedLabel(name)
	if err != nil {
		return WorkspaceView{}, err
	}
	if name == "" {
		return WorkspaceView{}, core.NameRequiredError()
	}
	var previous string
	if s.CustomerName != nil {
		if before, err := s.Store.GetTenant(ctx, id); err == nil {
			previous = before.Label()
		}
	}
	t, err := s.Store.SetTenantDisplayName(ctx, id, name)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	s.followRenameInStripe(ctx, id, previous, t.Label())
	return view(t, "admin"), nil
}

// CustomerNameFollower renames a workspace's Stripe Customer after a rename
// (*billing.StripeClient), unless its name is a member's own: a company name
// set in Billing info or one typed into the Stripe portal.
type CustomerNameFollower interface {
	FollowWorkspaceRename(ctx context.Context, workspaceID, previous, name string) error
}

// followRenameInStripe gives the workspace's Stripe Customer its new name, so
// the next invoice bills it. The rename has happened by then, so a Stripe
// failure is logged, not returned, as a move to Hobby's email reset is: the
// client's own retries (stripe-go's network retries) are the only ones.
func (s *Service) followRenameInStripe(ctx context.Context, workspaceID, previous, name string) {
	if s.CustomerName == nil || previous == name {
		return
	}
	if err := s.CustomerName.FollowWorkspaceRename(ctx, workspaceID, previous, name); err != nil {
		log.Printf("workspaces: %s was renamed, but its Stripe Customer still bills its old name: %v", workspaceID, err)
	}
}

// SetRequireTwoFactor turns a workspace's "require two-factor authentication"
// setting on or off (docs/ADR012-auth.md §9a; Render's
// ChangeTeam2FAEnforcementEvent). Admin-only (can_manage on the exact
// workspace). While it is on, a member with no enrolled second factor — and
// any API key or SSH key that member created — cannot use the workspace's
// resources: core.Base's funnel asks the enforcement gate after every positive
// authorization decision.
//
// A change that changes nothing writes nothing and records nothing. Enabling
// needs the Pro plan or higher (Render's pricing table, w7/070: 400
// PLAN_FEATURE_REQUIRED); a Hobby workspace that already enforces it keeps
// it, can re-send it and can turn it off (ADR030 §9). Enabling is also
// refused while the acting admin has no second factor themselves (409
// WORKSPACE_2FA_ADMIN_NOT_ENROLLED, so the setting cannot lock out its only
// actor), when the deployment has no identity lookup to enforce it with, and for
// the default workspace, which the gate never enforces (both 409
// WORKSPACE_2FA_UNAVAILABLE); disabling is never refused for either.
//
// Turning it on emails the day's reminders to the members without 2FA, as
// Render's confirmation promises (w1/m647, SendTwoFactorReminders' batch and
// daily limit), best-effort: the setting is on whatever the mail does.
func (s *Service) SetRequireTwoFactor(ctx context.Context, id string, enabled bool) (WorkspaceView, error) {
	// Denials stay on the record; the allowed row waits for the write, so a
	// no-op leaves none.
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, id)
	if err != nil {
		return WorkspaceView{}, err
	}
	return s.setLoginRequirement(ctx, id, enabled, twoFactorSetting)
}

// SetRequireGoogleLogin turns a workspace's "require Google login" setting on
// or off (docs/ADR012-auth.md §9b, w7/m465; Render's
// ChangeTeamAllowedLoginMethodsEvent), with SetRequireTwoFactor's rules: admin
// only, Pro and higher to turn on (400 PLAN_FEATURE_REQUIRED), a no-op records
// nothing. Turning it on is refused while the acting admin would not meet it
// themselves — signed in with Google, with the Google account of their email
// (409 WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN) — when the deployment
// cannot look up Google accounts, and for the default workspace (both 409
// WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE).
func (s *Service) SetRequireGoogleLogin(ctx context.Context, id string, enabled bool) (WorkspaceView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, id)
	if err != nil {
		return WorkspaceView{}, err
	}
	return s.setLoginRequirement(ctx, id, enabled, googleLoginSetting)
}

// loginSetting is one login requirement as its mutations see it, on a
// workspace and on an organization (w7/m466): the stored value, the plan
// feature that gates turning it on, the refusals and lock-out guard of turning
// it on, the write and the audit row.
type loginSetting struct {
	current          func(store.Tenant) bool
	orgCurrent       func(store.Organization) bool
	feature          core.PlanFeature
	defaultWorkspace func() *core.CodedError
	// actorMeets is the lock-out guard: whether the caller meets the
	// requirement right now. adminNotMet and ownerNotMet are its refusal on a
	// workspace and on an org.
	actorMeets  func(*Service, context.Context) (bool, error)
	adminNotMet func() *core.CodedError
	ownerNotMet func(orgID string) *core.CodedError
	write       func(WorkspaceStore, context.Context, string, bool) (store.Tenant, bool, error)
	orgWrite    func(OrganizationStore, context.Context, string, bool) (bool, error)
	record      func(*Service, context.Context, string, bool)
	orgRecord   func(*Service, context.Context, string, bool)
	// remindOnEnforce: turning it on in a workspace sends the day's
	// reminders to the members who do not meet it (2FA only, as on Render,
	// w1/m647).
	remindOnEnforce bool
}

var twoFactorSetting = loginSetting{
	current:          func(t store.Tenant) bool { return t.RequireTwoFactor },
	orgCurrent:       func(o store.Organization) bool { return o.RequireTwoFactor },
	feature:          core.FeatureTwoFactorEnforcement,
	defaultWorkspace: core.NewWorkspaceTwoFactorDefaultWorkspaceError,
	actorMeets:       (*Service).actorEnrolled,
	adminNotMet:      core.NewWorkspaceTwoFactorAdminNotEnrolledError,
	ownerNotMet:      core.NewOrganizationTwoFactorOwnerNotEnrolledError,
	write:            WorkspaceStore.SetTenantRequireTwoFactor,
	orgWrite:         OrganizationStore.SetOrganizationRequireTwoFactor,
	record:           func(s *Service, ctx context.Context, id string, on bool) { s.RecordTwoFactorEnforcement(ctx, id, on) },
	orgRecord: func(s *Service, ctx context.Context, orgID string, on bool) {
		s.RecordOrganizationTwoFactorEnforcement(ctx, orgID, on)
	},
	remindOnEnforce: true,
}

var googleLoginSetting = loginSetting{
	current:          func(t store.Tenant) bool { return t.RequireGoogleLogin },
	orgCurrent:       func(o store.Organization) bool { return o.RequireGoogleLogin },
	feature:          core.FeatureGoogleLoginEnforcement,
	defaultWorkspace: core.NewWorkspaceGoogleLoginDefaultWorkspaceError,
	actorMeets:       (*Service).actorSignedInWithGoogle,
	adminNotMet:      core.NewWorkspaceGoogleLoginAdminNotSignedInError,
	ownerNotMet:      core.NewOrganizationGoogleLoginOwnerNotSignedInError,
	write:            WorkspaceStore.SetTenantRequireGoogleLogin,
	orgWrite:         OrganizationStore.SetOrganizationRequireGoogleLogin,
	record:           func(s *Service, ctx context.Context, id string, on bool) { s.RecordGoogleLoginEnforcement(ctx, id, on) },
	orgRecord: func(s *Service, ctx context.Context, orgID string, on bool) {
		s.RecordOrganizationGoogleLoginEnforcement(ctx, orgID, on)
	},
}

// setLoginRequirement is the body both mutations share, once the caller is
// authorized as the workspace's admin.
func (s *Service) setLoginRequirement(ctx context.Context, id string, enabled bool, setting loginSetting) (WorkspaceView, error) {
	if s.Store == nil {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	if enabled && id == core.DefaultTenant {
		return WorkspaceView{}, setting.defaultWorkspace()
	}
	t, err := s.Store.GetTenant(ctx, id)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	if setting.current(t) == enabled {
		return view(t, "admin"), nil // already there — nothing to write
	}
	if enabled {
		if err := s.guardLoginRequirement(ctx, t.Plan, setting, setting.adminNotMet()); err != nil {
			return WorkspaceView{}, err
		}
	}
	t, changed, err := setting.write(s.Store, ctx, id, enabled)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	if !changed {
		return view(t, "admin"), nil // another admin made this same change first
	}
	if s.TwoFactorChanged != nil {
		s.TwoFactorChanged(id)
	}
	setting.record(s, ctx, id, enabled)
	if enabled && setting.remindOnEnforce {
		s.remindOnEnforce(ctx, t)
	}
	return view(t, "admin"), nil
}

// guardLoginRequirement is what turning a requirement on asks on a workspace
// and on an org alike: the plan (Pro and higher, ADR030 §9), then the
// lock-out guard, refused with notMet.
func (s *Service) guardLoginRequirement(ctx context.Context, plan string, setting loginSetting, notMet *core.CodedError) error {
	if !core.PlanAllows(plan, setting.feature) {
		return core.NewPlanFeatureError("", plan, setting.feature)
	}
	met, err := setting.actorMeets(s, ctx)
	if err != nil {
		return err
	}
	if !met {
		return notMet
	}
	return nil
}

// actorEnrolled is the 2FA lock-out guard: the admin or Owner turning the
// setting on must have a second factor of their own. An API key stands for
// the member who created it; a caller that cannot enroll at all (a key with no
// recorded creator, a platform principal) does not count as enrolled.
func (s *Service) actorEnrolled(ctx context.Context) (bool, error) {
	if s.Identities == nil {
		return false, core.NewWorkspaceTwoFactorUnavailableError()
	}
	id, _ := core.IdentityFrom(ctx)
	subject := id.Subject
	if id.IsAPIKey() {
		subject = ""
		if s.KeyOwners != nil {
			subject, _ = s.KeyOwners.KeyOwner(ctx, id.KeyClientID())
		}
	}
	if subject == "" {
		return false, nil
	}
	enrolled, human, err := s.Identities.MFAEnrolled(ctx, subject)
	if err != nil {
		log.Printf("workspaces: two-factor guard could not read the acting admin's enrollment: %v", err)
		return false, core.ErrAuthzUnavailable
	}
	return human && enrolled, nil
}

// actorSignedInWithGoogle is the Google-login setting's lock-out guard: the
// admin or Owner turning it on must meet it right now, by the gate's own
// check. A caller with no identity (a platform worker) never does.
func (s *Service) actorSignedInWithGoogle(ctx context.Context) (bool, error) {
	if s.GoogleLogin == nil {
		return false, core.NewWorkspaceGoogleLoginUnavailableError()
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok {
		return false, nil
	}
	signedIn, err := s.GoogleLogin.SignedInWithGoogle(ctx, id)
	if err != nil {
		log.Printf("workspaces: Google-login guard could not check the acting admin: %v", err)
		return false, core.ErrAuthzUnavailable
	}
	return signedIn, nil
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
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, id)
	if err != nil {
		return WorkspaceView{}, err
	}
	if s.Store == nil {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	plan, err = normalizePlan(plan)
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
	v, err := s.changePlan(ctx, t, plan)
	// Hobby's billing email is its owner's account email (w6/m627): the plan
	// write cleared the stored one, and Stripe's copy follows it.
	if err == nil && plan == store.PlanHobby {
		s.resetCustomerEmail(ctx, t.ID)
	}
	return v, err
}

// changePlan is ChangePlan's move of workspace t to plan, which differs from
// t's own.
func (s *Service) changePlan(ctx context.Context, t store.Tenant, plan string) (WorkspaceView, error) {
	id := t.ID
	identity, _ := core.IdentityFrom(ctx)
	if err := s.requirePlanEntitlement(ctx, identity.Subject, id, plan); err != nil {
		return WorkspaceView{}, err
	}
	// A workspace in an organization follows its org's plan (w7/m464,
	// ADR115 D2, D6).
	orgs, hasOrgs := s.organizations()
	if hasOrgs && t.OrganizationID != "" {
		return s.changeOrganizationPlan(ctx, orgs, t, plan)
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
	if err := s.guardPlanCapacity(ctx, id, plan); err != nil {
		return WorkspaceView{}, err
	}
	// Upgrading a standalone workspace to Scale or Enterprise creates its
	// organization, with this workspace as the primary (ADR115 D2).
	if hasOrgs && core.OrganizationPlan(plan) {
		return s.upgradeToOrganization(ctx, orgs, id, plan)
	}
	nt, err := s.Store.UpdateTenantPlan(ctx, id, plan)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	return view(nt, "admin"), nil
}

// guardPlanCapacity refuses a plan the workspace would not fit: its members and
// pending invites against MaxMembers, its services against MaxServices, the
// caller's own workspaces of that plan against MaxWorkspacesPerUser, and any
// member or pending invite holding a role the plan does not offer.
func (s *Service) guardPlanCapacity(ctx context.Context, id, plan string) error {
	members, err := s.Store.ListTenantMembers(ctx, id)
	if err != nil {
		return err
	}
	invites, err := s.Store.ListInvites(ctx, id)
	if err != nil {
		return err
	}
	limits := store.LimitsFor(plan)
	if limits.MaxMembers > 0 && len(members)+len(invites) > limits.MaxMembers {
		if len(invites) == 0 {
			return fmt.Errorf("%w: workspace has %d member(s), exceeds %s plan's limit of %d",
				core.ErrBadRequest, len(members), plan, limits.MaxMembers)
		}
		// An invite is a seat already promised, so it counts — and naming the
		// invitees is the difference between "you're over the cap" and "revoke these".
		return fmt.Errorf(
			"%w: workspace has %d member(s) + %d pending invite(s) (%s), exceeds %s plan's limit of %d; revoke the invite(s) first",
			core.ErrBadRequest, len(members), len(invites), strings.Join(inviteEmails(invites), ", "),
			plan, limits.MaxMembers)
	}
	if limits.MaxServices > 0 {
		n, err := s.Store.CountAppsForTenant(ctx, id)
		if err != nil {
			return err
		}
		if n > limits.MaxServices {
			return fmt.Errorf("%w: workspace has %d services, exceeds %s plan's limit of %d",
				core.ErrBadRequest, n, plan, limits.MaxServices)
		}
	}
	if caller, ok := core.IdentityFrom(ctx); ok {
		if err := s.guardPerUserWorkspaceCap(ctx, caller.Subject, plan); err != nil {
			return err
		}
	}
	if blocked := rolesOutsidePlan(members, plan); len(blocked) > 0 {
		return fmt.Errorf("%w: workspace has members with roles not allowed on %s (%s); downgrade first",
			core.ErrBadRequest, plan, strings.Join(s.memberRoles(ctx, blocked), ", "))
	}
	if blocked := invitesOutsidePlan(invites, plan); len(blocked) > 0 {
		return fmt.Errorf("%w: workspace has pending invites with roles not allowed on %s (%s); revoke them first",
			core.ErrBadRequest, plan, strings.Join(invitedRoles(blocked), ", "))
	}
	return nil
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

// rolesOutsidePlan returns the members whose role wouldn't be assignable on
// plan — ChangePlan's role-downgrade guard (t004): a downgrade must not leave
// a member holding a role the new plan no longer offers.
func rolesOutsidePlan(members []store.TenantMember, plan string) []store.TenantMember {
	var blocked []store.TenantMember
	for _, m := range members {
		if !store.RoleAllowedOnPlan(plan, m.Role) {
			blocked = append(blocked, m)
		}
	}
	return blocked
}

// memberRoles names members as "email:role" pairs, the identity an admin
// finds them by on the Members page (w7/087). A member whose email does not
// resolve (no IdP wired, an unread identity) is named by their subject.
func (s *Service) memberRoles(ctx context.Context, members []store.TenantMember) []string {
	var emails map[string]string
	if s.Identities != nil && len(members) > 0 {
		subjects := make([]string, 0, len(members))
		for _, m := range members {
			subjects = append(subjects, m.Subject)
		}
		emails, _ = s.Identities.LookupEmails(ctx, subjects)
	}
	out := make([]string, 0, len(members))
	for _, m := range members {
		name := emails[m.Subject]
		if name == "" {
			name = m.Subject
		}
		out = append(out, name+":"+m.Role)
	}
	return out
}

// invitesOutsidePlan is rolesOutsidePlan for the seats not taken yet
// (w6/m15/t003): the pending invites whose role the target plan doesn't
// offer. Such an invite would be refused at accept time by
// store.AcceptInvitesForEmail's own plan check, so the downgrade would quietly
// strand it — name it here instead. Invites are keyed by email (they have no
// subject until redeemed), which is also the identity the admin revokes by.
func invitesOutsidePlan(invites []store.Invite, plan string) []store.Invite {
	var blocked []store.Invite
	for _, inv := range invites {
		if !store.RoleAllowedOnPlan(plan, inv.Role) {
			blocked = append(blocked, inv)
		}
	}
	return blocked
}

// invitedRoles names pending invites as "email:role" pairs.
func invitedRoles(invites []store.Invite) []string {
	out := make([]string, 0, len(invites))
	for _, inv := range invites {
		out = append(out, inv.Email+":"+inv.Role)
	}
	return out
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
// recoverable: it marks the delete started, revokes each member's OpenFGA
// tuple, runs the pre-cascade purgers (the API keys bound there, OpenBao
// secrets, env groups, managed Databases/KeyValues, OpenSandbox sandboxes, the
// Stripe subscription) while the tenant row still exists, deletes the tenant
// row (the FK cascade drops its apps, domains, memberships, sandbox
// key, and billing rows in the same statement), nudges the projector to prune
// the orphaned App CRs, then runs the post-cascade purger (the App CRs). Running
// the bulk of the teardown BEFORE the row is dropped is what makes it retryable:
// a pre-cascade purger failure leaves the row (and its confirmation phrase)
// intact, so re-issuing the identical Delete re-runs the whole sweep. Admin-only,
// and guarded by a confirmation that must equal DeleteConfirmation(name) —
// Render's "sudo delete workspace <name>" phrase (a typo is a no-op, not a
// destroyed workspace). Every purger and the revoke tolerate already-gone
// tuples/resources so the retry converges.
//
// The retry is the admin's (w9/121). The first attempt revoked every member's
// tuple, so OpenFGA refuses everyone; the store then admits a person whose
// tenant_members row still makes them an admin of this workspace, and only
// once a delete has started on it (DeletingTenantAdmin). Nobody else gets in
// that way: not a member at another role, not someone removed before, not an
// API key (its purger revoked it anyway), and not an admin of a workspace no
// delete has reached.
func (s *Service) Delete(ctx context.Context, id, confirmName string) error {
	// codex-security round-6 #16: destroying a workspace is the most
	// destructive verb the API has, so can_manage is decided against the source
	// of truth, uncached — a just-revoked admin must not ride a ≤PositiveTTL
	// cached positive into it. can_manage is a write relation, which the funnel
	// asks OpenFGA fresh, and the store's answer is uncached too.
	id, err := s.AuthorizeWorkspaceOrRecorded(ctx, core.RelCanManage, id, s.deletingAdmin)
	if err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrWorkspacesUnavailable
	}
	t, err := s.Store.GetTenant(ctx, id)
	if err != nil {
		return mapStoreErr(err)
	}
	if want := DeleteConfirmation(t.Label()); confirmName != want {
		return fmt.Errorf("%w: confirmation must be %q", core.ErrBadRequest, want)
	}
	org, err := s.guardOrganizationDelete(ctx, t)
	if err != nil {
		return err
	}
	if err := s.deleteWorkspace(ctx, id); err != nil {
		return err
	}
	s.recordOrganizationDelete(ctx, org, t)
	return nil
}

// deletingAdmin is Delete's decision once OpenFGA has refused the caller: is
// the store still recording them as an admin of a workspace whose delete has
// started?
func (s *Service) deletingAdmin(ctx context.Context, workspaceID, subject string) (bool, error) {
	if s.Store == nil {
		return false, nil
	}
	return s.Store.DeletingTenantAdmin(ctx, workspaceID, subject)
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
	// ADR088 §4: the pinned ops workspace's membership is the observability-UI
	// ACL, so its teardown is refused outright — on the public Delete verb AND
	// on account-deletion's teardown (whose disposition should already have
	// classified the workspace blocked; this is the backstop for a deletion
	// begun before the pin was configured).
	if s.OpsWorkspaceID != "" && id == s.OpsWorkspaceID {
		return core.NewOpsWorkspaceProtectedError()
	}
	// Re-read so a background retry that starts after a prior cascade converges
	// as not-found at AccountTeardown's boundary.
	if _, err := s.Store.GetTenant(ctx, id); err != nil {
		return mapStoreErr(err)
	}
	// Settle before delete (ADR091 D10.4): first, before any member loses
	// access, so a declined charge leaves the workspace exactly as it was.
	if s.Settler != nil {
		if err := s.Settler.SettleWorkspace(ctx, id); err != nil {
			return err
		}
	}
	// Mark the delete started before any member loses access (w9/121). Once
	// the revoke below has left the members no tuple, the mark is what lets
	// the workspace's recorded admins retry a delete whose teardown failed, and
	// what keeps every path that would seat someone in it again from doing so.
	if err := s.Store.StartTenantDelete(ctx, id); err != nil {
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
	// FreeUsed/FreeLimit report the one-free-instance rule for datastores
	// (w4/m241): every workspace may hold one free Postgres and one free
	// Key Value on any plan. Zero on kinds the rule does not govern.
	FreeUsed  int `json:"freeUsed"`
	FreeLimit int `json:"freeLimit"`
}

// ResourceLimitsView is the per-workspace resource usage vs. cap (w7/m9):
// how many of each resource kind the workspace currently owns (Used) and the
// maximum it may create (Limit; 0 = unlimited). Returned by ResourceLimits.
type ResourceLimitsView struct {
	Services  ResourceCapView `json:"services"`
	Postgres  ResourceCapView `json:"postgres"`
	KeyValues ResourceCapView `json:"keyValues"`
	// PipelineMinutesExceeded is true while the workspace's builds are stopped
	// for the month: it spent its plan's pipeline minutes and can buy no more
	// (no payment method, or its spend limit reached). Render's dashboard reads
	// the same state as pipelineMinutesExceeded and disables Manual Build and
	// Manual Deploy while it holds (w3/108).
	PipelineMinutesExceeded bool `json:"pipelineMinutesExceeded"`
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
		Postgres:  ResourceCapView{Limit: int(caps.Postgres), FreeLimit: 1},
		KeyValues: ResourceCapView{Limit: int(caps.KeyValues), FreeLimit: 1},
	}
	if s.Client == nil {
		return out, nil
	}
	var apps appv1alpha1.AppList
	if listErr := s.ListByTenant(ctx, &apps, tenantID); listErr != nil {
		return ResourceLimitsView{}, fmt.Errorf("counting services: %w", listErr)
	}
	// Before the filter: a build source's carrier App is marked, and refused
	// a build, like a service.
	out.PipelineMinutesExceeded = pipelineMinutesExceeded(apps.Items, s.Now())
	apps.Items = core.ServiceApps(apps.Items)
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
	out.Postgres.FreeLimit = 1
	out.KeyValues.FreeLimit = 1
	for i := range dbs.Items {
		if dbs.Items[i].Spec.Plan == core.FreePlanID {
			out.Postgres.FreeUsed++
		}
	}
	for i := range kvs.Items {
		if kvs.Items[i].Spec.Plan == core.FreePlanID {
			out.KeyValues.FreeUsed++
		}
	}
	return out, nil
}

// pipelineMinutesExceeded reports whether the buildminutes sweeper has stopped
// the workspace's builds this month: one of its Apps carries the current
// month's mark, the one the operator refuses a build or a pre-deploy run on
// (docs/ADR030 §13). The sweeper marks and lifts every App of a workspace
// together, except one being deleted, which it leaves as it was, so a deleting
// App's mark says nothing about the workspace now.
func pipelineMinutesExceeded(apps []appv1alpha1.App, now time.Time) bool {
	for i := range apps {
		if apps[i].DeletionTimestamp.IsZero() && appv1alpha1.BuildMinutesExhaustedIn(&apps[i], now) {
			return true
		}
	}
	return false
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
