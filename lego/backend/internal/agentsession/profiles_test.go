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
			if profile.Executable != "/usr/local/bin/claude-code-acp" {
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
		endpoint, err := RegisteredModelEndpoint(id, "")
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

func TestAgentProfilesManifestMatchesAgentImageCopy(t *testing.T) {
	imagePath := filepath.Join("..", "..", "..", "agent-image", "agent-profiles.json")
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("read agent-image manifest: %v", err)
	}
	if string(imageBytes) != string(agentProfilesJSON) {
		t.Fatal("agentsession embed drifted from lego/agent-image/agent-profiles.json")
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
