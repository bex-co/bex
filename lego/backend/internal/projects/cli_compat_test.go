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

package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestOfficialCLIProjectsWalksPopulatedPages is an opt-in compatibility test
// against an unmodified render-oss/cli binary. The ordinary REST walk test is
// unconditional; this one supplies the dated artifact evidence without making
// CI install an external executable. Run with RENDER_CLI_BIN=/path/to/render.
func TestOfficialCLIProjectsWalksPopulatedPages(t *testing.T) {
	bin := os.Getenv("RENDER_CLI_BIN")
	if bin == "" {
		t.Skip("set RENDER_CLI_BIN to an official render CLI binary")
	}
	const total = core.MaxPageLimit + 1 // the CLI requests pages of 100
	seeded := make([]store.Project, 0, total)
	for i := 1; i <= total; i++ {
		seeded = append(seeded, store.Project{
			ID:       fmt.Sprintf("prj-%03d", i),
			TenantID: "tea-cli",
			Name:     fmt.Sprintf("project-%03d", i),
		})
	}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newFakeProjectStore(seeded...)}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/projects" {
			requests.Add(1)
		}
		mux.ServeHTTP(w, r.WithContext(ctxAs("cli-user")))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "projects", "--output", "json")
	cmd.Env = []string{
		"HOME=" + t.TempDir(),
		"PATH=" + os.Getenv("PATH"),
		"RENDER_HOST=" + server.URL + "/v1/",
		"RENDER_API_KEY=compat-test-token",
		"RENDER_WORKSPACE=tea-cli",
		"RENDER_CLI_CONFIG_PATH=" + filepath.Join(t.TempDir(), "cli.yaml"),
	}
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			t.Fatalf("official CLI did not terminate: %v", ctx.Err())
		}
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("official CLI: %v: %s", err, exit.Stderr)
		}
		t.Fatal(err)
	}
	var projects []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &projects); err != nil {
		t.Fatalf("decode official CLI output: %v", err)
	}
	if len(projects) != total || requests.Load() != 2 {
		t.Fatalf("official CLI returned %d projects in %d requests; want %d in 2", len(projects), requests.Load(), total)
	}
}
