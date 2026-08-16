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

package jobs

import (
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the one-off jobs GraphQL fragment: jobs(serviceId) query and
// createJob/cancelJob mutations, mirroring the REST surface so the three
// surfaces cannot diverge. All delegates to the same Service verbs REST uses.

var jobGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Job",
	Fields: graphql.Fields{
		"id":           gqlutil.StrField(func(v JobView) any { return v.ID }),
		"serviceId":    gqlutil.StrField(func(v JobView) any { return v.ServiceID }),
		"startCommand": gqlutil.StrField(func(v JobView) any { return v.StartCommand }),
		"planId":       gqlutil.StrField(func(v JobView) any { return v.PlanID }),
		"status":       gqlutil.StrField(func(v JobView) any { return v.Status }),
		"createdAt":    gqlutil.StrField(func(v JobView) any { return v.CreatedAt.UTC().Format(time.RFC3339) }),
		"startedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v JobView) any {
			if v.StartedAt == nil {
				return ""
			}
			return v.StartedAt.UTC().Format(time.RFC3339)
		})},
		"finishedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v JobView) any {
			if v.FinishedAt == nil {
				return ""
			}
			return v.FinishedAt.UTC().Format(time.RFC3339)
		})},
	},
})

var jobServiceIDArg = graphql.FieldConfigArgument{
	"serviceId": gqlutil.ReqArg(graphql.String),
}

// GraphQLQuery returns jobs(serviceId, …) for the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"jobs": &graphql.Field{
			Type: graphql.NewList(jobGQLType),
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
				"cursor":    gqlutil.Arg(graphql.String),
				"limit":     gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				limit, err := gqlutil.PositiveLimit(p.Args)
				if err != nil {
					return nil, err
				}
				cursor := gqlutil.Str(p.Args, "cursor")
				filter, err := FilterFromStrings(nil, "", "", "", "", "", "", cursor, limit)
				if err != nil {
					return nil, err
				}
				return s.List(p.Context, p.Args["serviceId"].(string), filter)
			},
		},
		"job": &graphql.Field{
			Type: jobGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
				"jobId":     gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, p.Args["serviceId"].(string), p.Args["jobId"].(string))
			},
		},
	}
}

// GraphQLMutation returns createJob and cancelJob mutations.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createJob": &graphql.Field{
			Type: jobGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId":    gqlutil.ReqArg(graphql.String),
				"startCommand": gqlutil.ReqArg(graphql.String),
				"planId":       gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				planID := gqlutil.Str(p.Args, "planId")
				return s.Create(p.Context, p.Args["serviceId"].(string), p.Args["startCommand"].(string), planID)
			},
		},
		"cancelJob": &graphql.Field{
			Type: jobGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
				"jobId":     gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Cancel(p.Context, p.Args["serviceId"].(string), p.Args["jobId"].(string))
			},
		},
	}
}
