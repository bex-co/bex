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

package api

import (
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// w9/125: the SSH gateway holds a stream a bex API key opened to the action
// its ticket names, and each mint takes that action from its package's
// constant. Every operation that mints such a ticket must need exactly that
// action, or a key could open a stream under one permission and keep it
// open under another.
func TestStreamTicketMintsNeedTheActionTheGatewayHoldsThemTo(t *testing.T) {
	for op, held := range map[string]core.KeyAction{
		"REST POST /v1/services/{id}/shell-ticket":             apps.ShellKeyAction,
		"GQL Mutation.createShellSession":                      apps.ShellKeyAction,
		"REST POST /v1/sandboxes/{id}/exec":                    sandbox.ExecKeyAction,
		"REST POST /v1/sandboxes/{id}/runs/{operation}/token":  sandbox.ExecKeyAction,
		"MCP sandbox_exec":                                     sandbox.ExecKeyAction,
		"REST POST /v1/sandboxes/{id}/files/{operation}/token": sandbox.FileKeyAction,
		"REST POST /v1/agent-sessions/{id}/attach-ticket":      agentsessions.AttachKeyAction,
		"GQL Mutation.attachAgentSession":                      agentsessions.AttachKeyAction,
		"MCP attach_agent_session":                             agentsessions.AttachKeyAction,
	} {
		if needed := keyPermissionOps[op]; !slices.Equal(needed, []core.KeyAction{held}) {
			t.Errorf("%s needs %v, but the gateway holds the stream it opens to %q", op, needed, held)
		}
	}
}
