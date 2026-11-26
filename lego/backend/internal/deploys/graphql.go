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
	"maps"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
		"imageSha":   gqlutil.StrField(func(d DeployView) any { return d.ImageSHA }), // REST's image.sha, flat like commitId (w8/m122)
		"rollbackOf": gqlutil.StrField(func(d DeployView) any { return d.RollbackOf }),
		// buildAvailable is false when the registry no longer holds the build
		// this deploy ran on (w8/m156; Render: "You can only roll back to a
		// particular deploy if its build artifact is still available"). The
		// dashboard offers Rollback only where it is not false. A bex
		// extension: Render's deploy object has no such field.
		"buildAvailable": &graphql.Field{
			Type:    graphql.Boolean,
			Resolve: gqlutil.Field(func(d DeployView) any { return !d.BuildGone }),
		},
		// The resolved commit this deploy ran (w9/001 + w2/m42) — flat fields,
		// the fragment's existing style (REST nests them as Render's commit
		// object). Empty/nil when unresolved; clients omit rather than fake.
		"commitId":        gqlutil.StrField(func(d DeployView) any { return d.CommitID }),
		"buildId":         gqlutil.StrField(func(d DeployView) any { return d.BuildID }),
		"commitMessage":   gqlutil.StrField(func(d DeployView) any { return d.CommitMessage }),
		"commitCreatedAt": gqlutil.StrField(func(d DeployView) any { return formatTimePtr(d.CommitAuthorAt) }),
		"createdAt":       gqlutil.StrField(func(d DeployView) any { return formatTime(d.CreatedAt) }),
		"updatedAt":       gqlutil.StrField(func(d DeployView) any { return formatTime(d.UpdatedAt) }),
		"startedAt":       gqlutil.StrField(func(d DeployView) any { return formatTimePtr(d.StartedAt) }),
		"finishedAt":      gqlutil.StrField(func(d DeployView) any { return formatTimePtr(d.FinishedAt) }),
		// Pre-deploy step outcome (w1/m33): "running"|"succeeded"|"failed"|"canceled", empty
		// when no pre-deploy step ran. Distinguishes a migration failure from a
		// health-check failure (both status=update_failed).
		"preDeployStatus": gqlutil.StrField(func(d DeployView) any { return d.PreDeployStatus }),
		// The Blueprint's one-time initialDeployHook, run after this deploy
		// went live (w33/m435): "running"|"succeeded"|"failed", empty when it
		// did not run for this deploy. It never changes status. FinishedAt is
		// where the deploy's log ends once the hook has.
		"initialDeployHookStatus":     gqlutil.StrField(func(d DeployView) any { return d.InitialDeployHookStatus }),
		"initialDeployHookFinishedAt": gqlutil.TimeField(func(d DeployView) any { return d.InitialDeployHookFinishedAt }),
		// Actionable cause of a failed deploy (w9/011): the operator's diagnosis
		// (crash loop with the $PORT hint, image-pull failure, build error) or a
		// health-gate-timeout line. Empty unless the deploy failed.
		"failureReason": gqlutil.StrField(func(d DeployView) any { return d.FailureReason }),
		// Neutral cause of a non-user cancel (w4/089): "Superseded by dep-…".
		// Empty for user cancels. Distinct from failureReason so the dashboard
		// can render it without text-destructive treatment.
		"cancelReason": gqlutil.StrField(func(d DeployView) any { return d.CancelReason }),
		// w4/m112: why an OPEN deploy is not progressing. Empty while a
		// rollout is healthy; cleared when the deploy goes terminal.
		"stallReason": gqlutil.StrField(func(d DeployView) any { return d.StallReason }),
		// The failure, stall and cancel sentences by code, for a client that
		// words them in its own language (w4/566, w4/591); null for a sentence
		// with none.
		"failureReasonTemplate": gqlutil.DeployReasonField(func(d DeployView) string { return d.FailureReason }),
		"stallReasonTemplate":   gqlutil.DeployReasonField(func(d DeployView) string { return d.StallReason }),
		"cancelReasonTemplate":  gqlutil.DeployReasonField(func(d DeployView) string { return d.CancelReason }),
		// The trigger facts Render's deploy page Notices read (w1/m659),
		// Render's triggerDetails flattened as rollbackOf is: the deploy
		// cleared the build cache ("Build cache cleared"), moved the service
		// to the free plan ("Updated to a free compute plan"), and the
		// Blueprint and sync that opened it ("Synced from"), null for any
		// other deploy. REST and MCP keep Render's deploy object, which has
		// none of them.
		"clearCache":             gqlutil.ReqBoolField(func(d DeployView) any { return d.ClearCache }),
		"isTransitionToFreeTier": gqlutil.ReqBoolField(func(d DeployView) any { return d.FreePlanTransition }),
		"blueprintId":            gqlutil.OptionalStrField(func(d DeployView) any { return d.BlueprintID }),
		"blueprintSyncId":        gqlutil.OptionalStrField(func(d DeployView) any { return d.BlueprintSyncID }),
		// timedOutSeconds is the rollout budget a failed deploy ran out of,
		// null for any other: Render's deploy_ended reason.timedOutSeconds,
		// its deploy page's "Timed out" (w1/m659).
		"timedOutSeconds": gqlutil.IntField(func(d DeployView) any {
			if d.TimedOutSeconds == 0 {
				return nil
			}
			return d.TimedOutSeconds
		}),
		// reservedPort is the port bex reserves (18012, 18013, 19099) that
		// the deploy's instance serves HTTP on, as its in-flight or failure
		// reason names it; null for none. Render's deploy page warns about
		// it on a free plan, from openPorts' HTTP mark (w1/m659).
		"reservedPort": gqlutil.IntField(func(d DeployView) any {
			if d.ReservedPort == 0 {
				return nil
			}
			return d.ReservedPort
		}),
	},
})

// The rollbackComparison read's types (w1/m646): Render's snapshotFields,
// docs/render-artifacts/rollback-comparison.md. An empty command or path is
// null, as an absent one is, so a client compares them as Render's does.
var keysType = graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))

var envVarChangesGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvVarChanges",
	Fields: graphql.Fields{
		"createdVarKeys":  gqlutil.Typed(keysType, func(c EnvVarChanges) any { return c.CreatedVarKeys }),
		"deletedVarKeys":  gqlutil.Typed(keysType, func(c EnvVarChanges) any { return c.DeletedVarKeys }),
		"updatedVarKeys":  gqlutil.Typed(keysType, func(c EnvVarChanges) any { return c.UpdatedVarKeys }),
		"createdFileKeys": gqlutil.Typed(keysType, func(c EnvVarChanges) any { return c.CreatedFileKeys }),
		"deletedFileKeys": gqlutil.Typed(keysType, func(c EnvVarChanges) any { return c.DeletedFileKeys }),
		"updatedFileKeys": gqlutil.Typed(keysType, func(c EnvVarChanges) any { return c.UpdatedFileKeys }),
	},
})

var snapshotEnvGroupGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceConfigSnapshotEnvGroup",
	Fields: graphql.Fields{
		"id":   gqlutil.ReqStrField(func(g SnapshotEnvGroup) any { return g.ID }),
		"name": gqlutil.ReqStrField(func(g SnapshotEnvGroup) any { return g.Name }),
	},
})

var serviceConfigSnapshotGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceConfigSnapshot",
	Fields: graphql.Fields{
		"runCommand":       gqlutil.OptionalStrField(func(s ServiceConfigSnapshot) any { return s.RunCommand }),
		"preDeployCommand": gqlutil.OptionalStrField(func(s ServiceConfigSnapshot) any { return s.PreDeployCommand }),
		"dockerCommand":    gqlutil.OptionalStrField(func(s ServiceConfigSnapshot) any { return s.DockerCommand }),
		"healthCheckPath":  gqlutil.OptionalStrField(func(s ServiceConfigSnapshot) any { return s.HealthCheckPath }),
		"instances": gqlutil.IntField(func(s ServiceConfigSnapshot) any {
			if s.Instances == nil {
				return nil
			}
			return *s.Instances
		}),
		// plan is the compute plan the release ran on, in Render's spelling
		// ("starter", "pro_plus"); null when the record does not show it.
		"plan": gqlutil.OptionalStrField(func(s ServiceConfigSnapshot) any { return s.Plan }),
		"envGroups": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(snapshotEnvGroupGQLType))), func(s ServiceConfigSnapshot) any {
			if s.EnvGroups == nil {
				return []SnapshotEnvGroup{}
			}
			return s.EnvGroups
		}),
		"deletedEnvGroupCount": gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(s ServiceConfigSnapshot) any { return s.DeletedEnvGroupCount }),
		"envVarChanges":        gqlutil.Typed(graphql.NewNonNull(envVarChangesGQLType), func(s ServiceConfigSnapshot) any { return s.EnvVarChanges }),
	},
})

var comparedDeployGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "RollbackComparisonDeploy",
	Fields: graphql.Fields{
		"id": gqlutil.ReqStrField(func(d ComparedDeploy) any { return d.ID }),
		// null when the release keeps no configuration record.
		"serviceConfigSnapshot": gqlutil.Typed(serviceConfigSnapshotGQLType, func(d ComparedDeploy) any {
			if d.Snapshot == nil {
				return nil
			}
			return *d.Snapshot
		}),
	},
})

var rollbackComparisonGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "RollbackComparison",
	Fields: graphql.Fields{
		"currentLiveDeploy": gqlutil.Typed(comparedDeployGQLType, func(c RollbackComparison) any {
			if c.CurrentLiveDeploy == nil {
				return nil
			}
			return *c.CurrentLiveDeploy
		}),
		"deploy": gqlutil.Typed(graphql.NewNonNull(comparedDeployGQLType), func(c RollbackComparison) any { return c.Deploy }),
		// currentPlanLower stands in for Render's serverPlans: true when the
		// saved plan, which a rollback keeps, is below the one the target ran
		// on, so the page shows Render's plan warning.
		"currentPlanLower": gqlutil.ReqBoolField(func(c RollbackComparison) any { return c.CurrentPlanLower }),
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
	fields := graphql.Fields{
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
		"deployActions": &graphql.Field{
			Type: gqlutil.ActionDecisionsOut,
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
				"ownerId":   gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				ctx := core.WithWorkspace(p.Context, gqlutil.Str(p.Args, "ownerId"))
				return s.ActionCapabilities(ctx, p.Args["serviceId"].(string))
			},
		},
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
		// rollbackComparison is Render's read behind its Confirm Rollback page
		// (w1/m646): the live release's configuration beside the target's.
		"rollbackComparison": &graphql.Field{
			Type: rollbackComparisonGQLType,
			Args: deployMutationArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RollbackComparison(p.Context, p.Args["serviceId"].(string), p.Args["deployId"].(string))
			},
		},
	}
	maps.Copy(fields, s.buildGQLQueryFields())
	return fields
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
	fields := graphql.Fields{
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
				"buildId":    gqlutil.Arg(graphql.String),
				// Render's "clear" | "do_not_clear" enum. With registry caching
				// enabled, "clear" rebuilds without importing prior layers (w7/m88);
				// with the gate off both values are no-ops. See TriggerParams.
				"clearCache": gqlutil.Arg(graphql.String),
				// The protected-environment phrase for an imageUrl/commitId
				// deploy (w4/m176), copied from the first call's refusal.
				"confirm": gqlutil.Arg(graphql.String),
				// Render's dashboard "Deploy a specific commit" turns auto-deploy
				// off; its API does not (render.com/docs/deploys). The dashboard
				// passes true with commitId, so the pin and the switch-off are one
				// write (w4/m431); every other caller keeps auto-deploy.
				"disableAutoDeploy": &graphql.ArgumentConfig{Type: graphql.Boolean, DefaultValue: false},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				disable, _ := p.Args["disableAutoDeploy"].(bool)
				return s.TriggerManual(core.WithConfirm(p.Context, gqlutil.Str(p.Args, "confirm")), p.Args["serviceId"].(string), TriggerParams{
					CommitID:          gqlutil.Str(p.Args, "commitId"),
					DeployMode:        gqlutil.Str(p.Args, "deployMode"),
					ImageURL:          gqlutil.Str(p.Args, "imageUrl"),
					ClearCache:        gqlutil.Str(p.Args, "clearCache"),
					BuildID:           gqlutil.Str(p.Args, "buildId"),
					DisableAutoDeploy: disable,
				})
			},
		},
		// restartServer restarts on the running commit (Service.Restart, w1/m148),
		// the dashboard's header and row Restart included. Returns Deploy (not
		// Service): every restart opens a deploy-history row (w2/m30).
		"restartServer": &graphql.Field{
			Type: deployGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Restart(p.Context, p.Args["serviceId"].(string))
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
			Args: graphql.FieldConfigArgument{
				"serviceId": deployMutationArgs["serviceId"],
				"deployId":  deployMutationArgs["deployId"],
				// Render turns auto-deploy off for a rollback started in its dashboard and
				// leaves it alone for one started through its API (render.com/docs/
				// rollbacks). The dashboard passes true; every other caller gets the API
				// behaviour by default (w1/m152 t009).
				"disableAutoDeploy": &graphql.ArgumentConfig{Type: graphql.Boolean, DefaultValue: false},
				"confirm":           gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				disable, _ := p.Args["disableAutoDeploy"].(bool)
				return s.Rollback(core.WithConfirm(p.Context, gqlutil.Str(p.Args, "confirm")), p.Args["serviceId"].(string), p.Args["deployId"].(string),
					RollbackOptions{DisableAutoDeploy: disable})
			},
		},
	}
	maps.Copy(fields, s.buildGQLMutationFields())
	return fields
}
