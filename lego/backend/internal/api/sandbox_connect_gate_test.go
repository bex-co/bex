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

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// TestSandboxConnectTokenIsNotAnAPIBearer pins the two halves of the run
// connect-token boundary on the composed root mux (w7/m147): a connect token
// presented to any OAuth-gated route is refused by the gate (it is not an
// introspectable credential), and the outside-gate redeem route refuses an
// OAuth-shaped bearer — each credential works on exactly its own route.
func TestSandboxConnectTokenIsNotAnAPIBearer(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/os-1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "os-1", "metadata": map[string]string{
			"bex.co/owner": "identity-1", "bex.co/workspace": "tea-a", "app.bex.co/regime": "sandbox", "bex.co/network-policy": "deny-all",
		}})
	}))
	t.Cleanup(upstream.Close)
	srv := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Workspace: fakeWorkspace{"identity-1": "tea-a"}, Authz: &fakeChecker{allow: true}}, Deps{
		SandboxClient: sandbox.NewClient(upstream.URL),
		SandboxExec:   &sandbox.ExecConfig{Secret: []byte("exec-secret"), GatewayURL: "http://127.0.0.1:1", TTL: time.Minute},
	})
	srv.HydraAdminURL = fakeHydraURL(t)
	handler, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	root := httptest.NewServer(handler)
	t.Cleanup(root.Close)

	// Mint a real connect token through the service, as the gated route would.
	ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "identity-1", Method: "oauth2"})
	minted, err := srv.Sandbox.ConnectRun(ctx, sandbox.ConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: sandbox.ConnectOperationStream, Command: "true"}, root.URL)
	if err != nil {
		t.Fatalf("ConnectRun: %v", err)
	}

	// (1) The connect token is not an API bearer: the gated exec route 401s.
	req, _ := http.NewRequest(http.MethodPost, root.URL+"/v1/sandboxes/os-1/exec", strings.NewReader(`{"command":"true"}`))
	req.Header.Set("Authorization", "Bearer "+minted.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("connect token at the gated exec route = %d, want 401", resp.StatusCode)
	}

	// (2) An OAuth-shaped bearer is not a connect token: the redeem route 401s
	// with a JSON {message} body, and never reaches the gateway.
	req, _ = http.NewRequest(http.MethodPost, minted.URI, strings.NewReader(`{"command":"true"}`))
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != http.StatusUnauthorized || body["message"] == "" {
		t.Fatalf("oauth bearer at the redeem route = %d %v, want 401 with a message", resp.StatusCode, body)
	}
}
