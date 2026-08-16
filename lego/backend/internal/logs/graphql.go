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

	"github.com/bex-co/bex/lego/backend/internal/core"
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
		"timestamp":  gqlutil.StrField(func(e LogEntry) any { return e.Timestamp }),
		"message":    gqlutil.StrField(func(e LogEntry) any { return e.Message }),
		"type":       gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelType] }),
		"instance":   gqlutil.StrField(func(e LogEntry) any { return e.Labels["instance"] }),
		"level":      gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelLevel] }),
		"method":     gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelMethod] }),
		"statusCode": gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelStatusCode] }),
	},
})

// logFilterArgs are the filter arguments logs() and logLabelValues() share — the
// same vocabulary as REST/MCP. `resource` may name an App or managed Postgres;
// the datastore path accepts its documented range/text/instance subset and
// refuses service-only filters. `type` and `text` stay single-valued strings
// (the shape the dashboard's query already sends); request filters are lists.
func logFilterArgs() graphql.FieldConfigArgument {
	list := gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String)))
	return graphql.FieldConfigArgument{
		"resource":   gqlutil.ReqArg(graphql.String),
		"type":       gqlutil.Arg(graphql.String),
		"text":       gqlutil.Arg(graphql.String),
		"level":      list,
		"instance":   list,
		"host":       list,
		"statusCode": list,
		"method":     list,
		"path":       list,
		"direction":  gqlutil.Arg(graphql.String),
		// startTime/endTime (w9/m1/t002): the RFC3339 window REST (?startTime=…&
		// endTime=…) and MCP (list_logs) already accept — the deploy detail page
		// scopes its log query to a deploy's createdAt..finishedAt window. Absent
		// ⇒ prior unbounded behavior; BEX_MAX_QUERY_HOURS is enforced by the
		// resolvers below (s.checkWindow), the same call REST's logsQuery/
		// logsValues make — not duplicated here as a second cap.
		"startTime": gqlutil.Arg(graphql.String),
		"endTime":   gqlutil.Arg(graphql.String),
	}
}

// GraphQLQuery returns the logs read + label-discovery fields for the composition
// root's Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	logsArgs := logFilterArgs()
	logsArgs["limit"] = gqlutil.Arg(graphql.Int)

	valuesArgs := logFilterArgs()
	valuesArgs["label"] = gqlutil.ReqArg(graphql.String)

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
				if err := s.checkWindow(q); err != nil {
					return nil, err
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
				if err := s.checkWindow(q); err != nil {
					return nil, err
				}
				label := gqlutil.Str(p.Args, "label")
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
	startTime, _ := args["startTime"].(string)
	endTime, _ := args["endTime"].(string)
	since, end, err := core.ParseTimeWindow(startTime, endTime)
	if err != nil {
		return LogQuery{}, err
	}
	q.Since, q.End = since, end
	return q, nil
}
