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

// Package code implements the Bex-native coding-agent launchers: `bex glm`,
// `bex muse`, `bex kimi`, and `bex deepseek` each start a Claude Code
// instance connected to that provider's Anthropic-compatible endpoint, in an
// isolated per-provider CLAUDE_CONFIG_DIR, with a bring-your-own API key
// captured once (hidden paste, validated live, stored owner-only). `bex code`
// is the management hub (key status, set/unset). Claude Code is an external
// runtime dependency; the integration uses only its documented environment
// seams (ANTHROPIC_BASE_URL, ANTHROPIC_AUTH_TOKEN, CLAUDE_CONFIG_DIR, model
// slot variables) — nothing is forked and no configuration file is generated.
package code

// A Provider is one Anthropic-compatible model API in the curated catalog.
// Every provider documents this exact usage (point ANTHROPIC_BASE_URL at the
// endpoint, supply the key as a bearer token), so adding one is a data change.
type Provider struct {
	// Name is the top-level command (`bex <name>`) and the key-store entry.
	Name        string
	DisplayName string
	// BaseURL is the ANTHROPIC_BASE_URL value (Claude Code appends /v1/messages).
	BaseURL string
	// ConsoleURL is where the provider issues API keys; opened during the
	// first-launch key capture.
	ConsoleURL string
	// KeyEnvs are probed for the key, first set value wins; the environment
	// always beats the stored key file.
	KeyEnvs []string
	// DefaultModel fills ANTHROPIC_MODEL and the opus/sonnet slots;
	// SmallFastModel fills ANTHROPIC_SMALL_FAST_MODEL and the haiku slot.
	DefaultModel   string
	SmallFastModel string
}

// Catalog is the curated provider set. Endpoints and model ids were verified
// against each provider's own Claude-integration documentation; the launcher
// surfaces the provider's error verbatim if one drifts.
func Catalog() []Provider {
	return []Provider{
		{
			Name:           "glm",
			DisplayName:    "GLM (Z.ai coding plan)",
			BaseURL:        "https://api.z.ai/api/anthropic",
			ConsoleURL:     "https://z.ai/manage-apikey/apikey-list",
			KeyEnvs:        []string{"ZAI_API_KEY", "GLM_API_KEY"},
			DefaultModel:   "glm-5.2",
			SmallFastModel: "glm-5-turbo",
		},
		{
			Name:           "muse",
			DisplayName:    "Muse Spark (Meta Model API)",
			BaseURL:        "https://api.meta.ai",
			ConsoleURL:     "https://dev.meta.ai",
			KeyEnvs:        []string{"META_MODEL_API_KEY", "META_API_KEY"},
			DefaultModel:   "muse-spark-1.2",
			SmallFastModel: "muse-spark-1.2",
		},
		{
			Name:           "kimi",
			DisplayName:    "Kimi (Moonshot platform)",
			BaseURL:        "https://api.moonshot.ai/anthropic",
			ConsoleURL:     "https://platform.moonshot.ai/console/api-keys",
			KeyEnvs:        []string{"MOONSHOT_API_KEY", "KIMI_API_KEY"},
			DefaultModel:   "kimi-k3",
			SmallFastModel: "kimi-k2.5",
		},
		{
			Name:           "deepseek",
			DisplayName:    "DeepSeek",
			BaseURL:        "https://api.deepseek.com/anthropic",
			ConsoleURL:     "https://platform.deepseek.com/api_keys",
			KeyEnvs:        []string{"DEEPSEEK_API_KEY"},
			DefaultModel:   "deepseek-chat",
			SmallFastModel: "deepseek-chat",
		},
	}
}

func findProvider(name string) *Provider {
	for _, p := range Catalog() {
		if p.Name == name {
			provider := p
			return &provider
		}
	}
	return nil
}

func providerNames() string {
	names := ""
	for i, p := range Catalog() {
		if i > 0 {
			names += ", "
		}
		names += p.Name
	}
	return names
}
