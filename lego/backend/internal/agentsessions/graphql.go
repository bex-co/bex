package agentsessions

import (
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
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

var agentProfileGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionProfile",
	Fields: graphql.Fields{
		"id":    &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v AgentProfile) any { return v.ID })},
		"label": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v AgentProfile) any { return v.Label })},
	},
})

var githubReadinessGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionGitHubReadiness",
	Fields: graphql.Fields{
		"connected":    &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v GitHubReadinessView) any { return v.Connected })},
		"accountLogin": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v GitHubReadinessView) any { return v.AccountLogin })},
		"installUrl":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v GitHubReadinessView) any { return v.InstallURL })},
	},
})

var agentCapabilitiesGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionCapabilities",
	Fields: graphql.Fields{
		"enabled":       &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v Capabilities) any { return v.Enabled })},
		"github":        &graphql.Field{Type: graphql.NewNonNull(githubReadinessGQLType), Resolve: gqlutil.Field(func(v Capabilities) any { return v.GitHub })},
		"modelKeyReady": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v Capabilities) any { return v.ModelKeyReady })},
		"agents":        &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentProfileGQLType))), Resolve: gqlutil.Field(func(v Capabilities) any { return v.Agents })},
		"ready":         &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v Capabilities) any { return v.Ready })},
	},
})

var evidenceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionEvidence",
	Fields: graphql.Fields{
		"commandLog":   &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Resolve: gqlutil.Field(func(v Evidence) any { return v.CommandLog })},
		"testOutput":   &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Resolve: gqlutil.Field(func(v Evidence) any { return v.TestOutput })},
		"outputTail":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v Evidence) any { return v.OutputTail })},
		"changedFiles": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Resolve: gqlutil.Field(func(v Evidence) any { return v.ChangedFiles })},
		"commits":      &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(v Evidence) any { return v.Commits })},
		"truncated":    &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v Evidence) any { return v.Truncated })},
	},
})

var agentSessionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSession",
	Fields: graphql.Fields{
		"id":          &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.ID })},
		"ownerId":     &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.OwnerID })},
		"repo":        &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.Repo })},
		"branch":      &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.Branch })},
		"agentConfig": &graphql.Field{Type: graphql.NewNonNull(agentConfigGQLType), Resolve: gqlutil.Field(func(v View) any { return v.AgentConfig })},
		"sandboxId":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any { return v.SandboxID })},
		"sshAddress":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any { return v.SSHAddress })},
		"phase":       &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.Phase })},
		"status":      &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return v.Status })},
		"headSha":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any { return v.HeadSHA })},
		"prUrl":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any { return v.PRURL })},
		"prNumber":    &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(v View) any { return v.PRNumber })},
		"evidence": &graphql.Field{Type: evidenceGQLType, Resolve: gqlutil.Field(func(v View) any {
			if v.Evidence == nil {
				return nil
			}
			return *v.Evidence
		})},
		"turns":         &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(v View) any { return v.Turns })},
		"deliveryMode":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any { return v.DeliveryMode })},
		"failureReason": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any { return v.FailureReason })},
		"createdAt":     &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return gqlTime(v.CreatedAt) })},
		"updatedAt":     &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v View) any { return gqlTime(v.UpdatedAt) })},
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
		"pinned":        &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v View) any { return v.Pinned })},
		"snapshotBytes": &graphql.Field{Type: graphql.Float, Resolve: gqlutil.Field(func(v View) any { return v.SnapshotBytes })},
		"hibernatedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any {
			if v.HibernatedAt == nil {
				return nil
			}
			return gqlTime(*v.HibernatedAt)
		})},
		"retainUntil": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any {
			if v.RetainUntil == nil {
				return nil
			}
			return gqlTime(*v.RetainUntil)
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
		"agentSessionCapabilities": &graphql.Field{
			Type: graphql.NewNonNull(agentCapabilitiesGQLType),
			Args: graphql.FieldConfigArgument{"ownerId": &graphql.ArgumentConfig{Type: graphql.String}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Capabilities(p.Context, stringArg(p.Args, "ownerId"))
			},
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
				return s.Create(p.Context, CreateRequest{OwnerID: stringArg(p.Args, "ownerId"), Repo: stringArg(p.Args, "repo"), Branch: stringArg(p.Args, "branch"), AgentConfig: configArg(p.Args), EgressAllowlist: gqlutil.StringList(p.Args["egressAllowlist"])})
			},
		},
		"steerAgentSession": &graphql.Field{
			Type: agentSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"id":              &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"prompt":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"egressAllowlist": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Steer(p.Context, SteerRequest{SessionID: stringArg(p.Args, "id"), Prompt: stringArg(p.Args, "prompt"), EgressAllowlist: gqlutil.StringList(p.Args["egressAllowlist"])})
			},
		},
		"resumeAgentSession": &graphql.Field{Type: agentSessionGQLType, Args: idArg, Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.Resume(p.Context, stringArg(p.Args, "id"))
		}},
		"attachAgentSession": &graphql.Field{
			Type: agentSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"action": &graphql.ArgumentConfig{
					Type:        graphql.String,
					Description: "Ticket action: 'read' for transcript replay (GET), 'turn' for live prompt execution (POST). Requires can_create for 'turn'. Defaults to 'read'.",
				},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				action := stringArg(p.Args, "action")
				if action == "" {
					action = agentsessionticket.ActionRead // Default to read for safety
				}
				return s.AttachTicket(p.Context, stringArg(p.Args, "id"), action)
			},
		},
		"cancelAgentSession": &graphql.Field{Type: agentSessionGQLType, Args: idArg, Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.Cancel(p.Context, stringArg(p.Args, "id"))
		}},
		"pinAgentSession": &graphql.Field{Type: agentSessionGQLType, Args: idArg, Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.Pin(p.Context, stringArg(p.Args, "id"))
		}},
		"unpinAgentSession": &graphql.Field{Type: agentSessionGQLType, Args: idArg, Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.Unpin(p.Context, stringArg(p.Args, "id"))
		}},
	}
}
