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

package sandbox

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// agentSandboxUpstream serves one agent-session sandbox and counts pauses.
func agentSandboxUpstream(t *testing.T, pauseCalls *int) *httptest.Server {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes/os-agent":
			sb := osSandbox{ID: "os-agent", Metadata: map[string]string{
				metadataOwner: "id-a", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
				metadataNetworkPolicy: string(NetworkPolicyDenyAll), agentsession.LabelSession: "ags-one",
			}}
			sb.Status.State = "Running"
			_ = json.NewEncoder(w).Encode(sb)
		case r.Method == http.MethodPost && r.URL.Path == "/sandboxes/os-agent/pause":
			*pauseCalls++
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	return upstream
}

// An agent-session sandbox suspends directly: the sandbox holds only the
// model-proxy placeholder and a credential-free Git remote, so there is no
// pre-snapshot exec hop (ADR090, ADR062) — for an admin or a contributor.
func TestAgentSuspendSnapshotsWithoutPreSnapshotExec(t *testing.T) {
	for name, authz := range map[string]core.Checker{"owner": nil, "contributor": contributorChecker{}} {
		t.Run(name, func(t *testing.T) {
			pauseCalls := 0
			upstream := agentSandboxUpstream(t, &pauseCalls)
			gateway := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("suspend must not exec into the agent sandbox")
			}))
			t.Cleanup(gateway.Close)
			base := &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}}
			if authz != nil {
				base.Authz = authz
			}
			service := &Service{
				Base:   base,
				Client: NewClient(upstream.URL),
				Exec:   &ExecConfig{Secret: []byte("s"), GatewayURL: gateway.URL, Client: gateway.Client()},
			}
			if err := service.Suspend(callerCtx(), "os-agent"); err != nil {
				t.Fatalf("suspend: %v", err)
			}
			if pauseCalls != 1 {
				t.Fatalf("pause calls = %d, want 1", pauseCalls)
			}
		})
	}
}
