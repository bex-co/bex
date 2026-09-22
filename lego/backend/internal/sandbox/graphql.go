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

package sandbox

import (
	"context"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

var sandboxNetworkPolicyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxNetworkPolicy",
	Fields: graphql.Fields{
		"default": gqlutil.ReqStrField(func(p *NetworkPolicy) any { return string(p.Default) }),
	},
})

var sandboxNetworkPolicyInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "SandboxNetworkPolicyInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"default": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

// sandboxGQLType is the GraphQL projection of a Sandbox, keeping the third
// surface behavior-identical to REST/MCP (internal/api/AGENTS.md three-adapter
// parity). Extra fields (owner/workspace/image) are a safe superset over the
// Render REST shape.
var sandboxGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Sandbox",
	Fields: graphql.Fields{
		"id":        gqlutil.ReqStrField(func(s Sandbox) any { return s.ID }),
		"plan":      gqlutil.StrField(func(s Sandbox) any { return string(s.Plan) }),
		"status":    gqlutil.ReqStrField(func(s Sandbox) any { return string(s.Status) }),
		"owner":     gqlutil.StrField(func(s Sandbox) any { return s.Owner }),
		"workspace": gqlutil.StrField(func(s Sandbox) any { return s.Workspace }),
		"image":     gqlutil.StrField(func(s Sandbox) any { return s.Image }),
		"region":    gqlutil.StrField(func(s Sandbox) any { return s.Region }),
		"timeoutSeconds": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(s Sandbox) any {
			return s.TimeoutSeconds
		})},
		"networkPolicy": &graphql.Field{Type: sandboxNetworkPolicyGQLType, Resolve: gqlutil.Field(func(s Sandbox) any {
			return s.NetworkPolicy
		})},
	},
})

// ownerScoped selects the requested workspace while preserving the default when omitted.
func ownerScoped(p graphql.ResolveParams) context.Context {
	return core.WithWorkspace(p.Context, gqlutil.Str(p.Args, "ownerId"))
}

func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"sandboxes": &graphql.Field{
			Type: graphql.NewList(sandboxGQLType),
			Args: graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.List(ownerScoped(p))
			},
		},
		"sandbox": &graphql.Field{
			Type: sandboxGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(ownerScoped(p), p.Args["id"].(string))
			},
		},
	}
}

func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createSandbox": &graphql.Field{
			Type: sandboxGQLType,
			Args: graphql.FieldConfigArgument{
				"template":       gqlutil.Arg(graphql.String),
				"plan":           gqlutil.Arg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
				"region":         gqlutil.Arg(graphql.String),
				"timeoutSeconds": gqlutil.Arg(graphql.Int),
				"networkPolicy":  gqlutil.Arg(sandboxNetworkPolicyInput),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				var policy *NetworkPolicy
				if raw, ok := p.Args["networkPolicy"].(map[string]any); ok {
					policy = &NetworkPolicy{Default: NetworkPolicyDefault(gqlutil.Str(raw, "default"))}
				}
				return s.Create(p.Context, CreateRequest{
					OwnerID:        gqlutil.Str(p.Args, "ownerId"),
					Template:       gqlutil.Str(p.Args, "template"),
					Plan:           Plan(gqlutil.Str(p.Args, "plan")),
					Region:         gqlutil.Str(p.Args, "region"),
					TimeoutSeconds: gqlutil.Int(p.Args, "timeoutSeconds"),
					NetworkPolicy:  policy,
				})
			},
		},
		"terminateSandbox": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.Terminate(ownerScoped(p), p.Args["id"].(string)); err != nil {
					return false, err
				}
				return true, nil
			},
		},
	}
}
