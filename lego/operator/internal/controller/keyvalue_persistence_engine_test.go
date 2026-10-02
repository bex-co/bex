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
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Run with BEX_TEST_VALKEY=1. These tests use the same pinned 7/8 engine images
// and actual persistent Docker volumes; unit/envtest cannot prove disk format
// conversion or expiration survival. They do not touch a Kubernetes cluster.
func TestValkeyPersistenceEngine(t *testing.T) {
	if os.Getenv("BEX_TEST_VALKEY") != "1" {
		t.Skip("set BEX_TEST_VALKEY=1 to run pinned Valkey Docker durability tests")
	}
	for _, major := range []string{"7", "8"} {
		t.Run(major, func(t *testing.T) {
			t.Run("old_projection_loses_saved_data", func(t *testing.T) {
				e := newPersistenceEngine(t, major)
				c := e.start("journal-snapshot")
				e.set(c, "counter", "1")
				e.stop()
				c = e.start("snapshot")
				e.set(c, "counter", "23")
				e.set(c, "new", "kept")
				if err := c.Save(context.Background()).Err(); err != nil {
					t.Fatal(err)
				}
				e.stop()
				c = e.start("journal-snapshot")
				if got := c.Get(context.Background(), "counter").Val(); got != "1" {
					t.Fatalf("old projection did not reproduce rollback: %q", got)
				}
				if c.Exists(context.Background(), "new").Val() != 0 {
					t.Fatal("old projection unexpectedly kept new key")
				}
			})
			t.Run("live_handoff_and_reverse_survive_restart", func(t *testing.T) {
				e := newPersistenceEngine(t, major)
				e.init("journal-snapshot", "journal-snapshot", "")
				c := e.start("journal-snapshot")
				e.set(c, "baseline", "original")
				e.set(c, "counter", "1")
				if err := c.Set(context.Background(), "expiry", "value", time.Hour).Err(); err != nil {
					t.Fatal(err)
				}
				deadline := c.PExpireTime(context.Background(), "expiry").Val()
				e.stop()
				e.init("journal-snapshot", "snapshot", "")
				c = e.start("snapshot")
				e.set(c, "new", "kept")
				e.set(c, "counter", "23")
				if err := c.Save(context.Background()).Err(); err != nil {
					t.Fatal(err)
				}
				e.stop()
				c = e.start("snapshot") // explicit SAVE + ordinary-restart control
				e.assert(c, "counter", "23")
				e.assert(c, "new", "kept")
				e.set(c, "after-save", "live")
				e.prepare(c)
				// An acknowledged write AFTER completion must be journaled too. Kill
				// without shutdown SAVE: the live gate must not depend on old grace time.
				e.set(c, "after-handoff", "acknowledged")
				if err := c.Do(context.Background(), "WAITAOF", 1, 0, 5000).Err(); err != nil {
					// Valkey 7 lacks WAITAOF; appendfsync always is used for this fixture.
					t.Logf("WAITAOF unavailable: %v", err)
				}
				e.kill()
				e.init("journal-snapshot", "journal-snapshot", "prepared-1")
				c = e.start("journal-snapshot")
				for k, v := range map[string]string{"baseline": "original", "counter": "23", "new": "kept", "after-save": "live", "after-handoff": "acknowledged"} {
					e.assert(c, k, v)
				}
				if got := c.PExpireTime(context.Background(), "expiry").Val(); got != deadline {
					t.Fatalf("expiry moved: %v -> %v", deadline, got)
				}
				e.stop()
				// Consume a reversal; then retry the same old token after new snapshot
				// writes. Reapplying the token would incorrectly reload an older AOF.
				e.init("journal-snapshot", "snapshot", "prepared-1")
				c = e.start("snapshot")
				e.set(c, "counter", "41")
				e.stop()
				e.init("journal-snapshot", "snapshot", "prepared-1")
				c = e.start("snapshot")
				e.assert(c, "counter", "41")
				e.stop()
				// Resume a suspended store: no serving process exists for a live gate.
				e.init("journal-snapshot", "journal-snapshot", "prepared-1")
				c = e.start("journal-snapshot")
				e.assert(c, "counter", "41")
			})
			t.Run("all_mode_pairs", func(t *testing.T) {
				for _, from := range []string{"journal-snapshot", "snapshot", "off"} {
					for _, to := range []string{"journal-snapshot", "snapshot", "off"} {
						t.Run(from+"_to_"+to, func(t *testing.T) {
							e := newPersistenceEngine(t, major)
							e.init(from, from, "")
							c := e.start(from)
							e.set(c, "marker", "present")
							e.stop()
							e.init(from, to, "")
							c = e.start(to)
							if from == "off" || to == "off" {
								if c.Exists(context.Background(), "marker").Val() != 0 {
									t.Fatal("Off transition resurrected data")
								}
							} else {
								e.assert(c, "marker", "present")
							}
						})
					}
				}
			})

			t.Run("interrupted_handoffs_and_off_reversal", func(t *testing.T) {
				for _, tc := range []struct{ name, source, target, boundary string }{
					{"journal_after_save", "journal-snapshot", "snapshot", "cli SHUTDOWN NOSAVE"},
					{"snapshot_after_enable", "snapshot", "journal-snapshot", "complete=no"},
					{"off_before_deletion", "snapshot", "off", "# These are the exact managed Valkey defaults"},
				} {
					t.Run(tc.name, func(t *testing.T) {
						e := newPersistenceEngine(t, major)
						e.init(tc.source, tc.source, "")
						c := e.start(tc.source)
						e.set(c, "authoritative", "yes")
						e.stop()
						interrupted := strings.Replace(keyValuePersistenceScript, tc.boundary, "exit 42\n"+tc.boundary, 1)
						if out, err := e.initScriptResult(tc.source, tc.target, "", interrupted); err == nil {
							t.Fatalf("interruption not reached: %s", out)
						}
						// Reverse the interrupted Off request. Its persisted discard intent
						// must complete before durable mode can start, rather than resurrect.
						target := tc.target
						if target == "off" {
							target = "journal-snapshot"
						}
						e.init(tc.source, target, "")
						c = e.start(target)
						if tc.target == "off" {
							if c.Exists(context.Background(), "authoritative").Val() != 0 {
								t.Fatal("interrupted Off resurrected old data")
							}
						} else {
							e.assert(c, "authoritative", "yes")
						}
					})
				}
			})
			t.Run("new_suspended_mode_reversal", func(t *testing.T) {
				for _, pair := range [][2]string{{"snapshot", "journal-snapshot"}, {"journal-snapshot", "snapshot"}} {
					t.Run(pair[0]+"_to_"+pair[1], func(t *testing.T) {
						e := newPersistenceEngine(t, major)
						e.init(pair[0], pair[1], "")
						c := e.start(pair[1])
						e.set(c, "first-start", "retained")
						e.stop()
						e.init(pair[0], pair[1], "")
						c = e.start(pair[1])
						e.assert(c, "first-start", "retained")
					})
				}
			})
			t.Run("initialized_journal_invalid_manifest_fails", func(t *testing.T) {
				for name, body := range map[string]string{"comments-only": "# empty journal manifest\n", "history-only": "file obsolete.aof seq 1 type h\n", "empty": ""} {
					t.Run(name, func(t *testing.T) {
						e := newPersistenceEngine(t, major)
						e.init("journal-snapshot", "journal-snapshot", "")
						c := e.start("journal-snapshot")
						e.set(c, "recoverable", "yes")
						e.stop()
						e.docker("run", "--rm", "-v", e.volume+":/data", "-e", "MANIFEST="+body, "--entrypoint", "sh", e.image, "-ec", `cp /data/appendonlydir/appendonly.aof.manifest /data/original.manifest; printf '%s' "$MANIFEST" > /data/appendonlydir/appendonly.aof.manifest`)
						checksums := func() string {
							return e.docker("run", "--rm", "-v", e.volume+":/data", "--entrypoint", "sh", e.image, "-ec", "sha256sum /data/dump.rdb /data/.bex-persistence-mode /data/appendonlydir/*")
						}
						before := checksums()
						if out, err := e.initResult("journal-snapshot", "journal-snapshot", ""); err == nil {
							t.Fatalf("invalid journal accepted: %s", out)
						}
						assertPersistenceFilesUnchanged(t, before, checksums())
						// Valkey parses an existing manifest even with AOF disabled.
						// Repair the deliberately corrupted fixture, then prove the
						// preserved journal still recovers the acknowledged write.
						e.docker("run", "--rm", "-v", e.volume+":/data", "--entrypoint", "sh", e.image, "-ec", "mv /data/original.manifest /data/appendonlydir/appendonly.aof.manifest")
						c = e.start("journal-snapshot")
						e.assert(c, "recoverable", "yes")
					})
				}
			})
			t.Run("initialized_journal_missing_marker_fails", func(t *testing.T) {
				e := newPersistenceEngine(t, major)
				e.init("journal-snapshot", "journal-snapshot", "")
				c := e.start("journal-snapshot")
				e.set(c, "retained", "yes")
				e.stop()
				e.docker("run", "--rm", "-v", e.volume+":/data", "--entrypoint", "sh", e.image, "-ec", "rm /data/.bex-persistence-mode")
				if out, err := e.initScriptResult("new:journal-snapshot", "journal-snapshot", "", keyValuePersistenceScript); err == nil {
					t.Fatalf("lost marker guessed new volume: %s", out)
				}
				c = e.start("journal-snapshot")
				e.assert(c, "retained", "yes")
			})
			t.Run("initialized_snapshot_missing_same_mode_fails", func(t *testing.T) {
				e := newPersistenceEngine(t, major)
				e.init("snapshot", "snapshot", "")
				e.docker("run", "--rm", "-v", e.volume+":/data", "--entrypoint", "sh", e.image, "-ec", "rm /data/dump.rdb")
				if out, err := e.initResult("snapshot", "snapshot", ""); err == nil {
					t.Fatalf("missing initialized snapshot accepted: %s", out)
				}
			})
			t.Run("missing_manifest_and_invalid_marker_fail_closed", func(t *testing.T) {
				e := newPersistenceEngine(t, major)
				e.init("journal-snapshot", "journal-snapshot", "")
				c := e.start("journal-snapshot")
				e.set(c, "recoverable", "yes")
				e.stop()
				e.docker("run", "--rm", "-v", e.volume+":/data", "--entrypoint", "sh", e.image, "-ec", "rm /data/appendonlydir/appendonly.aof.manifest")
				if out, err := e.initResult("journal-snapshot", "journal-snapshot", ""); err == nil {
					t.Fatalf("missing initialized journal accepted: %s", out)
				}
				if out, err := e.initResult("journal-snapshot", "snapshot", ""); err == nil {
					t.Fatalf("missing journal accepted: %s", out)
				}
				e.docker("run", "--rm", "-v", e.volume+":/data", "--entrypoint", "sh", e.image, "-ec", "printf 'unknown\\n' > /data/.bex-persistence-mode")
				if out, err := e.initResult("journal-snapshot", "snapshot", ""); err == nil {
					t.Fatalf("invalid marker accepted: %s", out)
				}
				// Even both failures must leave the recoverable snapshot untouched.
				c = e.start("snapshot")
				e.assert(c, "recoverable", "yes")
			})
			t.Run("failed_rewrite_retains_snapshot_and_retries", func(t *testing.T) {
				e := newPersistenceEngine(t, major)
				e.init("snapshot", "snapshot", "")
				c := e.start("snapshot")
				e.set(c, "current", "snapshot-data")
				e.stop()
				// A file where AOF needs its directory reliably fails conversion.
				e.docker("run", "--rm", "-v", e.volume+":/data", "--entrypoint", "sh", e.image, "-ec", "touch /data/appendonlydir")
				if out, err := e.initResult("snapshot", "journal-snapshot", ""); err == nil {
					t.Fatalf("conversion unexpectedly succeeded: %s", out)
				}
				c = e.start("snapshot")
				e.assert(c, "current", "snapshot-data")
				e.stop()
				e.docker("run", "--rm", "-v", e.volume+":/data", "--entrypoint", "sh", e.image, "-ec", "rm /data/appendonlydir")
				e.init("snapshot", "journal-snapshot", "")
				c = e.start("journal-snapshot")
				e.assert(c, "current", "snapshot-data")
			})
		})
	}
}

type persistenceEngine struct {
	t                        *testing.T
	image, volume, container string
	client                   *redis.Client
	bootstrapped             bool
}

func newPersistenceEngine(t *testing.T, major string) *persistenceEngine {
	e := &persistenceEngine{t: t, image: valkeyImage(major)}
	e.volume = strings.TrimSpace(e.docker("volume", "create"))
	t.Cleanup(func() {
		if e.client != nil {
			_ = e.client.Close()
		}
		if e.container != "" {
			e.docker("rm", "-f", e.container)
		}
		e.docker("volume", "rm", e.volume)
	})
	// Simulate the production PVC fsGroup before running the non-root init.
	e.docker("run", "--rm", "-v", e.volume+":/data", "--entrypoint", "sh", e.image, "-ec", "chown 0:1000 /data; chmod 2770 /data")
	return e
}
func (e *persistenceEngine) docker(args ...string) string {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		e.t.Fatalf("docker %s: %v\n%s", strings.Join(args[:min(len(args), 2)], " "), err, out)
	}
	return string(out)
}
func (e *persistenceEngine) start(mode string) *redis.Client {
	e.t.Helper()
	args := []string{"run", "-d", "-p", "127.0.0.1::6379", "-v", e.volume + ":/data", e.image, "valkey-server", "--requirepass", "fixture-password", "--appendfsync", "always"}
	switch mode {
	case "off":
		args = append(args, "--appendonly", "no", "--save", "", "--dir", "/tmp")
	case "snapshot":
		args = append(args, "--appendonly", "no")
	default:
		args = append(args, "--appendonly", "yes")
	}
	output := strings.Split(strings.TrimSpace(e.docker(args...)), "\n")
	e.container = output[len(output)-1]
	portCtx, cancelPort := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelPort()
	portOutput, err := exec.CommandContext(portCtx, "docker", "port", e.container, "6379/tcp").CombinedOutput()
	if err != nil {
		e.t.Fatalf("Valkey port unavailable: %v: %s\n%s", err, portOutput, e.docker("logs", e.container))
	}
	address := strings.TrimSpace(string(portOutput))
	c := redis.NewClient(&redis.Options{Addr: address, Password: "fixture-password", Protocol: 2, MaxRetries: -1})
	e.client = c
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if c.Ping(context.Background()).Err() == nil {
			return c
		}
		time.Sleep(50 * time.Millisecond)
	}
	e.t.Fatalf("Valkey did not start: %s", e.docker("logs", e.container))
	return nil
}
func (e *persistenceEngine) stop() {
	e.t.Helper()
	if e.client != nil {
		_ = e.client.Close()
		e.client = nil
	}
	e.docker("stop", "-t", "30", e.container)
	e.docker("rm", e.container)
	e.container = ""
}
func (e *persistenceEngine) kill() {
	e.t.Helper()
	_ = e.client.Close()
	e.client = nil
	e.docker("kill", e.container)
	e.docker("rm", e.container)
	e.container = ""
}
func (e *persistenceEngine) initResult(source, target, token string) (string, error) {
	if !e.bootstrapped {
		source = "new:" + source
		e.bootstrapped = true
	}
	return e.initScriptResult(source, target, token, keyValuePersistenceScript)
}
func (e *persistenceEngine) initScriptResult(source, target, token, script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cidFile := filepath.Join(e.t.TempDir(), "init.cid")
	args := []string{"run", "--rm", "--cidfile", cidFile, "--user", "999:1000", "-v", e.volume + ":/data", "-e", "VALKEY_PASSWORD=fixture-password", "-e", "PERSISTENCE_SOURCE=" + source, "-e", "PERSISTENCE_TARGET=" + target, "-e", "PERSISTENCE_TOKEN=" + token, "--entrypoint", "sh", e.image, "-ec", script}
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	// Killing a timed-out docker CLI does not stop its container. Remove the
	// exact captured container before the volume cleanup, even on failure.
	if cid, readErr := os.ReadFile(cidFile); readErr == nil {
		cleanup, cancelCleanup := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelCleanup()
		_ = exec.CommandContext(cleanup, "docker", "rm", "-f", strings.TrimSpace(string(cid))).Run()
	}
	return string(out), err
}
func (e *persistenceEngine) init(source, target, token string) {
	e.t.Helper()
	if out, err := e.initResult(source, target, token); err != nil {
		e.t.Fatalf("init %s -> %s: %v\n%s", source, target, err, out)
	}
}
func (e *persistenceEngine) set(c *redis.Client, k, v string) {
	e.t.Helper()
	if err := c.Set(context.Background(), k, v, 0).Err(); err != nil {
		e.t.Fatal(err)
	}
}
func (e *persistenceEngine) assert(c *redis.Client, k, v string) {
	e.t.Helper()
	got, err := c.Get(context.Background(), k).Result()
	if err != nil || got != v {
		e.t.Fatalf("%s = %q (%v), want %q", k, got, err, v)
	}
}
func (e *persistenceEngine) prepare(c *redis.Client) {
	e.t.Helper()
	for range 100 {
		ready, err := prepareValkeyJournal(context.Background(), c)
		if err != nil {
			e.t.Fatal(err)
		}
		if ready {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	e.t.Fatal(fmt.Errorf("live journal handoff timed out"))
}

// assertPersistenceFilesUnchanged proves a refused handoff leaves the original
// marker and durable files intact for diagnosis and explicit recovery.
func assertPersistenceFilesUnchanged(t *testing.T, before, after string) {
	t.Helper()
	if after != before {
		t.Fatalf("refused migration modified authoritative files: before %s; after %s", before, after)
	}
}
