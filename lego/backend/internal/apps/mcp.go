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

// updatePlanArgs is update_service_plan's input — Render's plan spelling
// (e.g. "pro_plus"), same as the REST/GraphQL surfaces.
type updatePlanArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Plan      string `json:"plan" jsonschema:"the new instance plan, e.g. starter, standard, pro, pro_plus, pro_max, pro_ultra"`
}

// scaleArgs is scale_service's input — the desired running instance count,
// keyed on numInstances like Render's REST/GraphQL surfaces.
type scaleArgs struct {
	ServiceID    string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	NumInstances int32  `json:"numInstances" jsonschema:"the desired number of running instances (1-100)"`
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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_service_plan",
		Description: "Change a service's instance plan/size (e.g. to starter, standard, pro, pro_plus, pro_max, pro_ultra). Resizes the pod's resources and rolls it. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updatePlanArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetPlan(ctx, in.ServiceID, in.Plan)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "scale_service",
		Description: "Scale a service to a specific number of running instances (numInstances, 1-100). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in scaleArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.Scale(ctx, in.ServiceID, in.NumInstances)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})
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
