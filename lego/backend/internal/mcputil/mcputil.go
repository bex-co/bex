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
	"errors"
	"log"
	"runtime/debug"

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
		res, out, err = h(ctx, req, in)
		return res, out, core.MCPError(err)
	})
}
