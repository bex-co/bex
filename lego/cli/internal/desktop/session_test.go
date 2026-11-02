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

package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/render-oss/cli/pkg/client"
)

// The contract file mirrors the backend View's field names this command
// reads; a backend rename has to move it, and this test, together.
func TestAgentSessionContractFields(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/agent-session-ssh-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var session agentSession
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatal(err)
	}
	if session.ID == "" || session.SSHAddress == "" {
		t.Fatalf("contract fields did not decode: %+v", session)
	}
}

func TestFetchSSHAddress(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   string
		err    string
	}{
		"running":   {200, `{"id":"ags-a1","phase":"running","sshAddress":"ags-a1@ssh.bex.co"}`, "ags-a1@ssh.bex.co", ""},
		"closed":    {200, `{"id":"ags-a1","phase":"completed"}`, "", "no running sandbox"},
		"missing":   {404, `{"id":"not_found","message":"agent session not found"}`, "", "404 agent session not found"},
		"bex error": {403, `{"error":"insufficient OAuth scope"}`, "", "403 insufficient OAuth scope"},
		"forbidden": {403, `forbidden`, "", "403 Forbidden"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var gotPath, gotAuth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c, err := client.NewClient(srv.URL+"/v1/", client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer test")
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			got, err := fetchSSHAddress(context.Background(), c, "ags-a1")
			if gotPath != "/v1/agent-sessions/ags-a1" || gotAuth != "Bearer test" {
				t.Fatalf("request path %q auth %q", gotPath, gotAuth)
			}
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}
