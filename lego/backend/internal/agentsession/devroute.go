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

package agentsession

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Model routes (ADR090) point the claude profile at OpenRouter's
// Anthropic-compatible endpoint. agent-profiles.json is untouched; a route is
// resolved per workspace at create, dispatch and mint time, never from the
// endpoint a session stored, so a session created on api.anthropic.com moves
// onto (or off) a route on its next steer or provider exchange.
//
// Two routes exist:
//
//   - the DEV route (BEX_AGENT_DEV_MODEL_ROUTE) covers every workspace on a
//     dev control plane, uses the workspace's own BYO key, and is refused on a
//     production control plane (EnableDevModelRoute);
//   - the PLATFORM route (BEX_AGENT_PLATFORM_MODEL_ROUTE) is production-capable
//     but covers only an explicit workspace allowlist, injects a platform-owned
//     key (the workspace BYO key is never read) and pins the model: the gateway
//     refuses any request body whose top-level "model" differs.
//
// The platform route wins for its allowlisted workspaces; the dev route covers
// everything else on a dev control plane; with neither, behaviour is the
// release profile's.

const (
	// DevModelRouteOpenRouter is the one supported route value for both
	// BEX_AGENT_DEV_MODEL_ROUTE and BEX_AGENT_PLATFORM_MODEL_ROUTE.
	DevModelRouteOpenRouter = "openrouter"
	// DefaultDevModel is the OpenRouter model id a route selects when its model
	// env is unset: the newest DeepSeek flash snapshot listed by
	// GET https://openrouter.ai/api/v1/models on 2026-10-22.
	DefaultDevModel = "deepseek/deepseek-v4.1-flash-1017"

	// Claude Code appends /v1/messages to ANTHROPIC_BASE_URL. The base it sees
	// is the gateway model proxy, so the agent's request path stays
	// /v1/messages and the gateway prepends openRouterPathPrefix on the
	// upstream hop: https://openrouter.ai/api/v1/messages.
	openRouterEndpoint   = "https://" + OpenRouterHost + openRouterPathPrefix
	openRouterPathPrefix = "/api"
	openRouterAgent      = "claude"
	// OpenRouterHost is the upstream host a route mints; the gateway model
	// proxy admits it alongside api.anthropic.com.
	OpenRouterHost = "openrouter.ai"
)

// modelRoute is one active route. The agent, endpoint, host and path prefix
// are the openRouter* constants; only the model, scope and key vary.
type modelRoute struct {
	model string
	// workspaces is the platform route's allowlist; nil (the dev route) covers
	// every workspace.
	workspaces map[string]bool
	// platformKey is the platform-owned credential the mint injects instead of
	// the workspace BYO key. Empty for the dev route. Never logged or returned
	// outside ModelMintResponse.Credential.
	platformKey string
}

func (r *modelRoute) covers(workspaceID string) bool {
	return r != nil && (r.workspaces == nil || r.workspaces[workspaceID])
}

var (
	activeDevModelRoute      atomic.Pointer[modelRoute]
	activePlatformModelRoute atomic.Pointer[modelRoute]
	devModelIDPattern        = regexp.MustCompile(`^[a-z0-9~][a-z0-9._/:~-]{0,127}$`)
)

// EnableDevModelRoute installs (or, for an empty route, clears) the dev model
// route for this bex-api process. It refuses a production control plane: the
// identity must be a non-empty, non-"production" BEX_CP_IDENTITY (dev-N).
func EnableDevModelRoute(route, model, controlPlaneIdentity string) error {
	resolved, err := resolveDevModelRoute(route, model, controlPlaneIdentity)
	if err != nil {
		return err
	}
	activeDevModelRoute.Store(resolved)
	return nil
}

// ValidateDevModelRoute reports whether EnableDevModelRoute would accept the
// configuration, without installing it (bex-api's startup config check).
func ValidateDevModelRoute(route, model, controlPlaneIdentity string) error {
	_, err := resolveDevModelRoute(route, model, controlPlaneIdentity)
	return err
}

// resolveDevModelRoute expects route and model already trimmed (bex-api's
// config loader does).
func resolveDevModelRoute(route, model, controlPlaneIdentity string) (*modelRoute, error) {
	if route == "" {
		return nil, nil
	}
	if route != DevModelRouteOpenRouter {
		return nil, fmt.Errorf("BEX_AGENT_DEV_MODEL_ROUTE %q is not supported (only %q)", route, DevModelRouteOpenRouter)
	}
	identity := strings.TrimSpace(controlPlaneIdentity)
	if identity == "" || identity == store.DefaultControlPlaneIdentity {
		return nil, fmt.Errorf("BEX_AGENT_DEV_MODEL_ROUTE is dev-only and refused on the %q control plane (set BEX_CP_IDENTITY to a dev identity)", store.DefaultControlPlaneIdentity)
	}
	model, err := routeModel("BEX_AGENT_DEV_MODEL", model)
	if err != nil {
		return nil, err
	}
	return &modelRoute{model: model}, nil
}

// EnablePlatformModelRoute installs (or, for an empty route, clears) the
// workspace-allowlisted platform model route. workspaces must be non-empty
// and key non-empty when route is set; the caller (bex-api's config loader)
// validates workspace id shape. Errors never include the key.
func EnablePlatformModelRoute(route, model string, workspaces []string, key string) error {
	resolved, err := resolvePlatformModelRoute(route, model, workspaces)
	if err != nil {
		return err
	}
	if resolved != nil {
		if resolved.platformKey = strings.TrimSpace(key); resolved.platformKey == "" {
			return fmt.Errorf("BEX_AGENT_PLATFORM_MODEL_ROUTE requires BEX_AGENT_PLATFORM_MODEL_KEY")
		}
	}
	activePlatformModelRoute.Store(resolved)
	return nil
}

// ValidatePlatformModelRoute checks the route, model and allowlist without
// the key and without installing anything (bex-api's startup config check;
// a missing key is a degraded-but-running state the loader handles itself).
func ValidatePlatformModelRoute(route, model string, workspaces []string) error {
	_, err := resolvePlatformModelRoute(route, model, workspaces)
	return err
}

func resolvePlatformModelRoute(route, model string, workspaces []string) (*modelRoute, error) {
	if route == "" {
		return nil, nil
	}
	if route != DevModelRouteOpenRouter {
		return nil, fmt.Errorf("BEX_AGENT_PLATFORM_MODEL_ROUTE %q is not supported (only %q)", route, DevModelRouteOpenRouter)
	}
	model, err := routeModel("BEX_AGENT_PLATFORM_MODEL", model)
	if err != nil {
		return nil, err
	}
	allow := map[string]bool{}
	for _, ws := range workspaces {
		if ws = strings.TrimSpace(ws); ws != "" {
			allow[ws] = true
		}
	}
	if len(allow) == 0 {
		return nil, fmt.Errorf("BEX_AGENT_PLATFORM_MODEL_ROUTE requires a non-empty BEX_AGENT_PLATFORM_MODEL_WORKSPACES allowlist")
	}
	return &modelRoute{model: model, workspaces: allow}, nil
}

func routeModel(env, model string) (string, error) {
	if model == "" {
		model = DefaultDevModel
	}
	if !devModelIDPattern.MatchString(model) {
		return "", fmt.Errorf("%s %q is not a model id", env, model)
	}
	return model, nil
}

// routeFor is the route that applies to agent in workspaceID, or nil.
func routeFor(workspaceID, agent string) *modelRoute {
	if strings.ToLower(strings.TrimSpace(agent)) != openRouterAgent {
		return nil
	}
	if route := activePlatformModelRoute.Load(); route.covers(workspaceID) {
		return route
	}
	if route := activeDevModelRoute.Load(); route.covers(workspaceID) {
		return route
	}
	return nil
}

// PlatformModelKeyReady reports whether workspaceID's model credential is the
// platform route's key, so readiness need not (and must not) consult the
// workspace's BYO key. The platform route only serves the claude profile.
func PlatformModelKeyReady(workspaceID string) bool {
	return activePlatformModelRoute.Load().covers(workspaceID)
}

// registeredEndpoint is the endpoint a profile resolves to for workspaceID:
// the release manifest's, unless a route covers that workspace and profile.
func registeredEndpoint(workspaceID string, profile AgentProfile) string {
	if routeFor(workspaceID, profile.ID) != nil {
		return openRouterEndpoint
	}
	return profile.ModelEndpoint
}

// ModelRouteEnv is the extra sandbox environment a route needs for agent in
// workspaceID: the model id under every name Claude Code reads. Nil when no
// route applies.
func ModelRouteEnv(workspaceID, agent string) map[string]string {
	route := routeFor(workspaceID, agent)
	if route == nil {
		return nil
	}
	return map[string]string{
		"ANTHROPIC_MODEL":                route.model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   route.model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": route.model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  route.model,
	}
}
