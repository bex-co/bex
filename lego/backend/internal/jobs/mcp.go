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

package jobs

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// mcp.go is the MCP fragment: list_jobs, create_job, get_job, cancel_job.
// Upstream ships no job tools, so the parity pin classifies all four as
// Extension (internal/api/mcp_parity.go asserts this rather than this comment
// claiming it). They mirror Render's REST shape so agents can trigger one-off
// commands in a service's container without dropping to the REST API.
//
// NOTE (w1/m70): one-off jobs are listed as a deliberate non-goal in
// .pm/DO_NOT_DO.md and marked `—` on all four surfaces in ADR018, yet this
// package ships a full Service plus REST, GraphQL, and these four MCP tools.
// Either the ledger row is stale or this surface should not be here; flagged
// for a decision rather than silently resolved in a parity-pin milestone.

type jobServiceIDArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
}

type listJobsArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Limit     int    `json:"limit,omitempty" jsonschema:"page size (default: all)"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"keyset cursor from the previous list_jobs call; omit for the first page"`
}

type listJobsResult struct {
	Jobs   []renderJob `json:"jobs"`
	Cursor string      `json:"cursor"`
}

type createJobArgs struct {
	ServiceID    string `json:"serviceId"    jsonschema:"the service id (bex App name)"`
	StartCommand string `json:"startCommand" jsonschema:"the shell command to run in the service's current container image"`
	PlanID       string `json:"planId,omitempty" jsonschema:"optional plan override (default: starter)"`
}

type jobIDArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name)"`
	JobID     string `json:"jobId"     jsonschema:"the job id (job-…), as returned by list_jobs or create_job"`
}

// RegisterMCP adds the one-off job tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_jobs",
		Description: "bex extension: list one-off jobs for a service, newest first. Returns the full history unless `limit` is set; pass the returned `cursor` back to page.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listJobsArgs) (*mcp.CallToolResult, listJobsResult, error) {
		filter, err := FilterFromStrings(nil, "", "", "", "", "", "", in.Cursor, in.Limit)
		if err != nil {
			return nil, listJobsResult{}, err
		}
		views, err := s.List(ctx, in.ServiceID, filter)
		if err != nil {
			return nil, listJobsResult{}, err
		}
		out := make([]renderJob, len(views))
		for i, v := range views {
			out[i] = toRenderJob(v)
		}
		res := listJobsResult{Jobs: out}
		if len(views) > 0 {
			res.Cursor = views[len(views)-1].ID
		}
		return nil, res, nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_job",
		Description: "bex extension: run a one-off command in the service's current container image (like `render jobs create`). The job is pending until the cluster schedules the pod; poll get_job until status is succeeded, failed, or canceled.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createJobArgs) (*mcp.CallToolResult, renderJob, error) {
		v, err := s.Create(ctx, in.ServiceID, in.StartCommand, in.PlanID)
		if err != nil {
			return nil, renderJob{}, err
		}
		return nil, toRenderJob(v), nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_job",
		Description: "bex extension: get a one-off job's current status — poll this after create_job until status is succeeded, failed, or canceled.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in jobIDArgs) (*mcp.CallToolResult, renderJob, error) {
		v, err := s.Get(ctx, in.ServiceID, in.JobID)
		if err != nil {
			return nil, renderJob{}, err
		}
		return nil, toRenderJob(v), nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "cancel_job",
		Description: "bex extension: cancel a pending or running one-off job. Returns 409 if the job is already in a terminal state (succeeded/failed/canceled).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in jobIDArgs) (*mcp.CallToolResult, renderJob, error) {
		v, err := s.Cancel(ctx, in.ServiceID, in.JobID)
		if err != nil {
			return nil, renderJob{}, err
		}
		return nil, toRenderJob(v), nil
	})
}
