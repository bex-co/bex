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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Since v2.38.0 the imported CLI ends every command with a once-a-day "Update
// available" notice (render-oss/cli internal/updatenotice, started in
// cmd.Execute). It compares the pinned Render CLI version against
// render-oss/cli releases and tells the reader to install Render's CLI from
// render.com, so it must never print in bex, whatever the state directory
// holds: bex's own release hint (internal/update) speaks for bex. Upstream
// gives AI agents the notice as one line, terminal or not, so an agent's
// marker drives the check here without a pseudo-terminal.
func TestUpstreamUpdateNoticeNeverPrints(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/owners" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"owner":{"id":"tea-bex","name":"Bex","email":"bex@example.test","type":"team"}}]`))
	}))
	t.Cleanup(api.Close)

	// The states upstream reads from <state dir>/update-check.json: none yet, a
	// lookup younger than its four-hour reuse window (no request at all), and
	// one older than it, whose refreshed lookup fails here and keeps the cached
	// release. The newer release is what Render's own CLI records when the two
	// share a state directory (an explicit RENDER_CLI_CONFIG_DIR).
	newer := func(checked time.Time) string {
		raw, _ := json.Marshal(map[string]any{
			"latest_version": "99.0.0",
			"checked_at":     checked.UTC().Format(time.RFC3339),
			"published_at":   checked.Add(-48 * time.Hour).UTC().Format(time.RFC3339),
		})
		return string(raw)
	}
	for name, cached := range map[string]string{
		"no cache":    "",
		"fresh cache": newer(time.Now().Add(-time.Hour)),
		"stale cache": newer(time.Now().Add(-5 * time.Hour)),
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			state := filepath.Join(home, ".bex", "state")
			if err := os.MkdirAll(state, 0o700); err != nil {
				t.Fatal(err)
			}
			cache := filepath.Join(state, "update-check.json")
			if cached != "" {
				if err := os.WriteFile(cache, []byte(cached), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			for _, args := range [][]string{{"workspaces", "-o", "text"}, {"help"}} {
				command := exec.Command(buildBex(), args...)
				command.Env = append(updateTestEnv(home),
					"BEX_HOST="+api.URL+"/v1/",
					"BEX_ACCESS_TOKEN=test-access-token",
					// No bex release hint either: this test is about upstream's.
					"BEX_NO_UPDATE_NOTIFIER=1",
					"CLAUDECODE=1")
				output, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("bex %v: %v\n%s", args, err, output)
				}
				for _, leak := range []string{"Update available", "render-oss/cli", "render.com/docs/cli", "99.0.0", "brew upgrade render"} {
					if strings.Contains(string(output), leak) {
						t.Fatalf("bex %v printed upstream's release notice (%q):\n%s", args, leak, output)
					}
				}
			}

			// Upstream's check did not run: it left the state as it found it.
			got, err := os.ReadFile(cache)
			switch {
			case cached == "" && !os.IsNotExist(err):
				t.Errorf("the release check ran and wrote %s: %s (%v)", cache, got, err)
			case cached != "" && string(got) != cached:
				t.Errorf("the release check ran and rewrote %s:\n%s\nwant it untouched:\n%s", cache, got, cached)
			}
		})
	}
}
