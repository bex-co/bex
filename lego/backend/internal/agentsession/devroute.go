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

	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
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
//     and covers every workspace except one that set its own model key; it
//     injects a platform-owned key and pins the model: the gateway refuses any
//     request body whose top-level "model" differs.
//
// A workspace's own model key (BYO, ADR091 D7) wins over the platform route:
// a workspace whose admin set one is never on the platform route, so it is
// neither metered, billed nor spend-capped for tokens. Otherwise the platform
// route wins when it is active; otherwise the dev route covers every workspace
// on a dev control plane; with neither, behaviour is the release
// profile's. Every route decision takes byoKey, the workspace's recorded
// model_key_set_at flag (store.AgentEnvironment.ModelKeySet).

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
	openRouterAgent      = ClaudeAgent
	// ClaudeAgent is the claude agent profile: the one the model routes serve
	// and the dashboard composer's default, so a session bex starts on a
	// user's behalf (a `/bex` takeover) runs it too.
	ClaudeAgent = "claude"
	// OpenRouterHost is the upstream host a route mints; the gateway model
	// proxy admits it alongside api.anthropic.com.
	OpenRouterHost = "openrouter.ai"
	// MetaModelHost is Meta's official model API, the muse profile's only
	// upstream (ADR108 D6).
	MetaModelHost = "api.meta.ai"
)

// modelRoute is one active route. The agent, endpoint, host and path prefix
// are the openRouter* constants; only the model and key vary. An active route
// covers every workspace.
type modelRoute struct {
	model string
	// platformKey is the platform-owned credential the mint injects instead of
	// the workspace BYO key. Empty for the dev route. Never logged or returned
	// outside ModelMintResponse.Credential.
	platformKey string
}

func (r *modelRoute) active() bool {
	return r != nil
}

var (
	activeDevModelRoute      atomic.Pointer[modelRoute]
	activePlatformModelRoute atomic.Pointer[modelRoute]
	// activeMetaRoute serves the platform-only profiles on Meta's official
	// API (ADR108 D5, muse) with a bex-owned key in every workspace. Its
	// model is each profile's own.
	activeMetaRoute atomic.Pointer[modelRoute]
	modelIDPattern  = regexp.MustCompile(`^[a-z0-9~][a-z0-9._/:~-]{0,127}$`)
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
// platform model route for every workspace. key must be non-empty when route
// is set. Errors never include the key.
func EnablePlatformModelRoute(route, model, key string) error {
	resolved, err := resolvePlatformModelRoute(route, model)
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

// EnablePlatformMetaRoute installs the Meta route for the profiles on Meta's
// API in every workspace and reports whether it is active: an empty key
// clears it. The key is never logged.
func EnablePlatformMetaRoute(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		activeMetaRoute.Store(nil)
		return false
	}
	activeMetaRoute.Store(&modelRoute{platformKey: key})
	return true
}

// platformOnlyHost is the model host of agent when it is a platform-only
// profile (ADR108 D5), else "".
func platformOnlyHost(agent string) string {
	profile, ok := LookupAgentProfile(agent)
	if !ok || !profile.PlatformOnly {
		return ""
	}
	host, err := sessionegress.ModelEndpointHost(profile.ModelEndpoint)
	if err != nil {
		return ""
	}
	return host
}

// OnMetaRoute reports whether agent is a platform-only profile whose model
// is Meta's API: one the Meta route's single key serves.
func OnMetaRoute(agent string) bool {
	return platformOnlyHost(agent) == MetaModelHost
}

// platformKey is the bex-owned key that serves the platform-only profiles on
// host, or "" when none does: the Meta route's on Meta's API (muse), the
// OpenRouter platform route's on OpenRouter (ADR114, deepseek). Routes cover
// every workspace (w10/m14 t007). Unlike the claude profile on the OpenRouter
// route, a platform-only profile never yields to a workspace's own key, and
// it runs its own model (AgentProfile.Model), not the route's.
func platformKey(host string) string {
	var route *modelRoute
	switch host {
	case MetaModelHost:
		route = activeMetaRoute.Load()
	case OpenRouterHost:
		route = activePlatformModelRoute.Load()
	}
	if route.active() {
		return route.platformKey
	}
	return ""
}

// ValidatePlatformModelRoute checks the route and model without the key and
// without installing anything (bex-api's startup config check; a missing key
// is a degraded-but-running state the loader handles itself).
func ValidatePlatformModelRoute(route, model string) error {
	_, err := resolvePlatformModelRoute(route, model)
	return err
}

func resolvePlatformModelRoute(route, model string) (*modelRoute, error) {
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
	return &modelRoute{model: model}, nil
}

func routeModel(env, model string) (string, error) {
	if model == "" {
		model = DefaultDevModel
	}
	if !modelIDPattern.MatchString(model) {
		return "", fmt.Errorf("%s %q is not a model id", env, model)
	}
	return model, nil
}

// routeFor is the route that applies to agent, or nil. byoKey
// (the workspace set its own model key) takes the workspace off the platform
// route.
func routeFor(agent string, byoKey bool) *modelRoute {
	if !PinnedAgent(agent) {
		return nil
	}
	if route := activePlatformModelRoute.Load(); !byoKey && route.active() {
		return route
	}
	if route := activeDevModelRoute.Load(); route.active() {
		return route
	}
	return nil
}

// platformRouteCovers reports whether the platform route serves the agent
// profile, BEFORE the BYO precedence: the only case in
// which the mint must look up whether the workspace set its own key.
func platformRouteCovers(agent string) bool {
	return PinnedAgent(agent) && activePlatformModelRoute.Load().active()
}

// PlatformRouteServes reports whether a bex-owned route supplies agent's
// model credential when the workspace has no key of its own: the OpenRouter
// route for the claude profile, and for a platform-only one the route of its
// provider (platformKey). Codex and Gemini need the workspace's key
// (ModelKeySecretPath) to run at all.
func PlatformRouteServes(agent string) bool {
	return PlatformRouteMeters(agent, false)
}

// PlatformRouteMeters reports whether agent's sessions run on
// a bex-owned key, and so are priced, spend-capped and counted toward the
// trust tier: the claude profile on the OpenRouter route unless the workspace
// set its own key (byoKey), and a platform-only profile wherever its route
// serves it.
func PlatformRouteMeters(agent string, byoKey bool) bool {
	return platformKey(platformOnlyHost(agent)) != "" || (!byoKey && platformRouteCovers(agent))
}

// ProfileServed reports whether a workspace can run agent at all. A
// platform-only profile (ADR108 D5) runs only where a bex-owned route serves
// it; every other profile can also run on the workspace's own key.
func ProfileServed(agent string) bool {
	profile, ok := LookupAgentProfile(agent)
	return ok && (!profile.PlatformOnly || PlatformRouteServes(profile.ID))
}

// PlatformModelKeyReady reports whether a workspace's model credential is the
// platform route's key, so readiness need not (and must not) consult the
// workspace's BYO key: for the claude profile, which the platform route
// serves only for a workspace without its own key (byoKey). A platform-only
// profile's readiness is ProfileServed's.
func PlatformModelKeyReady(byoKey bool) bool {
	return !byoKey && activePlatformModelRoute.Load().active()
}

// PlatformPinnedModel is the model the platform route pins for a workspace's
// claude sessions — the gateway refuses any other request model with a 403 —
// and whether the workspace is on that route at all (never with its own key,
// byoKey). Session create refuses a different explicit model up front
// (AGENT_SESSION_MODEL_PINNED) and the workspace agent environment reports it
// as read-only (w10/m17).
func PlatformPinnedModel(byoKey bool) (string, bool) {
	route := activePlatformModelRoute.Load()
	if byoKey || !route.active() {
		return "", false
	}
	return route.model, true
}

// IsPlatformModelEndpoint reports whether a session's stored model endpoint
// is the platform model route's (bex-billed) rather than a provider the
// workspace's own key pays for (ADR091 D6).
func IsPlatformModelEndpoint(endpoint string) bool {
	return strings.TrimRight(strings.TrimSpace(endpoint), "/") == openRouterEndpoint
}

// PinnedAgent reports whether agent is the profile a model route serves, so
// a pinned workspace's other profiles keep their own model choice.
func PinnedAgent(agent string) bool {
	return strings.ToLower(strings.TrimSpace(agent)) == openRouterAgent
}

// registeredEndpoint is the endpoint a profile resolves to: the release
// manifest's, unless a route covers the profile.
func registeredEndpoint(profile AgentProfile, byoKey bool) string {
	if routeFor(profile.ID, byoKey) != nil {
		return openRouterEndpoint
	}
	return profile.ModelEndpoint
}

// ModelRouteEnv is the extra sandbox environment a route needs for agent: the
// model id under every name Claude Code reads. Nil when no
// route applies (byoKey: see routeFor).
func ModelRouteEnv(agent string, byoKey bool) map[string]string {
	route := routeFor(agent, byoKey)
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
