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

// Package audit is the audit-log feature (w4/m10): a read verb over the
// audit_events control-plane table (internal/store) plus the retention sweep
// that bounds both ordinary events and SSH-session audit metadata. The WRITE side is core.Base.Audit
// (core.AuditSink, satisfied directly by *store.PGStore) wired in the
// composition root — this Service never writes an event, it only reads and
// prunes what core.Base.Authorize/AuthorizeOn already recorded. Requires the
// control-plane store (BEX_CP_DB_URI): with it unwired, List reports
// core.ErrAuditUnavailable (503) — omitted, not faked, the deploy-history
// precedent.
package audit

import (
	"context"
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// AuditStore is the Service's seam to the control-plane store — the read and
// retention slice; the write side is core.AuditSink (*store.PGStore
// satisfies both, but this Service only ever calls these two).
type AuditStore interface {
	ListAuditEvents(ctx context.Context, workspaceID string, filter store.AuditFilter) ([]store.AuditRow, error)
	PurgeAuditEvents(ctx context.Context, before time.Time) (int64, error)
	PurgeSSHSessions(ctx context.Context, before time.Time) (int64, error)
	PurgeCLITelemetryEvents(ctx context.Context, before time.Time) (int64, error)
	PurgeProductAnalytics(ctx context.Context, before time.Time) (int64, error)
}

// Service is the audit-log feature. Base carries the authz gate every verb
// calls first. List's AuthorizeOn(RelCanManage, ...) records its denials only:
// reading the trail changes nothing, and recording it wrote one row per page
// (core.readVerbs, w4/m321).
type Service struct {
	*core.Base
	Store AuditStore
	// UserIDs resolves a human actor's subject to its user id, which the
	// Render REST adapter answers as usr- (w5/m277). Nil leaves the subject.
	UserIDs core.UserIDLookup
	// Emails resolves a human actor's subject to the email of the identity it
	// names today (Render's actor.email, w9/m450). The row stores only the
	// subject; a deleted user's identity is gone, so its rows answer the usr-
	// id alone. Nil resolves no email.
	Emails EmailLookup
	// RetentionDays is how long an event survives before the sweep purges it
	// (BEX_AUDIT_RETENTION_DAYS). <1 means DefaultRetentionDays.
	RetentionDays int
}

// EmailLookup resolves identity subjects to their current emails in one
// batched read (workspaces.KratosIdentities). A subject with no identity is
// absent.
type EmailLookup interface {
	LookupEmails(ctx context.Context, subjects []string) (map[string]string, error)
}

// Event is the neutral projection of a store.AuditRow the adapters render in
// Render's audit-log shape.
type Event struct {
	ID                string
	Caller            string
	CallerMethod      string
	Verb              string
	Resource          string
	Target            string // the resource acted ON ("service:my-api"); Render's metadata carries it
	TargetName        string // the target's display name (migration 0038); "" on pre-0038 rows
	Outcome           string
	At                time.Time
	MaintenanceModeTo *bool
	// RoleFrom/RoleTo are the team-membership verbs' typed role detail
	// (w1/m33, migration 0040) — nil for every other verb.
	// RevokedKeyCount is how many API keys a membership removal revoked
	// (w2/m163); nil for every other verb.
	RevokedKeyCount *int32
	RoleFrom        *string
	RoleTo          *string
	// Relation is the RelCan… the decision was made against. Empty on typed
	// system events and pre-0088 rows.
	Relation string
	// OAuthClientID / OAuthAudience / OAuthScopes are verified human-OAuth
	// grant facts. Empty on session, machine, system, and pre-0088 rows.
	OAuthClientID string
	OAuthAudience string
	OAuthScopes   []string
	// OAuthGrantScopes are the scopes of the grant a GrantOAuthAccessEvent
	// records (w4/m454); nil for every other verb.
	OAuthGrantScopes []string
	// A rename's label on each side, a custom-domain verb's hostname and the
	// env group a link changed (w5/183); empty for every other verb.
	DisplayNameFrom string
	DisplayNameTo   string
	Domain          string
	EnvGroupID      string
	EnvGroupName    string
	// TwoFactorRequiredTo is the value a workspace admin set the two-factor
	// requirement TO (w22/m402, migration 0261); nil for every other verb.
	TwoFactorRequiredTo *bool
	// GoogleLoginRequiredTo is the value a workspace admin set the
	// Google-login requirement TO (w7/m465, migration 0294); nil for every
	// other verb.
	GoogleLoginRequiredTo *bool
	// SourceIP and UserAgent are where the actor acted from (w9/m450,
	// migration 0283); empty when no request of the actor caused the row.
	SourceIP  string
	UserAgent string
	// InstanceID is the service instance a shell session ran on (w9/m451,
	// migration 0289); empty on every other row.
	InstanceID string
	// EnvVarKeys are the variable names an env var row created, changed or
	// deleted (w9/m449); nil on every other row.
	EnvVarKeys []string
	// ActorID and ActorEmail are Render's actor.id and actor.email, resolved
	// at read time (resolveActors): a person's usr- id and current email.
	// Empty for every other actor, whose id is its stored subject (Caller).
	ActorID    string
	ActorEmail string
}

func view(r store.AuditRow) Event {
	return Event{
		ID:                r.ID,
		Caller:            r.Caller,
		CallerMethod:      r.CallerMethod,
		Verb:              r.Verb,
		Resource:          r.Resource,
		Target:            r.Target,
		TargetName:        r.TargetName,
		Outcome:           r.Outcome,
		At:                r.At,
		MaintenanceModeTo: r.MaintenanceModeTo,
		RevokedKeyCount:   r.RevokedKeyCount,
		RoleFrom:          r.RoleFrom,
		RoleTo:            r.RoleTo,
		Relation:          r.Relation,
		OAuthClientID:     r.OAuthClientID,
		OAuthAudience:     r.OAuthAudience,
		OAuthScopes:       r.OAuthScopes,
		OAuthGrantScopes:  r.OAuthGrantScopes,
		DisplayNameFrom:   r.DisplayNameFrom,
		DisplayNameTo:     r.DisplayNameTo,
		Domain:            r.Domain,
		EnvGroupID:        r.EnvGroupID,
		EnvGroupName:      r.EnvGroupName,

		TwoFactorRequiredTo:   r.TwoFactorRequiredTo,
		GoogleLoginRequiredTo: r.GoogleLoginRequiredTo,
		SourceIP:              r.SourceIP,
		UserAgent:             r.UserAgent,
		InstanceID:            r.InstanceID,
		EnvVarKeys:            r.EnvVarKeys,
	}
}

// Render's direction vocabulary (api-docs.render.com list-owner-audit-logs) —
// the shared enum core.ParseDirection validates for every list surface that
// honors it, aliased here for the adapters' convenience.
const (
	DirectionBackward = core.DirectionBackward
	DirectionForward  = core.DirectionForward
)

// renderEvents maps a bex verb onto Render's audit-log event vocabulary
// (docs/render-artifacts/audit-logs-api.md, w4/m26) where an unambiguous 1:1
// exists — the lifecycle verbs plus the ip-allow-list and service-rename
// writes; every other verb passes through renderEvent as bex's own
// "<feature>.<Verb>" event value — a documented extension, never a guessed
// mapping onto a Render name whose semantics might not match. The SINGLE
// source of this vocabulary, consumed by both the REST and GraphQL fragments
// (the events feed's eventTypes and webhooks' verbEvents are its snake_case
// siblings for their own surfaces); TestRenderEventVerbsExist guards the
// keys against silent service-method renames.
var renderEvents = map[string]string{
	"apps.Create":                 "CreateServerEvent",
	"apps.Delete":                 "DeleteServerEvent",
	"apps.Suspend":                "SuspendServiceEvent",
	"apps.Resume":                 "ResumeServiceEvent",
	"apps.SetDisplayName":         "UpdateServiceNameEvent",
	"apps.SetIPAllowList":         "UpdateIPAllowListEvent",
	"postgres.CreatePostgres":     "CreatePostgresEvent",
	"postgres.DeletePostgres":     "DeletePostgresEvent",
	"postgres.Suspend":            "SuspendPostgresEvent",
	"postgres.Resume":             "ResumePostgresEvent",
	"postgres.SetIPAllowList":     "UpdateIPAllowListEvent",
	"keyvalue.CreateKeyValue":     "CreateRedisEvent",
	"keyvalue.DeleteKeyValue":     "DeleteRedisEvent",
	"keyvalue.SetIPAllowList":     "UpdateIPAllowListEvent",
	"keyvalue.EnableInternalAuth": "EnableRedisInternalAuthEvent",
	"projects.Create":             "CreateProjectEvent",
	"projects.Delete":             "DeleteProjectEvent",
	"workspaces.Create":           "CreateWorkspaceEvent",
	"workspaces.Delete":           "DeleteWorkspaceEvent",
	// Render's org events (w7/m464, ADR115 D7), recorded in the org's own
	// log (renderOrganizationEvents is the inventory).
	core.AuditVerbOrgInvited:              "InviteToOrgEvent",
	core.AuditVerbOrgInviteAccepted:       "AcceptOrgInviteEvent",
	core.AuditVerbOrgMemberAdded:          "AddOrgMemberEvent",
	core.AuditVerbOrgMemberRemoved:        "RemoveOrgMemberEvent",
	core.AuditVerbOrgRoleChanged:          "ChangeOrgRoleEvent",
	core.AuditVerbJoinWorkspace:           "JoinTeamEvent",
	core.AuditVerbJoinWorkspaceAsAdmin:    "JoinTeamEvent",
	core.AuditVerbOrgRenamed:              "ChangeOrgNameEvent",
	core.AuditVerbOrgWorkspaceCreated:     "CreateWorkspaceEvent",
	core.AuditVerbWorkspacePrivacyChanged: "ChangeWorkspacePrivacyEvent",
	// The org's login settings (w7/m466); each entry carries the value it was
	// set to, as the workspace settings' entries do.
	core.AuditVerbOrgSetRequireTwoFactor:   "ChangeOrg2FAEnforcementEvent",
	core.AuditVerbOrgSetRequireGoogleLogin: "ChangeOrgAllowedLoginMethodsEvent",
	// Render's workspace "require two-factor authentication" enforcement
	// (w22/m402); the entry carries the value it was set to.
	core.AuditVerbSetRequireTwoFactor: "ChangeTeam2FAEnforcementEvent",
	// Render's workspace Google-login enforcement (w7/m465), its allowed
	// login methods; the entry carries the value it was set to.
	core.AuditVerbSetRequireGoogleLogin: "ChangeTeamAllowedLoginMethodsEvent",
	"environments.Create":               "CreateEnvironmentEvent",
	"environments.Delete":               "DeleteEnvironmentEvent",

	// The verbs every surface actually calls, and the rest of Render's 1:1
	// actions (w4/m321). A project or environment delete and a service move
	// record one row each.
	"projects.CreateWithEnvironments":     "CreateProjectEvent",
	"projects.DeleteIfEmpty":              "DeleteProjectEvent",
	"environments.CreateWithACL":          "CreateEnvironmentEvent",
	"environments.DeleteIfEmpty":          "DeleteEnvironmentEvent",
	core.AuditVerbProjectServiceMoved:     "MoveEnvironmentResourceEvent",
	core.AuditVerbEnvironmentServiceMoved: "MoveEnvironmentResourceEvent",
	core.AuditVerbSetWorkspaceIPAllowList: "UpdateIPAllowListEvent",
	"webhooks.Create":                     "CreateWebhookEvent",
	"webhooks.Update":                     "UpdateWebhookEvent",
	"webhooks.Delete":                     "DeleteWebhookEvent",
	"envgroups.DeleteEnvGroup":            "DeleteEnvGroupEvent",
	core.AuditVerbAddDisk:                 "CreateServerDiskEvent",
	core.AuditVerbDeleteDisk:              "DeleteServerDiskEvent",
	core.AuditVerbRestoreDiskSnapshot:     "RestoreDiskSnapshotEvent",
	"apps.ApproveBlueprint":               "ApplyBlueprintEvent",
	"apps.SyncBlueprint":                  "ApplyBlueprintEvent",
	"apps.CreateBuildSource":              "CreateBuildSourceEvent",
	"apps.UpdateBuildSource":              "UpdateBuildSourceEvent",
	"apps.DeleteBuildSource":              "DeleteBuildSourceEvent",
	core.AuditVerbDeleteEnvVar:            "DeleteEnvVarsEvent",

	core.AuditVerbMaintenanceModeEnabled:    "MaintenanceModeEnabledEvent",
	core.AuditVerbMaintenanceModeURIUpdated: "MaintenanceModeURIUpdatedEvent",
	core.AuditVerbUpdatePaymentMethod:       "UpdateWorkspacePaymentMethodEvent",
	core.AuditVerbViewEnvVarValues:          "ViewEnvVarValuesEvent",
	core.AuditVerbViewConnectionInfo:        "ViewConnectionInfoEvent",
	// A list that handed out a Postgres logical backup's download URL (w21/105).
	core.AuditVerbDownloadDatabaseBackup: "DownloadDatabaseBackupEvent",

	// The rest of Render's workspace events bex has the action for (w9/m449;
	// renderWorkspaceEvents is the inventory). A member leaving is Render's
	// removal too: Render documents no leave event, and the actor (the member
	// themselves) tells the two apart. Resending or revoking an invite and
	// verifying a domain have no Render event and keep bex's names.
	core.AuditVerbMemberInvited:     "InviteToTeamEvent",
	core.AuditVerbInviteAccepted:    "AcceptTeamInviteEvent",
	core.AuditVerbMemberRoleChanged: "ChangeTeamMemberRoleEvent",
	core.AuditVerbMemberRemoved:     "RemoveUserFromTeamEvent",
	core.AuditVerbMemberLeft:        "RemoveUserFromTeamEvent",
	core.AuditVerbCreateCronJob:     "CreateCronJobEvent",
	core.AuditVerbDeleteCronJob:     "DeleteCronJobEvent",
	core.AuditVerbAddDomain:         "CreateCustomDomainEvent",
	core.AuditVerbDeleteDomain:      "DeleteCustomDomainEvent",
	// Auto-deploy turned on or off; a change between two triggers that both
	// deploy, and a build filter change, are its trigger settings.
	core.AuditVerbSetAutoDeploy:             "UpdateAutoDeployEvent",
	core.AuditVerbSetAutoDeployTrigger:      "UpdateAutoDeployTriggerEvent",
	core.AuditVerbSetBuildFilter:            "UpdateAutoDeployTriggerEvent",
	core.AuditVerbLinkBuildSource:           "UpdateServiceBuildSourceEvent",
	core.AuditVerbLinkBuildSourceEnvGroup:   "LinkBuildSourceEnvGroupEvent",
	core.AuditVerbUnlinkBuildSourceEnvGroup: "UnlinkBuildSourceEnvGroupEvent",
	// One env var save's per-kind rows, each naming its keys; the save's own
	// secrets.* row is feed_only (core.RecordEnvVarWrite).
	core.AuditVerbCreateEnvVars:             "CreateEnvVarsEvent",
	core.AuditVerbUpdateEnvVars:             "UpdateEnvVarsEvent",
	core.AuditVerbDeleteEnvVars:             "DeleteEnvVarsEvent",
	core.AuditVerbSetEnvironmentProtection:  "ChangeEnvironmentProtectionEvent",
	core.AuditVerbSetEnvironmentIsolation:   "UpdateEnvironmentIsolatedEvent",
	core.AuditVerbSetEnvironmentIPAllowList: "UpdateIPAllowListEvent",
	// A member approving or revoking an OAuth client's access (w4/m454). The
	// rows the operator's revoke-oauth-grant wrote as oauth.RevokeGrant before
	// it read as the same event until the retention sweep removes them.
	core.AuditVerbGrantOAuthAccess:  "GrantOAuthAccessEvent",
	core.AuditVerbRevokeOAuthAccess: "RevokeOAuthAccessEvent",
	legacyVerbOperatorRevokeGrant:   "RevokeOAuthAccessEvent",
	// A shell session into a service instance (w9/m451): the gateway's start
	// and end rows, and the denied row of either verb that refuses an
	// identified caller's shell (only a denial of these read-relation verbs is
	// ever recorded, so their rows are always StartShellEvent with status
	// error). An agent-session sandbox's SSH authorizes through agentsessions
	// and records neither.
	// A member signing in and out (w9/m452), recorded once in each of their
	// workspaces by internal/signins.
	core.AuditVerbLogin:       "LoginEvent",
	core.AuditVerbLogout:      "LogoutEvent",
	core.AuditVerbStartShell:  "StartShellEvent",
	core.AuditVerbEndShell:    "EndShellEvent",
	"apps.ResolveSSHSession":  "StartShellEvent",
	"apps.CreateShellSession": "StartShellEvent",
}

// docsOnlyEvents are Render events render.com/docs/audit-logs (read
// 2026-11-16) documents that the pinned OpenAPI enum does not carry yet. bex
// emits them as it emits a bex verb outside the enum, a documented extension
// (docs/render-artifacts/audit-logs-api.md).
var docsOnlyEvents = map[string]bool{
	"UpdateAutoDeployEvent":          true,
	"UpdateAutoDeployTriggerEvent":   true,
	"CreateCustomDomainEvent":        true,
	"DeleteCustomDomainEvent":        true,
	"UpdateEnvironmentIsolatedEvent": true,
}

// legacyVerbOperatorRevokeGrant is the verb the operator's revoke-oauth-grant
// recorded before w4/m454 gave it RevokeOAuthAccessEvent's own.
const legacyVerbOperatorRevokeGrant = "oauth.RevokeGrant"

func renderEvent(verb string) string {
	if ev, ok := renderEvents[verb]; ok {
		return ev
	}
	return verb
}

// denied is the one place the package reads the binary outcome vocabulary —
// REST's success|error mapping, its metadata denied marker, and GraphQL's
// richer denied status all consult it, so the three can't drift if the
// outcome vocabulary ever grows.
func denied(e Event) bool { return e.Outcome == string(core.AuditDenied) }

// Filter narrows List — the neutral shape the REST/GraphQL adapters translate
// Render's startTime/endTime/direction/cursor/limit query params into.
type Filter struct {
	Since time.Time
	Until time.Time
	// Direction is Render's ordering enum: "" or DirectionBackward is
	// newest-first, DirectionForward oldest-first; anything else is a named
	// 400 (w3/m8's "nothing accepted is ignored" — w4/013 closed the drift
	// where this param was accepted and ignored). Validated here in the verb
	// so REST and GraphQL cannot diverge.
	Direction string
	Cursor    string
	// Limit is clamped, not rejected, when non-positive or above the audit cap
	// (both fall back to the default page, not to the cap): the
	// store substitutes a real default page (store.DefaultAuditPageSize)
	// rather than reading <=0 as "unbounded", which is the discriminator
	// core.ErrLimitNotPositive documents between the two limit policies. The
	// deploys/jobs lists reject for exactly the opposite reason.
	Limit int
}

// FilterOf builds a Filter from the params in the string form every adapter
// has them in (a query value, a GraphQL argument) — one translator for both
// surfaces, the deploys.FilterOf/jobs.FilterFromStrings precedent, so a REST
// call and a GraphQL query with the same params cannot page differently.
//
// A malformed time bound is a named 400, NOT a silently dropped filter. Both
// adapters used to parse these inline with `if err == nil { assign }`, so
// `?startTime=yesterday` answered with the store's default page of the
// caller's whole history, UNFILTERED by the window they asked for — a wrong
// answer shaped exactly like the right one, with no error to notice. (The read
// was never row-unbounded: Limit stayed 0 and the store substitutes
// store.DefaultAuditPageSize. It is the dropped FILTER that is the defect.)
//
// That is the same rule the Direction field above already carries (w3/m8,
// "nothing accepted is ignored"); it had simply never been applied to the time
// bounds beside it.
func FilterOf(startTime, endTime, direction, cursor string, limit int) (Filter, error) {
	f := Filter{Direction: direction, Cursor: cursor, Limit: limit}
	var err error
	if f.Since, err = core.ParseTime("startTime", startTime); err != nil {
		return Filter{}, err
	}
	if f.Until, err = core.ParseTime("endTime", endTime); err != nil {
		return Filter{}, err
	}
	return f, nil
}

// List returns ownerID's audit trail — newest-first unless
// filter.Direction is DirectionForward (Render's GET
// /owners/{id}/audit-logs) — admin-scoped (RelCanManage), the same bar
// workspace rename/delete uses: only a workspace's own admins may see who did
// what to it. ownerID must be the caller's own workspace — AuthorizeOn checks
// the named object directly, so a caller from another workspace naming this
// one by id gets ErrForbidden, never a leak of its trail through a guessed id.
func (s *Service) List(ctx context.Context, ownerID string, filter Filter) ([]Event, error) {
	ownerID, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, ownerID)
	if err != nil {
		return nil, err
	}
	// Render: audit logs are a Pro-and-higher feature (w8/m138), and their
	// history starts at the upgrade to Pro: "Retention starts only when you
	// upgrade to Pro or higher. Earlier data is unavailable." (w9/098). Events
	// are still recorded on Hobby; only this read hides them, so REST, GraphQL
	// and the dashboard's CSV export all start at the same instant.
	plan, visibleFrom, gated, err := s.WorkspaceAuditPlanOf(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if gated && !core.PlanAllows(plan, core.FeatureAuditLogs) {
		return nil, core.NewPlanFeatureError("", plan, core.FeatureAuditLogs)
	}
	return s.list(ctx, ownerID, visibleFrom, filter)
}

// list reads one log, a workspace's or an org's (its rows are keyed by the
// org id), from visibleFrom on.
func (s *Service) list(ctx context.Context, key string, visibleFrom time.Time, filter Filter) ([]Event, error) {
	if visibleFrom.After(filter.Since) {
		filter.Since = visibleFrom
	}
	oldestFirst, err := core.ParseDirection(filter.Direction)
	if err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrAuditUnavailable
	}
	rows, err := s.Store.ListAuditEvents(ctx, key, store.AuditFilter{
		Since: filter.Since, Until: filter.Until, Cursor: filter.Cursor, Limit: filter.Limit,
		OldestFirst: oldestFirst,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = view(r)
	}
	return out, nil
}

// DefaultRetentionDays is the default retention window (t002): 90 days
// matches Render's documented minimum audit-log retention
// (render.com/docs/audit-logs), a reasonable bex default absent its own
// compliance requirement.
const DefaultRetentionDays = 90

// RetentionInterval is the sweep cadence — daily, the same cadence usage's
// compaction loop (w8/m4) uses for its retention pass.
const RetentionInterval = 24 * time.Hour

// Run starts the retention sweep loop; blocks until ctx is cancelled. Only
// meaningful with Store wired (BEX_CP_DB_URI) — cmd/api/main.go starts this
// in a goroutine alongside the usage retention loop.
func (s *Service) Run(ctx context.Context) {
	s.RunWithInterval(ctx, RetentionInterval)
}

func (s *Service) RunWithInterval(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.purge(ctx) // once at startup so restarts don't defer the first sweep a full interval
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.purge(ctx)
		}
	}
}

func (s *Service) purge(ctx context.Context) {
	days := s.RetentionDays
	if days < 1 {
		days = DefaultRetentionDays
	}
	before := s.Now().UTC().AddDate(0, 0, -days)
	events, err := s.Store.PurgeAuditEvents(ctx, before)
	if err != nil {
		log.Printf("audit: purge events before %s: %v", before.Format(time.RFC3339), err)
		return
	}
	sessions, err := s.Store.PurgeSSHSessions(ctx, before)
	if err != nil {
		log.Printf("audit: purge SSH sessions before %s: %v", before.Format(time.RFC3339), err)
		return
	}
	telemetry, err := s.Store.PurgeCLITelemetryEvents(ctx, before)
	if err != nil {
		log.Printf("audit: purge CLI telemetry events before %s: %v", before.Format(time.RFC3339), err)
		return
	}
	product, err := s.Store.PurgeProductAnalytics(ctx, before)
	if err != nil {
		log.Printf("audit: purge product analytics before %s: %v", before.Format(time.RFC3339), err)
		return
	}
	if events+sessions+telemetry+product > 0 {
		log.Printf("audit: purged %d events, %d SSH sessions, %d CLI telemetry events, and %d product analytics rows older than %s", events, sessions, telemetry, product, before.Format(time.RFC3339))
	}
}
