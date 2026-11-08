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
	"encoding/json"
	"maps"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// GraphQL for ADR101 agent automations, over the same core as REST and MCP.
// The shapes mirror the REST JSON field for field; the template's
// outputSchema is JSON text, as on a session create (GraphQL has no JSON
// scalar here).

func strList(f func(AutomationTemplate) any) *graphql.Field {
	return &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Resolve: gqlutil.Field(f)}
}

var automationScheduleGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentAutomationSchedule",
	Description: "When an automation fires (ADR101 D2).",
	Fields: graphql.Fields{
		"frequency": gqlutil.ReqStrField(func(v AutomationSchedule) any { return v.Frequency }),
		"minute": gqlutil.IntField(func(v AutomationSchedule) any {
			if v.Minute == nil {
				return nil
			}
			return *v.Minute
		}),
		"at":       gqlutil.OptionalStrField(func(v AutomationSchedule) any { return v.At }),
		"weekdays": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Resolve: gqlutil.Field(func(v AutomationSchedule) any { return v.Weekdays })},
		"runAt":    gqlutil.TimeField(func(v AutomationSchedule) any { return v.RunAt }),
		"cron":     gqlutil.OptionalStrField(func(v AutomationSchedule) any { return v.Cron }),
		"timezone": gqlutil.ReqStrField(func(v AutomationSchedule) any { return v.Timezone }),
		"endsAt":   gqlutil.TimeField(func(v AutomationSchedule) any { return v.EndsAt }),
	},
})

var automationTriggerGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentAutomationTrigger",
	Fields: graphql.Fields{
		"type": gqlutil.ReqStrField(func(v AutomationTrigger) any { return v.Type }),
		"schedule": gqlutil.Typed(automationScheduleGQLType, func(v AutomationTrigger) any {
			if v.Schedule == nil {
				return nil
			}
			return *v.Schedule
		}),
	},
})

var automationAgentConfigGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentAutomationAgentConfig",
	Fields: graphql.Fields{
		"agent":    gqlutil.ReqStrField(func(v AutomationAgentConfig) any { return v.Agent }),
		"model":    gqlutil.OptionalStrField(func(v AutomationAgentConfig) any { return v.Model }),
		"template": gqlutil.OptionalStrField(func(v AutomationAgentConfig) any { return v.Template }),
	},
})

var automationTemplateGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentAutomationTemplate",
	Description: "The session each fire starts (ADR101 D5).",
	Fields: graphql.Fields{
		"task":        gqlutil.ReqStrField(func(v AutomationTemplate) any { return v.Task }),
		"repo":        gqlutil.OptionalStrField(func(v AutomationTemplate) any { return v.Repo }),
		"branch":      gqlutil.OptionalStrField(func(v AutomationTemplate) any { return v.Branch }),
		"title":       gqlutil.OptionalStrField(func(v AutomationTemplate) any { return v.Title }),
		"tags":        strList(func(v AutomationTemplate) any { return v.Tags }),
		"agentConfig": gqlutil.Typed(graphql.NewNonNull(automationAgentConfigGQLType), func(v AutomationTemplate) any { return v.AgentConfig }),
		"maxSpendUSD": gqlutil.FloatField(func(v AutomationTemplate) any {
			if v.MaxSpendUSD == nil {
				return nil
			}
			return *v.MaxSpendUSD
		}),
		"secretNames": strList(func(v AutomationTemplate) any { return v.SecretNames }),
		"skills":      &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(agentSessionSkillGQLType)), Resolve: gqlutil.Field(func(v AutomationTemplate) any { return v.Skills })},
		"outputSchema": jsonTextField("The JSON Schema each session's result must satisfy, as JSON text, or null.",
			func(v AutomationTemplate) any {
				if v.OutputSchema == nil {
					return nil
				}
				return v.OutputSchema
			}),
		"outputRequired": gqlutil.BoolField(func(v AutomationTemplate) any {
			if v.OutputRequired == nil {
				return nil
			}
			return *v.OutputRequired
		}),
		"egressAllowlist": strList(func(v AutomationTemplate) any { return v.EgressAllowlist }),
	},
})

var agentAutomationGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentAutomation",
	Description: "A schedule trigger plus a session template that runs as the member who created it (ADR101).",
	Fields: graphql.Fields{
		"id":        gqlutil.ReqStrField(func(v AgentAutomation) any { return v.ID }),
		"ownerId":   gqlutil.ReqStrField(func(v AgentAutomation) any { return v.OwnerID }),
		"name":      gqlutil.ReqStrField(func(v AgentAutomation) any { return v.Name }),
		"enabled":   gqlutil.ReqBoolField(func(v AgentAutomation) any { return v.Enabled }),
		"createdBy": gqlutil.ReqStrField(func(v AgentAutomation) any { return v.CreatedBy }),
		"triggers": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(automationTriggerGQLType))),
			func(v AgentAutomation) any { return v.Triggers }),
		"template":            gqlutil.Typed(graphql.NewNonNull(automationTemplateGQLType), func(v AgentAutomation) any { return v.Template }),
		"overlapPolicy":       gqlutil.ReqStrField(func(v AgentAutomation) any { return v.OverlapPolicy }),
		"nextFireAt":          gqlutil.TimeField(func(v AgentAutomation) any { return v.NextFireAt }),
		"lastFireAt":          gqlutil.TimeField(func(v AgentAutomation) any { return v.LastFireAt }),
		"lastSessionId":       gqlutil.OptionalStrField(func(v AgentAutomation) any { return v.LastSessionID }),
		"pausedReason":        gqlutil.OptionalStrField(func(v AgentAutomation) any { return v.PausedReason }),
		"consecutiveFailures": gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(v AgentAutomation) any { return v.ConsecutiveFailures }),
		"version":             gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(v AgentAutomation) any { return v.Version }),
		"createdAt":           gqlutil.TimeField(func(v AgentAutomation) any { return v.CreatedAt }),
		"updatedAt":           gqlutil.TimeField(func(v AgentAutomation) any { return v.UpdatedAt }),
	},
})

var agentAutomationListGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentAutomationList",
	Fields: graphql.Fields{
		"ownerId":     gqlutil.ReqStrField(func(v AgentAutomationList) any { return v.OwnerID }),
		"automations": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentAutomationGQLType))), func(v AgentAutomationList) any { return v.Automations }),
		"nextCursor":  gqlutil.OptionalStrField(func(v AgentAutomationList) any { return v.NextCursor }),
	},
})

var agentAutomationRunGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentAutomationRun",
	Description: "One fire of an automation (ADR101 D3, D7).",
	Fields: graphql.Fields{
		"automationId": gqlutil.ReqStrField(func(v AgentAutomationRun) any { return v.AutomationID }),
		"fireAt":       gqlutil.TimeField(func(v AgentAutomationRun) any { return v.FireAt }),
		"outcome":      gqlutil.ReqStrField(func(v AgentAutomationRun) any { return v.Outcome }),
		"sessionId":    gqlutil.OptionalStrField(func(v AgentAutomationRun) any { return v.SessionID }),
		"missedTicks":  gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(v AgentAutomationRun) any { return v.MissedTicks }),
		"error":        gqlutil.OptionalStrField(func(v AgentAutomationRun) any { return v.Error }),
		"createdAt":    gqlutil.TimeField(func(v AgentAutomationRun) any { return v.CreatedAt }),
		"settledAt":    gqlutil.TimeField(func(v AgentAutomationRun) any { return v.SettledAt }),
	},
})

var agentAutomationRunListGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentAutomationRunList",
	Fields: graphql.Fields{
		"automationId": gqlutil.ReqStrField(func(v AgentAutomationRunList) any { return v.AutomationID }),
		"runs":         gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(agentAutomationRunGQLType))), func(v AgentAutomationRunList) any { return v.Runs }),
		"nextCursor":   gqlutil.OptionalStrField(func(v AgentAutomationRunList) any { return v.NextCursor }),
	},
})

func strListInput(description string) *graphql.InputObjectFieldConfig {
	return &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String)), Description: description}
}

var automationScheduleGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "AgentAutomationScheduleInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"frequency": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String), Description: "once, hour, day, week or custom."},
		"minute":    &graphql.InputObjectFieldConfig{Type: graphql.Int},
		"at":        &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "HH:MM, 24-hour."},
		"weekdays":  strListInput("mon..sun"),
		"runAt":     &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "RFC 3339 instant, for once."},
		"cron":      &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "Five fields, for custom."},
		"timezone":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String), Description: "An IANA time zone name."},
		"endsAt":    &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "RFC 3339 instant, exclusive."},
	},
})

var automationTriggerGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "AgentAutomationTriggerInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"type":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String), Description: "schedule"},
		"schedule": &graphql.InputObjectFieldConfig{Type: automationScheduleGQLInput},
	},
})

var automationAgentConfigGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "AgentAutomationAgentConfigInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"agent":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"model":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		"template": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

var automationTemplateGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name:        "AgentAutomationTemplateInput",
	Description: "The session each fire starts: a create-session request without resumeSessionId, attachments, inline secrets and idempotencyKey.",
	Fields: graphql.InputObjectConfigFieldMap{
		"task":            &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"repo":            &graphql.InputObjectFieldConfig{Type: graphql.String},
		"branch":          &graphql.InputObjectFieldConfig{Type: graphql.String},
		"title":           &graphql.InputObjectFieldConfig{Type: graphql.String},
		"tags":            strListInput(""),
		"agentConfig":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(automationAgentConfigGQLInput)},
		"maxSpendUSD":     &graphql.InputObjectFieldConfig{Type: graphql.Float, Description: "Required when the model is platform-metered."},
		"secretNames":     strListInput("Workspace agent secrets exposed as env vars."),
		"skills":          &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(agentSessionSkillGQLInput))},
		"outputSchema":    &graphql.InputObjectFieldConfig{Type: graphql.String, Description: "A JSON Schema object as JSON text."},
		"outputRequired":  &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		"egressAllowlist": strListInput(""),
	},
})

var agentAutomationGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name:        "AgentAutomationInput",
	Description: "An automation's fields. On update, omitted fields keep their value; triggers and template replace the stored ones whole.",
	Fields: graphql.InputObjectConfigFieldMap{
		"name":      &graphql.InputObjectFieldConfig{Type: graphql.String},
		"enabled":   &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		"triggers":  &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(automationTriggerGQLInput)), Description: "Exactly one trigger, of type schedule."},
		"template":  &graphql.InputObjectFieldConfig{Type: automationTemplateGQLInput},
		"ifVersion": &graphql.InputObjectFieldConfig{Type: graphql.Int, Description: "Update only: refuse unless the automation is still at this version."},
	},
})

// automationInputArg reads the input object through the REST JSON shape, so
// the template's strict decoding and its named refusals apply here too.
func automationInputArg(raw any) (AgentAutomationInput, error) {
	var in AgentAutomationInput
	m, _ := raw.(map[string]any)
	if tpl, ok := m["template"].(map[string]any); ok {
		tpl = maps.Clone(tpl)
		if text, ok := tpl["outputSchema"].(string); ok {
			var schema map[string]any
			if json.Unmarshal([]byte(text), &schema) != nil || schema == nil {
				return in, automationInvalid("template.outputSchema", "outputSchema must be a JSON object")
			}
			tpl["outputSchema"] = schema
		}
		m = maps.Clone(m)
		m["template"] = tpl
	}
	data, err := json.Marshal(m)
	if err != nil {
		return in, automationInvalid("input", "the input is not valid JSON")
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return in, err
	}
	return in, nil
}

func automationRefArgs() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"ownerId": gqlutil.Arg(graphql.String),
		"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String), Description: "The automation's aut- id or its name."},
	}
}

func (s *Service) automationQueries() graphql.Fields {
	runsArgs := automationRefArgs()
	runsArgs["cursor"] = gqlutil.Arg(graphql.String)
	runsArgs["limit"] = gqlutil.Arg(graphql.Int)
	return graphql.Fields{
		"agentAutomations": &graphql.Field{
			Type:        graphql.NewNonNull(agentAutomationListGQLType),
			Description: "The workspace's agent automations, newest first. mine keeps the caller's own.",
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"mine":    gqlutil.Arg(graphql.Boolean),
				"cursor":  gqlutil.Arg(graphql.String),
				"limit":   gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				limit, err := automationLimitArg(p.Args)
				if err != nil {
					return nil, err
				}
				return s.AgentAutomations(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Bool(p.Args, "mine"), gqlutil.Str(p.Args, "cursor"), limit)
			},
		},
		"agentAutomation": &graphql.Field{
			Type:        graphql.NewNonNull(agentAutomationGQLType),
			Description: "One automation by aut- id or name.",
			Args:        automationRefArgs(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.AgentAutomation(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "id"))
			},
		},
		"agentAutomationRuns": &graphql.Field{
			Type:        graphql.NewNonNull(agentAutomationRunListGQLType),
			Description: "An automation's runs, newest first.",
			Args:        runsArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				limit, err := automationLimitArg(p.Args)
				if err != nil {
					return nil, err
				}
				return s.AgentAutomationRuns(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "id"), gqlutil.Str(p.Args, "cursor"), limit)
			},
		},
	}
}

// automationLimitArg: absent is the default page; a non-positive limit is the
// same 400 REST answers.
func automationLimitArg(args map[string]any) (int, error) {
	if _, ok := args["limit"]; !ok {
		return 0, nil
	}
	n := gqlutil.Int(args, "limit")
	if n < 1 {
		return 0, core.ErrLimitNotPositive
	}
	return n, nil
}

func (s *Service) automationMutations() graphql.Fields {
	verb := func(description string, call func(*Service, graphql.ResolveParams) (any, error), out graphql.Output) *graphql.Field {
		return &graphql.Field{Type: out, Description: description, Args: automationRefArgs(),
			Resolve: func(p graphql.ResolveParams) (any, error) { return call(s, p) }}
	}
	ref := func(p graphql.ResolveParams) (string, string) {
		return gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "id")
	}
	updateArgs := automationRefArgs()
	updateArgs["input"] = gqlutil.ReqArg(agentAutomationGQLInput)
	nonNull := graphql.NewNonNull(agentAutomationGQLType)
	return graphql.Fields{
		"createAgentAutomation": &graphql.Field{
			Type:        nonNull,
			Description: "Add an automation owned by the caller.",
			Args:        graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String), "input": gqlutil.ReqArg(agentAutomationGQLInput)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				in, err := automationInputArg(p.Args["input"])
				if err != nil {
					return nil, err
				}
				return s.CreateAgentAutomation(p.Context, gqlutil.Str(p.Args, "ownerId"), in)
			},
		},
		"updateAgentAutomation": &graphql.Field{
			Type:        nonNull,
			Description: "Change an automation (its creator only).",
			Args:        updateArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				in, err := automationInputArg(p.Args["input"])
				if err != nil {
					return nil, err
				}
				owner, id := ref(p)
				return s.UpdateAgentAutomation(p.Context, owner, id, in)
			},
		},
		"deleteAgentAutomation": verb("Delete an automation and its run history (its creator or a workspace admin).",
			func(s *Service, p graphql.ResolveParams) (any, error) {
				owner, id := ref(p)
				if err := s.DeleteAgentAutomation(p.Context, owner, id); err != nil {
					return nil, err
				}
				return true, nil
			}, graphql.Boolean),
		"pauseAgentAutomation": verb("Stop an automation firing.", func(s *Service, p graphql.ResolveParams) (any, error) {
			owner, id := ref(p)
			return s.PauseAgentAutomation(p.Context, owner, id)
		}, nonNull),
		"resumeAgentAutomation": verb("Re-enable an automation.", func(s *Service, p graphql.ResolveParams) (any, error) {
			owner, id := ref(p)
			return s.ResumeAgentAutomation(p.Context, owner, id)
		}, nonNull),
		"takeOverAgentAutomation": verb("Make the calling workspace admin the automation's creator.", func(s *Service, p graphql.ResolveParams) (any, error) {
			owner, id := ref(p)
			return s.TakeOverAgentAutomation(p.Context, owner, id)
		}, nonNull),
		"runAgentAutomation": verb("Fire an automation now, as a normal run.", func(s *Service, p graphql.ResolveParams) (any, error) {
			owner, id := ref(p)
			return s.RunAgentAutomationNow(p.Context, owner, id)
		}, graphql.NewNonNull(agentAutomationRunGQLType)),
	}
}
