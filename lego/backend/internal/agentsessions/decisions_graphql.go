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

// agentSessionDecisionGQLType is the GraphQL projection of Decision (ADR100
// D2). Detail and answer are JSON text, like the transcript part.
var agentSessionDecisionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentSessionDecision",
	Description: "An approval or question a running agent turn paused on (ADR100).",
	Fields: graphql.Fields{
		"id":         gqlutil.ReqStrField(func(v Decision) any { return v.ID }),
		"sessionId":  gqlutil.ReqStrField(func(v Decision) any { return v.SessionID }),
		"turn":       gqlutil.IntField(func(v Decision) any { return v.Turn }),
		"kind":       gqlutil.ReqStrField(func(v Decision) any { return v.Kind }),
		"title":      gqlutil.ReqStrField(func(v Decision) any { return v.Title }),
		"detail":     jsonTextField("What the agent asked, as JSON text: {toolKind, command, paths, rule} for an approval, {question, options} for a question.", func(v Decision) any { return v.Detail }),
		"state":      gqlutil.ReqStrField(func(v Decision) any { return v.State }),
		"answer":     jsonTextField("The final answer as JSON text ({allow, scope} or {text}); null while pending.", func(v Decision) any { return v.Answer }),
		"createdAt":  gqlutil.TimeField(func(v Decision) any { return v.CreatedAt }),
		"answeredAt": gqlutil.TimeField(func(v Decision) any { return v.AnsweredAt }),
		"expiresAt":  gqlutil.TimeField(func(v Decision) any { return v.ExpiresAt }),
	},
})

func (s *Service) decisionMutations() graphql.Fields {
	return graphql.Fields{
		"answerAgentSessionDecision": &graphql.Field{
			Type:        graphql.NewNonNull(agentSessionDecisionGQLType),
			Description: "Answer the decision a running agent turn paused on: allow (and scope once|session) for an approval, answer for a question.",
			Args: graphql.FieldConfigArgument{
				"id":         gqlutil.ReqArg(graphql.String),
				"decisionId": gqlutil.ReqArg(graphql.String),
				"allow":      gqlutil.Arg(graphql.Boolean),
				"scope":      gqlutil.Arg(graphql.String),
				"answer":     gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.AnswerDecision(p.Context, AnswerDecisionRequest{
					SessionID: gqlutil.Str(p.Args, "id"), DecisionID: gqlutil.Str(p.Args, "decisionId"),
					Allow: gqlutil.BoolPtr(p.Args, "allow"), Scope: gqlutil.Str(p.Args, "scope"), Answer: gqlutil.StrPtr(p.Args, "answer"),
				})
			},
		},
	}
}
