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

package agentcredential

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
)

func TestGetFetchesOnDemandAndWritesOnlyStdout(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get(agentsession.NamespaceHeader) != "tea-a-sandbox" {
			t.Errorf("namespace header = %q", r.Header.Get(agentsession.NamespaceHeader))
		}
		var req agentsession.MintRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.SessionID != "ags-one" || req.Repository != "octo/repo" || req.Branch != "bex-agent/task-1" {
			t.Errorf("request = %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		token := "ghs_memory_only"
		if calls == 2 {
			token = "ghs_refreshed_after_ttl"
		}
		_ = json.NewEncoder(w).Encode(agentsession.MintResponse{Username: "x-access-token", Token: token, ExpiresAt: "2026-08-01T13:00:00Z"})
	}))
	defer server.Close()

	dir := t.TempDir()
	before, _ := os.ReadDir(dir)
	var out bytes.Buffer
	cfg := Config{GatewayURL: server.URL, Namespace: "tea-a-sandbox", SessionID: "ags-one", Branch: "bex-agent/task-1", HTTP: server.Client()}
	input := "protocol=https\nhost=github.com\npath=Octo/Repo.git\n\n"
	if err := Run(context.Background(), "get", strings.NewReader(input), &out, cfg); err != nil {
		t.Fatal(err)
	}
	if out.String() != "username=x-access-token\npassword=ghs_memory_only\n\n" || calls != 1 {
		t.Fatalf("output=%q calls=%d", out.String(), calls)
	}
	after, _ := os.ReadDir(dir)
	if len(before) != len(after) || len(after) != 0 {
		t.Fatalf("helper wrote filesystem entries: %v", after)
	}

	// A second Git operation performs a second fetch; no cached result exists.
	out.Reset()
	if err := Run(context.Background(), "get", strings.NewReader(input), &out, cfg); err != nil || calls != 2 || !strings.Contains(out.String(), "ghs_refreshed_after_ttl") {
		t.Fatalf("second get err=%v calls=%d output=%q", err, calls, out.String())
	}
}

func TestStoreEraseNoOpAndUnreachableFailsLoudly(t *testing.T) {
	for _, op := range []string{"store", "erase"} {
		var out bytes.Buffer
		if err := Run(context.Background(), op, strings.NewReader("password=must-not-store\n\n"), &out, Config{}); err != nil || out.Len() != 0 {
			t.Fatalf("%s err=%v output=%q", op, err, out.String())
		}
	}
	var out bytes.Buffer
	err := Run(context.Background(), "get", strings.NewReader("protocol=https\nhost=github.com\npath=octo/repo\n\n"), &out, Config{
		GatewayURL: "http://127.0.0.1:1/session-credential", Namespace: "tea-a-sandbox",
		SessionID: "ags-one", Branch: "bex-agent/task-1",
	})
	if err == nil || !strings.Contains(err.Error(), "could not reach") || out.Len() != 0 {
		t.Fatalf("unreachable err=%v output=%q", err, out.String())
	}
}
