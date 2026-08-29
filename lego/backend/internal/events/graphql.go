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
// yesterday passes startTime, exactly as a REST client must.

var triggerGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DeployTrigger",
	Fields: graphql.Fields{
		"firstBuild":       gqlutil.BoolField(func(t Trigger) any { return t.FirstBuild }),
		"envUpdated":       gqlutil.BoolField(func(t Trigger) any { return t.EnvUpdated }),
		"manual":           gqlutil.BoolField(func(t Trigger) any { return t.Manual }),
		"deployedByRender": gqlutil.BoolField(func(t Trigger) any { return t.DeployedByRender }),
		"clearCache":       gqlutil.BoolField(func(t Trigger) any { return t.ClearCache }),
		"rollback":         gqlutil.BoolField(func(t Trigger) any { return t.Rollback }),
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
		"trigger": &graphql.Field{Type: triggerGQLType, Resolve: gqlutil.Field(func(d Details) any {
			if d.Trigger == nil {
				return nil
			}
			return *d.Trigger
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
		// service_moved before/after placement (w6/m134): public prj-/env- ids,
		// null = no placement on that side.
		"projectFrom":     gqlutil.StrField(func(d Details) any { return d.ProjectFrom }),
		"projectTo":       gqlutil.StrField(func(d Details) any { return d.ProjectTo }),
		"environmentFrom": gqlutil.StrField(func(d Details) any { return d.EnvironmentFrom }),
		"environmentTo":   gqlutil.StrField(func(d Details) any { return d.EnvironmentTo }),
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
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, p.Args["id"].(string))
			},
		},
		"serviceEvents": &graphql.Field{
			Type: graphql.NewList(eventGQLType),
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
	return FilterOf(str("type"), str("startTime"), str("endTime"), str("cursor"), limit)
}
