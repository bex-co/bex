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

package agentsessions

import (
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
)

func createEgress(config AgentConfig, requested []string) (string, []string, error) {
	endpoint := config.ModelEndpoint
	if endpoint == "" {
		switch strings.ToLower(strings.TrimSpace(config.Agent)) {
		case "codex", "openai":
			endpoint = "https://api.openai.com/v1"
		case "claude", "claude-code", "anthropic":
			endpoint = "https://api.anthropic.com/v1"
		case "gemini", "google":
			endpoint = "https://generativelanguage.googleapis.com/v1beta"
		default:
			return "", nil, core.NewBadRequestError(
				"AGENT_SESSION_MODEL_ENDPOINT_INVALID",
				"agentConfig.modelEndpoint is required for this agent provider",
				map[string]any{"field": "agentConfig.modelEndpoint"},
			)
		}
	}
	if _, err := sessionegress.ModelEndpointHost(endpoint); err != nil {
		return "", nil, err
	}
	allowlist, err := sessionegress.ExtraDestinations(requested)
	if err != nil {
		return "", nil, err
	}
	return endpoint, allowlist, nil
}
