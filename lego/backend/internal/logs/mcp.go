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

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the MCP fragment for logs. list_logs is 1:1 with Render's official
// MCP tool: a required `resource` array of service ids and an optional `limit`.

// listLogsArgs mirrors Render's list_logs. bex omits Render's structured-log
// filters (level, statusCode, method, ...) it can't honor over raw pod logs.
type listLogsArgs struct {
	Resource []string `json:"resource" jsonschema:"service ids (bex App names) to read logs for; all must belong to the same owner"`
	Limit    int64    `json:"limit,omitempty" jsonschema:"max log lines to return (default 100)"`
}

type listLogsResult struct {
	Logs []LogEntry `json:"logs"`
}

// RegisterMCP adds the list_logs tool to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_logs",
		Description: "List recent log lines for one or more services (Render's `resource` array), timestamp-sorted and aggregated across instances.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listLogsArgs) (*mcp.CallToolResult, listLogsResult, error) {
		var all []LogEntry
		for _, id := range in.Resource {
			entries, err := s.Logs(ctx, id, in.Limit)
			if err != nil {
				return nil, listLogsResult{}, err
			}
			all = append(all, entries...)
		}
		// Re-sort across resources; keep the newest `limit` lines (Render's limit
		// is a total, not per-instance).
		sort.SliceStable(all, func(i, j int) bool { return all[i].Timestamp < all[j].Timestamp })
		if in.Limit > 0 && int64(len(all)) > in.Limit {
			all = all[int64(len(all))-in.Limit:]
		}
		return nil, listLogsResult{Logs: all}, nil
	})
}
