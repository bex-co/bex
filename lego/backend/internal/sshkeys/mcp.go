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

package sshkeys

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/mcputil"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type createSSHKeyArgs struct {
	Name      string `json:"name" jsonschema:"descriptive name for this public key"`
	PublicKey string `json:"publicKey" jsonschema:"one OpenSSH authorized_keys public key record"`
}

type deleteSSHKeyArgs struct {
	ID string `json:"id" jsonschema:"SSH key id returned by list_ssh_keys"`
}

type listSSHKeysResult struct {
	SSHKeys []store.SSHKey `json:"sshKeys"`
}

type deleteSSHKeyResult struct {
	Deleted bool `json:"deleted"`
}

func (s *Service) RegisterMCP(server *mcp.Server) {
	mcputil.AddTool(server, &mcp.Tool{Name: "list_ssh_keys", Description: "List the caller's registered SSH public keys. bex extension over Render's MCP."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listSSHKeysResult, error) {
			keys, err := s.List(ctx)
			return nil, listSSHKeysResult{SSHKeys: keys}, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "add_ssh_key", Description: "Register one SSH public key for the caller. Private keys are never accepted. bex extension over Render's MCP."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createSSHKeyArgs) (*mcp.CallToolResult, store.SSHKey, error) {
			key, err := s.Create(ctx, in.Name, in.PublicKey)
			return nil, key, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "delete_ssh_key", Description: "Delete one of the caller's SSH public keys. bex extension over Render's MCP."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in deleteSSHKeyArgs) (*mcp.CallToolResult, deleteSSHKeyResult, error) {
			err := s.Delete(ctx, in.ID)
			return nil, deleteSSHKeyResult{Deleted: err == nil}, err
		})
}
