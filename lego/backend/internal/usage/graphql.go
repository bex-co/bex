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

package usage

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// graphql.go is the usage GraphQL fragment. The `usage` query is a bex
// dashboard companion alongside `monthToDateBandwidth` — workspace-scoped (no
// per-resource filter needed) and returning the same data as GET /v1/usage.

var usageRowGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "UsageRow",
	Fields: graphql.Fields{
		"kind":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r store.UsageSummaryRow) any { return r.Kind })},
		"tier":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r store.UsageSummaryRow) any { return r.Tier })},
		"total": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(r store.UsageSummaryRow) any { return r.Total })},
	},
})

var serviceUsageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceUsage",
	Fields: graphql.Fields{
		"serviceId": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(s ServiceUsage) any { return s.ServiceID })},
		"rows":      &graphql.Field{Type: graphql.NewList(usageRowGQLType), Resolve: gqlutil.Field(func(s ServiceUsage) any { return s.Rows })},
	},
})

var usageSummaryGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "UsageSummary",
	Fields: graphql.Fields{
		"workspaceId": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(s Summary) any { return s.WorkspaceID })},
		"services":    &graphql.Field{Type: graphql.NewList(serviceUsageGQLType), Resolve: gqlutil.Field(func(s Summary) any { return s.Services })},
	},
})

// GraphQLQuery contributes the `usage` query to the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"usage": &graphql.Field{
			Type: usageSummaryGQLType,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.MonthToDate(p.Context)
			},
		},
	}
}
