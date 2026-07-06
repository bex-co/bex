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
	"time"

	"github.com/graphql-go/graphql"
)

// graphql.go is the GraphQL adapter, matching the operation names Render's
// dashboard uses (captured live): query server(id) / services; mutations
// suspendService(id) / resumeService(id) / restartServer(id); type Service with
// the string `suspended` enum. Every resolver delegates to Core — the schema is
// presentation, the behavior is shared with REST.

// gqlField adapts a typed projection into a GraphQL resolver: it type-asserts
// the source and applies f, resolving nil for foreign sources. One helper for
// every object type (AppView, PostgresView, connection info, logs, API keys).
func gqlField[T any](f func(T) any) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		v, ok := p.Source.(T)
		if !ok {
			return nil, nil
		}
		return f(v), nil
	}
}

var serviceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Service",
	Fields: graphql.Fields{
		// Render-shaped fields (id is the App name; type is always web_service).
		"id":           &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.Name })},
		"name":         &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.Name })},
		"type":         &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return renderWebService })},
		"suspended":    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return suspendedEnum(a.Suspended) })},
		"dashboardUrl": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.URL })},
		"url":          &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.URL })},
		"createdAt":    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.CreatedAt })},
		// bex-native extras.
		"phase":    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.Phase })},
		"replicas": &graphql.Field{Type: graphql.Int, Resolve: gqlField(func(a AppView) any { return a.Replicas })},
		"revision": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.Revision })},
	},
})

func suspendedEnum(b bool) string {
	if b {
		return renderSuspended
	}
	return renderNotSuspended
}

// Render's dashboard GraphQL calls a managed Postgres a "database" (query
// database(id), databaseStatusQuery, databaseCredentialList, ...) — captured
// live — even though its REST noun is "postgres". bex mirrors that split: REST
// /v1/postgres, GraphQL database* (which also matches bex's own Database CRD).
var postgresGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Database",
	Fields: graphql.Fields{
		"id":                      &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.ID })},
		"name":                    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.Name })},
		"plan":                    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.Plan })},
		"version":                 &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.Version })},
		"status":                  &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.Status })},
		"databaseName":            &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.DatabaseName })},
		"databaseUser":            &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.DatabaseUser })},
		"diskSizeGB":              &graphql.Field{Type: graphql.Int, Resolve: gqlField(func(v PostgresView) any { return v.DiskSizeGB })},
		"highAvailabilityEnabled": &graphql.Field{Type: graphql.Boolean, Resolve: gqlField(func(v PostgresView) any { return v.HighAvailabilityEnabled })},
		"suspended":               &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.Suspended })},
		"createdAt":               &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.CreatedAt })},
		"externalHost":            &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.ExternalHost })},
		"public":                  &graphql.Field{Type: graphql.Boolean, Resolve: gqlField(func(v PostgresView) any { return v.Public })},
	},
})

var connectionInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PostgresConnectionInfo",
	Fields: graphql.Fields{
		"password":                 &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresConnectionInfo) any { return v.Password })},
		"internalConnectionString": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresConnectionInfo) any { return v.InternalConnectionString })},
		"externalConnectionString": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresConnectionInfo) any { return v.ExternalConnectionString })},
		"psqlCommand":              &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresConnectionInfo) any { return v.PSQLCommand })},
	},
})

// apiKeyGQLType mirrors the REST APIKey object (bex extension; Render's
// dashboard manages keys outside its public schemas). secret is non-empty only
// in the createApiKey payload — list resolves it empty.
var apiKeyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ApiKey",
	Fields: graphql.Fields{
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlField(func(k APIKey) any { return k.ID })},
		"name":      &graphql.Field{Type: graphql.String, Resolve: gqlField(func(k APIKey) any { return k.Name })},
		"secret":    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(k APIKey) any { return k.Secret })},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(k APIKey) any { return k.CreatedAt })},
	},
})

// logGQLType is the GraphQL projection of a LogEntry — a flat row (the REST
// adapter renders the same data as Render's labels array instead). type is
// Render's `app` value; instance comes from the entry's labels.
var logGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LogEntry",
	Fields: graphql.Fields{
		"timestamp": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(e LogEntry) any { return e.Timestamp })},
		"message":   &graphql.Field{Type: graphql.String, Resolve: gqlField(func(e LogEntry) any { return e.Message })},
		"type":      &graphql.Field{Type: graphql.String, Resolve: gqlField(func(e LogEntry) any { return renderLogTypeApp })},
		"instance":  &graphql.Field{Type: graphql.String, Resolve: gqlField(func(e LogEntry) any { return e.Labels["instance"] })},
	},
})

// logQueryFromArgs maps the GraphQL logs() args onto a Core LogQuery, accepting
// the same `app` alias for the application type as the REST adapter.
func logQueryFromArgs(args map[string]any) LogQuery {
	q := LogQuery{App: args["resource"].(string)}
	if t, ok := args["type"].(string); ok {
		switch t {
		case renderLogTypeApp, LogTypeApplication:
			q.Type = LogTypeApplication
		case LogTypeRequest:
			q.Type = LogTypeRequest
		case LogTypeBuild:
			q.Type = LogTypeBuild
		}
	}
	if s, ok := args["text"].(string); ok {
		q.Search = s
	}
	if n, ok := args["limit"].(int); ok {
		q.Limit = int64(n)
	}
	return q
}

// --- Metrics GraphQL types (Render metrics shape, flat rows) ---

var metricPointGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetricPoint",
	Fields: graphql.Fields{
		"timestamp": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MetricPoint).Timestamp, nil
		}},
		"value": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MetricPoint).Value, nil
		}},
	},
})

var metricLabelGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetricLabel",
	Fields: graphql.Fields{
		"field": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(renderMetricLabel).Field, nil
		}},
		"value": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(renderMetricLabel).Value, nil
		}},
	},
})

// metricSeriesGQLType projects Core's MetricSeries; labels are exposed as Render's
// sorted {field,value} array (reusing the REST mapper) so both surfaces agree.
var metricSeriesGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetricSeries",
	Fields: graphql.Fields{
		"unit": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MetricSeries).Unit, nil
		}},
		"labels": &graphql.Field{Type: graphql.NewList(metricLabelGQLType), Resolve: func(p graphql.ResolveParams) (any, error) {
			return toRenderMetricSeries(p.Source.(MetricSeries)).Labels, nil
		}},
		"points": &graphql.Field{Type: graphql.NewList(metricPointGQLType), Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MetricSeries).Points, nil
		}},
	},
})

// metricQueryFromArgs maps the GraphQL metrics() args onto a Core MetricQuery,
// accepting the same vocabulary as the REST adapter.
func metricQueryFromArgs(args map[string]any) MetricQuery {
	q := MetricQuery{App: args["resource"].(string), Metric: args["metric"].(string)}
	if s, ok := args["startTime"].(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			q.Start = t
		}
	}
	if e, ok := args["endTime"].(string); ok {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			q.End = t
		}
	}
	if n, ok := args["resolutionSeconds"].(int); ok {
		q.Resolution = time.Duration(n) * time.Second
	}
	if f, ok := args["quantile"].(float64); ok {
		q.Quantile = f
	}
	if b, ok := args["percentage"].(bool); ok {
		q.Percentage = b
	}
	if s, ok := args["statusCode"].(string); ok {
		q.StatusCode = s
	}
	if s, ok := args["host"].(string); ok {
		q.Host = s
	}
	if s, ok := args["path"].(string); ok {
		q.Path = s
	}
	if s, ok := args["groupBy"].(string); ok {
		q.GroupBy = s
	}
	return q
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
				"apiKeys": &graphql.Field{
					Type:    graphql.NewList(apiKeyGQLType),
					Resolve: func(p graphql.ResolveParams) (any, error) { return coreFrom(p.Context).ListAPIKeys(p.Context) },
				},
				"logs": &graphql.Field{
					Type: graphql.NewList(logGQLType),
					Args: graphql.FieldConfigArgument{
						"resource": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
						"type":     &graphql.ArgumentConfig{Type: graphql.String},
						"text":     &graphql.ArgumentConfig{Type: graphql.String},
						"limit":    &graphql.ArgumentConfig{Type: graphql.Int},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).QueryLogs(p.Context, logQueryFromArgs(p.Args))
					},
				},
				"metrics": &graphql.Field{
					Type: graphql.NewList(metricSeriesGQLType),
					Args: graphql.FieldConfigArgument{
						"resource":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
						"metric":            &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, // cpu|memory|instance_count|http_requests|http_latency|bandwidth
						"startTime":         &graphql.ArgumentConfig{Type: graphql.String},
						"endTime":           &graphql.ArgumentConfig{Type: graphql.String},
						"resolutionSeconds": &graphql.ArgumentConfig{Type: graphql.Int},
						"quantile":          &graphql.ArgumentConfig{Type: graphql.Float},
						"percentage":        &graphql.ArgumentConfig{Type: graphql.Boolean},
						"statusCode":        &graphql.ArgumentConfig{Type: graphql.String},
						"host":              &graphql.ArgumentConfig{Type: graphql.String},
						"path":              &graphql.ArgumentConfig{Type: graphql.String},
						"groupBy":           &graphql.ArgumentConfig{Type: graphql.String},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).Metrics(p.Context, metricQueryFromArgs(p.Args))
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
				"createApiKey": &graphql.Field{
					Type: apiKeyGQLType,
					Args: graphql.FieldConfigArgument{
						"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).CreateAPIKey(p.Context, p.Args["name"].(string))
					},
				},
				"revokeApiKey": &graphql.Field{
					Type: graphql.Boolean,
					Args: idArg,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						err := coreFrom(p.Context).RevokeAPIKey(p.Context, p.Args["id"].(string))
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
