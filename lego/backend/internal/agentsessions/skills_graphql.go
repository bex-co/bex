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

package agentsessions

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// GraphQL for ADR099 workspace skills, over the same core as REST and MCP.

var agentSkillGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentSkill",
	Description: "A workspace skill for cloud agents (ADR099): a SKILL.md every session of the workspace receives. body and content are on single reads only.",
	Fields: graphql.Fields{
		"id":          gqlutil.ReqStrField(func(v AgentSkill) any { return v.ID }),
		"ownerId":     gqlutil.ReqStrField(func(v AgentSkill) any { return v.OwnerID }),
		"name":        gqlutil.ReqStrField(func(v AgentSkill) any { return v.Name }),
		"description": gqlutil.ReqStrField(func(v AgentSkill) any { return v.Description }),
		"triggers": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
			func(v AgentSkill) any { return v.Triggers }),
		"enabled":   gqlutil.ReqBoolField(func(v AgentSkill) any { return v.Enabled }),
		"body":      gqlutil.OptionalStrField(func(v AgentSkill) any { return v.Body }),
		"content":   gqlutil.OptionalStrField(func(v AgentSkill) any { return v.Content }),
		"version":   gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(v AgentSkill) any { return v.Version }),
		"updatedBy": gqlutil.OptionalStrField(func(v AgentSkill) any { return v.UpdatedBy }),
		"createdAt": gqlutil.ReqStrField(func(v AgentSkill) any { return gqlTime(v.CreatedAt) }),
		"updatedAt": gqlutil.ReqStrField(func(v AgentSkill) any { return gqlTime(v.UpdatedAt) }),
	},
})

var agentSkillListGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSkillList",
	Fields: graphql.Fields{
		"ownerId": gqlutil.ReqStrField(func(v AgentSkillList) any { return v.OwnerID }),
		"skills":  gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentSkillGQLType))), func(v AgentSkillList) any { return v.Skills }),
	},
})

var agentSkillGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name:        "AgentSkillInput",
	Description: "A workspace skill's fields, or a whole SKILL.md as content. On update, omitted fields keep their value.",
	Fields: graphql.InputObjectConfigFieldMap{
		"name":        &graphql.InputObjectFieldConfig{Type: graphql.String},
		"description": &graphql.InputObjectFieldConfig{Type: graphql.String},
		"triggers":    &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: `["user","model"] (default) or ["user"] (only when invoked).`},
		"enabled":     &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		"body":        &graphql.InputObjectFieldConfig{Type: graphql.String},
		"content":     &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "A whole SKILL.md, instead of name, description, triggers and body."},
		"ifVersion":   &graphql.InputObjectFieldConfig{Type: graphql.Int, Description: "Update only: refuse unless the skill is still at this version."},
	},
})

var agentSessionSkillGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name:        "AgentSessionSkillInput",
	Description: "A workspace skill a turn must follow (ADR099), with optional arguments for $ARGUMENTS.",
	Fields: graphql.InputObjectConfigFieldMap{
		"name":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"arguments": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

var agentSessionSkillGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionSkill",
	Fields: graphql.Fields{
		"name":      gqlutil.ReqStrField(func(v SkillInvocation) any { return v.Name }),
		"arguments": gqlutil.OptionalStrField(func(v SkillInvocation) any { return v.Arguments }),
	},
})

func skillInputArg(raw any) AgentSkillInput {
	m, _ := raw.(map[string]any)
	in := AgentSkillInput{
		Name: gqlutil.StrPtr(m, "name"), Description: gqlutil.StrPtr(m, "description"),
		Enabled: gqlutil.BoolPtr(m, "enabled"), Body: gqlutil.StrPtr(m, "body"), Content: gqlutil.StrPtr(m, "content"),
		Triggers: gqlutil.StringList(m["triggers"]),
	}
	if v, ok := m["ifVersion"].(int); ok {
		in.IfVersion = &v
	}
	return in
}

// skillsArg reads the ADR099 invocations shared by create and steer.
func skillsArg(args map[string]any) []SkillInvocation {
	raw, _ := args["skills"].([]any)
	var out []SkillInvocation
	for _, item := range raw {
		m, _ := item.(map[string]any)
		out = append(out, SkillInvocation{Name: gqlutil.Str(m, "name"), Arguments: gqlutil.Str(m, "arguments")})
	}
	return out
}

func skillRefArgs() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"ownerId": gqlutil.Arg(graphql.String),
		"skill":   gqlutil.ReqArg(graphql.String),
	}
}

func (s *Service) skillQueries() graphql.Fields {
	return graphql.Fields{
		"agentSkills": &graphql.Field{
			Type:        graphql.NewNonNull(agentSkillListGQLType),
			Description: "The workspace's skills for cloud agents, without bodies.",
			Args:        graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.AgentSkills(p.Context, gqlutil.Str(p.Args, "ownerId"))
			},
		},
		"agentSkill": &graphql.Field{
			Type:        graphql.NewNonNull(agentSkillGQLType),
			Description: "One workspace skill by skl- id or name, with its body and SKILL.md.",
			Args:        skillRefArgs(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.AgentSkill(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "skill"))
			},
		},
	}
}

func (s *Service) skillMutations() graphql.Fields {
	updateArgs := skillRefArgs()
	updateArgs["input"] = gqlutil.ReqArg(agentSkillGQLInput)
	return graphql.Fields{
		"createAgentSkill": &graphql.Field{
			Type:        graphql.NewNonNull(agentSkillGQLType),
			Description: "Add a workspace skill (admins only).",
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"input":   gqlutil.ReqArg(agentSkillGQLInput),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreateAgentSkill(p.Context, gqlutil.Str(p.Args, "ownerId"), skillInputArg(p.Args["input"]))
			},
		},
		"updateAgentSkill": &graphql.Field{
			Type:        graphql.NewNonNull(agentSkillGQLType),
			Description: "Change a workspace skill (admins only).",
			Args:        updateArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UpdateAgentSkill(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "skill"), skillInputArg(p.Args["input"]))
			},
		},
		"deleteAgentSkill": &graphql.Field{
			Type:        graphql.Boolean,
			Description: "Delete a workspace skill (admins only).",
			Args:        skillRefArgs(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.DeleteAgentSkill(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "skill")); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
	}
}
