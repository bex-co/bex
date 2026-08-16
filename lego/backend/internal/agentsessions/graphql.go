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
		"agent":         gqlutil.ReqStrField(func(v AgentConfig) any { return v.Agent }),
		"model":         gqlutil.StrField(func(v AgentConfig) any { return v.Model }),
		"modelEndpoint": gqlutil.StrField(func(v AgentConfig) any { return v.ModelEndpoint }),
		"task":          gqlutil.ReqStrField(func(v AgentConfig) any { return v.Task }),
		"template":      gqlutil.StrField(func(v AgentConfig) any { return v.Template }),
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
		"id":    gqlutil.ReqStrField(func(v AgentProfile) any { return v.ID }),
		"label": gqlutil.ReqStrField(func(v AgentProfile) any { return v.Label }),
	},
})

var githubReadinessGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionGitHubReadiness",
	Fields: graphql.Fields{
		"connected":    gqlutil.ReqBoolField(func(v GitHubReadinessView) any { return v.Connected }),
		"accountLogin": gqlutil.StrField(func(v GitHubReadinessView) any { return v.AccountLogin }),
		"installUrl":   gqlutil.StrField(func(v GitHubReadinessView) any { return v.InstallURL }),
	},
})

var agentCapabilitiesGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionCapabilities",
	Fields: graphql.Fields{
		"enabled":       gqlutil.ReqBoolField(func(v Capabilities) any { return v.Enabled }),
		"github":        gqlutil.Typed(graphql.NewNonNull(githubReadinessGQLType), func(v Capabilities) any { return v.GitHub }),
		"modelKeyReady": gqlutil.ReqBoolField(func(v Capabilities) any { return v.ModelKeyReady }),
		"agents":        gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentProfileGQLType))), func(v Capabilities) any { return v.Agents }),
		"ready":         gqlutil.ReqBoolField(func(v Capabilities) any { return v.Ready }),
	},
})

var evidenceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionEvidence",
	Fields: graphql.Fields{
		"commandLog":   gqlutil.Typed(graphql.NewList(graphql.NewNonNull(graphql.String)), func(v Evidence) any { return v.CommandLog }),
		"testOutput":   gqlutil.Typed(graphql.NewList(graphql.NewNonNull(graphql.String)), func(v Evidence) any { return v.TestOutput }),
		"outputTail":   gqlutil.StrField(func(v Evidence) any { return v.OutputTail }),
		"changedFiles": gqlutil.Typed(graphql.NewList(graphql.NewNonNull(graphql.String)), func(v Evidence) any { return v.ChangedFiles }),
		"commits":      gqlutil.IntField(func(v Evidence) any { return v.Commits }),
		"truncated":    gqlutil.BoolField(func(v Evidence) any { return v.Truncated }),
	},
})

var agentSessionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSession",
	Fields: graphql.Fields{
		"id":          gqlutil.ReqStrField(func(v View) any { return v.ID }),
		"ownerId":     gqlutil.ReqStrField(func(v View) any { return v.OwnerID }),
		"repo":        gqlutil.ReqStrField(func(v View) any { return v.Repo }),
		"branch":      gqlutil.ReqStrField(func(v View) any { return v.Branch }),
		"agentConfig": gqlutil.Typed(graphql.NewNonNull(agentConfigGQLType), func(v View) any { return v.AgentConfig }),
		"sandboxId":   gqlutil.StrField(func(v View) any { return v.SandboxID }),
		"sshAddress":  gqlutil.StrField(func(v View) any { return v.SSHAddress }),
		"phase":       gqlutil.ReqStrField(func(v View) any { return v.Phase }),
		"status":      gqlutil.ReqStrField(func(v View) any { return v.Status }),
		"headSha":     gqlutil.StrField(func(v View) any { return v.HeadSHA }),
		"prUrl":       gqlutil.StrField(func(v View) any { return v.PRURL }),
		"prNumber":    gqlutil.IntField(func(v View) any { return v.PRNumber }),
		"evidence": &graphql.Field{Type: evidenceGQLType, Resolve: gqlutil.Field(func(v View) any {
			if v.Evidence == nil {
				return nil
			}
			return *v.Evidence
		})},
		"turns":         gqlutil.IntField(func(v View) any { return v.Turns }),
		"deliveryMode":  gqlutil.StrField(func(v View) any { return v.DeliveryMode }),
		"failureReason": gqlutil.StrField(func(v View) any { return v.FailureReason }),
		"createdAt":     gqlutil.ReqStrField(func(v View) any { return gqlTime(v.CreatedAt) }),
		"updatedAt":     gqlutil.ReqStrField(func(v View) any { return gqlTime(v.UpdatedAt) }),
		"canceledAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any {
			if v.CanceledAt == nil {
				return nil
			}
			return gqlTime(*v.CanceledAt)
		})},
		"ticket": gqlutil.StrField(func(v View) any { return v.Ticket }),
		"url":    gqlutil.StrField(func(v View) any { return v.URL }),
		"expiresAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any {
			if v.ExpiresAt == nil {
				return nil
			}
			return gqlTime(*v.ExpiresAt)
		})},
		"pinned":        gqlutil.BoolField(func(v View) any { return v.Pinned }),
		"snapshotBytes": gqlutil.FloatField(func(v View) any { return v.SnapshotBytes }),
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

func configArg(args map[string]any) AgentConfig {
	raw, _ := args["agentConfig"].(map[string]any)
	return AgentConfig{Agent: gqlutil.Str(raw, "agent"), Model: gqlutil.Str(raw, "model"), ModelEndpoint: gqlutil.Str(raw, "modelEndpoint"), Task: gqlutil.Str(raw, "task"), Template: gqlutil.Str(raw, "template")}
}

func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"agentSessions": &graphql.Field{
			Type:    graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentSessionGQLType))),
			Args:    graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.List(p.Context, gqlutil.Str(p.Args, "ownerId")) },
		},
		"agentSession": gqlutil.IDVerb(agentSessionGQLType, s.Get),
		"agentSessionCapabilities": &graphql.Field{
			Type: graphql.NewNonNull(agentCapabilitiesGQLType),
			Args: graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Capabilities(p.Context, gqlutil.Str(p.Args, "ownerId"))
			},
		},
	}
}

func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createAgentSession": &graphql.Field{
			Type: agentSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId":         gqlutil.Arg(graphql.String),
				"repo":            gqlutil.ReqArg(graphql.String),
				"branch":          gqlutil.ReqArg(graphql.String),
				"agentConfig":     gqlutil.ReqArg(agentConfigGQLInput),
				"egressAllowlist": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Create(p.Context, CreateRequest{OwnerID: gqlutil.Str(p.Args, "ownerId"), Repo: gqlutil.Str(p.Args, "repo"), Branch: gqlutil.Str(p.Args, "branch"), AgentConfig: configArg(p.Args), EgressAllowlist: gqlutil.StringList(p.Args["egressAllowlist"])})
			},
		},
		"steerAgentSession": &graphql.Field{
			Type: agentSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"id":              gqlutil.ReqArg(graphql.String),
				"prompt":          gqlutil.ReqArg(graphql.String),
				"egressAllowlist": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Steer(p.Context, SteerRequest{SessionID: gqlutil.Str(p.Args, "id"), Prompt: gqlutil.Str(p.Args, "prompt"), EgressAllowlist: gqlutil.StringList(p.Args["egressAllowlist"])})
			},
		},
		"resumeAgentSession": gqlutil.IDVerb(agentSessionGQLType, s.Resume),
		"attachAgentSession": &graphql.Field{
			Type: agentSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"id": gqlutil.ReqArg(graphql.String),
				"action": &graphql.ArgumentConfig{
					Type:        graphql.String,
					Description: "Ticket action: 'read' for transcript replay (GET), 'turn' for live prompt execution (POST). Requires can_create for 'turn'. Defaults to 'read'.",
				},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				action := gqlutil.Str(p.Args, "action")
				if action == "" {
					action = agentsessionticket.ActionRead // Default to read for safety
				}
				return s.AttachTicket(p.Context, gqlutil.Str(p.Args, "id"), action)
			},
		},
		"cancelAgentSession": gqlutil.IDVerb(agentSessionGQLType, s.Cancel),
		"pinAgentSession":    gqlutil.IDVerb(agentSessionGQLType, s.Pin),
		"unpinAgentSession":  gqlutil.IDVerb(agentSessionGQLType, s.Unpin),
	}
}
