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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
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

// Hibernation runs exactly the snapshot script (no scrub hook) with the PUT URL
// as a verbatim argv tail, and keeps the dirty-git signal for retention.
func TestHibernateRunsSnapshotScriptDirectlyAndReportsDirtyGit(t *testing.T) {
	pauseCalls := 0
	upstream := agentSandboxUpstream(t, &pauseCalls)
	const putURL = "https://s3.example.com/snap?X-Amz-Signature=abc"
	sha := strings.Repeat("ab", 32)
	secret := []byte("exec-secret")
	execCalls := 0
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now())
		if err != nil {
			t.Fatalf("ticket: %v", err)
		}
		want := []string{"/bin/sh", "-c", hibernateScript, "bex-hibernate", putURL}
		if strings.Join(claims.Command, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("hibernate command = %q", claims.Command)
		}
		if claims.AgentSessionID != "ags-one" {
			t.Fatalf("agent-session claim = %q", claims.AgentSessionID)
		}
		execCalls++
		w.Header().Set("Content-Type", "text/event-stream")
		out, _ := json.Marshal(map[string]string{"stream": "stdout", "data": sha + "\n4096\ndirty\n"})
		_, _ = fmt.Fprintf(w, "event: output\ndata: %s\n\nevent: exit\ndata: {\"exit_code\":0}\n\n", out)
	}))
	t.Cleanup(gateway.Close)
	service := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: NewClient(upstream.URL),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gateway.URL, Client: gateway.Client()},
	}
	lifecycle := NewAgentSessionLifecycle(service)
	got, err := lifecycle.HibernateAgentSessionSandbox(context.Background(), "tea-a", "ags-one", "os-agent", putURL)
	if err != nil {
		t.Fatalf("hibernate: %v", err)
	}
	if execCalls != 1 || got.Bytes != 4096 || got.SHA256 != sha || !got.DirtyGit {
		t.Fatalf("hibernate result=%+v execCalls=%d", got, execCalls)
	}
	if pauseCalls != 0 {
		t.Fatal("hibernation must not pause; the Completer terminates after recording")
	}
}

// parseHibernateOutput reads the digest/size/dirty lines the snapshot script
// prints; malformed output must fail closed rather than record a bogus snapshot.
func TestParseHibernateOutput(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	ok := []struct {
		name, in string
		bytes    int64
		dirty    bool
	}{
		{"clean", sha + "\n4096\nclean\n", 4096, false},
		{"dirty", sha + "\n12\ndirty\n", 12, true},
		{"trailing blanks", sha + "\n8\nclean\n\n\n", 8, false},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			res, err := parseHibernateOutput(tc.in)
			if err != nil {
				t.Fatalf("parse %q = %v", tc.in, err)
			}
			if res.Bytes != tc.bytes || res.SHA256 != sha || res.DirtyGit != tc.dirty {
				t.Fatalf("got %+v, want bytes=%d dirty=%v", res, tc.bytes, tc.dirty)
			}
		})
	}
	for name, in := range map[string]string{
		"empty":         "",
		"missing dirty": sha + "\n4096\n",
		"zero bytes":    sha + "\n0\nclean\n",
		"bad size":      sha + "\nnotanumber\nclean\n",
		"short digest":  "abcd\n4096\nclean\n",
	} {
		t.Run("reject "+name, func(t *testing.T) {
			if _, err := parseHibernateOutput(in); err == nil {
				t.Fatalf("parse %q should have failed", in)
			}
		})
	}
}

func TestHibernateScriptShowsCurlErrorsAndSnapshotDetailStripsQueries(t *testing.T) {
	if !strings.Contains(hibernateScript, "curl -sSf") {
		t.Fatal("hibernate script must use curl -sSf so DNS/TLS failures reach stderr")
	}
	if !strings.Contains(hibernateScript, "-H 'If-None-Match: *'") {
		t.Fatal("hibernate script must send If-None-Match:* matching the create-once presign")
	}
	got := snapshotExecDetail(ExecResult{Stderr: "curl: (6) Could not resolve host: s3.example.com?X-Amz-Signature=secret"})
	if got != ": curl: (6) Could not resolve host: s3.example.com" {
		t.Fatalf("snapshotExecDetail = %q", got)
	}
	if snapshotExecDetail(ExecResult{}) != "" {
		t.Fatal("empty exec result should add no detail")
	}
}

// TestHibernateScriptTakesURLFromArgv pins the codex-security target #6 fix: the
// presigned PUT URL is passed as the script's first positional argument, never
// interpolated into the script body, so no byte in the URL can break out of the
// `sh -c` boundary.
func TestHibernateScriptTakesURLFromArgv(t *testing.T) {
	if !strings.Contains(hibernateScript, `--upload-file "$SNAP" "$1"`) {
		t.Fatal(`hibernate script must PUT to "$1" (the argv-supplied URL), not an interpolated value`)
	}
	// No format verbs remain: neither the URL nor a scrub hook is spliced in.
	if strings.Contains(hibernateScript, "%s") || strings.Contains(hibernateScript, "snapshot-grant") {
		t.Fatal("hibernate script must be a fixed string with no interpolation slots")
	}
	// A hostile URL never lands in the script text; it stays a verbatim argv tail.
	hostile := "https://s3.example.com/o'; rm -rf / #"
	command := hibernateScript
	if strings.Contains(command, hostile) || strings.Contains(command, "rm -rf") {
		t.Fatal("URL must not be interpolated into the hibernate script body")
	}
	argv := []string{"/bin/sh", "-c", command, "bex-hibernate", hostile}
	if argv[len(argv)-1] != hostile {
		t.Fatalf("URL must be the final argv element verbatim, got %q", argv[len(argv)-1])
	}
}
