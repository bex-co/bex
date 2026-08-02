package agentsessions

import (
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

var agentConfigGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionConfig",
	Fields: graphql.Fields{
		"agent":         &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v AgentConfig) any { return v.Agent })},
		"model":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v AgentConfig) any { return v.Model })},
		"modelEndpoint": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v AgentConfig) any { return v.ModelEndpoint })},
		"task":          &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v AgentConfig) any { return v.Task })},
		"template":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v AgentConfig) any { return v.Template })},
	},
})

var agentConfigGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "AgentSessionConfigInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"agent":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"model":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"modelEndpoint": &graphql.InputObjectFieldConfig{Type: graphql.String},
		"task":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"template":      &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

func gqlTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

var agentSessionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSession",
	Fields: graphql.Fields{
		"id":          &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.ID })},
		"ownerId":     &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.OwnerID })},
		"repo":        &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.Repo })},
		"branch":      &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.Branch })},
		"agentConfig": &graphql.Field{Type: graphql.NewNonNull(agentConfigGQLType), Resolve: gqlutil.Field(func(v View) any { return v.AgentConfig })},
		"sandboxId":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any { return v.SandboxID })},
		"phase":       &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.Phase })},
		"status":      &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.Status })},
		"createdAt":   &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return gqlTime(v.CreatedAt) })},
		"updatedAt":   &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return gqlTime(v.UpdatedAt) })},
		"canceledAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any {
			if v.CanceledAt == nil {
				return nil
			}
			return gqlTime(*v.CanceledAt)
		})},
		"ticket": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any { return v.Ticket })},
		"url":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any { return v.URL })},
		"expiresAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any {
			if v.ExpiresAt == nil {
				return nil
			}
			return gqlTime(*v.ExpiresAt)
		})},
	},
})

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func configArg(args map[string]any) AgentConfig {
	raw, _ := args["agentConfig"].(map[string]any)
	return AgentConfig{Agent: stringArg(raw, "agent"), Model: stringArg(raw, "model"), ModelEndpoint: stringArg(raw, "modelEndpoint"), Task: stringArg(raw, "task"), Template: stringArg(raw, "template")}
}

func stringListArg(args map[string]any, key string) []string {
	raw, _ := args[key].([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok {
			out = append(out, value)
		}
	}
	return out
}

func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"agentSessions": &graphql.Field{
			Type:    graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentSessionGQLType))),
			Args:    graphql.FieldConfigArgument{"ownerId": &graphql.ArgumentConfig{Type: graphql.String}},
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.List(p.Context, stringArg(p.Args, "ownerId")) },
		},
		"agentSession": &graphql.Field{
			Type:    agentSessionGQLType,
			Args:    graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.Get(p.Context, stringArg(p.Args, "id")) },
		},
	}
}

func (s *Service) GraphQLMutation() graphql.Fields {
	idArg := graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}}
	return graphql.Fields{
		"createAgentSession": &graphql.Field{
			Type: agentSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId":         &graphql.ArgumentConfig{Type: graphql.String},
				"repo":            &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"branch":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"agentConfig":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(agentConfigGQLInput)},
				"egressAllowlist": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Create(p.Context, CreateRequest{OwnerID: stringArg(p.Args, "ownerId"), Repo: stringArg(p.Args, "repo"), Branch: stringArg(p.Args, "branch"), AgentConfig: configArg(p.Args), EgressAllowlist: stringListArg(p.Args, "egressAllowlist")})
			},
		},
		"resumeAgentSession": &graphql.Field{Type: agentSessionGQLType, Args: idArg, Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.Resume(p.Context, stringArg(p.Args, "id"))
		}},
		"cancelAgentSession": &graphql.Field{Type: agentSessionGQLType, Args: idArg, Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.Cancel(p.Context, stringArg(p.Args, "id"))
		}},
	}
}
