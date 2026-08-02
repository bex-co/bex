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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
)

func TestAgentSuspendScrubsBeforeSnapshotAndFailsClosed(t *testing.T) {
	for _, exitCode := range []int{0, 17} {
		t.Run(map[int]string{0: "success pauses", 17: "scrub failure blocks pause"}[exitCode], func(t *testing.T) {
			pauseCalls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/sandboxes/os-agent":
					_ = json.NewEncoder(w).Encode(osSandbox{ID: "os-agent", Metadata: map[string]string{
						metadataOwner: "id-a", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
						metadataNetworkPolicy: string(NetworkPolicyDenyAll), agentsession.LabelSession: "ags-one",
					}})
				case r.Method == http.MethodPost && r.URL.Path == "/sandboxes/os-agent/pause":
					pauseCalls++
					w.WriteHeader(http.StatusNoContent)
				default:
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()

			secret := []byte("exec-secret")
			execCalls := 0
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				claims, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now())
				if err != nil || len(claims.Command) != 3 || claims.Command[2] != "/usr/local/bin/bex-pre-snapshot" {
					t.Fatalf("scrub ticket claims=%+v err=%v", claims, err)
				}
				execCalls++
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("event: exit\ndata: {\"exitCode\":" + map[int]string{0: "0", 17: "17"}[exitCode] + "}\n\n"))
			}))
			defer gateway.Close()

			service := &Service{
				Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
				Client: NewClient(upstream.URL),
				Exec:   &ExecConfig{Secret: secret, GatewayURL: gateway.URL, Client: gateway.Client()},
			}
			err := service.Suspend(callerCtx(), "os-agent")
			if execCalls != 1 {
				t.Fatalf("scrub calls = %d, want 1", execCalls)
			}
			if exitCode == 0 {
				if err != nil || pauseCalls != 1 {
					t.Fatalf("successful scrub err=%v pauseCalls=%d", err, pauseCalls)
				}
			} else if err == nil || pauseCalls != 0 {
				t.Fatalf("failed scrub err=%v pauseCalls=%d", err, pauseCalls)
			}
		})
	}
}
