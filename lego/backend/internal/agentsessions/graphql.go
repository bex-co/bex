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
		"githubCli": &graphql.Field{Type: graphql.Boolean, Description: "This session's own gh binding: null inherits the workspace's GitHub CLI setting, false turned gh off for it, true turned it on against an inherited off. A child session is governed by its tree's chain and its own binding can only turn gh off.",
			Resolve: gqlutil.Field(func(v AgentConfig) any { return v.GitHubCLI })},
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
		"githubCli":     &graphql.InputObjectFieldConfig{Type: graphql.Boolean, Description: "Omit (or null) to inherit the workspace's GitHub CLI setting. false turns gh off for this session; anyone who can start it may. true turns it on against an inherited off: only a workspace admin may, and nobody may while the workspace's gh off is mandatory (AGENT_SESSION_GITHUB_CLI_LOCKED). A child session can only turn gh off."},
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
		"model": &graphql.Field{Type: graphql.String, Description: "The model a session of this agent runs when it names none: the workspace default while it is offered, else the platform catalog's first model or the agent's fixed one. Null when the provider's own default runs.",
			Resolve: gqlutil.Field(func(v AgentProfile) any { return optionalString(v.Model) })},
		"models": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), Description: "The models a session of this agent may name: on the platform model route the catalog models the workspace's admins left on (enabledModels on the agent environment). Empty where any model the workspace's own provider serves is accepted.",
			Resolve: gqlutil.Field(func(v AgentProfile) any { return v.Models })},
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

var agentSandboxTemplateGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentSandboxTemplate",
	Description: "An agent sandbox template a session can name in agentConfig.template: its CPU and memory.",
	Fields: graphql.Fields{
		"name":     gqlutil.ReqStrField(func(v SandboxTemplate) any { return v.Name }),
		"cpu":      &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Description: "CPU cores.", Resolve: gqlutil.Field(func(v SandboxTemplate) any { return v.CPU })},
		"memoryGb": &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Description: "Memory in GB.", Resolve: gqlutil.Field(func(v SandboxTemplate) any { return v.MemoryGB })},
	},
})

// withTemplateFields adds a TemplateOffer's fields, which the environment
// and the capabilities reads share, to fields.
func withTemplateFields[T any](fields graphql.Fields, offer func(T) TemplateOffer) graphql.Fields {
	fields["defaultTemplate"] = &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: "The sandbox template a session or automation run that names none runs on: the workspace default, else agent.",
		Resolve: gqlutil.Field(func(v T) any { return offer(v).DefaultTemplate })}
	fields["templates"] = &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentSandboxTemplateGQLType))), Description: "The agent sandbox templates a session may name, smallest first.",
		Resolve: gqlutil.Field(func(v T) any { return offer(v).Templates })}
	return fields
}

var agentCapabilitiesGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionCapabilities",
	Fields: withTemplateFields(graphql.Fields{
		"enabled":       gqlutil.ReqBoolField(func(v Capabilities) any { return v.Enabled }),
		"github":        gqlutil.Typed(graphql.NewNonNull(githubReadinessGQLType), func(v Capabilities) any { return v.GitHub }),
		"modelKeyReady": gqlutil.ReqBoolField(func(v Capabilities) any { return v.ModelKeyReady }),
		"attachments":   &graphql.Field{Type: graphql.NewNonNull(attachmentLimitsGQLType), Resolve: gqlutil.Field(func(v Capabilities) any { return v.Attachments })},
		"agents":        gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentProfileGQLType))), func(v Capabilities) any { return v.Agents }),
		"ready":         gqlutil.ReqBoolField(func(v Capabilities) any { return v.Ready }),
	}, func(v Capabilities) TemplateOffer { return v.TemplateOffer }),
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

var agentDataUseGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentDataUse",
	Fields: graphql.Fields{
		"agent": gqlutil.ReqStrField(func(v AgentDataUse) any { return v.Agent }),
		"acknowledgedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v AgentDataUse) any {
			if v.AcknowledgedAt == nil {
				return nil
			}
			return gqlTime(*v.AcknowledgedAt)
		})},
		"acknowledgedBy": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v AgentDataUse) any { return optionalString(v.AcknowledgedBy) })},
	},
})

var agentEnvironmentGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentEnvironment",
	Fields: withTemplateFields(graphql.Fields{
		"ownerId":      gqlutil.ReqStrField(func(v Environment) any { return v.OwnerID }),
		"networkTier":  gqlutil.Typed(graphql.NewNonNull(agentNetworkTierGQLEnum), func(v Environment) any { return v.NetworkTier }),
		"customHosts":  gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), func(v Environment) any { return v.CustomHosts }),
		"defaultModel": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v Environment) any { return optionalString(v.DefaultModel) })},
		"models": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), Description: "The whole platform model catalog, its default first; enabledModels is the part members may choose. Empty on the workspace's own key, where any model id is accepted.",
			Resolve: gqlutil.Field(func(v Environment) any { return v.Models })},
		"enabledModels": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), Description: "The catalog models the workspace's admins left on: the models a claude session, a message, an automation and defaultModel may name, the catalog's default always first. Every catalog model, one added to the catalog later included, until an admin turns one off; from then on it is an allowlist, and a model added to the catalog later is off (newModels) until an admin turns it on. A defaultModel not listed here does not take effect: the catalog's default runs. Empty on the workspace's own key.",
			Resolve: gqlutil.Field(func(v Environment) any { return v.EnabledModels })},
		"newModels": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), Description: "The catalog models off only because they joined the catalog after an admin narrowed the list (the allowlist predates them), apart from the ones an admin turned off; an admin turns one on through enabledModels. Empty while every model is allowed and on the workspace's own key.",
			Resolve: gqlutil.Field(func(v Environment) any { return v.NewModels })},
		"modelPinned": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Description: "True on the platform model route, whose gateway pins each claude session to the catalog model it chose.",
			Resolve: gqlutil.Field(func(v Environment) any { return v.ModelPinned })},
		"pinnedModel": &graphql.Field{Type: graphql.String, Description: "The platform catalog's default model; models lists every choice.",
			Resolve: gqlutil.Field(func(v Environment) any { return optionalString(v.PinnedModel) })},
		"modelKeySet":   gqlutil.ReqBoolField(func(v Environment) any { return v.ModelKeySet }),
		"modelSource":   gqlutil.Typed(graphql.NewNonNull(agentModelSourceGQLEnum), func(v Environment) any { return v.ModelSource }),
		"registryHosts": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), func(v Environment) any { return v.RegistryHosts }),
		"enabledAgents": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), Description: "The agent profiles members can start sessions with, as the workspace's admins chose.", Resolve: gqlutil.Field(func(v Environment) any { return v.EnabledAgents })},
		"agentDataUse":  &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentDataUseGQLType))), Description: "Each agent profile whose model provider may train on what its sessions send, and the admin acknowledgement turning it on requires.", Resolve: gqlutil.Field(func(v Environment) any { return v.AgentDataUse })},
		"githubCliEnabled": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Description: "Whether the workspace's repo sessions get a gh that reaches their repository through bex; true unless an admin turned it off. Off, turns dispatched afterwards get no gh configuration and a running session's next gh call is refused; Git pushes to the session branch and the bex MCP tools keep working.",
			Resolve: gqlutil.Field(func(v Environment) any { return v.GitHubCLIEnabled })},
		"automationsGithubCliEnabled": optionalBoolField("The gh binding of the sessions the workspace's automations start: null inherits githubCliEnabled, false turns gh off for them, true turns it on for them. An automation's own template.agentConfig.githubCli wins over it, and it has no effect under a mandatory off.",
			func(v Environment) *bool { return v.AutomationsGitHubCLIEnabled }),
		"githubCliMandatory": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Description: "Whether the workspace's gh off is mandatory: while githubCliEnabled is false, no automations default, automation or session turns gh back on. No effect while githubCliEnabled is true.",
			Resolve: gqlutil.Field(func(v Environment) any { return v.GitHubCLIMandatory })},
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
		"agentModelCreditUsd":           optionalFloatField("The billing credit (USD) taken off this charge window's agent-model spend before the card is charged: credits pay first. 0 until bex has read the balance in this window; null when the store is off.", func(v Environment) *float64 { return v.AgentModelCreditUSD }),
		"agentModelUnbilledUsd":         optionalFloatField("The agent-model spend (USD) a threshold charge would collect now: this charge window's spend less the billing credit and the charges already paid in it.", func(v Environment) *float64 { return v.AgentModelUnbilledUSD }),
		"nextChargeThresholdUsd":        optionalFloatField("The agent-model spend beyond billing credits (which pay first) at which the card is charged mid-cycle; null at the top tier (month-end only).", func(v Environment) *float64 { return v.NextChargeThresholdUSD }),
		"trustTierLocked":               optionalBoolField("A failed or disputed payment holds the workspace at the first trust tier.", func(v Environment) *bool { return v.TrustTierLocked }),
		"agentModelSpendCapTierLimited": optionalBoolField("The trust tier's ceiling, not the plan default or override, sets the effective spend cap.", func(v Environment) *bool { return v.AgentModelSpendCapTierLimited }),
		"agentModelSpendCapReached":     optionalBoolField("Platform-model requests are refused at the spend cap now. False while the next request would charge the card at the trust tier's threshold instead.", func(v Environment) *bool { return v.AgentModelSpendCapReached }),
		"agentModelChargesAtThreshold":  optionalBoolField("Reaching nextChargeThresholdUsd charges the workspace's card and it keeps working. False where bex cannot charge it (no subscription, excluded or comped), on its own model key, or under a payment hold.", func(v Environment) *bool { return v.AgentModelChargesAtThreshold }),
		"billingBalanceDue":             optionalBoolField("A failed invoice is unpaid, so platform-model requests are refused with BILLING_BALANCE_DUE until it is paid.", func(v Environment) *bool { return v.BillingBalanceDue }),
	}, func(v Environment) TemplateOffer { return v.TemplateOffer }),
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
		"defaultTemplate":                &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "The sandbox template sessions and automation runs that name none run on: one of templates; \"\" returns to agent. Applies to sessions created afterwards."},
		spendCapField:                    &graphql.InputObjectFieldConfig{Type: graphql.Float, Description: "Monthly agent-model spend cap in USD (more than 0, at most 10x the plan default)."},
		"resetAgentModelMonthlySpendCap": &graphql.InputObjectFieldConfig{Type: graphql.Boolean, Description: "true returns the spend cap to the plan default."},
		"modelApiKey":                    &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Write-only: sets or rotates the workspace's own model provider key. Never returned; read modelKeySet."},
		"clearModelApiKey":               &graphql.InputObjectFieldConfig{Type: graphql.Boolean, Description: "true removes the workspace's own model provider key."},
		"enabledAgents":                  &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: "Replaces the agent profiles members can start sessions with; the others are turned off. A profile in agentDataUse needs its acknowledgement first."},
		"acknowledgeAgentDataUse":        &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: "Agent profiles from agentDataUse whose data use this admin acknowledges."},
		"githubCliEnabled":               &graphql.InputObjectFieldConfig{Type: graphql.Boolean, Description: "false turns gh off for the workspace's agent sessions: turns dispatched afterwards get no gh configuration and a running session's next gh call is refused. true turns it back on."},
		automationsGitHubCLIField:        &graphql.InputObjectFieldConfig{Type: graphql.Boolean, Description: "The gh binding of the sessions automations start: false turns gh off for them, true turns it on for them, resetAutomationsGithubCli returns it to inheriting githubCliEnabled. true is refused with AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED while the workspace's gh off is mandatory."},
		"resetAutomationsGithubCli":      &graphql.InputObjectFieldConfig{Type: graphql.Boolean, Description: "true returns the automations default to inheriting githubCliEnabled (the explicit null this input cannot carry)."},
		"githubCliMandatory":             &graphql.InputObjectFieldConfig{Type: graphql.Boolean, Description: "true makes the workspace's gh off mandatory: while githubCliEnabled is false nothing below turns gh back on, and saving an automations default, automation or session that would is refused with a GITHUB_CLI_LOCKED code. false lets lower levels choose again."},
		"enabledModels":                  &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: "Replaces the catalog models members may choose; the catalog's others are turned off. Every catalog model on allows all models, one added to the catalog later included; any model off makes the list an allowlist, which leaves a model added to the catalog later off until an admin turns it on. It must keep the catalog's default and name only catalog models (AGENT_ENVIRONMENT_MODELS_INVALID). New sessions, messages and automation saves see it at once; a session already on a turned-off model keeps it for the turn in flight and runs the workspace default from its next turn."},
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
	input.DefaultTemplate = gqlutil.StrPtr(raw, "defaultTemplate")
	input.ModelAPIKey = gqlutil.StrPtr(raw, "modelApiKey")
	if clear := gqlutil.BoolPtr(raw, "clearModelApiKey"); clear != nil {
		input.ClearModelAPIKey = *clear
	}
	if agents, ok := raw["enabledAgents"].([]any); ok {
		list := gqlutil.StringList(agents)
		input.EnabledAgents = &list
	}
	if acks, ok := raw["acknowledgeAgentDataUse"].([]any); ok {
		input.AcknowledgeAgentDataUse = gqlutil.StringList(acks)
	}
	input.GitHubCLIEnabled = gqlutil.BoolPtr(raw, "githubCliEnabled")
	if v, present := raw[automationsGitHubCLIField]; present {
		input.AutomationsGitHubCLIEnabledSet = true
		if b, ok := v.(bool); ok {
			input.AutomationsGitHubCLIEnabled = &b
		}
	}
	if reset, ok := raw["resetAutomationsGithubCli"].(bool); ok {
		input.ResetAutomationsGitHubCLI = reset
	}
	input.GitHubCLIMandatory = gqlutil.BoolPtr(raw, "githubCliMandatory")
	if models, ok := raw["enabledModels"].([]any); ok {
		list := gqlutil.StringList(models)
		input.EnabledModels = &list
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
		"state":   gqlutil.OptionalStrField(func(v PullRequest) any { return v.State }),
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
		"id":                 gqlutil.ReqStrField(func(v View) any { return v.ID }),
		"ownerId":            gqlutil.ReqStrField(func(v View) any { return v.OwnerID }),
		"repo":               gqlutil.ReqStrField(func(v View) any { return v.Repo }),
		"branch":             gqlutil.ReqStrField(func(v View) any { return v.Branch }),
		"title":              gqlutil.StrField(func(v View) any { return v.Title }),
		"maxSpendUSD":        gqlutil.StrField(func(v View) any { return v.MaxSpendUSD }),
		"tags":               &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Resolve: gqlutil.Field(func(v View) any { return v.Tags })},
		"secretNames":        &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: "Names of the secrets attached to the session; values are never returned.", Resolve: gqlutil.Field(func(v View) any { return v.SecretNames })},
		"appliedSecrets":     &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(appliedSecretGQLType))), Description: "The secrets the latest turn received, each with its source (inline, explicit, repo or workspace); values are never returned.", Resolve: gqlutil.Field(func(v View) any { return v.AppliedSecrets })},
		"environmentBuildId": &graphql.Field{Type: graphql.String, Description: "The repository environment build (arb-…) the latest turn's sandbox booted from; null when it ran the setup steps.", Resolve: gqlutil.OptionalStrField(func(v View) any { return v.EnvironmentBuildID }).Resolve},
		"automationId":       &graphql.Field{Type: graphql.String, Description: "The agent automation (aut-…) that started this session, when one did.", Resolve: gqlutil.Field(func(v View) any { return v.AutomationID })},
		"createdBy":          &graphql.Field{Type: graphql.String, Description: "Who started the session: a member's identity id, or an API key's client id. An automation's sessions carry the automation's creator and a child session its parent's creator. Absent on sessions from before it was recorded.", Resolve: gqlutil.Field(func(v View) any { return v.CreatedBy })},
		"parentSessionId":    &graphql.Field{Type: graphql.String, Description: "The agent session (ags-…) that spawned this one, when one did.", Resolve: gqlutil.Field(func(v View) any { return v.ParentSessionID })},
		"rootSessionId":      &graphql.Field{Type: graphql.String, Description: "The top of this session's tree, absent on a top-level session.", Resolve: gqlutil.Field(func(v View) any { return v.RootSessionID })},
		"depth":              &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Description: "Distance from the root of the session's tree; 0 for a top-level session.", Resolve: gqlutil.Field(func(v View) any { return v.Depth })},
		"agentConfig":        gqlutil.Typed(graphql.NewNonNull(agentConfigGQLType), func(v View) any { return v.AgentConfig }),
		"sandboxId":          gqlutil.StrField(func(v View) any { return v.SandboxID }),
		"sshAddress":         gqlutil.StrField(func(v View) any { return v.SSHAddress }),
		"phase":              gqlutil.ReqStrField(func(v View) any { return v.Phase }),
		"state":              gqlutil.ReqStrField(func(v View) any { return v.State }),
		"steerable":          &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v View) any { return v.Steerable })},
		"status":             gqlutil.ReqStrField(func(v View) any { return v.Status }),
		"headSha":            gqlutil.StrField(func(v View) any { return v.HeadSHA }),
		"prUrl":              gqlutil.StrField(func(v View) any { return v.PRURL }),
		"prState":            &graphql.Field{Type: graphql.String, Description: "The pull request's state as bex last read it from GitHub: open (a draft too), merged or closed; null until first read.", Resolve: gqlutil.OptionalStrField(func(v View) any { return v.PRState }).Resolve},
		"prStateAt":          gqlutil.TimeField(func(v View) any { return v.PRStateAt }),
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
		"pendingMessages": &graphql.Field{Type: graphql.NewNonNull(graphql.Int),
			Description: "How many queued messages wait to run as this session's next turns (ADR047 D12); set on lists and single-session reads.",
			Resolve:     gqlutil.Field(func(v View) any { return v.PendingMessages })},
		"queuedMessages": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(queuedMessageGQLType)),
			Description: "The messages waiting to run, next first; only on a single-session read.",
			Resolve: func(p graphql.ResolveParams) (any, error) {
				v, ok := p.Source.(View)
				if !ok {
					return nil, nil
				}
				return v.queued(p.Context), nil
			}},
		"queuedMessage": &graphql.Field{Type: queuedMessageGQLType,
			Description: "The message a steerAgentSession queued behind the running turn instead of starting a turn; null when it started one.",
			Resolve: gqlutil.Field(func(v View) any {
				if v.Queued == nil {
					return nil
				}
				return *v.Queued
			})},
		"turnOrigins": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(agentSessionTurnOriginGQLType)),
			Description: "Which turns PR feedback started (a review, a comment or a failed CI run); only on a single-session read.",
			Resolve:     gqlutil.Field(func(v View) any { return v.TurnOrigins })},
		"waitingFor":   &graphql.Field{Type: graphql.String, Description: waitingForDescription, Resolve: gqlutil.OptionalStrField(func(v View) any { return v.WaitingFor }).Resolve},
		"waitingSince": &graphql.Field{Type: graphql.String, Description: waitingSinceDescription, Resolve: gqlutil.TimeField(func(v View) any { return v.WaitingSince }).Resolve},
		"failureCause": &graphql.Field{Type: graphql.String, Description: failureCauseDescription, Resolve: gqlutil.OptionalStrField(func(v View) any { return v.FailureCause }).Resolve},
	},
})

var agentSessionTurnOriginGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentSessionTurnOrigin",
	Fields: graphql.Fields{
		"turn":   gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(v TurnOrigin) any { return v.Turn }),
		"kind":   gqlutil.ReqStrField(func(v TurnOrigin) any { return v.Kind }),
		"author": gqlutil.OptionalStrField(func(v TurnOrigin) any { return v.Author }),
		"url":    gqlutil.OptionalStrField(func(v TurnOrigin) any { return v.URL }),
		// ci_failure: the base branch a failing check also fails on.
		"baseFailingRef": gqlutil.OptionalStrField(func(v TurnOrigin) any { return v.BaseFailingRef }),
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
		"interrupted":    &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Description: "A person stopped the turn before it ended on its own (ADR047 D12).", Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.Interrupted })},
		"outputRequired": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.OutputRequired })},
		"result":         jsonTextField("The agent's last valid structured result for the turn as JSON text, readable while the turn runs, or null.", func(v TranscriptTurn) any { return v.Result }),
		"resultFinal":    &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Description: "The agent declared result final; only a final result becomes outcome.result.", Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.ResultFinal })},
		"retention": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: retentionDescription,
			Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.Retention })},
		"retainedUntil": &graphql.Field{Type: graphql.String, Description: retainedUntilDescription, Resolve: gqlutil.Field(func(v TranscriptTurn) any {
			if v.RetainedUntil == nil {
				return nil
			}
			return gqlTime(*v.RetainedUntil)
		})},
		"retentionLostReason": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: retentionLostReasonDescription + " Empty otherwise.",
			Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.RetentionLostReason })},
		"retentionCause": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: retentionCauseDescription + " Empty when retention is none.",
			Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.RetentionCause })},
		"unpushedCommits": &graphql.Field{Type: graphql.Int, Description: unpushedCommitsDescription + " Null when retention is none or the count is unknown.",
			Resolve: gqlutil.Field(func(v TranscriptTurn) any {
				if v.UnpushedCommits == nil {
					return nil
				}
				return *v.UnpushedCommits
			})},
		"uncommittedChanges": &graphql.Field{Type: graphql.Boolean, Description: uncommittedChangesDescription + " Null when retention is none or it is unknown.",
			Resolve: gqlutil.Field(func(v TranscriptTurn) any {
				if v.UncommittedChanges == nil {
					return nil
				}
				return *v.UncommittedChanges
			})},
		"startNote": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: startNoteDescription + " Empty otherwise.",
			Resolve: gqlutil.Field(func(v TranscriptTurn) any { return v.StartNote })},
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
	Description: "A workspace agent secret: its name and scope. The value is never readable.",
	Fields: graphql.Fields{
		"name":  gqlutil.ReqStrField(func(v AgentSecret) any { return v.Name }),
		"scope": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: "workspace (every session receives it), repo (sessions on `repo`) or explicit (only sessions that name it in secretNames).", Resolve: gqlutil.Field(func(v AgentSecret) any { return v.Scope })},
		"repo":  gqlutil.OptionalStrField(func(v AgentSecret) any { return v.Repo }),
	},
})

var repoStepGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentRepoStep",
	Description: "A repository environment's setup step or check: a name and a shell command.",
	Fields: graphql.Fields{
		"name": gqlutil.ReqStrField(func(v RepoStep) any { return v.Name }),
		"run":  gqlutil.ReqStrField(func(v RepoStep) any { return v.Run }),
	},
})

var repoEnvironmentGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentRepoEnvironment",
	Description: "A repository's environment for agent sessions (ADR109): setup steps run before the agent, checks it runs before delivering.",
	Fields: graphql.Fields{
		"ownerId": gqlutil.ReqStrField(func(v RepoEnvironment) any { return v.OwnerID }),
		"repo":    gqlutil.ReqStrField(func(v RepoEnvironment) any { return v.Repo }),
		"setup":   gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(repoStepGQLType))), func(v RepoEnvironment) any { return v.Setup }),
		"checks":  gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(repoStepGQLType))), func(v RepoEnvironment) any { return v.Checks }),
		"version": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: "The SHA-256 of the setup steps; a build of another version is stale.", Resolve: gqlutil.Field(func(v RepoEnvironment) any { return v.Version })},
		"latestBuild": &graphql.Field{Type: repoEnvironmentBuildGQLType, Description: "The newest build of the repository's environment, if it has one.", Resolve: gqlutil.Field(func(v RepoEnvironment) any {
			if v.LatestBuild == nil {
				return nil
			}
			return *v.LatestBuild
		})},
		"updatedAt": gqlutil.TimeField(func(v RepoEnvironment) any { return v.UpdatedAt }),
	},
})

var repoEnvironmentListGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentRepoEnvironmentList",
	Fields: graphql.Fields{
		"ownerId":      gqlutil.ReqStrField(func(v RepoEnvironmentList) any { return v.OwnerID }),
		"environments": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(repoEnvironmentGQLType))), func(v RepoEnvironmentList) any { return v.Environments }),
	},
})

var repoStepGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "AgentRepoStepInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"name": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"run":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

// repoStepsArg reads a list of AgentRepoStepInput.
func repoStepsArg(arg any) []RepoStep {
	raw, _ := arg.([]any)
	out := make([]RepoStep, 0, len(raw))
	for _, item := range raw {
		m, _ := item.(map[string]any)
		out = append(out, RepoStep{Name: gqlutil.Str(m, "name"), Run: gqlutil.Str(m, "run")})
	}
	return out
}

var appliedSecretGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentAppliedSecret",
	Description: "A secret a session's latest turn received: its name and why.",
	Fields: graphql.Fields{
		"name":   gqlutil.ReqStrField(func(v AppliedSecret) any { return v.Name }),
		"source": gqlutil.ReqStrField(func(v AppliedSecret) any { return v.Source }),
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
	return AgentConfig{Agent: gqlutil.Str(raw, "agent"), Model: gqlutil.Str(raw, "model"), ModelEndpoint: gqlutil.Str(raw, "modelEndpoint"), Task: gqlutil.Str(raw, "task"), Template: gqlutil.Str(raw, "template"), GitHubCLI: gqlutil.BoolPtr(raw, "githubCli")}
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
				"prStates":        &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: "Only the sessions whose pull request is in one of these states: open, merged, closed."},
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
					PRStates:        gqlutil.StringList(p.Args["prStates"]),
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
		"agentRepoEnvironments": &graphql.Field{
			Type:        graphql.NewNonNull(repoEnvironmentListGQLType),
			Description: "The workspace's repository environments (setup steps and checks).",
			Args:        graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RepoEnvironments(p.Context, gqlutil.Str(p.Args, "ownerId"))
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
	maps.Copy(fields, s.repoEnvironmentBuildQueries())
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
				"idempotencyKey":  &graphql.ArgumentConfig{Type: graphql.String, Description: "Makes a retried send harmless: the same key and message return what the first attempt did, the turn it started or the message it queued."},
				"egressAllowlist": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"attachmentIds":   gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"outputSchema":    gqlutil.Arg(graphql.String),
				"outputRequired":  gqlutil.Arg(graphql.Boolean),
				"secretNames":     gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"secrets":         gqlutil.Arg(graphql.NewList(graphql.NewNonNull(agentSessionSecretGQLInput))),
				"skills":          gqlutil.Arg(graphql.NewList(graphql.NewNonNull(agentSessionSkillGQLInput))),
				"model":           &graphql.ArgumentConfig{Type: graphql.String, Description: "Switch the session's model from the turn this message starts (a queued message switches it when it runs). On the platform model route it must be one of the agent's capabilities models; omitted keeps the session's model."},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				schema, required, err := outputArgs(p.Args)
				if err != nil {
					return nil, err
				}
				return s.Steer(p.Context, SteerRequest{OutputSchema: schema, OutputRequired: required, SessionID: gqlutil.Str(p.Args, "id"), Prompt: gqlutil.Str(p.Args, "prompt"), IdempotencyKey: gqlutil.Str(p.Args, "idempotencyKey"), EgressAllowlist: gqlutil.StringList(p.Args["egressAllowlist"]), AttachmentIDs: gqlutil.StringList(p.Args["attachmentIds"]), SessionSecretsInput: secretsArg(p.Args), Skills: skillsArg(p.Args), Model: gqlutil.Str(p.Args, "model")})
			},
		},
		"setAgentRepoEnvironment": &graphql.Field{
			Type:        graphql.NewNonNull(repoEnvironmentGQLType),
			Description: "Create or replace a repository's environment (admins only).",
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"repo":    gqlutil.ReqArg(graphql.String),
				"setup":   gqlutil.ReqArg(graphql.NewList(graphql.NewNonNull(repoStepGQLInput))),
				"checks":  gqlutil.ReqArg(graphql.NewList(graphql.NewNonNull(repoStepGQLInput))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetRepoEnvironment(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "repo"),
					RepoEnvironmentInput{Setup: repoStepsArg(p.Args["setup"]), Checks: repoStepsArg(p.Args["checks"])})
			},
		},
		"deleteAgentRepoEnvironment": &graphql.Field{
			Type:        graphql.Boolean,
			Description: "Delete a repository's environment (admins only).",
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"repo":    gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.DeleteRepoEnvironment(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "repo")); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"setAgentSecret": &graphql.Field{
			Type:        graphql.NewNonNull(agentSecretGQLType),
			Description: "Create, rotate or rescope a workspace agent secret (admins only). The value is write-only; omit it to change only the scope of an existing secret. A new secret without a scope is workspace-wide.",
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"name":    gqlutil.ReqArg(graphql.String),
				"value":   gqlutil.Arg(graphql.String),
				"scope":   gqlutil.Arg(graphql.String),
				"repo":    gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetAgentSecret(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "name"),
					AgentSecretInput{Value: gqlutil.Str(p.Args, "value"), Scope: gqlutil.StrPtr(p.Args, "scope"), Repo: gqlutil.Str(p.Args, "repo")})
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
	maps.Copy(fields, s.queueMutations())
	maps.Copy(fields, s.repoEnvironmentBuildMutations())
	return fields
}
