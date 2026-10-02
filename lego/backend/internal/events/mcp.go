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
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// mcp.go is the events MCP fragment: Render's list_events, bex's older name for
// the same read, and a bex-only single-event lookup.
//
// History, because the header this replaces was true when written and then
// wasn't. At 2a00be1 (checked 2026-07-12) render-oss/render-mcp-server had NO
// events tool, so bex shipped list_service_events as a deliberate extension in
// Render's tool grammar. Upstream then shipped the same capability as
// list_events (9d1f2b8, #170; pinned in openapi/render-mcp-tools.json since
// w1/m165), which turned a bex extension into two names for one tool. So:
//
//   - list_events is Render's contract, argument for argument: serviceId
//     (required), eventTypes (an array), startTime, endTime, cursor, limit, plus
//     the workspaceId every scoped tool gets from the api package's middleware.
//     Like upstream the first page defaults to the LAST 7 DAYS
//     (listEventsDefaultLookback) — an agent written against Render's tool
//     asks "why did this restart last night?" without a startTime.
//   - list_service_events stays, unchanged, as a working alias (user decision
//     2026-09-30: retiring a published tool is a breaking change for its
//     callers). Its `type` string and now-1h default are REST's.
//   - get_service_event (one event by evt-… id) remains a bex extension;
//     upstream still has no equivalent.
//
// Both list tools answer from the one Service.List over the one FilterOf
// translator REST and GraphQL use, so there is no fourth events vocabulary: the
// same types, ids, details and cursors, and the same multi-type semantics —
// eventTypes ["a","b"] is REST's `?type=a,b`, exactly the collapse Render's own
// server performs (pkg/event/tools.go: strings.Join(eventTypes, ",")). An
// unknown type matches nothing, as it does on GraphQL; only the REST gate
// validates types against Render's pinned enum.

// listEventsDefaultLookback is upstream list_events' window when the caller
// names no startTime (render-mcp-server pkg/event/tools.go defaultLookback).
const listEventsDefaultLookback = 7 * 24 * time.Hour

// listEventsArgs is Render's list_events input, names verbatim from the pin.
type listEventsArgs struct {
	ServiceID  string   `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	EventTypes []string `json:"eventTypes,omitempty" jsonschema:"filter to any of these event types, e.g. server_failed and deploy_ended; omit for all types"`
	StartTime  *string  `json:"startTime,omitempty" jsonschema:"RFC3339 start of the window; defaults to 7 days ago on the first page"`
	EndTime    *string  `json:"endTime,omitempty" jsonschema:"RFC3339 end of the window; defaults to now"`
	Cursor     string   `json:"cursor,omitempty" jsonschema:"resume after this cursor (the cursor of the last event of the previous page)"`
	Limit      int      `json:"limit,omitempty" jsonschema:"page size, 1-100 (default 20)"`
}

// listServiceEventsArgs is list_service_events' input — the same five params
// Render's REST endpoint takes, keyed on serviceId like every other per-service
// tool.
type listServiceEventsArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Type      string `json:"type,omitempty" jsonschema:"filter to one event type (or several, comma-separated), e.g. deploy_ended or suspender_added; omit for all"`
	StartTime string `json:"startTime,omitempty" jsonschema:"RFC3339 start of the window; DEFAULTS TO ONE HOUR AGO — pass it to look further back"`
	EndTime   string `json:"endTime,omitempty" jsonschema:"RFC3339 end of the window; defaults to now"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"resume after this cursor (the cursor of the last event of the previous page)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"page size, 1-100 (default 20)"`
}

// listServiceEventsResult preserves the legacy structured JSON object.
// Each item carries its cursor alongside the event, the same envelope REST
// returns. This is the legacy tool's published output.
type listServiceEventsResult struct {
	Events []eventWithCursor `json:"events"`
}

type getServiceEventArgs struct {
	ID string `json:"id" jsonschema:"required,the evt-… id from a webhook data.id or a service event list item"`
}

type getServiceEventResult struct {
	Event renderEvent `json:"event"`
}

// RegisterMCP adds the events tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
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
	mcputil.AddTool(srv, &mcp.Tool{
		Name: "list_events",
		Description: "List a service's event history, newest first: deploys and builds started/ended, " +
			"server failures and recoveries, suspends and resumes, restarts, scaling, plan changes, " +
			"env-var and config writes — each with who did it and when. The first page defaults to the LAST 7 DAYS; " +
			"set startTime to reach older events. Page with cursor. Env-var VALUES never appear in an event. " +
			"Events cover services only, not Postgres or Key Value instances.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listEventsArgs) (*mcp.CallToolResult, any, error) {
		filter := FilterOf(in.EventTypes, "", "", in.Cursor, in.Limit)
		var err error
		filter.Since, err = parseListEventsTime("startTime", in.StartTime)
		if err != nil {
			return nil, nil, err
		}
		filter.Until, err = parseListEventsTime("endTime", in.EndTime)
		if err != nil {
			return nil, nil, err
		}
		filter.OpenStart = in.StartTime == nil && in.Cursor != ""
		if filter.Since.IsZero() && !filter.OpenStart {
			filter.Since = s.Now().Add(-listEventsDefaultLookback)
		}
		events, err := s.List(ctx, in.ServiceID, filter)
		if err != nil {
			return nil, nil, err
		}
		out := make([]renderEvent, 0, len(events))
		cursor := `""`
		for _, event := range events {
			out = append(out, toRenderEvent(event))
			cursor = event.Cursor
		}
		body, err := json.Marshal(out)
		if err != nil {
			return nil, nil, err
		}
		// Render returns text, not structured output: a bare event array followed
		// by the last item's cursor (or a quoted empty string for an empty page).
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{
			Text: string(body) + "\n\n cursor: " + cursor,
		}}}, nil, nil
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name: "list_service_events",
		Description: "Alias of list_events kept for existing callers (bex's name for the tool before " +
			"Render shipped list_events): same events, but filters with a single `type` string and " +
			"defaults to the LAST HOUR; pass startTime to look further back. Page with cursor.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listServiceEventsArgs) (*mcp.CallToolResult, listServiceEventsResult, error) {
		events, err := s.List(ctx, in.ServiceID, FilterOf([]string{in.Type}, in.StartTime, in.EndTime, in.Cursor, in.Limit))
		if err != nil {
			return nil, listServiceEventsResult{}, err
		}
		return nil, listServiceEventsResult{Events: toEventList(events)}, nil
	})
}

func parseListEventsTime(field string, value *string) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}
	// Upstream distinguishes an omitted bound from a supplied empty timestamp.
	if *value == "" {
		return time.Time{}, fmt.Errorf("%w: %s must be an RFC3339 timestamp", core.ErrBadRequest, field)
	}
	return core.ParseTime(field, *value)
}
