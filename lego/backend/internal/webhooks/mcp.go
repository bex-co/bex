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

package webhooks

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go is the outbound-webhooks MCP fragment — a bex superset: Render's
// official MCP server (render-oss/render-mcp-server v0.3.0, checked
// 2026-07-12) ships no webhook tools at all, so an agent can wire up its own
// integrations here where it couldn't against Render. The signing secret is
// returned by create_webhook_endpoint once and never by any other tool — the
// same mint-once rule REST/GraphQL hold.

type listEndpointsArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"restrict the list to this workspace id (tea-…); omit to use the session's selected workspace, if any"`
}

type listEndpointsResult struct {
	Endpoints []endpointWire `json:"endpoints"`
	// EventTypes is the subscribable vocabulary, so an agent needn't guess
	// what create_webhook_endpoint accepts.
	EventTypes []string `json:"eventTypes"`
}

type createEndpointArgs struct {
	OwnerID    string   `json:"ownerId,omitempty" jsonschema:"the workspace id (tea-…) to create the endpoint in; omit to use the session's selected workspace, if any"`
	Name       string   `json:"name,omitempty" jsonschema:"a human display label; defaults to the URL when omitted"`
	URL        string   `json:"url" jsonschema:"the absolute http(s) destination bex POSTs signed event notifications to"`
	EventTypes []string `json:"eventTypes" jsonschema:"the event types to subscribe to, e.g. deploy_started, deploy_ended, service_suspended — list_webhook_endpoints returns the full vocabulary"`
}

type updateEndpointArgs struct {
	ID         string   `json:"id" jsonschema:"the webhook endpoint id (whk-…), as returned by list_webhook_endpoints"`
	OwnerID    string   `json:"ownerId,omitempty" jsonschema:"the workspace id (tea-…) the endpoint belongs to; omit to use the session's selected workspace, if any"`
	Name       string   `json:"name,omitempty" jsonschema:"new display label; omit to keep the current value"`
	URL        string   `json:"url,omitempty" jsonschema:"new destination URL (must be absolute https or http); omit to keep the current value"`
	EventTypes []string `json:"eventTypes,omitempty" jsonschema:"new subscription list (replaces current); omit to keep the current value"`
	Enabled    *bool    `json:"enabled,omitempty" jsonschema:"enable or disable the endpoint; omit to keep the current state"`
}

type deleteEndpointArgs struct {
	ID      string `json:"id" jsonschema:"the webhook endpoint id (whk-…), as returned by list_webhook_endpoints"`
	OwnerID string `json:"ownerId,omitempty" jsonschema:"the workspace id (tea-…) the endpoint belongs to; omit to use the session's selected workspace, if any"`
}

type deletedResult struct {
	Deleted bool `json:"deleted"`
}

// RegisterMCP adds the outbound-webhook tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_webhook_endpoints",
		Description: "List the workspace's outbound webhook endpoints (URL, subscribed event types, enabled state — never the signing secret) plus the subscribable event-type vocabulary. bex extension — Render's own MCP server has no webhook tools.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listEndpointsArgs) (*mcp.CallToolResult, listEndpointsResult, error) {
		views, err := s.List(ctx, core.SelectedWorkspace(s.Selections, req, in.OwnerID))
		if err != nil {
			return nil, listEndpointsResult{}, err
		}
		return nil, listEndpointsResult{Endpoints: toWireList(views), EventTypes: EventTypes}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_webhook_endpoint",
		Description: "Register an outbound webhook: bex will POST a signed, thin JSON payload ({type, timestamp, data}) to the URL whenever a subscribed event happens (deploys, service lifecycle, scaling, cron runs, and sourceable Postgres/Key Value changes). The response includes the Standard-Webhooks signing secret exactly once — store it; it is not retrievable afterwards.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createEndpointArgs) (*mcp.CallToolResult, endpointWire, error) {
		v, err := s.Create(ctx, CreateRequest{
			OwnerID: core.SelectedWorkspace(s.Selections, req, in.OwnerID),
			Name:    in.Name, URL: in.URL, EventTypes: in.EventTypes,
		})
		if err != nil {
			return nil, endpointWire{}, err
		}
		return nil, toWire(v), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_webhook_endpoint",
		Description: "Update an outbound webhook endpoint's name, destination URL, event subscription, or enabled state. Supply only the fields to change; omitted fields keep their current values.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in updateEndpointArgs) (*mcp.CallToolResult, endpointWire, error) {
		v, err := s.Update(ctx, core.SelectedWorkspace(s.Selections, req, in.OwnerID), in.ID, UpdateRequest{
			Name:       in.Name,
			URL:        in.URL,
			EventTypes: in.EventTypes,
			Enabled:    in.Enabled,
		})
		if err != nil {
			return nil, endpointWire{}, err
		}
		return nil, toWire(v), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_webhook_endpoint",
		Description: "Delete an outbound webhook endpoint and its delivery history. No further events are sent to it.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in deleteEndpointArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.Delete(ctx, core.SelectedWorkspace(s.Selections, req, in.OwnerID), in.ID)
		return nil, deletedResult{Deleted: err == nil}, err
	})
}
