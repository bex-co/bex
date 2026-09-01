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

package workspaces

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the GraphQL fragment the dashboard consumes: the `workspaces`
// query (the switcher's list) and create/rename/delete mutations. It is
// deliberately GraphQL-only — Render exposes no REST workspace-mutation API
// (verified in .pm/w6/RESEARCH-workspaces.md: /v1/owners is read-only), so the
// REST owners read endpoints + MCP tools are w6/m2, not a REST mutation surface
// here. Every resolver delegates to the Service; the schema is presentation.

// workspaceGQLType renders WorkspaceView. Fields follow Render's owner/workspace
// vocabulary (id, name, plan) plus the caller's role; the REST owner shape in
// w6/m2 maps the same view.
var workspaceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Workspace",
	Fields: graphql.Fields{
		"id":        gqlutil.StrField(func(w WorkspaceView) any { return w.ID }),
		"name":      gqlutil.StrField(func(w WorkspaceView) any { return w.Name }),
		"plan":      gqlutil.StrField(func(w WorkspaceView) any { return w.Plan }),
		"role":      gqlutil.StrField(func(w WorkspaceView) any { return w.Role }),
		"createdAt": gqlutil.StrField(func(w WorkspaceView) any { return w.CreatedAt }),
	},
})

var workspaceCreationPolicyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WorkspaceCreationPolicy",
	Fields: graphql.Fields{
		"mode":              gqlutil.StrField(func(v WorkspaceCreationPolicy) any { return v.Mode }),
		"paymentRequired":   gqlutil.BoolField(func(v WorkspaceCreationPolicy) any { return v.PaymentRequired }),
		"providerAvailable": gqlutil.BoolField(func(v WorkspaceCreationPolicy) any { return v.ProviderAvailable }),
	},
})

var workspaceCreationAttemptGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WorkspaceCreationAttempt",
	Fields: graphql.Fields{
		"id":              gqlutil.StrField(func(v WorkspaceCreationAttemptView) any { return v.ID }),
		"name":            gqlutil.StrField(func(v WorkspaceCreationAttemptView) any { return v.Name }),
		"plan":            gqlutil.StrField(func(v WorkspaceCreationAttemptView) any { return v.Plan }),
		"billingEmail":    gqlutil.StrField(func(v WorkspaceCreationAttemptView) any { return v.BillingEmail }),
		"paymentRequired": gqlutil.BoolField(func(v WorkspaceCreationAttemptView) any { return v.PaymentRequired }),
		"state":           gqlutil.StrField(func(v WorkspaceCreationAttemptView) any { return v.State }),
		"clientSecret":    gqlutil.StrField(func(v WorkspaceCreationAttemptView) any { return v.ClientSecret }),
		"publishableKey":  gqlutil.StrField(func(v WorkspaceCreationAttemptView) any { return v.PublishableKey }),
	},
})

// resourceCapGQLType is the used/limit pair for one resource kind (w7/m9), plus
// terminating: how many of `used` are finishing deletion (w6/m129) — the count
// that reconciles this figure with the resource list, which drops those rows but
// whose quota they still hold.
var resourceCapGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ResourceCap",
	Fields: graphql.Fields{
		"used":        gqlutil.IntField(func(c ResourceCapView) any { return c.Used }),
		"terminating": gqlutil.IntField(func(c ResourceCapView) any { return c.Terminating }),
		"limit":       gqlutil.IntField(func(c ResourceCapView) any { return c.Limit }),
	},
})

// resourceLimitsGQLType is the per-workspace cap report (w7/m9).
var resourceLimitsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ResourceLimits",
	Fields: graphql.Fields{
		"services":  gqlutil.Typed(resourceCapGQLType, func(r ResourceLimitsView) any { return r.Services }),
		"postgres":  gqlutil.Typed(resourceCapGQLType, func(r ResourceLimitsView) any { return r.Postgres }),
		"keyValues": gqlutil.Typed(resourceCapGQLType, func(r ResourceLimitsView) any { return r.KeyValues }),
	},
})

// GraphQLQuery contributes the caller's workspace list and limits query to the
// root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"workspaces": &graphql.Field{
			Type:    graphql.NewList(workspaceGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.List(p.Context) },
		},
		"workspaceCreationPolicy": &graphql.Field{
			Type: workspaceCreationPolicyGQLType,
			Args: graphql.FieldConfigArgument{"plan": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.WorkspaceCreationPolicy(p.Context, p.Args["plan"].(string))
			},
		},
		"workspaceCreationAttempt": &graphql.Field{
			Type: workspaceCreationAttemptGQLType,
			Args: graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ResumeWorkspaceCreation(p.Context, p.Args["id"].(string))
			},
		},
		// workspaceLimits returns the named workspace's resource usage vs. cap
		// (w7/m9): "3/5 services" visibility surface. Authorizes can_view on the
		// workspace (the same membership source as the workspaces list).
		"workspaceLimits": &graphql.Field{
			Type: resourceLimitsGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ResourceLimits(p.Context, p.Args["ownerId"].(string))
			},
		},
	}
}

// GraphQLMutation contributes the lifecycle mutations to the root Mutation.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"prepareWorkspaceCreation": &graphql.Field{
			Type: workspaceCreationAttemptGQLType,
			Args: graphql.FieldConfigArgument{
				"name":                 gqlutil.ReqArg(graphql.String),
				"plan":                 gqlutil.ReqArg(graphql.String),
				"billingEmail":         gqlutil.ReqArg(graphql.String),
				"attemptId":            gqlutil.Arg(graphql.String),
				"collectPaymentMethod": gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				collect, _ := p.Args["collectPaymentMethod"].(bool)
				return s.PrepareWorkspaceCreation(p.Context, p.Args["name"].(string), p.Args["plan"].(string), p.Args["billingEmail"].(string), gqlutil.Str(p.Args, "attemptId"), collect)
			},
		},
		"finalizeWorkspaceCreation": &graphql.Field{
			Type: workspaceGQLType,
			Args: graphql.FieldConfigArgument{"attemptId": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.FinalizeWorkspaceCreation(p.Context, p.Args["attemptId"].(string))
			},
		},
		"cancelWorkspaceCreation": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{"attemptId": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.CancelWorkspaceCreation(p.Context, p.Args["attemptId"].(string)); err != nil {
					return false, err
				}
				return true, nil
			},
		},
		"createWorkspace": &graphql.Field{
			Type: workspaceGQLType,
			Args: graphql.FieldConfigArgument{
				"name": gqlutil.ReqArg(graphql.String),
				"plan": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				plan := gqlutil.Str(p.Args, "plan")
				return s.Create(p.Context, p.Args["name"].(string), plan)
			},
		},
		"renameWorkspace": gqlutil.ArgMutation(workspaceGQLType, "name", s.Rename),
		"changeWorkspacePlan": &graphql.Field{
			// w6/m12: upgrade/downgrade a workspace's plan
			// (docs/render-artifacts/workspace-plan-change.md). GraphQL-only —
			// Render's REST owners surface has no plan-mutation endpoint and its
			// MCP has no workspace mutations, so REST/MCP deliberately gain
			// nothing here (parity by absence).
			Type: workspaceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   gqlutil.ReqArg(graphql.String),
				"plan": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ChangePlan(p.Context, p.Args["id"].(string), p.Args["plan"].(string))
			},
		},
		"deleteWorkspace": &graphql.Field{
			// Returns the deleted workspace's id on success (Render dashboard
			// delete mutations echo the affected id); a boolean would lose it.
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"id": gqlutil.ReqArg(graphql.String),
				"confirmation": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
					// Render's live delete guard: "sudo delete workspace <name>"
					// (docs/render-artifacts/workspace-lifecycle.md).
					Description: `must equal "sudo delete workspace <name>"`,
				},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				id := p.Args["id"].(string)
				if err := s.Delete(p.Context, id, p.Args["confirmation"].(string)); err != nil {
					return nil, err
				}
				return id, nil
			},
		},
	}
}
