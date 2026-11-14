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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentProfilesManifestCoversLegacyAdapters(t *testing.T) {
	for _, id := range []string{"claude", "codex", "gemini"} {
		profile, ok := LookupAgentProfile(id)
		if !ok {
			t.Fatalf("profile %q missing from manifest", id)
		}
		switch id {
		case "claude":
			if profile.Executable != "/usr/local/bin/claude-agent-acp" {
				t.Fatalf("claude executable = %q", profile.Executable)
			}
		case "codex":
			if profile.Executable != "/usr/local/bin/codex-acp" {
				t.Fatalf("codex executable = %q", profile.Executable)
			}
			if profile.Env["NO_BROWSER"] != "1" {
				t.Fatalf("codex env = %#v", profile.Env)
			}
		case "gemini":
			if profile.Executable != "/usr/local/bin/gemini" || len(profile.Args) != 1 || profile.Args[0] != "--acp" {
				t.Fatalf("gemini runtime = %#v", profile)
			}
		}
		endpoint, err := RegisteredModelEndpoint("tea-a", id, "", false)
		if err != nil {
			t.Fatalf("RegisteredModelEndpoint(%q) = %v", id, err)
		}
		if endpoint != profile.ModelEndpoint {
			t.Fatalf("profile %q endpoint = %q, manifest = %q", id, endpoint, profile.ModelEndpoint)
		}
	}
}

func TestLookupAgentProfileRejectsUnknown(t *testing.T) {
	if _, ok := LookupAgentProfile("unknown"); ok {
		t.Fatal("unknown profile should fail closed")
	}
}

func TestAgentImageUsesEmbeddedManifest(t *testing.T) {
	dockerfile, err := os.ReadFile(filepath.Join("..", "..", "..", "agent-image", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(dockerfile), "COPY backend/internal/agentsession/agent-profiles.json ./agent-profiles.json") != 2 || !strings.Contains(string(dockerfile), "node dist/install-profiles.js") {
		t.Fatal("image must install and verify profiles from the backend's embedded release artifact")
	}
	if _, err := os.Stat(filepath.Join("..", "..", "..", "agent-image", "agent-profiles.json")); !os.IsNotExist(err) {
		t.Fatal("duplicate image profile manifest must not exist")
	}
}

func TestAgentProfileIDsStable(t *testing.T) {
	ids := AgentProfileIDs()
	if len(ids) < 3 {
		t.Fatalf("ids = %#v", ids)
	}
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			t.Fatalf("empty id in %#v", ids)
		}
	}
}

// Both language validators consume these malformed contracts to prevent drift.
func TestAgentProfileValidationRejectsMalformedContracts(t *testing.T) {
	raw, err := os.ReadFile("profiles-invalid-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name  string
		Path  []any
		Value any
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			var value any
			if err := json.Unmarshal(agentProfilesJSON, &value); err != nil {
				t.Fatal(err)
			}
			cursor := value
			for _, key := range tc.Path[:len(tc.Path)-1] {
				switch v := cursor.(type) {
				case map[string]any:
					cursor = v[key.(string)]
				case []any:
					cursor = v[int(key.(float64))]
				}
			}
			cursor.(map[string]any)[tc.Path[len(tc.Path)-1].(string)] = tc.Value
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var manifest AgentProfileManifest
			err = json.Unmarshal(raw, &manifest)
			if err == nil {
				err = validateAgentProfilesManifest(manifest)
			}
			if err == nil {
				t.Fatal("malformed release profile accepted")
			}
		})
	}
	for _, duplicate := range []string{"id", "executable"} {
		t.Run("duplicate "+duplicate, func(t *testing.T) {
			var manifest AgentProfileManifest
			if err := json.Unmarshal(agentProfilesJSON, &manifest); err != nil {
				t.Fatal(err)
			}
			if duplicate == "id" {
				manifest.Profiles[1].ID = manifest.Profiles[0].ID
			} else {
				manifest.Profiles[1].Executable = manifest.Profiles[0].Executable
			}
			if validateAgentProfilesManifest(manifest) == nil {
				t.Fatal("duplicate accepted")
			}
		})
	}
}

func TestAgentProfileLookupCannotMutateReleaseContract(t *testing.T) {
	p, _ := LookupAgentProfile("codex")
	p.Env["NO_BROWSER"] = "0"
	*p.ModelProxy.BaseURLSuffix = "/evil"
	p, _ = LookupAgentProfile("codex")
	if p.Env["NO_BROWSER"] != "1" || *p.ModelProxy.BaseURLSuffix != "/v1" {
		t.Fatal("lookup mutated shared release contract")
	}
}

func TestClaudeRuntimeEnvPreservesNativeTranscriptBeforeExit(t *testing.T) {
	_, envJSON := AgentProfileRuntimeJSON("claude")
	var env map[string]string
	if err := json.Unmarshal([]byte(envJSON), &env); err != nil {
		t.Fatal(err)
	}
	if env["CLAUDE_CODE_EAGER_FLUSH"] != "1" {
		t.Fatal("Claude must flush native session state before returning the result the driver terminates on")
	}
}

// The muse profile (ADR108) is the release contract its decisions rest on:
// a pinned native binary for both image architectures, the contributor model,
// platform-only, and Muse's own sandbox off under bex's.
func TestMuseProfileContract(t *testing.T) {
	profile, ok := LookupAgentProfile("muse")
	if !ok {
		t.Fatal("muse profile missing from manifest")
	}
	if profile.Executable != "/usr/local/bin/muse-code-acp" || profile.NPMPackage != "@bex-co/muse-code-acp@0.11.0" {
		t.Fatalf("muse adapter = %q from %q", profile.Executable, profile.NPMPackage)
	}
	native := profile.NativeBinary
	if native == nil || native.Path != "/opt/muse/muse" || profile.Env["MUSE_CODE_EXECUTABLE"] != native.Path {
		t.Fatalf("muse native binary = %+v, env = %v", native, profile.Env)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		artifact, ok := native.Artifacts[arch]
		if !ok || !strings.Contains(artifact.URL, "version="+native.Version) {
			t.Fatalf("muse %s artifact = %+v, version %q", arch, artifact, native.Version)
		}
	}
	if profile.Model != "muse-spark-1.3-contributor" || !profile.PlatformOnly {
		t.Fatalf("muse model = %q, platformOnly = %v", profile.Model, profile.PlatformOnly)
	}
	if profile.SessionConfig["sandbox"] != "disabled" || profile.Env["MUSE_CODE_ACP_ALLOW_YOLO"] != "1" {
		t.Fatalf("muse sandbox config = %v, env = %v", profile.SessionConfig, profile.Env)
	}
	endpoint, err := RegisteredModelEndpoint("tea-a", "muse", "", false)
	if err != nil || endpoint != "https://"+MetaModelHost+"/v1" {
		t.Fatalf("muse endpoint = %q, %v", endpoint, err)
	}
	// Until a bex-owned route serves it, no workspace can run it.
	if ProfileServed("tea-a", "muse") {
		t.Fatal("muse served without a route")
	}
	if !ProfileServed("tea-a", "codex") || ProfileServed("tea-a", "unknown") {
		t.Fatal("ProfileServed must pass a BYO-capable profile and refuse an unknown one")
	}
	// A copy handed out cannot change the release contract.
	native.Artifacts["amd64"] = NativeArtifact{}
	profile.SessionConfig["sandbox"] = "enabled"
	again, _ := LookupAgentProfile("muse")
	if again.NativeBinary.Artifacts["amd64"].SHA256 == "" || again.SessionConfig["sandbox"] != "disabled" {
		t.Fatal("lookup leaked a mutable view of the muse profile")
	}
}

func TestResolveAuthSchemeMetaIsBearer(t *testing.T) {
	for _, credential := range []string{"meta-key", "sk-ant-oat01-looks-like-anthropic"} {
		if got := ResolveAuthScheme(MetaModelHost, credential); got != AuthSchemeBearer {
			t.Fatalf("ResolveAuthScheme(meta, %q) = %q, want bearer", credential, got)
		}
	}
}
