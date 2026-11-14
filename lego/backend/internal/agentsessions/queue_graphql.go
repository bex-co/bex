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

// GraphQL for the ADR047 D12 message queue and Stop turn, over the same core
// as REST and MCP.

var queuedMessageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentSessionQueuedMessage",
	Description: "A message sent to an agent session while its turn ran, waiting to run as its own turn (ADR047 D12).",
	Fields: graphql.Fields{
		"id":        gqlutil.ReqStrField(func(v QueuedMessage) any { return v.ID }),
		"sessionId": gqlutil.ReqStrField(func(v QueuedMessage) any { return v.SessionID }),
		"position":  gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(v QueuedMessage) any { return v.Position }),
		"prompt":    gqlutil.ReqStrField(func(v QueuedMessage) any { return v.Prompt }),
		"author": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: "The member the turn runs as: who queued the message, or who last edited it.",
			Resolve: gqlutil.Field(func(v QueuedMessage) any { return v.Author })},
		"attachmentIds":   gqlutil.Typed(graphql.NewList(graphql.NewNonNull(graphql.String)), func(v QueuedMessage) any { return v.AttachmentIDs }),
		"secretNames":     gqlutil.Typed(graphql.NewList(graphql.NewNonNull(graphql.String)), func(v QueuedMessage) any { return v.SecretNames }),
		"egressAllowlist": gqlutil.Typed(graphql.NewList(graphql.NewNonNull(graphql.String)), func(v QueuedMessage) any { return v.EgressAllowlist }),
		"hasOutputSchema": gqlutil.Typed(graphql.NewNonNull(graphql.Boolean), func(v QueuedMessage) any { return v.HasOutputSchema }),
		"blockedReason": &graphql.Field{Type: graphql.String, Description: "Why the last attempt to run it failed (a spend or live-sandbox limit); it is retried about every half minute.",
			Resolve: gqlutil.OptionalStrField(func(v QueuedMessage) any { return v.BlockedReason }).Resolve},
		"state":         gqlutil.ReqStrField(func(v QueuedMessage) any { return v.State }),
		"turn":          gqlutil.IntField(func(v QueuedMessage) any { return v.Turn }),
		"droppedReason": gqlutil.OptionalStrField(func(v QueuedMessage) any { return v.DroppedReason }),
		"createdAt":     gqlutil.TimeField(func(v QueuedMessage) any { return v.CreatedAt }),
		"updatedAt":     gqlutil.TimeField(func(v QueuedMessage) any { return v.UpdatedAt }),
	},
})

func (s *Service) queueMutations() graphql.Fields {
	interrupt := gqlutil.IDVerb(agentSessionGQLType, s.Interrupt)
	interrupt.Description = "Stop turn: ends only the running turn. The session settles as completed with the turn interrupted and stays steerable; queued messages keep their place. Cancel, which ends the session, is unchanged."
	return graphql.Fields{
		"interruptAgentSession": interrupt,
		"editQueuedAgentSessionMessage": &graphql.Field{
			Type:        graphql.NewNonNull(queuedMessageGQLType),
			Description: "Replaces a pending message's prompt; the turn then runs as you.",
			Args: graphql.FieldConfigArgument{
				"id":        gqlutil.ReqArg(graphql.String),
				"messageId": gqlutil.ReqArg(graphql.String),
				"prompt":    gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.EditQueuedMessage(p.Context, gqlutil.Str(p.Args, "id"), gqlutil.Str(p.Args, "messageId"), gqlutil.Str(p.Args, "prompt"))
			},
		},
		"deleteQueuedAgentSessionMessage": &graphql.Field{
			Type:        graphql.Boolean,
			Description: "Removes a pending message so it never runs.",
			Args: graphql.FieldConfigArgument{
				"id":        gqlutil.ReqArg(graphql.String),
				"messageId": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.DeleteQueuedMessage(p.Context, gqlutil.Str(p.Args, "id"), gqlutil.Str(p.Args, "messageId")); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"moveQueuedAgentSessionMessage": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(queuedMessageGQLType))),
			Description: "Moves a pending message to a place in the queue (1 runs next; past the end is last) and returns the queue. Send now is interruptAgentSession, then a move to 1.",
			Args: graphql.FieldConfigArgument{
				"id":        gqlutil.ReqArg(graphql.String),
				"messageId": gqlutil.ReqArg(graphql.String),
				"position":  gqlutil.ReqArg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				list, err := s.MoveQueuedMessage(p.Context, gqlutil.Str(p.Args, "id"), gqlutil.Str(p.Args, "messageId"), gqlutil.Int(p.Args, "position"))
				if err != nil {
					return nil, err
				}
				return list.QueuedMessages, nil
			},
		},
	}
}
