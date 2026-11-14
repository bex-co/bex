package api

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func toolsetSession(t *testing.T, srv *Server, name string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	toolset := srv.MCPToolsetServer(name)
	if toolset == nil {
		t.Fatalf("no toolset %q", name)
	}
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := toolset.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// The cloud-agents mount lists exactly its toolset (ADR094 D1/D2), and every
// name in it is a tool /mcp really registers — the narrow endpoint is a filter
// of the one registry, so it cannot name a tool that does not exist.
func TestCloudAgentsToolsetListsExactlyItsTools(t *testing.T) {
	srv := fullyWiredServer()
	full := map[string]bool{}
	for _, tool := range listBexTools(t, srv).Tools {
		full[tool.Name] = true
	}
	listed, err := toolsetSession(t, srv, ToolsetCloudAgents).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tool := range listed.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)
	var want []string
	for name := range mcpToolsets[ToolsetCloudAgents] {
		if !full[name] {
			t.Errorf("toolset names %q, which /mcp does not register", name)
		}
		want = append(want, name)
	}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("cloud-agents tools = %v, want %v", got, want)
	}
	for _, banned := range []string{"delete_service", "update_env_vars", "archive_agent_session", "attach_agent_session", "update_agent_environment"} {
		for _, name := range got {
			if name == banned {
				t.Errorf("cloud-agents leaks %s", banned)
			}
		}
	}
}

func TestCloudAgentsToolsetRefusesToolsOutsideIt(t *testing.T) {
	srv := fullyWiredServer()
	res, err := toolsetSession(t, srv, ToolsetCloudAgents).CallTool(context.Background(), &mcp.CallToolParams{Name: "archive_agent_session", Arguments: map[string]any{"id": "ags-x"}})
	if err != nil {
		t.Fatal(err)
	}
	// The refusal must name the toolset, not merely be "some tool error": a
	// nonexistent session would fail inside the handler too, so a test that
	// only checks IsError would pass even with the toolset filter gone.
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	if want := `unknown tool "archive_agent_session" on this endpoint`; !res.IsError || !strings.Contains(text.String(), want) {
		t.Fatalf("archive_agent_session on /mcp/cloud-agents = %+v, want the toolset refusal %q", res, want)
	}
}

func TestCloudAgentsMountIsAuthGatedAndDiscoverable(t *testing.T) {
	base := &core.Base{Client: fakeClient(), Namespace: "default"}
	srv := NewServer(base, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.OAuthIssuer, srv.OAuthResource = "https://oauth.example", "https://api.example/mcp"
	h := buildHandler(t, srv)
	if w := do(t, h, http.MethodPost, "/mcp/cloud-agents", "", `{}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /mcp/cloud-agents = %d, want 401", w.Code)
	}
	w := do(t, h, http.MethodGet, "/.well-known/oauth-protected-resource/mcp/cloud-agents", "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"resource":"https://api.example/mcp"`) {
		t.Fatalf("metadata = %d %s", w.Code, w.Body.String())
	}
}

func TestUnknownToolsetIsNil(t *testing.T) {
	if NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{}).MCPToolsetServer("nope") != nil {
		t.Fatal("unknown toolset built a server")
	}
}
