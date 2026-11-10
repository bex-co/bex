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
	"fmt"
	"maps"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// networkPolicyTypeField resolves both `type` and its `default` alias.
var networkPolicyTypeField = gqlutil.ReqStrField(func(p *NetworkPolicy) any { return string(p.Default) })

var sandboxNetworkPolicyRuleGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxNetworkPolicyRule",
	Fields: graphql.Fields{
		"domain":   gqlutil.ReqStrField(func(r NetworkPolicyRule) any { return r.Domain }),
		"protocol": gqlutil.ReqStrField(func(r NetworkPolicyRule) any { return r.Protocol }),
	},
})

var sandboxNetworkPolicyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxNetworkPolicy",
	Fields: graphql.Fields{
		"type":    networkPolicyTypeField,
		"default": networkPolicyTypeField, // Render's deprecated alias of type
		// Allow-list destinations; null for allow-all and deny-all, as REST
		// and MCP omit them.
		"rules": gqlutil.Typed(graphql.NewList(graphql.NewNonNull(sandboxNetworkPolicyRuleGQLType)), func(p *NetworkPolicy) any {
			if p.Rules == nil {
				return nil
			}
			return p.Rules
		}),
	},
})

var sandboxNetworkPolicyRuleInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "SandboxNetworkPolicyRuleInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"domain":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"protocol": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

var sandboxNetworkPolicyInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "SandboxNetworkPolicyInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"type":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		"default": &graphql.InputObjectFieldConfig{Type: graphql.String},
		"rules":   &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(sandboxNetworkPolicyRuleInput))},
	},
})

// networkPolicyFromArgs decodes the GraphQL input into the shared create shape:
// an absent `rules` stays nil (so allow-all/deny-all are not refused), a
// present one — even empty — is passed on for the shared validation.
func networkPolicyFromArgs(raw map[string]any) *NetworkPolicy {
	policy := &NetworkPolicy{Default: NetworkPolicyDefault(gqlutil.Str(raw, "default")), Type: NetworkPolicyDefault(gqlutil.Str(raw, "type"))}
	if items, ok := raw["rules"].([]any); ok {
		policy.Rules = make([]NetworkPolicyRule, 0, len(items))
		for _, item := range items {
			rule, _ := item.(map[string]any)
			policy.Rules = append(policy.Rules, NetworkPolicyRule{Domain: gqlutil.Str(rule, "domain"), Protocol: gqlutil.Str(rule, "protocol")})
		}
	}
	return policy
}

// sandboxExecutionGQLType is Render's execution record (w32/m298).
var sandboxExecutionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxExecution",
	Fields: graphql.Fields{
		"id":        gqlutil.ReqStrField(func(e Execution) any { return e.ID }),
		"sandboxId": gqlutil.ReqStrField(func(e Execution) any { return e.SandboxID }),
		"type":      gqlutil.ReqStrField(func(e Execution) any { return e.Type }),
		"operation": gqlutil.ReqStrField(func(e Execution) any { return e.Operation }),
		"command":   gqlutil.StrField(func(e Execution) any { return e.Command }),
		"userId":    gqlutil.ReqStrField(func(e Execution) any { return e.UserID }),
		"startedAt": gqlutil.TimeField(func(e Execution) any { return e.StartedAt }),
		"stoppedAt": gqlutil.TimeField(func(e Execution) any { return e.StoppedAt }),
		"exitCode":  gqlutil.IntField(func(e Execution) any { return e.ExitCode }),
	},
})

// sandboxGQLType is the GraphQL projection of a Sandbox, keeping the third
// surface behavior-identical to REST/MCP (internal/api/AGENTS.md three-adapter
// parity). Extra fields (owner/workspace/image) are a safe superset over the
// Render REST shape.
var sandboxGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Sandbox",
	Fields: graphql.Fields{
		"id": gqlutil.ReqStrField(func(s Sandbox) any { return s.ID }),
		// Render's optional sandbox name (w5/m287); null when unnamed.
		"name": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(s Sandbox) any {
			if s.Name == nil {
				return nil
			}
			return *s.Name
		})},
		"plan":      gqlutil.StrField(func(s Sandbox) any { return string(s.Plan) }),
		"status":    gqlutil.ReqStrField(func(s Sandbox) any { return string(s.Status) }),
		"owner":     gqlutil.StrField(func(s Sandbox) any { return s.Owner }),
		"workspace": gqlutil.StrField(func(s Sandbox) any { return s.Workspace }),
		"image":     gqlutil.StrField(func(s Sandbox) any { return s.Image }),
		"region":    gqlutil.StrField(func(s Sandbox) any { return s.Region }),
		"timeoutSeconds": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(s Sandbox) any {
			return s.TimeoutSeconds
		})},
		"idleTimeoutSeconds": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(s Sandbox) any {
			return s.IdleTimeoutSeconds
		})},
		"networkPolicy": &graphql.Field{Type: sandboxNetworkPolicyGQLType, Resolve: gqlutil.Field(func(s Sandbox) any {
			return s.NetworkPolicy
		})},
		// The group the sandbox belongs to (w22/m311); null for an agent-session
		// sandbox and when the deployment has no group store.
		"sandboxGroupId": gqlutil.OptionalStrField(func(s Sandbox) any { return s.SandboxGroupID }),
	},
})

// envFromArgs turns GraphQL's `[{key, value}]` into Render's env map. A
// sandbox has no generated values, and a repeated key is ambiguous.
func envFromArgs(raw any) (map[string]string, error) {
	list, _ := raw.([]any)
	if len(list) == 0 {
		return nil, nil
	}
	env := make(map[string]string, len(list))
	for _, item := range list {
		m, _ := item.(map[string]any)
		key, _ := m["key"].(string)
		if generate, _ := m["generateValue"].(bool); generate {
			return nil, core.NewBadRequestError(CodeSandboxEnvInvalid,
				fmt.Sprintf("environment variable %q: generateValue is not supported for sandboxes; pass a value", key),
				map[string]any{"field": "env", "name": key})
		}
		if _, dup := env[key]; dup {
			return nil, core.NewBadRequestError(CodeSandboxEnvInvalid,
				fmt.Sprintf("environment variable %q is given more than once", key),
				map[string]any{"field": "env", "name": key})
		}
		env[key], _ = m["value"].(string)
	}
	return env, nil
}

// ownerScoped selects the requested workspace while preserving the default when omitted.
func ownerScoped(p graphql.ResolveParams) context.Context {
	return core.WithWorkspace(p.Context, gqlutil.Str(p.Args, "ownerId"))
}

func (s *Service) GraphQLQuery() graphql.Fields {
	fields := graphql.Fields{
		"sandboxes": &graphql.Field{
			Type: graphql.NewList(sandboxGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				// Render's list `status` filter; omitted = every status but terminated.
				"status": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				// Render's exact-match name filter.
				"name": gqlutil.Arg(graphql.String),
				// Render's group filter (w22/m311): the group must belong to ownerId.
				"sandboxGroupId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.listFiltered(ownerScoped(p), gqlutil.StringList(p.Args["status"]), gqlutil.Str(p.Args, "name"), gqlutil.Str(p.Args, "sandboxGroupId"))
			},
		},
		// Render's execution history, newest first (w32/m298).
		"sandboxExecutions": &graphql.Field{
			Type: graphql.NewList(sandboxExecutionGQLType),
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
				"cursor":  gqlutil.Arg(graphql.String),
				"limit":   gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.listExecutionViews(ownerScoped(p), p.Args["id"].(string), gqlutil.Str(p.Args, "cursor"), core.PageLimitOrDefault(gqlutil.Int(p.Args, "limit")))
			},
		},
		"sandboxExecution": &graphql.Field{
			Type: sandboxExecutionGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          gqlutil.ReqArg(graphql.String),
				"executionId": gqlutil.ReqArg(graphql.String),
				"ownerId":     gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetExecution(ownerScoped(p), p.Args["id"].(string), p.Args["executionId"].(string))
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
	maps.Copy(fields, s.groupQueryFields())
	return fields
}

func (s *Service) GraphQLMutation() graphql.Fields {
	fields := graphql.Fields{
		"createSandbox": &graphql.Field{
			Type: sandboxGQLType,
			Args: graphql.FieldConfigArgument{
				"template":       gqlutil.Arg(graphql.String),
				"plan":           gqlutil.Arg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
				"region":         gqlutil.Arg(graphql.String),
				"timeoutSeconds": gqlutil.Arg(graphql.Int),
				// Render's idleTimeoutSeconds: omitted = 900, 0 disables idle
				// suspension; see resolveIdleTimeout.
				"idleTimeoutSeconds": gqlutil.Arg(graphql.Int),
				"networkPolicy":      gqlutil.Arg(sandboxNetworkPolicyInput),
				"name":               gqlutil.Arg(graphql.String),
				// Render's sandboxGroupId (w22/m311): omitted = the default group.
				"sandboxGroupId": gqlutil.Arg(graphql.String),
				// Render's create-time env; write-only, so no read field.
				"env": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(gqlutil.EnvVarInputType))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				env, err := envFromArgs(p.Args["env"])
				if err != nil {
					return nil, err
				}
				var policy *NetworkPolicy
				if raw, ok := p.Args["networkPolicy"].(map[string]any); ok {
					policy = networkPolicyFromArgs(raw)
				}
				return s.Create(p.Context, CreateRequest{
					OwnerID:            gqlutil.Str(p.Args, "ownerId"),
					Template:           gqlutil.Str(p.Args, "template"),
					Plan:               Plan(gqlutil.Str(p.Args, "plan")),
					Region:             gqlutil.Str(p.Args, "region"),
					TimeoutSeconds:     gqlutil.IntPtr(p.Args, "timeoutSeconds"),
					IdleTimeoutSeconds: gqlutil.IntPtr(p.Args, "idleTimeoutSeconds"),
					Name:               gqlutil.Str(p.Args, "name"),
					NetworkPolicy:      policy,
					Env:                env,
					SandboxGroupID:     gqlutil.Str(p.Args, "sandboxGroupId"),
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
	maps.Copy(fields, s.groupMutationFields())
	return fields
}
