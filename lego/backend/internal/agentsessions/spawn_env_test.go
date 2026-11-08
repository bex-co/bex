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
	"net/url"
	"regexp"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR102 D9: the sandbox gains a way to start children without gaining a
// credential. The only environment the spawn path adds is its Pod-bound URL
// (a path naming the session, no userinfo or query), and nothing in the
// driver's environment looks like a bex API credential or user token.
func TestSpawnPathAddsOnlyAnOpaqueURLToTheSandboxEnv(t *testing.T) {
	svc, _, _, lifecycle := fixture()
	view, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	with := lifecycle.driverEnv

	spawnURL := with["BEX_AGENT_SPAWN_URL"]
	parsed, err := url.Parse(spawnURL)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		t.Fatalf("spawn URL %q carries more than a path (%v)", spawnURL, err)
	}
	ns, sid, _, err := agentsession.ParseSpawnPath(parsed.Path)
	if err != nil || ns != "tea-a-sandbox" || sid != view.ID {
		t.Fatalf("spawn URL %q names %q %q / %v", spawnURL, ns, sid, err)
	}

	// A session at the depth limit is offered no URL at all.
	deep := store.AgentSession{ID: view.ID, WorkspaceID: "tea-a", Depth: store.MaxAgentSessionDepth}
	if _, ok := svc.withSpawnEnv(map[string]string{}, deep)["BEX_AGENT_SPAWN_URL"]; ok {
		t.Fatal("a depth-limit session got a spawn URL")
	}

	// No variable of the sandbox looks like a bex API credential or user token.
	credential := regexp.MustCompile(`(?i)(token|secret|password|passwd|credential|bearer|api_?key|authorization)`)
	allowed := map[string]bool{
		"BEX_AGENT_GRANT_PUBLIC_KEY": true, // a verification key, not a secret
		"BEX_AGENT_MODEL_API_KEY":    true, // the per-session placeholder the model proxy swaps
	}
	for name, value := range with {
		if credential.MatchString(name) && !allowed[name] {
			t.Errorf("sandbox env %s looks like a credential", name)
		}
		if credential.MatchString(value) && name != "BEX_AGENT_MODEL_API_KEY" && name != "BEX_AGENT_PROMPT" {
			t.Errorf("sandbox env %s=%q carries credential-looking text", name, value)
		}
	}
	if placeholder := with["BEX_AGENT_MODEL_API_KEY"]; placeholder != "" && placeholder != agentsession.ModelKeyPlaceholder(view.ID) {
		t.Errorf("BEX_AGENT_MODEL_API_KEY = %q, want the placeholder", placeholder)
	}
}
