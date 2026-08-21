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
