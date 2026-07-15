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

package notifications

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the notifications MCP fragment (bex extension — an agent can read
// and tune its own deploy-notification preferences the same way it can check
// usage, pillar 3).

type updateSettingsArgs struct {
	DeployStarted   bool `json:"deployStarted" jsonschema:"email me when one of my services' deploys starts"`
	DeploySucceeded bool `json:"deploySucceeded" jsonschema:"email me when one of my services' deploys succeeds"`
	DeployFailed    bool `json:"deployFailed" jsonschema:"email me when one of my services' deploys fails"`
}

// RegisterMCP adds the notification-settings tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_notification_settings",
		Description: "Get the caller's own deploy-notification email preferences for their workspace. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, SettingsView, error) {
		v, err := s.GetSettings(ctx)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_notification_settings",
		Description: "Update the caller's own deploy-notification email preferences for their workspace. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateSettingsArgs) (*mcp.CallToolResult, SettingsView, error) {
		v, err := s.UpdateSettings(ctx, in.DeployStarted, in.DeploySucceeded, in.DeployFailed)
		return nil, v, err
	})
}
