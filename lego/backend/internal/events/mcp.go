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

package events

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the events MCP fragment — a deliberate bex EXTENSION, not a mirror.
//
// The official server (render-oss/render-mcp-server @ 2a00be1, checked 2026-07-12)
// registers 24 tools across owner/service/deploy/postgres/keyvalue/logs/metrics
// and has NO events tool: its generated REST client carries the events types
// (pkg/client/events), but nothing wires them to a tool. So there is no name to
// mirror and no parity gap to close — an events tool is bex going further.
//
// bex adds it anyway, under a name in Render's own tool grammar
// (list_<resource>, keyed on serviceId, like list_deploys): an agent asking "what
// happened to my service overnight?" can otherwise only poll list_deploys, which
// sees rollouts and nothing else — not the suspend, not the scale, not the env-var
// change that explains them. Activity, not just current state, is the point
// (docs/ADR008-vision.md pillar 3), and this is the one tool that carries it.
//
// The window default is Render's (now-1h): an agent asking about "overnight"
// must pass startTime, which the tool description says outright so it does.

// listServiceEventsArgs is list_service_events' input — the same five params
// Render's REST endpoint takes, keyed on serviceId like every other per-service
// tool.
type listServiceEventsArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Type      string `json:"type,omitempty" jsonschema:"filter to one event type, e.g. deploy_ended or suspender_added; omit for all"`
	StartTime string `json:"startTime,omitempty" jsonschema:"RFC3339 start of the window; DEFAULTS TO ONE HOUR AGO — pass it to look further back"`
	EndTime   string `json:"endTime,omitempty" jsonschema:"RFC3339 end of the window; defaults to now"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"resume after this cursor (the cursor of the last event of the previous page)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"page size, 1-100 (default 20)"`
}

// listServiceEventsResult wraps the array — MCP tool outputs must be JSON objects.
// Each item carries its cursor alongside the event, the same envelope REST returns.
type listServiceEventsResult struct {
	Events []eventWithCursor `json:"events"`
}

type getServiceEventArgs struct {
	ID string `json:"id" jsonschema:"required,the evt-… id from a webhook data.id or a service event list item"`
}

type getServiceEventResult struct {
	Event renderEvent `json:"event"`
}

// RegisterMCP adds the events tool to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_service_event",
		Description: "Retrieve one authorized service, Postgres, or Key Value event by its evt-… id. " +
			"Use the data.id from a Bex outbound webhook to hydrate its thin payload. This is a Bex extension; " +
			"Render's official MCP server does not currently expose an event retrieval tool.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getServiceEventArgs) (*mcp.CallToolResult, getServiceEventResult, error) {
		event, err := s.Get(ctx, in.ID)
		if err != nil {
			return nil, getServiceEventResult{}, err
		}
		return nil, getServiceEventResult{Event: toRenderEvent(event)}, nil
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_service_events",
		Description: "List what has happened to a service, newest first: deploys started/ended, " +
			"suspends and resumes, restarts, plan and instance-count changes, env-var and config " +
			"writes — each with who did it and when. Defaults to the LAST HOUR; pass startTime to " +
			"look further back. Page with cursor. Env-var VALUES never appear in an event.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listServiceEventsArgs) (*mcp.CallToolResult, listServiceEventsResult, error) {
		// The same translator REST and GraphQL use, so a tool call and a REST call
		// with the same params return the same page.
		filter := FilterOf(in.Type, in.StartTime, in.EndTime, in.Cursor, in.Limit)
		events, err := s.List(ctx, in.ServiceID, filter)
		if err != nil {
			return nil, listServiceEventsResult{}, err
		}
		return nil, listServiceEventsResult{Events: toEventList(events)}, nil
	})
}
