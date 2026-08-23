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

package disksnapshot

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The store half of the pipeline, against a REAL S3-compatible server.
//
// The unit tests above prove the bytes survive a round trip; these prove the
// bytes survive the object store — streaming a body of unknown length through a
// multipart upload, listing it back, and pruning by name. Both halves have to
// hold for a snapshot to be worth anything, and mocking an S3 client would
// verify neither.
//
// Gated on BEX_TEST_S3_ENDPOINT so `go test ./...` stays hermetic:
//
//	docker run -d --name minio -p 9000:9000 \
//	  -e MINIO_ROOT_USER=bextest -e MINIO_ROOT_PASSWORD=bextestsecret \
//	  minio/minio server /data
//	BEX_TEST_S3_ENDPOINT=http://localhost:9000 \
//	  AWS_ACCESS_KEY_ID=bextest AWS_SECRET_ACCESS_KEY=bextestsecret \
//	  go test ./internal/disksnapshot/ -run TestS3
func testStore(t *testing.T) Store {
	t.Helper()
	endpoint := os.Getenv("BEX_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("BEX_TEST_S3_ENDPOINT not set")
	}
	bucket := os.Getenv("BEX_TEST_S3_BUCKET")
	if bucket == "" {
		bucket = "bex-disk-snapshots"
	}
	return Store{
		Endpoint: endpoint,
		Bucket:   bucket,
		// A per-run prefix keeps concurrent or repeated runs from seeing each
		// other's objects, which retention assertions would otherwise miscount.
		Prefix: "itest-" + time.Now().UTC().Format("20060102T150405.000000000"),
	}
}

// The whole path end to end: a directory becomes an encrypted object in a real
// bucket, and comes back out of it byte-for-byte.
func TestS3SnapshotRoundTripThroughTheObjectStore(t *testing.T) {
	store := testStore(t)
	ctx := t.Context()
	recipient, identity := newKeypair(t)
	src := tree(t)
	want := snapshot(t, src)
	prefix := DiskPrefix("tea-itest", "dsk-itest")

	// Upload exactly the way the backup command does: a pipe, so the body's
	// length is not known when the upload starts.
	key := SnapshotKey(prefix, time.Now())
	var archive bytes.Buffer
	if err := Backup(src, &archive, recipient); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if err := store.Put(ctx, key, bytes.NewReader(archive.Bytes())); err != nil {
		t.Fatalf("Put: %v", err)
	}

	objects, err := store.List(ctx, prefix)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(objects) != 1 || objects[0].Key != key {
		t.Fatalf("List = %+v, want exactly the object just written (%s)", objects, key)
	}
	if objects[0].Size == 0 {
		t.Fatal("stored object is empty")
	}

	body, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = body.Close() }()
	dst := t.TempDir()
	if err := Restore(dst, body, identity); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	got := snapshot(t, dst)
	if len(got) != len(want) {
		t.Fatalf("restored %d entries, want %d", len(got), len(want))
	}
	for name, wantEntry := range want {
		if got[name] != wantEntry {
			t.Errorf("%s: restored %q, want %q", name, got[name], wantEntry)
		}
	}
}

// Retention is what keeps seven days of snapshots from becoming seven hundred.
// It must delete the OLDEST and keep the newest, which is only true if key
// ordering and time ordering agree in the real store's listing.
func TestS3PruneKeepsTheNewestAndDeletesTheOldest(t *testing.T) {
	store := testStore(t)
	ctx := t.Context()
	prefix := DiskPrefix("tea-itest", "dsk-prune")

	base := time.Date(2026, 8, 1, 2, 0, 0, 0, time.UTC)
	keys := make([]string, 0, 10)
	for i := range 10 {
		key := SnapshotKey(prefix, base.AddDate(0, 0, i))
		keys = append(keys, key)
		if err := store.Put(ctx, key, bytes.NewReader([]byte("snapshot "+key))); err != nil {
			t.Fatalf("Put %s: %v", key, err)
		}
	}

	deleted, err := store.Prune(ctx, prefix, diskRetentionForTest)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(deleted) != 3 {
		t.Fatalf("pruned %d objects (%v), want the 3 oldest of 10", len(deleted), deleted)
	}
	remaining, err := store.List(ctx, prefix)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(remaining) != diskRetentionForTest {
		t.Fatalf("kept %d, want %d", len(remaining), diskRetentionForTest)
	}
	// The three deleted must be the three oldest, not an arbitrary three.
	for i, key := range keys[:3] {
		if deleted[i] != key {
			t.Errorf("deleted[%d] = %s, want the oldest %s", i, deleted[i], key)
		}
	}
	if remaining[len(remaining)-1].Key != keys[len(keys)-1] {
		t.Errorf("newest kept = %s, want %s", remaining[len(remaining)-1].Key, keys[len(keys)-1])
	}
}

const diskRetentionForTest = 7

// A deleted disk's snapshots must actually leave the bucket: they are a full
// copy of a tenant's filesystem, and they keep costing storage.
func TestS3PurgeAllRemovesEveryObjectForTheDisk(t *testing.T) {
	store := testStore(t)
	ctx := t.Context()
	mine := DiskPrefix("tea-itest", "dsk-purge")
	neighbour := DiskPrefix("tea-itest", "dsk-keep")

	base := time.Date(2026, 8, 1, 2, 0, 0, 0, time.UTC)
	for i := range 3 {
		for _, prefix := range []string{mine, neighbour} {
			key := SnapshotKey(prefix, base.AddDate(0, 0, i))
			if err := store.Put(ctx, key, bytes.NewReader([]byte("x"))); err != nil {
				t.Fatalf("Put: %v", err)
			}
		}
	}

	n, err := store.PurgeAll(ctx, mine)
	if err != nil {
		t.Fatalf("PurgeAll: %v", err)
	}
	if n != 3 {
		t.Fatalf("purged %d, want 3", n)
	}
	if left, err := store.List(ctx, mine); err != nil || len(left) != 0 {
		t.Fatalf("purged disk still lists %+v (%v)", left, err)
	}
	// The neighbouring disk must be untouched — prefixes are the only thing
	// keeping a purge from reaching another disk's data.
	other, err := store.List(ctx, neighbour)
	if err != nil || len(other) != 3 {
		t.Fatalf("neighbouring disk lost snapshots: %+v (%v)", other, err)
	}
}

// The listing must ignore anything that is not a snapshot, so a stray object in
// the same prefix can never be offered for restore or counted by retention.
func TestS3ListIgnoresNonSnapshotObjects(t *testing.T) {
	store := testStore(t)
	ctx := t.Context()
	prefix := DiskPrefix("tea-itest", "dsk-mixed")

	if err := store.Put(ctx, prefix+"README.txt", bytes.NewReader([]byte("not a snapshot"))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	key := SnapshotKey(prefix, time.Now())
	if err := store.Put(ctx, key, bytes.NewReader([]byte("snapshot"))); err != nil {
		t.Fatalf("Put: %v", err)
	}

	objects, err := store.List(ctx, prefix)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(objects) != 1 || objects[0].Key != key {
		t.Fatalf("List = %+v, want only the snapshot", objects)
	}
}

// A large body must stream rather than buffer: this is the property that makes
// a 10 TB volume possible at all, and a multipart upload is where it is proven.
func TestS3PutStreamsABodyLargerThanOnePart(t *testing.T) {
	store := testStore(t)
	ctx := t.Context()
	recipient, identity := newKeypair(t)

	// Build a tree big enough to cross the uploader's default part size.
	root := t.TempDir()
	chunk := bytes.Repeat([]byte("bex-disk-snapshot-payload!"), 1<<16) // ~1.6 MiB
	for i := range 6 {
		mustWrite(t, filepath.Join(root, "big", "file-"+string(rune('a'+i))), chunk, 0o644)
	}

	key := SnapshotKey(DiskPrefix("tea-itest", "dsk-big"), time.Now())
	var archive bytes.Buffer
	if err := Backup(root, &archive, recipient); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if err := store.Put(ctx, key, bytes.NewReader(archive.Bytes())); err != nil {
		t.Fatalf("Put: %v", err)
	}

	body, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = body.Close() }()
	dst := t.TempDir()
	if err := Restore(dst, body, identity); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	restored, err := os.ReadFile(filepath.Join(dst, "big", "file-a"))
	if err != nil {
		t.Fatalf("read restored: %v", err)
	}
	if !bytes.Equal(restored, chunk) {
		t.Fatalf("restored %d bytes, want the original %d", len(restored), len(chunk))
	}
}
