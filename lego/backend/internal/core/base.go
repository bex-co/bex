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
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Pod label + container name the controller stamps on an App's pods. Kept in
// sync by hand: the api layer must not import the operator. The logs and metrics
// features both select on these, so they live in the shared kernel.
const (
	PodLabelApp      = "app.bex.co/app"
	PodLabelRevision = "app.bex.co/revision"
	AppContainer     = "app"
	// PodLabelBuild identifies the build Job/Build pods for one App (w7/m28).
	// Build containers are discovered from the Pod status rather than named here:
	// BuildKit, signed BuildKit, and kpack use different container names.
	PodLabelBuild = "app.bex.co/build"
	// PodLabelPreDeploy + PreDeployContainer name the pre-deploy step's Job pod
	// (w1/m33, lego/operator/internal/predeploy) so the logs feature can read a
	// migration's output. Kept in sync by hand, like PodLabelApp above.
	PodLabelPreDeploy  = "app.bex.co/predeploy"
	PreDeployContainer = "predeploy"
	// PodLabelCNPGCluster + CNPGPostgresContainer identify CNPG pods for a
	// managed Postgres. CNPG stamps every cluster pod with cnpg.io/cluster=<cr-name>;
	// the main postgres container is always named "postgres". The database logs
	// feature (w3/m28) uses the label for exact pod fallback and durable-log
	// instance discovery.
	PodLabelCNPGCluster   = "cnpg.io/cluster"
	CNPGPostgresContainer = "postgres"
)

// PodLogSource fetches the raw (timestamped) log stream for one pod container.
// Defined in core (not the logs package) so the postgres feature can inject it
// without importing logs — the cross-feature import rule (w3/m28). Production
// wires it via logs.NewPodLogSource; nil => log verbs report ErrLogsUnavailable.
type PodLogSource func(ctx context.Context, namespace, pod, container string, tail int64) (io.ReadCloser, error)

// Checker is the feature services' seam to the authorization service
// (docs/ADR012-auth.md): may `subject` act with `relation` on `object`? OpenFGA in
// production (internal/authz), a fake in tests. nil Base.Authz => every verb is
// allowed — the single-operator mode bex ran in before authorization existed.
type Checker interface {
	Check(ctx context.Context, subject, relation, object string) (bool, error)
}

// The relations feature verbs require, matching deploy/gitops/authz/model.fga
// (Render's workspace roles). Each check targets the caller's workspace —
// workspace:tea-<id> when the control-plane store resolves the caller to a
// tenant (w1/m9), workspace:default otherwise (the platform bootstrap tenant,
// and the legacy single-tenant mode when the store is off).
const (
	RelCanView          = "can_view"            // viewer and up: lists, details, metrics
	RelCanViewLogs      = "can_view_logs"       // contributor and up (Render: viewers can't see logs)
	RelCanOperate       = "can_operate"         // contributor and up: restart/suspend/resume
	RelCanCreate        = "can_create"          // developer and up: create/delete resources
	RelCanViewSensitive = "can_view_sensitive"  // developer and up: connection strings
	RelCanManageKeys    = "can_manage_keys"     // developer and up: workspace API keys
	RelCanManageSSHKeys = "can_manage_ssh_keys" // any workspace member: their own SSH public keys
	RelCanManage        = "can_manage"          // admin only: manage the workspace itself (rename/delete)

	DefaultWorkspace = "workspace:default"
	// DefaultTenant is the tenant/workspace id used when no control-plane tenant
	// resolves for a caller (the single-workspace fallback) — the bare id form of
	// DefaultWorkspace ("workspace:" + DefaultTenant). Features that key
	// per-workspace state by tenant id (github connections, secrets) share this
	// so their fallback key can't drift.
	DefaultTenant = "default"
)

// WorkspaceObject is the OpenFGA object for a workspace (tenant) id — the target
// the workspace lifecycle verbs authorize against, e.g. workspace:tea-abc.
func WorkspaceObject(tenantID string) string { return "workspace:" + tenantID }

// LabelTenant carries a projected App CR's owning tenant id (tea-<id>) —
// stamped by internal/store's reconciler (its LabelTenant references this
// constant, single source of truth) and by apps.Create for a directly-created
// App (see GetApp). GetApp checks it against the caller's resolved tenant, so
// every feature that fetches an App by name (apps/logs/metrics/secrets)
// inherits the same cross-tenant gate from the one shared fetch.
const LabelTenant = "bex.co/tenant"

// LabelAppID carries a service's Render-shaped typed id (srv-<xid>). The
// control-plane store mints it and stamps it on projected App CRs; API-created
// Apps in store-less mode receive one too. Keeping the label key in core lets
// every feature resolve a public service id without importing the store.
const LabelAppID = "bex.co/app-id"

// LabelServiceName carries an App CR's PUBLIC name (w4/m19) — the workspace-
// scoped name a caller creates and reaches it by, as opposed to metadata.Name,
// which for a store-managed App is now CRName(tenant, name) and so no longer
// equals it. Stamped alongside LabelTenant by apps.Create and store's
// projector (single source: LabelTenant's own doc). GetApp's cross-workspace
// fallback selects on this label — the only way it can find "web" in a
// workspace other than the one its object name is prefixed with.
const LabelServiceName = "bex.co/service-name"

// LabelWorkspace carries the owning workspace (tenant) id on App CRs, Database
// CRs, and KeyValue CRs and their descendant pods. The operator reads it and
// propagates it to pod templates so NetworkPolicy selectors can express
// "same-workspace" rules without touching Deployments or StatefulSets directly.
// Value is the tenant id ("tea-<xid>") — DNS/label-safe by design (hyphen, not
// underscore). Absent on hand-applied CRs (legacy mode): those run without
// NetworkPolicy enforcement, consistent with prior behavior.
//
// Kept in sync by hand with operator's labelWorkspace — the api layer must not
// import the operator (same pattern as LabelTenant / store.LabelTenant).
const LabelWorkspace = "app.bex.co/workspace"

// LabelProject carries a Database or KeyValue CR's project assignment (w1/m31
// extension): the owning Project's id (prj-<xid>), or absent when unassigned.
// Services group via apps.project_id and the projector mirrors it onto this
// label for read-side adapters. Database/KeyValue CRs have no store row, so
// this label is their source of association —
// set/cleared by postgres.Service.SetProjectID / keyvalue.Service.SetProjectID.
const LabelProject = "app.bex.co/project-id"

// LabelEnvironment carries a Database or KeyValue CR's environment assignment
// (w6/m20 extension, LabelProject's sibling): the owning Environment's id
// (env-<xid>), or absent when unassigned. Services group via
// apps.environment_id and the projector mirrors it onto this label for
// read-side adapters. Database/KeyValue CRs have no store row, so this label
// is their source of association — set/
// cleared by postgres.Service.SetEnvironmentID / keyvalue.Service.SetEnvironmentID.
const LabelEnvironment = "app.bex.co/environment-id"

// LabelNetworkIsolation carries an App CR's environment id (w6/m19
// protected-environment ACLs), but ONLY while that environment currently has
// networkIsolationEnabled=true — unlike LabelEnvironment above (unconditional
// DB/KV environment membership, w6/m20), this label's mere PRESENCE is itself
// the signal, not just its value. Stamped/cleared by environments.Service
// whenever service membership changes (SetServices) or the isolation flag
// flips (SetACL). Read by the operator (app_controller.go's
// reconcileNetworkPolicy, kept in sync by hand, same LabelWorkspace pattern)
// to scope that App's NetworkPolicy to same-environment peers instead of
// same-workspace ones. Absent: no environment, or the environment has
// isolation off — the App keeps ordinary same-workspace connectivity.
//
// A distinct label from LabelEnvironment (not a reuse) because the two
// features that separately needed "an environment id on a label" this same
// week (w6/m19, w6/m20) landed different semantics: m20's is unconditional
// membership (App/DB/KV all belong to at most one environment, mirroring
// LabelProject); m19's is conditional on a policy flag and, today, App-only.
const LabelNetworkIsolation = "app.bex.co/network-isolation"

// Render's protectedStatus enum on an Environment (w6/m19). Live in this
// shared leaf, not the environments feature package, because apps.Service's
// destructive-verb guard (apps/protection.go) needs to compare against it too
// — a cross-feature import for two string constants would be the wrong
// direction (apps -> environments); both features importing core instead
// keeps neither depending on the other.
const (
	ProtectedStatusProtected   = "protected"
	ProtectedStatusUnprotected = "unprotected"
)

// WorkspaceResolver maps an authenticated caller to its workspace: the tenant
// id ("tea-<id>") for a tenant member or a bound API key (ok=true). ok=false
// means the store is on but the caller resolves to no tenant — an unbound
// machine key, or the platform bootstrap — and the caller is left on the
// default workspace (bootstrap is seeded admin there; a tuple-less key 403s).
// A nil Base.Workspace means the store is off: every caller stays on
// workspace:default, the legacy single-tenant mode (byte-identical to before).
//
// Tenant answers the caller's DEFAULT workspace — the oldest membership
// (store.TenantForIdentity), deterministic since w6/m14 — used when the caller
// names no workspace. IsMember answers "may this caller act in THAT workspace",
// the gate for a workspace the caller DID name (core.WithWorkspace) and for an
// App that belongs to one of the caller's other workspaces (GetApp): a caller
// belongs to many workspaces, so neither an override nor a cross-workspace App
// can be honored on the strength of the default resolution alone.
type WorkspaceResolver interface {
	Tenant(ctx context.Context, id Identity) (tenantID string, ok bool)
	IsMember(ctx context.Context, id Identity, tenantID string) (bool, error)
}

// Render's suspended enum (a string, NOT a bool) — shared by the service and
// database projections, so it lives in the kernel both features import.
const (
	RenderSuspended    = "suspended"
	RenderNotSuspended = "not_suspended"
)

// SuspendedEnum maps a bool onto Render's suspended string enum.
func SuspendedEnum(suspended bool) string {
	if suspended {
		return RenderSuspended
	}
	return RenderNotSuspended
}

// Base is the shared kernel every feature service embeds: the apiserver-thin
// client, the watched namespace, an injectable clock, and the authorization
// gate. Feature services embed *Base and call Authorize / Now / GetApp / AppPods.
type Base struct {
	Client    client.Client
	Namespace string
	// Clock supplies the current time; injectable for tests. nil => time.Now.
	Clock func() time.Time
	// Authz decides what the authenticated caller may do (OpenFGA); nil => every
	// verb allowed (pre-authorization behavior).
	Authz Checker
	// Workspace resolves the caller's tenant (workspace:tea-<id>) when the
	// control-plane store is wired; nil => every check targets workspace:default
	// (the legacy store-off mode). See WorkspaceResolver for the ok contract.
	Workspace WorkspaceResolver
	// Audit records write-verb authorization decisions (w4/m10); nil => audit.go's
	// NoopAuditSink, the store-off degrade every other store-backed feature uses.
	Audit AuditSink
}

// Now returns the (injectable) current time.
func (b *Base) Now() time.Time {
	if b.Clock != nil {
		return b.Clock()
	}
	return time.Now()
}

// Authorize gates a verb on the caller's permission against its OWN
// workspace: workspace:tea-<id> when the resolver (Workspace) finds a tenant
// for the caller, workspace:default otherwise (store off, or an unbound
// machine caller that 403s on the default workspace's tuples unless it is the
// seeded bootstrap). Every App/logs/metrics/apikeys verb starts here — it is
// the caller-scoped case of AuthorizeOn, which every one of those verbs uses
// implicitly by never naming a workspace itself. This is also the audit
// interception point (w4/m10, audit.go): the caller two frames up is the verb
// itself (docs/... every verb calls Authorize/AuthorizeOn exactly once, per
// CLAUDE.md), so it's resolved here rather than threaded through 80+ call
// sites — a write-relation verb can't opt out of being recorded.
// A caller who NAMES a workspace (core.WithWorkspace — REST/GraphQL ownerId, an
// MCP session's select_workspace) is checked against THAT workspace instead of
// their default one, but only once they are shown to be a member of it: naming a
// workspace the caller does not belong to is ErrForbidden, never a silent
// fall-back to their own (that fall-back is the confused-deputy shape — the
// caller asks for B, gets served A, and a write lands in the wrong workspace).
func (b *Base) Authorize(ctx context.Context, relation string) error {
	object, err := b.callerWorkspace(ctx)
	return b.authorizeAndAudit(ctx, relation, object, "", callerVerb(verbFrameSkip), err)
}

// AuthorizeTarget is Authorize for a verb that acts on ONE named resource: the
// same caller-workspace check, plus the target (e.g. core.ServiceTarget(name))
// recorded on the audit event. It is what makes the per-service events feed a
// view rather than a second write path (internal/events): the audit row already
// written on every write verb now also says WHICH service the verb acted on.
//
// The target is the name the caller ASKED for, resolved before the App is
// fetched — so a verb that then 404s or 403s still records its attempt, exactly
// as the audit log already does for a denied authorize. internal/events filters
// those out (allowed-only, workspace-scoped) so they cannot pollute a feed.
// Pass a resource NAME, never a value: Target is on the redacted-by-structure
// side of the audit contract (core.AuditEvent).
func (b *Base) AuthorizeTarget(ctx context.Context, relation, target string) error {
	object, err := b.callerWorkspace(ctx)
	return b.authorizeAndAudit(ctx, relation, object, target, callerVerb(verbFrameSkip), err)
}

// callerWorkspace is the OpenFGA object of the workspace the caller is acting
// in: workspace:tea-<id> for the workspace they NAMED (once membership-checked)
// or, naming none, for the one the resolver resolves them to; workspace:default
// otherwise (store off, or an unbound machine caller that 403s there unless it
// is the seeded bootstrap). The error is the membership refusal — returned
// alongside the object the caller ASKED for, so the denial is audited against
// the workspace they tried to reach rather than the one they came from.
func (b *Base) callerWorkspace(ctx context.Context) (string, error) {
	tenantID, err := b.resolveWorkspace(ctx)
	if err != nil {
		named, _ := WorkspaceFrom(ctx)
		return WorkspaceObject(named), err
	}
	if tenantID == "" {
		return DefaultWorkspace, nil
	}
	return WorkspaceObject(tenantID), nil
}

// resolveWorkspace is the ONE place a request's effective workspace is decided
// (w6/m14) — every gate on Base (Authorize, Tenant, GetApp) reads it, so the
// three surfaces cannot drift on which workspace a request acts in:
//
//   - the caller NAMED one (core.WithWorkspace): honored once IsMember confirms
//     they belong to it; a non-member is ErrForbidden and an unreachable
//     membership store is ErrAuthzUnavailable — fail closed either way, never a
//     fall-back to the caller's own workspace.
//   - the caller named none: their default workspace (the resolver's oldest
//     membership — deterministic since w6/m14's TenantForIdentity ORDER BY).
//
// "" (no error) means no workspace resolves — the store is off, the caller has
// no identity, or it is an unbound machine key / the bootstrap — which leaves
// the caller on workspace:default, exactly as before.
func (b *Base) resolveWorkspace(ctx context.Context) (string, error) {
	if b.Workspace == nil {
		return "", nil // store off: one workspace, nothing to resolve or override
	}
	id, ok := IdentityFrom(ctx)
	if !ok {
		return "", nil
	}
	tenantID, _ := b.Workspace.Tenant(ctx, id)
	named, ok := WorkspaceFrom(ctx)
	if !ok || named == tenantID {
		// Named none, or named the one they'd get anyway (the dashboard sends the
		// switcher's workspace on every create, which for most callers IS their
		// default). No membership round-trip: the default workspace is DERIVED from
		// tenant_members, so resolving to it already proves the membership.
		return tenantID, nil
	}
	if err := b.requireMember(ctx, id, named); err != nil {
		return "", err
	}
	return named, nil
}

// AuthorizeOn gates a verb on the caller's permission against a specific object
// (e.g. workspace:tea-abc) — the seam for verbs scoped to a named workspace
// rather than the caller's own (the workspaces lifecycle verbs check `admin` on
// the exact workspace being renamed/deleted, which may differ from the
// caller's default — the cross-tenant case the audit log's denial events cover
// for the RelCanManage/RelCanCreate/RelCanOperate/RelCanManageKeys verbs that
// call this directly). Same audit interception as Authorize.
func (b *Base) AuthorizeOn(ctx context.Context, relation, object string) error {
	return b.authorizeAndAudit(ctx, relation, object, "", callerVerb(verbFrameSkip), nil)
}

// AuthorizeOnTarget is AuthorizeOn for a verb that acts on ONE named
// sub-resource of the named workspace — AuthorizeTarget's explicit-object
// sibling (the members verbs act on a member/invite OF a workspace the caller
// administers, which may not be their default one). The target (e.g.
// core.MemberTarget(subject)) is recorded on the audit event exactly as
// AuthorizeTarget records a service name: the identifier the caller ASKED to
// act on, resolved before any fetch, never a verb argument's value. Calls
// callerVerb itself rather than delegating to a sibling entry point — see
// verbFrameSkip's comment for why delegation would silently rename every
// recorded verb.
func (b *Base) AuthorizeOnTarget(ctx context.Context, relation, object, target string) error {
	return b.authorizeAndAudit(ctx, relation, object, target, callerVerb(verbFrameSkip), nil)
}

// authorizeAndAudit runs the OpenFGA check and records the outcome
// (audit.go) — the one place Authorize, AuthorizeTarget and AuthorizeOn
// funnel through, so a verb is recorded exactly once regardless of which
// entry point it calls. A write relation always records (allowed or
// denied); a read relation records only when denied (audit.go's
// readRelations — an allowed read stays unrecorded, volume-prohibitive).
// resolveErr is a refusal that happened BEFORE the check (the caller named a
// workspace they are not a member of): it short-circuits the check but is
// still audited, so a cross-workspace attempt leaves the same denial trail
// an OpenFGA "no" does.
func (b *Base) authorizeAndAudit(ctx context.Context, relation, object, target, verb string, resolveErr error) error {
	return b.authorizeAndRecord(ctx, relation, object, target, verb, resolveErr, true)
}

// authorizeAndRecord is authorizeAndAudit with the DENIAL recording
// suppressible (recordDenial=false): AuthorizeApp's name-collision fallback
// loop caps how many rejected candidates it individually audits (w4/015), so
// its beyond-the-cap checks run through here without a per-candidate row. An
// ALLOWED write still always records — the write-audit invariant ("a write
// relation always records, allowed or denied") is never suppressible by
// recordDenial; the one allowed-write suppression is the caller's own
// WithDeferredAllowedWriteAudit (a composite atomic verb deferring its
// success events until the mutation lands — denials stay visible either way).
func (b *Base) authorizeAndRecord(ctx context.Context, relation, object, target, verb string, resolveErr error, recordDenial bool) error {
	err := resolveErr
	if err == nil {
		err = b.checkAuthz(ctx, relation, object)
	}
	switch {
	case err == nil:
		if writeRelations[relation] && !defersAllowedWriteAudit(ctx) {
			b.emit(ctx, verb, object, target, nil)
		}
	case recordDenial && auditsDenial(relation):
		b.emit(ctx, verb, object, target, err)
	}
	return err
}

// checkAuthz is the raw OpenFGA decision, no audit side effect: nil checker
// allows (authorization not enforced); with a checker wired, no identity in
// context or a negative check is ErrForbidden, and an unreachable checker
// fails closed with ErrAuthzUnavailable — never a pass-through, so the three
// surfaces stay authorization-identical.
func (b *Base) checkAuthz(ctx context.Context, relation, object string) error {
	if b.Authz == nil {
		return nil
	}
	id, ok := IdentityFrom(ctx)
	if !ok {
		return ErrForbidden
	}
	allowed, err := b.Authz.Check(ctx, "user:"+id.Subject, relation, object)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuthzUnavailable, err)
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

// Tenant returns the workspace this request acts in: the one the caller named
// (core.WithWorkspace), else their default (resolveWorkspace). It is what a
// create stamps its new resource with, so naming a workspace is what puts a new
// service in it. ok=false when there is no resolver (store off), the caller has
// no tenant, or the named workspace is refused; callers that filter to the
// caller's workspace check b.Workspace != nil first to distinguish "store off
// (no filter)" from "store on (filter, maybe empty)".
//
// A refused override cannot reach a verb's body: every verb calls Authorize
// first (CLAUDE.md), which fails the request with the same ErrForbidden — so
// ok=false here means "no workspace", never "a workspace you may not use".
func (b *Base) Tenant(ctx context.Context) (string, bool) {
	tenantID, err := b.resolveWorkspace(ctx)
	if err != nil || tenantID == "" {
		return "", false
	}
	return tenantID, true
}

// AuthorizeApp is the single seam for a verb scoped to ONE named App (w6/m17):
// fetch it, authorize `relation` against ITS OWN workspace once, and record
// exactly one audit event (target = core.ServiceTarget(name)). It replaces the
// old Authorize(relation) + GetApp(relation, name) pair, whose independent
// caller-workspace and resource-workspace checks made effective permission the
// INTERSECTION of two unrelated workspaces' roles (w6/013): a caller who is
// admin of their OWN workspace but was invited as a viewer elsewhere could not
// operate their own service whenever the invited workspace happened to be
// their default (the oldest membership — the common case for someone who
// signed up BECAUSE they were invited, since EnsureTenant redeems the pending
// invite before minting their personal tenant). Authorize alone could only
// ever produce a false negative here: it checked the wrong workspace.
//
// 403-before-404: TestAuthzGuardsEveryVerb sweeps every verb against an empty
// fake client and expects ErrForbidden, not ErrNotFound. A missing App has no
// workspace to derive, so it is authorized against the workspace the REQUEST
// is ACTING in (callerWorkspace) — the same object the old standalone
// Authorize call used — before reporting absence; a resource that exists is
// authorized against its OWN workspace instead (resourceWorkspace), which is
// what fixes the caller-default/resource-workspace intersection bug above.
//
// name first resolves as an exact LabelAppID (the Render-compatible srv- id),
// then follows the same three-tier name lookup as GetApp (w4/m19, see
// appCandidateNames + GetApp's own doc): the acting workspace's own object name,
// the bare name (hand-applied or pre-w4/m19 CRs), then — only if neither direct
// candidate exists — every App carrying `name` as LabelServiceName, authorized
// in turn via resourceWorkspace/authorizeAndRecord so a denied candidate still
// leaves its own audit trail (consistent with the resolveErr-before-the-check
// case above) until one the caller may access is found or the search is
// exhausted. Per-candidate denial rows are capped at
// maxFallbackCandidateAudits per request, with one aggregate row past the cap
// (w4/015 — see the constant's doc for the latency/amplification rationale).
func (b *Base) AuthorizeApp(ctx context.Context, relation, name string) (*appv1alpha1.App, error) {
	verb := callerVerb(verbFrameSkip)
	acting, actingErr := b.resolveWorkspace(ctx)
	if actingErr != nil {
		named, _ := WorkspaceFrom(ctx)
		if err := b.authorizeAndAudit(ctx, relation, WorkspaceObject(named), ServiceTarget(name), verb, actingErr); err != nil {
			return nil, err
		}
	}
	// Render clients address services by their typed srv- id. The App CR keeps
	// that public id in LabelAppID while metadata.Name may be tenant-prefixed,
	// so resolve the unique id before trying the name-compatible fallbacks.
	var byID appv1alpha1.AppList
	if err := b.Client.List(ctx, &byID,
		client.InNamespace(b.Namespace), client.MatchingLabels{LabelAppID: name}); err != nil {
		return nil, err
	}
	if len(byID.Items) > 0 {
		var lastErr error
		for i := range byID.Items {
			object, resolveErr := b.resourceWorkspaceFor(ctx, acting, actingErr, byID.Items[i].Labels)
			if err := b.authorizeAndAudit(ctx, relation, object, canonicalAppTarget(&byID.Items[i]), verb, resolveErr); err == nil {
				return &byID.Items[i], nil
			} else {
				lastErr = err
			}
		}
		return nil, lastErr
	}
	var a appv1alpha1.App
	for _, candidate := range appCandidateNames(acting, name) {
		getErr := b.Client.Get(ctx, client.ObjectKey{Namespace: b.Namespace, Name: candidate}, &a)
		if getErr == nil {
			object, resolveErr := b.resourceWorkspaceFor(ctx, acting, actingErr, a.Labels)
			if err := b.authorizeAndAudit(ctx, relation, object, canonicalAppTarget(&a), verb, resolveErr); err != nil {
				return nil, err
			}
			return &a, nil
		}
		if !apierrors.IsNotFound(getErr) {
			return nil, getErr
		}
	}
	if acting != "" {
		var list appv1alpha1.AppList
		if err := b.Client.List(ctx, &list,
			client.InNamespace(b.Namespace), client.MatchingLabels{LabelServiceName: name}); err != nil {
			return nil, err
		}
		// Every colliding candidate is a distinct authorization decision, but
		// individually auditing an unbounded number of them would let one GET
		// against a common colliding name serialize one synchronous audit
		// write (bounded by auditRecordTimeout EACH) per tenant sharing it
		// (w4/015, a concern since w4/m20/t001 widened denied-READ recording
		// to this loop). So: the first maxFallbackCandidateAudits denials
		// record per-candidate as before; the remainder are still fully
		// checked but unrecorded, and ONE aggregate denial row against the
		// caller's own workspace marks that the probe went past the cap. An
		// allowed candidate always records per the normal relation rules
		// regardless of position (authorizeAndRecord).
		//
		// BatchCheck assessment (w4/m30/t003, verify-first): NOT adopted. This
		// loop's per-candidate cost was two round trips each — the
		// Workspace.Tenant store query resourceWorkspaceFor now hoists above the
		// loop (this milestone), and one OpenFGA checkAuthz call. Only the
		// latter is a genuine per-candidate OpenFGA decision (distinct
		// resource objects, not redundant), and core.Checker's single
		// Check(ctx, relation, object) method is satisfied by fakes across
		// every feature's test suite — widening it to a batch shape would
		// ripple through the whole authz surface to save HTTP round trips on
		// a denial-path-only, name-collision-only fallback that already
		// short-circuits on the first ALLOWED candidate (the common case: a
		// caller collides with exactly one workspace they belong to) and
		// already bounds its audit-write amplification via
		// maxFallbackCandidateAudits. Revisit only if collision-heavy tenants
		// make this loop's OpenFGA round-trip count a measured latency
		// problem — it is not one today.
		var lastErr error
		denied := 0
		for i := range list.Items {
			object, resolveErr := b.resourceWorkspaceFor(ctx, acting, actingErr, list.Items[i].Labels)
			record := denied < maxFallbackCandidateAudits
			if err := b.authorizeAndRecord(ctx, relation, object, canonicalAppTarget(&list.Items[i]), verb, resolveErr, record); err == nil {
				return &list.Items[i], nil
			} else {
				lastErr = err
				denied++
			}
		}
		if lastErr != nil {
			if denied > maxFallbackCandidateAudits && auditsDenial(relation) {
				// acting was already resolved above (and is non-empty in this
				// branch) — no need for callerWorkspace's second store round-trip
				// on what is already the worst-case-latency path.
				b.emit(ctx, verb, WorkspaceObject(acting), ServiceTarget(name), lastErr)
			}
			return nil, lastErr
		}
	}
	object, resolveErr := b.callerWorkspace(ctx)
	if err := b.authorizeAndAudit(ctx, relation, object, ServiceTarget(name), verb, resolveErr); err != nil {
		return nil, err
	}
	return nil, ErrNotFound
}

// canonicalAppTarget keeps one service activity stream regardless of whether
// a client addressed the App by mutable public name, internal CR name, or its
// stable srv-… id. Store-managed Apps always carry LabelServiceName; legacy
// and hand-applied Apps fall back to metadata.name.
func canonicalAppTarget(a *appv1alpha1.App) string {
	name := a.Labels[LabelServiceName]
	if name == "" {
		name = a.Name
	}
	return ServiceTarget(name)
}

// AuthorizeDatabase is AuthorizeApp for a managed Postgres Database — same
// single seam, same 403-before-404 fallback, same resource-workspace gate. See
// AuthorizeApp's doc for the design; the two are siblings because Database (and
// KeyValue, below) carry the same core.LabelTenant contract as an App.
func (b *Base) AuthorizeDatabase(ctx context.Context, relation, name string) (*appv1alpha1.Database, error) {
	verb := callerVerb(verbFrameSkip)
	var d appv1alpha1.Database
	getErr := b.Client.Get(ctx, client.ObjectKey{Namespace: b.Namespace, Name: name}, &d)
	if apierrors.IsNotFound(getErr) {
		object, resolveErr := b.callerWorkspace(ctx)
		if err := b.authorizeAndAudit(ctx, relation, object, DatabaseTarget(name), verb, resolveErr); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	if getErr != nil {
		return nil, getErr
	}
	object, resolveErr := b.resourceWorkspace(ctx, d.Labels)
	if err := b.authorizeAndAudit(ctx, relation, object, DatabaseTarget(name), verb, resolveErr); err != nil {
		return nil, err
	}
	return &d, nil
}

// AuthorizeKeyValue is AuthorizeApp for a managed KeyValue — see AuthorizeApp
// and AuthorizeDatabase.
func (b *Base) AuthorizeKeyValue(ctx context.Context, relation, name string) (*appv1alpha1.KeyValue, error) {
	verb := callerVerb(verbFrameSkip)
	var kv appv1alpha1.KeyValue
	getErr := b.Client.Get(ctx, client.ObjectKey{Namespace: b.Namespace, Name: name}, &kv)
	if apierrors.IsNotFound(getErr) {
		object, resolveErr := b.callerWorkspace(ctx)
		if err := b.authorizeAndAudit(ctx, relation, object, KeyValueTarget(name), verb, resolveErr); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	if getErr != nil {
		return nil, getErr
	}
	object, resolveErr := b.resourceWorkspace(ctx, kv.Labels)
	if err := b.authorizeAndAudit(ctx, relation, object, KeyValueTarget(name), verb, resolveErr); err != nil {
		return nil, err
	}
	return &kv, nil
}

// resourceWorkspace is the (object, err) pair AuthorizeApp/AuthorizeDatabase/
// AuthorizeKeyValue authorize a FOUND resource against — the resource-side
// counterpart of callerWorkspace, called only once the fetch succeeds (a
// missing resource has no labels to read, so those call sites use
// callerWorkspace directly instead — its own fallback for "no workspace
// resolves" is exactly the object a missing resource should be checked
// against too, so there is nothing left for this function to add there).
//
//   - The caller named a workspace it is not a member of => the refusal from
//     resolveWorkspace, against the workspace it ASKED for (matches
//     callerWorkspace's own error branch).
//   - No workspace resolves (store off, or an unbound/identity-less caller)
//     => the DEFAULT workspace fallback — the same single check a standalone
//     Authorize call would have made; there is nothing resource-specific to
//     gate on in this mode.
//   - The resource carries no owner label at all, but the request resolved to
//     a real workspace => ErrForbidden: it belongs to no one, so it is
//     nobody's cross-workspace read either.
//   - The resource's owner IS the acting workspace => allowed, checked
//     directly (no more "trust the verb already checked" shortcut — this seam
//     IS the verb's only check now).
//   - The resource's owner is ANOTHER of the caller's workspaces => allowed
//     only if the caller is first a MEMBER there (requireMember) and `relation`
//     also holds there — an admin of A who is merely a viewer of B still
//     cannot operate B's resource by naming it.
func (b *Base) resourceWorkspace(ctx context.Context, labels map[string]string) (string, error) {
	acting, err := b.resolveWorkspace(ctx)
	return b.resourceWorkspaceFor(ctx, acting, err, labels)
}

// resourceWorkspaceFor is resourceWorkspace's body, taking the acting
// workspace (and its resolution error) as already-computed inputs rather than
// calling resolveWorkspace itself. resolveWorkspace is ctx-dependent only —
// same ctx, same result — so a caller resolving it once (e.g. AuthorizeApp's
// three candidate-collision loops, each evaluating a distinct App against the
// SAME acting workspace) and passing it into every iteration avoids N
// identical Workspace.Tenant store queries for N candidates (w4/m30).
// resourceWorkspace above remains the single-shot entry point for call sites
// with nothing to hoist (AuthorizeDatabase, AuthorizeKeyValue).
func (b *Base) resourceWorkspaceFor(ctx context.Context, acting string, actingErr error, labels map[string]string) (string, error) {
	if actingErr != nil {
		named, _ := WorkspaceFrom(ctx)
		return WorkspaceObject(named), actingErr
	}
	if acting == "" {
		return DefaultWorkspace, nil
	}
	owner := labels[LabelTenant]
	switch {
	case owner == "":
		return WorkspaceObject(acting), ErrForbidden
	case owner == acting:
		return WorkspaceObject(acting), nil
	default:
		id, _ := IdentityFrom(ctx) // present: only a resolved workspace reaches here
		if merr := b.requireMember(ctx, id, owner); merr != nil {
			return WorkspaceObject(owner), merr
		}
		return WorkspaceObject(owner), nil
	}
}

// CRName is the collision-free App CR object name for a store-managed app:
// "<tenant>-<name>" (w4/m19). All tenants' Apps share one namespace
// (BEX_API_NAMESPACE), so two tenants both naming a service "web" need
// distinct k8s object names — this is that scheme, computed the same way by
// both the apps feature's create path and store's projector (reconciler.go's
// own CRName delegates here) so a row is never independently re-created under
// a different name by whichever side reconciles it first.
func CRName(tenant, name string) string { return tenant + "-" + name }

// appCandidateNames lists the direct object names an App might be stored
// under (w4/m19), tried in order: CRName(acting, name) — a store-managed App
// created after this scheme shipped — then the bare name — a hand-applied CR
// or one projected before the scheme existed (never renamed in place). acting
// == "" (store off, no identity) skips the prefixed candidate entirely, byte-
// identical to before this milestone. Shared by GetApp and AuthorizeApp so
// the two can't drift on how a name resolves to an object.
func appCandidateNames(acting, name string) []string {
	if acting == "" {
		return []string{name}
	}
	return []string{CRName(acting, name), name}
}

// GetApp fetches one App by typed service id or name, mapping absence to
// ErrNotFound, and gates it on `relation` — the SAME relation the calling verb
// just authorized. Shared by the apps/logs/metrics/secrets/deploys/events/
// envgroups services — each needs "does this App exist / read its status"
// without reimplementing the not-found mapping, and each inherits the
// cross-workspace gate from this one fetch (a caller who knows an App name must
// not read its logs, metrics, or secrets just because apps.Service alone learned
// to check).
//
// An exact LabelAppID match is tried first. Otherwise name is resolved against
// TWO candidate object names (w4/m19): CRName(acting workspace, name) first,
// then the bare name. A store-managed App created
// after this scheme shipped is object-named the first way, so its own
// workspace's caller finds it on the first try — including at create time,
// which is what lets two workspaces both claim "web" without one shadowing
// the other. The bare-name fallback covers hand-applied CRs and any
// store-managed App projected before this scheme (never renamed in place —
// k8s objects can't be renamed; new creates alone adopt CRName). A caller with
// no resolvable acting workspace (store off, no identity) tries only the bare
// name — byte-identical to before this milestone.
//
// The gate authorizes against the App's OWN workspace, not the caller's default
// one (w6/m14) — Render's model, where a resource's permissions come from the
// owner it belongs to. That matters because a caller belongs to MANY workspaces
// and may hold a different role in each:
//
//   - The App is in the workspace the request acts in (named, or the caller's
//     default) => allowed; the verb's own Authorize already checked `relation`
//     there. This is the ordinary path.
//   - The App is in ANOTHER of the caller's workspaces => allowed only if
//     `relation` also holds THERE. Before w6/m14 this was a flat ErrForbidden,
//     which 403'd an owner reading their own App whenever the implicit
//     resolution happened to pick their other workspace (w6/m11, live). Checking
//     the relation — rather than mere membership — is what makes lifting that
//     403 safe: an admin of A who is only a viewer of B still cannot delete B's
//     App by naming it, because can_create does not hold for them in B.
//   - The App is in a workspace the caller has no relation on => ErrForbidden,
//     not ErrNotFound, matching the existing convention — "not yours," not
//     "doesn't exist," so a cross-tenant caller can't probe existence by name.
//
// A no-identity caller (the git webhook's HMAC-authenticated redeploy, which
// carries no core.Identity) and the store-off mode skip the gate — neither has a
// workspace to check against, byte-identical to before.
//
// Neither direct candidate is guaranteed to find an App in ANOTHER of the
// caller's workspaces (w4/m19): since two workspaces may now legitimately
// share a name, that App's object name is prefixed with ITS OWN tenant, not
// the acting one. So a third, final step lists every App carrying `name` as
// its LabelServiceName and authorizes each in turn, returning the first the
// caller may access — this is what still lets an owner reach her OTHER
// workspace's App by bare name with no explicit selection (w6/m11's
// guarantee), without that search ever widening what a CREATE considers a
// duplicate (Service.create scopes its own probe to the target workspace
// directly, never through GetApp, for exactly that reason).
func (b *Base) GetApp(ctx context.Context, relation, name string) (*appv1alpha1.App, error) {
	acting, err := b.resolveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	// Prefer an exact public id match. Name lookup remains below for backwards
	// compatibility with bex-native callers and hand-applied legacy CRs.
	var byID appv1alpha1.AppList
	if err := b.Client.List(ctx, &byID,
		client.InNamespace(b.Namespace), client.MatchingLabels{LabelAppID: name}); err != nil {
		return nil, err
	}
	if len(byID.Items) > 0 {
		lastErr := error(ErrNotFound)
		for i := range byID.Items {
			if err := b.AuthorizeLabeled(ctx, relation, byID.Items[i].Labels); err == nil {
				return &byID.Items[i], nil
			} else {
				lastErr = err
			}
		}
		return nil, lastErr
	}
	var a appv1alpha1.App
	for _, candidate := range appCandidateNames(acting, name) {
		err := b.Client.Get(ctx, client.ObjectKey{Namespace: b.Namespace, Name: candidate}, &a)
		if err == nil {
			if err := b.AuthorizeLabeled(ctx, relation, a.Labels); err != nil {
				return nil, err
			}
			return &a, nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	if acting == "" {
		return nil, ErrNotFound
	}
	var list appv1alpha1.AppList
	if err := b.Client.List(ctx, &list,
		client.InNamespace(b.Namespace), client.MatchingLabels{LabelServiceName: name}); err != nil {
		return nil, err
	}
	lastErr := error(ErrNotFound)
	for i := range list.Items {
		if err := b.AuthorizeLabeled(ctx, relation, list.Items[i].Labels); err == nil {
			return &list.Items[i], nil
		} else {
			lastErr = err
		}
	}
	return nil, lastErr
}

// AuthorizeLabeled is the cross-workspace gate for any tenant-labeled resource
// the caller reached BY NAME — an App (GetApp, above), a Database, a KeyValue.
// It is the one rule, in one place, so a feature cannot fetch its own CRs
// through a bare client.Get and quietly skip the workspace check that the App's
// shared fetch has always applied (postgres/keyvalue did exactly that until
// w6/m14: `GET /v1/postgres/{name}` authorized can_view against the CALLER's
// workspace and then returned whatever Database bore that name — another
// workspace's connection string included).
//
// The rule, given the resource's labels and the relation the calling verb just
// authorized:
//
//   - No workspace resolves (store off, the HMAC webhook's identity-less
//     redeploy, an unbound machine key, the bootstrap) => no gate, as before.
//   - The resource is in the workspace the request acts in => allowed; the verb's
//     own Authorize already checked `relation` there.
//   - The resource is in ANOTHER of the caller's workspaces => allowed only if
//     `relation` also holds THERE (checkWorkspaceAccess). This is what lets an
//     owner reach their second workspace's resources without a 403 (w6/m11) while
//     an admin of A who is merely a viewer of B still cannot delete B's.
//   - The resource carries no workspace at all (a hand-applied CR) => ErrForbidden
//     for any store-resolved caller: it belongs to no one, so it is nobody's.
func (b *Base) AuthorizeLabeled(ctx context.Context, relation string, labels map[string]string) error {
	acting, err := b.resolveWorkspace(ctx)
	if err != nil {
		return err
	}
	if acting == "" {
		return nil
	}
	owner := labels[LabelTenant]
	if owner == acting {
		return nil
	}
	if owner == "" {
		return ErrForbidden
	}
	return b.checkWorkspaceAccess(ctx, relation, owner)
}

// checkWorkspaceAccess is the two-gate test for acting in a workspace that is
// not the one the request resolved to: the caller must BE a member of it (the
// store, the source of truth) and must hold `relation` there (OpenFGA, the role
// model). Both, not either: with authorization off (nil Authz) the membership
// row is the only isolation left, and with the store's answer alone an admin of
// one workspace would inherit admin's verbs in every workspace they were merely
// invited to.
func (b *Base) checkWorkspaceAccess(ctx context.Context, relation, tenantID string) error {
	id, _ := IdentityFrom(ctx) // present: only a resolved workspace reaches here
	if err := b.requireMember(ctx, id, tenantID); err != nil {
		return err
	}
	return b.checkAuthz(ctx, relation, WorkspaceObject(tenantID))
}

// requireMember is the membership gate, and the ONE place its fail-closed
// contract lives: a non-member is ErrForbidden; an unreachable membership store
// is ErrAuthzUnavailable, never "not a member" and never a fall-back to the
// caller's own workspace (an outage must not silently re-route a write).
func (b *Base) requireMember(ctx context.Context, id Identity, tenantID string) error {
	member, err := b.Workspace.IsMember(ctx, id, tenantID)
	if err != nil {
		return fmt.Errorf("%w: workspace membership: %v", ErrAuthzUnavailable, err)
	}
	if !member {
		return ErrForbidden
	}
	return nil
}

// AppPods lists an App's replica pods (the controller's app.bex.co/app label) —
// the selection the logs and metrics features share.
func (b *Base) AppPods(ctx context.Context, app string) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := b.Client.List(ctx, &pods,
		client.InNamespace(b.Namespace),
		client.MatchingLabels{PodLabelApp: app}); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

// PreDeployPods lists an App's pre-deploy Job pods (the app.bex.co/predeploy
// label, w1/m33) in namespace — the logs feature reads a migration's output from
// these. namespace is the operator's build namespace (BEX_BUILD_NAMESPACE, where
// the Job runs); empty falls back to the API's own namespace, the operator's
// default when that env is unset.
func (b *Base) PreDeployPods(ctx context.Context, app, namespace string) ([]corev1.Pod, error) {
	if namespace == "" {
		namespace = b.Namespace
	}
	var pods corev1.PodList
	if err := b.Client.List(ctx, &pods,
		client.InNamespace(namespace),
		client.MatchingLabels{PodLabelPreDeploy: app}); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

// BuildPods lists an App's build pods in the operator's build namespace. Both
// the ephemeral BuildKit Job and kpack stamp app.bex.co/build=<app>, so the logs
// feature can follow either implementation without importing the operator.
func (b *Base) BuildPods(ctx context.Context, app, namespace string) ([]corev1.Pod, error) {
	if namespace == "" {
		namespace = b.Namespace
	}
	var pods corev1.PodList
	if err := b.Client.List(ctx, &pods,
		client.InNamespace(namespace),
		client.MatchingLabels{PodLabelBuild: app}); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

// DatabasePods lists the CNPG pods for a managed Postgres cluster — selected by
// the cnpg.io/cluster=<clusterName> label that CNPG stamps on every pod it owns.
// Used by the postgres logs feature (w3/m28); clusterName is the Database CR's
// metadata.name, which is also the CNPG Cluster CR name.
func (b *Base) DatabasePods(ctx context.Context, clusterName string) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := b.Client.List(ctx, &pods,
		client.InNamespace(b.Namespace),
		client.MatchingLabels{PodLabelCNPGCluster: clusterName}); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

// HostsFromURLs strips the scheme (and any trailing slash) from an App's status
// URLs — the bare-hostname vocabulary Render's `host` filter speaks. It lives here
// because logs and metrics both answer that filter's values, and features never
// import each other: one derivation, so the two surfaces can't disagree about what
// a host is.
func HostsFromURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		host := u
		if _, after, ok := strings.Cut(u, "://"); ok {
			host = after
		}
		out = append(out, strings.TrimSuffix(host, "/"))
	}
	return out
}
