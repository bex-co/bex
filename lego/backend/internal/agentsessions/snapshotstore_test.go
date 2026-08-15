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

package agentsessions

import (
	"strings"
	"testing"
	"time"
)

// Missing any required coordinate ⇒ nil store ⇒ hibernation stays off (the safe
// default). Only a fully configured set constructs a store.
func TestNewS3SnapshotStoreGatesOnConfig(t *testing.T) {
	full := S3SnapshotConfig{Endpoint: "https://s3.example", Bucket: "snaps", AccessKey: "ak", SecretKey: "sk"}
	if NewS3SnapshotStore(full) == nil {
		t.Fatal("fully configured store should not be nil")
	}
	for name, cfg := range map[string]S3SnapshotConfig{
		"no bucket":   {Endpoint: "https://s3.example", AccessKey: "ak", SecretKey: "sk"},
		"no endpoint": {Bucket: "snaps", AccessKey: "ak", SecretKey: "sk"},
		"no access":   {Endpoint: "https://s3.example", Bucket: "snaps", SecretKey: "sk"},
		"no secret":   {Endpoint: "https://s3.example", Bucket: "snaps", AccessKey: "ak"},
		"empty":       {},
	} {
		if NewS3SnapshotStore(cfg) != nil {
			t.Errorf("%s: store should be nil (hibernation disabled)", name)
		}
	}
}

// The object key is per-workspace-prefixed (never a flat/registry namespace) and
// unique per mint, so a re-hibernation never overwrites a still-referenced blob.
func TestS3SnapshotKeyIsPerWorkspacePrefixedAndUnique(t *testing.T) {
	s := NewS3SnapshotStore(S3SnapshotConfig{Endpoint: "https://s3.example", Bucket: "snaps", AccessKey: "ak", SecretKey: "sk", Prefix: "agent-snapshots"})
	n := int64(0)
	s.nowFn = func() time.Time { n++; return time.Unix(1_800_000_000+n, 0) }

	k1 := s.snapshotKey("tea-abc", "ags-1")
	k2 := s.snapshotKey("tea-abc", "ags-1")
	if !strings.HasPrefix(k1, "agent-snapshots/tea-abc/ags-1-") || !strings.HasSuffix(k1, ".tgz") {
		t.Fatalf("key %q not per-workspace-prefixed .tgz", k1)
	}
	if k1 == k2 {
		t.Fatalf("keys must be unique per mint, got %q twice", k1)
	}
}

// Delete on a nil store / empty ref is a safe no-op (retention of a never-stored
// snapshot must not panic or error).
func TestS3SnapshotDeleteNilSafe(t *testing.T) {
	var s *S3SnapshotStore
	if err := s.Delete(nil, "x"); err != nil {
		t.Fatalf("nil-store delete = %v, want nil", err)
	}
	store := NewS3SnapshotStore(S3SnapshotConfig{Endpoint: "https://s3.example", Bucket: "snaps", AccessKey: "ak", SecretKey: "sk"})
	if err := store.Delete(nil, ""); err != nil {
		t.Fatalf("empty-ref delete = %v, want nil", err)
	}
}
