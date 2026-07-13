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

package deploys

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the MCP fragment: list_deploys/get_deploy, under Render's
// official tool names (render-oss/render-mcp-server) — the deferral w2/m1's
// README recorded ("no list_deploys/get_deploy — add them when Core grows
// those verbs") closed in w2/m5. cancel_deploy/rollback_deploy (w2/m10) are
// bex extensions: Render's official MCP server ships neither, but "that
// deploy broke, roll it back" is the highest-value verb an agent driving the
// list_deploys/get_deploy poll loop is missing. All four delegate to the same
// List/Get/Cancel/Rollback the REST adapter calls, so the surfaces cannot
// drift.

// listDeploysArgs is list_deploys' input — Render keys deploy tools on
// serviceId, like every other single-service tool.
type listDeploysArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
}

// getDeployArgs is get_deploy's input — the deploy to poll after a trigger.
type getDeployArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	DeployID  string `json:"deployId" jsonschema:"the deploy id (dep-…), as returned by list_deploys"`
}

// listDeploysResult wraps the array — MCP tool outputs must be JSON objects.
type listDeploysResult struct {
	Deploys []renderDeploy `json:"deploys"`
}

// RegisterMCP adds the deploy-history tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_deploys",
		Description: "List a service's deploy history, newest first — status transitions build_in_progress/update_in_progress -> live, *_failed, or canceled.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listDeploysArgs) (*mcp.CallToolResult, listDeploysResult, error) {
		deploys, err := s.List(ctx, in.ServiceID)
		if err != nil {
			return nil, listDeploysResult{}, err
		}
		out := make([]renderDeploy, len(deploys))
		for i, d := range deploys {
			out[i] = toRenderDeploy(d)
		}
		return nil, listDeploysResult{Deploys: out}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_deploy",
		Description: "Get one deploy by id — poll this after triggering a deploy until status is live (or a *_failed/canceled status).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getDeployArgs) (*mcp.CallToolResult, renderDeploy, error) {
		d, err := s.Get(ctx, in.ServiceID, in.DeployID)
		if err != nil {
			return nil, renderDeploy{}, err
		}
		return nil, toRenderDeploy(d), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "cancel_deploy",
		Description: "bex extension: cancel a still-in-progress deploy — kills its in-flight build (if any) and marks it canceled. 409s once the deploy has already reached a final status (live/*_failed/canceled).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getDeployArgs) (*mcp.CallToolResult, renderDeploy, error) {
		d, err := s.Cancel(ctx, in.ServiceID, in.DeployID)
		if err != nil {
			return nil, renderDeploy{}, err
		}
		return nil, toRenderDeploy(d), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "rollback_deploy",
		Description: "bex extension: roll a service back to a previously-live deploy's exact image — creates a fresh deploy restoring it, never rewrites history. Only a deploy that itself reached live is a valid target.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getDeployArgs) (*mcp.CallToolResult, renderDeploy, error) {
		d, err := s.Rollback(ctx, in.ServiceID, in.DeployID)
		if err != nil {
			return nil, renderDeploy{}, err
		}
		return nil, toRenderDeploy(d), nil
	})
}
