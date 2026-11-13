package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func oauthID(scopes string) core.Identity {
	return core.Identity{Subject: "u", Method: "oauth2", Human: true, CanonicalScopes: scopes}
}

// bex.cloud_agents covers the cloud-agents MCP toolset and nothing else
// (ADR094 D9): a token holding only that scope can run agents but cannot touch
// services, env vars or secrets, and bex.read/bex.write behave as before.
func TestCloudAgentsScopeCoversOnlyTheToolset(t *testing.T) {
	srv := &Server{}
	cases := []struct {
		name   string
		scopes string
		op     string
		allow  bool
	}{
		{"create agent", core.ScopeCloudAgents, "MCP create_agent_session", true},
		{"message agent", core.ScopeCloudAgents, "MCP send_agent_session_message", true},
		{"cancel agent", core.ScopeCloudAgents, "MCP cancel_agent_session", true},
		{"read transcript", core.ScopeCloudAgents, "MCP get_agent_session_transcript", true},
		{"list repos", core.ScopeCloudAgents, "MCP list_repos", true},
		{"delete service", core.ScopeCloudAgents, "MCP delete_service", false},
		{"set env var", core.ScopeCloudAgents, "MCP set_env_var", false},
		{"archive agent stays off the toolset", core.ScopeCloudAgents, "MCP archive_agent_session", false},
		{"update agent environment is admin", core.ScopeCloudAgents, "MCP update_agent_environment", false},
		{"REST is not covered", core.ScopeCloudAgents, "REST POST /v1/agent-sessions", false},
		{"write still covers create", core.ScopeWrite, "MCP create_agent_session", true},
		{"read cannot create", core.ScopeRead, "MCP create_agent_session", false},
		{"read covers get", core.ScopeRead, "MCP get_agent_session", true},
	}
	for _, c := range cases {
		ctx := core.WithIdentity(t.Context(), oauthID(c.scopes))
		err := srv.requireScopeClass(ctx, c.op)
		if (err == nil) != c.allow {
			t.Errorf("%s: scopes=%q op=%q err=%v, want allow=%t", c.name, c.scopes, c.op, err, c.allow)
		}
	}
}

// TestMCPCloudAgentsScopeReachesTheRelationGate pins the second half of ADR094
// D9: the class gate lets bex.cloud_agents through for the toolset, and the MCP
// seam must carry that same narrowing onto the context the service verb
// authorizes with — otherwise every create/operate/view inside a session
// refuses with insufficient_token_scope for a scope the caller did consent to.
// Only a covered tool is marked; an unrelated tool, a token without the scope,
// and exempt identities are left untouched.
func TestMCPCloudAgentsScopeReachesTheRelationGate(t *testing.T) {
	cases := []struct {
		name   string
		id     core.Identity
		tool   string
		marked bool
	}{
		{"write tool in the toolset", oauthID(core.ScopeCloudAgents), "create_agent_session", true},
		{"read tool in the toolset", oauthID(core.ScopeCloudAgents), "get_agent_session", true},
		{"caller-scoped toolset lookup", oauthID(core.ScopeCloudAgents), "list_workspaces", true},
		{"tool outside the toolset", oauthID(core.ScopeCloudAgents), "update_service", false},
		{"read token on a write tool", oauthID(core.ScopeRead), "create_agent_session", false},
		{"write token keeps its own grant", oauthID(core.ScopeWrite), "create_agent_session", false},
		{"session is exempt and untouched", core.Identity{Subject: "u", Method: "session", Human: true}, "create_agent_session", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := core.WithIdentity(context.Background(), tc.id)
			req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: tc.tool, Arguments: json.RawMessage(`{}`)}}
			var seen core.Identity
			next := func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
				seen, _ = core.IdentityFrom(ctx)
				return &mcp.CallToolResult{}, nil
			}
			if _, err := mcpBindCallWorkspace(ctx, &core.Base{}, "tools/call", req, next, nil); err != nil {
				t.Fatalf("bind: %v", err)
			}
			if seen.CloudAgentsToolset != tc.marked {
				t.Errorf("CloudAgentsToolset = %t, want %t", seen.CloudAgentsToolset, tc.marked)
			}
			if seen.CanonicalScopes != tc.id.CanonicalScopes {
				t.Errorf("scopes rewritten: %q, want %q", seen.CanonicalScopes, tc.id.CanonicalScopes)
			}
		})
	}
}

// TestCloudAgentsMarkerClearsTheAgentSessionVerb proves the end-to-end
// sequence the MCP seam performs: mark, then the verb's own relation gate. The
// fake checker allows so only the OAuth half is under test.
func TestCloudAgentsMarkerClearsTheAgentSessionVerb(t *testing.T) {
	base := &core.Base{Authz: &fakeChecker{allow: true}, Workspace: fakeWorkspace{"u": "tea-a"}}
	id := oauthID(core.ScopeCloudAgents)
	ctx := core.WithIdentity(context.Background(), id)
	ctx = mcpMarkCloudAgentsToolset(ctx, "create_agent_session")
	if err := base.Authorize(ctx, core.RelCanCreate); err != nil {
		t.Fatalf("marked cloud-agents create: %v", err)
	}
	if err := base.Authorize(core.WithIdentity(context.Background(), id), core.RelCanCreate); err == nil {
		t.Fatal("unmarked token cleared can_create; the marker must be what bridges the toolset")
	}
}

func TestCloudAgentsScopeIsAdvertisedAndKept(t *testing.T) {
	found := false
	for _, s := range core.AdvertisedScopes() {
		found = found || s == core.ScopeCloudAgents
	}
	if !found {
		t.Fatalf("scopes_supported %v lacks %s", core.AdvertisedScopes(), core.ScopeCloudAgents)
	}
	grant, err := core.NormalizeOAuthGrant("openid bex.cloud_agents bex.bogus", nil, "c", "")
	if err != nil || grant.Scopes != core.ScopeCloudAgents || !grant.HasGranular() {
		t.Fatalf("grant = %+v err=%v; want only bex.cloud_agents kept and counted as granular", grant, err)
	}
}
