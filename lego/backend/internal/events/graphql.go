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
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the GraphQL fragment: one read query, serviceEvents(serviceId, …),
// for the dashboard's Events tab (w5/007) — the same service-scoped-list naming
// deploys(serviceId) established. It exposes the same objects the REST fragment
// renders (same ids, same types, same details, same cursors), because both go
// through the one Service.List; TestEventSurfaceParity holds them to it.
//
// The arguments mirror Render's REST query params 1:1 (type/startTime/endTime/
// cursor/limit), including the now-1h default window — a dashboard tab that wants
// yesterday passes startTime, exactly as a REST client must — and including
// `type`'s comma-separated multi-type form ("deploy_ended,server_failed").

// userRefGQLType is Render's {id, email} user object (w4/391).
var userRefGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EventUser",
	Fields: graphql.Fields{
		"id":    gqlutil.StrField(func(u UserRef) any { return u.ID }),
		"email": gqlutil.StrField(func(u UserRef) any { return u.Email }),
	},
})

var triggerGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DeployTrigger",
	Fields: graphql.Fields{
		"firstBuild":       gqlutil.BoolField(func(t Trigger) any { return t.FirstBuild }),
		"envUpdated":       gqlutil.BoolField(func(t Trigger) any { return t.EnvUpdated }),
		"manual":           gqlutil.BoolField(func(t Trigger) any { return t.Manual }),
		"deployedByRender": gqlutil.BoolField(func(t Trigger) any { return t.DeployedByRender }),
		"clearCache":       gqlutil.BoolField(func(t Trigger) any { return t.ClearCache }),
		"rollback":         gqlutil.BoolField(func(t Trigger) any { return t.Rollback }),
		// bex extension — see Trigger.DeployHook (w4/104).
		"deployHook": gqlutil.BoolField(func(t Trigger) any { return t.DeployHook }),
		"user": &graphql.Field{Type: userRefGQLType, Resolve: gqlutil.Field(func(t Trigger) any {
			if t.User == nil {
				return nil
			}
			return *t.User
		})},
	},
})

// edgeCacheTriggerGQLType is Render's trigger on the edge-cache events
// (w9/m345), distinct from DeployTrigger as on Render.
var edgeCacheTriggerGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EdgeCacheTrigger",
	Fields: graphql.Fields{
		// A caller's cache write is manual; a plan change's toggle is the
		// system's (w9/088).
		"manual": gqlutil.BoolField(func(t EdgeCacheTrigger) any { return !t.System }),
		"system": gqlutil.BoolField(func(t EdgeCacheTrigger) any { return t.System }),
		"user": &graphql.Field{Type: userRefGQLType, Resolve: gqlutil.Field(func(t EdgeCacheTrigger) any {
			if t.User == nil {
				return nil
			}
			return *t.User
		})},
	},
})

// endReasonIDGQLType, failureReasonGQLType and endReasonGQLType are Render's
// buildDeployEndReason on deploy_ended / build_ended (w8/m146).
var endReasonIDGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BuildDeployEndReasonId",
	Fields: graphql.Fields{
		"id": gqlutil.StrField(func(r EndReasonID) any { return r.ID }),
	},
})

var oomKilledGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "OomKilled",
	Fields: graphql.Fields{
		"memoryLimit": gqlutil.StrField(func(o OOMKilledInfo) any { return o.MemoryLimit }),
	},
})

var failureReasonGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "FailureReason",
	Fields: graphql.Fields{
		"earlyExit":       gqlutil.BoolField(func(f FailureReason) any { return f.EarlyExit }),
		"evicted":         gqlutil.BoolField(func(f FailureReason) any { return f.Evicted }),
		"nonZeroExit":     gqlutil.IntField(func(f FailureReason) any { return f.NonZeroExit }),
		"timedOutSeconds": gqlutil.IntField(func(f FailureReason) any { return f.TimedOutSeconds }),
		"oomKilled": &graphql.Field{Type: oomKilledGQLType, Resolve: gqlutil.Field(func(f FailureReason) any {
			if f.OOMKilled == nil {
				return nil
			}
			return *f.OOMKilled
		})},
	},
})

func endReasonIDField(get func(EndReason) *EndReasonID) *graphql.Field {
	return &graphql.Field{Type: endReasonIDGQLType, Resolve: gqlutil.Field(func(r EndReason) any {
		if id := get(r); id != nil {
			return *id
		}
		return nil
	})}
}

var endReasonGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BuildDeployEndReason",
	Fields: graphql.Fields{
		"buildFailed": endReasonIDField(func(r EndReason) *EndReasonID { return r.BuildFailed }),
		"newBuild":    endReasonIDField(func(r EndReason) *EndReasonID { return r.NewBuild }),
		"newDeploy":   endReasonIDField(func(r EndReason) *EndReasonID { return r.NewDeploy }),
		"failure": &graphql.Field{Type: failureReasonGQLType, Resolve: gqlutil.Field(func(r EndReason) any {
			if r.Failure == nil {
				return nil
			}
			return *r.Failure
		})},
	},
})

// eventDetailsGQLType is the per-type payload, flattened: a field is null for a
// type that does not define it (GraphQL has no ergonomic union-per-type here, and
// a client selects only the fields its type needs anyway).
var eventDetailsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceEventDetails",
	Fields: graphql.Fields{
		"deployId":     gqlutil.StrField(func(d Details) any { return d.DeployID }),
		"deployStatus": gqlutil.StrField(func(d Details) any { return d.DeployStatus }),
		// preDeployStatus is the deploy's pre-deploy step outcome (w1/m33) — the
		// dashboard's Events tab shows it to tell a migration failure apart from a
		// health-check failure. Empty when no pre-deploy step ran.
		"preDeployStatus": gqlutil.StrField(func(d Details) any { return d.PreDeployStatus }),
		// fullDeployStatus is the deploy object's own status, present only when
		// the deploy failed (w1/m138) — Render's deployStatus collapses every
		// failure kind to "failed". bex extra.
		"fullDeployStatus": gqlutil.StrField(func(d Details) any { return d.FullDeployStatus }),
		// failureReason is the human-actionable cause of a failed deploy,
		// present only on failures (w1/m138). bex extra.
		"failureReason": gqlutil.StrField(func(d Details) any { return d.FailureReason }),
		// cancelReason is the neutral cause of a non-user cancel (w4/089) —
		// "Superseded by dep-…". Empty for user cancels. bex extra.
		"cancelReason": gqlutil.StrField(func(d Details) any { return d.CancelReason }),
		// w4/m112: why an OPEN deploy is not progressing; deploy_started only.
		"stallReason": gqlutil.StrField(func(d Details) any { return d.StallReason }),
		// The failure, stall and cancel sentences by code (w4/566, w4/591), as
		// on Deploy.
		"failureReasonTemplate": gqlutil.DeployReasonField(func(d Details) string { return d.FailureReason }),
		"stallReasonTemplate":   gqlutil.DeployReasonField(func(d Details) string { return d.StallReason }),
		"cancelReasonTemplate":  gqlutil.DeployReasonField(func(d Details) string { return d.CancelReason }),
		// status is a lifecycle-step event's terminal outcome (w7/m66):
		// build_ended / pre_deploy_ended / job_run_ended → succeeded|failed|canceled.
		"status": gqlutil.StrField(func(d Details) any { return d.Status }),
		// Deploy enrichment for dashboard Events view (w1/m47): image, commit info, timing
		"image":         gqlutil.StrField(func(d Details) any { return d.Image }),
		"commitId":      gqlutil.StrField(func(d Details) any { return d.CommitID }),
		"commitMessage": gqlutil.StrField(func(d Details) any { return d.CommitMessage }),
		"startedAt":     gqlutil.StrField(func(d Details) any { return formatTime(d.StartedAt) }),
		"finishedAt":    gqlutil.StrField(func(d Details) any { return formatTime(d.FinishedAt) }),
		"actor":         gqlutil.StrField(func(d Details) any { return d.Actor }),
		"suspendedByUser": &graphql.Field{Type: userRefGQLType, Resolve: gqlutil.Field(func(d Details) any {
			if d.SuspendedByUser == nil {
				return nil
			}
			return *d.SuspendedByUser
		})},
		"resumedByUser": &graphql.Field{Type: userRefGQLType, Resolve: gqlutil.Field(func(d Details) any {
			if d.ResumedByUser == nil {
				return nil
			}
			return *d.ResumedByUser
		})},
		// user is cron_job_run_ended's {id, email} (w4/631).
		"user": &graphql.Field{Type: userRefGQLType, Resolve: gqlutil.Field(func(d Details) any {
			if d.User == nil {
				return nil
			}
			return *d.User
		})},
		"triggeredByUser": &graphql.Field{
			Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.TriggeredByUser }),
		},
		"reasonCode": gqlutil.StrField(func(d Details) any { return d.ReasonCode }),
		"instanceId": gqlutil.StrField(func(d Details) any { return d.InstanceID }),
		"commitUrl":  gqlutil.StrField(func(d Details) any { return d.CommitURL }),
		"fromCount":  gqlutil.IntField(func(d Details) any { return d.FromCount }),
		"toCount":    gqlutil.IntField(func(d Details) any { return d.ToCount }),
		"branchFrom": gqlutil.StrField(func(d Details) any { return d.BranchFrom }),
		"branchTo":   gqlutil.StrField(func(d Details) any { return d.BranchTo }),
		// cron_job_run_ended (w4/m114): the run's crr- id, and the failed run's
		// container exit status beside reasonCode non_zero_exit / oom_killed.
		"cronJobRunId": gqlutil.StrField(func(d Details) any { return d.CronJobRunID }),
		"exitCode":     gqlutil.IntField(func(d Details) any { return d.ExitCode }),
		// server_failed with reasonCode oom_killed (w8/m158): the plan's memory
		// limit the instance was killed at, as a Kubernetes quantity ("512Mi").
		"memoryLimit": gqlutil.StrField(func(d Details) any { return d.MemoryLimit }),
		// deploy_ended / build_ended (w8/m146): Render's buildDeployEndReason.
		"reason": &graphql.Field{Type: endReasonGQLType, Resolve: gqlutil.Field(func(d Details) any {
			if d.EndReason == nil {
				return nil
			}
			return *d.EndReason
		})},
		// postgres_cluster_leader_changed (w2/m171): the new primary, Render's leaderId.
		"leaderId": gqlutil.StrField(func(d Details) any { return d.LeaderID }),
		// service_disk_usage_high / _recovered (w2/m255): Render's disk details.
		"diskId":           gqlutil.StrField(func(d Details) any { return d.DiskID }),
		"diskName":         gqlutil.StrField(func(d Details) any { return d.DiskName }),
		"mountPath":        gqlutil.StrField(func(d Details) any { return d.MountPath }),
		"usagePercent":     gqlutil.FloatField(func(d Details) any { return d.UsagePercent }),
		"thresholdPercent": gqlutil.FloatField(func(d Details) any { return d.ThresholdPercent }),
		// disk_created / _updated (w3/m317): the size, and a resize's before.
		"sizeGB":     gqlutil.IntField(func(d Details) any { return d.DiskSizeGB }),
		"fromSizeGB": gqlutil.IntField(func(d Details) any { return d.DiskSizeFromGB }),
		// maintenance_mode_enabled / _uri_updated (w3/m317).
		"maintenanceEnabled": gqlutil.BoolField(func(d Details) any { return d.MaintenanceEnabled }),
		"fromURI":            gqlutil.StrField(func(d Details) any { return d.MaintenanceURIFrom }),
		"toURI":              gqlutil.StrField(func(d Details) any { return d.MaintenanceURITo }),
		// auto_deploy_enabled / _disabled (w3/m317): Render's trigger after an
		// enable or before a disable, and why a disable happened.
		"autoDeployTrigger":        gqlutil.StrField(func(d Details) any { return d.AutoDeployTrigger }),
		"autoDeployDisabledReason": gqlutil.StrField(func(d Details) any { return d.AutoDeployReason }),
		"trigger": &graphql.Field{Type: triggerGQLType, Resolve: gqlutil.Field(func(d Details) any {
			if d.Trigger == nil {
				return nil
			}
			return *d.Trigger
		})},
		// edge_cache_enabled / _disabled / _purged (w9/m345): Render's
		// {manual, user, system} trigger.
		"edgeCacheTrigger": &graphql.Field{Type: edgeCacheTriggerGQLType, Resolve: gqlutil.Field(func(d Details) any {
			if d.EdgeCacheTrigger == nil {
				return nil
			}
			return *d.EdgeCacheTrigger
		})},
		// plan_changed from/to plan name strings.
		"planFrom": gqlutil.StrField(func(d Details) any { return d.PlanFrom }),
		"planTo":   gqlutil.StrField(func(d Details) any { return d.PlanTo }),
		// instance_count_changed from/to instance counts.
		"instanceCountFrom": gqlutil.IntField(func(d Details) any { return d.InstanceCountFrom }),
		"instanceCountTo":   gqlutil.IntField(func(d Details) any { return d.InstanceCountTo }),
		// autoscaling_config_changed before/after min+max.
		"autoscalingMinFrom": gqlutil.IntField(func(d Details) any { return d.AutoscalingMinFrom }),
		"autoscalingMaxFrom": gqlutil.IntField(func(d Details) any { return d.AutoscalingMaxFrom }),
		"autoscalingMinTo":   gqlutil.IntField(func(d Details) any { return d.AutoscalingMinTo }),
		"autoscalingMaxTo":   gqlutil.IntField(func(d Details) any { return d.AutoscalingMaxTo }),
		// and each side's CPU and memory target percentages (w4/m335); null =
		// no such target.
		"autoscalingCPUFrom":    gqlutil.IntField(func(d Details) any { return d.AutoscalingCPUFrom }),
		"autoscalingCPUTo":      gqlutil.IntField(func(d Details) any { return d.AutoscalingCPUTo }),
		"autoscalingMemoryFrom": gqlutil.IntField(func(d Details) any { return d.AutoscalingMemoryFrom }),
		"autoscalingMemoryTo":   gqlutil.IntField(func(d Details) any { return d.AutoscalingMemoryTo }),
		// false when the from side's settings were kept by a disable, not
		// running (w4/597); null on a side with none and on older rows.
		"autoscalingEnabledFrom": gqlutil.BoolField(func(d Details) any { return d.AutoscalingEnabledFrom }),
		// service_moved before/after placement (w6/m134): public prj-/evm- ids,
		// null = no placement on that side.
		"projectFrom":     gqlutil.StrField(func(d Details) any { return d.ProjectFrom }),
		"projectTo":       gqlutil.StrField(func(d Details) any { return d.ProjectTo }),
		"environmentFrom": gqlutil.StrField(func(d Details) any { return d.EnvironmentFrom }),
		"environmentTo":   gqlutil.StrField(func(d Details) any { return d.EnvironmentTo }),
		// env_group_linked / _unlinked (w4/563): the group, by id and by its
		// name at the time.
		"envGroupId":   gqlutil.StrField(func(d Details) any { return d.EnvGroupID }),
		"envGroupName": gqlutil.StrField(func(d Details) any { return d.EnvGroupName }),
		// display_name_changed's label before and after, and a custom_domain_*
		// event's hostname (w4/m337); empty on rows recorded before them.
		"displayNameFrom": gqlutil.StrField(func(d Details) any { return d.DisplayNameFrom }),
		"displayNameTo":   gqlutil.StrField(func(d Details) any { return d.DisplayNameTo }),
		"domain":          gqlutil.StrField(func(d Details) any { return d.Domain }),
	},
})

// formatTime formats a *time.Time to RFC3339 string, or empty if nil (w1/m47)
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

var eventGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceEvent",
	Fields: graphql.Fields{
		"id":        gqlutil.StrField(func(e Event) any { return e.ID }),
		"type":      gqlutil.StrField(func(e Event) any { return e.Type }),
		"serviceId": gqlutil.StrField(func(e Event) any { return e.ServiceID }),
		"timestamp": gqlutil.StrField(func(e Event) any { return e.At.UTC().Format(time.RFC3339) }),
		"cursor":    gqlutil.StrField(func(e Event) any { return e.Cursor }),
		"details":   gqlutil.Typed(eventDetailsGQLType, func(e Event) any { return e.Details }),
	},
})

// GraphQLQuery returns the list and global single-event fields for the
// composition root to merge into the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"serviceEvent": &graphql.Field{
			Type: eventGQLType,
			Args: graphql.FieldConfigArgument{
				"id": gqlutil.ReqArg(graphql.String),
				// Same optional selector REST's GET /v1/events/{eventId} already
				// binds: an evt-… id is global, but the lookup is workspace-scoped,
				// so a webhook subscriber hydrating an event that belongs to a
				// non-default workspace must be able to name it rather than fall
				// back to their oldest membership and read EVENT_NOT_FOUND (w4/128).
				// Membership is still validated in Service.Get's Authorize.
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				ctx := core.WithWorkspace(p.Context, gqlutil.Str(p.Args, "ownerId"))
				return s.Get(ctx, p.Args["id"].(string))
			},
		},
		"serviceEvents": &graphql.Field{
			Type:        graphql.NewList(eventGQLType),
			Description: "Service activity, newest first. Without startTime, returns the last hour; pass an explicit range for older history.",
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
				"type":      gqlutil.Arg(graphql.String),
				"startTime": gqlutil.Arg(graphql.String),
				"endTime":   gqlutil.Arg(graphql.String),
				"cursor":    gqlutil.Arg(graphql.String),
				"limit":     gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.List(p.Context, p.Args["serviceId"].(string), filterFromArgs(p.Args))
			},
		},
	}
}

// filterFromArgs is the GraphQL twin of filterFromQuery: it pulls the same five
// params out of the resolver's argument map and hands them to the one shared
// translator, so the two surfaces cannot page differently.
func filterFromArgs(args map[string]any) Filter {
	str := func(key string) string {
		v, _ := args[key].(string)
		return v
	}
	limit, _ := args["limit"].(int)
	return FilterOf([]string{str("type")}, str("startTime"), str("endTime"), str("cursor"), limit)
}
