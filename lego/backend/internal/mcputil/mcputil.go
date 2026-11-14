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

// Package mcputil is the shared MCP tool-registration seam, the MCP counterpart
// to gqlutil: every feature registers its tools through AddTool so a single
// place decides what a tool's error looks like on the wire.
package mcputil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// AddTool registers an MCP tool, mapping the handler's error through
// core.MCPError so a *core.CodedError's stable code survives into the tool
// result, and isolating a handler panic to the one call rather than the process.
//
// The wrap must happen here, at registration, because the code cannot be
// recovered any later: the SDK's AddTool converts a handler error into a
// CallToolResult{IsError:true} carrying only err.Error() before any receiving
// middleware runs, so the handler's own return is the last place the typed
// error is readable. The signature mirrors mcp.AddTool, which every feature
// calls instead of the SDK directly.
func AddTool[In, Out any](s *mcp.Server, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	annotate(t)
	mcp.AddTool(s, t, func( //nolint:forbidigo // the seam itself: it wraps the handler, then delegates
		ctx context.Context, req *mcp.CallToolRequest, in In) (res *mcp.CallToolResult, out Out, err error) {
		// Transport-boundary panic isolation. REST (net/http's conn.serve) and
		// GraphQL (graphql-go's per-field resolve) already turn a handler panic
		// into a single-request 500 / field error; the MCP SDK invokes each tool
		// handler in a goroutine with no recover of its own, so without this one a
		// lone handler panic crashes the whole bex-api process — a crash loop any
		// authenticated MCP client could trigger. Convert it into the same
		// generic, text-redacted "internal error" core.MCPError returns for any
		// unclassified failure (parity with WriteErr); the raw panic value and
		// stack are logged here for diagnosis and never reach the client.
		defer func() {
			if p := recover(); p != nil {
				log.Printf("bex-api mcp: tool %q handler panic: %v\n%s", t.Name, p, debug.Stack())
				res, err = nil, errors.New("internal error")
			}
		}()
		ctx, cancel := context.WithTimeout(ctx, core.RequestBudget)
		defer cancel()
		res, out, err = h(ctx, req, in)
		return res, out, core.MCPError(err)
	})
}

// RenderText is the result of a tool whose answer must be the text Render's
// MCP server sends (w4/m378): json.Marshal of the value as the one text
// content, then any extra text items (a truncation notice), and no
// structuredContent, which Render's server never sends. Handlers registered
// with Out = any return it directly, so the SDK adds neither an output schema
// nor a second copy of the JSON. A list is a bare JSON array, never an
// object wrapping one: an agent or skill written against Render reads it
// unchanged. encoding/json writes the value once, so a number float64 cannot
// hold keeps every digit, where the SDK's typed output path re-encodes
// through float64 (w4/m158).
func RenderText(v any, extra ...string) (*mcp.CallToolResult, any, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err // unclassified: AddTool redacts it to "internal error"
	}
	content := []mcp.Content{&mcp.TextContent{Text: string(body)}}
	for _, text := range extra {
		content = append(content, &mcp.TextContent{Text: text})
	}
	return &mcp.CallToolResult{Content: content}, nil, nil
}

// RenderPlainText is a result whose one text content is a sentence, not JSON:
// Render's answer for an empty list_postgres_instances or list_key_value.
func RenderPlainText(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}

// RenderListWithCursor is Render's paged list text (list_deploys,
// list_events): the bare JSON array, then "\n\n cursor: " and the last
// item's cursor, or a quoted empty string ("") when the page is empty.
// items must be non-nil so an empty page reads [] rather than null.
func RenderListWithCursor[T any](items []T, cursor string) (*mcp.CallToolResult, any, error) {
	body, err := json.Marshal(items)
	if err != nil {
		return nil, nil, err
	}
	if cursor == "" {
		cursor = `""`
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{
		Text: string(body) + "\n\n cursor: " + cursor,
	}}}, nil, nil
}

// WithWarning is the CallToolResult a typed handler returns when a successful
// result carries a human warning: the JSON text block the SDK would have
// generated for out, then warning as its own text block, so an agent reading
// only the text still sees it. An empty warning returns nil — the SDK's
// default rendering, byte-identical to a handler that never called this. The
// SDK fills StructuredContent from out either way.
func WithWarning(out any, warning string) *mcp.CallToolResult {
	if warning == "" {
		return nil
	}
	body, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{
		&mcp.TextContent{Text: string(body)},
		&mcp.TextContent{Text: warning},
	}}
}

// BoolOrEnum reads an argument Render's MCP server types as a boolean while
// bex's REST contract spells it as a two-value string enum (w5/m145): true and
// false map onto ifTrue and ifFalse, the enum strings pass through, and
// anything else is a named bad request. Absent is "".
func BoolOrEnum(param string, v any, ifTrue, ifFalse string) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case bool:
		if x {
			return ifTrue, nil
		}
		return ifFalse, nil
	case string:
		return x, nil
	default:
		return "", fmt.Errorf("%w: %s must be a boolean (or %q/%q)", core.ErrBadRequest, param, ifTrue, ifFalse)
	}
}

// NumberOrString reads an argument Render's MCP server types as a number
// while bex's REST contract takes a string (a Postgres major version, say):
// a whole number becomes its decimal text, a string passes through, and
// anything else is a named bad request. Absent is "".
func NumberOrString(param string, v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case float64:
		if x != math.Trunc(x) {
			return "", fmt.Errorf("%w: %s must be a whole number", core.ErrBadRequest, param)
		}
		return strconv.FormatInt(int64(x), 10), nil
	case string:
		return x, nil
	default:
		return "", fmt.Errorf("%w: %s must be a number", core.ErrBadRequest, param)
	}
}

// annotate gives every tool the behavior hints an MCP client acts on
// (w5/m145). Without them the spec's defaults apply: every tool may be
// destructive and none is read-only, so a client prompts before each list or
// get. A tool that sets its own Annotations keeps them; otherwise the verb in
// its name decides, matching the values Render's MCP server sets on the tools
// bex shares with it (TestMCPAnnotationsMatchUpstream).
func annotate(t *mcp.Tool) {
	if t.Annotations == nil {
		t.Annotations = DefaultAnnotations(t.Name)
	}
}

// DefaultAnnotations is the behavior hint set for a tool named by verb:
// reads are read-only, idempotent and closed-world; creates are additive;
// every other write may be destructive.
func DefaultAnnotations(name string) *mcp.ToolAnnotations {
	verb, _, _ := strings.Cut(name, "_")
	switch verb {
	case "get", "list", "query", "describe", "search", "preview", "estimate", "validate":
		return &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)}
	case "create", "add", "generate":
		return &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(true)}
	default:
		return &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(true)}
	}
}

func boolPtr(b bool) *bool { return &b }

// Scoped is gqlutil.ScopeArg for an MCP tool: h runs in scope(ctx, idOf(in)),
// the context a feature resolves that id in (a build source's id scoped to
// its carrier, ADR097), and a scope refusal is the tool's error.
func Scoped[In, Out any](scope func(context.Context, string) (context.Context, error), idOf func(In) string, h mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		ctx, err := scope(ctx, idOf(in))
		if err != nil {
			var zero Out
			return nil, zero, err
		}
		return h(ctx, req, in)
	}
}
