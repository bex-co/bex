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
		"type":         &graphql.Field{Type: graphql.String, Resolve: appField(func(a AppView) any { return "web_service" })},
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
			},
		}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name: "Mutation",
			Fields: graphql.Fields{
				"suspendService": &graphql.Field{Type: serviceGQLType, Args: idArg, Resolve: verb((*Core).Suspend)},
				"resumeService":  &graphql.Field{Type: serviceGQLType, Args: idArg, Resolve: verb((*Core).Resume)},
				"restartServer":  &graphql.Field{Type: serviceGQLType, Args: idArg, Resolve: verb((*Core).Restart)},
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
