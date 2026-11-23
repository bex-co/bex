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

package mcputil_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

type args struct{}

// callTool registers one tool whose handler returns handlerErr, drives a real
// in-memory client/server pair through tools/call, and returns the text the
// agent sees. Going through the SDK (rather than calling the handler directly)
// is the point: the SDK converts a handler error into a result BEFORE any
// middleware runs, which is why the wrap has to happen at registration.
func callTool(t *testing.T, register func(*mcp.Server, *mcp.Tool, mcp.ToolHandlerFor[args, any]), handlerErr error) string {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	register(srv, &mcp.Tool{Name: "failing_tool", Description: "always fails"}, func(context.Context, *mcp.CallToolRequest, args) (*mcp.CallToolResult, any, error) {
		return nil, nil, handlerErr
	})

	ct, st := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(t.Context(), st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(t.Context(), ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "failing_tool"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result")
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(text.Text)
		}
	}
	return sb.String()
}

// TestAddToolCarriesTheCodeAgentsParse is the regression for the gap this
// milestone closed: 130 of 175 tools returned a bare message, so an agent could
// not tell a payment failure from any other refusal even though REST returns
// the code in `params` and GraphQL in `extensions`.
func TestAddToolCarriesTheCodeAgentsParse(t *testing.T) {
	coded := core.NewBadRequestError("PAYMENT_REQUIRED", "a payment method is required", map[string]any{"plan": "starter"})

	got := callTool(t, mcputil.AddTool[args, any], coded)
	if !strings.Contains(got, "PAYMENT_REQUIRED") {
		t.Fatalf("tool error %q dropped the machine-readable code", got)
	}

	// Pinning the raw SDK registration documents exactly what the seam adds. If
	// this ever fails, the SDK started carrying the code itself and the seam can
	// be retired.
	raw := callTool(t, mcp.AddTool[args, any], coded) //nolint:forbidigo // the premise under test
	if strings.Contains(raw, "PAYMENT_REQUIRED") {
		t.Fatal("raw mcp.AddTool now carries the code by itself; the seam can be retired")
	}
}

// TestAddToolRedactsUnclassifiedErrors verifies the wrap does not decorate an
// unclassified error with a bogus code AND, per the security-audit run-1 parity
// fix, redacts its raw text (which could carry pgx/Kubernetes internals) to a
// generic message — matching WriteErr (REST) and the GraphQL sanitizer.
func TestAddToolRedactsUnclassifiedErrors(t *testing.T) {
	got := callTool(t, mcputil.AddTool[args, any], errors.New(`pq: constraint "x" host=10.0.0.5`))
	if got != "internal error" {
		t.Fatalf("unclassified error surfaced as %q, want %q", got, "internal error")
	}
}

// TestAddToolRecoversHandlerPanic pins the transport-boundary panic isolation
// the seam adds. The MCP SDK runs a tool handler with no recover of its own, so
// a handler panic would otherwise crash the whole bex-api process — a crash loop
// any authenticated MCP client could trigger — rather than fail the single call
// the way REST (net/http) and GraphQL (graphql-go) already do. A panicking
// handler must surface as the redacted generic "internal error" (its raw text
// never leaking), and the process must stay up to serve the next call.
func TestAddToolRecoversHandlerPanic(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	mcputil.AddTool(srv, &mcp.Tool{Name: "panicking_tool", Description: "always panics"},
		func(context.Context, *mcp.CallToolRequest, args) (*mcp.CallToolResult, any, error) {
			panic(`boom: pq: constraint "x" host=10.0.0.5`)
		})

	ct, st := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(t.Context(), st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(t.Context(), ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "panicking_tool"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result from a panicking handler")
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(text.Text)
		}
	}
	if got := sb.String(); got != "internal error" {
		t.Fatalf("panic surfaced as %q, want redacted %q", got, "internal error")
	}
}

// TestAddToolDoesNotDoubleWrap protects the six adapters that already called
// core.MCPError themselves: their output must stay byte-identical rather than
// becoming "CODE: CODE: msg".
func TestAddToolDoesNotDoubleWrap(t *testing.T) {
	coded := core.NewBadRequestError("PLAN_LIMIT", "at the plan's limit", nil)

	viaSeam := callTool(t, mcputil.AddTool[args, any], coded)
	alreadyWrapped := callTool(t, mcputil.AddTool[args, any], core.MCPError(coded))
	if viaSeam != alreadyWrapped {
		t.Fatalf("handler-wrapped error differs from seam-wrapped: %q vs %q", alreadyWrapped, viaSeam)
	}
	if strings.Count(alreadyWrapped, "PLAN_LIMIT") != 1 {
		t.Fatalf("code appears more than once: %q", alreadyWrapped)
	}
}

// w5/m286: every tool call runs under core.RequestBudget, and a call that runs
// out of it answers REQUEST_TIMEOUT.
func TestAddToolBoundsTheCallWithTheRequestBudget(t *testing.T) {
	var deadline bool
	got := callTool(t, func(s *mcp.Server, tool *mcp.Tool, _ mcp.ToolHandlerFor[args, any]) {
		mcputil.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, _ args) (*mcp.CallToolResult, any, error) {
			_, deadline = ctx.Deadline()
			return nil, nil, context.DeadlineExceeded
		})
	}, nil)
	if !deadline {
		t.Error("the tool handler ran without a deadline")
	}
	if !strings.HasPrefix(got, core.RequestTimeoutCode+": ") {
		t.Fatalf("timed-out tool answered %q, want REQUEST_TIMEOUT", got)
	}
}

func TestDefaultAnnotations(t *testing.T) {
	cases := []struct {
		name        string
		readOnly    bool
		destructive bool
		idempotent  bool
		openWorld   bool
	}{
		{"get_service", true, false, true, false},
		{"list_services", true, false, true, false},
		{"query_render_postgres", true, false, true, false},
		{"describe_plan", true, false, true, false},
		{"search_logs", true, false, true, false},
		{"preview_blueprint", true, false, true, false},
		{"estimate_cost", true, false, true, false},
		{"validate_bex_yml", true, false, true, false},
		{"create_web_service", false, false, false, true},
		{"add_custom_domain", false, false, false, true},
		{"generate_token", false, false, false, true},
		{"clone_env_group", false, false, false, true},
		{"import_sandbox_image", false, false, false, true},
		{"invite_workspace_member", false, false, false, true},
		{"spawn_sandbox", false, false, false, true},
		{"resume_service", false, false, true, true},
		{"delete_service", false, true, false, true},
		{"update_service", false, true, false, true},
		{"trigger_deploy", false, true, false, true},
		{"suspend_service", false, true, false, true},
	}

	for _, tc := range cases {
		ann := mcputil.DefaultAnnotations(tc.name)
		if ann == nil {
			t.Fatalf("DefaultAnnotations(%q) = nil", tc.name)
		}
		if ann.DestructiveHint == nil || ann.OpenWorldHint == nil {
			t.Fatalf("DefaultAnnotations(%q) returned nil pointer hints: %+v", tc.name, ann)
		}
		if ann.ReadOnlyHint != tc.readOnly || *ann.DestructiveHint != tc.destructive ||
			ann.IdempotentHint != tc.idempotent || *ann.OpenWorldHint != tc.openWorld {
			t.Errorf("DefaultAnnotations(%q) = {readOnly:%v destructive:%v idempotent:%v openWorld:%v}, want {readOnly:%v destructive:%v idempotent:%v openWorld:%v}",
				tc.name, ann.ReadOnlyHint, *ann.DestructiveHint, ann.IdempotentHint, *ann.OpenWorldHint,
				tc.readOnly, tc.destructive, tc.idempotent, tc.openWorld)
		}
	}
}

func TestAnnotationConstructors(t *testing.T) {
	ro := mcputil.ReadOnlyAnnotations()
	if !ro.ReadOnlyHint || !ro.IdempotentHint || ro.DestructiveHint == nil || *ro.DestructiveHint || ro.OpenWorldHint == nil || *ro.OpenWorldHint {
		t.Errorf("ReadOnlyAnnotations() = %+v", ro)
	}
	nd := mcputil.NonDestructiveAnnotations(true, false)
	if nd.ReadOnlyHint || !nd.IdempotentHint || nd.DestructiveHint == nil || *nd.DestructiveHint || nd.OpenWorldHint == nil || *nd.OpenWorldHint {
		t.Errorf("NonDestructiveAnnotations(true, false) = %+v", nd)
	}
	d := mcputil.DestructiveAnnotations(true)
	if d.ReadOnlyHint || !d.IdempotentHint || d.DestructiveHint == nil || !*d.DestructiveHint || d.OpenWorldHint == nil || !*d.OpenWorldHint {
		t.Errorf("DestructiveAnnotations(true) = %+v", d)
	}
}
