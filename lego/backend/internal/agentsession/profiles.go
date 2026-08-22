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
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

//go:embed agent-profiles.json
var agentProfilesJSON []byte

// AgentProfileManifest is the release-locked, non-secret runtime profile contract
// shared by bex-api and the in-pod driver (w5/m77).
type AgentProfileManifest struct {
	Version  int           `json:"version"`
	Profiles []AgentProfile `json:"profiles"`
}

// AgentProfile binds a public profile id to an operator-owned executable and the
// model-proxy routing metadata the gateway credential path expects.
type AgentProfile struct {
	ID            string            `json:"id"`
	Executable    string            `json:"executable"`
	Args          []string          `json:"args"`
	Env           map[string]string `json:"env"`
	ModelEndpoint string            `json:"modelEndpoint"`
	ModelProxy    ModelProxyRoute   `json:"modelProxy"`
}

// ModelProxyRoute describes how the driver points a provider SDK at the gateway
// model proxy for a profile.
type ModelProxyRoute struct {
	BaseURLEnv     string `json:"baseUrlEnv"`
	BaseURLSuffix  string `json:"baseUrlSuffix"`
	CredentialEnv  string `json:"credentialEnv"`
}

var (
	agentProfilesManifest AgentProfileManifest
	agentProfilesByID     map[string]AgentProfile
	envNamePattern        = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
)

func init() {
	if err := json.Unmarshal(agentProfilesJSON, &agentProfilesManifest); err != nil {
		panic(fmt.Sprintf("agent profiles manifest: %v", err))
	}
	if err := validateAgentProfilesManifest(agentProfilesManifest); err != nil {
		panic(fmt.Sprintf("agent profiles manifest: %v", err))
	}
	agentProfilesByID = make(map[string]AgentProfile, len(agentProfilesManifest.Profiles))
	for _, profile := range agentProfilesManifest.Profiles {
		agentProfilesByID[profile.ID] = profile
	}
}

func validateAgentProfilesManifest(m AgentProfileManifest) error {
	if m.Version < 1 {
		return fmt.Errorf("version must be >= 1")
	}
	seen := make(map[string]struct{}, len(m.Profiles))
	for _, profile := range m.Profiles {
		id := strings.ToLower(strings.TrimSpace(profile.ID))
		if id == "" {
			return fmt.Errorf("profile id is required")
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate profile id %q", id)
		}
		seen[id] = struct{}{}
		if !filepath.IsAbs(profile.Executable) || profile.Executable != filepath.Clean(profile.Executable) {
			return fmt.Errorf("profile %q: executable must be an absolute path", id)
		}
		if strings.Contains(profile.Executable, "..") {
			return fmt.Errorf("profile %q: executable path is unsafe", id)
		}
		for key := range profile.Env {
			if !envNamePattern.MatchString(key) {
				return fmt.Errorf("profile %q: env key %q is invalid", id, key)
			}
		}
		if strings.TrimSpace(profile.ModelEndpoint) == "" {
			return fmt.Errorf("profile %q: modelEndpoint is required", id)
		}
		route := profile.ModelProxy
		for _, name := range []string{route.BaseURLEnv, route.CredentialEnv} {
			if !envNamePattern.MatchString(name) {
				return fmt.Errorf("profile %q: model proxy env %q is invalid", id, name)
			}
		}
	}
	return nil
}

// AgentProfileIDs returns the supported public profile identifiers in stable order.
func AgentProfileIDs() []string {
	ids := make([]string, 0, len(agentProfilesByID))
	for id := range agentProfilesByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// LookupAgentProfile resolves a public profile id to the release-locked manifest row.
func LookupAgentProfile(agent string) (AgentProfile, bool) {
	profile, ok := agentProfilesByID[strings.ToLower(strings.TrimSpace(agent))]
	return profile, ok
}

// AgentProfileCommand returns the operator-owned executable for a profile id.
func AgentProfileCommand(agent string) string {
	return LookupAgentProfileOrEmpty(agent).Executable
}

// AgentProfileRuntimeJSON renders args/env JSON for the sandbox driver environment.
func AgentProfileRuntimeJSON(agent string) (argsJSON, envJSON string) {
	profile, ok := LookupAgentProfile(agent)
	if !ok {
		return "", ""
	}
	if len(profile.Args) > 0 {
		raw, _ := json.Marshal(profile.Args)
		argsJSON = string(raw)
	}
	if len(profile.Env) > 0 {
		raw, _ := json.Marshal(profile.Env)
		envJSON = string(raw)
	}
	return argsJSON, envJSON
}

func LookupAgentProfileOrEmpty(agent string) AgentProfile {
	profile, _ := LookupAgentProfile(agent)
	return profile
}
