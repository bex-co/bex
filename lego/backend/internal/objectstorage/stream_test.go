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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// streamRepo is the registry the streamed contract needs, in memory.
type streamRepo struct {
	objectstorage.Repository
	buckets       map[string]store.ObjectBucket
	bytes, object int64
	setErr        error
	cors, life    json.RawMessage
}

func (r *streamRepo) EnsureSystemObjectBucket(_ context.Context, ws, name, region string) (store.ObjectBucket, error) {
	key := ws + "/" + name
	if b, ok := r.buckets[key]; ok {
		return b, nil
	}
	b := store.ObjectBucket{ID: ids.New(ids.ObjectBucket), WorkspaceID: ws, Name: name, Region: region, System: true}
	r.buckets[key] = b
	return b, nil
}

func (r *streamRepo) GetObjectBucket(_ context.Context, ws, name string) (store.ObjectBucket, error) {
	if b, ok := r.buckets[ws+"/"+name]; ok {
		return b, nil
	}
	return store.ObjectBucket{}, store.ErrNotFound
}

func (r *streamRepo) ObjectStorageUsageTotals(context.Context, string, string) (int64, int64, error) {
	return r.bytes, r.object, nil
}

func (r *streamRepo) SetObjectBucketCORS(_ context.Context, _, _ string, rules json.RawMessage) error {
	r.cors = rules
	return r.setErr
}

func (r *streamRepo) SetObjectBucketLifecycle(_ context.Context, _, _ string, rules json.RawMessage) error {
	r.life = rules
	return r.setErr
}

type streamEnv struct {
	Svc  *objectstorage.Service
	Repo *streamRepo
	Fake *fakes3.Server
	WS   string
	B    store.ObjectBucket
}

func newStreamEnv(t *testing.T) *streamEnv {
	t.Helper()
	stores, fake := oregonOnly(t)
	repo := &streamRepo{buckets: map[string]store.ObjectBucket{}}
	ws := ids.New(ids.Workspace)
	b := store.ObjectBucket{ID: ids.New(ids.ObjectBucket), WorkspaceID: ws, Name: "photos", Region: "oregon"}
	repo.buckets[ws+"/photos"] = b
	return &streamEnv{Svc: objectstorage.NewService(repo, stores), Repo: repo, Fake: fake, WS: ws, B: b}
}

func (e *streamEnv) wasabiKey(key string) string { return e.WS + "/" + e.B.ID + "/" + key }

func (e *streamEnv) puts() int {
	n := 0
	for _, r := range e.Fake.Requests() {
		if r.Method == http.MethodPut && r.Key != "" {
			n++
		}
	}
	return n
}

func TestPutStreamStoresTheBytesAndHeadersWithTheRealLength(t *testing.T) {
	e := newStreamEnv(t)
	ctx := context.Background()
	for _, size := range []int{0, 1, 5 << 20} {
		body := bytes.Repeat([]byte{'q'}, size)
		etag, err := e.Svc.PutStream(ctx, e.WS, e.B, "dir/o", int64(size), bytes.NewReader(body), objectstorage.ObjectHeaders{
			ContentType: "text/plain", CacheControl: "no-cache", ContentDisposition: "attachment; filename=o", ContentEncoding: "gzip",
			ContentLanguage: "en", Expires: "Wed, 21 Oct 2037 07:28:00 GMT",
		})
		if err != nil || etag == "" {
			t.Fatalf("PutStream %d bytes = %q, %v", size, etag, err)
		}
		got, ok := e.Fake.Get(objectstorage.Oregon.Bucket(), e.wasabiKey("dir/o"))
		if !ok || !bytes.Equal(got, body) {
			t.Fatalf("%d bytes: stored %d bytes, present %v", size, len(got), ok)
		}
	}
	// Every PUT carried a Content-Length, even the empty one, so the fake that
	// refuses a chunked body (as Wasabi does) took all three.
	if e.puts() != 3 {
		t.Fatalf("Wasabi took %d PUTs, want 3", e.puts())
	}
	for _, r := range e.Fake.Requests() {
		if r.Method == http.MethodPut && r.Header.Get("Content-Length") == "" && r.Key != "" {
			t.Errorf("PUT %s had no Content-Length", r.Key)
		}
	}

	// A reader gets the headers back as stored, and gzip bytes untouched.
	obj, err := e.Svc.OpenObject(ctx, e.WS, e.B, "dir/o")
	if err != nil {
		t.Fatal(err)
	}
	defer obj.Close()
	want := objectstorage.ObjectHeaders{
		ContentType: "text/plain", CacheControl: "no-cache", ContentDisposition: "attachment; filename=o", ContentEncoding: "gzip",
		ContentLanguage: "en", Expires: "Wed, 21 Oct 2037 07:28:00 GMT",
	}
	if obj.Info.Headers != want || obj.Info.Size != 5<<20 || obj.Info.Key != "dir/o" {
		t.Fatalf("object info = %+v, want headers %+v and 5 MiB", obj.Info, want)
	}
	if n, _ := io.Copy(io.Discard, obj); n != 5<<20 {
		t.Fatalf("read %d bytes of a 5 MiB object (Go's transport must not decompress a stored gzip)", n)
	}
}

func TestPutStreamRefusesBeforeItReadsAByte(t *testing.T) {
	e := newStreamEnv(t)
	ctx := context.Background()
	never := readerFunc(func([]byte) (int, error) { t.Error("a refused put read its body"); return 0, io.EOF })
	put := func(key string, size int64) error {
		_, err := e.Svc.PutStream(ctx, e.WS, e.B, key, size, never, objectstorage.ObjectHeaders{})
		return err
	}
	if err := put("a/../b", 1); !errors.Is(err, objectstorage.ErrInvalidKey) {
		t.Errorf("a .. key = %v", err)
	}
	if err := put("k", -1); !errors.Is(err, objectstorage.ErrMissingContentLength) {
		t.Errorf("an unknown length = %v", err)
	}
	if err := put("k", objectstorage.MaxObjectBytesStreamed+1); !errors.Is(err, objectstorage.ErrObjectTooLarge) {
		t.Errorf("300 MiB + 1 = %v", err)
	}
	e.Repo.bytes = objectstorage.MaxWorkspaceBytes
	var quota *objectstorage.QuotaError
	if err := put("k", 1); !errors.As(err, &quota) || quota.Resource != objectstorage.QuotaBytes {
		t.Errorf("a byte quota breach = %v", err)
	}
	e.Repo.bytes = 0
	// Every slot of the workspace is taken.
	var releases []func()
	for range objectstorage.MaxConcurrentStreamsPerWorkspace {
		release, err := e.Svc.AcquireStream(e.WS)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	if err := put("k", 1); !errors.Is(err, objectstorage.ErrTooManyStreams) {
		t.Errorf("the ninth stream = %v", err)
	}
	for _, r := range releases {
		r()
	}
	if e.puts() != 0 {
		t.Fatalf("%d PUTs reached Wasabi from refused writes", e.puts())
	}
	// Disabled and exactly at the cap are the two edges.
	if _, err := objectstorage.NewService(nil, nil).PutStream(ctx, e.WS, e.B, "k", 1, never, objectstorage.ObjectHeaders{}); !errors.Is(err, objectstorage.ErrNotConfigured) {
		t.Errorf("a disabled service = %v", err)
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func TestPutStreamShortBodyStoresNothingAndFreesItsSlot(t *testing.T) {
	e := newStreamEnv(t)
	_, err := e.Svc.PutStream(context.Background(), e.WS, e.B, "short", 10, strings.NewReader("12345"), objectstorage.ObjectHeaders{})
	if !errors.Is(err, objectstorage.ErrIncompleteBody) {
		t.Fatalf("a body of 5 against 10 = %v, want ErrIncompleteBody", err)
	}
	if _, ok := e.Fake.Get(objectstorage.Oregon.Bucket(), e.wasabiKey("short")); ok {
		t.Fatal("a short body left an object behind")
	}
	for range objectstorage.MaxConcurrentStreamsPerWorkspace { // all slots are free again
		release, err := e.Svc.AcquireStream(e.WS)
		if err != nil {
			t.Fatalf("a failed put kept its stream slot: %v", err)
		}
		defer release()
	}
}

// TestPutStreamErrorsNeverCarryTheSignedURL: the URL's query is a signature
// good for 15 minutes, and a failure is logged by the adapter.
func TestPutStreamErrorsNeverCarryTheSignedURL(t *testing.T) {
	e := newStreamEnv(t)
	ctx := context.Background()
	e.Fake.SetDown(true)
	_, err := e.Svc.PutStream(ctx, e.WS, e.B, "k", 1, strings.NewReader("x"), objectstorage.ObjectHeaders{})
	if err == nil || strings.Contains(err.Error(), "X-Amz-Signature") {
		t.Fatalf("an answered failure = %v", err)
	}
	e.Fake.SetDown(false)
	e.Fake.Close() // nothing listens: net/http's error would quote the whole URL
	_, err = e.Svc.PutStream(ctx, e.WS, e.B, "k", 1, strings.NewReader("x"), objectstorage.ObjectHeaders{})
	if err == nil || strings.Contains(err.Error(), "X-Amz-Signature") || strings.Contains(err.Error(), "X-Amz-Credential") {
		t.Fatalf("a connection failure = %v, which quotes the presigned URL", err)
	}
	// And it is Wasabi's failure, not the caller's short body: the caller sent
	// all of it (or would have, had Wasabi been there to take it).
	if errors.Is(err, objectstorage.ErrIncompleteBody) {
		t.Fatalf("an unreachable Wasabi is reported as a short body: %v", err)
	}
}

func TestOpenObjectGivesItsSlotBackOnCloseAndOnAMiss(t *testing.T) {
	e := newStreamEnv(t)
	ctx := context.Background()
	if _, err := e.Svc.PutStream(ctx, e.WS, e.B, "k", 3, strings.NewReader("abc"), objectstorage.ObjectHeaders{}); err != nil {
		t.Fatal(err)
	}
	free := func() int {
		n := 0
		var releases []func()
		for {
			release, err := e.Svc.AcquireStream(e.WS)
			if err != nil {
				break
			}
			releases = append(releases, release)
			n++
		}
		for _, r := range releases {
			r()
		}
		return n
	}
	obj, err := e.Svc.OpenObject(ctx, e.WS, e.B, "k")
	if err != nil {
		t.Fatal(err)
	}
	if free() != objectstorage.MaxConcurrentStreamsPerWorkspace-1 {
		t.Fatalf("an open object holds %d slots", objectstorage.MaxConcurrentStreamsPerWorkspace-free())
	}
	_ = obj.Close()
	_ = obj.Close() // closing twice returns the slot once
	if free() != objectstorage.MaxConcurrentStreamsPerWorkspace {
		t.Fatalf("a closed object kept its slot")
	}
	if _, err := e.Svc.OpenObject(ctx, e.WS, e.B, "ghost"); !errors.Is(err, objectstorage.ErrObjectNotFound) {
		t.Fatalf("a missing key = %v", err)
	}
	if free() != objectstorage.MaxConcurrentStreamsPerWorkspace {
		t.Fatalf("a missed open kept its slot")
	}
	// Another workspace's bucket of the same id string cannot be reached.
	other := store.ObjectBucket{ID: e.B.ID, WorkspaceID: ids.New(ids.Workspace), Region: "oregon"}
	if _, err := e.Svc.OpenObject(ctx, other.WorkspaceID, other, "k"); !errors.Is(err, objectstorage.ErrObjectNotFound) {
		t.Fatalf("the same bucket id in another workspace reads %v", err)
	}
}

func TestRemoveObjectIsIdempotent(t *testing.T) {
	e := newStreamEnv(t)
	ctx := context.Background()
	if _, err := e.Svc.PutStream(ctx, e.WS, e.B, "k", 1, strings.NewReader("x"), objectstorage.ObjectHeaders{}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := e.Svc.RemoveObject(ctx, e.WS, e.B, "k"); err != nil {
			t.Fatalf("RemoveObject = %v; a missing key is success, as S3 answers", err)
		}
	}
	if _, ok := e.Fake.Get(objectstorage.Oregon.Bucket(), e.wasabiKey("k")); ok {
		t.Fatal("the object is still there")
	}
	if err := e.Svc.RemoveObject(ctx, e.WS, e.B, "a/./b"); !errors.Is(err, objectstorage.ErrInvalidKey) {
		t.Fatalf("a . key = %v", err)
	}
}

func TestResolveBucketCreatesSystemBucketsAndNothingElse(t *testing.T) {
	e := newStreamEnv(t)
	ctx := context.Background()
	user, err := e.Svc.ResolveBucket(ctx, e.WS, "photos")
	if err != nil || user.ID != e.B.ID {
		t.Fatalf("a user bucket = %+v, %v", user, err)
	}
	sys, err := e.Svc.ResolveBucket(ctx, e.WS, "render-oregon")
	if err != nil || !sys.System || sys.Region != "oregon" {
		t.Fatalf("render-oregon = %+v, %v; want it created on first use", sys, err)
	}
	if again, _ := e.Svc.ResolveBucket(ctx, e.WS, "render-oregon"); again.ID != sys.ID {
		t.Fatal("the system bucket was created twice")
	}
	for _, name := range []string{"render-elsewhere", "render-", "ghost"} {
		if _, err := e.Svc.ResolveBucket(ctx, e.WS, name); !errors.Is(err, objectstorage.ErrBucketNotFound) {
			t.Errorf("%q = %v, want ErrBucketNotFound", name, err)
		}
	}
}

func TestSetRulesValidateStoreAndMapAMissingBucket(t *testing.T) {
	e := newStreamEnv(t)
	ctx := context.Background()
	if err := e.Svc.SetCORS(ctx, e.WS, e.B, nil); err != nil || string(e.Repo.cors) != "[]" {
		t.Fatalf("SetCORS(nil) = %v, stored %s; no rules must be stored as [], not null", err, e.Repo.cors)
	}
	maxAge := int64(60)
	cors := []objectstorage.CORSRule{{Allowed: objectstorage.CORSAllowed{Origins: []string{"*"}, Methods: []string{"GET"}}, MaxAgeSeconds: &maxAge}}
	if err := e.Svc.SetCORS(ctx, e.WS, e.B, cors); err != nil || !strings.Contains(string(e.Repo.cors), `"maxAgeSeconds":60`) {
		t.Fatalf("SetCORS = %v, stored %s", err, e.Repo.cors)
	}
	life := []objectstorage.LifecycleRule{{ID: "a", Enabled: true, DeleteObjectsTransition: &objectstorage.LifecycleDelete{
		Condition: objectstorage.LifecycleDeleteCondition{Type: objectstorage.LifecycleAge, MaxAge: &maxAge}}}}
	if err := e.Svc.SetLifecycle(ctx, e.WS, e.B, life); err != nil || !strings.Contains(string(e.Repo.life), `"id":"a"`) {
		t.Fatalf("SetLifecycle = %v, stored %s", err, e.Repo.life)
	}

	// A set that fails validation is stored nowhere, whoever calls.
	e.Repo.cors, e.Repo.life = json.RawMessage("untouched"), json.RawMessage("untouched")
	if err := e.Svc.SetCORS(ctx, e.WS, e.B, []objectstorage.CORSRule{{}}); !errors.Is(err, objectstorage.ErrInvalidCORS) || string(e.Repo.cors) != "untouched" {
		t.Errorf("an invalid CORS rule = %v, stored %s", err, e.Repo.cors)
	}
	if err := e.Svc.SetLifecycle(ctx, e.WS, e.B, []objectstorage.LifecycleRule{{ID: "x"}}); !errors.Is(err, objectstorage.ErrInvalidLifecycle) || string(e.Repo.life) != "untouched" {
		t.Errorf("an invalid lifecycle rule = %v, stored %s", err, e.Repo.life)
	}

	e.Repo.setErr = store.ErrNotFound
	if err := e.Svc.SetCORS(ctx, e.WS, e.B, nil); !errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Fatalf("a bucket gone from the registry = %v", err)
	}
}

func TestValidateLifecycleRefusesWhatTheSweepWouldNotRun(t *testing.T) {
	age := func(id string, seconds int64) objectstorage.LifecycleRule {
		return objectstorage.LifecycleRule{ID: id, Enabled: true, DeleteObjectsTransition: &objectstorage.LifecycleDelete{
			Condition: objectstorage.LifecycleDeleteCondition{Type: objectstorage.LifecycleAge, MaxAge: &seconds}}}
	}
	if err := objectstorage.ValidateLifecycle([]objectstorage.LifecycleRule{age("a", 1), age("b", 0)}); err != nil {
		t.Fatalf("valid rules: %v", err)
	}
	for name, rules := range map[string][]objectstorage.LifecycleRule{
		"duplicate id": {age("a", 1), age("a", 2)},
		"empty id":     {age("", 1)},
		"no action":    {{ID: "a", Enabled: true}},
		"huge age":     {age("a", objectstorage.MaxLifecycleAgeSeconds+1)},
		"control char": {func() objectstorage.LifecycleRule { r := age("a", 1); r.Conditions.Prefix = "a\x01"; return r }()},
	} {
		if err := objectstorage.ValidateLifecycle(rules); !errors.Is(err, objectstorage.ErrInvalidLifecycle) {
			t.Errorf("%s: %v, want ErrInvalidLifecycle", name, err)
		}
	}
	ia := age("a", 1)
	ia.StorageClassTransitions = json.RawMessage(`[{"condition":{"type":"Age","maxAge":1},"storageClass":"InfrequentAccess"}]`)
	if err := objectstorage.ValidateLifecycle([]objectstorage.LifecycleRule{ia}); !errors.Is(err, objectstorage.ErrLifecycleStorageClass) {
		t.Errorf("a storage class transition = %v", err)
	}
	many := make([]objectstorage.LifecycleRule, objectstorage.MaxLifecycleRules+1)
	for i := range many {
		many[i] = age(fmt.Sprintf("r%d", i), 1)
	}
	if err := objectstorage.ValidateLifecycle(many[:objectstorage.MaxLifecycleRules]); err != nil {
		t.Errorf("exactly the cap: %v", err)
	}
	if err := objectstorage.ValidateLifecycle(many); !errors.Is(err, objectstorage.ErrLifecycleOverCap) {
		t.Errorf("one over the cap = %v", err)
	}
}

// sumSink adds up what the transfer meters flush.
type sumSink struct {
	mu     sync.Mutex
	totals map[string]int64
}

func (u *sumSink) AddObjectStorageUsage(_ context.Context, deltas []store.ObjectStorageUsageDelta) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.totals == nil {
		u.totals = map[string]int64{}
	}
	for _, d := range deltas {
		u.totals[d.Kind] += d.Quantity
	}
	return nil
}

// TestClosingAnObjectTwiceMetersItsBytesOnce: the HTTP layer closes a body on
// every path, so a second Close must not count the egress again.
func TestClosingAnObjectTwiceMetersItsBytesOnce(t *testing.T) {
	e := newStreamEnv(t)
	sink := &sumSink{}
	e.Svc.SetUsageSink(sink)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.Svc.RunMeter(ctx, time.Hour); close(done) }()

	if _, err := e.Svc.PutStream(context.Background(), e.WS, e.B, "k", 6, strings.NewReader("abcdef"), objectstorage.ObjectHeaders{}); err != nil {
		t.Fatal(err)
	}
	obj, err := e.Svc.OpenObject(context.Background(), e.WS, e.B, "k")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, obj); err != nil {
		t.Fatal(err)
	}
	_ = obj.Close()
	_ = obj.Close()
	cancel()
	<-done
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if got := sink.totals[store.UsageKindObjectStorageEgressBytes]; got != 6 {
		t.Errorf("egress = %d, want 6 counted once", got)
	}
	if got := sink.totals[store.UsageKindObjectStorageIngressBytes]; got != 6 {
		t.Errorf("ingress = %d, want 6", got)
	}
}

// TestAStalledWasabiDoesNotHoldAStreamSlot: a Wasabi that accepts a connection
// and never answers is cut at the stream deadline, and the slot comes back.
func TestAStalledWasabiDoesNotHoldAStreamSlot(t *testing.T) {
	stalled := make(chan struct{})
	wasabi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-stalled:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(stalled); wasabi.Close() })
	st := objectstorage.NewStore(objectstorage.Config{Endpoint: wasabi.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1})
	repo := &streamRepo{buckets: map[string]store.ObjectBucket{}}
	svc := objectstorage.NewService(repo, map[objectstorage.Region]objectstorage.Store{objectstorage.Oregon: st})
	svc.SetStreamDeadline(150 * time.Millisecond)
	ws := ids.New(ids.Workspace)
	b := store.ObjectBucket{ID: ids.New(ids.ObjectBucket), WorkspaceID: ws, Name: "photos", Region: "oregon"}

	slots := func() int {
		n := 0
		var releases []func()
		for {
			release, err := svc.AcquireStream(ws)
			if err != nil {
				break
			}
			releases = append(releases, release)
			n++
		}
		for _, r := range releases {
			r()
		}
		return n
	}
	start := time.Now()
	if _, err := svc.PutStream(context.Background(), ws, b, "k", 1, strings.NewReader("x"), objectstorage.ObjectHeaders{}); err == nil {
		t.Fatal("a put to a stalled Wasabi succeeded")
	}
	if _, err := svc.OpenObject(context.Background(), ws, b, "k"); err == nil {
		t.Fatal("a get from a stalled Wasabi succeeded")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("the stalled calls took %v; the deadline did not cut them", time.Since(start))
	}
	if got := slots(); got != objectstorage.MaxConcurrentStreamsPerWorkspace {
		t.Fatalf("%d of %d slots are free after the stalled calls", got, objectstorage.MaxConcurrentStreamsPerWorkspace)
	}
}
