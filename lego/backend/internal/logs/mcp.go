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
	"context"
	"slices"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go is the MCP fragment for logs. Both tools mirror Render's official MCP
// server (render-oss/render-mcp-server, pkg/logs/tools.go) — same names, same
// argument names and arities — so an agent written against Render's server drives
// bex unchanged:
//
//	list_logs             — the filtered read (resource + the full filter set)
//	list_log_label_values — filter-value discovery ("what levels/statuses exist?")
//
// Every filter is routed through the same Core verbs the REST/GraphQL adapters
// use, so all three surfaces honor a filter identically. Filters bex cannot honor
// in pod-log fallback mode return ErrLogStoreUnavailable rather than being quietly
// dropped — an agent is told, not misled.

// logFilters is the filter set both tools share, in Render's shape (repeatable
// filters are arrays). Embedded so the two tools can never drift apart.
type logFilters struct {
	Resource   []string `json:"resource" jsonschema:"service ids (bex App names) to read logs for; all must belong to the same owner"`
	Level      []string `json:"level,omitempty" jsonschema:"filter logs by severity level (debug|info|warn|error|unknown); * wildcards supported"`
	Type       []string `json:"type,omitempty" jsonschema:"filter logs by type: app (the service's own output) | request (edge access logs) | build (build output)"`
	Instance   []string `json:"instance,omitempty" jsonschema:"filter logs by the instance (replica) they were emitted from; applies to app logs"`
	Host       []string `json:"host,omitempty" jsonschema:"filter request logs by their host; * wildcards supported"`
	StatusCode []string `json:"statusCode,omitempty" jsonschema:"filter request logs by status code (e.g. 404 or the class 4xx); * wildcards supported"`
	Method     []string `json:"method,omitempty" jsonschema:"filter request logs by HTTP method; * wildcards supported"`
	Path       []string `json:"path,omitempty" jsonschema:"filter request logs by their path; * wildcards supported"`
	Text       []string `json:"text,omitempty" jsonschema:"case-insensitive substring to filter the log message by"`
	StartTime  string   `json:"startTime,omitempty" jsonschema:"RFC3339 lower time bound (inclusive)"`
	EndTime    string   `json:"endTime,omitempty" jsonschema:"RFC3339 upper time bound (inclusive)"`
	Direction  string   `json:"direction,omitempty" jsonschema:"which end of the time range to take lines from: backward (default, most recent) | forward (oldest)"`
}

// listLogsArgs mirrors Render's list_logs: the shared filters plus a line cap.
type listLogsArgs struct {
	logFilters
	Limit int64 `json:"limit,omitempty" jsonschema:"max log lines to return (default 20, max 100)"`
}

// listLogLabelValuesArgs mirrors Render's list_log_label_values: the shared
// filters plus the label to enumerate.
//
// The filters that are stream LABELS (type/level/instance/statusCode/method) narrow
// which streams are searched, exactly as they do for list_logs. The three that are
// LINE filters (text/path/host) cannot: a label-values lookup reads the store's
// stream index, which holds no line content. They are still accepted — the args
// mirror Render's tool — but they do not narrow the values returned, which the
// tool's description says out loud rather than leaving a caller to discover it
// (docs/ADR010-observability.md § Render compatibility).
type listLogLabelValuesArgs struct {
	logFilters
	Label string `json:"label" jsonschema:"the label to list values for: host|instance|level|method|statusCode|type"`
}

type listLogsResult struct {
	Logs []LogEntry `json:"logs"`
}

// listLogLabelValuesResult carries the discovered values. Render's tool returns a
// bare array; MCP tool results are objects, so bex names the field.
type listLogLabelValuesResult struct {
	Values []string `json:"values"`
}

// query resolves the shared filters into a LogQuery (App unset — the caller loops
// over Resource). Time bounds and the type vocabulary are validated here, so a
// malformed filter is an error naming it rather than a silently ignored argument.
func (f logFilters) query() (LogQuery, error) {
	types, err := NormalizeTypes(f.Type)
	if err != nil {
		return LogQuery{}, err
	}
	// Direction is vetted by the verb (LogQuery.validate) — one refusal, three
	// surfaces.
	q := LogQuery{
		Types:      types,
		Search:     firstNonEmpty(f.Text),
		Level:      f.Level,
		Instance:   f.Instance,
		Host:       f.Host,
		StatusCode: f.StatusCode,
		Method:     f.Method,
		Path:       f.Path,
		Direction:  f.Direction,
	}
	if f.StartTime != "" {
		t, err := time.Parse(time.RFC3339, f.StartTime)
		if err != nil {
			return LogQuery{}, core.Err("startTime: " + err.Error())
		}
		q.Since = t
	}
	if f.EndTime != "" {
		t, err := time.Parse(time.RFC3339, f.EndTime)
		if err != nil {
			return LogQuery{}, core.Err("endTime: " + err.Error())
		}
		q.End = t
	}
	return q, nil
}

// RegisterMCP adds the list_logs + list_log_label_values tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_logs",
		Description: "List log lines for one or more services (Render's `resource` array), filtered by type (app | request), level, instance, host, statusCode, method, path, text and time range. " +
			"Timestamp-sorted and aggregated across instances. Use list_log_label_values to discover which filter values exist for a service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listLogsArgs) (*mcp.CallToolResult, listLogsResult, error) {
		q, err := in.query()
		if err != nil {
			return nil, listLogsResult{}, err
		}
		// BEX_MAX_QUERY_HOURS bounds this surface too (w9/004) — the same
		// checkWindow REST's logsQuery/logsValues and GraphQL's resolvers call,
		// so no adapter lets a caller scan the store unbounded.
		if err := s.checkWindow(q); err != nil {
			return nil, listLogsResult{}, err
		}
		q.Limit = in.Limit

		var all []LogEntry
		for _, id := range in.Resource {
			q.App = id
			entries, err := s.QueryLogs(ctx, q)
			if err != nil {
				return nil, listLogsResult{}, err
			}
			all = append(all, entries...)
		}
		// Re-sort across resources; the limit is a total, not per-instance
		// (QueryLogs already applied the per-App cap), and the direction decides
		// which end of the window survives the cap.
		sort.SliceStable(all, func(i, j int) bool { return all[i].Timestamp < all[j].Timestamp })
		return nil, listLogsResult{Logs: q.normalized().capToLimit(all)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_log_label_values",
		Description: "List all values a given log label (host | instance | level | method | statusCode | type) takes in the logs matching the provided filters. " +
			"Use it to discover what values are available for filtering with list_logs, instead of guessing. " +
			"The label filters (type/level/instance/statusCode/method) and the time range narrow the search; the line filters (text/path/host) do not affect which values are returned.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listLogLabelValuesArgs) (*mcp.CallToolResult, listLogLabelValuesResult, error) {
		q, err := in.query()
		if err != nil {
			return nil, listLogLabelValuesResult{}, err
		}
		if err := s.checkWindow(q); err != nil {
			return nil, listLogLabelValuesResult{}, err
		}
		all := []string{}
		for _, id := range in.Resource {
			q.App = id
			values, err := s.LogLabelValues(ctx, in.Label, q)
			if err != nil {
				return nil, listLogLabelValuesResult{}, err
			}
			all = append(all, values...)
		}
		slices.Sort(all)
		return nil, listLogLabelValuesResult{Values: slices.Compact(all)}, nil
	})
}

// firstNonEmpty returns the first non-empty string (Render's `text` filter is an
// array; Core applies a single substring, so the tool honors the first).
func firstNonEmpty(ss []string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
