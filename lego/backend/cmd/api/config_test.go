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

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
)

func loadFor(t *testing.T, env map[string]string) (*Config, []string, error) {
	t.Helper()
	return loadConfig(func(k string) string { return env[k] }, time.Now(), []string{"api"})
}

// cpEnv is a minimal healthy control-plane environment: the store URI plus the
// two values whose absence is itself fatal (CP token, OpenFGA URL).
func cpEnv(extra map[string]string) map[string]string {
	env := map[string]string{
		"BEX_CP_DB_URI":   "postgres://cp",
		"BEX_CP_TOKEN":    "token",
		"BEX_OPENFGA_URL": "http://openfga:8080",
	}
	for k, v := range extra {
		env[k] = v
	}
	return env
}

func hasWarning(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, warnings, err := loadFor(t, nil)
	if err != nil {
		t.Fatalf("empty env must load cleanly: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("empty env produced warnings: %v", warnings)
	}
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"APIAddr", cfg.APIAddr, ":8090"},
		{"Namespace", cfg.Namespace, "default"},
		{"CPAddr (CP off)", cfg.CPAddr, ""},
		{"MCPStdio", cfg.MCPStdio, false},
		{"RequireVerifiedInviteEmail", cfg.RequireVerifiedInviteEmail, true},
		{"MaxBodyBytes", cfg.MaxBodyBytes, int64(2097152)},
		{"MaxQueryHours", cfg.MaxQueryHours, 720},
		{"MaxSSEConns", cfg.MaxSSEConns, int64(100)},
		{"MaxSSEConnsPerSubject", cfg.MaxSSEConnsPerSubject, 5},
		{"MaxSSEConnsPerWorkspace", cfg.MaxSSEConnsPerWorkspace, 20},
		{"RateLimitRPM", cfg.RateLimitRPM, 500.0},
		{"DeviceRateRPM", cfg.DeviceRateRPM, 30.0},
		{"WebhookRateRPM", cfg.WebhookRateRPM, 600.0},
		{"DeployHookLookupRPM", cfg.DeployHookLookupRPM, 60.0},
		{"DeployHookLookupBurst", cfg.DeployHookLookupBurst, 10},
		{"AuthFailureRPM", cfg.AuthFailureRPM, 60.0},
		{"AuthMaxInflight", cfg.AuthMaxInflight, 64},
		{"AuthMaxInflightPerCredential", cfg.AuthMaxInflightPerCredential, 64},
		{"AgentSandboxIdleTTL", cfg.AgentSandboxIdleTTL, 30 * time.Minute},
		{"AgentTurnTimeout", cfg.AgentTurnTimeout, 30 * time.Minute},
		{"AgentSnapshotRetentionTTL", cfg.AgentSnapshotRetentionTTL, 7 * 24 * time.Hour},
		{"AgentMaxLiveSandboxesPerWorkspace", cfg.AgentMaxLiveSandboxesPerWorkspace, 5},
		{"AgentMaxPinnedSandboxesPerWorkspace", cfg.AgentMaxPinnedSandboxesPerWorkspace, 10},
		{"MaxBlueprintGroupings", cfg.MaxBlueprintGroupings, 1000},
		{"ImageCompatibilityWorkspaces (every workspace)", cfg.ImageCompatibilityWorkspaces["*"], true},
		{"PrivateImagePortsDisabled (default on)", cfg.PrivateImagePortsDisabled, false},
		{"MaxEnvGroupsPerWorkspace", cfg.MaxEnvGroupsPerWorkspace, 100},
		{"MaxGitConnectionsPerWorkspace", cfg.MaxGitConnectionsPerWorkspace, 10},
		{"MaxRegistryCredentialsPerWorkspace", cfg.MaxRegistryCredentialsPerWorkspace, 50},
		{"MaxCustomDomainsPerService", cfg.MaxCustomDomainsPerService, 100},
		{"MaxCustomDomainsPerWorkspace", cfg.MaxCustomDomainsPerWorkspace, 500},
		{"SandboxImage (sandbox off)", cfg.SandboxImage, ""},
		{"StripeEnabled", cfg.StripeEnabled, false},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestLoadConfigFatalKnobs pins the "fatal" policy: each previously
// log.Fatalf'd parse is now a collected error, raised before any side effect.
func TestLoadConfigFatalKnobs(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"revalidate interval", map[string]string{"BEX_LOG_STREAM_REVALIDATE_INTERVAL": "bogus"}, "bad BEX_LOG_STREAM_REVALIDATE_INTERVAL"},
		{"trusted proxies", map[string]string{"BEX_TRUSTED_PROXY_CIDRS": "not-a-cidr"}, "bad BEX_TRUSTED_PROXY_CIDRS"},
		{"rate limit", map[string]string{"BEX_RATE_LIMIT": "abc"}, `bad BEX_RATE_LIMIT "abc"`},
		{"device rate limit", map[string]string{"BEX_DEVICE_RATE_LIMIT": "abc"}, "bad BEX_DEVICE_RATE_LIMIT"},
		{"auth max inflight", map[string]string{"BEX_AUTH_MAX_INFLIGHT": "many"}, "bad BEX_AUTH_MAX_INFLIGHT"},
		{"auth max inflight per credential", map[string]string{"BEX_AUTH_MAX_INFLIGHT_PER_CREDENTIAL": "many"}, "bad BEX_AUTH_MAX_INFLIGHT_PER_CREDENTIAL"},
		{"webhook secret without store", map[string]string{"BEX_WEBHOOK_SECRET": "s"}, "Git webhooks require BEX_CP_DB_URI"},
		{"github webhook secret without store", map[string]string{"BEX_GITHUB_WEBHOOK_SECRET": "s"}, "Git webhooks require BEX_CP_DB_URI"},
		{"partial snapshot config", map[string]string{"BEX_AGENT_SNAPSHOT_S3_BUCKET": "b"}, "agent-session hibernation config"},
		{"payment method mode", map[string]string{"BEX_REQUIRE_PAYMENT_METHOD": "maybe"}, "BEX_REQUIRE_PAYMENT_METHOD must be 1"},
		{"cp resync", cpEnv(map[string]string{"BEX_CP_RESYNC": "soon"}), "bad BEX_CP_RESYNC"},
		{"cp identity", cpEnv(map[string]string{"BEX_CP_IDENTITY": "not a label!"}), "not a valid Kubernetes label value"},
		{"webhook backoff", cpEnv(map[string]string{"BEX_WEBHOOK_BACKOFF": "bogus"}), ""},
		{"stripe epoch", cpEnv(map[string]string{"BEX_STRIPE_SECRET_KEY": "sk_test_x", "BEX_STRIPE_EPOCH": "yesterday"}), "bad BEX_STRIPE_EPOCH"},
		{"stripe grace floor", cpEnv(map[string]string{
			"BEX_STRIPE_SECRET_KEY":      "sk_test_x",
			"BEX_STRIPE_DUNNING_ENABLED": "1",
			"BEX_STRIPE_GRACE_PERIOD":    "10s",
		}), "BEX_STRIPE_GRACE_PERIOD must be a duration >= 1m"},
		{"model proxy url", map[string]string{
			"BEX_OPENSANDBOX_URL":       "http://sandbox",
			"BEX_AGENT_MODEL_PROXY_URL": "not a url at all\x7f",
		}, "bad BEX_AGENT_MODEL_PROXY_URL"},
		// ADR090: the dev-only model route never starts on a production control
		// plane (BEX_CP_IDENTITY unset or "production"), with or without a store.
		{"dev model route without control plane", map[string]string{"BEX_AGENT_DEV_MODEL_ROUTE": "openrouter"}, "dev-only"},
		{"dev model route on production", cpEnv(map[string]string{"BEX_AGENT_DEV_MODEL_ROUTE": "openrouter"}), "dev-only"},
		{"dev model route on explicit production", cpEnv(map[string]string{"BEX_AGENT_DEV_MODEL_ROUTE": "openrouter", "BEX_CP_IDENTITY": "production"}), "dev-only"},
		{"unknown dev model route", cpEnv(map[string]string{"BEX_AGENT_DEV_MODEL_ROUTE": "anthropic", "BEX_CP_IDENTITY": "dev-10"}), "not supported"},
		{"model proxy port", map[string]string{
			"BEX_OPENSANDBOX_URL":       "http://sandbox",
			"BEX_AGENT_MODEL_PROXY_URL": "http://gw:0",
		}, `bad BEX_AGENT_MODEL_PROXY_URL port "0"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := loadFor(t, tc.env)
			if err == nil {
				t.Fatalf("wanted an error for %v", tc.env)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestLoadConfigCollectsEveryProblem pins the crashloop fix's ergonomics: one
// failed start reports the complete fix list, not just the first bad knob.
func TestLoadConfigCollectsEveryProblem(t *testing.T) {
	_, _, err := loadFor(t, map[string]string{
		"BEX_RATE_LIMIT":        "abc",
		"BEX_AUTH_MAX_INFLIGHT": "many",
	})
	if err == nil {
		t.Fatal("wanted an error")
	}
	for _, want := range []string{"BEX_RATE_LIMIT", "BEX_AUTH_MAX_INFLIGHT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("collected error %v is missing %s", err, want)
		}
	}
}

func TestLoadConfigAuthzFailClosed(t *testing.T) {
	// Store on, OpenFGA off: refuse to start (w1/m53 + w1/m65 F16).
	env := cpEnv(nil)
	delete(env, "BEX_OPENFGA_URL")
	if _, _, err := loadFor(t, env); err == nil ||
		!strings.Contains(err.Error(), "Refusing to start a multi-tenant API") {
		t.Fatalf("store-on/FGA-off must fail closed, got %v", err)
	}

	// The documented local-dev override downgrades it to a loud warning.
	env["BEX_ALLOW_INSECURE_AUTHZ"] = "1"
	_, warnings, err := loadFor(t, env)
	if err != nil {
		t.Fatalf("insecure-authz override still errored: %v", err)
	}
	if !hasWarning(warnings, "FAIL-OPEN") {
		t.Fatalf("override must warn loudly, warnings = %v", warnings)
	}

	// Healthy: FGA wired, no error, no fail-open warning.
	if _, warnings, err := loadFor(t, cpEnv(nil)); err != nil || hasWarning(warnings, "FAIL-OPEN") {
		t.Fatalf("healthy CP env: err=%v warnings=%v", err, warnings)
	}

	// stdio mode never runs the control plane, so the posture does not apply.
	env = map[string]string{"BEX_CP_DB_URI": "postgres://cp", "BEX_MCP_STDIO": "1"}
	if _, _, err := loadFor(t, env); err != nil {
		t.Fatalf("stdio mode must not enforce CP posture: %v", err)
	}
}

func TestLoadConfigCPTokenFailClosed(t *testing.T) {
	env := cpEnv(nil)
	delete(env, "BEX_CP_TOKEN")
	if _, _, err := loadFor(t, env); err == nil ||
		!strings.Contains(err.Error(), "BEX_CP_TOKEN is required") {
		t.Fatalf("empty CP token must fail closed, got %v", err)
	}
	env["BEX_CP_INSECURE"] = "1"
	if _, _, err := loadFor(t, env); err != nil {
		t.Fatalf("BEX_CP_INSECURE=1 override still errored: %v", err)
	}
}

// TestLoadConfigWarnKnobs pins the "warn" policy: a set-but-invalid tuning
// knob keeps its default and surfaces one startup warning.
func TestLoadConfigWarnKnobs(t *testing.T) {
	cfg, warnings, err := loadFor(t, map[string]string{
		"BEX_AGENT_SANDBOX_IDLE_TTL":       "-5m",
		"BEX_MAX_ENV_GROUPS_PER_WORKSPACE": "-1",
	})
	if err != nil {
		t.Fatalf("warn-policy knobs must not be fatal: %v", err)
	}
	if cfg.AgentSandboxIdleTTL != 30*time.Minute {
		t.Errorf("AgentSandboxIdleTTL = %v, want the 30m default kept", cfg.AgentSandboxIdleTTL)
	}
	if cfg.MaxEnvGroupsPerWorkspace != 100 {
		t.Errorf("MaxEnvGroupsPerWorkspace = %d, want the 100 default kept", cfg.MaxEnvGroupsPerWorkspace)
	}
	if !hasWarning(warnings, "BEX_AGENT_SANDBOX_IDLE_TTL") || !hasWarning(warnings, "BEX_MAX_ENV_GROUPS_PER_WORKSPACE") {
		t.Errorf("warnings missing knob names: %v", warnings)
	}

	// The usage-retention warn knob only parses with the control plane on.
	cfg, warnings, err = loadFor(t, cpEnv(map[string]string{"BEX_USAGE_RETENTION_MONTHS": "0"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.UsageRetentionSet {
		t.Error("invalid retention must not set UsageRetentionSet")
	}
	if !hasWarning(warnings, "BEX_USAGE_RETENTION_MONTHS") {
		t.Errorf("warnings = %v", warnings)
	}
}

// TestLoadConfigQuietKnobs pins the "quiet" policy: malformed becomes the
// documented sentinel (0) with no error and no warning.
func TestLoadConfigQuietKnobs(t *testing.T) {
	cfg, warnings, err := loadFor(t, map[string]string{"BEX_MAX_BODY_BYTES": "huge"})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("quiet knob must stay quiet: err=%v warnings=%v", err, warnings)
	}
	if cfg.MaxBodyBytes != 0 {
		t.Errorf("MaxBodyBytes = %d, want the 0 sentinel", cfg.MaxBodyBytes)
	}
}

func TestLoadConfigOAuthAudienceWarning(t *testing.T) {
	_, warnings, err := loadFor(t, map[string]string{"BEX_OAUTH_RESOURCE": "https://api.bex.co"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasWarning(warnings, "BEX_OAUTH_REQUIRE_AUDIENCE is off") {
		t.Fatalf("codex-F6 warning missing: %v", warnings)
	}
	_, warnings, err = loadFor(t, map[string]string{
		"BEX_OAUTH_RESOURCE":         "https://api.bex.co",
		"BEX_OAUTH_REQUIRE_AUDIENCE": "1",
	})
	if err != nil || hasWarning(warnings, "BEX_OAUTH_REQUIRE_AUDIENCE is off") {
		t.Fatalf("audience on must not warn: err=%v warnings=%v", err, warnings)
	}
}

func TestLoadConfigPlatformClientRegistry(t *testing.T) {
	cfg, _, err := loadFor(t, map[string]string{
		"BEX_OAUTH_PLATFORM_CLIENTS": " render-cli, bex-mobile, ,render-cli ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(cfg.OAuthPlatformClients, ","); got != "render-cli,bex-mobile,render-cli" {
		t.Errorf("OAuthPlatformClients = %q, want trimmed non-empty IDs", got)
	}
}

// TestLoadConfigStdioSkipsServingKnobs pins the gating: a local agent's
// leftover env must never fail the stdio subprocess, exactly as the inline
// reads it replaced were skipped after the stdio return.
func TestLoadConfigStdioSkipsServingKnobs(t *testing.T) {
	cfg, _, err := loadFor(t, map[string]string{
		"BEX_MCP_STDIO":  "1",
		"BEX_RATE_LIMIT": "abc", // fatal outside stdio mode
	})
	if err != nil {
		t.Fatalf("stdio mode must skip serving knobs: %v", err)
	}
	if !cfg.MCPStdio {
		t.Fatal("MCPStdio not detected")
	}
}

func TestLoadConfigMCPStdioFromArgs(t *testing.T) {
	cfg, _, err := loadConfig(func(string) string { return "" }, time.Now(), []string{"api", "mcp-stdio"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.MCPStdio {
		t.Fatal("mcp-stdio subcommand not detected")
	}
}

// TestLoadConfigStripeStaysInert pins the gate ordering: with no runtime key,
// stale malformed BEX_STRIPE_* values must not fail startup.
func TestLoadConfigStripeStaysInert(t *testing.T) {
	cfg, _, err := loadFor(t, cpEnv(map[string]string{
		"BEX_STRIPE_EPOCH":           "garbage",
		"BEX_STRIPE_GRACE_PERIOD":    "1s",
		"BEX_STRIPE_DUNNING_ENABLED": "1",
	}))
	if err != nil {
		t.Fatalf("keyless Stripe env must stay inert: %v", err)
	}
	if cfg.StripeEnabled {
		t.Fatal("StripeEnabled without a secret key")
	}
}

func TestLoadConfigVerifiedInviteEmail(t *testing.T) {
	for raw, want := range map[string]bool{"": true, "1": true, "yes": true, "0": false, "false": false, "FALSE": false} {
		cfg, _, err := loadFor(t, map[string]string{"BEX_REQUIRE_VERIFIED_INVITE_EMAIL": raw})
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if cfg.RequireVerifiedInviteEmail != want {
			t.Errorf("RequireVerifiedInviteEmail(%q) = %v, want %v", raw, cfg.RequireVerifiedInviteEmail, want)
		}
	}
}

// TestLoadConfigVerifiedEmail: the w2/m168 gate is secure by default; only an
// explicit 0/false turns it off.
func TestLoadConfigVerifiedEmail(t *testing.T) {
	for raw, want := range map[string]bool{"": true, "1": true, "0": false, "false": false, "FALSE": false} {
		cfg, _, err := loadFor(t, map[string]string{"BEX_REQUIRE_VERIFIED_EMAIL": raw})
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if cfg.RequireVerifiedEmail != want {
			t.Errorf("RequireVerifiedEmail(%q) = %v, want %v", raw, cfg.RequireVerifiedEmail, want)
		}
	}
}

// TestLoadConfigOpsWorkspacePin covers the ADR088 §4 pin: both vars set arms
// the verb (and, without a control plane, still parses the internal listener
// address); exactly one set warns loudly and stays disabled; stdio mode never
// serves. Both-unset defaults ride TestLoadConfigDefaults' zero-warning gate.
func TestLoadConfigOpsWorkspacePin(t *testing.T) {
	cfg, warnings, err := loadFor(t, map[string]string{
		"BEX_OPS_WORKSPACE":  "tea-ops",
		"BEX_OPS_ROLE_TOKEN": "s3cret",
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("both set: err=%v warnings=%v", err, warnings)
	}
	if cfg.OpsWorkspace != "tea-ops" || cfg.OpsRoleToken != "s3cret" {
		t.Fatalf("ops pin = %q/%q", cfg.OpsWorkspace, cfg.OpsRoleToken)
	}
	if cfg.CPAddr != ":8091" {
		t.Fatalf("CPAddr = %q, want :8091 (the ops-only internal listener must know its address)", cfg.CPAddr)
	}

	for _, env := range []map[string]string{
		{"BEX_OPS_WORKSPACE": "tea-ops"},
		{"BEX_OPS_ROLE_TOKEN": "s3cret"},
	} {
		cfg, warnings, err := loadFor(t, env)
		if err != nil {
			t.Fatalf("partial pin must not be fatal: %v", err)
		}
		if !hasWarning(warnings, "BEX_OPS_WORKSPACE/BEX_OPS_ROLE_TOKEN") {
			t.Fatalf("partial pin: want the loud disabled warning, got %v", warnings)
		}
		if cfg.CPAddr != "" {
			t.Fatalf("partial pin must not arm the internal listener: CPAddr=%q", cfg.CPAddr)
		}
	}

	cfg, _, err = loadFor(t, map[string]string{
		"BEX_OPS_WORKSPACE":  "tea-ops",
		"BEX_OPS_ROLE_TOKEN": "s3cret",
		"BEX_MCP_STDIO":      "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CPAddr != "" {
		t.Fatalf("stdio mode parsed the internal listener addr: %q", cfg.CPAddr)
	}
}

// BEX_PRIVATE_IMAGE_PORTS is the w6/m148 rollback switch: on by default, and a
// typo refuses startup instead of silently flipping the creation default.
func TestLoadConfigPrivateImagePorts(t *testing.T) {
	for value, disabled := range map[string]bool{"": false, "on": false, " off ": true, "off": true} {
		cfg, _, err := loadFor(t, map[string]string{"BEX_PRIVATE_IMAGE_PORTS": value})
		if err != nil || cfg.PrivateImagePortsDisabled != disabled {
			t.Errorf("%q: disabled=%t err=%v, want disabled=%t", value, cfg.PrivateImagePortsDisabled, err, disabled)
		}
	}
	for _, bad := range []string{"1", "true", "false", "OFF", "disabled"} {
		if _, _, err := loadFor(t, map[string]string{"BEX_PRIVATE_IMAGE_PORTS": bad}); err == nil ||
			!strings.Contains(err.Error(), "BEX_PRIVATE_IMAGE_PORTS") {
			t.Errorf("%q: err = %v, want a startup refusal", bad, err)
		}
	}
}

func TestLoadConfigImageCompatibilityWorkspaces(t *testing.T) {
	cfg, _, err := loadFor(t, map[string]string{
		"BEX_IMAGE_COMPATIBILITY_WORKSPACES": " tea-d98210cbbpdc73dcrkvg , ,tea-daif693dqjvc73e7as3g",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ImageCompatibilityWorkspaces) != 2 || !cfg.ImageCompatibilityWorkspaces["tea-d98210cbbpdc73dcrkvg"] ||
		!cfg.ImageCompatibilityWorkspaces["tea-daif693dqjvc73e7as3g"] {
		t.Fatalf("workspaces = %v", cfg.ImageCompatibilityWorkspaces)
	}
	if cfg, _, err = loadFor(t, map[string]string{"BEX_IMAGE_COMPATIBILITY_WORKSPACES": "*"}); err != nil || !cfg.ImageCompatibilityWorkspaces["*"] {
		t.Fatalf("wildcard: %v %v", cfg.ImageCompatibilityWorkspaces, err)
	}
	// "off" is the only spelling of the strict policy; it is not a list entry.
	if cfg, _, err = loadFor(t, map[string]string{"BEX_IMAGE_COMPATIBILITY_WORKSPACES": " off "}); err != nil || cfg.ImageCompatibilityWorkspaces != nil {
		t.Fatalf("off: %v %v", cfg.ImageCompatibilityWorkspaces, err)
	}
	// The retired boolean spelling and non-workspace ids must refuse startup
	// rather than silently admitting no workspace (or the wrong one).
	for _, bad := range []string{"1", "srv-d98210cbbpdc73dcrkvg", "tea-ops", "off,tea-d98210cbbpdc73dcrkvg"} {
		if _, _, err := loadFor(t, map[string]string{"BEX_IMAGE_COMPATIBILITY_WORKSPACES": bad}); err == nil ||
			!strings.Contains(err.Error(), "BEX_IMAGE_COMPATIBILITY_WORKSPACES") {
			t.Errorf("%q: err = %v, want a startup refusal", bad, err)
		}
	}
}

func TestModelProxyPort(t *testing.T) {
	cases := []struct {
		raw     string
		want    uint16
		wantErr bool
	}{
		{"", 0, false},
		{"http://gateway", 8084, false},
		{"http://gateway:9000", 9000, false},
		{"http://gateway:0", 0, true},
		{"http://gateway:notaport", 0, true},
	}
	for _, tc := range cases {
		got, err := modelProxyPort(tc.raw)
		if (err != nil) != tc.wantErr {
			t.Errorf("modelProxyPort(%q) err = %v, wantErr %v", tc.raw, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("modelProxyPort(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
}

func TestRouterConfiguration(t *testing.T) {
	for _, endpoint := range []string{"https://router.example/graphql", "http://router.internal/graphql"} {
		cfg, _, err := loadFor(t, map[string]string{"BEX_ROUTER_URL": endpoint, "BEX_ROUTER_ASSERTION_SECRET": strings.Repeat("s", 32)})
		if err != nil || cfg.RouterURL != endpoint {
			t.Fatalf("valid Router configuration rejected: %v", err)
		}
	}
	for _, env := range []map[string]string{
		{"BEX_ROUTER_URL": "https://router.example/graphql"},
		{"BEX_ROUTER_URL": "file:///private/router", "BEX_ROUTER_ASSERTION_SECRET": strings.Repeat("s", 32)},
		{"BEX_ROUTER_URL": "https://user:password@router.example/graphql", "BEX_ROUTER_ASSERTION_SECRET": strings.Repeat("s", 32)},
	} {
		if _, _, err := loadFor(t, env); err == nil {
			t.Fatal("invalid Router configuration accepted")
		}
	}
}

// BEX_RESERVED_DOMAINS (w5/m121) lists zones, normalized like a custom
// domain; a malformed entry refuses to start rather than reserve nothing.
func TestLoadConfigReservedDomains(t *testing.T) {
	cfg, _, err := loadFor(t, map[string]string{"BEX_RESERVED_DOMAINS": " Bex.Acme.com. , ,ops.acme.com"})
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if got := strings.Join(cfg.ReservedDomains, ","); got != "bex.acme.com,ops.acme.com" {
		t.Fatalf("ReservedDomains = %q, want bex.acme.com,ops.acme.com", got)
	}
	for _, bad := range []string{"bad_zone", "https://bex.acme.com", "localhost"} {
		if _, _, err := loadFor(t, map[string]string{"BEX_RESERVED_DOMAINS": bad}); err == nil || !strings.Contains(err.Error(), "BEX_RESERVED_DOMAINS") {
			t.Errorf("BEX_RESERVED_DOMAINS=%q = %v, want a startup error naming it", bad, err)
		}
	}
}

// TestLoadConfigDevModelRouteOnDevControlPlane: a dev-N identity admits the
// route, defaulting the model to the pinned DeepSeek flash id.
func TestLoadConfigDevModelRouteOnDevControlPlane(t *testing.T) {
	cfg, _, err := loadFor(t, cpEnv(map[string]string{"BEX_AGENT_DEV_MODEL_ROUTE": "openrouter", "BEX_CP_IDENTITY": "dev-10"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentDevModelRoute != "openrouter" || cfg.AgentDevModel != "" {
		t.Fatalf("dev route config = %q / %q", cfg.AgentDevModelRoute, cfg.AgentDevModel)
	}
}

// TestLoadConfigPlatformModelRoute pins the workspace-allowlisted platform
// model route's startup contract (ADR090): route + allowlist + key enable it
// on a production control plane; a malformed route, model, empty allowlist or
// non-workspace id refuses startup; a missing key only disables the route with
// a loud warning (the Secret is optional, so bex-api must not crash-loop), and
// the key's value never appears in any message.
func TestLoadConfigPlatformModelRoute(t *testing.T) {
	const key = "sk-or-v1-PLATFORM-SECRET"
	base := func(extra map[string]string) map[string]string {
		env := map[string]string{
			"BEX_AGENT_PLATFORM_MODEL_ROUTE":      "openrouter",
			"BEX_AGENT_PLATFORM_MODEL_WORKSPACES": " tea-d98210cbbpdc73dcrkvg ,",
			"BEX_AGENT_PLATFORM_MODEL_KEY":        key,
		}
		for k, v := range extra {
			env[k] = v
		}
		return cpEnv(env)
	}
	cfg, warnings, err := loadFor(t, base(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentPlatformModelRoute != "openrouter" || cfg.AgentPlatformModelKey != key ||
		strings.Join(cfg.AgentPlatformModelWorkspaces, ",") != "tea-d98210cbbpdc73dcrkvg" {
		t.Fatalf("route = %q workspaces = %v keySet=%v", cfg.AgentPlatformModelRoute, cfg.AgentPlatformModelWorkspaces, cfg.AgentPlatformModelKey != "")
	}
	if strings.Contains(strings.Join(warnings, "\n"), key) {
		t.Fatal("a warning carries the platform key")
	}

	for name, extra := range map[string]map[string]string{
		"unknown route":   {"BEX_AGENT_PLATFORM_MODEL_ROUTE": "bedrock"},
		"empty allowlist": {"BEX_AGENT_PLATFORM_MODEL_WORKSPACES": " , "},
		"non-workspace":   {"BEX_AGENT_PLATFORM_MODEL_WORKSPACES": "tea-d98210cbbpdc73dcrkvg,srv-d98210cbbpdc73dcrkvg"},
		"malformed id":    {"BEX_AGENT_PLATFORM_MODEL_WORKSPACES": "tea_d98210cbbpdc73dcrkvg"},
		"bad model":       {"BEX_AGENT_PLATFORM_MODEL": "claude opus; rm"},
	} {
		_, _, err := loadFor(t, base(extra))
		if err == nil || !strings.Contains(err.Error(), "BEX_AGENT_PLATFORM_MODEL") {
			t.Errorf("%s: err = %v, want a startup refusal naming the platform route", name, err)
		} else if strings.Contains(err.Error(), key) {
			t.Errorf("%s: the startup error carries the platform key", name)
		}
	}

	cfg, warnings, err = loadFor(t, base(map[string]string{"BEX_AGENT_PLATFORM_MODEL_KEY": ""}))
	if err != nil {
		t.Fatalf("missing key refused startup: %v", err)
	}
	if cfg.AgentPlatformModelRoute != "" {
		t.Fatalf("route = %q with no key, want disabled", cfg.AgentPlatformModelRoute)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "platform model route is DISABLED") {
		t.Fatalf("warnings = %q, want a loud disabled-route warning", warnings)
	}
	// Enabling exactly what loadConfig produced succeeds (route cleared ⇒ off).
	if err := agentsession.EnablePlatformModelRoute(cfg.AgentPlatformModelRoute, cfg.AgentPlatformModel, cfg.AgentPlatformModelWorkspaces, cfg.AgentPlatformModelKey); err != nil {
		t.Fatalf("enable after a disabled route: %v", err)
	}
	if agentsession.PlatformModelKeyReady("tea-d98210cbbpdc73dcrkvg") {
		t.Fatal("route active without a key")
	}

	// Unset route: nothing is required.
	if cfg, _, err := loadFor(t, cpEnv(nil)); err != nil || cfg.AgentPlatformModelRoute != "" {
		t.Fatalf("unset route: %v / %q", err, cfg.AgentPlatformModelRoute)
	}
}
