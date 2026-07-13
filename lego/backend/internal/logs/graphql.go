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

package logs

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// logGQLType is the GraphQL projection of a LogEntry — a flat row (the REST
// adapter renders the same data as Render's labels array instead). The row's
// fields come from the entry's labels, so a request line reports type=request with
// its method/statusCode, and an app line its level — the split the dashboard's
// Logs tab renders.
var logGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LogEntry",
	Fields: graphql.Fields{
		"timestamp":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return e.Timestamp })},
		"message":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return e.Message })},
		"type":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return e.Labels[LabelType] })},
		"instance":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return e.Labels["instance"] })},
		"level":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return e.Labels[LabelLevel] })},
		"method":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return e.Labels[LabelMethod] })},
		"statusCode": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return e.Labels[LabelStatusCode] })},
	},
})

// logFilterArgs are the filter arguments logs() and logLabelValues() share — the
// same vocabulary as REST/MCP. `type` and `text` stay single-valued strings (the
// shape the dashboard's query already sends); the request filters are lists, since
// Render's are.
func logFilterArgs() graphql.FieldConfigArgument {
	list := &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))}
	return graphql.FieldConfigArgument{
		"resource":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		"type":       &graphql.ArgumentConfig{Type: graphql.String},
		"text":       &graphql.ArgumentConfig{Type: graphql.String},
		"level":      list,
		"instance":   list,
		"host":       list,
		"statusCode": list,
		"method":     list,
		"path":       list,
		"direction":  &graphql.ArgumentConfig{Type: graphql.String},
	}
}

// GraphQLQuery returns the logs read + label-discovery fields for the composition
// root's Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	logsArgs := logFilterArgs()
	logsArgs["limit"] = &graphql.ArgumentConfig{Type: graphql.Int}

	valuesArgs := logFilterArgs()
	valuesArgs["label"] = &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}

	return graphql.Fields{
		"logs": &graphql.Field{
			Type: graphql.NewList(logGQLType),
			Args: logsArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				q, err := logQueryFromArgs(p.Args)
				if err != nil {
					return nil, err
				}
				if n, ok := p.Args["limit"].(int); ok {
					q.Limit = int64(n)
				}
				return s.QueryLogs(p.Context, q)
			},
		},
		// The dashboard's filter dropdowns (and any GraphQL client) discover real
		// values here — the logs sibling of metricsFilters.
		"logLabelValues": &graphql.Field{
			Type: graphql.NewList(graphql.NewNonNull(graphql.String)),
			Args: valuesArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				q, err := logQueryFromArgs(p.Args)
				if err != nil {
					return nil, err
				}
				label, _ := p.Args["label"].(string)
				return s.LogLabelValues(p.Context, label, q)
			},
		},
	}
}

// logQueryFromArgs maps the GraphQL filter args onto a LogQuery. An unknown type
// or direction errors (naming the value) rather than widening the query silently —
// the same contract the REST adapter enforces.
func logQueryFromArgs(args map[string]any) (LogQuery, error) {
	q := LogQuery{App: args["resource"].(string)}
	if t, ok := args["type"].(string); ok {
		types, err := NormalizeTypes([]string{t})
		if err != nil {
			return LogQuery{}, err
		}
		q.Types = types
	}
	if s, ok := args["text"].(string); ok {
		q.Search = s
	}
	// Direction is validated by the verb (LogQuery.validate), not here — one
	// enforcement point for all three surfaces.
	q.Direction, _ = args["direction"].(string)
	q.Level = gqlutil.StringList(args["level"])
	q.Instance = gqlutil.StringList(args["instance"])
	q.Host = gqlutil.StringList(args["host"])
	q.StatusCode = gqlutil.StringList(args["statusCode"])
	q.Method = gqlutil.StringList(args["method"])
	q.Path = gqlutil.StringList(args["path"])
	return q, nil
}
