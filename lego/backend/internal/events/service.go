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

// Package events is the per-service activity feed (w3/m7): Render's
// GET /services/{id}/events, composed from the durable control-plane facts bex
// already needs for deploy history, audit, and observed transitions.
//
// # Sources
//
// Public events are derived from three closed sources. There is no generic
// event payload or adapter-side emitter:
//
//	deploys       → deploy_started (created_at) and deploy_ended (finished_at)
//	audit_events  → every other type, one row per authorized write verb, keyed by
//	                target = core.ServiceTarget(app) and mapped through eventTypes
//	service_event_facts
//	              → typed operator-observed and signed-Git decisions which are
//	                neither a deploy timestamp nor an authorized API write
//
// # Vocabulary (Render's names where its OpenAPI defines one, bex-named otherwise)
//
// Render's list-events enum has 39 types; bex emits the subset its three sources
// can support truthfully, under Render's exact names:
//
//	deploy_started              deploys row opened
//	deploy_ended                deploys row closed (details.deployStatus succeeded|failed)
//	suspender_added             apps.Suspend      (details.actor = the caller)
//	suspender_removed           apps.Resume       (details.actor = the caller)
//	server_restarted            apps.Restart      (details.triggeredByUser = the caller)
//	plan_changed                apps.SetPlan      (details.from/to = plan name strings)
//	instance_count_changed      apps.Scale        (details.from/to = instance counts)
//	autoscaling_config_changed  apps.SetAutoscaling / apps.DeleteAutoscaling (details.previous/current min+max)
//	cron_job_run_started        apps.TriggerCronRun
//	cron_job_run_ended          apps.CancelCronRun / apps.CancelCurrentCronRun
//
// and these bex-named types, for writes Render's vocabulary has no name for:
//
//	env_vars_changed            secrets.Set/DeleteEnvVar(s)     (KEYS and VALUES both absent — see Redaction)
//	service_environment_changed secrets.PatchEnvironment        (env vars + files; names and values absent)
//	service_moved               projects.MoveService / environments.MoveService (w6/m134: one row per
//	                            service whose project/environment placement changed in a successful
//	                            bulk SetServices replacement; details carry the before/after public
//	                            prj-/env- ids — NOT the env-var fact service_environment_changed is)
//	env_group_linked/unlinked   envgroups.Link/UnlinkService
//	auto_deploy_enabled         apps.SetAutoDeploy(enabled=true)  — new rows only
//	auto_deploy_disabled        apps.SetAutoDeploy(enabled=false) — new rows only
//	auto_deploy_changed         apps.SetAutoDeploy — legacy rows without a recorded boolean
//	idle_timeout_changed        apps.SetIdleTTL                 (a bex-only feature: "sleep = free")
//	root_directory_changed      apps.SetRootDir
//	dockerfile_path_changed     apps.SetDockerfilePath
//	build_filter_changed        apps.SetBuildFilter
//	commands_changed            apps.SetCommands
//	source_changed              apps.SetSourceAndRegistryCredential / apps.SetRegistryCredential (legacy rows may still name apps.SetSource)
//	display_name_changed        apps.SetDisplayName
//	pre_deploy_command_changed  apps.SetPreDeployCommand
//	max_shutdown_delay_changed  apps.SetMaxShutdownDelay
//	publish_path_changed        apps.SetPublishPath
//	routes_changed              apps.SetRoutes
//	headers_changed             apps.SetHeaders
//	custom_domain_added/removed/verified apps.Add/Delete/VerifyDomain
//	notify_on_fail_changed      apps.SetNotifyOnFail/SetNotificationsToSend (w4/m21 + w3/m15, a bex-only name — Render has no dedicated event type)
//	subdomain_policy_changed    apps.SetSubdomainPolicy         (w7/m31, a bex-only name — Render's renderSubdomainPolicy has no dedicated event type)
//	maintenance_mode_enabled    apps.SetMaintenanceMode         (w1/m37, matching Render's webhook/audit vocabulary; audit metadata.to distinguishes enable from disable)
//	maintenance_mode_uri_updated
//	                            apps.SetMaintenanceModeURI      (w1/m37, matching Render's webhook/audit vocabulary)
//
// # Redaction (structural, not filtered)
//
// No arbitrary value can reach an event: an audit row carries a verb NAME, a
// caller subject, and (since w3/m7) a target resource NAME — never a generic
// verb-arguments object. The typed detail fields (plan from/to, instance counts,
// autoscaling min/max, auto_deploy_enabled) are non-secret scalars stored in
// dedicated typed columns on audit_events — the same structural discipline as
// maintenance_mode_to. An env-var write is therefore visible as "alice changed
// env vars at 03:12" and cannot be anything more, however the feed is queried.
//
// # Deploy-lifecycle facts (w7/m66)
//
// The build, pre-deploy, and one-off-job beats Render shows as distinct timeline
// entries are observed control-plane facts on the same service_event_facts path
// image_pull_failed rides — no faking, a durable source each:
//
//	build_started / build_ended            reconciler observing a repo-backed
//	                                       deploy's BuildKit build phase (ADR034),
//	                                       or the Cancel verb closing one directly
//	                                       (w6/m128, the one lifecycle transition
//	                                       the reconciler never revisits);
//	                                       build_ended.details.status is
//	                                       succeeded|failed|canceled
//	pre_deploy_started / pre_deploy_ended  reconciler observing status.preDeploy
//	                                       (w1/m33); the ended details.status is
//	                                       succeeded|failed (in addition to the
//	                                       preDeployStatus still on deploy_ended)
//	job_run_ended                          internal/jobs observing a one-off job
//	                                       finishing (details.status
//	                                       succeeded|failed) — alongside the
//	                                       existing job_started/job_canceled
//	branch_deleted                         the GitHub webhook's branch-delete
//	                                       signal (push deleted=true, or the
//	                                       `delete` event); auto-deploy is disabled
//
// # Omissions (honest, not faked)
//
//   - provider hardware/maintenance, billing, workflow, preview-environment,
//     persistent-disk, and mutable edge-cache events have no accepted bex
//     mechanism or durable source.
//     (maintenance_* now has a source — see the maintenance-mode types above —
//     but only for the tenant-facing toggle this feed's apps.SetMaintenanceMode
//     records; Render's platform-scheduled infra-maintenance concept, if its
//     enum names one, still has none.)
//
// Requires the control-plane store (BEX_CP_DB_URI): all sources live there, so
// with it unwired the verb reports core.ErrEventsUnavailable (503) — omitted, not
// faked, the deploy-history precedent.
package events

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/eventvocab"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// EventStore is the Service's seam to the control-plane store: one composed list
// and one owner-indexed source lookup. *store.PGStore satisfies it.
type EventStore interface {
	ListServiceEvents(ctx context.Context, appID, target, ownerWorkspace string, f store.ServiceEventFilter) ([]store.ServiceEventRow, error)
	GetServiceEvent(ctx context.Context, workspaceID, eventID string) (store.ServiceEventLookup, error)
}

// Render's event types (the subset bex can emit truthfully), spelled exactly as
// its list-events enum spells them — a Render-trained client switches on these.
const (
	TypeDeployStarted            = "deploy_started"
	TypeDeployEnded              = "deploy_ended"
	TypeSuspenderAdded           = "suspender_added"
	TypeSuspenderRemoved         = "suspender_removed"
	TypeServerRestarted          = "server_restarted"
	TypePlanChanged              = eventvocab.TypePlanChanged
	TypeInstanceCountChanged     = "instance_count_changed"
	TypeAutoscalingConfigChanged = "autoscaling_config_changed"
	TypeCronJobRunStarted        = "cron_job_run_started"
	TypeCronJobRunEnded          = "cron_job_run_ended"
	TypeImagePullFailed          = "image_pull_failed"
	TypeServiceSuspended         = "service_suspended"
	TypeServiceResumed           = "service_resumed"
	// Free-tier idle auto-sleep (w6/m47), distinct from the user-driven pair
	// above so the timeline never claims a human suspended a service that
	// simply went to sleep on its idle timeout.
	TypeServiceHibernated  = "service_hibernated"
	TypeServiceWoken       = "service_woken"
	TypeServerFailed       = "server_failed"
	TypeServerAvailable    = "server_available"
	TypeBranchChanged      = "branch_changed"
	TypeBranchDeleted      = "branch_deleted"
	TypeCommitIgnored      = "commit_ignored"
	TypeAutoscalingStarted = "autoscaling_started"
	TypeAutoscalingEnded   = "autoscaling_ended"
	// Deploy-lifecycle facts (w7/m66): the build, pre-deploy, and one-off-job
	// beats Render shows as distinct timeline entries. The *_ended types carry a
	// details.status (succeeded|failed|canceled); observed via the control-plane
	// reconciler + jobs sync, not an API write.
	TypeBuildStarted               = "build_started"
	TypeBuildEnded                 = "build_ended"
	TypePreDeployStarted           = "pre_deploy_started"
	TypePreDeployEnded             = "pre_deploy_ended"
	TypeJobRunEnded                = "job_run_ended"
	TypePostgresCreated            = eventvocab.TypePostgresCreated
	TypePostgresRestarted          = eventvocab.TypePostgresRestarted
	TypePostgresCredentialsCreated = eventvocab.TypePostgresCredentialsCreated
	TypePostgresCredentialsDeleted = eventvocab.TypePostgresCredentialsDeleted
	TypePostgresBackupStarted      = eventvocab.TypePostgresBackupStarted
	// Field-level datastore configuration changes (w3/m82). Like their siblings
	// above they are indexed audit effects with no service-scoped list home;
	// details carry the value the field was set to.
	TypePostgresHAStatusChanged              = eventvocab.TypePostgresHAStatusChanged
	TypePostgresConnectionPoolEnabledChanged = eventvocab.TypePostgresConnectionPoolEnabledChanged
	TypePostgresDiskSizeChanged              = eventvocab.TypePostgresDiskSizeChanged
	TypeKeyValueConfigRestart                = eventvocab.TypeKeyValueConfigRestart
	// Observed datastore lifecycle (w3/m82): typed datastore_event_facts rows
	// the control-plane reconciler records, not audit effects. Like the audit
	// names above they have no service-scoped list home — a datastore is not an
	// App, so GET /services/{id}/events can never join them — which is why they
	// are absent from allFactTypes and reachable only through Get by evt-… id.
	TypePostgresUnavailable      = eventvocab.TypePostgresUnavailable
	TypePostgresAvailable        = eventvocab.TypePostgresAvailable
	TypeKeyValueUnhealthy        = eventvocab.TypeKeyValueUnhealthy
	TypeKeyValueAvailable        = eventvocab.TypeKeyValueAvailable
	TypePostgresBackupCompleted  = eventvocab.TypePostgresBackupCompleted
	TypePostgresBackupFailed     = eventvocab.TypePostgresBackupFailed
	TypePostgresRestoreSucceeded = eventvocab.TypePostgresRestoreSucceeded
	TypePostgresRestoreFailed    = eventvocab.TypePostgresRestoreFailed
	TypePostgresUpgradeStarted   = eventvocab.TypePostgresUpgradeStarted
	TypePostgresUpgradeSucceeded = eventvocab.TypePostgresUpgradeSucceeded
	TypePostgresUpgradeFailed    = eventvocab.TypePostgresUpgradeFailed
)

// bex-named types — real writes Render's vocabulary has no name for. Named in
// Render's snake_case house style so they read as one vocabulary.
const (
	TypeEnvVarsChanged            = "env_vars_changed"
	TypeServiceEnvironmentChanged = "service_environment_changed"
	// TypeServiceMoved is a project/environment reassignment (w6/m134) —
	// deliberately NOT TypeServiceEnvironmentChanged, which despite its name is
	// the env-var/config-rollout fact. Render's pinned enums have no
	// membership-move type (render-public-api-1.json eventTypeParam +
	// docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-08-17.json,
	// re-checked 2026-08-28), so this is a bex extension.
	TypeServiceMoved     = "service_moved"
	TypeEnvGroupLinked   = "env_group_linked"
	TypeEnvGroupUnlinked = "env_group_unlinked"
	// TypeAutoDeployEnabled and TypeAutoDeployDisabled replace the bex-named
	// TypeAutoDeployChanged for new rows that carry the auto_deploy_enabled boolean.
	// Legacy rows without a recorded value still produce TypeAutoDeployChanged.
	TypeAutoDeployEnabled  = "auto_deploy_enabled"
	TypeAutoDeployDisabled = "auto_deploy_disabled"
	// TypeAutoDeployChanged is the bex-named fallback for legacy audit rows without
	// a recorded auto_deploy_enabled value. New rows always produce
	// TypeAutoDeployEnabled or TypeAutoDeployDisabled.
	TypeAutoDeployChanged         = "auto_deploy_changed"
	TypeIdleTimeoutChanged        = "idle_timeout_changed"
	TypeRootDirectoryChanged      = "root_directory_changed"
	TypeDockerfilePathChanged     = "dockerfile_path_changed"
	TypeBuildFilterChanged        = "build_filter_changed"
	TypeCommandsChanged           = "commands_changed"
	TypeSourceChanged             = "source_changed"
	TypeDisplayNameChanged        = "display_name_changed"
	TypePreDeployChanged          = "pre_deploy_command_changed"
	TypeMaxShutdownDelayChanged   = "max_shutdown_delay_changed"
	TypePublishPathChanged        = "publish_path_changed"
	TypeRoutesChanged             = "routes_changed"
	TypeHeadersChanged            = "headers_changed"
	// Disk lifecycle types match Render's eventTypeParam / webhook enum
	// (w8/m34): disk_created/disk_deleted — not the earlier bex spellings
	// disk_attached/disk_detached. TypeDiskRestored is a labeled bex extension
	// (Render has no restore-from-snapshot event).
	TypeDiskCreated  = "disk_created"
	TypeDiskUpdated  = "disk_updated"
	TypeDiskDeleted  = "disk_deleted"
	TypeDiskRestored = "disk_restored"
	TypeCustomDomainAdded         = "custom_domain_added"
	TypeCustomDomainRemoved       = "custom_domain_removed"
	TypeCustomDomainVerified      = "custom_domain_verified"
	TypeDeployHookRegenerated     = "deploy_hook_regenerated"
	TypeNotifyOnFailChanged       = "notify_on_fail_changed"
	TypeSubdomainPolicyChanged    = "subdomain_policy_changed"
	TypeIPAllowListChanged        = "ip_allow_list_changed"
	TypeJobStarted                = "job_started"
	TypeJobCanceled               = "job_canceled"
	TypeMaintenanceModeEnabled    = "maintenance_mode_enabled"
	TypeMaintenanceModeURIUpdated = "maintenance_mode_uri_updated"
)

// eventTypes maps an audited verb (core.callerVerb's "<package>.<Method>") to the
// event type it produces. It is the SINGLE source of the vocabulary: the store
// filters audit rows to exactly these verbs (auditVerbs), so a verb absent here
// is not an event and a page is never silently short.
//
// Deliberately absent:
//   - apps.Create — its first deploy already appears as deploy_started with
//     trigger.firstBuild, which is how Render itself shows a service's birth.
//   - apps.Delete — the service (and its feed) is gone; the row survives in the
//     workspace audit log, which is where "who deleted it" belongs.
//   - deploys.Trigger — the deploys row it opens IS the deploy_started event;
//     mapping the verb too would show every API deploy twice.
var eventTypes = map[string]string{
	"apps.Suspend":                          TypeSuspenderAdded,
	"apps.Resume":                           TypeSuspenderRemoved,
	"apps.Restart":                          TypeServerRestarted,
	"apps.SetPlan":                          TypePlanChanged,
	"apps.Scale":                            TypeInstanceCountChanged,
	"apps.SetAutoscaling":                   TypeAutoscalingConfigChanged,
	"apps.DeleteAutoscaling":                TypeAutoscalingConfigChanged,
	"apps.TriggerCronRun":                   TypeCronJobRunStarted,
	"apps.CancelCronRun":                    TypeCronJobRunEnded,
	"apps.CancelCurrentCronRun":             TypeCronJobRunEnded,
	"apps.SetAutoDeploy":                    TypeAutoDeployChanged,
	"apps.SetNotifyOnFail":                  TypeNotifyOnFailChanged,
	"apps.SetNotificationsToSend":           TypeNotifyOnFailChanged,
	"apps.SetSubdomainPolicy":               TypeSubdomainPolicyChanged,
	"apps.SetIPAllowList":                   TypeIPAllowListChanged,
	"apps.SetIdleTTL":                       TypeIdleTimeoutChanged,
	"apps.SetRootDir":                       TypeRootDirectoryChanged,
	"apps.SetDockerfilePath":                TypeDockerfilePathChanged,
	"apps.SetBuildFilter":                   TypeBuildFilterChanged,
	"apps.SetCommands":                      TypeCommandsChanged,
	"apps.SetSource":                        TypeSourceChanged,
	"apps.SetRegistryCredential":            TypeSourceChanged,
	"apps.SetSourceAndRegistryCredential":   TypeSourceChanged,
	"apps.SetDisplayName":                   TypeDisplayNameChanged,
	"apps.SetPreDeployCommand":              TypePreDeployChanged,
	"apps.SetMaxShutdownDelay":              TypeMaxShutdownDelayChanged,
	"apps.SetPublishPath":                   TypePublishPathChanged,
	"apps.SetRoutes":                        TypeRoutesChanged,
	"apps.SetHeaders":                       TypeHeadersChanged,
	"apps.AddDisk":                          TypeDiskCreated,
	"apps.UpdateDisk":                       TypeDiskUpdated,
	"apps.DeleteDisk":                       TypeDiskDeleted,
	"apps.RestoreDiskSnapshot":              TypeDiskRestored,
	"apps.AddDomain":                        TypeCustomDomainAdded,
	"apps.DeleteDomain":                     TypeCustomDomainRemoved,
	"apps.VerifyDomain":                     TypeCustomDomainVerified,
	"secrets.SetEnvVars":                    TypeEnvVarsChanged,
	"secrets.SetEnvVar":                     TypeEnvVarsChanged,
	"secrets.DeleteEnvVar":                  TypeEnvVarsChanged,
	"secrets.SeedEnvVars":                   TypeEnvVarsChanged, // blueprint seed-once (w1/m35)
	"secrets.PatchEnvironment":              TypeServiceEnvironmentChanged,
	core.AuditVerbProjectServiceMoved:       TypeServiceMoved,
	core.AuditVerbEnvironmentServiceMoved:   TypeServiceMoved,
	"envgroups.LinkService":                 TypeEnvGroupLinked,
	"envgroups.UnlinkService":               TypeEnvGroupUnlinked,
	"envgroups.LinkEnvGroup":                TypeEnvGroupLinked, // blueprint fromGroup (w1/m35)
	"deploys.RegenerateDeployHook":          TypeDeployHookRegenerated,
	"jobs.Create":                           TypeJobStarted,
	"jobs.Cancel":                           TypeJobCanceled,
	core.AuditVerbMaintenanceModeEnabled:    TypeMaintenanceModeEnabled,
	core.AuditVerbMaintenanceModeURIUpdated: TypeMaintenanceModeURIUpdated,
}

// indexedAuditEventTypes are webhook-visible datastore effects that have no
// service-scoped list home. They are intentionally separate from eventTypes:
// adding them there would claim Postgres/Key Value rows can appear in
// GET /services/{id}/events and would broaden that query's audit vocabulary.
var indexedAuditEventTypes = eventvocab.DatastoreAuditTypes()

// allVerbs is eventTypes' key set and allPhases the two deploy transitions —
// the unfiltered query's push-down sets, computed once (they are constants).
var (
	allVerbs     = slices.Sorted(maps.Keys(eventTypes))
	allPhases    = []string{store.EventPhaseStarted, store.EventPhaseEnded}
	allFactTypes = []string{
		TypeImagePullFailed,
		TypeServiceSuspended,
		TypeServiceResumed,
		TypeServiceHibernated,
		TypeServiceWoken,
		TypeServerFailed,
		TypeServerAvailable,
		TypeBranchChanged,
		TypeBranchDeleted,
		TypeCommitIgnored,
		TypeAutoscalingStarted,
		TypeAutoscalingEnded,
		TypeBuildStarted,
		TypeBuildEnded,
		TypePreDeployStarted,
		TypePreDeployEnded,
		TypeJobRunEnded,
	}
)

// pushDown translates a caller's event-TYPE filter into the sets the store
// query takes: the audit verbs that produce that type, the deploy phases that
// do, and (for auto-deploy types) an AutoDeployFilter that discriminates the
// three sub-types in SQL before the LIMIT. It is what keeps the type filter IN
// SQL — filtering the result in Go after the store's LIMIT would return short
// (sometimes empty) pages, which a cursor client reads as the end of the feed
// and stops on.
//
// An empty type asks for everything. An unknown type matches nothing (empty
// sets), which is an empty feed — not, as a Go-side filter would give, a page
// of zero items that a client cannot distinguish from the end.
func pushDown(eventType string) (verbs, phases, factTypes []string, autoDeploy store.AutoDeployFilter) {
	switch eventType {
	case "":
		return allVerbs, allPhases, allFactTypes, store.AutoDeployFilterNone
	case TypeDeployStarted:
		return nil, []string{store.EventPhaseStarted}, nil, store.AutoDeployFilterNone
	case TypeDeployEnded:
		return nil, []string{store.EventPhaseEnded}, nil, store.AutoDeployFilterNone
	case TypeAutoDeployEnabled:
		return []string{core.AuditVerbSetAutoDeploy}, nil, nil, store.AutoDeployFilterEnabled
	case TypeAutoDeployDisabled:
		return []string{core.AuditVerbSetAutoDeploy}, nil, nil, store.AutoDeployFilterDisabled
	case TypeAutoDeployChanged:
		return []string{core.AuditVerbSetAutoDeploy}, nil, nil, store.AutoDeployFilterChanged
	case TypeImagePullFailed, TypeServiceSuspended, TypeServiceResumed,
		TypeServiceHibernated, TypeServiceWoken,
		TypeServerFailed, TypeServerAvailable, TypeBranchChanged, TypeBranchDeleted,
		TypeCommitIgnored, TypeAutoscalingStarted, TypeAutoscalingEnded,
		TypeBuildStarted, TypeBuildEnded, TypePreDeployStarted, TypePreDeployEnded,
		TypeJobRunEnded:
		return nil, nil, []string{eventType}, store.AutoDeployFilterNone
	}
	for _, verb := range allVerbs {
		if eventTypes[verb] == eventType {
			verbs = append(verbs, verb)
		}
	}
	return verbs, nil, nil, store.AutoDeployFilterNone
}

// DefaultWindow is how far back an events query reaches when the caller names no
// startTime — Render's documented default for this endpoint (now-1h), matched so
// a Render client's unparameterized call means the same thing against bex. Ask
// for more with startTime.
const DefaultWindow = time.Hour

// Trigger is Render's deploy trigger object (deploy_started.details.trigger) —
// which of the mutually-exclusive causes started this rollout. bex fills the two
// its deploys table records ("create" ⇒ FirstBuild, "api" ⇒ Manual) and leaves
// the rest false rather than guessing.
type Trigger struct {
	FirstBuild       bool
	EnvUpdated       bool
	Manual           bool
	DeployedByRender bool
	ClearCache       bool
	Rollback         bool
}

// Details is the per-type payload, a closed struct rather than a free-form map:
// nothing can be attached to an event that this type does not already name, which
// is the same structural reason a value cannot reach one. Fields are populated
// only for the types that define them (see the package doc); everything else
// serializes as an empty object, matching Render's payload-less types.
type Details struct {
	// deploy_started / deploy_ended
	DeployID     string
	DeployStatus string // "succeeded" | "failed" — Render's deployStatus enum
	// PreDeployStatus is the deploy's pre-deploy step outcome (w1/m33): "running"
	// | "succeeded" | "failed"; empty when no pre-deploy step ran. deploy_ended
	// only. Distinguishes a migration failure from a health-check failure.
	PreDeployStatus string
	// Status is a lifecycle-step fact's terminal outcome (w7/m66): build_ended /
	// pre_deploy_ended / job_run_ended carry succeeded|failed|canceled; empty for
	// the started/observed kinds and every other type.
	Status  string
	Trigger *Trigger // deploy_started only
	// Deploy details enriched for dashboard (w1/m47): deployed image, commit info, timing
	Image         string     // container image URI; empty for non-deploy events
	CommitID      string     // git revision; empty for non-deploy events
	CommitMessage string     // commit description; empty for non-deploy events
	StartedAt     *time.Time // when deploy started executing; nil for non-deploy or not-yet-started
	FinishedAt    *time.Time // when deploy reached terminal status; nil for non-deploy or ongoing
	// suspender_added / suspender_removed
	Actor string
	// server_restarted
	TriggeredByUser string
	// plan_changed: Render's required from/to plan name strings
	PlanFrom *string
	PlanTo   *string
	// instance_count_changed: Render's required from/to instance counts
	InstanceCountFrom *int32
	InstanceCountTo   *int32
	// autoscaling_config_changed: before and after min/max; nil = disabled
	AutoscalingMinFrom *int32
	AutoscalingMaxFrom *int32
	AutoscalingMinTo   *int32
	AutoscalingMaxTo   *int32
	// service_moved (w6/m134): the before/after public prj-/env- ids; nil = no
	// placement on that side, so assign, move, and unassign share one shape.
	ProjectFrom     *string
	ProjectTo       *string
	EnvironmentFrom *string
	EnvironmentTo   *string
	// Datastore configuration changes (w3/m82): the value the field was set TO.
	// postgres_ha_status_changed carries HighAvailabilityEnabled,
	// postgres_connection_pool_enabled_changed ConnectionPoolEnabled,
	// postgres_disk_size_changed DiskSizeGB, and key_value_config_restart the
	// resulting MaxmemoryPolicy/PersistenceMode pair.
	HighAvailabilityEnabled *bool
	ConnectionPoolEnabled   *bool
	DiskSizeGB              *int32
	MaxmemoryPolicy         *string
	PersistenceMode         *string
	// Durable fact details. ReasonCode is a closed public code, never a raw
	// Kubernetes or Git message.
	ReasonCode string
	InstanceID string
	FromCount  *int32
	ToCount    *int32
	BranchFrom string
	BranchTo   string
	CommitURL  string
}

// Event is the neutral projection every adapter renders. Cursor is the opaque
// position a client echoes back to resume — it encodes the feed's keyset
// (timestamp + row key), NOT the event id: the id is a hash of the source row,
// and a hash cannot be resumed from. It also outlives its row, so a cursor still
// pages correctly after the audit retention sweep deletes the event it named.
type Event struct {
	ID        string
	Type      string
	At        time.Time
	ServiceID string
	Details   Details
	Cursor    string
}

// Filter narrows List — the neutral shape the REST/GraphQL/MCP adapters translate
// Render's type/startTime/endTime/cursor/limit query params into.
type Filter struct {
	Type   string // one event type; empty ⇒ all
	Since  time.Time
	Until  time.Time
	Cursor string
	Limit  int
}

// FilterOf builds a Filter from the five params Render's endpoint takes, in the
// string form every adapter has them in (a query value, a GraphQL argument, an
// MCP tool field). One translator for all three, so a REST call and a tool call
// with the same params cannot page differently. An unparseable timestamp is left
// zero — the Service then applies Render's default window — rather than 400.
//
// That is this fragment ALONE, not the house style: every other caller-supplied
// time filter answers a named 400, and core.ParseTime's own doc states the rule
// — a silently dropped bound widens the effective window past caps like
// BEX_MAX_QUERY_HOURS. What justifies the permissiveness HERE is the default
// window below: a dropped bound still yields a defined, bounded result. A list
// with no default window has nothing to land on and must be strict.
func FilterOf(eventType, startTime, endTime, cursor string, limit int) Filter {
	f := Filter{Type: eventType, Cursor: cursor, Limit: pageLimit(limit)}
	if t, err := time.Parse(time.RFC3339, startTime); err == nil {
		f.Since = t
	}
	if t, err := time.Parse(time.RFC3339, endTime); err == nil {
		f.Until = t
	}
	return f
}

// pageLimit clamps a caller's limit to Render's bounds — the same clamp
// core.PageParams applies to a query string, for the two adapters that don't
// have one.
func pageLimit(limit int) int { return core.PageLimitOrAbsent(limit) }

// Service is the events feature. Store nil (BEX_CP_DB_URI unset) ⇒ every read
// reports core.ErrEventsUnavailable: the feed's sources are control-plane
// tables, so there is nothing to degrade to.
type Service struct {
	*core.Base
	Store EventStore
	// MaxQueryHours, when positive, caps the startTime–endTime window a caller may
	// ask for (BEX_MAX_QUERY_HOURS) — the same bound logs and metrics enforce.
	// Enforced HERE, not in the REST fragment, so all three surfaces reject the
	// same window (logs/metrics check it adapter-side and their GraphQL surfaces
	// are correspondingly unbounded — a wart this feature does not copy).
	MaxQueryHours int
}

const (
	EventIDInvalidCode = "EVENT_ID_INVALID"
	EventNotFoundCode  = "EVENT_NOT_FOUND"
)

// List returns a service's activity feed, newest first (Render's
// GET /services/{id}/events). A hand-applied App has no control-plane row, hence
// no deploys and no audit target: an empty feed, not an error — the deploy-history
// precedent.
func (s *Service) List(ctx context.Context, service string, filter Filter) ([]Event, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, service)
	if err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrEventsUnavailable
	}
	since, until := filter.Since, filter.Until
	if since.IsZero() {
		since = s.Now().Add(-DefaultWindow)
	}
	if err := s.checkWindow(since, until); err != nil {
		return nil, err
	}
	after, err := core.DecodeKeysetCursor(filter.Cursor)
	if err != nil {
		return nil, err
	}
	appID := a.Labels[store.LabelAppID]
	if appID == "" {
		return []Event{}, nil
	}
	legacyTarget := ""
	if publicName := a.Labels[core.LabelServiceName]; publicName != "" && publicName != a.Name {
		legacyTarget = core.ServiceTarget(publicName)
	}
	verbs, phases, factTypes, autoDeploy := pushDown(filter.Type)
	rows, err := s.Store.ListServiceEvents(ctx, appID, core.ServiceTarget(a.Name), a.Labels[core.LabelTenant], store.ServiceEventFilter{
		Since:        since,
		Until:        until,
		AfterAt:      after.At,
		AfterKey:     after.Key,
		Verbs:        verbs,
		Phases:       phases,
		FactTypes:    factTypes,
		AutoDeploy:   autoDeploy,
		LegacyTarget: legacyTarget,
		Limit:        filter.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, view(r, service))
	}
	return out, nil
}

// Get returns one globally-addressed event by its stable evt-… id. Unlike List,
// the route has no service identifier from which to discover the owner, so the
// lookup is scoped to the caller's effective workspace before the source row is
// read. A miss in another workspace is therefore indistinguishable from an id
// that never existed.
func (s *Service) Get(ctx context.Context, eventID string) (Event, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Event{}, err
	}
	kind, ok := ids.KindOf(eventID)
	if !ok || kind != ids.Event {
		return Event{}, core.NewBadRequestError(EventIDInvalidCode, "event id must be a valid evt-… identifier", nil)
	}
	if s.Store == nil {
		return Event{}, core.ErrEventsUnavailable
	}
	lookup, err := s.Store.GetServiceEvent(ctx, s.WorkspaceOrDefault(ctx), eventID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Event{}, eventNotFound(eventID)
		}
		return Event{}, err
	}
	event := view(lookup.Event, lookup.ServiceID)
	// The trigger indexes every safe source row, while this package owns the
	// closed public vocabulary. An indexed-but-unmapped audit effect stays hidden
	// exactly as it does in List and webhook dispatch.
	if event.Type == "" || event.ID != eventID {
		return Event{}, eventNotFound(eventID)
	}
	return event, nil
}

func eventNotFound(eventID string) error {
	return core.NewNotFoundError(EventNotFoundCode, "event not found", map[string]any{"id": eventID})
}

// checkWindow rejects a range wider than MaxQueryHours (BEX_MAX_QUERY_HOURS) —
// the same guard logs and metrics apply, mapped to core.ErrBadRequest so all
// three surfaces answer 400 identically.
func (s *Service) checkWindow(since, until time.Time) error {
	if s.MaxQueryHours <= 0 {
		return nil
	}
	if until.IsZero() {
		until = s.Now()
	}
	if until.Sub(since) > time.Duration(s.MaxQueryHours)*time.Hour {
		return fmt.Errorf("%w: query range exceeds %d hours", core.ErrBadRequest, s.MaxQueryHours)
	}
	return nil
}

// view projects one composed store row onto the Render event vocabulary. The id
// is DERIVED from the source row (ids.Derive), never minted: a client pages,
// re-fetches and dedupes on it, so the same source row must yield the same evt-…
// id on every read, forever.
func view(r store.ServiceEventRow, service string) Event {
	ev := Event{
		ID:        ids.Derive(ids.Event, r.Key),
		At:        r.At,
		ServiceID: service,
		Cursor:    core.EncodeKeysetCursor(r.At, r.Key),
	}
	switch r.Source {
	case store.EventSourceDeploy:
		ev.Details.DeployID = r.DeployID
		// Enrich with deploy details for dashboard display (w1/m47)
		ev.Details.Image = r.Image
		ev.Details.CommitID = r.CommitID
		ev.Details.CommitMessage = r.CommitMessage
		ev.Details.StartedAt = r.StartedAt
		ev.Details.FinishedAt = r.FinishedAt
		if r.Phase == store.EventPhaseStarted {
			ev.Type = TypeDeployStarted
			ev.Details.Trigger = &Trigger{
				FirstBuild: r.Trigger == store.TriggerCreate,
				// Render's envUpdated is "a configuration write caused this
				// rollout" — the nearest flag in its vocabulary for the deploys
				// a Settings field, env var, secret file, or env-group link now
				// opens (w6/m51). Manual stays false: nobody clicked Deploy.
				EnvUpdated: r.Trigger == store.TriggerConfigChange,
				Manual:     r.Trigger == store.TriggerAPI || r.Trigger == store.TriggerDeployHook,
				Rollback:   r.Trigger == store.TriggerRollback,
			}
		} else {
			ev.Type = TypeDeployEnded
			ev.Details.DeployStatus = store.RenderDeployStatus(r.Status)
			ev.Details.PreDeployStatus = r.PreDeployStatus
		}
	case store.EventSourceAudit:
		ev.Type = eventTypes[r.Verb]
		if ev.Type == "" {
			ev.Type = indexedAuditEventTypes[r.Verb]
		}
		switch ev.Type {
		case TypeSuspenderAdded, TypeSuspenderRemoved:
			ev.Details.Actor = r.Caller
		case TypeServerRestarted:
			ev.Details.TriggeredByUser = r.Caller
		case TypePlanChanged:
			ev.Details.PlanFrom = r.PlanFrom
			ev.Details.PlanTo = r.PlanTo
		case TypeInstanceCountChanged:
			ev.Details.InstanceCountFrom = r.InstanceCountFrom
			ev.Details.InstanceCountTo = r.InstanceCountTo
		case TypeAutoscalingConfigChanged:
			ev.Details.AutoscalingMinFrom = r.AutoscalingMinFrom
			ev.Details.AutoscalingMaxFrom = r.AutoscalingMaxFrom
			ev.Details.AutoscalingMinTo = r.AutoscalingMinTo
			ev.Details.AutoscalingMaxTo = r.AutoscalingMaxTo
		case TypeServiceMoved:
			ev.Details.ProjectFrom = r.ProjectFrom
			ev.Details.ProjectTo = r.ProjectTo
			ev.Details.EnvironmentFrom = r.EnvironmentFrom
			ev.Details.EnvironmentTo = r.EnvironmentTo
		case TypePostgresHAStatusChanged:
			ev.Details.HighAvailabilityEnabled = r.HighAvailabilityEnabled
		case TypePostgresConnectionPoolEnabledChanged:
			ev.Details.ConnectionPoolEnabled = r.ConnectionPoolEnabled
		case TypePostgresDiskSizeChanged:
			ev.Details.DiskSizeGB = r.DiskSizeGB
		case TypeKeyValueConfigRestart:
			ev.Details.MaxmemoryPolicy = r.MaxmemoryPolicy
			ev.Details.PersistenceMode = r.PersistenceMode
		case TypeAutoDeployChanged:
			// Discriminate: new rows carry the boolean; legacy rows keep the bex name.
			if r.AutoDeployEnabled != nil {
				if *r.AutoDeployEnabled {
					ev.Type = TypeAutoDeployEnabled
				} else {
					ev.Type = TypeAutoDeployDisabled
				}
			}
		}
	case store.EventSourceFact:
		ev.Type = r.FactType
		ev.Details.DeployID = r.DeployID
		ev.Details.Image = r.Image
		ev.Details.CommitID = r.CommitID
		ev.Details.ReasonCode = r.ReasonCode
		ev.Details.InstanceID = r.InstanceID
		ev.Details.FromCount = r.FromCount
		ev.Details.ToCount = r.ToCount
		ev.Details.BranchFrom = r.BranchFrom
		ev.Details.BranchTo = r.BranchTo
		ev.Details.CommitURL = r.CommitURL
		ev.Details.Status = r.FactStatus
	}
	return ev
}
