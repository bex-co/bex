package api

import (
	"testing"

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
