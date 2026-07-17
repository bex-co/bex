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
		"firstBuild":       &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(t Trigger) any { return t.FirstBuild })},
		"envUpdated":       &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(t Trigger) any { return t.EnvUpdated })},
		"manual":           &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(t Trigger) any { return t.Manual })},
		"deployedByRender": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(t Trigger) any { return t.DeployedByRender })},
		"clearCache":       &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(t Trigger) any { return t.ClearCache })},
		"rollback":         &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(t Trigger) any { return t.Rollback })},
	},
})

// eventDetailsGQLType is the per-type payload, flattened: a field is null for a
// type that does not define it (GraphQL has no ergonomic union-per-type here, and
// a client selects only the fields its type needs anyway).
var eventDetailsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceEventDetails",
	Fields: graphql.Fields{
		"deployId":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.DeployID })},
		"deployStatus": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.DeployStatus })},
		// preDeployStatus is the deploy's pre-deploy step outcome (w1/m33) — the
		// dashboard's Events tab shows it to tell a migration failure apart from a
		// health-check failure. Empty when no pre-deploy step ran.
		"preDeployStatus": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.PreDeployStatus })},
		// Deploy enrichment for dashboard Events view (w1/m47): image, commit info, timing
		"image":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.Image })},
		"commitId":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.CommitID })},
		"commitMessage": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.CommitMessage })},
		"startedAt":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return formatTime(d.StartedAt) })},
		"finishedAt":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return formatTime(d.FinishedAt) })},
		"actor":           &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.Actor })},
		"triggeredByUser": &graphql.Field{
			Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.TriggeredByUser }),
		},
		"trigger": &graphql.Field{Type: triggerGQLType, Resolve: gqlutil.Field(func(d Details) any {
			if d.Trigger == nil {
				return nil
			}
			return *d.Trigger
		})},
		// plan_changed from/to plan name strings.
		"planFrom": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.PlanFrom })},
		"planTo":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d Details) any { return d.PlanTo })},
		// instance_count_changed from/to instance counts.
		"instanceCountFrom": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(d Details) any { return d.InstanceCountFrom })},
		"instanceCountTo":   &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(d Details) any { return d.InstanceCountTo })},
		// autoscaling_config_changed before/after min+max.
		"autoscalingMinFrom": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(d Details) any { return d.AutoscalingMinFrom })},
		"autoscalingMaxFrom": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(d Details) any { return d.AutoscalingMaxFrom })},
		"autoscalingMinTo":   &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(d Details) any { return d.AutoscalingMinTo })},
		"autoscalingMaxTo":   &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(d Details) any { return d.AutoscalingMaxTo })},
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
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.ID })},
		"type":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.Type })},
		"serviceId": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.ServiceID })},
		"timestamp": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.At.UTC().Format(time.RFC3339) })},
		"cursor":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.Cursor })},
		"details":   &graphql.Field{Type: eventDetailsGQLType, Resolve: gqlutil.Field(func(e Event) any { return e.Details })},
	},
})

// GraphQLQuery returns the serviceEvents(serviceId, …) field for the composition
// root to merge into the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"serviceEvents": &graphql.Field{
			Type: graphql.NewList(eventGQLType),
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"type":      &graphql.ArgumentConfig{Type: graphql.String},
				"startTime": &graphql.ArgumentConfig{Type: graphql.String},
				"endTime":   &graphql.ArgumentConfig{Type: graphql.String},
				"cursor":    &graphql.ArgumentConfig{Type: graphql.String},
				"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
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
