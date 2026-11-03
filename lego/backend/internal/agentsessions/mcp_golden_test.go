package agentsessions

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var updateGolden = flag.Bool("update-mcp-golden", false, "rewrite testdata/mcp_tools.golden.json")

// TestAgentMCPToolsGolden pins every agent-session MCP tool's name,
// description, input schema and annotations (ADR094 D2). Any edit to the
// contract must regenerate the golden file in the same change, so the docs and
// the server cannot drift apart silently. Regenerate with:
//
//	go test ./internal/agentsessions -run TestAgentMCPToolsGolden -update-mcp-golden
func TestAgentMCPToolsGolden(t *testing.T) {
	tools := listAgentTools(t)
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	got, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	const path = "testdata/mcp_tools.golden.json"
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update-mcp-golden): %v", err)
	}
	if string(want) != string(got) {
		t.Fatalf("agent MCP tool contract drifted from %s; if intended, regenerate with -update-mcp-golden and update the docs", path)
	}
}

// TestOnlyAttachMentionsTickets guards the ADR094 drift fix: spawn and resume
// never mint an attach ticket, so their descriptions must not claim to.
func TestOnlyAttachMentionsTickets(t *testing.T) {
	tools := listAgentTools(t)
	for _, tool := range tools {
		if tool.Name == "spawn_agent_session" || tool.Name == "resume_agent_session" {
			for _, bad := range []string{"and return its sandbox attach ticket", "and mint a fresh attach ticket"} {
				if strings.Contains(tool.Description, bad) {
					t.Errorf("%s still claims to mint a ticket: %q", tool.Name, tool.Description)
				}
			}
		}
	}
}

// TestCanonicalAndAliasToolsShareOneContract: ADR094 D2 renames the create and
// follow-up tools; the old names must stay callable with identical input
// schemas, so a client written against either spelling keeps working.
func TestCanonicalAndAliasToolsShareOneContract(t *testing.T) {
	tools := listAgentTools(t)
	byName := map[string]*mcp.Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	for canonical, alias := range map[string]string{
		"create_agent_session":       "spawn_agent_session",
		"send_agent_session_message": "steer_agent_session",
	} {
		c, a := byName[canonical], byName[alias]
		if c == nil || a == nil {
			t.Fatalf("missing %s (%v) or alias %s (%v)", canonical, c != nil, alias, a != nil)
		}
		cin, _ := json.Marshal(c.InputSchema)
		ain, _ := json.Marshal(a.InputSchema)
		if string(cin) != string(ain) {
			t.Errorf("%s and %s input schemas differ:\n%s\n%s", canonical, alias, cin, ain)
		}
		cout, _ := json.Marshal(c.OutputSchema)
		aout, _ := json.Marshal(a.OutputSchema)
		if string(cout) != string(aout) {
			t.Errorf("%s and %s output schemas differ", canonical, alias)
		}
	}
}

// listAgentTools registers the agent tools on a throwaway server and returns
// what a client sees from tools/list.
func listAgentTools(t *testing.T) []*mcp.Tool {
	t.Helper()
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	(&Service{}).RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	return listed.Tools
}
