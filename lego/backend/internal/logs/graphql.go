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
	"errors"
	"fmt"
	"slices"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
)

// logGQLType is the GraphQL projection of a LogEntry — a flat row (the REST
// adapter renders the same data as Render's labels array instead). The row's
// fields come from the entry's labels, so a request line reports type=request with
// its method/statusCode, and an app line its level — the split the dashboard's
// Logs tab renders.
var logGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LogEntry",
	Fields: graphql.Fields{
		// id is the record's identity — REST's per-record id (instance,
		// original timestamp, message hash) — so a client can keep two records
		// that share a millisecond apart and recognize a replay of one (w4/m217).
		"id":        gqlutil.StrField(func(e LogEntry) any { return logID(e) }),
		"timestamp": gqlutil.StrField(func(e LogEntry) any { return e.Timestamp }),
		// resource is the line's resource id (REST's `resource` label, Render's
		// SERVICE): a project view's lines come from several (w3/m505).
		"resource":   gqlutil.StrField(func(e LogEntry) any { return e.Labels["service"] }),
		"message":    gqlutil.StrField(func(e LogEntry) any { return e.Message }),
		"type":       gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelType] }),
		"instance":   gqlutil.StrField(func(e LogEntry) any { return e.Labels["instance"] }),
		"level":      gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelLevel] }),
		"method":     gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelMethod] }),
		"statusCode": gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelStatusCode] }),
		// A request line's host and path, as Render labels them (w4/m325).
		"host": gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelHost] }),
		"path": gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelPath] }),
		// A request line's edge-minted Rndr-Id (w8/m125); empty for app lines.
		"requestId": gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelRequestID] }),
		// A build line's build (w8/m442, w2/m611): a service build's dep- id or
		// a Build Source build's bld- id; empty for other lines.
		"build": gqlutil.StrField(func(e LogEntry) any { return e.Labels[LabelBuild] }),
	},
})

// logListGQLType is Render's logs envelope ({hasMore, nextStartTime,
// nextEndTime, logs}) — the same shape REST returns, so GraphQL callers can
// page and tell a capped result from a complete one (w4/m107).
var logListGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LogList",
	Fields: graphql.Fields{
		"hasMore":       &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
		"nextStartTime": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		"nextEndTime":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		"logs":          &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(logGQLType))},
		// resources is what a project-scoped read read (w3/m505): the
		// resources of the view the caller may read the logs of. Null for a
		// `resource` read.
		"resources": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(logResourceGQLType))},
	},
})

// logResourceGQLType is one resource a project view read (w3/m505, a Bex
// extension): its id, shown name, type (a service type, postgres or
// key_value) and environment, so a client names each line's resource.
var logResourceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LogResource",
	Fields: graphql.Fields{
		"id":            gqlutil.ReqStrField(func(r savedsearch.Resource) any { return r.ID }),
		"name":          gqlutil.ReqStrField(func(r savedsearch.Resource) any { return r.Name }),
		"type":          gqlutil.ReqStrField(func(r savedsearch.Resource) any { return r.Type }),
		"environmentId": gqlutil.OptionalStrField(func(r savedsearch.Resource) any { return r.EnvironmentID }),
	},
})

// logRetentionGQLType is one resource's log window (w9/m389, a Bex extension
// the dashboard bounds its range presets and custom picker by): the workspace
// plan's core.LogRetention, at which every REST, GraphQL and MCP read is
// floored.
var logRetentionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LogRetention",
	Fields: graphql.Fields{
		// plan is the resource's workspace plan; days how far back it keeps logs.
		"plan": gqlutil.ReqStrField(func(r *RetentionWindow) any { return r.Plan }),
		"days": gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(r *RetentionWindow) any { return r.Days }),
		// visibleFrom is the floor an upgrade kept while it is later than days
		// back from now (RFC3339); null otherwise. The answer holds still between
		// reads: a client slides the days-back floor forward on its own clock.
		"visibleFrom": gqlutil.TimeField(func(r *RetentionWindow) any { return r.VisibleFrom }),
		// upgradePlan/upgradeDays name the cheapest plan that keeps logs longer;
		// null on the longest window.
		"upgradePlan": gqlutil.OptionalStrField(func(r *RetentionWindow) any { return r.UpgradePlan }),
		"upgradeDays": gqlutil.IntField(func(r *RetentionWindow) any {
			if r.UpgradeDays == 0 {
				return nil
			}
			return r.UpgradeDays
		}),
	},
})

// logFilterArgs are the filter arguments logs() and logLabelValues() share — the
// same vocabulary as REST/MCP. `resource` may name an App, managed Postgres, or Key Value;
// datastore logs support range/text/instance and Postgres also supports level;
// both refuse service-only filters. `type` and `text` stay single-valued strings
// (the shape the dashboard's query already sends); request filters are lists.
func logFilterArgs() graphql.FieldConfigArgument {
	list := gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String)))
	return graphql.FieldConfigArgument{
		"resource": gqlutil.ReqArg(graphql.String),
		"type":     gqlutil.Arg(graphql.String),
		"text":     gqlutil.Arg(graphql.String),
		// textMode (w4/m189, a Bex extension): raw (default) | display — whether
		// text matches the stored bytes or the terminal-display text.
		"textMode":   gqlutil.Arg(graphql.String),
		"level":      list,
		"instance":   list,
		"host":       list,
		"statusCode": list,
		"method":     list,
		"path":       list,
		// build (w8/m442): Render's build filter, as REST takes it.
		"build":     list,
		"direction": gqlutil.Arg(graphql.String),
		// startTime/endTime (w9/m1/t002): the RFC3339 window REST (?startTime=…&
		// endTime=…) and MCP (list_logs) already accept — the deploy detail page
		// scopes its log query to a deploy's createdAt..finishedAt window. An
		// absent startTime reads the last hour, Render's default (w5/197); that
		// and BEX_MAX_QUERY_HOURS are applied by the resolvers below
		// (s.renderWindow), the same call REST's logsQuery/logsValues make —
		// not duplicated here.
		"startTime": gqlutil.Arg(graphql.String),
		"endTime":   gqlutil.Arg(graphql.String),
	}
}

// GraphQLQuery returns the logs read + label-discovery fields for the composition
// root's Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	logsArgs := logFilterArgs()
	logsArgs["limit"] = gqlutil.Arg(graphql.Int)
	// project + environment + region + filterDocument (w3/m505, w4/m596): read
	// a project's view or Render's project/environment/region scope; resource
	// narrows a project when filterDocument is absent.
	logsArgs["resource"] = gqlutil.Arg(graphql.String)
	logsArgs["project"] = gqlutil.Arg(graphql.String)
	logsArgs["environment"] = gqlutil.Arg(graphql.String)
	logsArgs["region"] = gqlutil.Arg(graphql.String)
	logsArgs["filterDocument"] = gqlutil.Arg(gqlutil.JSONObject)
	// deployId (w21/m324, a Bex extension): scope a build or predeploy read to
	// one deploy. The time window alone cannot: two deploys that overlap in time
	// share every line inside it. Absent => the window alone, as before.
	logsArgs["deployId"] = gqlutil.Arg(graphql.String)

	valuesArgs := logFilterArgs()
	valuesArgs["label"] = gqlutil.ReqArg(graphql.String)
	// project + environment + region + filterDocument (w3/079, w4/m596): the
	// values a label takes across a project's view or Render's project scope.
	valuesArgs["resource"] = gqlutil.Arg(graphql.String)
	valuesArgs["project"] = gqlutil.Arg(graphql.String)
	valuesArgs["environment"] = gqlutil.Arg(graphql.String)
	valuesArgs["region"] = gqlutil.Arg(graphql.String)
	valuesArgs["filterDocument"] = gqlutil.Arg(gqlutil.JSONObject)

	return graphql.Fields{
		"logs": &graphql.Field{
			Type: logListGQLType,
			Args: logsArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				q, err := logQueryFromArgs(p.Args)
				if err != nil {
					return nil, err
				}
				q.Deploy, _ = p.Args["deployId"].(string)
				if n, ok := p.Args["limit"].(int); ok {
					q.Limit = int64(n)
				}
				scopeReq, useView, err := logScopeFromGQL(p, q.App)
				if err != nil {
					return nil, err
				}
				if err := s.renderWindow(&q); err != nil {
					return nil, err
				}
				if useView {
					page, resources, err := s.queryProjectLogPage(p.Context, scopeReq.Project, p.Args["filterDocument"], q)
					if err != nil {
						return nil, err
					}
					out := logListResult(page)
					out["resources"] = resources
					return out, nil
				}
				if scopeReq.Project != "" {
					ctx := core.WithRequestMemo(core.WithAuditVerb(p.Context, AuditVerbQueryLogs))
					resolved, err := s.resolveLogScope(ctx, scopeReq, &q, nil)
					if err != nil {
						return nil, err
					}
					if q.Limit == 0 {
						q.Limit = projectLogPageSize
					}
					if err := q.validate(); err != nil {
						return nil, err
					}
					var page logPage
					if len(resolved.IDs) == 0 {
						n := q.normalized()
						page = logPage{Entries: []LogEntry{}}
						page.NextStartTime, page.NextEndTime = pageCursors(nil, n.Since, n.End, n.Direction)
					} else {
						page, err = s.queryLogPage(ctx, resolved.IDs, q)
						if err != nil {
							return nil, err
						}
					}
					out := logListResult(page)
					out["resources"] = resolved.Resources
					return out, nil
				}
				page, err := s.queryLogPage(p.Context, []string{q.App}, q)
				if err != nil {
					return nil, err
				}
				return logListResult(page), nil
			},
		},
		// How far back the resource's logs reach on its workspace plan (w9/m389);
		// null where no plan applies.
		"logRetention": gqlutil.KeyVerb(logRetentionGQLType, "resource", s.LogRetention),
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
				scopeReq, useView, err := logScopeFromGQL(p, q.App)
				if err != nil {
					return nil, err
				}
				if err := s.renderWindow(&q); err != nil {
					return nil, err
				}
				label := gqlutil.Str(p.Args, "label")
				if useView {
					return s.queryProjectLabelValues(p.Context, scopeReq.Project, p.Args["filterDocument"], label, q)
				}
				if scopeReq.Project != "" {
					ctx := core.WithRequestMemo(core.WithAuditVerb(p.Context, AuditVerbLogLabelValues))
					if err := checkLabel(label, projectDiscoverableLabels); err != nil {
						return nil, err
					}
					resolved, err := s.resolveLogScope(ctx, scopeReq, &q, nil)
					if err != nil {
						return nil, err
					}
					if err := q.validate(); err != nil {
						return nil, err
					}
					values := []string{}
					for _, r := range resolved.Resources {
						if q.skipDatastoreForLabelValues(r.ID, label) {
							continue
						}
						got, err := s.labelValues(ctx, label, q.forResource(r.ID), projectDiscoverableLabels)
						if err != nil {
							if errors.Is(err, core.ErrForbidden) || errors.Is(err, core.ErrNotFound) {
								continue
							}
							return nil, err
						}
						values = append(values, got...)
					}
					return slices.Compact(slices.Sorted(slices.Values(values))), nil
				}
				return s.LogLabelValues(p.Context, label, q)
			},
		},
	}
}

// logScopeFromGQL parses resource/project/environment/region/filterDocument on
// GraphQL logs() and logLabelValues(). A w3/m505 saved-view read (project with
// filterDocument, or bare project with no environment/region/resource) keeps
// the view path; adding environment, region or resource alongside project uses
// Render's scope rules (w4/m596).
func logScopeFromGQL(p graphql.ResolveParams, resource string) (savedsearch.ScopeRequest, bool, error) {
	project, _ := p.Args["project"].(string)
	env, _ := p.Args["environment"].(string)
	region, _ := p.Args["region"].(string)
	hasDoc := p.Args["filterDocument"] != nil
	if hasDoc && project == "" {
		return savedsearch.ScopeRequest{}, false, fmt.Errorf("%w: filterDocument applies to a project read", core.ErrBadRequest)
	}
	if hasDoc && (resource != "" || env != "" || region != "") {
		return savedsearch.ScopeRequest{}, false, fmt.Errorf("%w: filterDocument cannot be combined with resource, environment, or region", core.ErrBadRequest)
	}
	if project == "" && (env != "" || region != "") {
		if env != "" {
			return savedsearch.ScopeRequest{}, false, fmt.Errorf("%w: environment requires project", core.ErrBadRequest)
		}
		return savedsearch.ScopeRequest{}, false, fmt.Errorf("%w: region requires project", core.ErrBadRequest)
	}
	if project == "" && resource == "" {
		return savedsearch.ScopeRequest{}, false, fmt.Errorf("%w: %s requires resource or project", core.ErrBadRequest, p.Info.FieldName)
	}
	req := savedsearch.ScopeRequest{
		Project:     project,
		Environment: env,
		Region:      region,
	}
	if resource != "" {
		req.Resources = []string{resource}
	}
	useView := project != "" && (hasDoc || (resource == "" && env == "" && region == ""))
	return req, useView, nil
}

// logListResult is a page as the LogList fields.
func logListResult(page logPage) map[string]any {
	return map[string]any{
		"hasMore":       page.HasMore,
		"nextStartTime": page.NextStartTime,
		"nextEndTime":   page.NextEndTime,
		"logs":          page.Entries,
	}
}

// logQueryFromArgs maps the GraphQL filter args onto a LogQuery. An unknown type
// or direction errors (naming the value) rather than widening the query silently —
// the same contract the REST adapter enforces.
func logQueryFromArgs(args map[string]any) (LogQuery, error) {
	resource, _ := args["resource"].(string)
	q := LogQuery{App: resource}
	if t, ok := args["type"].(string); ok {
		types, err := NormalizeTypes([]string{t})
		if err != nil {
			return LogQuery{}, err
		}
		q.Types = types
	}
	if s, ok := args["text"].(string); ok {
		q.Search = []string{s}
	}
	q.TextMode, _ = args["textMode"].(string) // vetted by LogQuery.validate
	// Direction is validated by the verb (LogQuery.validate), not here — one
	// enforcement point for all three surfaces.
	q.Direction, _ = args["direction"].(string)
	q.Level = gqlutil.StringList(args["level"])
	q.Instance = gqlutil.StringList(args["instance"])
	q.Host = gqlutil.StringList(args["host"])
	q.StatusCode = gqlutil.StringList(args["statusCode"])
	q.Method = gqlutil.StringList(args["method"])
	q.Path = gqlutil.StringList(args["path"])
	q.Build = gqlutil.StringList(args["build"])
	startTime, _ := args["startTime"].(string)
	endTime, _ := args["endTime"].(string)
	since, end, err := core.ParseTimeWindow(startTime, endTime)
	if err != nil {
		return LogQuery{}, err
	}
	q.Since, q.End = since, end
	return q, nil
}
