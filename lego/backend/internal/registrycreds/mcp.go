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

package registrycreds

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the registry-credentials MCP fragment. A bex superset — Render's
// official MCP server (render-oss/render-mcp-server, checked live 2026-07-13)
// ships no registry-credential tools at all, so this is new surface, not a
// tracked Render tool name. list_/create_/update_/delete_ follows the
// existing list_services/create_web_service naming convention. The secret
// (authToken) is accepted on create/update but never returned by any tool —
// the same "write-only past creation" rule REST/GraphQL hold.

type listCredentialsArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"restrict the list to this workspace id (tea-…); omit to use the session's selected workspace, if any"`
}

type listCredentialsResult struct {
	Credentials []credentialWire `json:"credentials"`
}

type getCredentialArgs struct {
	ID string `json:"id" jsonschema:"the registry credential id, as returned by list_registry_credentials"`
}

type createCredentialArgs struct {
	OwnerID   string `json:"ownerId,omitempty" jsonschema:"the workspace id (tea-…) to create the credential in; omit to use the session's selected workspace, if any"`
	Name      string `json:"name,omitempty" jsonschema:"a human display label; defaults to host when omitted"`
	Host      string `json:"host" jsonschema:"the registry hostname the credential authenticates to, e.g. ghcr.io, docker.io, registry.gitlab.com"`
	Username  string `json:"username" jsonschema:"the registry username"`
	AuthToken string `json:"authToken" jsonschema:"the registry password or access token — stored, never returned by any tool after this call"`
	ExpiresAt string `json:"expiresAt,omitempty" jsonschema:"RFC3339 timestamp the credential expires at, if it does (e.g. a short-lived AWS ECR token); omit for a credential that never expires"`
}

type updateCredentialArgs struct {
	ID   string  `json:"id" jsonschema:"the registry credential id, as returned by list_registry_credentials"`
	Name *string `json:"name,omitempty" jsonschema:"new display label; omit to leave unchanged"`
	// Username/AuthToken/ExpiresAt use the MCP go-sdk's own omission handling
	// (an absent optional field decodes as nil) — the same nil-means-unchanged
	// convention as name.
	Username  *string `json:"username,omitempty" jsonschema:"new registry username; omit to leave unchanged"`
	AuthToken *string `json:"authToken,omitempty" jsonschema:"new registry password/token to rotate to; omit to leave the stored secret unchanged"`
	ExpiresAt *string `json:"expiresAt,omitempty" jsonschema:"new RFC3339 expiry; pass an empty string to clear the expiry, omit to leave unchanged"`
}

type deleteCredentialArgs struct {
	ID string `json:"id" jsonschema:"the registry credential id, as returned by list_registry_credentials"`
}

type deletedResult struct {
	Deleted bool `json:"deleted"`
}

// RegisterMCP adds the registry-credentials tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_registry_credentials",
		Description: "List the workspace's stored external image-registry credentials (host, username, expiry status — never the secret). Use this before create_registry_credential to check whether a host is already configured.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listCredentialsArgs) (*mcp.CallToolResult, listCredentialsResult, error) {
		views, err := s.List(ctx, in.OwnerID)
		if err != nil {
			return nil, listCredentialsResult{}, err
		}
		return nil, listCredentialsResult{Credentials: toWireList(views)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_registry_credential",
		Description: "Get one stored registry credential by id (host, username, expiry status — never the secret).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getCredentialArgs) (*mcp.CallToolResult, credentialWire, error) {
		v, err := s.Get(ctx, in.ID)
		if err != nil {
			return nil, credentialWire{}, err
		}
		return nil, toWire(v), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_registry_credential",
		Description: "Store a credential for a private external image registry (Docker Hub, GHCR, GitLab Container Registry, ECR, etc.), so a create_web_service call whose image references that host can pull successfully. bex extension — Render's own MCP server has no registry-credential tools.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createCredentialArgs) (*mcp.CallToolResult, credentialWire, error) {
		expiresAt, err := parseExpiresAt(in.ExpiresAt)
		if err != nil {
			return nil, credentialWire{}, err
		}
		v, err := s.Create(ctx, CreateRequest{
			OwnerID: in.OwnerID, Name: in.Name, Host: in.Host, Username: in.Username,
			Secret: in.AuthToken, ExpiresAt: expiresAt,
		})
		if err != nil {
			return nil, credentialWire{}, err
		}
		return nil, toWire(v), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_registry_credential",
		Description: "Change a stored registry credential's name/username/expiry, and/or rotate its secret. Omitted fields are left unchanged; pass expiresAt as an empty string to clear an expiry.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateCredentialArgs) (*mcp.CallToolResult, credentialWire, error) {
		req := UpdateRequest{Name: in.Name, Username: in.Username, Secret: in.AuthToken}
		if in.ExpiresAt != nil {
			expiresAt, err := parseExpiresAt(*in.ExpiresAt)
			if err != nil {
				return nil, credentialWire{}, err
			}
			req.ExpiresAtSet = true
			req.ExpiresAt = expiresAt
		}
		v, err := s.Update(ctx, in.ID, req)
		if err != nil {
			return nil, credentialWire{}, err
		}
		return nil, toWire(v), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_registry_credential",
		Description: "Delete a stored registry credential. Services already using the credential's materialized pull secret are unaffected until their next deploy re-resolves it.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deleteCredentialArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.Delete(ctx, in.ID)
		return nil, deletedResult{Deleted: err == nil}, err
	})
}
