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

package audit

import (
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go contributes the `auditLogs` root query — named to match the REST
// noun (t003), the dashboard's read path for the workspace Audit Log page.
// Resolves through the same List verb as REST, so the two surfaces can't
// diverge (t007 asserts this).

var auditMetadataGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AuditLogMetadata",
	Fields: graphql.Fields{
		"to": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(e Event) any {
			if e.MaintenanceModeTo == nil {
				return nil
			}
			return *e.MaintenanceModeTo
		})},
	},
})

var auditLogGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AuditLog",
	Fields: graphql.Fields{
		"id":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.ID })},
		"timestamp":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.At.UTC().Format(time.RFC3339) })},
		"actor":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.Caller })},
		"actorMethod": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.CallerMethod })},
		"action":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return renderAction(e.Verb) })},
		"status":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return renderStatus(e.Outcome) })},
		"resource":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.Resource })},
		"metadata": &graphql.Field{Type: auditMetadataGQLType, Resolve: gqlutil.Field(func(e Event) any {
			if e.MaintenanceModeTo == nil && e.Verb != core.AuditVerbMaintenanceModeURIUpdated {
				return nil
			}
			return e
		})},
	},
})

// GraphQLQuery contributes `auditLogs(ownerId, startTime, endTime, cursor,
// limit)` to the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"auditLogs": &graphql.Field{
			Type: graphql.NewList(auditLogGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"startTime": &graphql.ArgumentConfig{Type: graphql.String},
				"endTime":   &graphql.ArgumentConfig{Type: graphql.String},
				"cursor":    &graphql.ArgumentConfig{Type: graphql.String},
				"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				var f Filter
				if v, _ := p.Args["startTime"].(string); v != "" {
					if t, err := time.Parse(time.RFC3339, v); err == nil {
						f.Since = t
					}
				}
				if v, _ := p.Args["endTime"].(string); v != "" {
					if t, err := time.Parse(time.RFC3339, v); err == nil {
						f.Until = t
					}
				}
				f.Cursor, _ = p.Args["cursor"].(string)
				if v, ok := p.Args["limit"].(int); ok {
					f.Limit = v
				}
				return s.List(p.Context, p.Args["ownerId"].(string), f)
			},
		},
	}
}
