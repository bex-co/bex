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

type listEndpointsResult struct {
	Endpoints []endpointWire `json:"endpoints"`
	// EventTypes is the subscribable vocabulary, so an agent needn't guess
	// what create_webhook_endpoint accepts.
	EventTypes []string `json:"eventTypes"`
}

type createEndpointArgs struct {
	Name       string   `json:"name" jsonschema:"a unique non-empty name within the workspace"`
	URL        string   `json:"url" jsonschema:"the absolute HTTPS destination bex POSTs signed event notifications to"`
	EventTypes []string `json:"eventTypes" jsonschema:"the event types to subscribe to, e.g. deploy_started, deploy_ended, service_suspended — list_webhook_endpoints returns the full vocabulary"`
	Enabled    bool     `json:"enabled" jsonschema:"whether delivery starts enabled"`
}

type listDeliveriesArgs struct {
	ID         string `json:"id" jsonschema:"the webhook endpoint id (whk-…), as returned by list_webhook_endpoints"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"opaque cursor from the final item of the previous page"`
	Limit      int    `json:"limit,omitempty" jsonschema:"maximum delivery records to return (default 20, maximum 100)"`
	SentAfter  string `json:"sentAfter,omitempty" jsonschema:"strict RFC3339 lower bound on attempt send time"`
	SentBefore string `json:"sentBefore,omitempty" jsonschema:"strict RFC3339 upper bound on attempt send time"`
	Status     string `json:"status,omitempty" jsonschema:"attempt outcome: delivered or failed; omit for all completed attempts"`
}

type listDeliveriesResult struct {
	Deliveries []DeliveryView `json:"deliveries"`
}

type resendDeliveryArgs struct {
	EndpointID     string `json:"endpointId" jsonschema:"required,the webhook endpoint id (whk-…)"`
	AttemptID      string `json:"attemptId" jsonschema:"required,the failed or delivered attempt id (whd-…) to resend"`
	IdempotencyKey string `json:"idempotencyKey" jsonschema:"required,8 to 128 safe characters; retry the same request with the same key"`
}

type updateEndpointArgs struct {
	ID         string   `json:"id" jsonschema:"the webhook endpoint id (whk-…), as returned by list_webhook_endpoints"`
	Name       *string  `json:"name,omitempty" jsonschema:"new non-empty display label; omit to keep the current value"`
	URL        *string  `json:"url,omitempty" jsonschema:"new absolute HTTPS destination URL; omit to keep the current value"`
	EventTypes []string `json:"eventTypes,omitempty" jsonschema:"new subscription list (replaces current); omit to keep the current value"`
	Enabled    *bool    `json:"enabled,omitempty" jsonschema:"enable or disable the endpoint; omit to keep the current state"`
}

type deleteEndpointArgs struct {
	ID string `json:"id" jsonschema:"the webhook endpoint id (whk-…), as returned by list_webhook_endpoints"`
}

type deletedResult struct {
	Deleted bool `json:"deleted"`
}

// RegisterMCP adds the outbound-webhook tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_webhook_endpoints",
		Description: "List the workspace's outbound webhook endpoints (URL, subscribed event types, enabled state — never the signing secret) plus the subscribable event-type vocabulary. bex extension — Render's own MCP server has no webhook tools.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listEndpointsResult, error) {
		views, err := s.List(ctx, core.NamedWorkspace(ctx))
		if err != nil {
			return nil, listEndpointsResult{}, core.MCPError(err)
		}
		return nil, listEndpointsResult{Endpoints: toWireList(views), EventTypes: EventTypes}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_webhook_endpoint",
		Description: "Register an outbound webhook: bex will POST a signed, thin JSON payload ({type, timestamp, data}) to the URL whenever a subscribed event happens (deploys, service lifecycle, scaling, cron runs, and sourceable Postgres/Key Value changes). The response includes the Standard-Webhooks signing secret exactly once — store it; it is not retrievable afterwards.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createEndpointArgs) (*mcp.CallToolResult, endpointWire, error) {
		v, err := s.Create(ctx, CreateRequest{
			OwnerID: core.NamedWorkspace(ctx),
			Name:    in.Name, URL: in.URL, EventTypes: in.EventTypes, Enabled: in.Enabled,
		})
		if err != nil {
			return nil, endpointWire{}, core.MCPError(err)
		}
		return nil, toWire(v), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_webhook_deliveries",
		Description: "List every immutable send attempt for an endpoint with its request body, bounded response/transport evidence, exact send time, parent retry state, and opaque cursor. bex extension — Render's own MCP server has no webhook tools.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listDeliveriesArgs) (*mcp.CallToolResult, listDeliveriesResult, error) {
		sentAfter, err := core.ParseTime("sentAfter", in.SentAfter)
		if err != nil {
			return nil, listDeliveriesResult{}, core.MCPError(err)
		}
		sentBefore, err := core.ParseTime("sentBefore", in.SentBefore)
		if err != nil {
			return nil, listDeliveriesResult{}, core.MCPError(err)
		}
		views, err := s.ListDeliveriesFiltered(ctx, core.NamedWorkspace(ctx), in.ID, DeliveryFilter{
			Cursor: in.Cursor, Limit: in.Limit, SentAfter: sentAfter, SentBefore: sentBefore, Status: in.Status,
		})
		if err != nil {
			return nil, listDeliveriesResult{}, core.MCPError(err)
		}
		return nil, listDeliveriesResult{Deliveries: views}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "resend_webhook_delivery",
		Description: "Queue one immediate manual attempt using the source event's byte-identical request body. The endpoint must be enabled. Reusing the idempotency key returns the same reservation and never fans out another send. bex extension — Render exposes Resend only in its dashboard.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in resendDeliveryArgs) (*mcp.CallToolResult, DeliveryView, error) {
		view, err := s.Resend(ctx, core.NamedWorkspace(ctx), in.EndpointID, in.AttemptID, in.IdempotencyKey)
		return nil, view, core.MCPError(err)
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_webhook_endpoint",
		Description: "Update an outbound webhook endpoint's name, destination URL, event subscription, or enabled state. Supply only the fields to change; omitted fields keep their current values.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateEndpointArgs) (*mcp.CallToolResult, endpointWire, error) {
		var eventTypes *[]string
		if in.EventTypes != nil {
			eventTypes = &in.EventTypes
		}
		v, err := s.Update(ctx, core.NamedWorkspace(ctx), in.ID, UpdateRequest{
			Name:       in.Name,
			URL:        in.URL,
			EventTypes: eventTypes,
			Enabled:    in.Enabled,
		})
		if err != nil {
			return nil, endpointWire{}, core.MCPError(err)
		}
		return nil, toWire(v), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_webhook_endpoint",
		Description: "Delete an outbound webhook endpoint and its delivery history. No further events are sent to it.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deleteEndpointArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.Delete(ctx, core.NamedWorkspace(ctx), in.ID)
		return nil, deletedResult{Deleted: err == nil}, core.MCPError(err)
	})
}
