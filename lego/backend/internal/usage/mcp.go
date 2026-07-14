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

package usage

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the usage MCP fragment. get_usage gives an MCP agent direct
// insight into its own workspace's month-to-date resource consumption —
// pillar 3 ("agents as operators"): an agent that deploys should be able to
// check what it consumes (docs/ADR008-vision.md).

type getUsageArgs struct {
	Period string `json:"period,omitempty" jsonschema:"calendar month as YYYY-MM; defaults to the current month"`
}

// RegisterMCP adds the get_usage tool to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_usage",
		Description: "Get month-to-date usage (instance-seconds by tier, egress bytes, build seconds) for the caller's workspace, plus an estimated cost in USD (estimate only — not an invoice; 30% below Render on compute/Postgres/KeyValue/build, 90% below on bandwidth). Returns the same quantities and cost estimate as GET /v1/usage.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getUsageArgs) (*mcp.CallToolResult, usageResponse, error) {
		now := resolvePeriodEnd(in.Period, s.Now().UTC())
		summary, err := s.monthToDateAt(ctx, now)
		if err != nil {
			return nil, usageResponse{}, err
		}
		return nil, toUsageResponse(summary), nil
	})
}
