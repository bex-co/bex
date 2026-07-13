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
	PodLabelApp  = "app.bex.co/app"
	AppContainer = "app"
)

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
	RelCanView          = "can_view"           // viewer and up: lists, details, metrics
	RelCanViewLogs      = "can_view_logs"      // contributor and up (Render: viewers can't see logs)
	RelCanOperate       = "can_operate"        // contributor and up: restart/suspend/resume
	RelCanCreate        = "can_create"         // developer and up: create/delete resources
	RelCanViewSensitive = "can_view_sensitive" // developer and up: connection strings
	RelCanManageKeys    = "can_manage_keys"    // developer and up: workspace API keys
	RelCanManage        = "can_manage"         // admin only: manage the workspace itself (rename/delete)

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

// WorkspaceResolver maps an authenticated caller to its workspace: the tenant
// id ("tea-<id>") for a tenant member or a bound API key (ok=true). ok=false
// means the store is on but the caller resolves to no tenant — an unbound
// machine key, or the platform bootstrap — and the caller is left on the
// default workspace (bootstrap is seeded admin there; a tuple-less key 403s).
// A nil Base.Workspace means the store is off: every caller stays on
// workspace:default, the legacy single-tenant mode (byte-identical to before).
type WorkspaceResolver interface {
	Tenant(ctx context.Context, id Identity) (tenantID string, ok bool)
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
func (b *Base) Authorize(ctx context.Context, relation string) error {
	return b.authorizeAndAudit(ctx, relation, b.callerWorkspace(ctx), "", callerVerb(verbFrameSkip))
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
	return b.authorizeAndAudit(ctx, relation, b.callerWorkspace(ctx), target, callerVerb(verbFrameSkip))
}

// callerWorkspace is the OpenFGA object of the caller's OWN workspace:
// workspace:tea-<id> when the resolver finds a tenant, workspace:default
// otherwise (store off, or an unbound machine caller that 403s there unless it
// is the seeded bootstrap).
func (b *Base) callerWorkspace(ctx context.Context) string {
	if b.Workspace != nil {
		if id, ok := IdentityFrom(ctx); ok {
			if tenantID, found := b.Workspace.Tenant(ctx, id); found {
				return "workspace:" + tenantID
			}
		}
	}
	return DefaultWorkspace
}

// AuthorizeOn gates a verb on the caller's permission against a specific object
// (e.g. workspace:tea-abc) — the seam for verbs scoped to a named workspace
// rather than the caller's own (the workspaces lifecycle verbs check `admin` on
// the exact workspace being renamed/deleted, which may differ from the
// caller's default — the cross-tenant case the audit log's denial events cover
// for the RelCanManage/RelCanCreate/RelCanOperate/RelCanManageKeys verbs that
// call this directly). Same audit interception as Authorize.
func (b *Base) AuthorizeOn(ctx context.Context, relation, object string) error {
	return b.authorizeAndAudit(ctx, relation, object, "", callerVerb(verbFrameSkip))
}

// authorizeAndAudit runs the OpenFGA check and, for write relations only,
// records the outcome (audit.go) — the one place Authorize, AuthorizeTarget and
// AuthorizeOn funnel through, so a verb is recorded exactly once regardless of
// which entry point it calls.
func (b *Base) authorizeAndAudit(ctx context.Context, relation, object, target, verb string) error {
	err := b.checkAuthz(ctx, relation, object)
	if writeRelations[relation] {
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

// Tenant returns the caller's tenant id via the workspace resolver. ok=false
// when there is no resolver (store off) or the caller has no tenant; callers
// that filter to the caller's workspace check b.Workspace != nil first to
// distinguish "store off (no filter)" from "store on (filter, maybe empty)".
func (b *Base) Tenant(ctx context.Context) (string, bool) {
	if b.Workspace == nil {
		return "", false
	}
	id, ok := IdentityFrom(ctx)
	if !ok {
		return "", false
	}
	return b.Workspace.Tenant(ctx, id)
}

// GetApp fetches one App by name, mapping absence to ErrNotFound. Shared by the
// apps/logs/metrics/secrets services — each needs "does this App exist / read
// its status" without reimplementing the not-found mapping. With the
// control-plane store on (Workspace resolver wired), it also gates the App
// against the caller's tenant: a name that resolves to another tenant's App is
// ErrForbidden, not a leak through the shared fetch every feature uses (a
// cross-tenant caller who knows an App name must not read its logs, metrics,
// or secrets just because apps.Service alone learned to check). ErrForbidden,
// not ErrNotFound, matches the existing convention — "not yours," not "doesn't
// exist," so a cross-tenant caller can't probe existence by name. A no-identity
// caller (the git webhook's HMAC-authenticated redeploy, which carries no
// core.Identity) skips the gate — it has no workspace to check against.
func (b *Base) GetApp(ctx context.Context, name string) (*appv1alpha1.App, error) {
	var a appv1alpha1.App
	err := b.Client.Get(ctx, client.ObjectKey{Namespace: b.Namespace, Name: name}, &a)
	if apierrors.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if b.Workspace != nil {
		if id, ok := IdentityFrom(ctx); ok {
			if tenantID, found := b.Workspace.Tenant(ctx, id); found && a.Labels[LabelTenant] != tenantID {
				return nil, ErrForbidden
			}
		}
	}
	return &a, nil
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
