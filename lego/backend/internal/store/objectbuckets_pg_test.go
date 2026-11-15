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

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func TestObjectStorageMigrationRoundTrip(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	read := func(name string) string {
		raw, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	exists := func(rel string) bool {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, rel).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	exec := func(sql string) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}

	if !exists("object_buckets") || !exists("object_storage_usage") || !exists("object_buckets_live_name") {
		t.Fatal("0273 left its tables or the live-name index out")
	}
	exec(read("0273_object_storage.down.sql"))
	if exists("object_buckets") || exists("object_storage_usage") {
		t.Fatal("the down migration left a table behind")
	}
	exec(read("0273_object_storage.up.sql"))
	if !exists("object_buckets") || !exists("object_storage_usage") || !exists("object_buckets_live_name") {
		t.Fatal("the up migration did not recreate the tables after a down")
	}
}

func objectBucketStore(t *testing.T, name string) (*PGStore, string) {
	t.Helper()
	s, pool, tenant := agentSessionPGStore(t, name)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM object_storage_usage WHERE workspace_id=$1`, tenant.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM object_buckets WHERE workspace_id=$1`, tenant.ID)
	})
	return s, tenant.ID
}

func newObjectBucket(t *testing.T, s *PGStore, ws, name string, cap int) ObjectBucket {
	t.Helper()
	b, err := s.CreateObjectBucket(context.Background(), ObjectBucket{WorkspaceID: ws, Name: name, Region: "oregon", LocationHint: "wnam"}, cap)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return b
}

func TestObjectBucketRoundTrip(t *testing.T) {
	s, ws := objectBucketStore(t, "objbkt-rt")
	ctx := context.Background()

	b := newObjectBucket(t, s, ws, "photos", 10)
	if !strings.HasPrefix(b.ID, "obk-") || !ids.WellFormed(b.ID) {
		t.Fatalf("bucket id %q is not an obk- id", b.ID)
	}
	if b.System || b.Public || string(b.CORS) != "[]" || string(b.Lifecycle) != "[]" || b.LocationHint != "wnam" {
		t.Fatalf("new bucket defaults wrong: %+v", b)
	}

	got, err := s.GetObjectBucket(ctx, ws, "photos")
	if err != nil || got.ID != b.ID {
		t.Fatalf("GetObjectBucket = %+v, %v", got, err)
	}
	byID, err := s.GetObjectBucketByID(ctx, b.ID)
	if err != nil || byID.Name != "photos" {
		t.Fatalf("GetObjectBucketByID = %+v, %v", byID, err)
	}
	if _, err := s.GetObjectBucket(ctx, ws, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing bucket error = %v, want ErrNotFound", err)
	}

	newObjectBucket(t, s, ws, "videos", 10)
	list, err := s.ListObjectBuckets(ctx, ws)
	if err != nil || len(list) != 2 || list[0].Name != "photos" || list[1].Name != "videos" {
		t.Fatalf("ListObjectBuckets = %+v, %v", list, err)
	}
}

func TestObjectBucketNameIsUniquePerWorkspace(t *testing.T) {
	s, ws := objectBucketStore(t, "objbkt-dup")
	other, otherWS := objectBucketStore(t, "objbkt-dup-other")
	newObjectBucket(t, s, ws, "photos", 10)

	_, err := s.CreateObjectBucket(context.Background(), ObjectBucket{WorkspaceID: ws, Name: "photos", Region: "oregon"}, 10)
	if !errors.Is(err, ErrObjectBucketExists) || !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate name error = %v, want ErrObjectBucketExists (a conflict)", err)
	}
	// The same name in another workspace is a different bucket.
	newObjectBucket(t, other, otherWS, "photos", 10)
}

func TestObjectBucketRecreatedNameGetsANewID(t *testing.T) {
	s, ws := objectBucketStore(t, "objbkt-recreate")
	ctx := context.Background()
	first := newObjectBucket(t, s, ws, "photos", 10)
	if err := s.DeleteObjectBucket(ctx, ws, first.ID); err != nil {
		t.Fatal(err)
	}
	second := newObjectBucket(t, s, ws, "photos", 10)
	if second.ID == first.ID {
		t.Fatalf("recreated bucket reused id %s: the old objects' prefix would be visible again", first.ID)
	}
	if err := s.DeleteObjectBucket(ctx, ws, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting the old id again = %v, want ErrNotFound", err)
	}
}

func TestObjectBucketRetiredRowDoesNotHoldTheName(t *testing.T) {
	s, ws := objectBucketStore(t, "objbkt-retired")
	ctx := context.Background()
	old := newObjectBucket(t, s, ws, "photos", 10)
	// A workspace delete retires rows and keeps them until the objects are
	// purged (ADR112 D9); the name must be free meanwhile.
	if _, err := s.Pool.Exec(ctx, `UPDATE object_buckets SET deleted_at = now() WHERE id = $1`, old.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetObjectBucket(ctx, ws, "photos"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("retired bucket is still readable: %v", err)
	}
	if _, err := s.GetObjectBucketByID(ctx, old.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("retired bucket still resolves by id: %v", err)
	}
	if list, _ := s.ListObjectBuckets(ctx, ws); len(list) != 0 {
		t.Fatalf("retired bucket is listed: %+v", list)
	}
	if fresh := newObjectBucket(t, s, ws, "photos", 10); fresh.ID == old.ID {
		t.Fatal("a new bucket reused a retired id")
	}
}

func TestObjectBucketCapCountsUserBucketsOnly(t *testing.T) {
	s, ws := objectBucketStore(t, "objbkt-cap")
	ctx := context.Background()
	// With the system bucket already there, two user buckets still fit under a
	// cap of 2, and the third is refused: the system bucket does not count.
	sys, err := s.EnsureSystemObjectBucket(ctx, ws, "render-oregon", "oregon")
	if err != nil || !sys.System {
		t.Fatalf("EnsureSystemObjectBucket = %+v, %v", sys, err)
	}
	newObjectBucket(t, s, ws, "one", 2)
	newObjectBucket(t, s, ws, "two", 2)
	if _, err := s.CreateObjectBucket(ctx, ObjectBucket{WorkspaceID: ws, Name: "three", Region: "oregon"}, 2); !errors.Is(err, ErrObjectBucketQuota) {
		t.Fatalf("third user bucket under cap 2: %v, want ErrObjectBucketQuota", err)
	}
	// The cap does not stop a system bucket being created.
	if _, err := s.EnsureSystemObjectBucket(ctx, ws, "render-frankfurt", "frankfurt"); err != nil {
		t.Fatalf("system bucket refused at the cap: %v", err)
	}
}

func TestEnsureSystemObjectBucketIsIdempotent(t *testing.T) {
	s, ws := objectBucketStore(t, "objbkt-system")
	ctx := context.Background()
	a, err := s.EnsureSystemObjectBucket(ctx, ws, "render-oregon", "oregon")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.EnsureSystemObjectBucket(ctx, ws, "render-oregon", "oregon")
	if err != nil || b.ID != a.ID {
		t.Fatalf("second Ensure = %+v, %v; want the first row %s", b, err, a.ID)
	}
	other, err := s.EnsureSystemObjectBucket(ctx, ws, "render-frankfurt", "frankfurt")
	if err != nil || other.ID == a.ID || other.Region != "frankfurt" {
		t.Fatalf("the other region's system bucket = %+v, %v", other, err)
	}
	// A user bucket that already holds the name is a conflict, not a system bucket.
	newObjectBucket(t, s, ws, "render-ohio", 10)
	if _, err := s.EnsureSystemObjectBucket(ctx, ws, "render-ohio", "oregon"); !errors.Is(err, ErrObjectBucketExists) {
		t.Fatalf("Ensure over a user bucket = %v, want ErrObjectBucketExists", err)
	}
}

func TestObjectBucketSettingsAreScopedToTheWorkspace(t *testing.T) {
	s, ws := objectBucketStore(t, "objbkt-set")
	_, otherWS := objectBucketStore(t, "objbkt-set-other")
	ctx := context.Background()
	b := newObjectBucket(t, s, ws, "photos", 10)

	rules := json.RawMessage(`[{"allowed":{"origins":["*"]}}]`)
	if err := s.SetObjectBucketPublic(ctx, ws, b.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetObjectBucketCORS(ctx, ws, b.ID, rules); err != nil {
		t.Fatal(err)
	}
	if err := s.SetObjectBucketLifecycle(ctx, ws, b.ID, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetObjectBucket(ctx, ws, "photos")
	var gotRules, wantRules any
	_ = json.Unmarshal(got.CORS, &gotRules)
	_ = json.Unmarshal(rules, &wantRules)
	if !got.Public || len(got.CORS) == 0 || string(got.Lifecycle) != "[]" {
		t.Fatalf("settings not stored: %+v", got)
	}
	if a, b := mustJSON(t, gotRules), mustJSON(t, wantRules); a != b {
		t.Fatalf("cors = %s, want %s", a, b)
	}

	if err := s.SetObjectBucketPublic(ctx, otherWS, b.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another workspace changed the bucket: %v, want ErrNotFound", err)
	}
	if err := s.DeleteObjectBucket(ctx, otherWS, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another workspace deleted the bucket: %v, want ErrNotFound", err)
	}
	if still, _ := s.GetObjectBucket(ctx, ws, "photos"); !still.Public {
		t.Fatal("the other workspace's call changed the bucket")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestObjectStorageUsageTotalsSumPerWorkspaceAndRegion(t *testing.T) {
	s, ws := objectBucketStore(t, "objbkt-usage")
	_, otherWS := objectBucketStore(t, "objbkt-usage-other")
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Second)

	if bytes, objs, err := s.ObjectStorageUsageTotals(ctx, ws, "oregon"); err != nil || bytes != 0 || objs != 0 {
		t.Fatalf("unseen workspace = %d bytes, %d objects, %v; want zeros", bytes, objs, err)
	}
	a, b := newObjectBucket(t, s, ws, "bucket-a", 10), newObjectBucket(t, s, ws, "bucket-b", 10)
	for _, u := range []ObjectStorageUsage{
		{BucketID: a.ID, WorkspaceID: ws, Region: "oregon", Bytes: 100, Objects: 2, ObservedAt: at},
		{BucketID: b.ID, WorkspaceID: ws, Region: "oregon", Bytes: 50, Objects: 1, ObservedAt: at},
		{BucketID: "obk-frankfurtbucket", WorkspaceID: ws, Region: "frankfurt", Bytes: 7, Objects: 7, ObservedAt: at},
		{BucketID: "obk-someoneelses", WorkspaceID: otherWS, Region: "oregon", Bytes: 999, Objects: 9, ObservedAt: at},
	} {
		if err := s.UpsertObjectStorageUsage(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	if bytes, objs, _ := s.ObjectStorageUsageTotals(ctx, ws, "oregon"); bytes != 150 || objs != 3 {
		t.Fatalf("oregon totals = %d bytes, %d objects; want 150, 3", bytes, objs)
	}
	// The next sweep replaces a bucket's row rather than adding to it.
	if err := s.UpsertObjectStorageUsage(ctx, ObjectStorageUsage{BucketID: a.ID, WorkspaceID: ws, Region: "oregon", Bytes: 10, Objects: 1, ObservedAt: at}); err != nil {
		t.Fatal(err)
	}
	if bytes, objs, _ := s.ObjectStorageUsageTotals(ctx, ws, "oregon"); bytes != 60 || objs != 2 {
		t.Fatalf("after the re-sweep = %d bytes, %d objects; want 60, 2", bytes, objs)
	}
	if list, err := s.ListObjectStorageUsage(ctx, ws); err != nil || len(list) != 3 {
		t.Fatalf("ListObjectStorageUsage = %d rows, %v; want 3", len(list), err)
	}

	// Deleting a bucket drops its usage row with it.
	if err := s.DeleteObjectBucket(ctx, ws, b.ID); err != nil {
		t.Fatal(err)
	}
	if bytes, _, _ := s.ObjectStorageUsageTotals(ctx, ws, "oregon"); bytes != 10 {
		t.Fatalf("usage of a deleted bucket still counts: %d bytes, want 10", bytes)
	}
}

// The per-workspace lock is what keeps two concurrent callers from both
// passing the cap, or both creating a system bucket.
func TestObjectBucketCreationIsSerializedPerWorkspace(t *testing.T) {
	s, ws := objectBucketStore(t, "objbkt-concurrent")
	ctx := context.Background()

	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.CreateObjectBucket(ctx, ObjectBucket{WorkspaceID: ws, Name: fmt.Sprintf("bucket-%02d", i), Region: "oregon"}, 3)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	created, refused := 0, 0
	for err := range results {
		switch {
		case err == nil:
			created++
		case errors.Is(err, ErrObjectBucketQuota):
			refused++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if created != 3 || refused != 9 {
		t.Fatalf("12 racing creates under a cap of 3 made %d buckets and refused %d; want exactly 3 and 9", created, refused)
	}

	ids := make(chan string, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := s.EnsureSystemObjectBucket(ctx, ws, "render-oregon", "oregon")
			if err != nil {
				t.Error(err)
				return
			}
			ids <- b.ID
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		seen[id] = true
	}
	if len(seen) != 1 {
		t.Fatalf("16 racing EnsureSystemObjectBucket calls produced %d different buckets, want 1", len(seen))
	}
	list, _ := s.ListObjectBuckets(ctx, ws)
	system := 0
	for _, b := range list {
		if b.System {
			system++
		}
	}
	if system != 1 {
		t.Fatalf("%d system buckets exist, want 1", system)
	}
}
