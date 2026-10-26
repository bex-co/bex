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
package main

import (
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"

	"github.com/bex-co/bex/lego/operator/internal/disksnapshot"
)

func recipient(t *testing.T) string {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	return id.Recipient().String()
}

// Each way a nightly backup stops is recorded as its own stage, so an owner is
// told what went wrong (w6/m154).
func TestSnapshotNamesTheStageThatFailed(t *testing.T) {
	ctx := t.Context()
	unreachable := disksnapshot.Store{Endpoint: "http://127.0.0.1:1", Bucket: "b"}

	t.Run("no volume mounted is the Job's configuration", func(t *testing.T) {
		t.Setenv("AGE_PUBLIC_KEY", recipient(t))
		t.Setenv("BEX_DISK_MOUNT_PATH", filepath.Join(t.TempDir(), "missing"))
		if stage, err := snapshot(ctx, unreachable, "tea-x/app/"); err == nil || stage != disksnapshot.FailureConfig {
			t.Fatalf("stage=%q err=%v, want config", stage, err)
		}
	})
	t.Run("no encryption key is the Job's configuration", func(t *testing.T) {
		t.Setenv("AGE_PUBLIC_KEY", "")
		t.Setenv("BEX_DISK_MOUNT_PATH", t.TempDir())
		if stage, err := snapshot(ctx, unreachable, "tea-x/app/"); err == nil || stage != disksnapshot.FailureConfig {
			t.Fatalf("stage=%q err=%v, want config", stage, err)
		}
	})
	t.Run("a malformed encryption key is the Job's configuration, not the volume", func(t *testing.T) {
		t.Setenv("AGE_PUBLIC_KEY", "age1notarealkey")
		t.Setenv("BEX_DISK_MOUNT_PATH", t.TempDir())
		if stage, err := snapshot(ctx, unreachable, "tea-x/app/"); err == nil || stage != disksnapshot.FailureConfig {
			t.Fatalf("stage=%q err=%v, want config", stage, err)
		}
	})
	t.Run("an unreachable store is the upload, not the volume", func(t *testing.T) {
		mount := t.TempDir()
		if err := os.WriteFile(filepath.Join(mount, "data.bin"), make([]byte, 1<<20), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("AGE_PUBLIC_KEY", recipient(t))
		t.Setenv("BEX_DISK_MOUNT_PATH", mount)
		t.Setenv("AWS_ACCESS_KEY_ID", "x")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "y")
		if stage, err := snapshot(ctx, unreachable, "tea-x/app/"); err == nil || stage != disksnapshot.FailureUpload {
			t.Fatalf("stage=%q err=%v, want upload", stage, err)
		}
	})
}

// Against a real S3-compatible server (BEX_TEST_S3_ENDPOINT, see
// internal/disksnapshot/store_integration_test.go): an unreadable volume is
// recorded as a read failure, and the next successful backup clears it.
func TestBackupRecordsThenClearsAFailure(t *testing.T) {
	endpoint := os.Getenv("BEX_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("BEX_TEST_S3_ENDPOINT not set")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 directory, so the read failure cannot be staged")
	}
	bucket := os.Getenv("BEX_TEST_S3_BUCKET")
	if bucket == "" {
		bucket = "bex-disk-snapshots"
	}
	mount := t.TempDir()
	locked := filepath.Join(mount, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	t.Setenv("BEX_DISK_SNAPSHOT_ENDPOINT", endpoint)
	t.Setenv("BEX_DISK_SNAPSHOT_BUCKET", bucket)
	t.Setenv("BEX_DISK_SNAPSHOT_PREFIX", "cmdtest-"+filepath.Base(mount))
	t.Setenv("BEX_DISK_WORKSPACE", "tea-cmd")
	t.Setenv("BEX_DISK_ID", "app")
	t.Setenv("AGE_PUBLIC_KEY", recipient(t))
	t.Setenv("BEX_DISK_MOUNT_PATH", mount)
	store, prefix, err := storeFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	recorded := func(stage string) bool {
		body, err := store.Get(t.Context(), prefix+disksnapshot.FailureRecordPrefix+stage)
		if err != nil {
			return false
		}
		_ = body.Close()
		return true
	}

	if err := backup(t.Context()); err == nil {
		t.Fatal("a backup of an unreadable volume succeeded")
	}
	if !recorded(disksnapshot.FailureRead) {
		t.Fatal("the read failure was not recorded")
	}

	if err := os.Chmod(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := backup(t.Context()); err != nil {
		t.Fatalf("backup after the fix: %v", err)
	}
	if recorded(disksnapshot.FailureRead) {
		t.Fatal("a successful backup left the failure record behind")
	}
	if _, err := store.PurgeAll(t.Context(), prefix); err != nil {
		t.Fatal(err)
	}
}
