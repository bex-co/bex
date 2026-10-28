package agentsessions

import (
	"maps"
	"slices"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
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

var agentNetworkTierGQLEnum = graphql.NewEnum(graphql.EnumConfig{
	Name:        "AgentNetworkTier",
	Description: "Sandbox network access for a workspace's agent sessions. FULL is stored but currently enforced as REGISTRIES.",
	Values: graphql.EnumValueConfigMap{
		"REGISTRIES": &graphql.EnumValueConfig{Value: NetworkTierRegistries},
		"FULL":       &graphql.EnumValueConfig{Value: NetworkTierFull},
		"CUSTOM":     &graphql.EnumValueConfig{Value: NetworkTierCustom},
	},
})

var agentEnvironmentGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentEnvironment",
	Fields: graphql.Fields{
		"ownerId":       gqlutil.ReqStrField(func(v Environment) any { return v.OwnerID }),
		"networkTier":   gqlutil.Typed(graphql.NewNonNull(agentNetworkTierGQLEnum), func(v Environment) any { return v.NetworkTier }),
		"customHosts":   gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), func(v Environment) any { return v.CustomHosts }),
		"defaultModel":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v Environment) any { return optionalString(v.DefaultModel) })},
		"modelPinned":   gqlutil.ReqBoolField(func(v Environment) any { return v.ModelPinned }),
		"pinnedModel":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v Environment) any { return optionalString(v.PinnedModel) })},
		"registryHosts": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), func(v Environment) any { return v.RegistryHosts }),
		"updatedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v Environment) any {
			if v.UpdatedAt == nil {
				return nil
			}
			return gqlTime(*v.UpdatedAt)
		})},
	},
})

var agentEnvironmentGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "AgentEnvironmentInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"networkTier":  &graphql.InputObjectFieldConfig{Type: agentNetworkTierGQLEnum},
		"customHosts":  &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: "Replaces the whole list; [] clears it. Opened only by the CUSTOM tier."},
		"defaultModel": &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "\"\" returns to the provider default."},
	},
})

func optionalString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

// environmentInputArg maps the GraphQL input onto the partial update: an
// omitted or null field keeps its value.
func environmentInputArg(args map[string]any) EnvironmentInput {
	raw, _ := args["input"].(map[string]any)
	var input EnvironmentInput
	if tier, ok := raw["networkTier"].(string); ok {
		input.NetworkTier = &tier
	}
	if hosts, ok := raw["customHosts"].([]any); ok {
		list := gqlutil.StringList(hosts)
		input.CustomHosts = &list
	}
	if model, ok := raw["defaultModel"].(string); ok {
		input.DefaultModel = &model
	}
	return input
}

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
		"prNumber": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(v View) any {
			if v.PRNumber == 0 {
				return nil
			}
			return v.PRNumber
		})},
		"turns":             gqlutil.IntField(func(v View) any { return v.Turns }),
		"deliveryMode":      gqlutil.StrField(func(v View) any { return v.DeliveryMode }),
		"deliveryNote":      gqlutil.OptionalStrField(func(v View) any { return v.DeliveryNote }),
		"failureReason":     gqlutil.StrField(func(v View) any { return v.FailureReason }),
		"failureReasonCode": gqlutil.OptionalStrField(func(v View) any { return v.FailureReasonCode }),
		"createdAt":         gqlutil.ReqStrField(func(v View) any { return gqlTime(v.CreatedAt) }),
		"updatedAt":         gqlutil.ReqStrField(func(v View) any { return gqlTime(v.UpdatedAt) }),
		"canceledAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any {
			if v.CanceledAt == nil {
				return nil
			}
			return gqlTime(*v.CanceledAt)
		})},
		"ticket":    gqlutil.StrField(func(v View) any { return v.Ticket }),
		"url":       gqlutil.StrField(func(v View) any { return v.URL }),
		"streamUrl": gqlutil.StrField(func(v View) any { return v.StreamURL }),
		"expiresAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any {
			if v.ExpiresAt == nil {
				return nil
			}
			return gqlTime(*v.ExpiresAt)
		})},
		"archivedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any {
			if v.ArchivedAt == nil {
				return nil
			}
			return gqlTime(*v.ArchivedAt)
		})},
	},
})

var transcriptPartGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionTranscriptPart",
	Fields: graphql.Fields{
		"seq":       &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v TranscriptPart) any { return v.Seq })},
		"partIndex": &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v TranscriptPart) any { return v.PartIndex })},
		"turn":      &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v TranscriptPart) any { return v.Turn })},
		// The stored payload as JSON text — a completed item (ADR051 item
		// model) or a legacy chunk; clients parse it themselves.
		"part": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v TranscriptPart) any { return string(v.Part) })},
		// The live stream ended inside this item; a later harvest may complete
		// it under a NEW seq with the same (turn, partIndex), superseding it.
		"partial":   &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v TranscriptPart) any { return v.Partial })},
		"createdAt": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v TranscriptPart) any { return gqlTime(v.CreatedAt) })},
	},
})

// toolOutputFieldGQL is one field of a tool call's full output.
type toolOutputFieldGQL struct{ Field, Text string }

var toolOutputFieldGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionToolOutputField",
	Fields: graphql.Fields{
		"field": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: "output, errorText, or terminal:<terminalId>.",
			Resolve: gqlutil.Field(func(v toolOutputFieldGQL) any { return v.Field })},
		"text": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: "The complete text, at most 1 MiB (a longer one ends in a [bex: full output capped at 1 MiB, N chars omitted] marker).",
			Resolve: gqlutil.Field(func(v toolOutputFieldGQL) any { return v.Text })},
	},
})

var toolOutputGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionToolOutput",
	Fields: graphql.Fields{
		"sessionId":  &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v ToolOutput) any { return v.SessionID })},
		"turn":       &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v ToolOutput) any { return v.Turn })},
		"toolCallId": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v ToolOutput) any { return v.ToolCallID })},
		"outputs": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(toolOutputFieldGQLType))),
			Description: "One entry per stored field, sorted by field.",
			Resolve: gqlutil.Field(func(v ToolOutput) any {
				out := make([]toolOutputFieldGQL, 0, len(v.Outputs))
				for _, field := range slices.Sorted(maps.Keys(v.Outputs)) {
					out = append(out, toolOutputFieldGQL{field, v.Outputs[field]})
				}
				return out
			})},
	},
})

var transcriptTurnGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionTranscriptTurn",
	Fields: graphql.Fields{
		"turn":                &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.Turn })},
		"prompt":              &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.Prompt })},
		"deliveryMode":        &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.DeliveryMode })},
		"transcriptComplete":  &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.TranscriptComplete })},
		"transcriptTruncated": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.TranscriptTruncated })},
		"truncationReason":    &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.TruncationReason })},
		"createdAt":           &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v TranscriptTurn) any { return gqlTime(v.CreatedAt) })},
		"completedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v TranscriptTurn) any {
			if v.CompletedAt == nil {
				return nil
			}
			return gqlTime(*v.CompletedAt)
		})},
	},
})

var transcriptPageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionTranscriptPage",
	Fields: graphql.Fields{
		"parts":        &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(transcriptPartGQLType))), Resolve: gqlutil.Field(func(v TranscriptPage) any { return v.Parts })},
		"turns":        &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(transcriptTurnGQLType))), Resolve: gqlutil.Field(func(v TranscriptPage) any { return v.Turns })},
		"nextAfterSeq": &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v TranscriptPage) any { return v.NextAfterSeq })},
	},
})

func configArg(args map[string]any) AgentConfig {
	raw, _ := args["agentConfig"].(map[string]any)
	return AgentConfig{Agent: gqlutil.Str(raw, "agent"), Model: gqlutil.Str(raw, "model"), ModelEndpoint: gqlutil.Str(raw, "modelEndpoint"), Task: gqlutil.Str(raw, "task"), Template: gqlutil.Str(raw, "template")}
}

func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"agentSessions": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentSessionGQLType))),
			Args: graphql.FieldConfigArgument{
				"ownerId":       gqlutil.Arg(graphql.String),
				"archived":      &graphql.ArgumentConfig{Type: graphql.String, Description: "Archive membership: false (default, the working set), true (archived only), all."},
				"phases":        gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"repo":          gqlutil.Arg(graphql.String),
				"createdBefore": gqlutil.Arg(graphql.String),
				"createdAfter":  gqlutil.Arg(graphql.String),
				"cursor":        &graphql.ArgumentConfig{Type: graphql.String, Description: "The prior page's last session id; a shorter/empty page signals the end."},
				"limit":         gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				before, err := core.ParseTime("createdBefore", gqlutil.Str(p.Args, "createdBefore"))
				if err != nil {
					return nil, err
				}
				after, err := core.ParseTime("createdAfter", gqlutil.Str(p.Args, "createdAfter"))
				if err != nil {
					return nil, err
				}
				limit, err := gqlutil.PositiveLimit(p.Args)
				if err != nil {
					return nil, err
				}
				return s.List(p.Context, ListRequest{
					OwnerID:       gqlutil.Str(p.Args, "ownerId"),
					Archived:      gqlutil.Str(p.Args, "archived"),
					Phases:        gqlutil.StringList(p.Args["phases"]),
					Repo:          gqlutil.Str(p.Args, "repo"),
					CreatedBefore: before,
					CreatedAfter:  after,
					Cursor:        gqlutil.Str(p.Args, "cursor"),
					Limit:         limit,
				})
			},
		},
		"agentSessionTranscript": &graphql.Field{
			Type: graphql.NewNonNull(transcriptPageGQLType),
			Args: graphql.FieldConfigArgument{
				"id":       gqlutil.ReqArg(graphql.String),
				"afterSeq": &graphql.ArgumentConfig{Type: graphql.Int, Description: "Resume strictly after this seq; omit (or -1) for the whole transcript."},
				"limit":    gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				afterSeq := int64(-1)
				if n, ok := p.Args["afterSeq"].(int); ok {
					afterSeq = int64(n)
				}
				limit, err := gqlutil.PositiveLimit(p.Args)
				if err != nil {
					return nil, err
				}
				return s.Transcript(p.Context, gqlutil.Str(p.Args, "id"), afterSeq, limit)
			},
		},
		"agentSessionToolOutput": &graphql.Field{
			Type:        graphql.NewNonNull(toolOutputGQLType),
			Description: "The complete output of one tool call whose transcript part is flagged fullOutput (w10/m18). Not found when no full copy was kept.",
			Args: graphql.FieldConfigArgument{
				"sessionId":  gqlutil.ReqArg(graphql.String),
				"turn":       gqlutil.ReqArg(graphql.Int),
				"toolCallId": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				turn, _ := p.Args["turn"].(int)
				return s.ToolOutput(p.Context, gqlutil.Str(p.Args, "sessionId"), turn, gqlutil.Str(p.Args, "toolCallId"))
			},
		},
		"agentSession": gqlutil.IDVerb(agentSessionGQLType, s.Get),
		"agentEnvironment": &graphql.Field{
			Type: graphql.NewNonNull(agentEnvironmentGQLType),
			Args: graphql.FieldConfigArgument{"ownerId": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Environment(p.Context, gqlutil.Str(p.Args, "ownerId"))
			},
		},
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
		"updateAgentEnvironment": &graphql.Field{
			Type: graphql.NewNonNull(agentEnvironmentGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.ReqArg(graphql.String),
				"input":   gqlutil.ReqArg(agentEnvironmentGQLInput),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UpdateEnvironment(p.Context, gqlutil.Str(p.Args, "ownerId"), environmentInputArg(p.Args))
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
		"cancelAgentSession":    gqlutil.IDVerb(agentSessionGQLType, s.Cancel),
		"archiveAgentSession":   gqlutil.IDVerb(agentSessionGQLType, s.Archive),
		"unarchiveAgentSession": gqlutil.IDVerb(agentSessionGQLType, s.Unarchive),
		// Delete returns a bare Boolean (nothing survives to project), so it
		// can't ride the View-shaped IDVerb helper.
		"deleteAgentSession": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.Delete(p.Context, gqlutil.Str(p.Args, "id")); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
	}
}
