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

package core

import (
	"context"
	"strings"
)

// Product surfaces: which way a resource was created (w5/m97).
//
// This answers ADR008's headline question — how much of the platform is driven
// by agents rather than people — which ActorType cannot. ActorType says whether
// a human or a machine credential authenticated; it says nothing about whether
// that human used the dashboard or the CLI, and an agent holding a human's
// OAuth grant still reads as human. The two axes are independent on purpose.
//
// A closed set, because it becomes a dashboard dimension and a CHECK
// constraint. SurfaceUnknown is an honest bucket: legacy backfill and anything
// reaching the recorder outside a classified request land there rather than
// being guessed at.
const (
	SurfaceDashboard = "dashboard"
	SurfaceCLI       = "cli"
	SurfaceMCP       = "mcp"
	SurfaceAPI       = "api"
	SurfaceBlueprint = "blueprint"
	SurfaceUnknown   = "unknown"
)

// cliUserAgentPrefix is what the imported Render CLI sends, and what the bex
// launcher deliberately preserves — the launcher adds its own release in a
// separate header (w5/m94) precisely so this stays a truthful client marker.
const cliUserAgentPrefix = "render-cli/"

// Transports are the API layer's own closed request classification, declared
// here rather than there because both the origin-metrics labels and the surface
// derivation below switch on them. One symbol per value is what actually keeps
// the two axes from drifting; two copies of the same literal would not.
const (
	TransportREST     = "rest"
	TransportGraphQL  = "graphql"
	TransportMCP      = "mcp"
	TransportAuth     = "auth"
	TransportInternal = "internal"
)

// RequestOrigin is what a request carries at the moment it arrives: its
// transport, and the client that sent it. Identity is deliberately absent —
// authentication happens further in, so the surface is resolved later, when
// both halves are known.
type RequestOrigin struct {
	// Transport is one of the Transport* constants above.
	Transport string
	UserAgent string
}

type requestOriginKey struct{}
type blueprintApplyKey struct{}

// WithRequestOrigin records how a request arrived. Set once, at the edge, by
// middleware that runs for every request — not by the metrics middleware, whose
// absence must never change what analytics records.
func WithRequestOrigin(ctx context.Context, origin RequestOrigin) context.Context {
	return context.WithValue(ctx, requestOriginKey{}, origin)
}

// WithBlueprintApply marks a context as executing a Blueprint apply, so every
// resource created underneath it is attributed to the Blueprint rather than to
// whichever transport happened to trigger the sync. That matters because the
// same engine runs from a manual REST/GraphQL/MCP call and from the webhook
// auto-sync worker, which is not an HTTP request at all.
func WithBlueprintApply(ctx context.Context) context.Context {
	return context.WithValue(ctx, blueprintApplyKey{}, true)
}

// ProductSurface resolves the surface for the request in ctx.
//
// Precedence is meaningful, not incidental:
//
//  1. A Blueprint apply wins over its trigger — the user declared intent in a
//     manifest; the transport that kicked it off is an implementation detail.
//  2. MCP is agent traffic by construction, whatever client library is used.
//  3. On REST, the CLI identifies itself in User-Agent.
//  4. On GraphQL, a browser session is the dashboard; GraphQL is the surface
//     the dashboard uses and sessions are how it authenticates.
//  5. Anything else that is still a product request is direct API use.
//
// Nothing here infers from workspace, plan, or actor type: a wrong attribution
// is worse than an honest unknown on a panel that exists to be believed.
func ProductSurface(ctx context.Context) string {
	if apply, ok := ctx.Value(blueprintApplyKey{}).(bool); ok && apply {
		return SurfaceBlueprint
	}
	origin, ok := ctx.Value(requestOriginKey{}).(RequestOrigin)
	if !ok {
		return SurfaceUnknown
	}
	switch origin.Transport {
	case TransportMCP:
		return SurfaceMCP
	case TransportREST:
		if strings.HasPrefix(origin.UserAgent, cliUserAgentPrefix) {
			return SurfaceCLI
		}
		return SurfaceAPI
	case TransportGraphQL:
		if identity, ok := IdentityFrom(ctx); ok && identity.Method == "session" {
			return SurfaceDashboard
		}
		return SurfaceAPI
	default:
		// auth and internal transports are not product traffic; a resource
		// effect arriving through one is unclassified rather than "api".
		return SurfaceUnknown
	}
}

// validSurface clamps anything outside the closed set. The column's CHECK would
// reject a stray value outright, which would turn an analytics detail into a
// failed resource creation — the one thing this recorder must never cause.
func validSurface(surface string) string {
	switch surface {
	case SurfaceDashboard, SurfaceCLI, SurfaceMCP, SurfaceAPI, SurfaceBlueprint:
		return surface
	default:
		return SurfaceUnknown
	}
}
