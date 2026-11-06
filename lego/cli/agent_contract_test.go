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
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/render-oss/cli/cmd"
)

// The pinned Render CLI must not own `agent`: bex's Cloud Agents command
// would silently replace it. A pin bump that adds one fails here.
func TestPinnedUpstreamHasNoRootAgentCommand(t *testing.T) {
	for _, c := range cmd.RootCmd.Commands() {
		for _, name := range []string{"agent", "agents"} {
			if c.Name() == name || c.HasAlias(name) {
				t.Fatalf("upstream %q now claims `%s`; reconcile with bex agent before bumping the pin", c.CommandPath(), name)
			}
		}
	}
}

// `bex agent` reaches bex-api through the upstream client (its host, bearer
// and control-plane headers), and its state exit codes survive upstream's
// executor.
func TestBexBinaryAgentWaitAuthenticatesAndExitsByState(t *testing.T) {
	var gotAuth, gotPath string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ags-d0abc123def456ghij78","ownerId":"tea-x","state":"failed","phase":"failed",` +
			`"failureReasonCode":"STRUCTURED_OUTPUT_MISSING","failureReason":"no result","agentConfig":{"agent":"claude","task":"t"}}`))
	}))
	t.Cleanup(api.Close)

	c := exec.Command(buildBex(), "agent", "wait", "ags-d0abc123def456ghij78", "-o", "json")
	c.Env = append(withoutRenderEnv(os.Environ()),
		"HOME="+t.TempDir(),
		"BEX_HOST="+api.URL+"/v1/",
		"BEX_ACCESS_TOKEN=test-access-token",
		"BEX_NO_UPDATE_NOTIFIER=1",
	)
	var stdout, stderr bytes.Buffer
	c.Stdout, c.Stderr = &stdout, &stderr
	_ = c.Run()
	if code := c.ProcessState.ExitCode(); code != 3 {
		t.Fatalf("exit %d, want 3\nstdout %s\nstderr %s", code, stdout.String(), stderr.String())
	}
	if gotPath != "/v1/agent-sessions/ags-d0abc123def456ghij78" || gotAuth != "Bearer test-access-token" {
		t.Fatalf("request %s with %q", gotPath, gotAuth)
	}
	if !strings.Contains(stdout.String(), `"state": "failed"`) {
		t.Fatalf("stdout is not the session JSON: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "STRUCTURED_OUTPUT_MISSING: no result") {
		t.Fatalf("stderr lacks the reason: %s", stderr.String())
	}
}
