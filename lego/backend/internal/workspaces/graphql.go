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
	"context"
	"maps"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the GraphQL fragment the dashboard consumes: the `workspaces`
// query (the switcher's list) and create/rename/delete mutations. It is
// deliberately GraphQL-only — Render exposes no REST workspace-mutation API
// (verified in .pm/w6/RESEARCH-workspaces.md: /v1/owners is read-only), so the
// REST owners read endpoints + MCP tools are w6/m2, not a REST mutation surface
// here. Every resolver delegates to the Service; the schema is presentation.

// workspaceType renders WorkspaceView. Fields follow Render's owner/workspace
// vocabulary (id, name — the freeform display name — plan) plus the caller's
// role and bex's slug (the unique typable key, w3/m246); the REST owner shape
// in w6/m2 maps the same view. Built once per Service, so the one schema the
// query and mutation fragments both contribute to holds one Workspace type.
func (s *Service) workspaceType() *graphql.Object {
	s.gqlWorkspaceOnce.Do(func() {
		s.gqlWorkspace = graphql.NewObject(graphql.ObjectConfig{
			Name: "Workspace",
			Fields: graphql.Fields{
				"id":        gqlutil.StrField(func(w WorkspaceView) any { return w.ID }),
				"name":      gqlutil.StrField(func(w WorkspaceView) any { return w.Name }),
				"slug":      gqlutil.StrField(func(w WorkspaceView) any { return w.Slug }),
				"plan":      gqlutil.StrField(func(w WorkspaceView) any { return w.Plan }),
				"role":      gqlutil.StrField(func(w WorkspaceView) any { return w.Role }),
				"createdAt": gqlutil.StrField(func(w WorkspaceView) any { return w.CreatedAt }),
				// requireTwoFactor is the workspace's "require two-factor
				// authentication" setting, readable by every member so a blocked
				// member can see why.
				"requireTwoFactor": gqlutil.BoolField(func(w WorkspaceView) any { return w.RequireTwoFactor }),
				// requireGoogleLogin is the workspace's "require Google login"
				// setting (w7/m465), readable by every member like
				// requireTwoFactor.
				"requireGoogleLogin": gqlutil.BoolField(func(w WorkspaceView) any { return w.RequireGoogleLogin }),
				// ipAllowList is the workspace's inbound IP rules (w9/m388),
				// Render's owner.ipAllowList: what an Admin saved, or the seeded
				// allow-all for a workspace that never saved any; an empty list
				// is deny-all. Read only when selected, so the polled workspaces
				// list never pays for it. Every resolver returning a Workspace
				// authorized the caller on it first.
				"ipAllowList": &graphql.Field{
					Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryType))),
					Resolve: func(p graphql.ResolveParams) (any, error) {
						w, _ := p.Source.(WorkspaceView)
						if w.requiresLogin() {
							if err := s.CheckTwoFactor(p.Context, w.ID); err != nil {
								return nil, err
							}
						}
						if w.IPAllowList != nil {
							return w.IPAllowList, nil
						}
						views, err := s.withIPAllowLists(p.Context, []WorkspaceView{w})
						if err != nil {
							return nil, err
						}
						return views[0].IPAllowList, nil
					},
				},
				// organization, primary and privacy place the workspace in its
				// org (w7/m464, ADR115): null, false and null for a standalone
				// workspace.
				"organization": &graphql.Field{
					Type: organizationRefGQLType,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						o, err := s.workspaceOrganization(p.Context, p.Source.(WorkspaceView))
						if o == nil || err != nil {
							return nil, err
						}
						return *o, nil
					},
				},
				"primary": &graphql.Field{
					Type: graphql.NewNonNull(graphql.Boolean),
					Resolve: func(p graphql.ResolveParams) (any, error) {
						o, err := s.workspaceOrganization(p.Context, p.Source.(WorkspaceView))
						return o != nil && o.PrimaryWorkspaceID == p.Source.(WorkspaceView).ID, err
					},
				},
				"privacy": gqlutil.OptionalStrField(func(w WorkspaceView) any { return w.Privacy }),
				// overlappingDeployPolicy is the workspace's Overlapping Deploy
				// Policy (w4/653), "wait" or "override": Render's Workspace
				// Settings section of that name. Read only when selected; the
				// coded 403 for a member the workspace's 2FA requirement blocks.
				"overlappingDeployPolicy": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(p graphql.ResolveParams) (any, error) {
						w, _ := p.Source.(WorkspaceView)
						return s.overlappingDeployPolicy(p.Context, w)
					},
				},
			},
		})
	})
	return s.gqlWorkspace
}

var pipelineSettingsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "PipelineSettings",
	Description: "A workspace's Build Pipeline settings (w6/m309): Render's Workspace Settings → Build Pipeline.",
	Fields: graphql.Fields{
		"workspaceId": gqlutil.ReqStrField(func(p PipelineSettings) any { return p.WorkspaceID }),
		// spendLimitUsd is the monthly spend limit on build minutes beyond the
		// plan's included ones ("25.00"); null when there is none.
		"spendLimitUsd": gqlutil.StrField(func(p PipelineSettings) any { return p.SpendLimitUSD }),
	},
})

var billingEmailGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "WorkspaceBillingEmail",
	Description: "Where a workspace's invoices and billing notices go (w6/m627). email is the owner's account email on Hobby, and on a higher plan until the workspace sets its own; configurable is whether its plan lets it set one (Pro and higher).",
	Fields: graphql.Fields{
		"workspaceId":  gqlutil.ReqStrField(func(v BillingEmailView) any { return v.WorkspaceID }),
		"email":        gqlutil.ReqStrField(func(v BillingEmailView) any { return v.Email }),
		"configurable": gqlutil.ReqBoolField(func(v BillingEmailView) any { return v.Configurable }),
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
		"freeUsed":    gqlutil.IntField(func(c ResourceCapView) any { return c.FreeUsed }),
		"freeLimit":   gqlutil.IntField(func(c ResourceCapView) any { return c.FreeLimit }),
	},
})

// resourceLimitsGQLType is the per-workspace cap report (w7/m9).
var resourceLimitsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ResourceLimits",
	Fields: graphql.Fields{
		"services":  gqlutil.Typed(resourceCapGQLType, func(r ResourceLimitsView) any { return r.Services }),
		"postgres":  gqlutil.Typed(resourceCapGQLType, func(r ResourceLimitsView) any { return r.Postgres }),
		"keyValues": gqlutil.Typed(resourceCapGQLType, func(r ResourceLimitsView) any { return r.KeyValues }),
		// pipelineMinutesExceeded: the workspace's builds are stopped for the
		// month, its pipeline minutes spent with no way to buy more (w3/108).
		"pipelineMinutesExceeded": gqlutil.BoolField(func(r ResourceLimitsView) any { return r.PipelineMinutesExceeded }),
	},
})

// The enable confirmation's read (w7/m471, docs/ADR012-auth.md §9c): who a
// login requirement would block, on a workspace and on an org alike.
var loginRequirementGQLEnum = graphql.NewEnum(graphql.EnumConfig{
	Name:        "LoginRequirement",
	Description: "A login requirement a workspace or an organization can turn on: two-factor authentication, or signing in with Google.",
	Values: graphql.EnumValueConfigMap{
		"TWO_FACTOR":   &graphql.EnumValueConfig{Value: LoginRequirementTwoFactor},
		"GOOGLE_LOGIN": &graphql.EnumValueConfig{Value: LoginRequirementGoogleLogin},
	},
})

var loginImpactAPIKeyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LoginRequirementImpactApiKey",
	Fields: graphql.Fields{
		"id":   gqlutil.ReqStrField(func(k LoginImpactAPIKey) any { return k.ID }),
		"name": gqlutil.ReqStrField(func(k LoginImpactAPIKey) any { return k.Name }),
		// workspaceName is the workspace the key is bound to, on an org's
		// read; empty on a workspace's.
		"workspaceName": gqlutil.ReqStrField(func(k LoginImpactAPIKey) any { return k.WorkspaceName }),
	},
})

var loginImpactSSHKeyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LoginRequirementImpactSshKey",
	Fields: graphql.Fields{
		"id":          gqlutil.ReqStrField(func(k LoginImpactSSHKey) any { return k.ID }),
		"name":        gqlutil.ReqStrField(func(k LoginImpactSSHKey) any { return k.Name }),
		"fingerprint": gqlutil.ReqStrField(func(k LoginImpactSSHKey) any { return k.Fingerprint }),
	},
})

var loginImpactMemberGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LoginRequirementImpactMember",
	Fields: graphql.Fields{
		"userId":      gqlutil.ReqStrField(func(m LoginImpactMember) any { return m.UserID }),
		"email":       gqlutil.ReqStrField(func(m LoginImpactMember) any { return m.Email }),
		"name":        gqlutil.ReqStrField(func(m LoginImpactMember) any { return m.Name }),
		"losesAccess": gqlutil.ReqBoolField(func(m LoginImpactMember) any { return m.LosesAccess }),
		"apiKeys": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(loginImpactAPIKeyGQLType))),
			func(m LoginImpactMember) any { return m.APIKeys }),
		"sshKeys": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(loginImpactSSHKeyGQLType))),
			func(m LoginImpactMember) any { return m.SSHKeys }),
	},
})

var loginRequirementImpactGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "LoginRequirementImpact",
	Description: "Who turning a login requirement on would block, decided by the enforcement gate's own checks: members who would lose access or whose keys would stop working, and API keys no member created. Empty lists mean nobody would be blocked.",
	Fields: graphql.Fields{
		"members": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(loginImpactMemberGQLType))),
			func(i LoginRequirementImpact) any { return i.Members }),
		"unattributedApiKeys": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(loginImpactAPIKeyGQLType))),
			func(i LoginRequirementImpact) any { return i.UnattributedAPIKeys }),
	},
})

// loginRequirementImpactField is the read on one scope: the scope's id
// argument and the requirement being turned on.
func loginRequirementImpactField(scopeArg, description string, read func(ctx context.Context, id, requirement string) (LoginRequirementImpact, error)) *graphql.Field {
	return &graphql.Field{
		Type:        graphql.NewNonNull(loginRequirementImpactGQLType),
		Description: description,
		Args: graphql.FieldConfigArgument{
			scopeArg:      gqlutil.ReqArg(graphql.String),
			"requirement": gqlutil.ReqArg(loginRequirementGQLEnum),
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			return read(p.Context, p.Args[scopeArg].(string), p.Args["requirement"].(string))
		},
	}
}

// GraphQLQuery contributes the caller's workspace list and limits query to the
// root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	fields := s.organizationGraphQLQuery()
	maps.Copy(fields, s.workspaceGraphQLQuery())
	return fields
}

// withSelectedSettings reads the inbound IP rules and the Overlapping Deploy
// Policy for a whole workspaces list at once, one store read each, when the
// query selects them. Read per workspace by the field resolvers, a document
// the cost gate admits (11 copies of the list, each field aliased 10 times)
// made 42 store reads a membership: 42,000 and 4 s for a caller in 1,000
// workspaces, which an org Owner creates at no fee (2026-11-20). The polled
// switcher list selects neither, so it still pays for neither.
func (s *Service) withSelectedSettings(p graphql.ResolveParams, views []WorkspaceView) ([]WorkspaceView, error) {
	var err error
	if gqlutil.Selects(p, "ipAllowList") {
		if views, err = s.withIPAllowLists(p.Context, views); err != nil {
			return nil, err
		}
	}
	if gqlutil.Selects(p, "overlappingDeployPolicy") {
		return s.withOverlappingDeployPolicies(p.Context, views)
	}
	return views, nil
}

func (s *Service) workspaceGraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"workspaces": &graphql.Field{
			Type: graphql.NewList(s.workspaceType()),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				views, err := s.List(p.Context)
				if err != nil {
					return nil, err
				}
				return s.withSelectedSettings(p, views)
			},
		},
		// workspace is one workspace the caller belongs to (can_view), Render's
		// GET /v1/owners/{ownerId}: what Workspace Settings reads the inbound IP
		// rules through (w9/m388), so the polled workspaces list stays lean.
		"workspace": &graphql.Field{
			Type: s.workspaceType(),
			Args: graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetWorkspace(p.Context, p.Args["id"].(string))
			},
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
		// pipelineSettings is the workspace's Build Pipeline settings (w6/m309),
		// for members who may view billing details (Admin, Developer, Billing;
		// w9/m620). GraphQL-only: Render's REST and MCP have none.
		"pipelineSettings": &graphql.Field{
			Type: pipelineSettingsGQLType,
			Args: graphql.FieldConfigArgument{"workspaceId": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.PipelineSettings(p.Context, p.Args["workspaceId"].(string))
			},
		},
		// workspaceBillingEmail is the workspace's billing email (w6/m627), for
		// members who may view billing details (Admin, Developer, Billing).
		"workspaceBillingEmail": &graphql.Field{
			Type: billingEmailGQLType,
			Args: graphql.FieldConfigArgument{"workspaceId": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.BillingEmail(p.Context, p.Args["workspaceId"].(string))
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
		// workspaceLoginRequirementImpact is the enable confirmation's list
		// (w7/m471): admin only, GraphQL-only like the setters.
		"workspaceLoginRequirementImpact": loginRequirementImpactField("workspaceId",
			"Who turning a login requirement on in a workspace would block, with what the workspace already requires (admin only).",
			s.LoginRequirementImpact),
	}
}

// GraphQLMutation contributes the lifecycle mutations to the root Mutation.
func (s *Service) GraphQLMutation() graphql.Fields {
	fields := s.organizationGraphQLMutation()
	maps.Copy(fields, s.workspaceGraphQLMutation())
	return fields
}

func (s *Service) workspaceGraphQLMutation() graphql.Fields {
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
			Type: s.workspaceType(),
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
			Type: s.workspaceType(),
			Args: graphql.FieldConfigArgument{
				"name": gqlutil.ReqArg(graphql.String),
				"plan": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				plan := gqlutil.Str(p.Args, "plan")
				return s.Create(p.Context, p.Args["name"].(string), plan)
			},
		},
		"renameWorkspace": gqlutil.ArgMutation(s.workspaceType(), "name", s.Rename),
		// setWorkspaceIPAllowList replaces the workspace's inbound IP rules
		// (w9/m388): Admin only, Scale and Enterprise (PLAN_FEATURE_REQUIRED
		// below), an empty list is deny-all. GraphQL-only: Render's REST owners
		// surface is read-only and its MCP has no workspace mutations.
		"setWorkspaceIPAllowList": &graphql.Field{
			Type: s.workspaceType(),
			Args: graphql.FieldConfigArgument{
				"id":          gqlutil.ReqArg(graphql.String),
				"ipAllowList": gqlutil.ReqArg(graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetIPAllowList(p.Context, p.Args["id"].(string), core.ProvidedAllowList(gqlutil.AllowList(p.Args["ipAllowList"])))
			},
		},
		// setWorkspaceRequireTwoFactor turns "require two-factor
		// authentication" on or off for a workspace; admin-only. GraphQL-only:
		// Render has no REST workspace-mutation API.
		"setWorkspaceRequireTwoFactor": &graphql.Field{
			Type: s.workspaceType(),
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"enabled":     gqlutil.ReqArg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetRequireTwoFactor(p.Context, p.Args["workspaceId"].(string), p.Args["enabled"].(bool))
			},
		},
		// setWorkspaceRequireGoogleLogin turns "require Google login" on or
		// off for a workspace (w7/m465); admin-only, GraphQL-only like the 2FA
		// setting.
		"setWorkspaceRequireGoogleLogin": &graphql.Field{
			Type: s.workspaceType(),
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"enabled":     gqlutil.ReqArg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetRequireGoogleLogin(p.Context, p.Args["workspaceId"].(string), p.Args["enabled"].(bool))
			},
		},
		// setWorkspaceOverlappingDeployPolicy sets the workspace's Overlapping
		// Deploy Policy (w4/653), "wait" or "override": Admin only.
		// GraphQL-only: Render's public API and MCP have no such operation.
		"setWorkspaceOverlappingDeployPolicy": &graphql.Field{
			Type: s.workspaceType(),
			Args: graphql.FieldConfigArgument{
				"id":     gqlutil.ReqArg(graphql.String),
				"policy": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetOverlappingDeployPolicy(p.Context, p.Args["id"].(string), p.Args["policy"].(string))
			},
		},
		// setWorkspaceBillingEmail sets a Pro or higher workspace's billing
		// email (w6/m627): Admin and Billing; PLAN_FEATURE_REQUIRED on Hobby.
		"setWorkspaceBillingEmail": &graphql.Field{
			Type: billingEmailGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"email":       gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetBillingEmail(p.Context, p.Args["workspaceId"].(string), p.Args["email"].(string))
			},
		},
		// setPipelineSpendLimit sets the workspace's monthly pipeline spend
		// limit, or removes it when spendLimitUsd is null or left out (w6/m309).
		"setPipelineSpendLimit": &graphql.Field{
			Type: pipelineSettingsGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId":   gqlutil.ReqArg(graphql.String),
				"spendLimitUsd": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				var limit *string
				if v, ok := p.Args["spendLimitUsd"].(string); ok {
					limit = &v
				}
				return s.SetPipelineSpendLimit(p.Context, p.Args["workspaceId"].(string), limit)
			},
		},
		"changeWorkspacePlan": &graphql.Field{
			// w6/m12: upgrade/downgrade a workspace's plan
			// (docs/render-artifacts/workspace-plan-change.md). GraphQL-only —
			// Render's REST owners surface has no plan-mutation endpoint and its
			// MCP has no workspace mutations, so REST/MCP deliberately gain
			// nothing here (parity by absence).
			Type: s.workspaceType(),
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
					Description: `must equal "sudo delete workspace <name>" (the workspace's display name)`,
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
