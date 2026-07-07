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
// adapter renders the same data as Render's labels array instead). type is
// Render's `app` value; instance comes from the entry's labels.
var logGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LogEntry",
	Fields: graphql.Fields{
		"timestamp": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return e.Timestamp })},
		"message":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return e.Message })},
		"type":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return renderLogTypeApp })},
		"instance":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e LogEntry) any { return e.Labels["instance"] })},
	},
})

// GraphQLQuery returns the logs read field for the composition root's Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"logs": &graphql.Field{
			Type: graphql.NewList(logGQLType),
			Args: graphql.FieldConfigArgument{
				"resource": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"type":     &graphql.ArgumentConfig{Type: graphql.String},
				"text":     &graphql.ArgumentConfig{Type: graphql.String},
				"limit":    &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.QueryLogs(p.Context, logQueryFromArgs(p.Args))
			},
		},
	}
}

// logQueryFromArgs maps the GraphQL logs() args onto a LogQuery, accepting the
// same `app` alias for the application type as the REST adapter.
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
