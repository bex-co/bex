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

package main_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// bex-api answers a trigger that waits behind the deploy in progress with
// Render's 202 Queued and no body (w8/378, lego/backend/internal/deploys
// TestRESTTriggerAnswersRenders202WhenItsDeployWaits), and one it starts with
// 201 and the deploy. The pinned client must print the deploy and exit 0 on
// both: on a 202 it finds the queued deploy with ?status=queued&limit=1
// (views.WaitForDeployCreate).
func TestDeploysCreatePrintsTheDeployOnRenders201And202(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		wantID   string
		wantList bool
	}{
		{name: "started", status: http.StatusCreated, wantID: "dep-started"},
		{name: "queued", status: http.StatusAccepted, wantID: "dep-queued", wantList: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listed := false
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1/services/srv-q/deploys":
					if tc.status == http.StatusAccepted {
						w.WriteHeader(http.StatusAccepted)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"id":"dep-started","status":"created","trigger":"api"}`))
				case r.Method == http.MethodGet && r.URL.Path == "/v1/services/srv-q/deploys":
					q := r.URL.Query()
					if q.Get("status") != "queued" || q.Get("limit") != "1" {
						t.Errorf("list query = %q, want status=queued&limit=1", r.URL.RawQuery)
					}
					listed = true
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`[{"deploy":{"id":"dep-queued","status":"queued","trigger":"api"},"cursor":"dep-queued"}]`))
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(api.Close)

			command := bexAgainst(t, api.URL, "deploys", "create", "srv-q", "-o", "json", "--confirm")
			var out, errOut bytes.Buffer
			command.Stdout, command.Stderr = &out, &errOut
			if err := command.Run(); err != nil {
				t.Fatalf("deploys create: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
			}
			var got struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.ID != tc.wantID {
				t.Fatalf("stdout = %s (err %v), want the deploy %s", out.String(), err, tc.wantID)
			}
			if listed != tc.wantList {
				t.Errorf("listed queued deploys = %v, want %v", listed, tc.wantList)
			}
		})
	}
}
