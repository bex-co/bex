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
	"log"
	"regexp"
	"runtime"
	"strings"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// AuditOutcome is the result of an authorized write attempt (w4/m10).
type AuditOutcome string

const (
	AuditAllowed AuditOutcome = "allowed"
	AuditDenied  AuditOutcome = "denied"
)

// AuditEvent is one recorded write-verb authorization decision: who attempted
// what, against which object, and whether it was allowed. Never carries
// request bodies or arbitrary values — Verb/Resource/Target are derived from
// the call site, the OpenFGA object string, and the verb's resource NAME. The
// typed detail fields (MaintenanceModeTo, PlanFrom/PlanTo,
// InstanceCountFrom/InstanceCountTo, Autoscaling*From/*To, AutoDeployEnabled)
// are non-secret scalars required by Render's event contracts; there is no
// free-form path for a secret to reach an event.
type AuditEvent struct {
	Caller       string // core.Identity.Subject; empty for an unauthenticated caller
	CallerMethod string // core.Identity.Method ("oauth2" | "session")
	Verb         string // the verb's short name, e.g. "apps.Suspend" (derived, never hand-set)
	Resource     string // the OpenFGA object authorized against, e.g. "workspace:tea-abc"
	Target       string // the resource acted ON, e.g. "service:my-api"; empty for a workspace-wide verb
	// TargetName is the non-secret display name for a typed target. Datastore
	// webhook payloads require both the immutable dpg-/red- id (Target) and this
	// human name; ordinary audit rows may leave it empty.
	TargetName string
	Outcome    AuditOutcome
	At         time.Time
	// MaintenanceModeTo is Render's MaintenanceModeEnabledEvent metadata.to.
	// Nil for every other verb; a pointer preserves an explicit false value.
	MaintenanceModeTo *bool
	// Per-verb typed detail fields — exactly as maintenance_mode_to, one column per
	// value, no free-form object. Nil for every verb that does not define them.
	PlanFrom           *string
	PlanTo             *string
	InstanceCountFrom  *int32
	InstanceCountTo    *int32
	AutoscalingMinFrom *int32
	AutoscalingMaxFrom *int32
	AutoscalingMinTo   *int32
	AutoscalingMaxTo   *int32
	AutoDeployEnabled  *bool
	// RoleFrom/RoleTo are the member-role detail of the team-membership verbs
	// (w1/m33): a role change records old→new; an invite/acceptance records the
	// granted role in RoleTo alone. Roles are the closed members.Roles ladder,
	// never free-form.
	RoleFrom *string
	RoleTo   *string
	// BillingExcludedTo is the value a billing-exclusion toggle was set TO
	// (docs/ADR040-billing-metronome.md §7): true = comped/exempt out of
	// Stripe, false = billable again. Nil for every other verb. Admin-only,
	// set through the control-plane internal API.
	BillingExcludedTo *bool
}

type auditMaintenanceModeToKey struct{}

type deferAllowedWriteAuditKey struct{}

const (
	// AuditVerbMaintenanceModeEnabled and AuditVerbMaintenanceModeURIUpdated
	// are Render's two field-level maintenance effects. Unlike ordinary verbs,
	// both may be emitted by one atomic ConfigureMaintenanceMode write.
	AuditVerbMaintenanceModeEnabled    = "apps.SetMaintenanceMode"
	AuditVerbMaintenanceModeURIUpdated = "apps.SetMaintenanceModeURI"
	// AuditVerbEnvGroupOwnershipMigrated is the one-time system event emitted
	// when a pre-attribution environment group is assigned to the bootstrap
	// workspace. It is fixed vocabulary so migration code cannot inject an
	// arbitrary action into the audit trail.
	AuditVerbEnvGroupOwnershipMigrated = "envgroups.MigrateOwnership"
	// AuditVerbSetPlan, AuditVerbScale, AuditVerbSetAutoscaling,
	// AuditVerbDeleteAutoscaling, AuditVerbSetAutoDeploy are spelled exactly as
	// callerVerb() would derive them — duplicated here as constants so the
	// recording helpers can reference them without re-deriving via the call stack.
	AuditVerbSetPlan           = "apps.SetPlan"
	AuditVerbScale             = "apps.Scale"
	AuditVerbSetAutoscaling    = "apps.SetAutoscaling"
	AuditVerbDeleteAutoscaling = "apps.DeleteAutoscaling"
	AuditVerbSetAutoDeploy     = "apps.SetAutoDeploy"
	// Fixed datastore effects that have exact Render webhook counterparts. These
	// are recorded only after the underlying mutation succeeds, then consumed by
	// the outbound-webhook projection. Keeping them fixed prevents arbitrary
	// event vocabulary from entering the audit feed.
	AuditVerbPostgresCreated            = "postgres.CreatePostgres"
	AuditVerbPostgresRestarted          = "postgres.Restart"
	AuditVerbPostgresCredentialsCreated = "postgres.CreateUser"
	AuditVerbPostgresCredentialsDeleted = "postgres.DeleteUser"
	AuditVerbPostgresBackupStarted      = "postgres.CreateExport"
	AuditVerbPostgresPlanChanged        = "postgres.SetPlan"
	AuditVerbPostgresUpdated            = "postgres.UpdatePostgres"
	AuditVerbKeyValuePlanChanged        = "keyvalue.SetPlan"
	AuditVerbKeyValueUpdated            = "keyvalue.UpdateKeyValue"
	// Team-membership effects (w1/m33). Invite and ChangeRole are spelled exactly
	// as callerVerb() derives them (deferred-success recording, like SetPlan);
	// AcceptInvite is recorded explicitly — redemption happens on the auth gate's
	// login path (email match) or the token verb, neither of which passes a
	// write-relation authorize for the joined workspace.
	AuditVerbMemberInvited     = "members.Invite"
	AuditVerbMemberRoleChanged = "members.ChangeRole"
	AuditVerbInviteAccepted    = "members.AcceptInvite"
	// AuditVerbBillingExclusionChanged records an admin toggling a workspace's
	// Stripe billing exclusion (docs/ADR040-billing-metronome.md §7). It is
	// written directly by the control-plane internal API (not via a Base
	// authorize), so it carries no target — it is workspace-wide.
	AuditVerbBillingExclusionChanged = "billing.SetExclusion"
	AuditVerbBillingCompChanged      = "billing.SetComp"
	AuditVerbBillingGraceExtended    = "billing.ExtendGrace"
	AuditVerbBillingRecoveryForced   = "billing.ForceRecovery"
	AuditVerbBillingEnforced         = "billing.EnforceWorkspace"
	AuditVerbBillingRecovered        = "billing.RecoverWorkspace"
)

// DatabaseAuditEffect is the closed set of successful Database mutations that
// project to Render webhook events.
type DatabaseAuditEffect int

const (
	DatabaseCreated DatabaseAuditEffect = iota
	DatabaseRestarted
	DatabaseCredentialsCreated
	DatabaseCredentialsDeleted
	DatabaseBackupStarted
	DatabasePlanChanged
	DatabaseUpdated
)

var databaseAuditVerbs = map[DatabaseAuditEffect]string{
	DatabaseCreated:            AuditVerbPostgresCreated,
	DatabaseRestarted:          AuditVerbPostgresRestarted,
	DatabaseCredentialsCreated: AuditVerbPostgresCredentialsCreated,
	DatabaseCredentialsDeleted: AuditVerbPostgresCredentialsDeleted,
	DatabaseBackupStarted:      AuditVerbPostgresBackupStarted,
	DatabasePlanChanged:        AuditVerbPostgresPlanChanged,
	DatabaseUpdated:            AuditVerbPostgresUpdated,
}

// KeyValueAuditEffect is the closed successful-write vocabulary for Key Value.
type KeyValueAuditEffect int

const (
	KeyValuePlanChanged KeyValueAuditEffect = iota
	KeyValueUpdated
)

var keyValueAuditVerbs = map[KeyValueAuditEffect]string{
	KeyValuePlanChanged: AuditVerbKeyValuePlanChanged,
	KeyValueUpdated:     AuditVerbKeyValueUpdated,
}

// WithAuditMaintenanceModeTo attaches Render's one typed maintenance-toggle
// audit detail to the authorization context. It is deliberately narrower than
// a generic metadata map so value-bearing verb arguments remain structurally
// unable to enter the audit store.
func WithAuditMaintenanceModeTo(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, auditMaintenanceModeToKey{}, enabled)
}

// WithDeferredAllowedWriteAudit keeps denied authorization attempts visible
// while deferring an allowed write event until the caller confirms its state
// mutation succeeded. Composite atomic verbs use it when their successful
// effects must be represented by more than one Render event.
func WithDeferredAllowedWriteAudit(ctx context.Context) context.Context {
	return context.WithValue(ctx, deferAllowedWriteAuditKey{}, true)
}

func defersAllowedWriteAudit(ctx context.Context) bool {
	deferred, _ := ctx.Value(deferAllowedWriteAuditKey{}).(bool)
	return deferred
}

// RecordMaintenanceModeEffects records the successful field-level effects of
// one atomic maintenance-mode write. URI precedes enabled when both changed,
// matching the documented deterministic ordering. The fixed verbs keep this
// narrow: callers cannot inject arbitrary audit vocabulary or metadata.
func (b *Base) RecordMaintenanceModeEffects(
	ctx context.Context,
	app *appv1alpha1.App,
	uriChanged bool,
	enabledChanged *bool,
) {
	resource, err := b.resourceWorkspace(ctx, app.Labels)
	if err != nil {
		log.Printf("audit: resolve maintenance resource: %v", err)
		return
	}
	target := canonicalAppTarget(app)
	if uriChanged {
		b.emit(ctx, AuditVerbMaintenanceModeURIUpdated, resource, target, nil)
	}
	if enabledChanged != nil {
		toggleCtx := WithAuditMaintenanceModeTo(ctx, *enabledChanged)
		b.emit(toggleCtx, AuditVerbMaintenanceModeEnabled, resource, target, nil)
	}
}

// ServiceTarget is the AuditEvent.Target of a verb acting on one service — the
// dimension the audit log lacked until w3/m7 (Resource is the OpenFGA object,
// i.e. the WORKSPACE a verb was authorized against, which cannot say WHICH
// service was suspended). With it, the per-service events feed (internal/events)
// is a pure VIEW over audit_events + deploys: no second write path, and the
// structural redaction holds — a target is a resource name, never a value.
//
// Note the App CR name is globally unique (all Apps live in one namespace), so
// a service target is unambiguous across tenants; internal/events still scopes
// its read by workspace so a cross-tenant caller cannot inject rows into
// someone else's feed (see store.ListServiceEvents).
func ServiceTarget(name string) string { return "service:" + name }

// DatabaseTarget is ServiceTarget's sibling for a managed Postgres Database
// (AuthorizeDatabase).
func DatabaseTarget(name string) string { return "database:" + name }

// KeyValueTarget is ServiceTarget's sibling for a managed KeyValue
// (AuthorizeKeyValue).
func KeyValueTarget(name string) string { return "keyvalue:" + name }

// EnvGroupTarget is the audit target of the one-time environment-group
// ownership migration. Group ids are opaque resource identifiers, never
// secret values.
func EnvGroupTarget(groupID string) string { return "envgroup:" + groupID }

// MemberTarget is the audit target of a verb acting on one workspace MEMBER
// (role change, removal). The subject is the member's identity id — an opaque
// identifier, never a value; the events feed never reads this kind (members
// have no per-service feed), so it exists for the workspace audit trail alone.
func MemberTarget(subject string) string { return "member:" + subject }

// InviteTarget is MemberTarget's sibling for a pending invite (revoke, resend,
// acceptance). Invite ids are opaque inv-… identifiers.
func InviteTarget(inviteID string) string { return "invite:" + inviteID }

// WebhookAttemptTarget identifies a manual delivery action in the workspace
// audit trail. Endpoint and attempt IDs are opaque, non-secret identifiers;
// joining both prevents an attempt ID from losing its endpoint scope without
// placing the request payload, destination URL, or idempotency key in audit
// storage.
func WebhookAttemptTarget(endpointID, attemptID string) string {
	return "webhook_attempt:" + endpointID + "/" + attemptID
}

// SplitTarget is the constructors' inverse — the one place the
// "<kind>:<name>" target format is decoded (the audit read surface keys its
// Render metadata by kind, w4/m26), kept beside the mint sites above so the
// format stays a single contract. ok is false for an empty or kindless
// target (a workspace-wide verb records none).
func SplitTarget(target string) (kind, name string, ok bool) {
	kind, name, ok = strings.Cut(target, ":")
	if !ok || kind == "" || name == "" {
		return "", "", false
	}
	return kind, name, true
}

// RecordEnvGroupOwnershipMigration records the lazy ownership migration as a
// system action. This deliberately bypasses caller identity: the read merely
// discovers the legacy row, while the platform performs the migration.
func (b *Base) RecordEnvGroupOwnershipMigration(ctx context.Context, groupID, workspaceID string) {
	b.recordAudit(ctx, AuditEvent{
		Caller:       "system",
		CallerMethod: "system",
		Verb:         AuditVerbEnvGroupOwnershipMigrated,
		Resource:     WorkspaceObject(workspaceID),
		Target:       EnvGroupTarget(groupID),
		Outcome:      AuditAllowed,
		At:           b.Now(),
	})
}

// RecordPlanChanged records the typed from/to detail for a successful SetPlan.
// Called after the write path confirms success, paired with
// WithDeferredAllowedWriteAudit on the AuthorizeApp call.
func (b *Base) RecordPlanChanged(ctx context.Context, app *appv1alpha1.App, fromPlan, toPlan string) {
	resource, err := b.resourceWorkspace(ctx, app.Labels)
	if err != nil {
		log.Printf("audit: resolve plan-change resource: %v", err)
		return
	}
	ev := b.verbAuditEvent(ctx, AuditVerbSetPlan, resource, canonicalAppTarget(app))
	ev.PlanFrom = &fromPlan
	ev.PlanTo = &toPlan
	b.recordAudit(ctx, ev)
}

// RecordScaleEffect records the typed from/to detail for a successful Scale.
// Called after the write path confirms success, paired with
// WithDeferredAllowedWriteAudit on the AuthorizeApp call.
func (b *Base) RecordScaleEffect(ctx context.Context, app *appv1alpha1.App, fromCount, toCount int32) {
	resource, err := b.resourceWorkspace(ctx, app.Labels)
	if err != nil {
		log.Printf("audit: resolve scale resource: %v", err)
		return
	}
	ev := b.verbAuditEvent(ctx, AuditVerbScale, resource, canonicalAppTarget(app))
	ev.InstanceCountFrom = &fromCount
	ev.InstanceCountTo = &toCount
	b.recordAudit(ctx, ev)
}

// RecordAutoscalingChanged records the typed from/to detail for a successful
// SetAutoscaling or DeleteAutoscaling. fromMin/fromMax are nil when autoscaling
// was previously disabled; toMin/toMax are nil when autoscaling was deleted.
// Called after the write path confirms success, paired with
// WithDeferredAllowedWriteAudit on the AuthorizeApp call.
func (b *Base) RecordAutoscalingChanged(ctx context.Context, app *appv1alpha1.App, fromMin, fromMax, toMin, toMax *int32) {
	resource, err := b.resourceWorkspace(ctx, app.Labels)
	if err != nil {
		log.Printf("audit: resolve autoscaling resource: %v", err)
		return
	}
	verb := AuditVerbSetAutoscaling
	if toMin == nil {
		verb = AuditVerbDeleteAutoscaling
	}
	ev := b.verbAuditEvent(ctx, verb, resource, canonicalAppTarget(app))
	ev.AutoscalingMinFrom = fromMin
	ev.AutoscalingMaxFrom = fromMax
	ev.AutoscalingMinTo = toMin
	ev.AutoscalingMaxTo = toMax
	b.recordAudit(ctx, ev)
}

// RecordAutoDeployChanged records the typed enabled detail for a successful
// SetAutoDeploy. Called after the write path confirms success, paired with
// WithDeferredAllowedWriteAudit on the AuthorizeApp call.
func (b *Base) RecordAutoDeployChanged(ctx context.Context, app *appv1alpha1.App, enabled bool) {
	resource, err := b.resourceWorkspace(ctx, app.Labels)
	if err != nil {
		log.Printf("audit: resolve auto-deploy resource: %v", err)
		return
	}
	ev := b.verbAuditEvent(ctx, AuditVerbSetAutoDeploy, resource, canonicalAppTarget(app))
	ev.AutoDeployEnabled = &enabled
	b.recordAudit(ctx, ev)
}

// RecordMemberInvited records a successful workspace invite with its typed
// role detail (w1/m33). Recorded only after the invite row is written — paired
// with WithDeferredAllowedWriteAudit on the verb's authorize, because the
// invite id (the target) does not exist at authorize time. TargetName carries
// the invited email: the invite's human identifier, shown to the same
// manage-tier audience the pending-invites list already shows it to.
func (b *Base) RecordMemberInvited(ctx context.Context, workspaceID, inviteID, email, role string) {
	ev := b.verbAuditEvent(ctx, AuditVerbMemberInvited, WorkspaceObject(workspaceID), InviteTarget(inviteID))
	ev.TargetName = email
	ev.RoleTo = &role
	b.recordAudit(ctx, ev)
}

// RecordMemberRoleChanged records a successful member role change with the
// typed old→new pair, exactly like apps' RecordPlanChanged. Paired with
// WithDeferredAllowedWriteAudit on the verb's authorize so a refused change
// (plan-gated role, last admin) leaves only the denial/attempt trail.
func (b *Base) RecordMemberRoleChanged(ctx context.Context, workspaceID, subject, fromRole, toRole string) {
	ev := b.verbAuditEvent(ctx, AuditVerbMemberRoleChanged, WorkspaceObject(workspaceID), MemberTarget(subject))
	ev.RoleFrom = &fromRole
	ev.RoleTo = &toRole
	b.recordAudit(ctx, ev)
}

// RecordInviteAccepted writes the members.AcceptInvite audit event through
// sink — a package-level recorder (rather than only a Base helper) because one
// of its two callers is the auth gate's login-time email-match redemption
// (internal/api/tenancy.go), which has no feature Base. The caller identity is
// the ACCEPTING one — the invited teammate joining, not an admin acting on
// them.
func RecordInviteAccepted(ctx context.Context, sink AuditSink, at time.Time, workspaceID, inviteID, email, role, callerSubject, callerMethod string) {
	RecordAuditEvent(ctx, sink, AuditEvent{
		Caller:       callerSubject,
		CallerMethod: callerMethod,
		Verb:         AuditVerbInviteAccepted,
		Resource:     WorkspaceObject(workspaceID),
		Target:       InviteTarget(inviteID),
		TargetName:   email,
		Outcome:      AuditAllowed,
		At:           at,
		RoleTo:       &role,
	})
}

// RecordDatabaseEffect records one successful, sourceable managed-Postgres
// effect for the outbound-webhook feed. Callers pair it with
// WithDeferredAllowedWriteAudit so failed validation/writes never emit.
func (b *Base) RecordDatabaseEffect(ctx context.Context, d *appv1alpha1.Database, effect DatabaseAuditEffect) {
	ev, ok := b.databaseEffectEvent(ctx, d, effect)
	if !ok {
		return
	}
	b.recordAudit(ctx, ev)
}

// RecordDatabasePlanChanged records a successful managed-Postgres SetPlan with
// the typed from/to plan pair, exactly like apps' RecordPlanChanged. Following
// that precedent it is recorded even when fromPlan == toPlan: the verb names
// the call the caller actually made, and the equal pair states truthfully that
// nothing changed — never the generic Update* verb of a call they didn't make.
func (b *Base) RecordDatabasePlanChanged(ctx context.Context, d *appv1alpha1.Database, fromPlan, toPlan string) {
	ev, ok := b.databaseEffectEvent(ctx, d, DatabasePlanChanged)
	if !ok {
		return
	}
	ev.PlanFrom = &fromPlan
	ev.PlanTo = &toPlan
	b.recordAudit(ctx, ev)
}

func (b *Base) databaseEffectEvent(ctx context.Context, d *appv1alpha1.Database, effect DatabaseAuditEffect) (AuditEvent, bool) {
	verb, ok := databaseAuditVerbs[effect]
	if !ok {
		log.Printf("audit: unknown database effect %d", effect)
		return AuditEvent{}, false
	}
	resource, err := b.resourceWorkspace(ctx, d.Labels)
	if err != nil {
		log.Printf("audit: resolve database-effect resource: %v", err)
		return AuditEvent{}, false
	}
	ev := b.verbAuditEvent(ctx, verb, resource, DatabaseTarget(d.Name))
	ev.TargetName = d.Spec.Name
	return ev, true
}

// RecordKeyValueEffect records a successful Key Value mutation. Plan changes
// project to Render's plan_changed webhook; the generic update effect preserves
// the ordinary audit row when a deferred PATCH changes other fields only.
func (b *Base) RecordKeyValueEffect(ctx context.Context, kv *appv1alpha1.KeyValue, effect KeyValueAuditEffect) {
	ev, ok := b.keyValueEffectEvent(ctx, kv, effect)
	if !ok {
		return
	}
	b.recordAudit(ctx, ev)
}

// RecordKeyValuePlanChanged records a successful Key Value SetPlan with the
// typed from/to plan pair — the keyvalue mirror of RecordDatabasePlanChanged
// (see there for the recorded-even-when-idempotent rationale).
func (b *Base) RecordKeyValuePlanChanged(ctx context.Context, kv *appv1alpha1.KeyValue, fromPlan, toPlan string) {
	ev, ok := b.keyValueEffectEvent(ctx, kv, KeyValuePlanChanged)
	if !ok {
		return
	}
	ev.PlanFrom = &fromPlan
	ev.PlanTo = &toPlan
	b.recordAudit(ctx, ev)
}

func (b *Base) keyValueEffectEvent(ctx context.Context, kv *appv1alpha1.KeyValue, effect KeyValueAuditEffect) (AuditEvent, bool) {
	verb, ok := keyValueAuditVerbs[effect]
	if !ok {
		log.Printf("audit: unknown key-value effect %d", effect)
		return AuditEvent{}, false
	}
	resource, err := b.resourceWorkspace(ctx, kv.Labels)
	if err != nil {
		log.Printf("audit: resolve key-value plan-change resource: %v", err)
		return AuditEvent{}, false
	}
	ev := b.verbAuditEvent(ctx, verb, resource, KeyValueTarget(kv.Name))
	ev.TargetName = kv.Spec.Name
	return ev, true
}

// verbAuditEvent builds a pre-populated AuditEvent for a deferred allowed-write
// record. The verb and resource come from the caller (they cannot be derived
// from the call stack inside a recording helper); the identity comes from ctx.
func (b *Base) verbAuditEvent(ctx context.Context, verb, resource, target string) AuditEvent {
	ev := AuditEvent{Verb: verb, Resource: resource, Target: target, Outcome: AuditAllowed, At: b.Now()}
	if id, ok := IdentityFrom(ctx); ok {
		ev.Caller, ev.CallerMethod = id.Subject, id.Method
	}
	return ev
}

// AuditSink persists audit events. Base.emit bounds every call to
// auditRecordTimeout and always swallows a Record error (logged, never
// returned) — a sink outage adds at most that much latency to the verb it's
// recording, never fails or indefinitely stalls it.
type AuditSink interface {
	Record(ctx context.Context, ev AuditEvent) error
}

// auditRecordTimeout bounds Base.emit's sink call: recording must never
// indefinitely block the verb path it's instrumenting (a stalled sink adds at
// most this much latency, not an unbounded stall). Short relative to
// apikeys.Service.TouchAPIKey's fully-detached 10s fire-and-forget — this call
// is still synchronous on the verb's own path, so it stays tight.
const auditRecordTimeout = 2 * time.Second

// maxFallbackCandidateAudits caps how many rejected name-collision candidates
// AuthorizeApp's LabelServiceName fallback loop individually audits on one
// request (w4/015). Legitimate collisions (a handful of tenants naming a
// service "web") keep one row per candidate — each genuinely a distinct
// authorization decision — while a probe against a pathologically common name
// can no longer serialize an unbounded run of synchronous audit writes
// (auditRecordTimeout each) nor amplify one denied request into hundreds of
// audit_events inserts; denials past the cap are still fully checked, and one
// aggregate row against the CALLER's workspace records that the probe
// continued past the individually-audited candidates. Worst-case added
// latency on the denial path: (maxFallbackCandidateAudits+1) *
// auditRecordTimeout.
const maxFallbackCandidateAudits = 3

// noopAuditSink is the store-less default: recording is a no-op, the same
// degrade-by-omission every other store-backed feature uses.
type noopAuditSink struct{}

func (noopAuditSink) Record(context.Context, AuditEvent) error { return nil }

// NoopAuditSink is Base's effective sink when Audit is nil.
var NoopAuditSink AuditSink = noopAuditSink{}

// writeRelations are the Rel… constants that gate a mutation. These emit an
// audit event on every call, allowed or denied.
var writeRelations = map[string]bool{
	RelCanOperate:       true,
	RelCanCreate:        true,
	RelCanManageKeys:    true,
	RelCanManageSSHKeys: true,
	RelCanManage:        true,
	RelCanManageBilling: true,
}

// readRelations are the Rel… constants that gate a read. A successful read
// stays unrecorded (volume-prohibitive — every list/get/metrics call would
// double audit_events' write rate); a DENIED read is a security-relevant
// event (someone probed a resource they can't see) and is recorded exactly
// like a denied write (w4/m20/t001, closing the deliberate w4/m10 cut noted
// in docs/ADR006-bex-api.md § Audit log). No entry-point split: Authorize,
// AuthorizeTarget/AuthorizeApp-family and AuthorizeOn all funnel through
// authorizeAndAudit, so a denied AuthorizeOn-style workspace-wide read check
// records exactly like a denied resource-scoped one — Target is simply
// whatever the entry point already passes (empty for workspace-wide, the
// resource name for AuthorizeApp/AuthorizeDatabase/AuthorizeKeyValue), the
// same split write-verb recording already uses.
var readRelations = map[string]bool{
	RelCanView:          true,
	RelCanViewLogs:      true,
	RelCanViewSensitive: true,
}

// auditsDenial reports whether a DENIED check of relation records an audit
// event — the one predicate authorizeAndRecord and AuthorizeApp's aggregate
// fallback row both consult, so the aggregate row can't silently drift from
// the per-candidate rows when a relation class changes.
func auditsDenial(relation string) bool {
	return writeRelations[relation] || readRelations[relation]
}

// receiverRE strips a method's pointer-receiver type (e.g. "(*Service).") out
// of its runtime-reported name, so "apps.(*Service).Suspend" reads as
// "apps.Suspend" — a short, greppable verb.
var receiverRE = regexp.MustCompile(`\(\*?\w+\)\.`)

// verbFrameSkip is the runtime.Caller depth from inside callerVerb up to the
// verb method that called Authorize/AuthorizeTarget/AuthorizeOn: 0 is
// callerVerb's own frame, 1 is the entry point (callerVerb's direct caller), 2
// is the verb that called it — CLAUDE.md's "every verb starts with s.Authorize"
// invariant is what makes this constant, not a per-call guess.
//
// Named so every entry point in base.go cannot drift to different skip
// counts — and note this is why each of them calls callerVerb(verbFrameSkip)
// itself rather than delegating to a sibling: one entry point implemented in
// terms of another would add a frame, silently rename every recorded verb to
// "Base.Authorize…", and (since w3/m7 keys the events feed off the verb name)
// empty every service's activity feed without failing anything.
const verbFrameSkip = 2

// helperWalkLimit bounds how many unexported-helper frames callerVerb walks
// past looking for the exported verb — deeper than any real helper chain
// (setSuspended → writeThroughStore is 2), small enough that a pathological
// stack can't make every authorize call crawl it.
const helperWalkLimit = 8

// callerVerb resolves the short "<package>.<Method>" name of the verb that
// called into an authorize entry point, starting skip frames up the stack.
//
// The frame at skip is usually the verb itself ("every verb starts with
// s.Authorize…"). But a verb may share its write path with siblings through an
// UNEXPORTED helper method (apps.Suspend → setSuspended → writeThroughStore →
// AuthorizeApp, the w2/m30 consolidation), and recording the helper's name
// would collapse distinct verbs into one meaningless "apps.writeThroughStore"
// row — un-mapping every one of them from the events feed's and the outbound
// webhooks' verb-keyed vocabularies at once (found live by w3/m11/t008). So:
// walk upward past unexported *methods* — shared helpers by construction —
// until the exported verb method appears. Plain functions and closures are
// never skipped: a test's stand-in verb (audit_test.go's suspendLikeVerb) and
// a handler closure are terminal callers, not shared write paths.
//
// A bad skip count is a bug caught by audit_test.go's expected-verb
// assertions, not a panic (an unresolved frame degrades to "unknown", never
// blocks the verb).
func callerVerb(skip int) string {
	// One unwind for the whole candidate window (runtime.Callers), not one
	// full-stack runtime.Caller per candidate — this runs on every write-verb
	// authorize. +1: Callers counts skip from itself where Caller counts from
	// its caller.
	pcs := make([]uintptr, helperWalkLimit+1)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(skip+1, pcs)])
	first := ""
	for {
		frame, more := frames.Next()
		name := frame.Function
		if name == "" {
			break
		}
		if j := strings.LastIndex(name, "/"); j >= 0 {
			name = name[j+1:]
		}
		short := receiverRE.ReplaceAllString(name, "")
		if first == "" {
			first = short
		}
		if !unexportedHelperMethod(name, short) {
			return short
		}
		if !more {
			break
		}
	}
	if first == "" {
		return "unknown"
	}
	return first // nothing but helpers in budget: degrade to the nearest frame
}

// unexportedHelperMethod reports whether a resolved frame is an unexported
// METHOD ("apps.(*Service).writeThroughStore" → "apps.writeThroughStore") —
// the shared-helper shape callerVerb walks past. name is the runtime name
// after path-trimming (receiver still present); short is the receiver-stripped
// "<pkg>.<func>" form.
func unexportedHelperMethod(name, short string) bool {
	if !receiverRE.MatchString(name) {
		return false // plain function (or nothing to strip): terminal
	}
	_, method, ok := strings.Cut(short, ".")
	if !ok || strings.Contains(method, ".") {
		return false // closures ("….func1") are terminal, never walked past
	}
	r := rune(method[0])
	return r >= 'a' && r <= 'z'
}

// emit records one audit event for an authorize call: a write relation's
// success and denial alike, a read relation's denial only —
// authorizeAndAudit filters before calling. A sink error is logged and
// swallowed, never returned: audit recording must never fail the verb it's
// recording.
func (b *Base) emit(ctx context.Context, verb, resource, target string, authzErr error) {
	outcome := AuditAllowed
	if authzErr != nil {
		outcome = AuditDenied
	}
	ev := AuditEvent{Verb: verb, Resource: resource, Target: target, Outcome: outcome, At: b.Now()}
	if verb == AuditVerbMaintenanceModeEnabled {
		if enabled, ok := ctx.Value(auditMaintenanceModeToKey{}).(bool); ok {
			ev.MaintenanceModeTo = &enabled
		}
	}
	if id, ok := IdentityFrom(ctx); ok {
		ev.Caller, ev.CallerMethod = id.Subject, id.Method
	}
	b.recordAudit(ctx, ev)
}

// recordAudit is the one bounded, fail-open sink path for both authorization
// events and typed system events.
func (b *Base) recordAudit(ctx context.Context, ev AuditEvent) {
	RecordAuditEvent(ctx, b.Audit, ev)
}

// RecordAuditEvent writes ev through sink with the same bounded, fail-open
// semantics as Base's own recording — for the recorders that live outside a
// feature service (the auth gate's invite redemption, internal/api/tenancy.go).
// A nil sink is the store-off no-op; a Record error is logged, never returned.
func RecordAuditEvent(ctx context.Context, sink AuditSink, ev AuditEvent) {
	if sink == nil {
		sink = NoopAuditSink
	}
	recordCtx, cancel := context.WithTimeout(ctx, auditRecordTimeout)
	defer cancel()
	if err := sink.Record(recordCtx, ev); err != nil {
		log.Printf("audit: record %s %s: %v", ev.Verb, ev.Resource, err)
	}
}
