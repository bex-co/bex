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
	"net/http"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the MCP adapter — a third Render-consistent surface over Core beside
// REST (Render public API) and GraphQL (Render dashboard). Tool names and the
// returned service object track Render's official MCP server
// (render-oss/render-mcp-server): list_services / get_service / list_logs are
// 1:1 with Render's tools; the lifecycle verbs (restart/suspend/resume_service)
// are bex extensions named after Render's REST verbs, since Render's official MCP
// is read-heavy and omits them. Every tool delegates to the same Core method
// REST/GraphQL call, so the three surfaces cannot drift.

const (
	mcpServerName = "bex"
	mcpVersion    = "0.1.0"
)

// serviceArgs is the shared single-service argument. Render's tools key on
// `serviceId` (see get_service / list_deploys); for bex that id is the App name
// (opaque, round-tripped from list_services).
type serviceArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
}

// listServicesResult wraps the array — MCP tool outputs must be JSON objects.
type listServicesResult struct {
	Services []renderService `json:"services"`
}

// listLogsArgs mirrors Render's list_logs: a required `resource` array of service
// ids and an optional `limit`. bex omits Render's structured-log filters (level,
// statusCode, method, ...) it can't honor over raw pod logs — the same "omit what
// bex lacks" rule REST follows for build plans / regions / disks.
type listLogsArgs struct {
	Resource []string `json:"resource" jsonschema:"service ids (bex App names) to read logs for; all must belong to the same owner"`
	Limit    int64    `json:"limit,omitempty" jsonschema:"max log lines to return (default 100)"`
}

type listLogsResult struct {
	Logs []LogEntry `json:"logs"`
}

// getMetricsArgs mirrors Render's get_metrics: a required `resource` array of
// service ids and `metricTypes` (bex metric ids: cpu / memory / instance_count /
// http_requests / http_latency / bandwidth), plus the optional time window and
// options. bex omits Render's aggregation knobs it doesn't honor, the same
// "omit what bex lacks" rule the other tools follow.
type getMetricsArgs struct {
	Resource          []string `json:"resource" jsonschema:"service ids (bex App names) to read metrics for"`
	MetricTypes       []string `json:"metricTypes" jsonschema:"metric ids: cpu|memory|instance_count|http_requests|http_latency|bandwidth"`
	StartTime         string   `json:"startTime,omitempty" jsonschema:"RFC3339 start of the window (request metrics)"`
	EndTime           string   `json:"endTime,omitempty" jsonschema:"RFC3339 end of the window (request metrics)"`
	ResolutionSeconds int64    `json:"resolutionSeconds,omitempty" jsonschema:"request-metric step in seconds"`
	Quantile          float64  `json:"quantile,omitempty" jsonschema:"http_latency percentile 0..1 (default .95)"`
	Percentage        bool     `json:"percentage,omitempty" jsonschema:"report cpu/memory as a percentage of the pod limit"`
}

type getMetricsResult struct {
	Series []MetricSeries `json:"series"`
}

// MCPServer builds the MCP server with every tool registered over Core. The
// returned server is stateless w.r.t. sessions, so one instance is reused for
// stdio and across HTTP sessions.
func (s *Server) MCPServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: mcpServerName, Version: mcpVersion}, nil)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_services",
		Description: "List all services (bex Apps) in the workspace with their status.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listServicesResult, error) {
		apps, err := s.Core.List(ctx)
		if err != nil {
			return nil, listServicesResult{}, err
		}
		return nil, listServicesResult{Services: toRenderServices(apps)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_service",
		Description: "Get details about a specific service by id.",
	}, s.serviceTool((*Core).Get))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "restart_service",
		Description: "Restart a service (rolling restart, no downtime). bex extension over Render's MCP.",
	}, s.serviceTool((*Core).Restart))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "suspend_service",
		Description: "Suspend a service: scale to zero, keeping host and certificates. bex extension over Render's MCP.",
	}, s.serviceTool((*Core).Suspend))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "resume_service",
		Description: "Resume a suspended service, restoring its replicas. bex extension over Render's MCP.",
	}, s.serviceTool((*Core).Resume))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_logs",
		Description: "List recent log lines for one or more services (Render's `resource` array), timestamp-sorted and aggregated across instances.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listLogsArgs) (*mcp.CallToolResult, listLogsResult, error) {
		var all []LogEntry
		for _, id := range in.Resource {
			entries, err := s.Core.Logs(ctx, id, in.Limit)
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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_metrics",
		Description: "Get resource (cpu/memory/instance_count) and request (http_requests/http_latency/bandwidth) metrics for one or more services, as Render-shaped time-series.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getMetricsArgs) (*mcp.CallToolResult, getMetricsResult, error) {
		q := MetricQuery{
			Quantile:   in.Quantile,
			Percentage: in.Percentage,
			Resolution: time.Duration(in.ResolutionSeconds) * time.Second,
		}
		if in.StartTime != "" {
			if t, err := time.Parse(time.RFC3339, in.StartTime); err == nil {
				q.Start = t
			}
		}
		if in.EndTime != "" {
			if t, err := time.Parse(time.RFC3339, in.EndTime); err == nil {
				q.End = t
			}
		}
		var all []MetricSeries
		for _, id := range in.Resource {
			for _, metric := range in.MetricTypes {
				q.App, q.Metric = id, metric
				series, err := s.Core.Metrics(ctx, q)
				if err != nil {
					return nil, getMetricsResult{}, err
				}
				// Tag each series with its metric so multi-metric results stay distinct.
				for i := range series {
					if series[i].Labels == nil {
						series[i].Labels = map[string]string{}
					}
					series[i].Labels["metric"] = metric
				}
				all = append(all, series...)
			}
		}
		return nil, getMetricsResult{Series: all}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_api_key",
		Description: "Create a machine credential (OAuth2 client) for the platform API. The secret is returned once — store it. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createAPIKeyArgs) (*mcp.CallToolResult, APIKey, error) {
		key, err := s.Core.CreateAPIKey(ctx, in.Name)
		return nil, key, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_api_keys",
		Description: "List the platform API's machine credentials (secrets never included). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listAPIKeysResult, error) {
		keys, err := s.Core.ListAPIKeys(ctx)
		return nil, listAPIKeysResult{APIKeys: keys}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "revoke_api_key",
		Description: "Revoke a machine credential by keyId; its tokens stop working. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in revokeAPIKeyArgs) (*mcp.CallToolResult, revokeAPIKeyResult, error) {
		err := s.Core.RevokeAPIKey(ctx, in.KeyID)
		return nil, revokeAPIKeyResult{Revoked: err == nil}, err
	})

	return srv
}

type createAPIKeyArgs struct {
	Name string `json:"name" jsonschema:"human-readable name for the credential"`
}

type listAPIKeysResult struct {
	APIKeys []APIKey `json:"apiKeys"`
}

type revokeAPIKeyArgs struct {
	KeyID string `json:"keyId" jsonschema:"the API key id (OAuth2 client_id)"`
}

type revokeAPIKeyResult struct {
	Revoked bool `json:"revoked"`
}

// serviceTool adapts a single-service Core verb (Get/Restart/Suspend/Resume) into
// an MCP tool handler returning the Render service object — the same mapping
// REST's verb handlers use, so the surfaces stay identical.
func (s *Server) serviceTool(fn func(*Core, context.Context, string) (AppView, error)) mcp.ToolHandlerFor[serviceArgs, renderService] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := fn(s.Core, ctx, in.ServiceID)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	}
}

// mcpHTTPHandler serves the MCP streamable-HTTP transport. Mounted at /mcp behind
// the same auth gate as REST/GraphQL (see Handler), so an HTTP MCP client
// authenticates with an API-key token or session exactly like every other route.
func (s *Server) mcpHTTPHandler() http.Handler {
	srv := s.MCPServer()
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

// RunStdio serves the MCP adapter over stdio (newline-delimited JSON on
// stdin/stdout) — the transport a local agent launches bex as a subprocess with.
// Here the trust boundary is the process itself (the caller already holds the
// kube credentials), so no bearer applies; the HTTP transport keeps the gate.
// Blocks until the client disconnects or ctx is cancelled.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.MCPServer().Run(ctx, &mcp.StdioTransport{})
}
