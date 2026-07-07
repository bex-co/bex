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

package apps

import (
	"context"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the GraphQL fragment, matching the operation names Render's
// dashboard uses (captured live): query server(id) / services; mutations
// suspendService(id) / resumeService(id) / restartServer(id); type Service with
// the string `suspended` enum. Every resolver delegates to the Service — the
// schema is presentation, the behavior is shared with REST and MCP.

var serviceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Service",
	Fields: graphql.Fields{
		// Render-shaped fields (id is the App name; type is always web_service).
		"id":           &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Name })},
		"name":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Name })},
		"type":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return renderWebService })},
		"suspended":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return core.SuspendedEnum(a.Suspended) })},
		"dashboardUrl": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.URL })},
		"url":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.URL })},
		"createdAt":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.CreatedAt })},
		// bex-native extras.
		"phase":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Phase })},
		"replicas": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(a AppView) any { return a.Replicas })},
		"revision": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Revision })},
	},
})

// GraphQLQuery returns the App read fields (Render dashboard names services /
// server(id)) for the composition root to merge into the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"services": &graphql.Field{
			Type:    graphql.NewList(serviceGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.List(p.Context) },
		},
		"server": &graphql.Field{ // Render's dashboard query name
			Type: serviceGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, p.Args["id"].(string))
			},
		},
	}
}

// GraphQLMutation returns the lifecycle mutations (Render dashboard names
// suspendService / resumeService / restartServer).
func (s *Service) GraphQLMutation() graphql.Fields {
	verb := func(fn func(context.Context, string) (AppView, error)) graphql.FieldResolveFn {
		return func(p graphql.ResolveParams) (any, error) {
			return fn(p.Context, p.Args["id"].(string))
		}
	}
	return graphql.Fields{
		"suspendService": &graphql.Field{Type: serviceGQLType, Args: gqlutil.IDArg(), Resolve: verb(s.Suspend)},
		"resumeService":  &graphql.Field{Type: serviceGQLType, Args: gqlutil.IDArg(), Resolve: verb(s.Resume)},
		"restartServer":  &graphql.Field{Type: serviceGQLType, Args: gqlutil.IDArg(), Resolve: verb(s.Restart)},
	}
}
