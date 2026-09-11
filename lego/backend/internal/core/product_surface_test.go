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
	"testing"
)

func originCtx(transport, userAgent string) context.Context {
	return WithRequestOrigin(context.Background(), RequestOrigin{Transport: transport, UserAgent: userAgent})
}

func TestProductSurfaceDerivation(t *testing.T) {
	sessionGraphQL := WithIdentity(originCtx("graphql", "Mozilla/5.0"), Identity{Subject: "u-1", Method: "session"})
	tokenGraphQL := WithIdentity(originCtx("graphql", "python-requests/2.31"), Identity{Subject: "u-1", Method: "oauth2"})

	for _, tc := range []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"mcp is agent traffic by construction", originCtx("mcp", "anything"), SurfaceMCP},
		{"cli identifies itself on rest", originCtx("rest", "render-cli/2.27.0 (macOS - 26.5.1)"), SurfaceCLI},
		{"other rest clients are direct api use", originCtx("rest", "curl/8.4.0"), SurfaceAPI},
		{"a browser session on graphql is the dashboard", sessionGraphQL, SurfaceDashboard},
		{"a token on graphql is direct api use", tokenGraphQL, SurfaceAPI},
		{"graphql without any identity is not the dashboard", originCtx("graphql", "Mozilla/5.0"), SurfaceAPI},
		{"auth traffic is not a product surface", originCtx("auth", "curl/8.4.0"), SurfaceUnknown},
		{"internal traffic is not a product surface", originCtx("internal", ""), SurfaceUnknown},
		{"a bare context is honestly unknown", context.Background(), SurfaceUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProductSurface(tc.ctx); got != tc.want {
				t.Errorf("ProductSurface = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBlueprintApplyOutranksItsTrigger is the precedence that matters most: the
// same engine runs from a manual REST/GraphQL/MCP sync and from the webhook
// auto-sync worker, which is not a request at all. Attributing those to their
// trigger would scatter one declared intent across three surfaces.
func TestBlueprintApplyOutranksItsTrigger(t *testing.T) {
	for _, transport := range []string{"rest", "graphql", "mcp", "internal"} {
		t.Run(transport, func(t *testing.T) {
			ctx := WithBlueprintApply(originCtx(transport, "render-cli/2.27.0"))
			if got := ProductSurface(ctx); got != SurfaceBlueprint {
				t.Errorf("ProductSurface = %q over %s, want blueprint", got, transport)
			}
		})
	}
	// And with no request context at all — the auto-sync worker's case.
	if got := ProductSurface(WithBlueprintApply(context.Background())); got != SurfaceBlueprint {
		t.Errorf("ProductSurface = %q for a context-less apply, want blueprint", got)
	}
}

// TestMCPOutranksTheClientUserAgent covers the other collision: an agent
// driving MCP through a CLI-shaped client is still MCP traffic.
func TestMCPOutranksTheClientUserAgent(t *testing.T) {
	if got := ProductSurface(originCtx("mcp", "render-cli/2.27.0")); got != SurfaceMCP {
		t.Errorf("ProductSurface = %q, want mcp to outrank the user agent", got)
	}
}

// TestSurfaceIsIndependentOfActorType pins that the two axes do not collapse
// into each other: a machine credential on the dashboard's transport and a
// human on the CLI's must each be classified by surface, not by who they are.
func TestSurfaceIsIndependentOfActorType(t *testing.T) {
	human := WithIdentity(originCtx("rest", "render-cli/2.27.0"), Identity{Subject: "u-1", Method: "session", Human: true})
	if got := ProductSurface(human); got != SurfaceCLI {
		t.Errorf("a human on the CLI = %q, want cli", got)
	}
	machine := WithIdentity(originCtx("mcp", ""), Identity{Subject: "key-1", Method: "oauth2"})
	if got := ProductSurface(machine); got != SurfaceMCP {
		t.Errorf("a machine on MCP = %q, want mcp", got)
	}
}

// TestValidSurfaceClamps guards the recorder against ever handing the column a
// value its CHECK would reject — which would turn an analytics detail into a
// failed resource creation, the one thing this path must never cause.
func TestValidSurfaceClamps(t *testing.T) {
	for _, ok := range []string{SurfaceDashboard, SurfaceCLI, SurfaceMCP, SurfaceAPI, SurfaceBlueprint} {
		if got := validSurface(ok); got != ok {
			t.Errorf("validSurface(%q) = %q, want it preserved", ok, got)
		}
	}
	for _, bad := range []string{"", "DASHBOARD", "tenant-chosen", "'; DROP TABLE--", SurfaceUnknown} {
		if got := validSurface(bad); got != SurfaceUnknown {
			t.Errorf("validSurface(%q) = %q, want unknown", bad, got)
		}
	}
}
