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

package controller

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Simulated commands are shell functions to avoid process storms when the full
// suite runs on a busy host. One poll exercises the completion predicates;
// production polling and sync implementations are covered by real-engine tests.
const keyValuePersistenceSimulationCommands = `
seq() { printf '1\n'; }
sleep() { :; }
sync() { :; }
timeout() { shift; "$@"; }
`

// These are fault-injection tests of the executable production shell script,
// with simulated engine responses. The Docker suite separately proves real
// Valkey data survival; these cases exercise errors difficult to induce reliably.
func TestKeyValuePersistenceScriptRejectsIncompleteHandoff(t *testing.T) {
	for _, tc := range []struct {
		name, replacement string
		shutdownFailure   bool
	}{
		{name: "journal disabled", replacement: "aof_enabled:0"},
		{name: "rewrite active", replacement: "aof_rewrite_in_progress:1"},
		{name: "rewrite scheduled", replacement: "aof_rewrite_scheduled:1"},
		{name: "no rewrite has completed", replacement: "aof_rewrites:0"},
		{name: "rewrite failed", replacement: "aof_last_bgrewrite_status:err"},
		{name: "journal write failed", replacement: "aof_last_write_status:err"},
		{name: "shutdown failed", shutdownFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin, data := t.TempDir(), t.TempDir()
			write := func(path, contents string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(path, []byte(contents), mode); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(data, ".bex-persistence-mode"), "snapshot\n", 0o600)
			write(filepath.Join(data, "dump.rdb"), "current acknowledged snapshot", 0o600)
			write(filepath.Join(bin, "valkey-server"), "#!/bin/sh\nexit 0\n", 0o755)
			write(filepath.Join(bin, "valkey-cli"), `#!/bin/sh
while [ "$#" -gt 0 ]; do
 case "$1" in
  -s) shift 2 ;;
  -e|--raw) shift ;;
  *) break ;;
 esac
done
printf '%s\n' "$*" >> "$DATA_DIR/commands"
case "$1" in
 PING) echo PONG ;;
 CONFIG|SAVE) echo OK ;;
 INFO) cat "$DATA_DIR/info" ;;
 SHUTDOWN) [ ! -f "$DATA_DIR/shutdown-failure" ] ;;
 *) exit 1 ;;
esac
`, 0o755)
			healthy := "aof_enabled:1\naof_rewrite_in_progress:0\naof_rewrite_scheduled:0\naof_rewrites:1\naof_last_bgrewrite_status:ok\naof_last_write_status:ok\n"
			info := healthy
			if tc.replacement != "" {
				key, _, _ := strings.Cut(tc.replacement, ":")
				for line := range strings.SplitSeq(healthy, "\n") {
					if strings.HasPrefix(line, key+":") {
						info = strings.Replace(info, line, tc.replacement, 1)
					}
				}
			}
			write(filepath.Join(data, "info"), info, 0o600)
			if tc.shutdownFailure {
				write(filepath.Join(data, "shutdown-failure"), "", 0o600)
			}
			run := func(source, target string) error {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "sh", "-ec", keyValuePersistenceSimulationCommands+keyValuePersistenceHandoffScript)
				cmd.WaitDelay = 5 * time.Second
				cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "DATA_DIR=" + data, "SOURCE_MODE=" + source, "TARGET_MODE=" + target}
				out, err := cmd.CombinedOutput()
				if ctx.Err() != nil {
					t.Fatalf("script exceeded test bound: %s", out)
				}
				return err
			}
			read := func(name string) string {
				t.Helper()
				contents, err := os.ReadFile(filepath.Join(data, name))
				if err != nil {
					t.Fatal(err)
				}
				return string(contents)
			}
			if err := run("snapshot", "journal-snapshot"); err == nil {
				t.Fatal("published incomplete journal")
			}
			if got := read(".bex-persistence-mode"); got != "snapshot\n" {
				t.Fatalf("failure changed authoritative format: %q", got)
			}
			if got := read("dump.rdb"); got != "current acknowledged snapshot" {
				t.Fatalf("failure damaged recoverable snapshot: %q", got)
			}

			// A superseding desired generation can revert to the last committed format
			// even if its template bootstrap source has already changed to journal.
			before := read("commands")
			if err := run("journal-snapshot", "snapshot"); err != nil {
				t.Fatalf("reversal failed: %v", err)
			}
			if read("commands") != before {
				t.Fatal("reversal attempted to load uncommitted journal")
			}

			write(filepath.Join(data, "info"), healthy, 0o600)
			if tc.shutdownFailure {
				if err := os.Remove(filepath.Join(data, "shutdown-failure")); err != nil {
					t.Fatal(err)
				}
			}
			if err := run("snapshot", "journal-snapshot"); err != nil {
				t.Fatalf("healthy retry failed: %v", err)
			}
			if got := read(".bex-persistence-mode"); got != "journal-snapshot\n" {
				t.Fatalf("successful retry failed to commit format: %q", got)
			}
		})
	}
}

func TestKeyValuePersistenceUnknownSourceRequiresRecoveryOrExplicitOff(t *testing.T) {
	data := t.TempDir()
	for name, body := range map[string]string{"dump.rdb": "recoverable data", "unrelated": "retain"} {
		if err := os.WriteFile(filepath.Join(data, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(target string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "sh", "-ec", keyValuePersistenceSimulationCommands+keyValuePersistenceHandoffScript)
		cmd.WaitDelay = 5 * time.Second
		cmd.Env = []string{"PATH=/usr/bin:/bin", "DATA_DIR=" + data, "SOURCE_MODE=unknown", "TARGET_MODE=" + target}
		return cmd.Run()
	}
	if err := run("snapshot"); err == nil {
		t.Fatal("unknown source was treated as authoritative snapshot")
	}
	if data, err := os.ReadFile(filepath.Join(data, "dump.rdb")); err != nil || string(data) != "recoverable data" {
		t.Fatal("failed closed startup damaged recovery data")
	}
	if err := run("off"); err != nil {
		t.Fatalf("explicit destructive reset failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(data, "dump.rdb")); !os.IsNotExist(err) {
		t.Fatalf("reset retained old snapshot: %v", err)
	}
	if marker, err := os.ReadFile(filepath.Join(data, ".bex-persistence-mode")); err != nil || string(marker) != "off\n" {
		t.Fatalf("reset marker=%q err=%v", marker, err)
	}
	if data, err := os.ReadFile(filepath.Join(data, "unrelated")); err != nil || string(data) != "retain" {
		t.Fatal("reset touched unrelated volume data")
	}
}
