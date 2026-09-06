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
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"
)

//go:embed agent-profiles.json
var agentProfilesJSON []byte

// AgentProfileManifest is the release-locked, non-secret runtime profile contract
// shared by bex-api and the in-pod driver (w5/m77).
type AgentProfileManifest struct {
	Version  int            `json:"version"`
	Profiles []AgentProfile `json:"profiles"`
}

// AgentProfile binds a public profile id to an operator-owned executable and the
// model-proxy routing metadata the gateway credential path expects.
type AgentProfile struct {
	ID               string            `json:"id"`
	Executable       string            `json:"executable"`
	Args             []string          `json:"args"`
	Env              map[string]string `json:"env"`
	ModelEndpoint    string            `json:"modelEndpoint"`
	NPMPackage       string            `json:"npmPackage"`
	Authentication   string            `json:"authentication"`
	PermissionPolicy string            `json:"permissionPolicy"`
	ModelProxy       ModelProxyRoute   `json:"modelProxy"`
	// SessionState names the HOME-relative dirs/files carrying this agent's
	// on-disk conversation state. The hibernation snapshot must include them —
	// they are the substrate of ADR047 D3's `session/load` resume rung (ADR059
	// D3 continuity amendment, w5/m84); a guard test binds this list to the
	// sandbox hibernate script.
	SessionState []string `json:"sessionState"`
}

// ModelProxyRoute describes how the driver points a provider SDK at the gateway
// model proxy for a profile.
type ModelProxyRoute struct {
	BaseURLEnv    string  `json:"baseUrlEnv"`
	BaseURLSuffix *string `json:"baseUrlSuffix"`
	CredentialEnv string  `json:"credentialEnv"`
}

var (
	agentProfilesManifest    AgentProfileManifest
	agentProfilesByID        map[string]AgentProfile
	profileIDPattern         = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	profileExecutablePattern = regexp.MustCompile(`^/usr/local/bin/[a-zA-Z0-9_-]+$`)
	profilePackagePattern    = regexp.MustCompile(`^@[a-z0-9-]+/[a-z0-9-]+@[0-9]+\.[0-9]+\.[0-9]+$`)
	profileStatePattern      = regexp.MustCompile(`^\.[a-zA-Z0-9_-]+(?:\.[a-zA-Z0-9_-]+)*$`)
	profileEndpointPattern   = regexp.MustCompile(`^https://[a-z0-9]+(?:[.-][a-z0-9]+)*\.[a-z]+(?:/[a-zA-Z0-9_-]+)*$`)
	profileSuffixPattern     = regexp.MustCompile(`^(?:/[a-zA-Z0-9_-]+)*$`)
	envNamePattern           = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
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
	if m.Version != 1 || len(m.Profiles) == 0 {
		return fmt.Errorf("version must be 1 and profiles must be nonempty")
	}
	seen := make(map[string]struct{}, len(m.Profiles))
	commands := make(map[string]bool)
	for _, profile := range m.Profiles {
		id := profile.ID
		if !profileIDPattern.MatchString(id) {
			return fmt.Errorf("profile id is required")
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate profile id %q", id)
		}
		seen[id] = struct{}{}
		if !profileExecutablePattern.MatchString(profile.Executable) || commands[profile.Executable] {
			return fmt.Errorf("profile %q: executable must be a unique installed adapter path", id)
		}
		commands[profile.Executable] = true
		if profile.Args == nil || profile.Env == nil {
			return fmt.Errorf("profile %q: args and env are required", id)
		}
		for _, arg := range profile.Args {
			if strings.ContainsRune(arg, 0) {
				return fmt.Errorf("profile %q: unsafe arg", id)
			}
		}
		for key, value := range profile.Env {
			if !envNamePattern.MatchString(key) || strings.ContainsRune(value, 0) {
				return fmt.Errorf("profile %q: invalid env", id)
			}
		}
		if !profilePackagePattern.MatchString(profile.NPMPackage) {
			return fmt.Errorf("profile %q: exact npm package pin is required", id)
		}
		if profile.Authentication != "environment" || profile.PermissionPolicy != "sandbox-auto-approve" {
			return fmt.Errorf("profile %q: unsupported authentication or permission policy", id)
		}
		if len(profile.SessionState) == 0 {
			return fmt.Errorf("profile %q: session state is required", id)
		}
		for _, value := range profile.SessionState {
			if !profileStatePattern.MatchString(value) {
				return fmt.Errorf("profile %q: unsafe session state path", id)
			}
		}
		if !profileEndpointPattern.MatchString(profile.ModelEndpoint) {
			return fmt.Errorf("profile %q: invalid model endpoint", id)
		}

		route := profile.ModelProxy
		if route.BaseURLSuffix == nil || !profileSuffixPattern.MatchString(*route.BaseURLSuffix) || route.BaseURLEnv == route.CredentialEnv {
			return fmt.Errorf("profile %q: incomplete model proxy routing", id)
		}
		for _, name := range []string{"BEX_AGENT_MODEL_API_KEY", route.BaseURLEnv, route.CredentialEnv} {
			if _, exists := profile.Env[name]; exists {
				return fmt.Errorf("profile %q: bootstrap env overrides model routing", id)
			}
		}
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
	profile.Args = slices.Clone(profile.Args)
	profile.Env = maps.Clone(profile.Env)
	profile.SessionState = slices.Clone(profile.SessionState)
	if profile.ModelProxy.BaseURLSuffix != nil {
		value := *profile.ModelProxy.BaseURLSuffix
		profile.ModelProxy.BaseURLSuffix = &value
	}
	return profile, ok
}

// AgentProfileCommand returns the operator-owned executable for a profile id.
func AgentProfileCommand(agent string) string {
	profile, _ := LookupAgentProfile(agent)
	return profile.Executable
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
