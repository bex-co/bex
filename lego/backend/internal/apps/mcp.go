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

package apps

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the MCP fragment for services. Tool names track Render's official
// MCP server (render-oss/render-mcp-server): list_services / get_service are 1:1;
// the lifecycle verbs (restart/suspend/resume_service) are bex extensions named
// after Render's REST verbs. Every tool delegates to the same Service method
// REST/GraphQL call, so the three surfaces cannot drift.

// serviceArgs is the shared single-service argument. Render's tools key on
// `serviceId` (see get_service); for bex that id is the App name (opaque,
// round-tripped from list_services).
type serviceArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
}

// listServicesResult wraps the array — MCP tool outputs must be JSON objects.
type listServicesResult struct {
	Services []renderService `json:"services"`
}

// RegisterMCP adds the service tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_services",
		Description: "List all services (bex Apps) in the workspace with their status.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listServicesResult, error) {
		apps, err := s.List(ctx)
		if err != nil {
			return nil, listServicesResult{}, err
		}
		return nil, listServicesResult{Services: toRenderServices(apps)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_service",
		Description: "Get details about a specific service by id.",
	}, s.serviceTool(s.Get))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "restart_service",
		Description: "Restart a service (rolling restart, no downtime). bex extension over Render's MCP.",
	}, s.serviceTool(s.Restart))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "suspend_service",
		Description: "Suspend a service: scale to zero, keeping host and certificates. bex extension over Render's MCP.",
	}, s.serviceTool(s.Suspend))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "resume_service",
		Description: "Resume a suspended service, restoring its replicas. bex extension over Render's MCP.",
	}, s.serviceTool(s.Resume))
}

// serviceTool adapts a single-service verb (Get/Restart/Suspend/Resume) into an
// MCP tool handler returning the Render service object — the same mapping REST's
// verb handlers use, so the surfaces stay identical.
func (s *Service) serviceTool(fn func(context.Context, string) (AppView, error)) mcp.ToolHandlerFor[serviceArgs, renderService] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := fn(ctx, in.ServiceID)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	}
}
