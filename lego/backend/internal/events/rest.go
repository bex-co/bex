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

package events

import (
	"net/http"
	"net/url"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// rest.go is the events REST fragment: Render's GET /services/{serviceId}/events,
// verified field-by-field against its OpenAPI (render-public-api-1.json,
// operationId list-events) rather than the rendered docs.
//
// Honored, exactly as Render spells them:
//
//	query  type       one event type to filter to
//	query  startTime  RFC3339; DEFAULT now-1h (Render's own default — an
//	                  unparameterized call means the same thing on both)
//	query  endTime    RFC3339; default now
//	query  cursor     opaque; resume after the last item of the previous page
//	query  limit      default 20, clamped to [1,100] (core.PageParams)
//	200               a BARE ARRAY of {event, cursor} — no outer object, no
//	                  hasMore/nextCursor (Render's list envelope for this endpoint)
//	event             id (evt-…), timestamp, serviceId, type, details — all five
//	                  present on every item, as Render marks them required
//
// Diverging, deliberately:
//
//   - Render's `status` (deprecated integer) is not emitted on deploy_ended,
//     build_ended or pre_deploy_ended; deployStatus / buildStatus /
//     preDeployStatus, its replacements, are.
//   - Required fields bex does not record are omitted, never invented: an
//     actor's email, build and pre-deploy execution ids, a deleted branch's
//     successor, autoscaling criteria, and the disk, maintenance-mode and
//     auto-deploy facts of a row recorded before w3/m317 kept them (w3/m302).
//   - Rejecting a malformed cursor with 400 (Render's behavior here is unspecified).
//
// Mounted under /v1/services, like every other apps-adjacent
// route.

// renderEvent is Render's event object.
type renderEvent struct {
	ID        string        `json:"id"`
	Timestamp string        `json:"timestamp"`
	ServiceID string        `json:"serviceId"`
	Type      string        `json:"type"`
	Details   renderDetails `json:"details"`
}

// renderDetails is the per-type payload. Every field is omitempty, so each type
// serializes exactly the members Render defines for it and a payload-less type
// (service_resumed, maintenance_ended, and bex's config-change types) serializes
// as `{}` — Render's own shape for those.
//
// Each Render-documented type uses the keys of the details member its type
// names (w3/m302); the rest are bex extras a Render client ignores.
//
// Deploy detail enrichment (w1/m47): image, commit info, timing — populated for
// deploy events to enable rich dashboard display without additional queries.
type renderDetails struct {
	DeployID        string `json:"deployId,omitempty"`
	DeployStatus    string `json:"deployStatus,omitempty"`
	PreDeployStatus string `json:"preDeployStatus,omitempty"` // bex extra (w1/m33): the deploy's pre-deploy step outcome
	// FullDeployStatus is the deploy object's own status, present only when the
	// deploy failed (w1/m138): Render's deployStatus collapses every failure
	// kind to "failed". bex extra.
	FullDeployStatus string `json:"fullDeployStatus,omitempty"`
	// FailureReason is the human-actionable cause of a failed deploy, present
	// only on failures (w1/m138). bex extra — Render's deploy_ended has none.
	FailureReason string `json:"failureReason,omitempty"`
	// CancelReason is the neutral cause of a non-user cancel (w4/089). bex extra.
	CancelReason string `json:"cancelReason,omitempty"`
	// StallReason is why an OPEN deploy is not progressing (w4/m112). bex extra.
	StallReason string `json:"stallReason,omitempty"`
	// Status is a run's terminal outcome on job_run_ended and
	// cron_job_run_ended. build_ended and pre_deploy_ended carry theirs as
	// buildStatus / preDeployStatus, Render's keys; Render's integer status
	// there is deprecated and not emitted (w3/m302).
	Status string `json:"status,omitempty"`
	// BuildStatus is build_ended's succeeded|failed|canceled (w7/m66).
	BuildStatus string `json:"buildStatus,omitempty"`
	// BuildID is pipeline_minutes_exhausted's buildId (w9/m345): the refused
	// deploy, bex's build being its deploy's own.
	BuildID string `json:"buildId,omitempty"`
	// Trigger is a deploy's *renderTrigger (deploy_started,
	// pipeline_minutes_exhausted) or an edge-cache event's
	// *renderEdgeCacheTrigger: Render's two trigger objects share the key.
	Trigger any `json:"trigger,omitempty"`
	// Deploy enrichment for dashboard (w1/m47)
	Image         string `json:"image,omitempty"`
	CommitID      string `json:"commitId,omitempty"`
	CommitMessage string `json:"commitMessage,omitempty"`
	StartedAt     string `json:"startedAt,omitempty"`  // RFC3339; nil → omitted
	FinishedAt    string `json:"finishedAt,omitempty"` // RFC3339; nil → omitted
	Actor         string `json:"actor,omitempty"`
	// SuspendedByUser / ResumedByUser are Render's {id, email} objects on
	// the suspender pair (w4/391).
	SuspendedByUser *renderUser `json:"suspendedByUser,omitempty"`
	ResumedByUser   *renderUser `json:"resumedByUser,omitempty"`
	TriggeredByUser string      `json:"triggeredByUser,omitempty"`
	ReasonCode      string      `json:"reasonCode,omitempty"`
	InstanceID      string      `json:"instanceID,omitempty"`
	// FromInstances / ToInstances are the instance counts of
	// instance_count_changed and the autoscaling pair.
	FromInstances *int32 `json:"fromInstances,omitempty"`
	ToInstances   *int32 `json:"toInstances,omitempty"`
	// plan_changed ("Instance Type Changed" on Render) and bex's
	// branch_changed and display_name_changed carry a from/to string pair.
	From *string `json:"from,omitempty"`
	To   *string `json:"to,omitempty"`
	// ID / URL are commit_ignored's commit (Render's "Commit Ignored").
	ID  string `json:"id,omitempty"`
	URL string `json:"url,omitempty"`
	// ImageURL / Message are image_pull_failed's image and its cause.
	ImageURL string `json:"imageURL,omitempty"`
	Message  string `json:"message,omitempty"`
	// DeletedBranch is branch_deleted's branch.
	DeletedBranch string `json:"deletedBranch,omitempty"`
	// JobID is job_run_ended's job.
	JobID string `json:"jobId,omitempty"`
	// service_moved (w6/m134): the before/after public prj-/evm- ids. A side
	// with no placement is omitted, so assign shows only *To, unassign only
	// *From, and a move both.
	ProjectFrom     *string `json:"projectFrom,omitempty"`
	ProjectTo       *string `json:"projectTo,omitempty"`
	EnvironmentFrom *string `json:"environmentFrom,omitempty"`
	EnvironmentTo   *string `json:"environmentTo,omitempty"`
	// FromConfig / ToConfig are autoscaling_config_changed's policies.
	FromConfig *autoscalingState `json:"fromConfig,omitempty"`
	ToConfig   *autoscalingState `json:"toConfig,omitempty"`
	// cron_job_run_ended: Render's run id, and its reason object on a run that
	// failed for an observed cause (w4/m114). deploy_ended and build_ended
	// always carry Render's buildDeployEndReason here (w8/m146).
	CronJobRunID string `json:"cronJobRunId,omitempty"`
	Reason       any    `json:"reason,omitempty"`
	// postgres_cluster_leader_changed: Render's leaderId, the new primary.
	LeaderID string `json:"leaderId,omitempty"`
	// service_disk_usage_high / _recovered (w2/m255): Render's disk details.
	DiskID           string   `json:"diskId,omitempty"`
	DiskName         string   `json:"diskName,omitempty"`
	MountPath        string   `json:"mountPath,omitempty"`
	UsagePercent     *float64 `json:"usagePercent,omitempty"`
	ThresholdPercent *float64 `json:"thresholdPercent,omitempty"`
	// env_group_linked / _unlinked (w4/563): the group, by id and by its name
	// at the time (bex extension; these types are bex's own).
	EnvGroupID   string `json:"envGroupId,omitempty"`
	EnvGroupName string `json:"envGroupName,omitempty"`
	// custom_domain_added / _removed / _verified (w4/m337): the domain's
	// hostname (bex extension; these types are bex's own).
	Domain string `json:"domain,omitempty"`
	// disk_created's size, disk_updated's before and after (w3/m317).
	SizeGB     *int32 `json:"sizeGB,omitempty"`
	FromSizeGB *int32 `json:"fromSizeGB,omitempty"`
	ToSizeGB   *int32 `json:"toSizeGB,omitempty"`
	// maintenance_mode_enabled's toggle and maintenance_mode_uri_updated's
	// pages; an empty URI is Render's default page, so it is sent, not omitted.
	Enabled *bool   `json:"enabled,omitempty"`
	FromURI *string `json:"fromURI,omitempty"`
	ToURI   *string `json:"toURI,omitempty"`
	// auto_deploy_disabled's trigger before it (its reason rides on Reason) and
	// auto_deploy_enabled's trigger after it.
	FromTrigger *string `json:"fromTrigger,omitempty"`
	NewTrigger  *string `json:"newTrigger,omitempty"`
}

// unhealthyMessage is server_failed's reason when an instance failed its
// health check; the fact records which check, not its output.
const unhealthyMessage = "The instance failed its health check"

// toFailureReason maps a failure's closed reason code onto Render's
// failureReason object, the one Render's cron run, job and server_failed
// events share; nil when the cause was not observed. timed_out is a cron
// run's deadline, the only failure that records it. early_exit is a running
// service's process that ended on its own with status 0 (w8/m158).
func toFailureReason(d Details) *FailureReason {
	switch d.ReasonCode {
	case store.EventReasonNonZeroExit:
		return &FailureReason{NonZeroExit: d.ExitCode}
	case store.EventReasonOOMKilled:
		return &FailureReason{OOMKilled: &OOMKilledInfo{MemoryLimit: d.MemoryLimit}}
	case store.EventReasonEarlyExit:
		early := true
		return &FailureReason{EarlyExit: &early}
	case store.EventReasonEvicted:
		return &FailureReason{Evicted: true}
	case store.EventReasonTimedOut:
		timeout := appv1alpha1.CronRunActiveDeadlineSeconds
		return &FailureReason{TimedOutSeconds: &timeout}
	case store.EventReasonReadinessFailed:
		return &FailureReason{Unhealthy: unhealthyMessage}
	}
	return nil
}

// imagePullFailedMessage is image_pull_failed's message; the event records no
// registry text, so the message is the one cause the fact can carry.
const imagePullFailedMessage = "The image could not be pulled"

// autoscalingState is one side of autoscaling_config_changed: Render's
// autoscaling config, the body of its PUT …/autoscaling (w4/m335). A disable
// keeps the config turned off, as Render's does (w4/597), so a disable's
// toConfig is its fromConfig turned off and an enable's fromConfig is the kept
// config; a side with no config reads as bex's GET …/autoscaling does without
// autoscaling: zero bounds and no criterion enabled.
// Criteria is nil only on an enabled side recorded before the targets were, as
// an enabled config always has one.
type autoscalingState struct {
	Enabled  bool                 `json:"enabled"`
	Min      int32                `json:"min"`
	Max      int32                `json:"max"`
	Criteria *autoscalingCriteria `json:"criteria,omitempty"`
}

type autoscalingCriteria struct {
	CPU    autoscalingCriterion `json:"cpu"`
	Memory autoscalingCriterion `json:"memory"`
}

type autoscalingCriterion struct {
	Enabled    bool  `json:"enabled"`
	Percentage int32 `json:"percentage"`
}

// autoscalingSide is one side's config; enabled is false for settings a
// disable kept, and nil reads as running wherever there are settings.
func autoscalingSide(enabled *bool, minimum, maximum, cpu, memory *int32) *autoscalingState {
	if minimum == nil || maximum == nil {
		return &autoscalingState{Criteria: &autoscalingCriteria{}}
	}
	side := &autoscalingState{Enabled: enabled == nil || *enabled, Min: *minimum, Max: *maximum}
	if cpu != nil || memory != nil {
		side.Criteria = &autoscalingCriteria{CPU: autoscalingTarget(cpu), Memory: autoscalingTarget(memory)}
	}
	return side
}

func autoscalingTarget(percent *int32) autoscalingCriterion {
	if percent == nil {
		return autoscalingCriterion{}
	}
	return autoscalingCriterion{Enabled: true, Percentage: *percent}
}

// renderTrigger is deploy_started's trigger object — Render's six booleans
// always present, as Render marks them required, plus bex's deployHook.
// renderUser is Render's {id, email} user object (w4/391).
type renderUser struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
}

func toRenderUser(u *UserRef) *renderUser {
	if u == nil {
		return nil
	}
	return &renderUser{ID: u.ID, Email: u.Email}
}

type renderTrigger struct {
	FirstBuild       bool `json:"firstBuild"`
	EnvUpdated       bool `json:"envUpdated"`
	Manual           bool `json:"manual"`
	DeployedByRender bool `json:"deployedByRender"`
	ClearCache       bool `json:"clearCache"`
	Rollback         bool `json:"rollback"`
	// DeployHook is additive — a Render client ignores the unknown key, and
	// every Render-named flag keeps its Render meaning. See Trigger.DeployHook.
	DeployHook bool `json:"deployHook"`
	// User is the human who triggered the deploy (w4/391).
	User *renderUser `json:"user,omitempty"`
}

// renderEdgeCacheTrigger is Render's trigger on edge_cache_enabled,
// edge_cache_disabled and edge_cache_purged (w9/m345): manual and system are
// required, user names the person who changed the cache.
type renderEdgeCacheTrigger struct {
	Manual bool        `json:"manual"`
	User   *renderUser `json:"user,omitempty"`
	System bool        `json:"system"`
}

// renderCronRunStatus maps the stored lifecycle word onto Render's cronJobRun
// status enum (pending|successful|unsuccessful|canceled), the vocabulary the
// same run carries on GET /v1/cron-jobs/{id}/runs (w4/507). job_run_ended's
// status and the build/pre-deploy statuses keep succeeded|failed|canceled,
// which Render documents for them.
func renderCronRunStatus(status string) string {
	switch status {
	case store.EventStatusSucceeded:
		return "successful"
	case store.EventStatusFailed:
		return "unsuccessful"
	}
	return status
}

func toRenderEvent(e Event) renderEvent {
	d := renderDetails{
		DeployID:         e.Details.DeployID,
		DeployStatus:     e.Details.DeployStatus,
		PreDeployStatus:  e.Details.PreDeployStatus,
		FullDeployStatus: e.Details.FullDeployStatus,
		FailureReason:    e.Details.FailureReason,
		CancelReason:     e.Details.CancelReason,
		StallReason:      e.Details.StallReason,
		Image:            e.Details.Image,
		CommitID:         e.Details.CommitID,
		CommitMessage:    e.Details.CommitMessage,
		StartedAt:        formatTime(e.Details.StartedAt),
		FinishedAt:       formatTime(e.Details.FinishedAt),
		Actor:            e.Details.Actor,
		SuspendedByUser:  toRenderUser(e.Details.SuspendedByUser),
		ResumedByUser:    toRenderUser(e.Details.ResumedByUser),
		TriggeredByUser:  e.Details.TriggeredByUser,
		ReasonCode:       e.Details.ReasonCode,
		InstanceID:       e.Details.InstanceID,
	}
	if t := e.Details.Trigger; t != nil {
		d.Trigger = &renderTrigger{
			FirstBuild:       t.FirstBuild,
			EnvUpdated:       t.EnvUpdated,
			Manual:           t.Manual,
			DeployedByRender: t.DeployedByRender,
			ClearCache:       t.ClearCache,
			Rollback:         t.Rollback,
			DeployHook:       t.DeployHook,
			User:             toRenderUser(t.User),
		}
	}
	if t := e.Details.EdgeCacheTrigger; t != nil {
		// Every bex change is a caller's write: manual, never system.
		d.Trigger = &renderEdgeCacheTrigger{Manual: true, User: toRenderUser(t.User)}
	}
	switch e.Type {
	case TypePlanChanged:
		d.From = e.Details.PlanFrom
		d.To = e.Details.PlanTo
	case TypeInstanceCountChanged:
		d.FromInstances = e.Details.InstanceCountFrom
		d.ToInstances = e.Details.InstanceCountTo
	case TypeAutoscalingConfigChanged:
		d.FromConfig = autoscalingSide(e.Details.AutoscalingEnabledFrom, e.Details.AutoscalingMinFrom, e.Details.AutoscalingMaxFrom, e.Details.AutoscalingCPUFrom, e.Details.AutoscalingMemoryFrom)
		d.ToConfig = autoscalingSide(nil, e.Details.AutoscalingMinTo, e.Details.AutoscalingMaxTo, e.Details.AutoscalingCPUTo, e.Details.AutoscalingMemoryTo)
		if !d.ToConfig.Enabled && d.FromConfig.Enabled {
			kept := *d.FromConfig
			kept.Enabled = false
			d.ToConfig = &kept
		}
	case TypeBranchChanged:
		d.From = &e.Details.BranchFrom
		d.To = &e.Details.BranchTo
	case TypeBranchDeleted:
		d.DeletedBranch = e.Details.BranchFrom
	case TypeCommitIgnored:
		d.ID, d.URL = e.Details.CommitID, e.Details.CommitURL
		d.CommitID = ""
	case TypeServerFailed:
		if reason := toFailureReason(e.Details); reason != nil {
			d.Reason = reason
		}
		d.ReasonCode = ""
	case TypeImagePullFailed:
		d.ImageURL, d.Message = e.Details.Image, imagePullFailedMessage
		d.Image, d.ReasonCode = "", ""
	case TypeJobRunEnded:
		d.Status, d.JobID = e.Details.Status, e.Details.JobID
	case TypeServiceMoved:
		d.ProjectFrom = e.Details.ProjectFrom
		d.ProjectTo = e.Details.ProjectTo
		d.EnvironmentFrom = e.Details.EnvironmentFrom
		d.EnvironmentTo = e.Details.EnvironmentTo
	case TypeAutoscalingStarted, TypeAutoscalingEnded:
		d.FromInstances = e.Details.FromCount
		d.ToInstances = e.Details.ToCount
	case TypeCronJobRunStarted:
		d.CronJobRunID = e.Details.CronJobRunID
	case TypeCronJobRunEnded:
		d.CronJobRunID = e.Details.CronJobRunID
		d.Status = renderCronRunStatus(e.Details.Status)
		if reason := toFailureReason(e.Details); reason != nil {
			d.Reason = reason
		}
	case TypeBuildEnded:
		d.BuildStatus = e.Details.Status
		if e.Details.EndReason != nil {
			d.Reason = e.Details.EndReason
		}
	case TypePreDeployEnded:
		d.PreDeployStatus = e.Details.Status
		if e.Details.EndReason != nil {
			d.Reason = e.Details.EndReason
		}
	case TypeDeployEnded:
		if e.Details.EndReason != nil {
			d.Reason = e.Details.EndReason
		}
	case TypePipelineMinutesExhausted:
		d.BuildID = e.Details.DeployID
	case TypePostgresClusterLeaderChanged:
		d.LeaderID = e.Details.LeaderID
	case TypeDiskSnapshotRestored, TypeDiskDeleted:
		d.DiskID = e.Details.DiskID
	case TypeDiskCreated:
		d.DiskID, d.SizeGB = e.Details.DiskID, e.Details.DiskSizeGB
	case TypeDiskUpdated:
		d.DiskID, d.FromSizeGB, d.ToSizeGB = e.Details.DiskID, e.Details.DiskSizeFromGB, e.Details.DiskSizeGB
	case TypeMaintenanceModeEnabled:
		d.Enabled = e.Details.MaintenanceEnabled
	case TypeMaintenanceModeURIUpdated:
		d.FromURI, d.ToURI = e.Details.MaintenanceURIFrom, e.Details.MaintenanceURITo
	case TypeAutoDeployEnabled:
		d.NewTrigger = e.Details.AutoDeployTrigger
	case TypeAutoDeployDisabled:
		d.FromTrigger = e.Details.AutoDeployTrigger
		if reason := e.Details.AutoDeployReason; reason != nil {
			d.Reason = *reason
		}
	case TypeServiceDiskUsageHigh, TypeServiceDiskUsageRecovered:
		d.DiskID, d.DiskName, d.MountPath = e.Details.DiskID, e.Details.DiskName, e.Details.MountPath
		d.UsagePercent, d.ThresholdPercent = e.Details.UsagePercent, e.Details.ThresholdPercent
	case TypeEnvGroupLinked, TypeEnvGroupUnlinked:
		d.EnvGroupID, d.EnvGroupName = e.Details.EnvGroupID, e.Details.EnvGroupName
	case TypeDisplayNameChanged:
		if e.Details.DisplayNameTo != "" {
			d.From, d.To = &e.Details.DisplayNameFrom, &e.Details.DisplayNameTo
		}
	case TypeCustomDomainAdded, TypeCustomDomainRemoved, TypeCustomDomainVerified:
		d.Domain = e.Details.Domain
	}
	return renderEvent{
		ID:        e.ID,
		Timestamp: e.At.UTC().Format(time.RFC3339),
		ServiceID: e.ServiceID,
		Type:      e.Type,
		Details:   d,
	}
}

// eventWithCursor is Render's list-item envelope for this endpoint: the cursor is
// a SIBLING of the event, not a member of it.
type eventWithCursor struct {
	Event  renderEvent `json:"event"`
	Cursor string      `json:"cursor"`
}

func toEventList(events []Event) []eventWithCursor {
	out := make([]eventWithCursor, 0, len(events))
	for _, e := range events {
		out = append(out, eventWithCursor{Event: toRenderEvent(e), Cursor: e.Cursor})
	}
	return out
}

// filterFromQuery translates Render's type/startTime/endTime/cursor/limit query
// params into Filter, over the one shared translator (FilterOf) the GraphQL and
// MCP fragments also use. `type` may name several types comma-separated
// (Render's wire); the Render request gate has already checked each against the
// pinned enum (renderCommaListQueryCompatibility in internal/api).
func filterFromQuery(q url.Values) Filter {
	cursor, limit := core.PageParams(q)
	return FilterOf([]string{q.Get("type")}, q.Get("startTime"), q.Get("endTime"), cursor, limit)
}

// RegisterREST mounts Render's service-scoped list and global retrieve routes.
// Store unwired ⇒ the Service returns core.ErrEventsUnavailable ⇒ 503 on these
// routes only.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	const base = "/v1/services"
	mux.HandleFunc("GET "+base+"/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		events, err := s.List(r.Context(), r.PathValue("id"), filterFromQuery(r.URL.Query()))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toEventList(events))
	})
	mux.HandleFunc("GET /v1/events/{eventId}", core.HandleMapped(http.StatusOK, func(r *http.Request) (Event, error) {
		// Render's route has no owner selector because its dashboard resolves the
		// active workspace out of band. Bex accepts the same optional ownerId used
		// by its other caller-scoped REST reads so a dashboard source-event link in
		// a non-default workspace cannot silently fall back to the caller's oldest
		// membership. Authorization still validates membership in Service.Get.
		ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
		return s.Get(ctx, r.PathValue("eventId"))
	}, toRenderEvent))
}
