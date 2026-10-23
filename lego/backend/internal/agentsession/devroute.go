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

// The dev-only model route (ADR090) points the claude profile at
// OpenRouter's Anthropic-compatible endpoint so live legs can run on a cheap
// authorized key. It is never a release profile: agent-profiles.json is
// untouched, the route is off unless bex-api is started with
// BEX_AGENT_DEV_MODEL_ROUTE, and EnableDevModelRoute refuses it on a
// production control plane.

const (
	// DevModelRouteOpenRouter is the one supported BEX_AGENT_DEV_MODEL_ROUTE.
	DevModelRouteOpenRouter = "openrouter"
	// DefaultDevModel is the OpenRouter model id the route selects when
	// BEX_AGENT_DEV_MODEL is unset: the newest DeepSeek flash snapshot listed by
	// GET https://openrouter.ai/api/v1/models on 2026-10-22.
	DefaultDevModel = "deepseek/deepseek-v4.1-flash-1017"

	// Claude Code appends /v1/messages to ANTHROPIC_BASE_URL. The base it sees
	// is the gateway model proxy, so the agent's request path stays
	// /v1/messages and the gateway prepends openRouterPathPrefix on the
	// upstream hop: https://openrouter.ai/api/v1/messages.
	openRouterEndpoint   = "https://" + OpenRouterHost + openRouterPathPrefix
	openRouterPathPrefix = "/api"
	openRouterAgent      = "claude"
	// OpenRouterHost is the upstream host the dev route mints; the gateway
	// model proxy admits it alongside api.anthropic.com.
	OpenRouterHost = "openrouter.ai"
)

// devModelRoute is the active dev route. Only the model varies: the agent,
// endpoint, host and path prefix are the openRouter* constants.
type devModelRoute struct {
	model string
}

var (
	activeDevModelRoute atomic.Pointer[devModelRoute]
	devModelIDPattern   = regexp.MustCompile(`^[a-z0-9~][a-z0-9._/:~-]{0,127}$`)
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
func resolveDevModelRoute(route, model, controlPlaneIdentity string) (*devModelRoute, error) {
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
	if model == "" {
		model = DefaultDevModel
	}
	if !devModelIDPattern.MatchString(model) {
		return nil, fmt.Errorf("BEX_AGENT_DEV_MODEL %q is not a model id", model)
	}
	return &devModelRoute{model: model}, nil
}

// registeredEndpoint is the endpoint a profile resolves to in this process:
// the release manifest's, unless the dev route overrides that profile.
func registeredEndpoint(profile AgentProfile) string {
	if activeDevModelRoute.Load() != nil && profile.ID == openRouterAgent {
		return openRouterEndpoint
	}
	return profile.ModelEndpoint
}

// DevModelRouteEnv is the extra sandbox environment the dev route needs for
// agent: the model id under every name Claude Code reads. Nil when the route
// is off or does not apply to agent.
func DevModelRouteEnv(agent string) map[string]string {
	route := activeDevModelRoute.Load()
	if route == nil || strings.ToLower(strings.TrimSpace(agent)) != openRouterAgent {
		return nil
	}
	return map[string]string{
		"ANTHROPIC_MODEL":                route.model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   route.model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": route.model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  route.model,
	}
}

// upstreamPathPrefix is the path the gateway prepends to the agent's request
// path for host ("" for every release endpoint, whose agents already send the
// full vendor path).
func upstreamPathPrefix(host string) string {
	if activeDevModelRoute.Load() != nil && host == OpenRouterHost {
		return openRouterPathPrefix
	}
	return ""
}
