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
// serviceId, like every other single-service tool. Limit/Cursor (w2/m31)
// match Render's own MCP tool's pagination knobs (render-oss/render-mcp-server
// pkg/deploy/tools.go: limit default 10, clamped to [1,100]; cursor from the
// previous result).
type listDeploysArgs struct {
	ServiceID      string   `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Statuses       []string `json:"status,omitempty" jsonschema:"include deploys in any of these Render lifecycle statuses"`
	CreatedBefore  string   `json:"createdBefore,omitempty" jsonschema:"include deploys created before this RFC3339 timestamp"`
	CreatedAfter   string   `json:"createdAfter,omitempty" jsonschema:"include deploys created after this RFC3339 timestamp"`
	UpdatedBefore  string   `json:"updatedBefore,omitempty" jsonschema:"include deploys updated before this RFC3339 timestamp"`
	UpdatedAfter   string   `json:"updatedAfter,omitempty" jsonschema:"include deploys updated after this RFC3339 timestamp"`
	FinishedBefore string   `json:"finishedBefore,omitempty" jsonschema:"include deploys finished before this RFC3339 timestamp"`
	FinishedAfter  string   `json:"finishedAfter,omitempty" jsonschema:"include deploys finished after this RFC3339 timestamp"`
	Limit          int      `json:"limit,omitempty" jsonschema:"page size, 1-100 (default 10)"`
	Cursor         string   `json:"cursor,omitempty" jsonschema:"resume after this cursor (the cursor of the previous list_deploys result); omit for the first page"`
}

// getDeployArgs is get_deploy's input — the deploy to poll after a trigger.
type getDeployArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	DeployID  string `json:"deployId" jsonschema:"the deploy id (dep-…), as returned by list_deploys"`
}

// triggerDeployArgs is trigger_deploy's input (w2/m44).
type triggerDeployArgs struct {
	ServiceID  string `json:"serviceId" jsonschema:"the service id, as returned by list_services"`
	ImageURL   string `json:"imageUrl,omitempty" jsonschema:"image-backed services only: deploy this specific image tag instead of the current spec; rejected for repo-backed services"`
	CommitID   string `json:"commitId,omitempty" jsonschema:"repo-backed services only: build from this git ref instead of Branch HEAD; rejected for cron_job services"`
	DeployMode string `json:"deployMode,omitempty" jsonschema:"build_and_deploy (default) or deploy_only (image-backed only — skips rebuild, re-pulls current image)"`
}

type serviceArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
}

// listDeploysResult wraps the array — MCP tool outputs must be JSON objects.
// Cursor (w2/m31) is the bex-shaped equivalent of Render's trailing
// `cursor: <value>` text marker (its tool appends it after the deploys JSON;
// bex's outputs are JSON objects, so the cursor is a field instead): the last
// deploy's id, to echo back as `cursor` for the next page, or "" when this
// page is empty (the end of the history).
type listDeploysResult struct {
	Deploys []renderDeploy `json:"deploys"`
	Cursor  string         `json:"cursor"`
}

// RegisterMCP adds the deploy-history tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_deploy_hook",
		Description: "Get (or lazily create) a service's secret deploy-hook URL. Anyone holding the URL can trigger a deploy without an API key; treat it as a credential and do not log it. bex extension: Render exposes this only in its dashboard.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, DeployHookView, error) {
		hook, err := s.GetDeployHook(ctx, in.ServiceID)
		return nil, hook, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "regenerate_deploy_hook",
		Description: "Rotate a service's secret deploy-hook URL. The old URL stops working immediately, so update every CI system that used it. bex extension: Render exposes this only in its dashboard.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, DeployHookView, error) {
		hook, err := s.RegenerateDeployHook(ctx, in.ServiceID)
		return nil, hook, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_deploys",
		Description: "List a service's deploy history, newest first, with Render lifecycle status and created/updated/finished timestamp filters. Returns up to `limit` deploys (default 10) plus a `cursor`: pass it back to fetch the next page; an empty page means the end.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listDeploysArgs) (*mcp.CallToolResult, listDeploysResult, error) {
		// Render's own tool's bounds: default 10 when unset, clamped to [1,100].
		// The default is an intentional w2/m31 behavior change from bex's prior
		// always-the-full-history tool — an agent polling a long-lived service
		// should not re-fetch hundreds of deploys per call; page with cursor.
		limit := in.Limit
		if limit < 1 {
			limit = 10
		}
		filter, err := FilterOf(
			in.Statuses,
			in.CreatedBefore, in.CreatedAfter,
			in.UpdatedBefore, in.UpdatedAfter,
			in.FinishedBefore, in.FinishedAfter,
			in.Cursor, limit,
		)
		if err != nil {
			return nil, listDeploysResult{}, err
		}
		deploys, err := s.List(ctx, in.ServiceID, filter)
		if err != nil {
			return nil, listDeploysResult{}, err
		}
		out := make([]renderDeploy, len(deploys))
		for i, d := range deploys {
			out[i] = toRenderDeploy(d)
		}
		res := listDeploysResult{Deploys: out}
		if len(deploys) > 0 {
			res.Cursor = deploys[len(deploys)-1].ID
		}
		return nil, res, nil
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

	// trigger_deploy fires a new deploy — Render's POST .../deploys as an MCP
	// tool (w2/m44). imageUrl allows image-backed services to deploy a specific
	// tag without re-applying the whole service spec. commitId/deployMode carry
	// the same semantics as the REST body.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "trigger_deploy",
		Description: "Trigger a new deploy for a service. For image-backed services, imageUrl deploys a specific image tag. For repo-backed services, commitId pins the build to a specific git ref (default: Branch HEAD). Returns the new deploy; poll with get_deploy until status is live.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in triggerDeployArgs) (*mcp.CallToolResult, renderDeploy, error) {
		d, err := s.Trigger(ctx, in.ServiceID, TriggerParams{
			ImageURL:   in.ImageURL,
			CommitID:   in.CommitID,
			DeployMode: in.DeployMode,
		})
		if err != nil {
			return nil, renderDeploy{}, err
		}
		return nil, toRenderDeploy(d), nil
	})
}
