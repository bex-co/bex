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

package deploys

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the GraphQL fragment: a read query, deploys(serviceId), for
// the dashboard's Deploys/Events tab (w5) to nest deploy history under a
// service, plus w2/m10's cancelDeploy/rollbackService mutations and the
// triggerDeploy mutation (w2/006). Named deploys/serviceId (not id) since it
// isn't a single-resource fetch-by-id like server(id) — it's a service-scoped
// list, matching the milestone's own naming.

var deployGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Deploy",
	Fields: graphql.Fields{
		"id":         gqlutil.StrField(func(d DeployView) any { return d.ID }),
		"serviceId":  gqlutil.StrField(func(d DeployView) any { return d.ServiceID }),
		"status":     gqlutil.StrField(func(d DeployView) any { return d.Status }),
		"trigger":    gqlutil.StrField(func(d DeployView) any { return d.Trigger }),
		"image":      gqlutil.StrField(func(d DeployView) any { return d.Image }),
		"rollbackOf": gqlutil.StrField(func(d DeployView) any { return d.RollbackOf }),
		// The resolved commit this deploy ran (w9/001 + w2/m42) — flat fields,
		// the fragment's existing style (REST nests them as Render's commit
		// object). Empty/nil when unresolved; clients omit rather than fake.
		"commitId":        gqlutil.StrField(func(d DeployView) any { return d.CommitID }),
		"commitMessage":   gqlutil.StrField(func(d DeployView) any { return d.CommitMessage }),
		"commitCreatedAt": gqlutil.StrField(func(d DeployView) any { return formatTimePtr(d.CommitAuthorAt) }),
		"createdAt":       gqlutil.StrField(func(d DeployView) any { return formatTime(d.CreatedAt) }),
		"updatedAt":       gqlutil.StrField(func(d DeployView) any { return formatTime(d.UpdatedAt) }),
		"startedAt":       gqlutil.StrField(func(d DeployView) any { return formatTimePtr(d.StartedAt) }),
		"finishedAt":      gqlutil.StrField(func(d DeployView) any { return formatTimePtr(d.FinishedAt) }),
		// Pre-deploy step outcome (w1/m33): "running"|"succeeded"|"failed", empty
		// when no pre-deploy step ran. Distinguishes a migration failure from a
		// health-check failure (both status=update_failed).
		"preDeployStatus": gqlutil.StrField(func(d DeployView) any { return d.PreDeployStatus }),
		// Actionable cause of a failed deploy (w9/011): the operator's diagnosis
		// (crash loop with the $PORT hint, image-pull failure, build error) or a
		// health-gate-timeout line. Empty unless the deploy failed.
		"failureReason": gqlutil.StrField(func(d DeployView) any { return d.FailureReason }),
	},
})

var deployHookGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DeployHook",
	Fields: graphql.Fields{
		"url": gqlutil.StrField(func(h DeployHookView) any { return h.URL }),
	},
})

var deployHookArgs = graphql.FieldConfigArgument{
	"serviceId": gqlutil.ReqArg(graphql.String),
}

// GraphQLQuery returns the deploys(serviceId, …) field for the composition
// root to merge into the root Query. The filter arguments mirror the REST
// query params 1:1 — status, created/updated/finished time bounds, cursor, limit —
// through the same FilterOf translator, so the two surfaces cannot page
// differently; all absent means the full history, the pre-m31 contract.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		// deployHook is the sensitive settings read. It lazily mints the service's
		// stable secret URL and returns the same {url} shape as REST/MCP.
		"deployHook": &graphql.Field{
			Type: deployHookGQLType,
			Args: deployHookArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetDeployHook(p.Context, p.Args["serviceId"].(string))
			},
		},
		"deploys": &graphql.Field{
			Type: graphql.NewList(deployGQLType),
			Args: graphql.FieldConfigArgument{
				"serviceId":      gqlutil.ReqArg(graphql.String),
				"status":         gqlutil.Arg(graphql.NewList(graphql.String)),
				"createdBefore":  gqlutil.Arg(graphql.String),
				"createdAfter":   gqlutil.Arg(graphql.String),
				"updatedBefore":  gqlutil.Arg(graphql.String),
				"updatedAfter":   gqlutil.Arg(graphql.String),
				"finishedBefore": gqlutil.Arg(graphql.String),
				"finishedAfter":  gqlutil.Arg(graphql.String),
				"cursor":         gqlutil.Arg(graphql.String),
				"limit":          gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				filter, err := filterFromArgs(p.Args)
				if err != nil {
					return nil, err
				}
				return s.List(p.Context, p.Args["serviceId"].(string), filter)
			},
		},
		// deployActions projects the deploy verbs' per-service decisions
		// (ADR087, w6/m136): bare deploy / cancel / rollback, each with the
		// permission tri-state and the blocking precondition computed by the
		// same predicates the verbs enforce (RollbackEligible, the suspended +
		// billing gates). A bex extension consumed by mobile's action gating.
		"deployActions": gqlutil.KeyVerb(gqlutil.ActionDecisionsOut, "serviceId", s.ActionCapabilities),
		// deploy is the single-resource fetch-by-id twin of deploys(serviceId, …)
		// (w9/m1/t001), closing GraphQL's drift from REST's GET
		// .../deploys/{deployId} and MCP's get_deploy — the dashboard's deploy
		// detail page needs a by-id read, not just the list. Resolves through the
		// same Service.Get REST/MCP use, so a deployId belonging to another
		// service or an unknown deployId errors identically (core.ErrNotFound)
		// across all three surfaces.
		"deploy": &graphql.Field{
			Type: deployGQLType,
			Args: deployMutationArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, p.Args["serviceId"].(string), p.Args["deployId"].(string))
			},
		},
	}
}

// filterFromArgs is the GraphQL twin of rest.go's filterFromQuery: it pulls
// the same filter params out of the resolver's argument map and hands them to
// the one shared translator (FilterOf) — the events.filterFromArgs precedent.
func filterFromArgs(args map[string]any) (ListFilter, error) {
	str := func(key string) string {
		v, _ := args[key].(string)
		return v
	}
	var statuses []string
	if raw, _ := args["status"].([]any); len(raw) > 0 {
		statuses = make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok {
				statuses = append(statuses, s)
			}
		}
	}
	limit, err := gqlutil.PositiveLimit(args)
	if err != nil {
		return ListFilter{}, err
	}
	return FilterOf(
		statuses,
		str("createdBefore"), str("createdAfter"),
		str("updatedBefore"), str("updatedAfter"),
		str("finishedBefore"), str("finishedAfter"),
		str("cursor"), limit,
	)
}

// deployMutationArgs is the (serviceId, deployId) argument shape cancelDeploy and
// rollbackService share.
var deployMutationArgs = graphql.FieldConfigArgument{
	"serviceId": gqlutil.ReqArg(graphql.String),
	"deployId":  gqlutil.ReqArg(graphql.String),
}

// GraphQLMutation returns triggerDeploy (w2/006), restartServer (consolidated
// from apps — w2/m30), plus cancelDeploy/rollbackService (w2/m10).
// triggerDeploy mirrors Render's dashboard Manual Deploy action — delegates to
// the same Trigger verb as REST POST .../deploys so the surfaces cannot drift.
// restartServer replaces apps.GraphQL's restartServer: routing through
// deploys.Restart ensures every restart opens a deploy-history row.
// cancelDeploy/rollbackService follow the suspendService/resumeService convention.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"regenerateDeployHook": &graphql.Field{
			Type: deployHookGQLType,
			Args: deployHookArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RegenerateDeployHook(p.Context, p.Args["serviceId"].(string))
			},
		},
		"triggerDeploy": &graphql.Field{
			Type: deployGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId":  gqlutil.ReqArg(graphql.String),
				"commitId":   gqlutil.Arg(graphql.String),
				"deployMode": gqlutil.Arg(graphql.String),
				"imageUrl":   gqlutil.Arg(graphql.String),
				// Render's "clear" | "do_not_clear" enum. bex builds are always
				// cache-free, so this is a validated no-op (see TriggerParams) —
				// exposed for parity with the dashboard's Manual Deploy menu.
				"clearCache": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				commitID := gqlutil.Str(p.Args, "commitId")
				deployMode := gqlutil.Str(p.Args, "deployMode")
				imageURL := gqlutil.Str(p.Args, "imageUrl")
				clearCache := gqlutil.Str(p.Args, "clearCache")
				return s.Trigger(p.Context, p.Args["serviceId"].(string), TriggerParams{
					CommitID:   commitID,
					DeployMode: deployMode,
					ImageURL:   imageURL,
					ClearCache: clearCache,
				})
			},
		},
		// restartServer is kept for API callers that already send this mutation
		// name; the dashboard uses triggerDeploy directly. Returns Deploy (not
		// Service) — every restart opens a deploy-history row (w2/m30).
		"restartServer": &graphql.Field{
			Type: deployGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Trigger(p.Context, p.Args["serviceId"].(string), TriggerParams{})
			},
		},
		"cancelDeploy": &graphql.Field{
			Type: deployGQLType,
			Args: deployMutationArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Cancel(p.Context, p.Args["serviceId"].(string), p.Args["deployId"].(string))
			},
		},
		"rollbackService": &graphql.Field{
			Type: deployGQLType,
			Args: deployMutationArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Rollback(p.Context, p.Args["serviceId"].(string), p.Args["deployId"].(string))
			},
		},
	}
}
