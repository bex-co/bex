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
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

// All-empty ⇒ (nil, nil) (tier off). Full set ⇒ store. Partial set or the
// platform state bucket ⇒ error so startup cannot silently disable hibernation.
func TestNewS3SnapshotStoreGatesOnConfig(t *testing.T) {
	full := S3SnapshotConfig{Endpoint: "https://s3.example", Bucket: "snaps", AccessKey: "ak", SecretKey: "sk"}
	store, err := NewS3SnapshotStore(full)
	if err != nil || store == nil {
		t.Fatalf("fully configured store = (%v, %v), want store", store, err)
	}

	empty, err := NewS3SnapshotStore(S3SnapshotConfig{})
	if err != nil || empty != nil {
		t.Fatalf("empty config = (%v, %v), want (nil, nil)", empty, err)
	}

	for name, cfg := range map[string]S3SnapshotConfig{
		"no bucket":   {Endpoint: "https://s3.example", AccessKey: "ak", SecretKey: "sk"},
		"no endpoint": {Bucket: "snaps", AccessKey: "ak", SecretKey: "sk"},
		"no access":   {Endpoint: "https://s3.example", Bucket: "snaps", SecretKey: "sk"},
		"no secret":   {Endpoint: "https://s3.example", Bucket: "snaps", AccessKey: "ak"},
		"whitespace":  {Endpoint: " ", Bucket: "snaps", AccessKey: "ak", SecretKey: "sk"},
	} {
		got, err := NewS3SnapshotStore(cfg)
		if got != nil || err == nil || !errors.Is(err, ErrPartialS3SnapshotConfig) {
			t.Errorf("%s: got (%v, %v), want (nil, ErrPartialS3SnapshotConfig)", name, got, err)
		}
	}

	got, err := NewS3SnapshotStore(S3SnapshotConfig{
		Endpoint: "https://s3.example", Bucket: "bex-tfstate", AccessKey: "ak", SecretKey: "sk",
	})
	if got != nil || !errors.Is(err, ErrForbiddenSnapshotBucket) {
		t.Fatalf("tfstate bucket = (%v, %v), want ErrForbiddenSnapshotBucket", got, err)
	}
}

// The object key is per-workspace-prefixed (never a flat/registry namespace) and
// unique per mint, so a re-hibernation never overwrites a still-referenced blob.
func TestS3SnapshotKeyIsPerWorkspacePrefixedAndUnique(t *testing.T) {
	s := mustSnapshotStore(t, S3SnapshotConfig{Endpoint: "https://s3.example", Bucket: "snaps", AccessKey: "ak", SecretKey: "sk", Prefix: "agent-snapshots"})
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

// PrepareUpload signs If-None-Match:* so a retained argv URL cannot overwrite
// a completed snapshot (round-16 #9). Signing is local — no network required.
func TestPrepareUploadIsCreateOnce(t *testing.T) {
	s := mustSnapshotStore(t, S3SnapshotConfig{Endpoint: "https://s3.example.test", Bucket: "snaps", AccessKey: "ak", SecretKey: "sk"})
	s.nowFn = func() time.Time { return time.Unix(1_800_000_000, 0) }
	_, putURL, err := s.PrepareUpload(context.Background(), "tea-abc", "ags-1")
	if err != nil {
		t.Fatalf("PrepareUpload: %v", err)
	}
	u, err := url.Parse(putURL)
	if err != nil {
		t.Fatalf("parse put URL: %v", err)
	}
	signed := strings.ToLower(u.Query().Get("X-Amz-SignedHeaders"))
	if !strings.Contains(signed, "if-none-match") {
		t.Fatalf("presigned PUT SignedHeaders = %q, want if-none-match (create-once)", signed)
	}
}

// Delete on a nil store / empty ref is a safe no-op (retention of a never-stored
// snapshot must not panic or error).
func TestS3SnapshotDeleteNilSafe(t *testing.T) {
	var s *S3SnapshotStore
	if err := s.Delete(nil, "x"); err != nil {
		t.Fatalf("nil-store delete = %v, want nil", err)
	}
	store := mustSnapshotStore(t, S3SnapshotConfig{Endpoint: "https://s3.example", Bucket: "snaps", AccessKey: "ak", SecretKey: "sk"})
	if err := store.Delete(nil, ""); err != nil {
		t.Fatalf("empty-ref delete = %v, want nil", err)
	}
}

func mustSnapshotStore(t *testing.T, cfg S3SnapshotConfig) *S3SnapshotStore {
	t.Helper()
	s, err := NewS3SnapshotStore(cfg)
	if err != nil || s == nil {
		t.Fatalf("NewS3SnapshotStore = (%v, %v)", s, err)
	}
	return s
}
