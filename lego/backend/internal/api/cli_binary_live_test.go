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

// cli_binary_live_test.go drives the REAL, unmodified `bex` CLI binary against
// the REAL composed bex-api handler over HTTP, authenticating with a genuine
// Hydra-introspected human device-flow token (carrying cliauth.DeviceGrantScope).
// It is the live counterpart to the in-process TestRenderCLITokenClears… test:
// where that asserts HTTP status codes, this proves the actual CLI executable
// exits 0 and decodes the surface for a freshly-"logged-in" user — the exact
// path that produced "you are not allowed to take this action".
//
// Env-gated on BEX_CLI_BIN (path to a built `bex`), so it stays inert in normal
// CI. Reproduce:
//
//	cd lego/cli && go build -o /tmp/bex .
//	BEX_CLI_BIN=/tmp/bex go test ./internal/api -run TestRenderCLIBinaryAgainstLiveServer -v

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/cliauth"
	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const liveWorkspace = "tea-clilive"

// liveCLIServer builds the real composed bex-api handler over a fake Hydra that
// introspects testToken as the platform-marked, audience-less human device-flow
// token the CLI now mints, backed by one running App in liveWorkspace.
func liveCLIServer(t *testing.T) *httptest.Server {
	t.Helper()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: liveWorkspace,
			Labels: map[string]string{core.LabelTenant: liveWorkspace},
		},
		Spec:   appv1alpha1.AppSpec{Image: "web:v1", Replicas: 2},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, URL: "https://web.onbex.co"},
	}
	base := &core.Base{Client: fakeClient(app), Namespace: "default", Audit: &fakeAuditSink{}}
	hydra := newClassHydraScoped(t, "cli-user", "render-cli", cliauth.DeviceGrantScope,
		nil, map[string]bool{"render-cli": true})
	srv := NewServer(base, Deps{})
	srv.HydraAdminURL = hydra.url
	srv.OAuthResource = bexResource
	srv.OAuthRequireAudience = true
	srv.OAuthPlatformClients = hydra.platformClientIDs
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s
}

func TestRenderCLIBinaryAgainstLiveServer(t *testing.T) {
	bin := os.Getenv("BEX_CLI_BIN")
	if bin == "" {
		t.Skip("BEX_CLI_BIN not set; build lego/cli and point this at the binary")
	}
	srv := liveCLIServer(t)
	home := t.TempDir()

	runCLI := func(workspace string, args ...string) (string, string, error) {
		cmd := exec.Command(bin, args...)
		// Only BEX_* inputs, exactly like scripts/bex-cli-auth-e2e.sh: prove the
		// launcher defaults target bex, and inject the token + host + workspace.
		cmd.Env = append(os.Environ(),
			"HOME="+home,
			"BEX_HOST="+srv.URL+"/v1/",
			"BEX_ACCESS_TOKEN="+testToken,
			"BEX_WORKSPACE="+workspace,
		)
		for _, k := range []string{
			"RENDER_API_KEY", "RENDER_HOST", "RENDER_CLI_CONFIG_PATH", "RENDER_WORKSPACE",
			"RENDER_OUTPUT", "BEX_CLI_CONFIG_DIR", "BEX_CLI_CONFIG_PATH",
		} {
			cmd.Env = filterEnv(cmd.Env, k)
		}
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		err := cmd.Run()
		return out.String(), errb.String(), err
	}

	refused := func(t *testing.T, s string) {
		t.Helper()
		for _, needle := range []string{"not allowed to take this action", "INSUFFICIENT_SCOPE", "403"} {
			if strings.Contains(s, needle) {
				t.Fatalf("CLI was refused (%q): %s", needle, s)
			}
		}
	}

	t.Run("image create disabled auto deploy", func(t *testing.T) {
		for _, explicit := range []bool{false, true} {
			name := "image-omitted"
			if explicit {
				name = "image-disabled"
			}
			args := []string{"services", "create", "--name", name, "--type", "web_service", "--runtime", "image",
				"--image", "docker.io/traefik/whoami:v1.11.0", "--env-var", "WHOAMI_PORT_NUMBER=3000",
				"--env-var", "QA_MARKER=m99", "--plan", "free", "--region", "frankfurt", "--num-instances", "1",
				"--health-check-path", "/", "--confirm", "-o", "json"}
			if explicit {
				args = append(args, "--auto-deploy=false")
			}
			out, errb, err := runCLI(liveWorkspace, args...)
			if err != nil {
				t.Fatalf("create: %v stdout=%s stderr=%s", err, out, errb)
			}
			var created struct {
				ID         string `json:"id"`
				AutoDeploy string `json:"autoDeploy"`
				Trigger    string `json:"autoDeployTrigger"`
			}
			if err := json.Unmarshal([]byte(out), &created); err != nil {
				t.Fatal(err)
			}
			if created.ID != "" {
				t.Cleanup(func() {
					// The CLI delete picker needs a projects store; delete directly in this
					// in-memory harness, through the same authenticated backend route.
					req, err := http.NewRequest(http.MethodDelete, srv.URL+"/v1/services/"+created.ID, nil)
					if err != nil {
						t.Error(err)
						return
					}
					req.Header.Set("Authorization", "Bearer "+testToken)
					response, err := srv.Client().Do(req)
					if err != nil {
						t.Error(err)
						return
					}
					defer response.Body.Close()
					if response.StatusCode >= 300 {
						t.Errorf("fixture cleanup: HTTP %d", response.StatusCode)
					}
				})
			}
			if created.ID == "" || created.AutoDeploy != "no" || created.Trigger != "off" {
				t.Fatalf("unexpected create: %s", out)
			}

		}
	})

	// read — `bex services`, the exact command in the bug report. The decisive
	// assertion is that the human device-flow token is NEVER refused (before the
	// fix it 403'd here with "you are not allowed to take this action"). The CLI
	// resolves projects/environments as part of the listing, which needs the
	// optional control-plane store; unwired here it answers 503, so we accept
	// either a full App listing (store present) or that store-503 — both prove
	// the request cleared auth + the scope matrix.
	t.Run("services", func(t *testing.T) {
		out, errb, err := runCLI(liveWorkspace, "services", "-o", "json")
		refused(t, out+errb)
		if strings.Contains(out+errb, "store not configured") {
			t.Logf("bex services cleared auth + scope matrix; blocked only on the unwired projects store (503)")
			return
		}
		if err != nil {
			t.Fatalf("bex services failed for a non-auth, non-store reason: %v\nstdout=%s\nstderr=%s", err, out, errb)
		}
		if !strings.Contains(out, "web") {
			t.Fatalf("bex services did not list the running App: %s", out)
		}
		var decoded any
		if e := json.Unmarshal([]byte(out), &decoded); e != nil {
			t.Fatalf("bex services output not valid JSON: %v\n%s", e, out)
		}
		t.Logf("bex services -o json listed the App (%d bytes): %s", len(out), strings.TrimSpace(out))
	})

	// read — `bex workspaces`. The owners store is unwired in this minimal
	// harness, so a 503 "store not configured" is the expected backend limit; the
	// point here is only that auth + scope classification never refuse it.
	t.Run("workspaces", func(t *testing.T) {
		out, errb, _ := runCLI(liveWorkspace, "workspaces", "-o", "json")
		refused(t, out+errb)
		if strings.Contains(out+errb, "store not configured") {
			t.Logf("bex workspaces reached the handler (owners store unwired here): authorized OK")
			return
		}
		t.Logf("bex workspaces -o json OK: %s", strings.TrimSpace(out))
	})
}

func filterEnv(env []string, key string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return out
}
