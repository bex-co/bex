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
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go is the MCP fragment for logs. list_logs mirrors Render's official MCP
// tool (render-oss/render-mcp-server): the required `resource` array plus the
// filters bex can honor over raw pod logs — type, text, startTime, endTime,
// limit — routed through the same Core.QueryLogs the REST adapter uses. The
// structured request-log filters Render also accepts (level, instance, host,
// statusCode, method, path, direction) are omitted here, exactly as in REST,
// because bex sources application logs only (docs/ADR010-observability.md). Omitting a
// filter bex can't serve keeps the tool a safe subset rather than a fake one.

// listLogsArgs mirrors Render's list_logs. Repeatable filters are arrays (Render
// shape); bex maps `type` to a single concrete type (via narrowLogType) and
// applies the first `text` substring, matching the REST adapter's contract.
type listLogsArgs struct {
	Resource  []string `json:"resource" jsonschema:"service ids (bex App names) to read logs for; all must belong to the same owner"`
	Type      []string `json:"type,omitempty" jsonschema:"log type(s): app | request | build; bex sources app only, so request/build resolve to empty"`
	Text      []string `json:"text,omitempty" jsonschema:"case-insensitive substring to filter the log message by"`
	StartTime string   `json:"startTime,omitempty" jsonschema:"RFC3339 lower time bound (inclusive)"`
	EndTime   string   `json:"endTime,omitempty" jsonschema:"RFC3339 upper time bound (inclusive)"`
	Limit     int64    `json:"limit,omitempty" jsonschema:"max log lines to return (default 100, max 100)"`
}

type listLogsResult struct {
	Logs []LogEntry `json:"logs"`
}

// RegisterMCP adds the list_logs tool to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_logs",
		Description: "List recent log lines for one or more services (Render's `resource` array), optionally filtered by type / text / startTime / endTime, timestamp-sorted and aggregated across instances.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listLogsArgs) (*mcp.CallToolResult, listLogsResult, error) {
		typ, _ := narrowLogType(in.Type) // the tool tolerates unknown filter values
		q := LogQuery{Type: typ, Search: firstNonEmpty(in.Text), Limit: in.Limit}
		if in.StartTime != "" {
			t, err := time.Parse(time.RFC3339, in.StartTime)
			if err != nil {
				return nil, listLogsResult{}, core.Err("startTime: " + err.Error())
			}
			q.Since = t
		}
		if in.EndTime != "" {
			t, err := time.Parse(time.RFC3339, in.EndTime)
			if err != nil {
				return nil, listLogsResult{}, core.Err("endTime: " + err.Error())
			}
			q.End = t
		}

		var all []LogEntry
		for _, id := range in.Resource {
			q.App = id
			entries, err := s.QueryLogs(ctx, q)
			if err != nil {
				return nil, listLogsResult{}, err
			}
			all = append(all, entries...)
		}
		// Re-sort across resources; keep the newest `limit` lines (Render's limit
		// is a total, not per-instance — QueryLogs already applied the per-App cap).
		sort.SliceStable(all, func(i, j int) bool { return all[i].Timestamp < all[j].Timestamp })
		limit := in.Limit
		if limit <= 0 {
			limit = defaultLogTail
		}
		if int64(len(all)) > limit {
			all = all[int64(len(all))-limit:]
		}
		return nil, listLogsResult{Logs: all}, nil
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
