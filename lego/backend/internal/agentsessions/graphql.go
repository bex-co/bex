package agentsessions

import (
	"encoding/json"
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

var attachmentGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentAttachment",
	Fields: graphql.Fields{
		"id":        gqlutil.ReqStrField(func(v Attachment) any { return v.ID }),
		"sessionId": gqlutil.OptionalStrField(func(v Attachment) any { return v.SessionID }),
		"turn": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(v Attachment) any {
			if v.Turn == 0 {
				return nil
			}
			return v.Turn
		})},
		"direction":   gqlutil.ReqStrField(func(v Attachment) any { return v.Direction }),
		"name":        gqlutil.ReqStrField(func(v Attachment) any { return v.Name }),
		"mediaType":   gqlutil.ReqStrField(func(v Attachment) any { return v.MediaType }),
		"size":        &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Resolve: gqlutil.Field(func(v Attachment) any { return float64(v.Size) })},
		"sha256":      gqlutil.ReqStrField(func(v Attachment) any { return v.SHA256 }),
		"downloadUrl": gqlutil.ReqStrField(func(v Attachment) any { return v.DownloadURL }),
		"createdAt":   &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v Attachment) any { return gqlTime(v.CreatedAt) })},
		"expiresAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v Attachment) any {
			if v.ExpiresAt == nil {
				return nil
			}
			return gqlTime(*v.ExpiresAt)
		})},
	},
})

var turnAttachmentGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionTurnAttachment",
	Fields: graphql.Fields{
		"id":        gqlutil.ReqStrField(func(v TurnAttachment) any { return v.ID }),
		"name":      gqlutil.ReqStrField(func(v TurnAttachment) any { return v.Name }),
		"mediaType": gqlutil.ReqStrField(func(v TurnAttachment) any { return v.MediaType }),
		"size":      &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Resolve: gqlutil.Field(func(v TurnAttachment) any { return float64(v.Size) })},
	},
})

var attachmentLimitsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentAttachmentLimits",
	Fields: graphql.Fields{
		"enabled":         gqlutil.ReqBoolField(func(v AttachmentLimits) any { return v.Enabled }),
		"maxBytes":        &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Resolve: gqlutil.Field(func(v AttachmentLimits) any { return float64(v.MaxBytes) })},
		"maxPerTurn":      &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v AttachmentLimits) any { return v.MaxPerTurn })},
		"maxPerSession":   &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v AttachmentLimits) any { return v.MaxPerSession })},
		"maxSessionBytes": &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Resolve: gqlutil.Field(func(v AttachmentLimits) any { return float64(v.MaxSessionBytes) })},
	},
})

var agentProfileGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionProfile",
	Fields: graphql.Fields{
		"id":            gqlutil.ReqStrField(func(v AgentProfile) any { return v.ID }),
		"label":         gqlutil.ReqStrField(func(v AgentProfile) any { return v.Label }),
		"modelKeyReady": gqlutil.ReqBoolField(func(v AgentProfile) any { return v.ModelKeyReady }),
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
		"attachments":   &graphql.Field{Type: graphql.NewNonNull(attachmentLimitsGQLType), Resolve: gqlutil.Field(func(v Capabilities) any { return v.Attachments })},
		"agents":        gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentProfileGQLType))), func(v Capabilities) any { return v.Agents }),
		"ready":         gqlutil.ReqBoolField(func(v Capabilities) any { return v.Ready }),
	},
})

var agentNetworkTierGQLEnum = graphql.NewEnum(graphql.EnumConfig{
	Name:        "AgentNetworkTier",
	Description: "Sandbox network access for a workspace's agent sessions. FULL reaches any public host over HTTP(S) through the bex egress proxy (model providers stay blocked; no raw TCP or SSH).",
	Values: graphql.EnumValueConfigMap{
		"REGISTRIES": &graphql.EnumValueConfig{Value: NetworkTierRegistries},
		"FULL":       &graphql.EnumValueConfig{Value: NetworkTierFull},
		"CUSTOM":     &graphql.EnumValueConfig{Value: NetworkTierCustom},
	},
})

// agentModelSourceGQLEnum is the model source, upper-cased like
// AgentNetworkTier.
var agentModelSourceGQLEnum = graphql.NewEnum(graphql.EnumConfig{
	Name:        "AgentModelSource",
	Description: "The model credential a workspace's claude sessions use: BYO (the workspace's own key), PLATFORM (bex-provided, billed at the model provider's cost + 30%) or NONE.",
	Values: graphql.EnumValueConfigMap{
		"BYO":      &graphql.EnumValueConfig{Value: ModelSourceBYO},
		"PLATFORM": &graphql.EnumValueConfig{Value: ModelSourcePlatform},
		"NONE":     &graphql.EnumValueConfig{Value: ModelSourceNone},
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
		"modelKeySet":   gqlutil.ReqBoolField(func(v Environment) any { return v.ModelKeySet }),
		"modelSource":   gqlutil.Typed(graphql.NewNonNull(agentModelSourceGQLEnum), func(v Environment) any { return v.ModelSource }),
		"registryHosts": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), func(v Environment) any { return v.RegistryHosts }),
		"updatedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v Environment) any {
			if v.UpdatedAt == nil {
				return nil
			}
			return gqlTime(*v.UpdatedAt)
		})},
		"agentModelMonthlySpendCapUsd":  optionalFloatField("Effective monthly agent-model spend cap in USD: min(the admin override or the plan default, the trust tier's ceiling).", func(v Environment) *float64 { return v.AgentModelMonthlySpendCapUSD }),
		"agentModelPlanSpendCapUsd":     optionalFloatField("The plan's default monthly agent-model spend cap in USD.", func(v Environment) *float64 { return v.AgentModelPlanSpendCapUSD }),
		"agentModelSpendCapOverrideUsd": optionalFloatField("The admin's spend cap override in USD; null means the plan default.", func(v Environment) *float64 { return v.AgentModelSpendCapOverrideUSD }),
		"agentModelMaxSpendCapUsd":      optionalFloatField("The highest override an admin may set (10x the plan default).", func(v Environment) *float64 { return v.AgentModelMaxSpendCapUSD }),
		"agentModelSpendMonthToDateUsd": optionalFloatField("Month-to-date agent-model spend in USD (UTC calendar month).", func(v Environment) *float64 { return v.AgentModelSpendMonthToDateUSD }),
		"trustTier":                     &graphql.Field{Type: graphql.String, Description: "The workspace's billing trust tier (T0 new .. T4).", Resolve: gqlutil.Field(func(v Environment) any { return optionalString(v.TrustTier) })},
		"nextChargeThresholdUsd":        optionalFloatField("The agent-model spend at which the card is charged mid-cycle; null at the top tier (month-end only).", func(v Environment) *float64 { return v.NextChargeThresholdUSD }),
		"trustTierLocked":               optionalBoolField("A failed or disputed payment holds the workspace at the first trust tier.", func(v Environment) *bool { return v.TrustTierLocked }),
		"agentModelSpendCapTierLimited": optionalBoolField("The trust tier's ceiling, not the plan default or override, sets the effective spend cap.", func(v Environment) *bool { return v.AgentModelSpendCapTierLimited }),
		"agentModelSpendCapReached":     optionalBoolField("Platform-model requests are refused at the spend cap now. False while the next request would charge the card at the trust tier's threshold instead.", func(v Environment) *bool { return v.AgentModelSpendCapReached }),
		"agentModelChargesAtThreshold":  optionalBoolField("Reaching nextChargeThresholdUsd charges the workspace's card and it keeps working. False where bex cannot charge it (no subscription, excluded or comped), on its own model key, or under a payment hold.", func(v Environment) *bool { return v.AgentModelChargesAtThreshold }),
		"billingBalanceDue":             optionalBoolField("A failed invoice is unpaid, so platform-model requests are refused with BILLING_BALANCE_DUE until it is paid.", func(v Environment) *bool { return v.BillingBalanceDue }),
	},
})

func optionalBoolField(description string, get func(Environment) *bool) *graphql.Field {
	return &graphql.Field{Type: graphql.Boolean, Description: description, Resolve: gqlutil.Field(func(v Environment) any {
		if b := get(v); b != nil {
			return *b
		}
		return nil
	})}
}

func optionalFloatField(description string, get func(Environment) *float64) *graphql.Field {
	return &graphql.Field{Type: graphql.Float, Description: description, Resolve: gqlutil.Field(func(v Environment) any {
		if f := get(v); f != nil {
			return *f
		}
		return nil
	})}
}

var agentEnvironmentGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "AgentEnvironmentInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"networkTier":                    &graphql.InputObjectFieldConfig{Type: agentNetworkTierGQLEnum},
		"customHosts":                    &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: "Replaces the whole list; [] clears it. Opened only by the CUSTOM tier."},
		"defaultModel":                   &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "\"\" returns to the provider default."},
		spendCapField:                    &graphql.InputObjectFieldConfig{Type: graphql.Float, Description: "Monthly agent-model spend cap in USD (more than 0, at most 10x the plan default)."},
		"resetAgentModelMonthlySpendCap": &graphql.InputObjectFieldConfig{Type: graphql.Boolean, Description: "true returns the spend cap to the plan default."},
		"modelApiKey":                    &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Write-only: sets or rotates the workspace's own model provider key. Never returned; read modelKeySet."},
		"clearModelApiKey":               &graphql.InputObjectFieldConfig{Type: graphql.Boolean, Description: "true removes the workspace's own model provider key."},
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
	input.ModelAPIKey = gqlutil.StrPtr(raw, "modelApiKey")
	if clear := gqlutil.BoolPtr(raw, "clearModelApiKey"); clear != nil {
		input.ClearModelAPIKey = *clear
	}
	if reset, ok := raw["resetAgentModelMonthlySpendCap"].(bool); ok {
		input.ResetAgentModelMonthlySpendCap = reset
	}
	if v, present := raw[spendCapField]; present {
		input.AgentModelMonthlySpendCapSet = true
		if n, ok := v.(float64); ok { // graphql.Float coerces every literal to float64
			input.AgentModelMonthlySpendCapUSD = &n
		}
	}
	return input
}

var agentSessionPullRequestGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionPullRequest",
	Fields: graphql.Fields{
		"url":     gqlutil.ReqStrField(func(v PullRequest) any { return v.URL }),
		"number":  gqlutil.IntField(func(v PullRequest) any { return v.Number }),
		"headSha": gqlutil.StrField(func(v PullRequest) any { return v.HeadSHA }),
	},
})

var agentSessionCostGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentSessionCost",
	Description: "What the session has cost so far, in USD decimal strings: its platform-route model requests and its attributed sandbox compute (ADR094 D7).",
	Fields: graphql.Fields{
		"modelUSD":   gqlutil.ReqStrField(func(v Cost) any { return v.ModelUSD }),
		"computeUSD": gqlutil.ReqStrField(func(v Cost) any { return v.ComputeUSD }),
		"totalUSD":   gqlutil.ReqStrField(func(v Cost) any { return v.TotalUSD }),
	},
})

var agentSessionOutcomeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionOutcome",
	Fields: graphql.Fields{
		"state":     gqlutil.ReqStrField(func(v Outcome) any { return v.State }),
		"steerable": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v Outcome) any { return v.Steerable })},
		"turn":      gqlutil.IntField(func(v Outcome) any { return v.Turn }),
		"summary":   gqlutil.OptionalStrField(func(v Outcome) any { return v.Summary }),
		"pullRequest": &graphql.Field{Type: agentSessionPullRequestGQLType, Resolve: gqlutil.Field(func(v Outcome) any {
			if v.PullRequest == nil {
				return nil
			}
			return *v.PullRequest
		})},
		"deliveryNote":      gqlutil.OptionalStrField(func(v Outcome) any { return v.DeliveryNote }),
		"failureReasonCode": gqlutil.OptionalStrField(func(v Outcome) any { return v.FailureReasonCode }),
		"files": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(turnAttachmentGQLType)), Resolve: gqlutil.Field(func(v Outcome) any {
			if v.Files == nil {
				return nil
			}
			return v.Files
		})},
		"result": jsonTextField("The current turn's last valid final structured result (ADR094 D5) as JSON text, or null.", func(v Outcome) any { return v.Result }),
		"cost": &graphql.Field{Type: agentSessionCostGQLType, Resolve: gqlutil.Field(func(v Outcome) any {
			if v.Cost == nil {
				return nil
			}
			return *v.Cost
		})},
	},
})

// floatArg is an optional Float argument; nil when it is absent or null.
func floatArg(args map[string]any, key string) *float64 {
	if v, ok := args[key].(float64); ok { // graphql.Float coerces every literal to float64
		return &v
	}
	return nil
}

// jsonTextField is a nullable String field carrying a JSON value as text, the
// way the transcript part is (GraphQL has no JSON scalar here). Null when the
// value is absent.
func jsonTextField[T any](description string, value func(T) any) *graphql.Field {
	return &graphql.Field{Type: graphql.String, Description: description, Resolve: gqlutil.Field(func(v T) any {
		raw := value(v)
		if raw == nil {
			return nil
		}
		text, err := json.Marshal(raw)
		if err != nil {
			return nil
		}
		return string(text)
	})}
}

var agentSessionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSession",
	Fields: graphql.Fields{
		"id":              gqlutil.ReqStrField(func(v View) any { return v.ID }),
		"ownerId":         gqlutil.ReqStrField(func(v View) any { return v.OwnerID }),
		"repo":            gqlutil.ReqStrField(func(v View) any { return v.Repo }),
		"branch":          gqlutil.ReqStrField(func(v View) any { return v.Branch }),
		"title":           gqlutil.StrField(func(v View) any { return v.Title }),
		"maxSpendUSD":     gqlutil.StrField(func(v View) any { return v.MaxSpendUSD }),
		"tags":            &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Resolve: gqlutil.Field(func(v View) any { return v.Tags })},
		"secretNames":     &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: "Names of the secrets attached to the session; values are never returned.", Resolve: gqlutil.Field(func(v View) any { return v.SecretNames })},
		"automationId":    &graphql.Field{Type: graphql.String, Description: "The agent automation (aut-…) that started this session, when one did.", Resolve: gqlutil.Field(func(v View) any { return v.AutomationID })},
		"createdBy":       &graphql.Field{Type: graphql.String, Description: "Who started the session: a member's identity id, or an API key's client id. An automation's sessions carry the automation's creator and a child session its parent's creator. Absent on sessions from before it was recorded.", Resolve: gqlutil.Field(func(v View) any { return v.CreatedBy })},
		"parentSessionId": &graphql.Field{Type: graphql.String, Description: "The agent session (ags-…) that spawned this one, when one did.", Resolve: gqlutil.Field(func(v View) any { return v.ParentSessionID })},
		"rootSessionId":   &graphql.Field{Type: graphql.String, Description: "The top of this session's tree, absent on a top-level session.", Resolve: gqlutil.Field(func(v View) any { return v.RootSessionID })},
		"depth":           &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Description: "Distance from the root of the session's tree; 0 for a top-level session.", Resolve: gqlutil.Field(func(v View) any { return v.Depth })},
		"agentConfig":     gqlutil.Typed(graphql.NewNonNull(agentConfigGQLType), func(v View) any { return v.AgentConfig }),
		"sandboxId":       gqlutil.StrField(func(v View) any { return v.SandboxID }),
		"sshAddress":      gqlutil.StrField(func(v View) any { return v.SSHAddress }),
		"phase":           gqlutil.ReqStrField(func(v View) any { return v.Phase }),
		"state":           gqlutil.ReqStrField(func(v View) any { return v.State }),
		"steerable":       &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v View) any { return v.Steerable })},
		"status":          gqlutil.ReqStrField(func(v View) any { return v.Status }),
		"headSha":         gqlutil.StrField(func(v View) any { return v.HeadSHA }),
		"prUrl":           gqlutil.StrField(func(v View) any { return v.PRURL }),
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
		// created: true for a new session, false for an idempotent replay; null
		// on every read (ADR094 D4).
		"created": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v View) any {
			if v.Created == nil {
				return nil
			}
			return *v.Created
		})},
		"outcome": &graphql.Field{Type: agentSessionOutcomeGQLType, Resolve: gqlutil.Field(func(v View) any {
			if v.Outcome == nil {
				return nil
			}
			return *v.Outcome
		})},
		"archivedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v View) any {
			if v.ArchivedAt == nil {
				return nil
			}
			return gqlTime(*v.ArchivedAt)
		})},
		"turnOrigins": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(agentSessionTurnOriginGQLType)),
			Description: "Which turns PR feedback started (a review, a comment or a failed CI run); only on a single-session read.",
			Resolve:     gqlutil.Field(func(v View) any { return v.TurnOrigins })},
	},
})

var agentSessionTurnOriginGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionTurnOrigin",
	Fields: graphql.Fields{
		"turn":   gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(v TurnOrigin) any { return v.Turn }),
		"kind":   gqlutil.ReqStrField(func(v TurnOrigin) any { return v.Kind }),
		"author": gqlutil.OptionalStrField(func(v TurnOrigin) any { return v.Author }),
		"url":    gqlutil.OptionalStrField(func(v TurnOrigin) any { return v.URL }),
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
		"attachments": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(turnAttachmentGQLType))),
			Description: "The files this turn attached (ADR095), as recorded when it was accepted.",
			Resolve:     gqlutil.Field(func(v TranscriptTurn) any { return v.Attachments })},
		"skills": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentSessionSkillGQLType))),
			Description: "The workspace skills this turn invoked (ADR099).",
			Resolve:     gqlutil.Field(func(v TranscriptTurn) any { return v.Skills })},
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
		"outputSchema":   jsonTextField("The JSON Schema the turn was accepted with (ADR094 D5), as JSON text, or null for a free-form turn.", func(v TranscriptTurn) any { return v.OutputSchema }),
		"outputRequired": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.OutputRequired })},
		"result":         jsonTextField("The agent's last valid structured result for the turn as JSON text, readable while the turn runs, or null.", func(v TranscriptTurn) any { return v.Result }),
		"resultFinal":    &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Description: "The agent declared result final; only a final result becomes outcome.result.", Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.ResultFinal })},
	},
})

var turnModelCostGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentSessionTurnModelCost",
	Description: "One turn's platform-route model requests (ADR091 D6). Token counts are display only.",
	Fields: graphql.Fields{
		"turn":     &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v TurnModelCost) any { return v.Turn })},
		"costUsd":  gqlutil.ReqStrField(func(v TurnModelCost) any { return v.CostUSD }),
		"requests": &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v TurnModelCost) any { return v.Requests })},
		// Float, not Int: a long turn's token count can exceed 32 bits.
		"inputTokens":      &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Resolve: gqlutil.Field(func(v TurnModelCost) any { return v.InputTokens })},
		"outputTokens":     &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Resolve: gqlutil.Field(func(v TurnModelCost) any { return v.OutputTokens })},
		"cacheReadTokens":  &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Resolve: gqlutil.Field(func(v TurnModelCost) any { return v.CacheReadTokens })},
		"cacheWriteTokens": &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Resolve: gqlutil.Field(func(v TurnModelCost) any { return v.CacheWriteTokens })},
		"models":           &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), Resolve: gqlutil.Field(func(v TurnModelCost) any { return v.Models })},
		"firstAt":          gqlutil.TimeField(func(v TurnModelCost) any { return v.FirstAt }),
		"lastAt":           gqlutil.TimeField(func(v TurnModelCost) any { return v.LastAt }),
	},
})

var agentSessionTreeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentSessionTree",
	Description: "The tree of agent sessions a session belongs to (ADR102): its root and every session below it.",
	Fields: graphql.Fields{
		"rootSessionId": gqlutil.ReqStrField(func(v Tree) any { return v.RootSessionID }),
		"sessions":      &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentSessionGQLType))), Description: "The root first, then by depth and creation time.", Resolve: gqlutil.Field(func(v Tree) any { return v.Sessions })},
	},
})

var modelCostGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentSessionModelCost",
	Description: "What a session's platform-route model requests cost (provider cost + 30%), in total and per turn (ADR091 D6). A session on the workspace's own model key records none.",
	Fields: graphql.Fields{
		"sessionId":   gqlutil.ReqStrField(func(v ModelCost) any { return v.SessionID }),
		"modelSource": &graphql.Field{Type: graphql.NewNonNull(agentModelSourceGQLEnum), Description: "The model route the session runs on: PLATFORM (bex bills its requests) or BYO (the workspace's own provider bills them, so cost stays 0.00).", Resolve: gqlutil.Field(func(v ModelCost) any { return v.ModelSource })},
		"costUsd":     &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: "The session's charge so far in USD, rounded to the cent (\"1.56\"), the same format as usage.agentModel.", Resolve: gqlutil.Field(func(v ModelCost) any { return v.CostUSD })},
		"requests":    &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v ModelCost) any { return v.Requests })},
		"turns":       &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(turnModelCostGQLType))), Resolve: gqlutil.Field(func(v ModelCost) any { return v.Turns })},
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

var agentSessionSecretGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name:        "AgentSessionSecretInput",
	Description: "An inline secret for one agent session (ADR098): the agent reads it as the env var name. Write-only.",
	Fields: graphql.InputObjectConfigFieldMap{
		"name":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"value": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String), Description: "1 B to 32 KiB; never returned, redacted from the transcript."},
	},
})

var agentSecretGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentSecret",
	Description: "A workspace agent secret. Only its name is readable.",
	Fields: graphql.Fields{
		"name": gqlutil.ReqStrField(func(v AgentSecret) any { return v.Name }),
	},
})

var agentSecretListGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSecretList",
	Fields: graphql.Fields{
		"ownerId": gqlutil.ReqStrField(func(v AgentSecretList) any { return v.OwnerID }),
		"secrets": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentSecretGQLType))), func(v AgentSecretList) any { return v.Secrets }),
	},
})

// secretsArg reads the ADR098 attach arguments shared by create and steer.
func secretsArg(args map[string]any) SessionSecretsInput {
	out := SessionSecretsInput{SecretNames: gqlutil.StringList(args["secretNames"])}
	raw, _ := args["secrets"].([]any)
	for _, item := range raw {
		m, _ := item.(map[string]any)
		out.Secrets = append(out.Secrets, SecretInput{Name: gqlutil.Str(m, "name"), Value: gqlutil.Str(m, "value")})
	}
	return out
}

func configArg(args map[string]any) AgentConfig {
	raw, _ := args["agentConfig"].(map[string]any)
	return AgentConfig{Agent: gqlutil.Str(raw, "agent"), Model: gqlutil.Str(raw, "model"), ModelEndpoint: gqlutil.Str(raw, "modelEndpoint"), Task: gqlutil.Str(raw, "task"), Template: gqlutil.Str(raw, "template")}
}

func (s *Service) GraphQLQuery() graphql.Fields {
	fields := graphql.Fields{
		"agentSessions": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentSessionGQLType))),
			Args: graphql.FieldConfigArgument{
				"ownerId":         gqlutil.Arg(graphql.String),
				"archived":        &graphql.ArgumentConfig{Type: graphql.String, Description: "Archive membership: false (default, the working set), true (archived only), all."},
				"phases":          gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"states":          gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"tag":             gqlutil.Arg(graphql.String),
				"repo":            gqlutil.Arg(graphql.String),
				"automationId":    &graphql.ArgumentConfig{Type: graphql.String, Description: "Only the sessions this agent automation (aut-…) started."},
				"parentSessionId": &graphql.ArgumentConfig{Type: graphql.String, Description: "Only the direct children of this session (ags-…)."},
				"mine":            &graphql.ArgumentConfig{Type: graphql.Boolean, Description: "Only the sessions you started."},
				"createdBy":       &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: "Only the sessions these subjects started (at most 20)."},
				"createdBefore":   gqlutil.Arg(graphql.String),
				"createdAfter":    gqlutil.Arg(graphql.String),
				"cursor":          &graphql.ArgumentConfig{Type: graphql.String, Description: "The prior page's last session id; a shorter/empty page signals the end."},
				"limit":           gqlutil.Arg(graphql.Int),
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
					OwnerID:         gqlutil.Str(p.Args, "ownerId"),
					Archived:        gqlutil.Str(p.Args, "archived"),
					Phases:          gqlutil.StringList(p.Args["phases"]),
					States:          gqlutil.StringList(p.Args["states"]),
					Tag:             gqlutil.Str(p.Args, "tag"),
					Repo:            gqlutil.Str(p.Args, "repo"),
					AutomationID:    gqlutil.Str(p.Args, "automationId"),
					ParentSessionID: gqlutil.Str(p.Args, "parentSessionId"),
					Mine:            gqlutil.Bool(p.Args, "mine"),
					CreatedBy:       gqlutil.StringList(p.Args["createdBy"]),
					CreatedBefore:   before,
					CreatedAfter:    after,
					Cursor:          gqlutil.Str(p.Args, "cursor"),
					Limit:           limit,
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
				"id":         gqlutil.ReqArg(graphql.String),
				"turn":       gqlutil.ReqArg(graphql.Int),
				"toolCallId": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				turn, _ := p.Args["turn"].(int)
				return s.ToolOutput(p.Context, gqlutil.Str(p.Args, "id"), turn, gqlutil.Str(p.Args, "toolCallId"))
			},
		},
		"agentSession": gqlutil.IDVerb(agentSessionGQLType, s.Get),
		// agentSessionTree is the tree the session belongs to, any node's id (ADR102 D8).
		"agentSessionTree": gqlutil.IDVerb(graphql.NewNonNull(agentSessionTreeGQLType), s.Tree),
		// agentSessionModelCost is the session's platform-route model cost in
		// total and per turn (ADR091 D6).
		"agentSessionModelCost": gqlutil.IDVerb(graphql.NewNonNull(modelCostGQLType), s.ModelCost),
		"agentAttachment": &graphql.Field{
			Type:        attachmentGQLType,
			Description: "One agent attachment with a fresh 15-minute downloadUrl (ADR095).",
			Args:        graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetAttachment(p.Context, gqlutil.Str(p.Args, "id"))
			},
		},
		"agentSessionAttachments": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(attachmentGQLType))),
			Description: "A session's attachments oldest first; cursor is the previous page's last id.",
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{
				"id":        gqlutil.ReqArg(graphql.String),
				"direction": gqlutil.Arg(graphql.String),
				"turn":      gqlutil.Arg(graphql.Int),
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				limit, err := gqlutil.PositiveLimit(p.Args)
				if err != nil {
					return nil, err
				}
				var turn *int
				if _, ok := p.Args["turn"]; ok {
					n := gqlutil.Int(p.Args, "turn")
					turn = &n
				}
				return s.ListAttachments(p.Context, gqlutil.Str(p.Args, "id"), AttachmentListQuery{
					Direction: gqlutil.Str(p.Args, "direction"), Turn: turn, Cursor: gqlutil.Str(p.Args, "cursor"), Limit: limit,
				})
			},
		},
		"agentEnvironment": &graphql.Field{
			Type: graphql.NewNonNull(agentEnvironmentGQLType),
			Args: graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Environment(p.Context, gqlutil.Str(p.Args, "ownerId"))
			},
		},
		"agentSecrets": &graphql.Field{
			Type:        graphql.NewNonNull(agentSecretListGQLType),
			Description: "The workspace's agent secrets, names only.",
			Args:        graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.AgentSecrets(p.Context, gqlutil.Str(p.Args, "ownerId"))
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
	maps.Copy(fields, s.skillQueries())
	maps.Copy(fields, s.automationQueries())
	return fields
}

func (s *Service) GraphQLMutation() graphql.Fields {
	fields := graphql.Fields{
		"createAgentSession": &graphql.Field{
			Type: agentSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId":         gqlutil.Arg(graphql.String),
				"repo":            gqlutil.ReqArg(graphql.String),
				"branch":          gqlutil.Arg(graphql.String),
				"title":           gqlutil.Arg(graphql.String),
				"tags":            gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"agentConfig":     gqlutil.ReqArg(agentConfigGQLInput),
				"egressAllowlist": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"idempotencyKey":  gqlutil.Arg(graphql.String),
				"attachmentIds":   gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"outputSchema":    gqlutil.Arg(graphql.String),
				"outputRequired":  gqlutil.Arg(graphql.Boolean),
				"maxSpendUSD":     gqlutil.Arg(graphql.Float),
				"secretNames":     gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"secrets":         gqlutil.Arg(graphql.NewList(graphql.NewNonNull(agentSessionSecretGQLInput))),
				"skills":          gqlutil.Arg(graphql.NewList(graphql.NewNonNull(agentSessionSkillGQLInput))),
				"parentSessionId": &graphql.ArgumentConfig{Type: graphql.String, Description: "Start the session as a child of this active session (ags-…): at most 3 levels deep and 5 live children per parent; its spend counts against the parent's ceiling; it inherits the parent's secrets (secretNames may only narrow them)."},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				schema, required, err := outputArgs(p.Args)
				if err != nil {
					return nil, err
				}
				return s.Create(p.Context, CreateRequest{MaxSpendUSD: floatArg(p.Args, "maxSpendUSD"), OutputSchema: schema, OutputRequired: required, OwnerID: gqlutil.Str(p.Args, "ownerId"), Repo: gqlutil.Str(p.Args, "repo"), Branch: gqlutil.Str(p.Args, "branch"), AgentConfig: configArg(p.Args), EgressAllowlist: gqlutil.StringList(p.Args["egressAllowlist"]), IdempotencyKey: gqlutil.Str(p.Args, "idempotencyKey"), Title: gqlutil.Str(p.Args, "title"), Tags: gqlutil.StringList(p.Args["tags"]), AttachmentIDs: gqlutil.StringList(p.Args["attachmentIds"]), SessionSecretsInput: secretsArg(p.Args), Skills: skillsArg(p.Args), ParentSessionID: gqlutil.Str(p.Args, "parentSessionId")})
			},
		},
		"steerAgentSession": &graphql.Field{
			Type: agentSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"id":              gqlutil.ReqArg(graphql.String),
				"prompt":          gqlutil.ReqArg(graphql.String),
				"egressAllowlist": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"attachmentIds":   gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"outputSchema":    gqlutil.Arg(graphql.String),
				"outputRequired":  gqlutil.Arg(graphql.Boolean),
				"secretNames":     gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"secrets":         gqlutil.Arg(graphql.NewList(graphql.NewNonNull(agentSessionSecretGQLInput))),
				"skills":          gqlutil.Arg(graphql.NewList(graphql.NewNonNull(agentSessionSkillGQLInput))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				schema, required, err := outputArgs(p.Args)
				if err != nil {
					return nil, err
				}
				return s.Steer(p.Context, SteerRequest{OutputSchema: schema, OutputRequired: required, SessionID: gqlutil.Str(p.Args, "id"), Prompt: gqlutil.Str(p.Args, "prompt"), EgressAllowlist: gqlutil.StringList(p.Args["egressAllowlist"]), AttachmentIDs: gqlutil.StringList(p.Args["attachmentIds"]), SessionSecretsInput: secretsArg(p.Args), Skills: skillsArg(p.Args)})
			},
		},
		"setAgentSecret": &graphql.Field{
			Type:        graphql.NewNonNull(agentSecretGQLType),
			Description: "Create or rotate a workspace agent secret (admins only). The value is write-only.",
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"name":    gqlutil.ReqArg(graphql.String),
				"value":   gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetAgentSecret(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "name"), gqlutil.Str(p.Args, "value"))
			},
		},
		"deleteAgentSecret": &graphql.Field{
			Type:        graphql.Boolean,
			Description: "Delete a workspace agent secret (admins only).",
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"name":    gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.DeleteAgentSecret(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "name")); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"updateAgentEnvironment": &graphql.Field{
			Type: graphql.NewNonNull(agentEnvironmentGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
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
		"cancelAgentSession": gqlutil.IDVerb(agentSessionGQLType, s.Cancel),
		"setAgentSessionTags": &graphql.Field{
			Type:        agentSessionGQLType,
			Description: "Replaces all of a session's tags; [] clears them. Refused on an archived session.",
			Args: graphql.FieldConfigArgument{
				"id":   gqlutil.ReqArg(graphql.String),
				"tags": gqlutil.ReqArg(graphql.NewList(graphql.NewNonNull(graphql.String))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ReplaceTags(p.Context, gqlutil.Str(p.Args, "id"), gqlutil.StringList(p.Args["tags"]))
			},
		},
		"archiveAgentSession":   gqlutil.IDVerb(agentSessionGQLType, s.Archive),
		"unarchiveAgentSession": gqlutil.IDVerb(agentSessionGQLType, s.Unarchive),
		"deleteAgentAttachment": &graphql.Field{
			Type:        graphql.Boolean,
			Description: "Deletes your own upload that no turn has attached yet.",
			Args:        graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.DeleteAttachment(p.Context, gqlutil.Str(p.Args, "id")); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
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
	maps.Copy(fields, s.skillMutations())
	maps.Copy(fields, s.automationMutations())
	return fields
}
