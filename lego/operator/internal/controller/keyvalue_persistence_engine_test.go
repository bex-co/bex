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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Opt in with BEX_TEST_VALKEY_DOCKER=1 go test ./internal/controller -run TestKeyValuePersistenceEngine.
// Or set BEX_TEST_VALKEY_BIN_7/_8 to built engine bin directories (timeout must
// be on PATH). Native runs exercise the same files and process restarts; Docker
// additionally exercises the pinned image and production uid. Do not run native
// handoffs concurrently: the production init script uses a pod-local /tmp socket.
// Each Docker case owns a volume and exercises the production pinned engine.
// Container replacement models a StatefulSet pod replacement against its PVC.
func TestKeyValuePersistenceEngine(t *testing.T) {
	if os.Getenv("BEX_TEST_VALKEY_DOCKER") != "1" && os.Getenv("BEX_TEST_VALKEY_BIN_7") == "" && os.Getenv("BEX_TEST_VALKEY_BIN_8") == "" {
		t.Skip("set BEX_TEST_VALKEY_DOCKER=1 or BEX_TEST_VALKEY_BIN_7/_8 to test real Valkey persistence")
	}
	for _, major := range []string{"7", "8"} {
		t.Run(major, func(t *testing.T) {
			t.Run("old_projection_replays_stale_journal", func(t *testing.T) {
				engine := newPersistenceEngine(t, major)
				deadline := engine.seedStaleJournal()
				engine.start("journal-snapshot")
				engine.want("baseline", "original")
				engine.want("counter", "1")
				engine.want("snapshot-marker", "")
				engine.wantDeadline(deadline)
				t.Log("old startup projection reproduced acknowledged, SAVE-persisted marker loss and counter rollback 23 -> 1")
			})
			t.Run("handoff_preserves_current_snapshot", func(t *testing.T) {
				engine := newPersistenceEngine(t, major)
				deadline := engine.seedStaleJournal()
				engine.handoff("snapshot", "journal-snapshot")
				engine.start("journal-snapshot")
				engine.wantCurrent(deadline)
				engine.command("SET", "post-handoff", "newer-than-snapshot")
				engine.command("INCR", "counter")
				// Kill without taking another RDB: retrying conversion from the old RDB
				// would lose these acknowledged journal-only writes.
				engine.command("CONFIG", "SET", "appendfsync", "always")
				engine.command("SET", "fsync-barrier", "yes")
				engine.stop(false)
				engine.handoff("snapshot", "journal-snapshot")
				engine.start("journal-snapshot")
				engine.want("counter", "24")
				engine.want("post-handoff", "newer-than-snapshot")
				engine.want("snapshot-marker", "saved-current")
				engine.wantDeadline(deadline)
				engine.stop(true)
				engine.start("journal-snapshot")
				engine.want("counter", "24")
				engine.want("post-handoff", "newer-than-snapshot")
			})
			t.Run("graceful_snapshot_shutdown_saves_acknowledged_writes", func(t *testing.T) {
				engine := newPersistenceEngine(t, major)
				engine.seedStaleJournal()
				engine.start("snapshot")
				engine.command("SET", "unsaved", "acknowledged-before-shutdown")
				engine.command("INCR", "counter")
				engine.stop(true)
				engine.handoff("snapshot", "journal-snapshot")
				engine.start("journal-snapshot")
				engine.want("unsaved", "acknowledged-before-shutdown")
				engine.want("counter", "24")
			})
			t.Run("failed_journal_enable_preserves_snapshot_and_retries", func(t *testing.T) {
				engine := newPersistenceEngine(t, major)
				deadline := engine.seedStaleJournal()
				engine.files("mv appendonlydir preserved-old-journal; touch appendonlydir; chown 999:1000 appendonlydir")
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				out, err := engine.handoffCommand(ctx, "snapshot", "journal-snapshot").CombinedOutput()
				if err == nil {
					t.Fatalf("handoff unexpectedly succeeded with blocked journal path: %s", out)
				}
				if ctx.Err() != nil {
					t.Fatalf("handoff did not reject failed journal enable promptly: %s", out)
				}
				engine.start("snapshot")
				engine.wantCurrent(deadline)
				engine.stop(true)
				engine.files("rm appendonlydir")
				engine.handoff("snapshot", "journal-snapshot")
				engine.start("journal-snapshot")
				engine.wantCurrent(deadline)
			})
			t.Run("interrupted_off_retirement_then_reversal_never_resurrects", func(t *testing.T) {
				engine := newPersistenceEngine(t, major)
				engine.seedStaleJournal()
				// Model termination after committing Off intent and retiring only
				// the snapshot. The remaining old AOF must not become authority.
				engine.files("printf 'off\\n' > .bex-persistence-mode; chown 999:1000 .bex-persistence-mode; mkdir retired-before-kill; mv dump.rdb retired-before-kill/")
				engine.handoff("snapshot", "journal-snapshot")
				engine.start("journal-snapshot")
				engine.want("baseline", "")
				engine.want("counter", "")
				engine.want("snapshot-marker", "")
			})
			t.Run("all_mode_pairs", func(t *testing.T) {
				for _, source := range []string{"journal-snapshot", "snapshot", "off"} {
					for _, target := range []string{"journal-snapshot", "snapshot", "off"} {
						t.Run(source+"_to_"+target, func(t *testing.T) {
							engine := newPersistenceEngine(t, major)
							// Seed historical durable files before entering the source
							// mode: Off must never resurrect that earlier keyspace.
							engine.start("journal-snapshot")
							engine.command("SET", "historical", "must-not-resurrect")
							engine.command("SAVE")
							engine.stop(true)
							engine.handoff("journal-snapshot", source)
							engine.start(source)
							engine.command("SET", "current", "acknowledged")
							engine.stop(true)
							engine.handoff(source, target)
							engine.start(target)
							expected := "acknowledged"
							if source == "off" || target == "off" {
								expected = ""
								engine.want("historical", "")
							}
							engine.want("current", expected)
						})
					}
				}
			})
			t.Run("invalid_initialized_journal_fails_closed", func(t *testing.T) {
				for _, damage := range []string{"missing", "corrupt", "comment-only", "history-only"} {
					t.Run(damage, func(t *testing.T) {
						engine := newPersistenceEngine(t, major)
						engine.start("journal-snapshot")
						engine.command("SET", "baseline", "original")
						engine.command("SAVE")
						engine.command("SET", "journal-only", "newer-than-snapshot")
						engine.stop(false)
						engine.handoff("journal-snapshot", "journal-snapshot")
						engine.files("cp appendonlydir/appendonly.aof.manifest preserved.manifest")
						switch damage {
						case "missing":
							engine.files("rm appendonlydir/appendonly.aof.manifest")
						case "corrupt":
							engine.files("printf 'invalid manifest\\n' > appendonlydir/appendonly.aof.manifest")
						case "comment-only":
							engine.files("printf '# incomplete manifest\\n' > appendonlydir/appendonly.aof.manifest")
						case "history-only":
							engine.files("printf 'file appendonly.aof.1.base.rdb seq 1 type h\\n' > appendonlydir/appendonly.aof.manifest")
						}
						checksums := engine.fileOutput("find appendonlydir -type f -exec cksum {} \\; | sort")
						ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
						defer cancel()
						out, err := engine.handoffCommand(ctx, "journal-snapshot", "snapshot").CombinedOutput()
						if err == nil {
							t.Fatalf("handoff accepted %s initialized journal: %s", damage, out)
						}
						if ctx.Err() != nil {
							t.Fatalf("handoff did not fail within its own startup bound: %s", out)
						}
						if got := engine.fileOutput("find appendonlydir -type f -exec cksum {} \\; | sort"); got != checksums {
							t.Fatalf("failed handoff modified recoverable journal files: before %s after %s", checksums, got)
						}
						if got := engine.fileOutput("cat .bex-persistence-mode"); got != "journal-snapshot" {
							t.Fatalf("failed handoff committed mode %q", got)
						}
						engine.files("cp preserved.manifest appendonlydir/appendonly.aof.manifest")
						engine.handoff("journal-snapshot", "snapshot")
						engine.start("snapshot")
						engine.want("baseline", "original")
						engine.want("journal-only", "newer-than-snapshot")
					})
				}
			})
			t.Run("unknown_retained_storage_requires_explicit_off_reset", func(t *testing.T) {
				engine := newPersistenceEngine(t, major)
				deadline := engine.seedStaleJournal()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if out, err := engine.handoffCommand(ctx, "unknown", "journal-snapshot").CombinedOutput(); err == nil {
					t.Fatalf("unknown retained storage was adopted: %s", out)
				}
				engine.start("snapshot")
				engine.wantCurrent(deadline)
				engine.stop(true)
				engine.handoff("unknown", "off")
				engine.start("off")
				engine.want("baseline", "")
				engine.want("snapshot-marker", "")
			})
			t.Run("first_boot_reversal_after_initialization", func(t *testing.T) {
				for _, source := range []string{"journal-snapshot", "snapshot", "off"} {
					for _, target := range []string{"journal-snapshot", "snapshot", "off"} {
						t.Run(source+"_to_"+target, func(t *testing.T) {
							engine := newPersistenceEngine(t, major)
							// The first init must establish real files before declaring a
							// durable mode, even if desired mode reverses before app start.
							engine.handoff("new:"+source, source)
							engine.handoff("new:"+source, target)
							engine.start(target)
							engine.command("SET", "first-write", "acknowledged")
							engine.stop(true)
							engine.handoff("new:"+source, target)
							engine.start(target)
							expected := "acknowledged"
							if target == "off" {
								expected = ""
							}
							engine.want("first-write", expected)
						})
					}
				}
			})
			t.Run("snapshot_without_prior_journal", func(t *testing.T) {
				engine := newPersistenceEngine(t, major)
				engine.start("snapshot")
				engine.command("SET", "fresh", "never-journaled")
				engine.command("SAVE")
				engine.stop(true)
				engine.handoff("snapshot", "journal-snapshot")
				engine.start("journal-snapshot")
				engine.want("fresh", "never-journaled")
				engine.stop(false)
				engine.start("journal-snapshot")
				engine.want("fresh", "never-journaled")
			})
		})
	}
}

type persistenceEngine struct {
	t                        *testing.T
	image, volume, container string
	bin, data                string
	process                  *exec.Cmd
}

func newPersistenceEngine(t *testing.T, major string) *persistenceEngine {
	t.Helper()
	e := &persistenceEngine{t: t, image: valkeyImage(major), volume: fmt.Sprintf("bex-kv-persistence-%d", time.Now().UnixNano())}
	e.bin = os.Getenv("BEX_TEST_VALKEY_BIN_" + major)
	if e.bin != "" {
		var err error
		e.data, err = os.MkdirTemp("/tmp", "bex-kv-native-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if e.process != nil {
				e.stop(false)
			}
			if err := os.RemoveAll(e.data); err != nil {
				t.Error(err)
			}
		})
		return e
	}
	if os.Getenv("BEX_TEST_VALKEY_DOCKER") != "1" {
		t.Skip("no native engine directory for major " + major)
	}
	e.docker("volume", "create", e.volume)
	t.Cleanup(func() { e.docker("volume", "rm", e.volume) })
	t.Cleanup(func() {
		if e.container != "" {
			e.docker("rm", "-f", e.container)
		}
	})
	// Match the writable PVC ownership established by the pod fsGroup before init.
	e.docker("run", "--rm", "-v", e.volume+":/data", e.image, "chown", "999:1000", "/data")
	return e
}

func (e *persistenceEngine) docker(args ...string) string {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		e.t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (e *persistenceEngine) start(mode string) {
	e.t.Helper()
	appendonly := "no"
	if mode == "journal-snapshot" {
		appendonly = "yes"
	}
	if e.bin != "" {
		args := []string{"--dir", e.data, "--port", "0", "--unixsocket", filepath.Join(e.data, "server.sock"), "--requirepass", "test-secret", "--appendonly", appendonly, "--appendfsync", "always", "--logfile", filepath.Join(e.data, "server.log")}
		if mode == "off" {
			args = append(args, "--save", "")
		}
		e.process = exec.Command(filepath.Join(e.bin, "valkey-server"), args...)
		if err := e.process.Start(); err != nil {
			e.process = nil
			e.t.Fatal(err)
		}
	} else {
		e.container = e.volume + "-server"
		args := []string{"run", "--name", e.container, "-d", "-v", e.volume + ":/data", e.image, "valkey-server", "--dir", "/data", "--requirepass", "test-secret", "--appendonly", appendonly, "--appendfsync", "always"}
		if mode == "off" {
			args = append(args, "--save", "")
		}
		e.docker(args...)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		out, err := e.clientCommand(ctx, "PING").CombinedOutput()
		cancel()
		if err == nil && strings.TrimSpace(string(out)) == "PONG" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	if e.bin != "" {
		contents, _ := os.ReadFile(filepath.Join(e.data, "server.log"))
		e.t.Fatalf("engine failed to become ready: %s", contents)
	}
	e.t.Fatalf("engine failed to become ready: %s", e.docker("logs", e.container))
}

func (e *persistenceEngine) clientCommand(ctx context.Context, args ...string) *exec.Cmd {
	if e.bin != "" {
		cmd := exec.CommandContext(ctx, filepath.Join(e.bin, "valkey-cli"), append([]string{"-s", filepath.Join(e.data, "server.sock"), "-e", "--raw"}, args...)...)
		cmd.Env = append(os.Environ(), "REDISCLI_AUTH=test-secret")
		return cmd
	}
	return exec.CommandContext(ctx, "docker", append([]string{"exec", "-e", "REDISCLI_AUTH=test-secret", e.container, "valkey-cli", "-e", "--raw"}, args...)...)
}

func (e *persistenceEngine) command(args ...string) string {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	out, err := e.clientCommand(ctx, args...).CombinedOutput()
	if err != nil {
		e.t.Fatalf("valkey command %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (e *persistenceEngine) stop(graceful bool) {
	e.t.Helper()
	if e.bin != "" {
		if graceful {
			if err := e.process.Process.Signal(syscall.SIGTERM); err != nil {
				e.t.Error(err)
			}
		} else {
			_ = e.process.Process.Kill()
		}
		exited := make(chan error, 1)
		go func() { exited <- e.process.Wait() }()
		select {
		case err := <-exited:
			e.process = nil
			if graceful && err != nil {
				e.t.Fatalf("graceful Valkey shutdown: %v", err)
			}
		case <-time.After(20 * time.Second):
			_ = e.process.Process.Kill()
			<-exited
			e.process = nil
			e.t.Fatal("Valkey shutdown exceeded 20 seconds")
		}
		return
	}
	if graceful {
		e.docker("stop", "-t", "20", e.container)
	} else {
		e.docker("kill", e.container)
	}
	e.docker("rm", e.container)
	e.container = ""
}

func (e *persistenceEngine) handoff(source, target string) {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	out, err := e.handoffCommand(ctx, source, target).CombinedOutput()
	if err != nil {
		e.t.Fatalf("handoff %s -> %s: %v: %s", source, target, err, out)
	}
}

func (e *persistenceEngine) handoffCommand(ctx context.Context, source, target string) *exec.Cmd {
	if e.bin != "" {
		cmd := exec.CommandContext(ctx, "sh", "-ec", keyValuePersistenceHandoffScript)
		cmd.Env = append(os.Environ(), "PATH="+e.bin+string(os.PathListSeparator)+os.Getenv("PATH"), "SOURCE_MODE="+source, "TARGET_MODE="+target, "DATA_DIR="+e.data)
		return cmd
	}
	return exec.CommandContext(ctx, "docker", "run", "--rm", "--user", "999:1000", "-v", e.volume+":/data", "-e", "SOURCE_MODE="+source, "-e", "TARGET_MODE="+target, "-e", "DATA_DIR=/data", e.image, "sh", "-ec", keyValuePersistenceHandoffScript)
}

func (e *persistenceEngine) files(script string) {
	e.t.Helper()
	e.fileOutput(script)
}

func (e *persistenceEngine) fileOutput(script string) string {
	e.t.Helper()
	if e.bin != "" {
		// Native runs use the caller's uid; Docker exercises the production uid999.
		script = strings.ReplaceAll(script, "; chown 999:1000 appendonlydir", "")
		script = strings.ReplaceAll(script, "; chown 999:1000 .bex-persistence-mode", "")
		cmd := exec.Command("sh", "-ec", script)
		cmd.Dir = e.data
		out, err := cmd.CombinedOutput()
		if err != nil {
			e.t.Fatalf("fixture files: %v: %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	return e.docker("run", "--rm", "-v", e.volume+":/data", e.image, "sh", "-ec", "cd /data; "+script)
}

func (e *persistenceEngine) want(key, value string) {
	e.t.Helper()
	if got := e.command("GET", key); got != value {
		e.t.Fatalf("GET %s = %q, want %q", key, got, value)
	}
}

func (e *persistenceEngine) wantDeadline(deadline string) {
	e.t.Helper()
	if got := e.command("PEXPIRETIME", "expiring"); got != deadline {
		e.t.Fatalf("expiry deadline = %s, want %s (must never reset on restart)", got, deadline)
	}
}

func (e *persistenceEngine) wantCurrent(deadline string) {
	e.t.Helper()
	e.want("baseline", "original")
	e.want("counter", "23")
	e.want("snapshot-marker", "saved-current")
	e.wantDeadline(deadline)
	if got := e.command("PTTL", "snapshot-marker"); got != "-1" {
		e.t.Fatalf("non-expiring key PTTL = %s", got)
	}
}

func (e *persistenceEngine) seedStaleJournal() string {
	e.t.Helper()
	e.start("journal-snapshot")
	e.command("SET", "baseline", "original")
	e.command("SET", "counter", "1")
	deadline := strconv.FormatInt(time.Now().Add(60*time.Minute).UnixMilli(), 10)
	e.command("SET", "expiring", "ttl-value", "PXAT", deadline)
	e.command("SAVE")
	e.stop(true)
	e.start("snapshot")
	e.want("baseline", "original")
	e.want("counter", "1")
	e.wantDeadline(deadline)
	e.command("SET", "snapshot-marker", "saved-current")
	e.command("INCR", "counter")
	e.want("counter", "2")
	e.command("SET", "counter", "23")
	e.command("SAVE")
	e.stop(true)
	// The positive control rules out ordinary snapshot restart loss.
	e.start("snapshot")
	e.command("CONFIG", "SET", "maxmemory-policy", "noeviction")
	e.wantCurrent(deadline)
	e.stop(true)
	return deadline
}
