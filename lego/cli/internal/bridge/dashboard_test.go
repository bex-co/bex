package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/render-oss/cli/pkg/config"
)

// w8/090: a config that never ran device login had no dashboard origin, so
// the Dashboard action opened a hostless /cron/<id> path.
func TestEnsureDashboardURL(t *testing.T) {
	for _, tc := range []struct {
		name     string
		file     *string // nil: no config file at all
		host     string  // RENDER_HOST; empty means the Bex default
		use      bool
		wantURL  string
		wantSame bool // the file is left byte-identical
	}{
		{name: "missing file", use: true, wantURL: DefaultDashboardURL},
		{name: "version-only file", file: new("version: 1\n"), use: true, wantURL: DefaultDashboardURL},
		{name: "unrelated fields kept", file: new("version: 1\nworkspace: tea-x\nworkspace_name: canary\n"), use: true, wantURL: DefaultDashboardURL},
		{name: "saved origin kept", file: new("version: 1\ndashboard_url: https://oauth.bex.co\n"), use: true, wantSame: true},
		{name: "custom API host", file: new("version: 1\n"), host: "https://api.bex.local/v1/", use: true, wantSame: true},
		{name: "custom stored host", file: new("version: 1\napi:\n  host: https://api.bex.local/v1/\n"), use: true, wantSame: true},
		{name: "malformed config", file: new("version: [\n"), use: true, wantSame: true},
		{name: "command without a Dashboard action", file: new("version: 1\n"), use: false, wantSame: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cli.yaml")
			t.Setenv("RENDER_CLI_CONFIG_PATH", path)
			t.Setenv("RENDER_API_KEY", "rnd_secret_bearer")
			host := tc.host
			if host == "" {
				host = DefaultHost
			}
			t.Setenv("RENDER_HOST", host)
			if tc.file != nil {
				if err := os.WriteFile(path, []byte(*tc.file), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			EnsureDashboardURL(tc.use)

			after, err := os.ReadFile(path)
			if tc.wantSame {
				if tc.file == nil && !os.IsNotExist(err) || tc.file != nil && string(after) != *tc.file {
					t.Fatalf("config changed to %q", after)
				}
				return
			}
			if err != nil {
				t.Fatalf("read config: %v", err)
			}
			if got := config.DashboardURL(); got != tc.wantURL {
				t.Fatalf("dashboard origin = %q, want %q", got, tc.wantURL)
			}
			if strings.Contains(string(after), "rnd_secret_bearer") {
				t.Fatal("the environment bearer was persisted")
			}
			if tc.file != nil {
				for _, line := range strings.Split(strings.TrimSpace(*tc.file), "\n") {
					if !strings.Contains(string(after), strings.TrimSpace(line)) {
						t.Fatalf("lost %q: %s", line, after)
					}
				}
			}
		})
	}
}
