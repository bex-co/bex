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

package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/graphql-go/graphql"
)

// graphql.go is the GraphQL adapter, matching the operation names Render's
// dashboard uses (captured live): query server(id) / services; mutations
// suspendService(id) / resumeService(id) / restartServer(id); type Service with
// the string `suspended` enum. Every resolver delegates to Core — the schema is
// presentation, the behavior is shared with REST.

func appField(f func(AppView) any) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		a, ok := p.Source.(AppView)
		if !ok {
			return nil, nil
		}
		return f(a), nil
	}
}

var serviceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Service",
	Fields: graphql.Fields{
		// Render-shaped fields (id is the App name; type is always web_service).
		"id":           &graphql.Field{Type: graphql.String, Resolve: appField(func(a AppView) any { return a.Name })},
		"name":         &graphql.Field{Type: graphql.String, Resolve: appField(func(a AppView) any { return a.Name })},
		"type":         &graphql.Field{Type: graphql.String, Resolve: appField(func(a AppView) any { return renderWebService })},
		"suspended":    &graphql.Field{Type: graphql.String, Resolve: appField(func(a AppView) any { return suspendedEnum(a.Suspended) })},
		"dashboardUrl": &graphql.Field{Type: graphql.String, Resolve: appField(func(a AppView) any { return a.URL })},
		"url":          &graphql.Field{Type: graphql.String, Resolve: appField(func(a AppView) any { return a.URL })},
		"createdAt":    &graphql.Field{Type: graphql.String, Resolve: appField(func(a AppView) any { return a.CreatedAt })},
		// bex-native extras.
		"phase":    &graphql.Field{Type: graphql.String, Resolve: appField(func(a AppView) any { return a.Phase })},
		"replicas": &graphql.Field{Type: graphql.Int, Resolve: appField(func(a AppView) any { return a.Replicas })},
		"revision": &graphql.Field{Type: graphql.String, Resolve: appField(func(a AppView) any { return a.Revision })},
	},
})

func suspendedEnum(b bool) string {
	if b {
		return renderSuspended
	}
	return renderNotSuspended
}

func pgField(f func(PostgresView) any) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		v, ok := p.Source.(PostgresView)
		if !ok {
			return nil, nil
		}
		return f(v), nil
	}
}

func ciField(f func(PostgresConnectionInfo) any) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		v, ok := p.Source.(PostgresConnectionInfo)
		if !ok {
			return nil, nil
		}
		return f(v), nil
	}
}

// Render's dashboard GraphQL calls a managed Postgres a "database" (query
// database(id), databaseStatusQuery, databaseCredentialList, ...) — captured
// live — even though its REST noun is "postgres". bex mirrors that split: REST
// /v1/postgres, GraphQL database* (which also matches bex's own Database CRD).
var postgresGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Database",
	Fields: graphql.Fields{
		"id":                      &graphql.Field{Type: graphql.String, Resolve: pgField(func(v PostgresView) any { return v.ID })},
		"name":                    &graphql.Field{Type: graphql.String, Resolve: pgField(func(v PostgresView) any { return v.Name })},
		"plan":                    &graphql.Field{Type: graphql.String, Resolve: pgField(func(v PostgresView) any { return v.Plan })},
		"version":                 &graphql.Field{Type: graphql.String, Resolve: pgField(func(v PostgresView) any { return v.Version })},
		"status":                  &graphql.Field{Type: graphql.String, Resolve: pgField(func(v PostgresView) any { return v.Status })},
		"databaseName":            &graphql.Field{Type: graphql.String, Resolve: pgField(func(v PostgresView) any { return v.DatabaseName })},
		"databaseUser":            &graphql.Field{Type: graphql.String, Resolve: pgField(func(v PostgresView) any { return v.DatabaseUser })},
		"diskSizeGB":              &graphql.Field{Type: graphql.Int, Resolve: pgField(func(v PostgresView) any { return v.DiskSizeGB })},
		"highAvailabilityEnabled": &graphql.Field{Type: graphql.Boolean, Resolve: pgField(func(v PostgresView) any { return v.HighAvailabilityEnabled })},
		"suspended":               &graphql.Field{Type: graphql.String, Resolve: pgField(func(v PostgresView) any { return v.Suspended })},
		"createdAt":               &graphql.Field{Type: graphql.String, Resolve: pgField(func(v PostgresView) any { return v.CreatedAt })},
		"externalHost":            &graphql.Field{Type: graphql.String, Resolve: pgField(func(v PostgresView) any { return v.ExternalHost })},
		"public":                  &graphql.Field{Type: graphql.Boolean, Resolve: pgField(func(v PostgresView) any { return v.Public })},
	},
})

var connectionInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PostgresConnectionInfo",
	Fields: graphql.Fields{
		"password":                 &graphql.Field{Type: graphql.String, Resolve: ciField(func(v PostgresConnectionInfo) any { return v.Password })},
		"internalConnectionString": &graphql.Field{Type: graphql.String, Resolve: ciField(func(v PostgresConnectionInfo) any { return v.InternalConnectionString })},
		"externalConnectionString": &graphql.Field{Type: graphql.String, Resolve: ciField(func(v PostgresConnectionInfo) any { return v.ExternalConnectionString })},
		"psqlCommand":              &graphql.Field{Type: graphql.String, Resolve: ciField(func(v PostgresConnectionInfo) any { return v.PSQLCommand })},
	},
})

// newSchema builds the schema once; the Core is injected per-request via context.
func newSchema() (graphql.Schema, error) {
	idArg := graphql.FieldConfigArgument{
		"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
	}
	verb := func(fn func(*Core, context.Context, string) (AppView, error)) graphql.FieldResolveFn {
		return func(p graphql.ResolveParams) (any, error) {
			return fn(coreFrom(p.Context), p.Context, p.Args["id"].(string))
		}
	}

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"services": &graphql.Field{
					Type:    graphql.NewList(serviceGQLType),
					Resolve: func(p graphql.ResolveParams) (any, error) { return coreFrom(p.Context).List(p.Context) },
				},
				"server": &graphql.Field{ // Render's dashboard query name
					Type: serviceGQLType,
					Args: idArg,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).Get(p.Context, p.Args["id"].(string))
					},
				},
				"databases": &graphql.Field{ // list; Render lists via env, bex offers a top-level list
					Type:    graphql.NewList(postgresGQLType),
					Resolve: func(p graphql.ResolveParams) (any, error) { return coreFrom(p.Context).ListPostgres(p.Context) },
				},
				"database": &graphql.Field{ // Render's dashboard query name
					Type: postgresGQLType,
					Args: idArg,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).GetPostgres(p.Context, p.Args["id"].(string))
					},
				},
				"databaseConnectionInfo": &graphql.Field{
					Type: connectionInfoGQLType,
					Args: idArg,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).PostgresConnectionInfo(p.Context, p.Args["id"].(string))
					},
				},
			},
		}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name: "Mutation",
			Fields: graphql.Fields{
				"suspendService": &graphql.Field{Type: serviceGQLType, Args: idArg, Resolve: verb((*Core).Suspend)},
				"resumeService":  &graphql.Field{Type: serviceGQLType, Args: idArg, Resolve: verb((*Core).Resume)},
				"restartServer":  &graphql.Field{Type: serviceGQLType, Args: idArg, Resolve: verb((*Core).Restart)},
				"createDatabase": &graphql.Field{
					Type: postgresGQLType,
					Args: graphql.FieldConfigArgument{
						"name":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
						"plan":       &graphql.ArgumentConfig{Type: graphql.String},
						"version":    &graphql.ArgumentConfig{Type: graphql.String},
						"diskSizeGB": &graphql.ArgumentConfig{Type: graphql.Int},
						"public":     &graphql.ArgumentConfig{Type: graphql.Boolean},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						req := CreatePostgresRequest{Name: p.Args["name"].(string)}
						if v, ok := p.Args["plan"].(string); ok {
							req.Plan = v
						}
						if v, ok := p.Args["version"].(string); ok {
							req.Version = v
						}
						if v, ok := p.Args["diskSizeGB"].(int); ok {
							req.DiskSizeGB = int32(v)
						}
						if v, ok := p.Args["public"].(bool); ok {
							req.Public = v
						}
						return coreFrom(p.Context).CreatePostgres(p.Context, req)
					},
				},
				"deleteDatabase": &graphql.Field{
					Type: graphql.Boolean,
					Args: idArg,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						err := coreFrom(p.Context).DeletePostgres(p.Context, p.Args["id"].(string))
						return err == nil, err
					},
				},
			},
		}),
	})
}

type coreCtxKey struct{}

func coreFrom(ctx context.Context) *Core { return ctx.Value(coreCtxKey{}).(*Core) }

// graphqlHandler serves POST /graphql, injecting the Core per request so the
// compiled schema is reused.
func (s *Server) graphqlHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query         string         `json:"query"`
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		result := graphql.Do(graphql.Params{
			Schema:         s.schema,
			RequestString:  body.Query,
			OperationName:  body.OperationName,
			VariableValues: body.Variables,
			Context:        context.WithValue(r.Context(), coreCtxKey{}, s.Core),
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
}
