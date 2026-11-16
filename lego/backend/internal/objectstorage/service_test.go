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

package objectstorage_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

var _ objectstorage.Repository = (*store.PGStore)(nil)

// stubRepo answers only what a test sets; any other call panics on the nil
// embedded interface, so a test cannot pass by accident through an unexpected path.
type stubRepo struct {
	objectstorage.Repository
	create        func(store.ObjectBucket, int) (store.ObjectBucket, error)
	bytes, object int64
	usageErr      error
}

func (r *stubRepo) CreateObjectBucket(_ context.Context, in store.ObjectBucket, cap int) (store.ObjectBucket, error) {
	return r.create(in, cap)
}

func (r *stubRepo) ObjectStorageUsageTotals(context.Context, string, string) (int64, int64, error) {
	return r.bytes, r.object, r.usageErr
}

func oregonOnly(t *testing.T) (map[objectstorage.Region]objectstorage.Store, *fakes3.Server) {
	t.Helper()
	fake := fakes3.New(objectstorage.Oregon.Bucket(), objectstorage.Frankfurt.Bucket())
	t.Cleanup(fake.Close)
	store := objectstorage.NewStore(objectstorage.Config{
		Endpoint: fake.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1,
	})
	return map[objectstorage.Region]objectstorage.Store{objectstorage.Oregon: store}, fake
}

func TestDisabledServiceAnswersNotConfiguredEverywhere(t *testing.T) {
	ctx := context.Background()
	for _, s := range []*objectstorage.Service{nil, objectstorage.NewService(nil, nil), objectstorage.NewService(nil, map[objectstorage.Region]objectstorage.Store{})} {
		if s.Enabled() {
			t.Fatal("a service with no stores reports enabled")
		}
		if _, err := s.CreateBucket(ctx, "tea-x", "photos", objectstorage.Oregon, ""); !errors.Is(err, objectstorage.ErrNotConfigured) {
			t.Errorf("CreateBucket = %v", err)
		}
		if _, err := s.SystemBucket(ctx, "tea-x", objectstorage.Oregon); !errors.Is(err, objectstorage.ErrNotConfigured) {
			t.Errorf("SystemBucket = %v", err)
		}
		if _, err := s.GetBucket(ctx, "tea-x", "photos"); !errors.Is(err, objectstorage.ErrNotConfigured) {
			t.Errorf("GetBucket = %v", err)
		}
		if _, err := s.ListBuckets(ctx, "tea-x"); !errors.Is(err, objectstorage.ErrNotConfigured) {
			t.Errorf("ListBuckets = %v", err)
		}
		if err := s.DeleteBucket(ctx, "tea-x", "photos"); !errors.Is(err, objectstorage.ErrNotConfigured) {
			t.Errorf("DeleteBucket = %v", err)
		}
		if err := s.Gate(ctx, "tea-x", objectstorage.Oregon, 1); !errors.Is(err, objectstorage.ErrNotConfigured) {
			t.Errorf("Gate = %v", err)
		}
	}
}

func TestCreateBucketRefusesWhatTheRulesRefuse(t *testing.T) {
	stores, _ := oregonOnly(t)
	called := false
	repo := &stubRepo{create: func(in store.ObjectBucket, _ int) (store.ObjectBucket, error) {
		called = true
		in.ID = "obk-x"
		return in, nil
	}}
	s := objectstorage.NewService(repo, stores)
	ctx := context.Background()

	for name, want := range map[string]error{
		"Photos":        objectstorage.ErrInvalidBucketName,
		"ab":            objectstorage.ErrInvalidBucketName,
		"render-oregon": objectstorage.ErrReservedBucketName,
		"render-x1":     objectstorage.ErrReservedBucketName,
	} {
		if _, err := s.CreateBucket(ctx, "tea-x", name, objectstorage.Oregon, ""); !errors.Is(err, want) {
			t.Errorf("CreateBucket(%q) = %v, want %v", name, err, want)
		}
	}
	// Frankfurt has no store here, so a bucket cannot be created there.
	var notOffered *objectstorage.RegionNotOfferedError
	if _, err := s.CreateBucket(ctx, "tea-x", "photos", objectstorage.Frankfurt, ""); !errors.As(err, &notOffered) {
		t.Errorf("CreateBucket in a region with no store = %v, want a RegionNotOfferedError", err)
	}
	if called {
		t.Fatal("a refused request reached the repository")
	}
	b, err := s.CreateBucket(ctx, "tea-x", "photos", objectstorage.Oregon, "wnam")
	if err != nil || b.Name != "photos" || b.LocationHint != "wnam" || b.Region != "oregon" || !called {
		t.Fatalf("CreateBucket = %+v, %v", b, err)
	}
}

func TestCreateBucketMapsTheStoreErrors(t *testing.T) {
	stores, _ := oregonOnly(t)
	for _, c := range []struct {
		from error
		want error
	}{
		{store.ErrObjectBucketExists, objectstorage.ErrBucketExists},
		{store.ErrObjectBucketQuota, objectstorage.ErrTooManyBuckets},
	} {
		repo := &stubRepo{create: func(store.ObjectBucket, int) (store.ObjectBucket, error) { return store.ObjectBucket{}, c.from }}
		_, err := objectstorage.NewService(repo, stores).CreateBucket(context.Background(), "tea-x", "photos", objectstorage.Oregon, "")
		if !errors.Is(err, c.want) {
			t.Errorf("store error %v became %v, want %v", c.from, err, c.want)
		}
	}
	boom := errors.New("database is down")
	repo := &stubRepo{create: func(store.ObjectBucket, int) (store.ObjectBucket, error) { return store.ObjectBucket{}, boom }}
	if _, err := objectstorage.NewService(repo, stores).CreateBucket(context.Background(), "tea-x", "photos", objectstorage.Oregon, ""); !errors.Is(err, boom) {
		t.Errorf("an unrelated store error became %v", err)
	}
}

func TestCreateBucketPassesTheWorkspaceBucketCap(t *testing.T) {
	stores, _ := oregonOnly(t)
	gotCap := 0
	repo := &stubRepo{create: func(in store.ObjectBucket, cap int) (store.ObjectBucket, error) { gotCap = cap; return in, nil }}
	if _, err := objectstorage.NewService(repo, stores).CreateBucket(context.Background(), "tea-x", "photos", objectstorage.Oregon, ""); err != nil {
		t.Fatal(err)
	}
	if gotCap != objectstorage.MaxUserBuckets {
		t.Fatalf("the repository was given cap %d, want %d", gotCap, objectstorage.MaxUserBuckets)
	}
}

func TestGateRefusesWritesThatPassACap(t *testing.T) {
	stores, _ := oregonOnly(t)
	cases := []struct {
		name          string
		bytes, object int64
		add           int64
		want          string // "" = allowed, else the resource named
	}{
		{"empty workspace", 0, 0, 1 << 30, ""},
		{"exactly at the byte cap", objectstorage.MaxWorkspaceBytes - 10, 5, 10, ""},
		{"one byte over", objectstorage.MaxWorkspaceBytes - 10, 5, 11, "bytes"},
		{"already over", objectstorage.MaxWorkspaceBytes + 1, 5, 0, "bytes"},
		{"last object fits", 0, objectstorage.MaxWorkspaceObjects - 1, 1, ""},
		{"one object too many", 0, objectstorage.MaxWorkspaceObjects, 1, "objects"},
	}
	for _, c := range cases {
		repo := &stubRepo{bytes: c.bytes, object: c.object}
		err := objectstorage.NewService(repo, stores).Gate(context.Background(), "tea-x", objectstorage.Oregon, c.add)
		var q *objectstorage.QuotaError
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: refused: %v", c.name, err)
		case c.want != "" && (!errors.As(err, &q) || string(q.Resource) != c.want || q.Limit == 0):
			t.Errorf("%s: err = %v, want a QuotaError naming %s", c.name, err, c.want)
		}
	}
	// A usage read that fails must not read as "no usage": the write is refused.
	boom := errors.New("db down")
	repo := &stubRepo{usageErr: boom}
	if err := objectstorage.NewService(repo, stores).Gate(context.Background(), "tea-x", objectstorage.Oregon, 1); !errors.Is(err, boom) {
		t.Errorf("a failed usage read = %v, want it surfaced", err)
	}
}

func TestCheckObjectSize(t *testing.T) {
	max := objectstorage.MaxObjectBytesPresigned
	for _, ok := range []int64{0, 1, max} {
		if err := objectstorage.CheckObjectSize(ok, max); err != nil {
			t.Errorf("CheckObjectSize(%d) = %v", ok, err)
		}
	}
	for _, bad := range []int64{-1, max + 1} {
		if err := objectstorage.CheckObjectSize(bad, max); !errors.Is(err, objectstorage.ErrObjectTooLarge) {
			t.Errorf("CheckObjectSize(%d) = %v, want ErrObjectTooLarge", bad, err)
		}
	}
}

// pgEnv is the service over the real store, for what a stub cannot show: the
// unique name, the system bucket, what a delete does to the rows, and real
// presigned URLs against the fake Wasabi.
type pgEnv struct {
	Svc  *objectstorage.Service
	Fake *fakes3.Server
	WS   string
	PG   *store.PGStore
	Pool *pgxpool.Pool
}

// cleanup removes every bucket and usage row of another workspace id a test made up.
func (e *pgEnv) cleanup(t *testing.T, workspaceID string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = e.Pool.Exec(context.Background(), `DELETE FROM object_storage_usage WHERE workspace_id=$1`, workspaceID)
		_, _ = e.Pool.Exec(context.Background(), `DELETE FROM object_buckets WHERE workspace_id=$1`, workspaceID)
	})
}

func newPGEnv(t *testing.T, name string) *pgEnv {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, name+"-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	e := &pgEnv{WS: tenant.ID, PG: pg, Pool: pool}
	e.cleanup(t, tenant.ID)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id=$1`, tenant.ID) })
	var stores map[objectstorage.Region]objectstorage.Store
	stores, e.Fake = oregonOnly(t)
	e.Svc = objectstorage.NewService(pg, stores)
	return e
}

func pgService(t *testing.T, name string) (*objectstorage.Service, *fakes3.Server, string) {
	t.Helper()
	e := newPGEnv(t, name)
	return e.Svc, e.Fake, e.WS
}

func TestSystemBucketIsCreatedOnceAndListed(t *testing.T) {
	s, _, ws := pgService(t, "objsvc-system")
	ctx := context.Background()
	a, err := s.SystemBucket(ctx, ws, objectstorage.Oregon)
	if err != nil || !a.System || a.Name != "render-oregon" || a.Region != "oregon" {
		t.Fatalf("SystemBucket = %+v, %v", a, err)
	}
	b, err := s.SystemBucket(ctx, ws, objectstorage.Oregon)
	if err != nil || b.ID != a.ID {
		t.Fatalf("a second call = %+v, %v; want the same bucket %s", b, err, a.ID)
	}
	var notOffered *objectstorage.RegionNotOfferedError
	if _, err := s.SystemBucket(ctx, ws, objectstorage.Frankfurt); !errors.As(err, &notOffered) {
		t.Fatalf("a region with no store = %v, want a RegionNotOfferedError", err)
	}
	list, err := s.ListBuckets(ctx, ws)
	if err != nil || len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("ListBuckets = %+v, %v", list, err)
	}
	if _, err := s.CreateBucket(ctx, ws, "render-oregon", objectstorage.Oregon, ""); !errors.Is(err, objectstorage.ErrReservedBucketName) {
		t.Fatalf("a user bucket in the reserved namespace = %v", err)
	}
}

func TestCreateBucketRefusesADuplicateNameButNotInAnotherWorkspace(t *testing.T) {
	s, _, ws := pgService(t, "objsvc-dup")
	ctx := context.Background()
	if _, err := s.CreateBucket(ctx, ws, "photos", objectstorage.Oregon, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBucket(ctx, ws, "photos", objectstorage.Oregon, ""); !errors.Is(err, objectstorage.ErrBucketExists) {
		t.Fatalf("a duplicate name = %v, want ErrBucketExists", err)
	}
	// Another workspace's bucket of the same name is not visible here, and the
	// first workspace cannot see a bucket that is not its own.
	if _, err := s.GetBucket(ctx, "tea-someone-else", "photos"); !errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Fatalf("another workspace read the bucket: %v", err)
	}
}

func TestDeleteBucketNeedsAnEmptyBucket(t *testing.T) {
	s, fake, ws := pgService(t, "objsvc-delete")
	ctx := context.Background()
	b, err := s.CreateBucket(ctx, ws, "photos", objectstorage.Oregon, "")
	if err != nil {
		t.Fatal(err)
	}
	key, err := objectstorage.ObjectKey(ws, b.ID, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	fake.Put(objectstorage.Oregon.Bucket(), key, []byte("x"))

	if err := s.DeleteBucket(ctx, ws, "photos"); !errors.Is(err, objectstorage.ErrBucketNotEmpty) {
		t.Fatalf("deleting a bucket with an object = %v, want ErrBucketNotEmpty", err)
	}
	if _, err := s.GetBucket(ctx, ws, "photos"); err != nil {
		t.Fatalf("the refused delete removed the bucket row: %v", err)
	}

	// When Wasabi cannot be asked the bucket is not assumed empty.
	fake.SetDown(true)
	err = s.DeleteBucket(ctx, ws, "photos")
	fake.SetDown(false)
	if err == nil || errors.Is(err, objectstorage.ErrBucketNotEmpty) {
		t.Fatalf("delete while Wasabi is down = %v, want an error that is not a verdict", err)
	}
	if _, err := s.GetBucket(ctx, ws, "photos"); err != nil {
		t.Fatalf("a failed emptiness check removed the bucket row: %v", err)
	}

	st, _ := s.Store(objectstorage.Oregon)
	if err := st.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBucket(ctx, ws, "photos"); err != nil {
		t.Fatalf("deleting the emptied bucket: %v", err)
	}
	if err := s.DeleteBucket(ctx, ws, "photos"); !errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Fatalf("deleting it again = %v, want ErrBucketNotFound", err)
	}
}

func TestARecreatedBucketDoesNotSeeTheOldObjects(t *testing.T) {
	s, fake, ws := pgService(t, "objsvc-recreate")
	ctx := context.Background()
	old, err := s.CreateBucket(ctx, ws, "photos", objectstorage.Oregon, "")
	if err != nil {
		t.Fatal(err)
	}
	oldKey, _ := objectstorage.ObjectKey(ws, old.ID, "secret.txt")
	// An object that outlived its bucket row (a purge that never ran).
	if err := s.DeleteBucket(ctx, ws, "photos"); err != nil {
		t.Fatal(err)
	}
	fake.Put(objectstorage.Oregon.Bucket(), oldKey, []byte("old tenant data"))

	fresh, err := s.CreateBucket(ctx, ws, "photos", objectstorage.Oregon, "")
	if err != nil || fresh.ID == old.ID {
		t.Fatalf("recreated bucket = %+v, %v; want a new id", fresh, err)
	}
	st, _ := s.Store(objectstorage.Oregon)
	page, err := st.List(ctx, objectstorage.Prefix(ws, fresh.ID), "", 10)
	if err != nil || len(page.Objects) != 0 {
		t.Fatalf("the new bucket lists %d objects under its prefix (%v); the old data must be invisible", len(page.Objects), err)
	}
	if err := s.DeleteBucket(ctx, ws, "photos"); err != nil {
		t.Fatalf("the new, empty bucket could not be deleted because of the old data: %v", err)
	}
}

// TestASystemBucketIsNeverDeleted pins w4/678: render-<region> is the Render
// CLI's namespace, so a delete is refused before it lists anything, the
// bucket keeps its id, and a put URL minted before the attempt still uploads
// into a prefix the Render routes list.
func TestASystemBucketIsNeverDeleted(t *testing.T) {
	e := newPGEnv(t, "objsvc-system-delete")
	ctx := context.Background()
	body := []byte("still mine")
	put, err := e.Svc.MintPut(ctx, e.WS, objectstorage.Oregon, "late.txt", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	sys, err := e.Svc.SystemBucket(ctx, e.WS, objectstorage.Oregon)
	if err != nil {
		t.Fatal(err)
	}
	before := len(e.Fake.Requests())
	if err := e.Svc.DeleteBucket(ctx, e.WS, "render-oregon"); !errors.Is(err, objectstorage.ErrSystemBucket) {
		t.Fatalf("deleting the empty render-oregon = %v, want ErrSystemBucket", err)
	}
	if n := len(e.Fake.Requests()); n != before {
		t.Fatalf("the refused delete reached Wasabi %d times", n-before)
	}
	again, err := e.Svc.SystemBucket(ctx, e.WS, objectstorage.Oregon)
	if err != nil || again.ID != sys.ID || !again.CreatedAt.Equal(sys.CreatedAt) {
		t.Fatalf("render-oregon after the refused delete = %+v, %v; want the same bucket %s", again, err, sys.ID)
	}
	upload(t, put.URL, body)
	page, err := e.Svc.ListObjects(ctx, e.WS, objectstorage.Oregon, "", 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].Key != "late.txt" {
		t.Fatalf("ListObjects after the upload = %+v, %v; want late.txt listed", page, err)
	}
}

// TestAStreamRacingADeleteNeverOrphansItsObject pins w4/678 for named buckets:
// a put whose bytes are still moving when the bucket is deleted fails and
// leaves nothing under the gone prefix, and one that finished first keeps the
// bucket.
func TestAStreamRacingADeleteNeverOrphansItsObject(t *testing.T) {
	e := newPGEnv(t, "objsvc-stream-race")
	ctx := context.Background()
	b, err := e.Svc.CreateBucket(ctx, e.WS, "photos", objectstorage.Oregon, "")
	if err != nil {
		t.Fatal(err)
	}
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		_, err := e.Svc.PutStream(ctx, e.WS, b, "big.bin", 4, pr, objectstorage.ObjectHeaders{})
		done <- err
	}()
	// The first bytes are taken, so the put is past its checks and streaming.
	if _, err := pw.Write([]byte("ab")); err != nil {
		t.Fatal(err)
	}
	if err := e.Svc.DeleteBucket(ctx, e.WS, "photos"); err != nil {
		t.Fatalf("deleting the bucket while the stream had stored nothing = %v", err)
	}
	if _, err := pw.Write([]byte("cd")); err != nil {
		t.Fatal(err)
	}
	_ = pw.Close()
	if err := <-done; !errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Fatalf("the put into the deleted bucket = %v, want ErrBucketNotFound", err)
	}
	if keys := e.Fake.Keys(objectstorage.Oregon.Bucket(), objectstorage.Prefix(e.WS, b.ID)); len(keys) != 0 {
		t.Fatalf("the failed put left %v under the deleted bucket's prefix", keys)
	}

	kept, err := e.Svc.CreateBucket(ctx, e.WS, "photos", objectstorage.Oregon, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Svc.PutStream(ctx, e.WS, kept, "a.txt", 1, strings.NewReader("x"), objectstorage.ObjectHeaders{}); err != nil {
		t.Fatal(err)
	}
	if err := e.Svc.DeleteBucket(ctx, e.WS, "photos"); !errors.Is(err, objectstorage.ErrBucketNotEmpty) {
		t.Fatalf("deleting after the put finished = %v, want ErrBucketNotEmpty", err)
	}
	if keys := e.Fake.Keys(objectstorage.Oregon.Bucket(), objectstorage.Prefix(e.WS, kept.ID)); len(keys) != 1 {
		t.Fatalf("the kept bucket holds %v, want its one object", keys)
	}
}
