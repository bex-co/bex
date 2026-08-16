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

// auditMetadataGQLType stays deliberately closed to the maintenance `to`
// flag: REST's richer kind-keyed target metadata (w4/m26) is not mirrored
// here — the dashboard's one target need, the display name, ships as the
// flat `targetName` field on AuditLog below (w10/m5, the consumer the
// original scope cut waited for).
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
		"action":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return renderEvent(e.Verb) })},
		// status keeps bex's richer denied value here: GraphQL is bex's own
		// dialect (Render has no public GraphQL), and the dashboard's badge
		// distinguishes a denial from an error. REST maps it onto Render's
		// closed success|error enum (renderStatus).
		"status": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any {
			if denied(e) {
				return "denied"
			}
			return "success"
		})},
		"resource": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any { return e.Resource })},
		// targetName is the target's stored display name (migration 0038);
		// null for pre-0038 rows (stored "") so the dashboard falls back to
		// the raw id rather than rendering an empty cell.
		"targetName": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any {
			if e.TargetName == "" {
				return nil
			}
			return e.TargetName
		})},
		"metadata": &graphql.Field{Type: auditMetadataGQLType, Resolve: gqlutil.Field(func(e Event) any {
			if e.MaintenanceModeTo == nil && e.Verb != core.AuditVerbMaintenanceModeURIUpdated {
				return nil
			}
			return e
		})},
	},
})

// GraphQLQuery contributes `auditLogs(ownerId, startTime, endTime, direction,
// cursor, limit)` to the root Query. direction is validated by the verb
// (Service.List), so an unknown value errors identically over REST and here.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"auditLogs": &graphql.Field{
			Type: graphql.NewList(auditLogGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"startTime": &graphql.ArgumentConfig{Type: graphql.String},
				"endTime":   &graphql.ArgumentConfig{Type: graphql.String},
				"direction": &graphql.ArgumentConfig{Type: graphql.String},
				"cursor":    &graphql.ArgumentConfig{Type: graphql.String},
				"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				f, err := FilterOf(
					gqlutil.Str(p.Args, "startTime"), gqlutil.Str(p.Args, "endTime"),
					gqlutil.Str(p.Args, "direction"), gqlutil.Str(p.Args, "cursor"), gqlutil.Int(p.Args, "limit"),
				)
				if err != nil {
					return nil, err
				}
				return s.List(p.Context, p.Args["ownerId"].(string), f)
			},
		},
	}
}
