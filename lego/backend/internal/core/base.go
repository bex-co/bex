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
	"fmt"
	"io"
	"log"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	pathvalidation "k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/validation"
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
	// PodLabelKeyValue + ValkeyContainer identify Valkey pods for a managed Key
	// Value store. The KeyValue reconciler stamps app.bex.co/keyvalue=<name> on
	// every StatefulSet pod template; the main Valkey server container is always
	// named "valkey". The keyvalue logs feature (w3/m30) uses these for exact
	// pod fallback and Loki instance discovery.
	PodLabelKeyValue = "app.bex.co/keyvalue"
	ValkeyContainer  = "valkey"
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

// FreshChecker is an optional Checker capability: an authoritative decision that
// bypasses the positive-check cache. Every write relation uses it at the shared
// authorization seam, while AuthorizeFreshOn remains available for sensitive
// reads and for a second sink-adjacent assertion. This closes the recurring class
// where one mutation path forgot an ad-hoc fresh check and a recently revoked
// caller rode another replica's cached allow (round-10 finding 5).
// A checker that does not implement it (e.g. a test fake with no cache) is
// already authoritative, so AuthorizeFreshOn falls back to the cached path.
type FreshChecker interface {
	CheckFresh(ctx context.Context, subject, relation, object string) (bool, error)
}

type BillingMutationGate interface {
	CheckBillingMutationAllowed(context.Context, string) error
}

// PaymentGate is the local, provider-neutral paid-intent seam. Feature
// packages depend only on this core interface; internal/billing implements it
// over the webhook-stamped control-plane marker.
type PaymentGate interface {
	RequirePaymentMethod(context.Context, string) error
}

// The relations feature verbs require, matching deploy/gitops/authz/model.fga
// (Render's workspace roles). Each check targets the caller's workspace —
// workspace:tea-<id> when the control-plane store resolves the caller to a
// tenant (w1/m9), workspace:default otherwise (the platform bootstrap tenant,
// and the legacy single-tenant mode when the store is off).
//
// Adding a RelCan… constant also requires a follow-on audit_events CHECK
// migration (0089's relation list; do not rewrite 0089 after it has shipped).
// recordAudit swallows sink errors, so a CHECK miss silently drops the row —
// TestAuditRelationCheckMatchesRelCan pins the latest CHECK to RelCanRelations.
const (
	RelCanView          = "can_view"            // viewer and up: lists, details, metrics
	RelCanViewLogs      = "can_view_logs"       // contributor and up (Render: viewers can't see logs)
	RelCanOperate       = "can_operate"         // contributor and up: restart/suspend/resume
	RelCanCreate        = "can_create"          // developer and up: create/delete resources
	RelCanViewSensitive = "can_view_sensitive"  // developer and up: connection strings
	RelCanManageKeys    = "can_manage_keys"     // developer and up: workspace API keys
	RelCanManageSSHKeys = "can_manage_ssh_keys" // any workspace member: their own SSH public keys
	RelCanManage        = "can_manage"          // admin only: manage the workspace itself (rename/delete)
	RelCanManageBilling = "can_manage_billing"  // billing or admin: customer-billing setup/portal (Render's BILLING role)

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

// LifecycleOrCreate picks a verb's relation when the SAME verb is lifecycle for
// some calls and create-like for others — a deploy trigger that may or may not
// carry an imageUrl, a cron update that may or may not carry a command, a
// Postgres patch that may or may not turn on statement logging (w1/m68).
//
// It exists to name the pattern rather than to save four lines. The relation
// must be decided from the REQUEST, before the single AuthorizeApp/
// AuthorizeDatabase fetch, because a second authorization pass on an
// already-fetched resource resolves a different workspace than the first (see
// backend/CLAUDE.md) — so every such verb has to compute it the same way, up
// front. Three verbs did that independently before this helper; the fourth
// should not have to rediscover the shape.
//
// createLike true => can_create (developer and up); false => can_operate
// (contributor and up).
func LifecycleOrCreate(createLike bool) string {
	if createLike {
		return RelCanCreate
	}
	return RelCanOperate
}

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

// LabelBlueprint records the Git-connected Blueprint that manages a Service,
// Database, or KeyValue (w8/m23): stamped on blueprint create/adopt, cleared
// on disconnect (resources become unmanaged, Render semantics). A second
// blueprint naming the same resource is refused with
// BLUEPRINT_RESOURCE_CONFLICT unless the takeover confirmation transfers it.
const LabelBlueprint = "bex.co/blueprint-id"

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
	// Billing gates only explicitly billable feature mutations. Reads and
	// payment/Portal recovery remain available while enforcement is active.
	Billing BillingMutationGate
	// Payment gates only mutations whose target tier is non-free. nil preserves
	// the pre-ADR046 behavior exactly (BEX_REQUIRE_PAYMENT_METHOD unset).
	Payment PaymentGate
	// PaymentAllPlans widens the Payment gate to every billable create/plan
	// change, free tier included (ADR075 D7, BEX_REQUIRE_PAYMENT_METHOD=all):
	// RequirePlanBilling then consults the marker regardless of PaidPlan.
	// Meaningless while Payment is nil.
	PaymentAllPlans bool
	// PlatformClients proves an OAuth client id is one bex provisioned itself;
	// the composition root wires it from operator-owned configuration. Consumed by
	// AuthorizeMintClass; nil => a delegated (OAuth) caller can never pass that
	// gate — fail closed (codex round-7 F3).
	PlatformClients PlatformClientResolver
	// EnsureNamespaces synchronously provisions one workspace's per-tenant
	// namespaces (hosting + sandbox, ADR043) on demand; the composition root
	// wires it to store.NamespaceReconciler.EnsureWorkspace. Consumed by
	// EnsureWorkspaceNamespace; nil => creates race the async reconciler
	// (pre-w2/026 behavior).
	EnsureNamespaces NamespaceEnsurer
}

// NamespaceEnsurer provisions one workspace's per-tenant namespaces
// synchronously — the on-demand twin of the NamespaceReconciler's resync loop.
type NamespaceEnsurer func(ctx context.Context, workspaceID string) error

// EnsureWorkspaceNamespace guarantees a workspace's hosting namespace exists
// before a create writes into it. The NamespaceReconciler is level-triggered
// on a resync period and nothing kicks it on workspace mint, so a first
// service/postgres/keyvalue create within one resync of onboarding otherwise
// fails with a namespace NotFound from the API server, surfacing as a 500
// (w2/026). One cheap GET fast-paths the steady state; only a missing
// namespace pays for the synchronous ensure. A nil ensurer (store off, tests)
// or an empty/default tenant is a no-op, and a non-NotFound GET error is
// ignored so the create itself surfaces the real failure exactly as before.
func (b *Base) EnsureWorkspaceNamespace(ctx context.Context, tenantID string) error {
	if b.EnsureNamespaces == nil || tenantID == "" || tenantID == DefaultTenant {
		return nil
	}
	err := b.Client.Get(ctx, client.ObjectKey{Name: b.TenantNamespace(tenantID)}, &corev1.Namespace{})
	if err == nil || !apierrors.IsNotFound(err) {
		return nil
	}
	return b.EnsureNamespaces(ctx, tenantID)
}

// TenantNamespace returns the namespace a workspace's hosting workloads live
// in — store.WorkspaceNamespace(tenantID), which is the workspace id itself
// (ADR043 D1) — so every App / Database / KeyValue read/write and pod lookup
// must resolve there. An empty or default tenant (store off / unbound caller)
// has no workspace namespace to resolve, so it maps to b.Namespace instead.
//
// This covers datastores as well as Apps since ADR043 D8 (w7/m77). Until then
// only Apps moved, which silently broke every fromDatabase/fromService link: a
// secretKeyRef resolves same-namespace only, the tenant default-deny grants no
// path out of the namespace, and CNPG's bare-Service-name host does not resolve
// across one. Co-location is what fixes all three.
func (b *Base) TenantNamespace(tenantID string) string {
	if tenantID != "" && tenantID != DefaultTenant {
		return tenantID
	}
	return b.Namespace
}

// AppNamespace is TenantNamespace under its historical name. Kept because the
// App-side call sites read naturally as "the App's namespace"; new datastore
// call sites should say TenantNamespace, which is what it has always computed.
func (b *Base) AppNamespace(tenantID string) string { return b.TenantNamespace(tenantID) }

// DatastoreListOptions scopes a Database/KeyValue List to one workspace.
//
// For a resolved workspace the list runs CLUSTER-WIDE with a server-side
// LabelTenant selector rather than being pinned to a namespace — the same shape
// App lists take, and for the same reason (see the note above AuthorizeApp):
// under ADR043 D8 a workspace's datastores are in its own namespace, while
// un-migrated ones are still in the shared one, so no single namespace holds
// them all. The label selector, not the namespace, is what enforces the tenant
// boundary here — and it is applied by the API server, not in memory.
//
// With no workspace resolved (store off / unbound caller) there are no tenant
// namespaces at all, so the list stays scoped to the shared namespace exactly
// as before.
func (b *Base) DatastoreListOptions(tenantID string) []client.ListOption {
	if tenantID == "" || tenantID == DefaultTenant {
		return []client.ListOption{client.InNamespace(b.Namespace)}
	}
	return []client.ListOption{client.MatchingLabels{LabelTenant: tenantID}}
}

// DatastoreNamespaces returns the namespaces a Database/KeyValue CR may live
// in, most-likely first: the acting workspace's own hosting namespace (ADR043
// D8), then the shared apps namespace where every datastore lived before D8 and
// where un-migrated ones still do.
//
// Resolution is by lookup rather than computation — the same choice the App
// path makes (see appNamespaceByName) — so a not-yet-migrated datastore keeps
// working through the identical code path, with no `if legacy` branch anywhere
// and no flag day. See docs/runbooks/datastore-namespace-cutover.md.
func (b *Base) DatastoreNamespaces(tenantID string) []string {
	own := b.TenantNamespace(tenantID)
	if own == b.Namespace {
		return []string{b.Namespace}
	}
	return []string{own, b.Namespace}
}

// DatastoreInOwnWorkspaceNamespace is the datastore twin of
// AppInOwnWorkspaceNamespace: a labeled Database/KeyValue CR is in its canonical
// ADR043 D8 placement when its namespace IS its workspace id (the non-default
// branch of TenantNamespace). A CR that carries a workspace label but sits
// elsewhere is a stray — the cutover (docs/runbooks/datastore-namespace-cutover.md)
// deliberately leaves such a twin behind in the shared namespace between its
// Step 5 and Step 9. An unlabeled CR (hand-applied, store-off) has no canonical
// namespace and is never demoted.
func DatastoreInOwnWorkspaceNamespace(obj client.Object) bool {
	ws := obj.GetLabels()[LabelTenant]
	return ws == "" || obj.GetNamespace() == ws
}

// DedupDatabaseTwins collapses the same-name Database duplicates the ADR043 D8
// cutover leaves behind: between Step 5 and Step 9 of the cutover runbook a
// workspace legitimately has TWO Database CRs with the same metadata.name — the
// live one in its own `<ws>` namespace, the stale one in the shared namespace —
// and every label-scoped List (DatastoreListOptions) returns both. Keep one
// item per metadata.name, preferring the copy in its own workspace namespace,
// first-seen otherwise, and log the stray (the nag is the point — the loser
// needs the runbook's manual removal). The same policy store's indexManagedApps
// applies to App CRs; both sides must agree on which twin is live.
func DedupDatabaseTwins(items []appv1alpha1.Database) []appv1alpha1.Database {
	return dedupDatastoreTwins("Database", items)
}

// DedupKeyValueTwins is DedupDatabaseTwins' KeyValue twin.
func DedupKeyValueTwins(items []appv1alpha1.KeyValue) []appv1alpha1.KeyValue {
	return dedupDatastoreTwins("KeyValue", items)
}

func dedupDatastoreTwins[T any, PT interface {
	*T
	client.Object
}](kind string, items []T) []T {
	byName := make(map[string]int, len(items))
	out := make([]T, 0, len(items))
	for i := range items {
		candidate := PT(&items[i])
		prev, dup := byName[candidate.GetName()]
		if !dup {
			byName[candidate.GetName()] = len(out)
			out = append(out, items[i])
			continue
		}
		keep, drop := PT(&out[prev]), candidate
		if !DatastoreInOwnWorkspaceNamespace(keep) && DatastoreInOwnWorkspaceNamespace(drop) {
			keep, drop = drop, keep
		}
		log.Printf("core: duplicate %s CRs named %s: keeping %s/%s, ignoring stray %s/%s — "+
			"an ADR043 D8 cutover leftover; remove it per docs/runbooks/datastore-namespace-cutover.md",
			kind, candidate.GetName(), keep.GetNamespace(), keep.GetName(), drop.GetNamespace(), drop.GetName())
		out[prev] = *keep
	}
	return out
}

// An App List that must reach ANY workspace (an exact srv-id, a service-name
// collision search, a host-claim or deploy-hook-token sweep) always runs
// cluster-wide (label-filtered by the caller), exactly as the projector does
// (store/reconciler.go): App CRs are spread across every workspace's own
// namespace (ADR043), and bex-api's role grants cluster-wide App list for
// precisely this reason — so such a List call takes no namespace option.

func (b *Base) RequireBillingMutation(ctx context.Context, workspaceID string) error {
	if b == nil || b.Billing == nil || workspaceID == "" || workspaceID == DefaultTenant {
		return nil
	}
	err := b.Billing.CheckBillingMutationAllowed(ctx, workspaceID)
	if !errors.Is(err, ErrBillingEnforced) {
		return err
	}
	// Preserve an already-coded gate implementation; normalize the store's
	// historical plain sentinel so every caller gets the same transport code.
	var coded *CodedError
	if errors.As(err, &coded) {
		return err
	}
	return NewBillingEnforcedError()
}

// RequirePaymentMethod consults the injected local marker gate. A configured
// gate also sees an unexpectedly unresolved workspace and fails closed; the
// store-off compatibility path remains byte-identical because its gate is nil.
func (b *Base) RequirePaymentMethod(ctx context.Context, workspaceID string) error {
	if b == nil || b.Payment == nil {
		return nil
	}
	return b.Payment.RequirePaymentMethod(ctx, workspaceID)
}

// RequirePlanBilling is the paid-intent gate every billable create and plan
// change runs: a non-free plan additionally requires a bound payment method
// (ADR046) — or ANY plan when PaymentAllPlans widens the gate (ADR075 D7) —
// and every mutation is refused while dunning enforcement is active. Both
// checks in one seam so a new billable resource kind cannot wire only one of
// them — the drift class this exists to close.
func (b *Base) RequirePlanBilling(ctx context.Context, workspaceID, plan string) error {
	if PaidPlan(plan) || (b != nil && b.PaymentAllPlans) {
		if err := b.RequirePaymentMethod(ctx, workspaceID); err != nil {
			return err
		}
	}
	return b.RequireBillingMutation(ctx, workspaceID)
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
// MCP's per-call workspaceId) is checked against THAT workspace instead of
// their default one, but only once they are shown to be a member of it: naming a
// workspace the caller does not belong to is ErrForbidden, never a silent
// fall-back to their own (that fall-back is the confused-deputy shape — the
// caller asks for B, gets served A, and a write lands in the wrong workspace).
func (b *Base) Authorize(ctx context.Context, relation string) error {
	object, err := b.callerWorkspace(ctx)
	return b.authorizeAndAudit(ctx, relation, object, "", callerVerb(verbFrameSkip), err)
}

// AuthorizeTarget is Authorize for a verb that acts on one named
// sub-resource of the caller's acting workspace. It preserves Authorize's
// named-workspace membership check while recording the non-secret resource
// identity in the audit event. Use AuthorizeOnTarget only when the caller is
// authorizing an explicitly supplied OpenFGA object rather than the acting
// workspace resolved from context.
func (b *Base) AuthorizeTarget(ctx context.Context, relation, target string) error {
	object, err := b.callerWorkspace(ctx)
	return b.authorizeAndAudit(ctx, relation, object, target, callerVerb(verbFrameSkip), err)
}

// Can reports whether the caller holds relation on their acting workspace,
// without Authorize's audit side effect. It is a response-shaping probe for
// optional sensitive fields (a viewer listing blueprints did not ASK for the
// manifest, so the missing capability is not a recordable denial) — never a
// verb gate: a verb still opens with Authorize/AuthorizeApp. Fails closed: a
// resolution or checker error reads as "no". CanDecision (decision.go) is the
// same probe with the outcome preserved for callers that must tell a refusal
// from an unanswerable check.
func (b *Base) Can(ctx context.Context, relation string) bool {
	return b.CanDecision(ctx, relation).Allowed()
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
// resolveWorkspace returns the workspace this request acts in: the caller's
// default, or the one they explicitly named. It is the ONE place a request's
// effective workspace is decided (w6/m14) — every gate on Base reads it, so the
// three surfaces cannot drift on which workspace a request acts in.
//
// The result is memoized in the context: callers that need to resolve once and
// then evaluate many candidates (GetApp's name-collision fallback loop) should
// call resolveWorkspaceMemo instead and propagate the returned context, so each
// downstream AuthorizeLabeled/resourceWorkspaceFor call reuses the same result
// without redundant Workspace.Tenant / IsMember round trips (w4/027).
func (b *Base) resolveWorkspace(ctx context.Context) (string, error) {
	if cached, ok := resolvedWorkspaceFrom(ctx); ok {
		return cached.acting, cached.err
	}
	return b.resolveWorkspaceUncached(ctx)
}

// ValidateNamedWorkspace verifies the membership behind an adapter-supplied
// explicit workspace before a typed handler runs. It deliberately does
// nothing when no workspace was named: the handler will resolve the caller's
// normal default through the same Base path. An explicit workspace cannot be
// validated in store-off mode and therefore fails closed.
func (b *Base) ValidateNamedWorkspace(ctx context.Context) error {
	if _, named := WorkspaceFrom(ctx); !named {
		return nil
	}
	if b == nil || b.Workspace == nil {
		return ErrAuthzUnavailable
	}
	_, err := b.resolveWorkspace(ctx)
	return err
}

// resolveWorkspaceMemo is resolveWorkspace plus context memoization. Use it when
// the resolved workspace must be reused across multiple downstream calls in the
// same request.
func (b *Base) resolveWorkspaceMemo(ctx context.Context) (context.Context, string, error) {
	if cached, ok := resolvedWorkspaceFrom(ctx); ok {
		return ctx, cached.acting, cached.err
	}
	acting, err := b.resolveWorkspaceUncached(ctx)
	return withResolvedWorkspace(ctx, acting, err), acting, err
}

func (b *Base) resolveWorkspaceUncached(ctx context.Context) (string, error) {
	if b.Workspace == nil {
		return "", nil // store off: one workspace, nothing to resolve or override
	}
	if id, ok := IdentityFrom(ctx); ok {
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
	// Platform background caller (w1/m69): no request identity, but the work was
	// triggered by a durable store row naming its workspace. Honor the acting
	// tenant — the value is server-derived, so it needs no membership round-trip —
	// but only when it agrees with (or stands in for) the named workspace; a
	// disagreement is a programming error and fails closed rather than silently
	// picking one.
	if acting, ok := ActingTenantFrom(ctx); ok {
		if named, namedOK := WorkspaceFrom(ctx); namedOK && named != acting {
			return "", fmt.Errorf("%w: acting tenant %q conflicts with named workspace %q", ErrAuthzUnavailable, acting, named)
		}
		return acting, nil
	}
	return "", nil
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
	// OAuth capability first, even when membership already refused: a narrowed
	// token must not reach OpenFGA, and the insufficient-scope code must not
	// depend on whether the named workspace exists.
	if err := checkCapability(ctx, relation); err != nil {
		if recordDenial && auditsDenial(relation) {
			b.emit(ctx, verb, relation, object, target, err)
		}
		return err
	}
	err := resolveErr
	if err == nil {
		// Mutations never consume a positive cache entry. Centralizing this at the
		// same relation classification that drives write auditing prevents new
		// service verbs from silently reopening the cross-replica revocation
		// window; explicit sink-adjacent AuthorizeFresh calls remain defense in
		// depth for especially sensitive operations.
		if writeRelations[relation] {
			err = b.checkAuthzFresh(ctx, relation, object)
		} else {
			err = b.checkAuthz(ctx, relation, object)
		}
	}
	switch {
	case err == nil:
		if writeRelations[relation] && !defersAllowedWriteAudit(ctx) {
			b.emit(ctx, verb, relation, object, target, nil)
		}
	case recordDenial && auditsDenial(relation):
		b.emit(ctx, verb, relation, object, target, err)
	}
	return err
}

// checkCapability is the OAuth half of every authorization path. A denial is
// the shared insufficient-scope error; OpenFGA is not consulted.
func checkCapability(ctx context.Context, relation string) error {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil
	}
	return id.RequireCapability(relation)
}

// checkAuthz is the raw OpenFGA decision, no audit side effect: nil checker
// allows (authorization not enforced); with a checker wired, no identity in
// context or a negative check is ErrForbidden, and an unreachable checker
// fails closed with ErrAuthzUnavailable — never a pass-through, so the three
// surfaces stay authorization-identical.
func (b *Base) checkAuthz(ctx context.Context, relation, object string) error {
	if err := checkCapability(ctx, relation); err != nil {
		return err
	}
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

// checkAuthzFresh is checkAuthz that bypasses the positive-check cache when the
// wired checker supports it (FreshChecker); otherwise it degrades to the cached
// checkAuthz (an uncached checker is already authoritative). Same fail-closed
// contract: nil checker allows, no identity or a negative check is ErrForbidden,
// an unreachable checker is ErrAuthzUnavailable.
func (b *Base) checkAuthzFresh(ctx context.Context, relation, object string) error {
	if err := checkCapability(ctx, relation); err != nil {
		return err
	}
	fresh, ok := b.Authz.(FreshChecker)
	if !ok {
		return b.checkAuthz(ctx, relation, object)
	}
	id, ok := IdentityFrom(ctx)
	if !ok {
		return ErrForbidden
	}
	allowed, err := fresh.CheckFresh(ctx, "user:"+id.Subject, relation, object)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuthzUnavailable, err)
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

// AuthorizeFreshOn re-checks relation against a specific object with an
// authoritative (uncached) decision and NO audit side effect. A security-
// critical issuance verb calls it AFTER its normal auditing Authorize/AuthorizeOn
// so a caller whose membership or key was revoked within the last PositiveTTL
// cannot ride a stale positive to mint durable replacement access (round-5
// finding 4). The preceding Authorize already recorded the verb, so this second
// gate stays audit-silent; only its allow/deny decision matters.
func (b *Base) AuthorizeFreshOn(ctx context.Context, relation, object string) error {
	// checkAuthzFresh already allows a nil checker (its FreshChecker assertion
	// fails and it falls back to checkAuthz, which returns nil) — no guard here.
	return b.checkAuthzFresh(ctx, relation, object)
}

// AuthorizeFresh is AuthorizeFreshOn against the caller's own workspace — the
// object a plain Authorize resolves. A workspace-resolution error is returned as
// a refusal, matching Authorize.
func (b *Base) AuthorizeFresh(ctx context.Context, relation string) error {
	object, err := b.callerWorkspace(ctx)
	if err != nil {
		return err
	}
	return b.checkAuthzFresh(ctx, relation, object)
}

// AuthorizeAppFresh reasserts a relation against an already-authorized App's
// own workspace without another fetch or audit event. Credential reveal/mint
// verbs call this immediately before their irreversible sink so a positive
// decision cached by another API replica cannot survive membership revocation.
func (b *Base) AuthorizeAppFresh(ctx context.Context, relation string, app *appv1alpha1.App) error {
	if app == nil {
		return ErrForbidden
	}
	return b.authorizeLabeledFresh(ctx, relation, app.Labels)
}

// authorizeLabeledFresh is the uncached re-check the three Fresh seams share:
// resolve the resource's OWN workspace from its labels, then decide against it
// bypassing the decision cache. Held in one place for the same reason
// authorizeDatastore is — resolving against the caller's workspace instead of
// the resource's would reintroduce the w6/m17 intersection bug, and that
// mistake neither fails to compile nor shows up in a diff of one sibling.
func (b *Base) authorizeLabeledFresh(ctx context.Context, relation string, labels map[string]string) error {
	object, err := b.resourceWorkspace(ctx, labels)
	if err != nil {
		return err
	}
	return b.checkAuthzFresh(ctx, relation, object)
}

// AuthorizeDatabaseFresh is AuthorizeAppFresh for a managed Database — the
// datastore credential sinks (connection-info reveals, login minting) reassert
// against the Database's own workspace, uncached (codex round-8 #8).
func (b *Base) AuthorizeDatabaseFresh(ctx context.Context, relation string, d *appv1alpha1.Database) error {
	if d == nil {
		return ErrForbidden
	}
	return b.authorizeLabeledFresh(ctx, relation, d.Labels)
}

// AuthorizeKeyValueFresh is AuthorizeDatabaseFresh for a managed KeyValue.
func (b *Base) AuthorizeKeyValueFresh(ctx context.Context, relation string, kv *appv1alpha1.KeyValue) error {
	if kv == nil {
		return ErrForbidden
	}
	return b.authorizeLabeledFresh(ctx, relation, kv.Labels)
}

// AuthorizeMintClass gates a durable-credential mint verb (API-key creation,
// SSH-key enrollment) on the caller's CREDENTIAL CLASS, not a relation: the
// minted key later authenticates independently, so whatever mints it must
// itself be an authority that does not silently outlive revocation. Only a
// direct Kratos session, or a human OAuth token from a client bex provisioned
// itself (PlatformClients), passes (codex round-7 F3) — a machine
// (client_credentials) token must not self-replicate, and a consented
// third-party client must not persist past its consent. Call it BEFORE the
// verb's auditing Authorize: a class refusal is not a workspace decision and
// deliberately leaves no event, the same way an unauthenticated request does.
func (b *Base) AuthorizeMintClass(ctx context.Context) error {
	if validate, ok := FreshBearerValidatorFrom(ctx); ok {
		if err := validate(ctx); err != nil {
			return err
		}
	}
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil // no identity to classify; the verb's Authorize denies anyway
	}
	switch id.Method {
	case "session":
		// The session method exists only for a direct human Kratos login.
		return nil
	case "oauth2":
		if !id.Human {
			return ErrForbidden
		}
		if !HasGranularCapability(id.CanonicalScopes) {
			return NewInsufficientScopeError("")
		}
		if b.PlatformClients == nil {
			return ErrForbidden // trust cannot be established — fail closed
		}
		// Use the resolver's strongest path. The current operator-owned registry is
		// immutable, so it has no upstream marker or positive cache to go stale.
		platform, err := b.PlatformClients.IsPlatformClientFresh(ctx, id.ClientID)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrAuthzUnavailable, err)
		}
		if !platform {
			return ErrForbidden
		}
		return nil
	default:
		return ErrForbidden
	}
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

// WorkspaceOrDefault is Tenant collapsed to the single-workspace default when
// the caller has none — the form a verb wants when it must name SOME workspace
// to scope a store row by, rather than branching on absence.
func (b *Base) WorkspaceOrDefault(ctx context.Context) string {
	if tenantID, ok := b.Tenant(ctx); ok {
		return tenantID
	}
	return DefaultTenant
}

// AppWorkspace resolves the workspace whose GitHub connection owns an App's
// clone token. The App's OWN tenant label wins over the caller's: a trigger
// carrying no identity (the push webhook, a deploy hook) must still resolve the
// App's real connection instead of falling back to the default workspace.
func (b *Base) AppWorkspace(ctx context.Context, a *appv1alpha1.App) string {
	if t := a.Labels[LabelTenant]; t != "" {
		return t
	}
	return b.WorkspaceOrDefault(ctx)
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
// NotFoundIfDeleting implements the m81 read contract: once an App's deletion
// has been accepted (Kubernetes has stamped a DeletionTimestamp), every by-id
// tenant read reports it as absent — ErrNotFound, indistinguishable from a
// service that never existed — so the detail surfaces agree with List (which
// already drops deleting Apps) and with Render (a deleted service reaches GET
// 404). This is what keeps a by-id read from advertising a withdrawn URL while
// the operator's finalizer tears down the App's S3/TLS/registry state
// (docs/ADR006-bex-api.md § Reads while a deletion finalizes). Read verbs call
// it right after AuthorizeApp; WRITE verbs deliberately do not — a terminating
// resource still needs its finalizer-safe, operator-side teardown, and the
// backend never legitimately mutates one.
func NotFoundIfDeleting(a *appv1alpha1.App) error {
	if a == nil || a.DeletionTimestamp.IsZero() {
		return nil
	}
	return ErrNotFound
}

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
	// Legacy CR names may exceed a label's length limit. Skip only the label
	// lookups for invalid values; direct name resolution and authz still apply.
	validLabel := len(validation.IsValidLabelValue(name)) == 0
	var byID appv1alpha1.AppList
	if validLabel {
		if err := b.Client.List(ctx, &byID,
			client.MatchingLabels{LabelAppID: name}); err != nil {
			return nil, err
		}
	}
	preferOwnWorkspaceNamespace(byID.Items)
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
		if len(pathvalidation.IsValidPathSegmentName(candidate)) != 0 {
			continue // client-go rejects these names before reaching the apiserver
		}
		getErr := b.Client.Get(ctx, client.ObjectKey{Namespace: b.AppNamespace(acting), Name: candidate}, &a)
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
	if acting != "" && validLabel {
		var list appv1alpha1.AppList
		if err := b.Client.List(ctx, &list,
			client.MatchingLabels{LabelServiceName: name}); err != nil {
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
				b.emit(ctx, verb, relation, WorkspaceObject(acting), ServiceTarget(name), lastErr)
			}
			if !errors.Is(lastErr, ErrForbidden) {
				return nil, lastErr // an authz outage is not absence — fail closed
			}
			// codex round-7 F8: every workspace running an App with this NAME
			// denied the caller. Names are the one GUESSABLE lookup key (typed
			// ids are opaque), so this tier collapses to the absent-resource
			// answer — authorize against the acting workspace (already resolved
			// above; no second store round-trip on the worst-case path,
			// w4/m30/t003), then ErrNotFound — and a by-name probe cannot
			// distinguish "a foreign workspace runs `web`" from "nobody does".
			// The per-candidate denial rows above keep the forensic trail. The
			// typed-id and direct-candidate tiers above keep their 403: their
			// inputs are unguessable, and the distinction tells a legitimate
			// dual-member caller their access was the problem, not the resource.
			if err := b.authorizeAndAudit(ctx, relation, WorkspaceObject(acting), ServiceTarget(name), verb, nil); err != nil {
				return nil, err
			}
			return nil, ErrNotFound
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
// stable srv-… id. The CR name is namespace-unique; LabelServiceName is only
// workspace-unique, so using it here would merge every tenant's "web" audit
// rows whenever an unbound platform caller writes from workspace:default.
func canonicalAppTarget(a *appv1alpha1.App) string {
	return ServiceTarget(a.Name)
}

// AuthorizeDatabase is AuthorizeApp for a managed Postgres Database — same
// single seam, same 403-before-404 fallback, same resource-workspace gate. See
// AuthorizeApp's doc for the design; the two are siblings because Database (and
// KeyValue, below) carry the same core.LabelTenant contract as an App.
func (b *Base) AuthorizeDatabase(ctx context.Context, relation, name string) (*appv1alpha1.Database, error) {
	return authorizeDatastore(b, ctx, relation, DatabaseTarget(name), callerVerb(verbFrameSkip),
		func(ctx context.Context) (*appv1alpha1.Database, error) { return b.findDatabase(ctx, name) })
}

// AuthorizeKeyValue is AuthorizeApp for a managed KeyValue — see AuthorizeApp
// and AuthorizeDatabase.
func (b *Base) AuthorizeKeyValue(ctx context.Context, relation, name string) (*appv1alpha1.KeyValue, error) {
	return authorizeDatastore(b, ctx, relation, KeyValueTarget(name), callerVerb(verbFrameSkip),
		func(ctx context.Context) (*appv1alpha1.KeyValue, error) { return b.findKeyValue(ctx, name) })
}

// authorizeDatastore is the authorization POLICY the two datastore seams share,
// held in one place so it cannot drift between them: authorize a miss against
// the CALLER's workspace and only then report absence (403-before-404), and
// authorize a hit against the RESOURCE's own workspace (the w6/m17 fix — see
// AuthorizeApp). Both arms record exactly one audit event against target.
//
// It is one function rather than two copies because every line of it is a
// security decision: a copy that authorized the not-found arm against the
// resource's workspace would leak existence, and one that authorized the found
// arm against the caller's would reintroduce the two-workspace intersection
// bug. Neither mistake fails to compile, and neither is visible in a diff of
// the sibling alone. find, target, and the returned type are the only things
// that legitimately differ, so they are the only things passed in.
//
// find must return a non-nil object whenever it returns a nil error (both
// finders do): the hit arm reads the object's labels to decide WHICH workspace
// authorizes, so a (nil, nil) result would be an unauthorized read, not a
// missing one. It panics rather than silently falling back — the same property
// the inlined `d.Labels` had before the fold.
//
// verb is a parameter, not derived here: see verbFrameSkip.
func authorizeDatastore[PT client.Object](b *Base, ctx context.Context, relation, target, verb string,
	find func(context.Context) (PT, error)) (PT, error) {
	var zero PT
	// Resolve the acting workspace ONCE for the whole seam. find resolves it to
	// pick namespace candidates and the arms below resolve it again to decide
	// which workspace authorizes; same ctx, same answer, but the tenant resolver
	// deliberately does not cache NEGATIVES, so an unbound machine caller
	// otherwise pays two identical TenantForIdentity queries on every managed
	// Postgres/KeyValue request. Memoizing (rather than threading the value)
	// keeps callerWorkspace/resourceWorkspace on their normal paths — the
	// GetApp precedent, w4/027.
	ctx, _, _ = b.resolveWorkspaceMemo(ctx)
	obj, getErr := find(ctx)
	if apierrors.IsNotFound(getErr) {
		object, resolveErr := b.callerWorkspace(ctx)
		if err := b.authorizeAndAudit(ctx, relation, object, target, verb, resolveErr); err != nil {
			return zero, err
		}
		return zero, ErrNotFound
	}
	if getErr != nil {
		return zero, getErr
	}
	object, resolveErr := b.resourceWorkspace(ctx, obj.GetLabels())
	if err := b.authorizeAndAudit(ctx, relation, object, target, verb, resolveErr); err != nil {
		return zero, err
	}
	return obj, nil
}

// findDatabase locates a Database CR by its (globally unique) name without
// assuming which namespace it lives in — the acting workspace's own namespace
// first, then the shared apps namespace, then a cluster-wide sweep for the
// cross-workspace case a multi-workspace member can legitimately reach.
//
// The sweep is last, not first: it is the only unindexed read here, and the two
// direct candidates answer every request except an explicit cross-workspace
// one. This is the same shape AuthorizeApp uses (direct candidates, then a
// cluster-wide fallback), for the same reason.
//
// Returns a NotFound error when no CR matches, so callers keep their existing
// 403-before-404 handling unchanged.
func (b *Base) findDatabase(ctx context.Context, name string) (*appv1alpha1.Database, error) {
	var d appv1alpha1.Database
	found, err := b.getInDatastoreNamespaces(ctx, name, &d)
	if err != nil {
		return nil, err
	}
	if found {
		return &d, nil
	}
	var list appv1alpha1.DatabaseList
	if err := b.Client.List(ctx, &list); err != nil {
		return nil, err
	}
	for i := range list.Items {
		if list.Items[i].Name == name {
			return &list.Items[i], nil
		}
	}
	return nil, notFoundFor("databases", name)
}

// findKeyValue is findDatabase for a managed KeyValue — see its doc.
func (b *Base) findKeyValue(ctx context.Context, name string) (*appv1alpha1.KeyValue, error) {
	var kv appv1alpha1.KeyValue
	found, err := b.getInDatastoreNamespaces(ctx, name, &kv)
	if err != nil {
		return nil, err
	}
	if found {
		return &kv, nil
	}
	var list appv1alpha1.KeyValueList
	if err := b.Client.List(ctx, &list); err != nil {
		return nil, err
	}
	for i := range list.Items {
		if list.Items[i].Name == name {
			return &list.Items[i], nil
		}
	}
	return nil, notFoundFor("keyvalues", name)
}

// getInDatastoreNamespaces is the direct-candidate half of findDatabase and
// findKeyValue: try name in each namespace a datastore may live in (the acting
// workspace's own, then the shared apps namespace), reporting found=false only
// once every candidate came back NotFound. A non-NotFound error stops the
// sweep — an unreachable API server is not an absent resource.
//
// The cluster-wide sweep stays in each caller: it needs the typed List, and
// keeping it there is what makes the "indexed reads first, unindexed sweep
// last" ordering visible at the call site where the cost lives.
func (b *Base) getInDatastoreNamespaces(ctx context.Context, name string, out client.Object) (bool, error) {
	acting, _ := b.resolveWorkspace(ctx)
	for _, ns := range b.DatastoreNamespaces(acting) {
		err := b.Client.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, out)
		if err == nil {
			return true, nil
		}
		if !apierrors.IsNotFound(err) {
			return false, err
		}
	}
	return false, nil
}

// notFoundFor builds the apierrors NotFound the direct Get would have returned,
// so a caller's apierrors.IsNotFound check behaves identically whether the miss
// came from a Get or from the cluster-wide sweep.
func notFoundFor(resource, name string) error {
	return apierrors.NewNotFound(appv1alpha1.SchemeGroupVersion.WithResource(resource).GroupResource(), name)
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
// with nothing to hoist; the datastore seams reach it through resourceWorkspace
// but pay only one resolution, because authorizeDatastore memoizes the acting
// workspace into the ctx first.
func (b *Base) resourceWorkspaceFor(ctx context.Context, acting string, actingErr error, labels map[string]string) (string, error) {
	if actingErr != nil {
		named, _ := WorkspaceFrom(ctx)
		return WorkspaceObject(named), actingErr
	}
	if acting == "" {
		return DefaultWorkspace, nil
	}
	owner := labels[LabelTenant]
	if named, explicit := WorkspaceFrom(ctx); explicit && owner != "" && owner != named {
		return WorkspaceObject(named), ErrForbidden
	}
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
//     not ErrNotFound — "not yours," not "doesn't exist." For the typed-id and
//     direct-candidate tiers this stays 403 (their inputs are unguessable, and
//     the distinction tells a legitimate caller their access was the problem);
//     the by-NAME label sweep instead collapses an all-denied probe to
//     ErrNotFound (codex round-7 F8) — names like "web" are the one guessable
//     lookup key, and there a foreign-existence oracle is worth closing.
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
	ctx, acting, err := b.resolveWorkspaceMemo(ctx)
	if err != nil {
		return nil, err
	}
	// Prefer an exact public id match. Name lookup remains below for backwards
	// compatibility with bex-native callers and hand-applied legacy CRs.
	validLabel := len(validation.IsValidLabelValue(name)) == 0
	var byID appv1alpha1.AppList
	if validLabel {
		if err := b.Client.List(ctx, &byID,
			client.MatchingLabels{LabelAppID: name}); err != nil {
			return nil, err
		}
	}
	preferOwnWorkspaceNamespace(byID.Items)
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
		if len(pathvalidation.IsValidPathSegmentName(candidate)) != 0 {
			continue
		}
		err := b.Client.Get(ctx, client.ObjectKey{Namespace: b.AppNamespace(acting), Name: candidate}, &a)
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
	if acting == "" || !validLabel {
		return nil, ErrNotFound
	}
	var list appv1alpha1.AppList
	if err := b.Client.List(ctx, &list,
		client.MatchingLabels{LabelServiceName: name}); err != nil {
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
	// codex round-7 F8: an all-denied by-NAME sweep collapses to ErrNotFound —
	// names are the guessable lookup key, and a foreign App's existence is not
	// an answer a cross-tenant probe deserves (AuthorizeApp's fallback does the
	// same). Anything that is not a plain denial (an authz outage) stays a
	// distinct error so the caller fails closed.
	if errors.Is(lastErr, ErrForbidden) {
		return nil, ErrNotFound
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
	if named, explicit := WorkspaceFrom(ctx); explicit && owner != "" && owner != named {
		return ErrForbidden
	}
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
// the selection the logs and metrics features share. An App's replica pods
// live in its per-tenant namespace (their Deployment is created there), and
// bex-api's role does NOT grant cluster-wide pod list — only per-namespace —
// so this resolves the App's namespace first via the cluster-wide App list the
// role does grant. app is the App CR's metadata.name, which is exactly the
// value the controller stamps as PodLabelApp, so the by-name resolve and the
// pod filter agree.
func (b *Base) AppPods(ctx context.Context, app string) ([]corev1.Pod, error) {
	ns, err := b.appNamespaceByName(ctx, app)
	if err != nil {
		return nil, err
	}
	if ns == "" {
		// No such App => no pods.
		return nil, nil
	}
	return b.AppPodsIn(ctx, ns, app)
}

// AppPodsIn is AppPods for a caller that already holds the App CR, skipping the
// by-name namespace resolve. That resolve is an UNSELECTED cluster-wide App
// list against an uncached client, so it ships every App CR on the platform
// over the wire to learn one namespace; a verb that reached this point through
// AuthorizeApp already has app.Namespace in hand and must not pay for it.
func (b *Base) AppPodsIn(ctx context.Context, namespace, app string) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := b.Client.List(ctx, &pods,
		client.InNamespace(namespace),
		client.MatchingLabels{PodLabelApp: app}); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

// AppInOwnWorkspaceNamespace reports whether a labeled App CR lives in its own
// workspace's namespace — the canonical placement under ADR043, where a
// projected CR's namespace IS its workspace id (store.WorkspaceNamespace, an
// identity mapping this predicate assumes). A CR that carries a workspace
// label but sits elsewhere is a stray: the ADR043 migration left such
// duplicates behind in the old shared namespace, and one of them shadowing the
// live CR in a first-match lookup sent metrics queries to the wrong namespace.
// An unlabeled CR (hand-applied, store-off) has no canonical namespace and is
// never demoted. Exported for store's projector — the same rule must decide
// "which twin is live" on both the read and write paths.
func AppInOwnWorkspaceNamespace(a *appv1alpha1.App) bool {
	ws := a.Labels[LabelTenant]
	return ws == "" || a.Namespace == ws
}

// preferOwnWorkspaceNamespace stably reorders same-id App candidates so CRs in
// their canonical ADR043 namespace come first — making every first-authorized-
// wins loop over a LabelAppID list deterministic even when a stray duplicate
// (same id, wrong namespace) exists. Stable: ties keep the server's order.
func preferOwnWorkspaceNamespace(items []appv1alpha1.App) {
	if len(items) < 2 {
		return
	}
	slices.SortStableFunc(items, func(a, b appv1alpha1.App) int {
		ca, cb := AppInOwnWorkspaceNamespace(&a), AppInOwnWorkspaceNamespace(&b)
		switch {
		case ca && !cb:
			return -1
		case cb && !ca:
			return 1
		}
		return 0
	})
}

// AppNamespaceByName resolves the namespace an App's pods and Prometheus
// series live in from its metadata.name — for callers (e.g. the metrics
// resource-series funnel) that hold only the name, not the App CR. It is the
// App's per-tenant `<ws>` namespace (ADR043). A missing App or lookup error
// falls back to b.Namespace, where a query simply returns empty series — never
// an error to the caller. Callers that already hold the App CR should use
// app.Namespace directly instead.
func (b *Base) AppNamespaceByName(ctx context.Context, app string) string {
	ns, err := b.appNamespaceByName(ctx, app)
	if err != nil || ns == "" {
		return b.Namespace
	}
	return ns
}

// appNamespaceByName resolves the namespace of the App CR whose metadata.name is
// app — AppPods/AppNamespaceByName cannot list pods cluster-wide and so must
// first learn which workspace namespace the App (and therefore its pods)
// lives in. Returns "" when no such App exists. A name matched by two CRs
// (a stray wrong-namespace duplicate alongside the live one — see
// AppInOwnWorkspaceNamespace) resolves to the canonical-namespace CR, never
// first-in-list order.
func (b *Base) appNamespaceByName(ctx context.Context, app string) (string, error) {
	var list appv1alpha1.AppList
	if err := b.Client.List(ctx, &list); err != nil {
		return "", err
	}
	found := ""
	for i := range list.Items {
		if list.Items[i].Name != app {
			continue
		}
		if AppInOwnWorkspaceNamespace(&list.Items[i]) {
			return list.Items[i].Namespace, nil
		}
		if found == "" {
			found = list.Items[i].Namespace
		}
	}
	return found, nil
}

// PreDeployPods lists an App's pre-deploy Job pods (the app.bex.co/predeploy
// label, w1/m33) in namespace — the logs feature reads a migration's output from
// these. namespace is the App's own (per-workspace) namespace: the pre-deploy
// Job is co-located with the App (ADR043 D8), not in the build namespace.
// Empty falls back to the API's own namespace.
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
//
// namespace is the Database CR's own namespace (ADR043 D8) — callers hold the
// CR from AuthorizeDatabase, so they pass d.Namespace. Empty falls back to the
// shared apps namespace, matching the pre-D8 placement. Hardcoding b.Namespace
// here is what made datastore logs silently empty for a migrated workspace: a
// pod list in the wrong namespace returns no error, just nothing.
func (b *Base) DatabasePods(ctx context.Context, namespace, clusterName string) ([]corev1.Pod, error) {
	if namespace == "" {
		namespace = b.Namespace
	}
	var pods corev1.PodList
	if err := b.Client.List(ctx, &pods,
		client.InNamespace(namespace),
		client.MatchingLabels{PodLabelCNPGCluster: clusterName}); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

// KeyValuePods lists the Valkey pods for a managed Key Value store — selected
// by the app.bex.co/keyvalue=<name> label the KeyValue reconciler stamps on
// every StatefulSet pod template. Used by the keyvalue logs feature (w3/m30);
// name is the KeyValue CR's metadata.name (the immutable red- id). namespace
// follows the CR, exactly as DatabasePods above.
func (b *Base) KeyValuePods(ctx context.Context, namespace, name string) ([]corev1.Pod, error) {
	if namespace == "" {
		namespace = b.Namespace
	}
	var pods corev1.PodList
	if err := b.Client.List(ctx, &pods,
		client.InNamespace(namespace),
		client.MatchingLabels{PodLabelKeyValue: name}); err != nil {
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
