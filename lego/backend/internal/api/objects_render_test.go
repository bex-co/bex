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

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// memObjectRepo is objectstorage.Repository in memory, so the object routes
// are exercised end to end without Postgres. The store's own tests cover the SQL.
type memObjectRepo struct {
	mu            sync.Mutex
	buckets       []store.ObjectBucket
	usage         []store.ObjectStorageUsage
	bytes, object int64
}

func (r *memObjectRepo) CreateObjectBucket(_ context.Context, in store.ObjectBucket, userCap int) (store.ObjectBucket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	users := 0
	for _, b := range r.buckets {
		if b.WorkspaceID != in.WorkspaceID {
			continue
		}
		if b.Name == in.Name {
			return store.ObjectBucket{}, store.ErrObjectBucketExists
		}
		if !b.System {
			users++
		}
	}
	if !in.System && users >= userCap {
		return store.ObjectBucket{}, store.ErrObjectBucketQuota
	}
	in.ID = ids.New(ids.ObjectBucket)
	in.CreatedAt = time.Now().UTC()
	r.buckets = append(r.buckets, in)
	return in, nil
}

func (r *memObjectRepo) EnsureSystemObjectBucket(_ context.Context, ws, name, region string) (store.ObjectBucket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.buckets {
		if b.WorkspaceID == ws && b.Name == name {
			return b, nil
		}
	}
	b := store.ObjectBucket{ID: ids.New(ids.ObjectBucket), WorkspaceID: ws, Name: name, Region: region, System: true, CreatedAt: time.Now().UTC()}
	r.buckets = append(r.buckets, b)
	return b, nil
}

func (r *memObjectRepo) GetObjectBucket(_ context.Context, ws, name string) (store.ObjectBucket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.buckets {
		if b.WorkspaceID == ws && b.Name == name {
			return b, nil
		}
	}
	return store.ObjectBucket{}, store.ErrNotFound
}

func (r *memObjectRepo) ListObjectBucketSummaries(_ context.Context, ws string) ([]store.ObjectBucket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []store.ObjectBucket
	for _, b := range r.buckets {
		if b.WorkspaceID == ws {
			out = append(out, b)
		}
	}
	return out, nil
}

func (r *memObjectRepo) DeleteObjectBucket(_ context.Context, ws, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, b := range r.buckets {
		if b.WorkspaceID == ws && b.ID == id {
			r.buckets = append(r.buckets[:i], r.buckets[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (r *memObjectRepo) ObjectStorageUsageTotals(context.Context, string, string) (int64, int64, error) {
	return r.bytes, r.object, nil
}

func (r *memObjectRepo) ListObjectStorageUsage(_ context.Context, ws string) ([]store.ObjectStorageUsage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []store.ObjectStorageUsage
	for _, u := range r.usage {
		if u.WorkspaceID == ws {
			out = append(out, u)
		}
	}
	return out, nil
}

func (r *memObjectRepo) SetObjectBucketCORS(_ context.Context, ws, id string, rules json.RawMessage) error {
	return r.update(ws, id, func(b *store.ObjectBucket) { b.CORS = rules })
}

func (r *memObjectRepo) SetObjectBucketPublic(_ context.Context, ws, id string, public bool) error {
	return r.update(ws, id, func(b *store.ObjectBucket) { b.Public = public })
}

func (r *memObjectRepo) GetObjectBucketByID(_ context.Context, id string) (store.ObjectBucket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.buckets {
		if b.ID == id {
			return b, nil
		}
	}
	return store.ObjectBucket{}, store.ErrNotFound
}

func (r *memObjectRepo) SetObjectBucketLifecycle(_ context.Context, ws, id string, rules json.RawMessage) error {
	return r.update(ws, id, func(b *store.ObjectBucket) { b.Lifecycle = rules })
}

func (r *memObjectRepo) update(ws, id string, set func(*store.ObjectBucket)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.buckets {
		if r.buckets[i].WorkspaceID == ws && r.buckets[i].ID == id {
			set(&r.buckets[i])
			return nil
		}
	}
	return store.ErrNotFound
}

type objectsHarness struct {
	mux  http.Handler
	fake *fakes3.Server
	repo *memObjectRepo
	ws   string
}

func newObjectsHarness(t *testing.T) *objectsHarness { return newObjectsHarnessAuthz(t, nil) }

// newObjectsHarnessAuthz is the harness with an OpenFGA stand-in on the Base,
// so a test can say which relations a caller holds.
func newObjectsHarnessAuthz(t *testing.T, authz core.Checker) *objectsHarness {
	t.Helper()
	fake := fakes3.New(objectstorage.Oregon.Bucket())
	t.Cleanup(fake.Close)
	st := objectstorage.NewStore(objectstorage.Config{
		Endpoint: fake.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1,
	})
	repo := &memObjectRepo{}
	svc := objectstorage.NewService(repo, map[objectstorage.Region]objectstorage.Store{objectstorage.Oregon: st})
	srv := &Server{RenderObjects: &renderObjects{base: &core.Base{Authz: authz}, svc: svc}}
	return &objectsHarness{mux: srv.restHandler(), fake: fake, repo: repo, ws: ids.New(ids.Workspace)}
}

type objectsReply struct {
	Code   int
	Header http.Header
	Body   map[string]any
	Raw    string
}

func (h *objectsHarness) do(t *testing.T, method, path, body string) objectsReply {
	t.Helper()
	rec := httptest.NewRecorder()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user-1", Method: core.MethodSession}))
	h.mux.ServeHTTP(rec, req)
	out := objectsReply{Code: rec.Code, Header: rec.Header(), Raw: rec.Body.String()}
	_ = json.Unmarshal(rec.Body.Bytes(), &out.Body)
	return out
}

func (h *objectsHarness) path(region, key string) string {
	p := "/v1/objects/" + h.ws + "/" + region
	if key != "" {
		p += "/" + key
	}
	return p
}

func TestRenderObjectsRoundTripThroughTheRoutes(t *testing.T) {
	h := newObjectsHarness(t)
	body := []byte("hello from the render cli")

	put := h.do(t, http.MethodPut, h.path("oregon", "dumps/a.txt"), fmt.Sprintf(`{"sizeBytes":%d}`, len(body)))
	if put.Code != http.StatusOK || put.Body["maxSizeBytes"] != float64(len(body)) || put.Body["url"] == "" || put.Body["expiresAt"] == "" {
		t.Fatalf("PUT = %d %s", put.Code, put.Raw)
	}
	resp, err := http.DefaultClient.Do(mustPut(put.Body["url"].(string), body))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("uploading to the presigned URL: %v %v", resp, err)
	}

	list := h.do(t, http.MethodGet, h.path("oregon", "")+"?limit=200", "")
	items, _ := list.Body["items"].([]any)
	if list.Code != http.StatusOK || len(items) != 1 || list.Body["hasNext"] != false {
		t.Fatalf("list = %d %s", list.Code, list.Raw)
	}
	item := items[0].(map[string]any)
	obj := item["object"].(map[string]any)
	if obj["key"] != "dumps/a.txt" || obj["sizeBytes"] != float64(len(body)) || obj["lastModified"] == "" || item["cursor"] == "" {
		t.Fatalf("list item = %v", item)
	}
	if _, has := list.Body["nextCursor"]; has {
		t.Fatalf("nextCursor is present with hasNext false: %s", list.Raw)
	}

	get := h.do(t, http.MethodGet, h.path("oregon", "dumps/a.txt"), "")
	if get.Code != http.StatusOK || get.Body["url"] == "" || get.Body["expiresAt"] == "" {
		t.Fatalf("GET = %d %s", get.Code, get.Raw)
	}
	dl, err := http.Get(get.Body["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(dl.Body)
	dl.Body.Close()
	if !bytes.Equal(got, body) || dl.Header.Get("Content-Disposition") != "attachment" {
		t.Fatalf("download = %q with disposition %q", got, dl.Header.Get("Content-Disposition"))
	}

	if del := h.do(t, http.MethodDelete, h.path("oregon", "dumps/a.txt"), ""); del.Code != http.StatusNoContent || del.Raw != "" {
		t.Fatalf("DELETE = %d %q, want 204 with no body", del.Code, del.Raw)
	}
	gone := h.do(t, http.MethodGet, h.path("oregon", "dumps/a.txt"), "")
	if gone.Code != http.StatusNotFound || gone.Body["code"] != "OBJECT_NOT_FOUND" || gone.Body["message"] != "object not found" {
		t.Fatalf("GET after DELETE = %d %s, want a 404 the CLI prints as 'object not found'", gone.Code, gone.Raw)
	}
	if again := h.do(t, http.MethodDelete, h.path("oregon", "dumps/a.txt"), ""); again.Code != http.StatusNotFound {
		t.Fatalf("DELETE of a missing key = %d, want 404", again.Code)
	}
}

func mustPut(url string, body []byte) *http.Request {
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	return req
}

func TestRenderObjectsListAnEmptyBucketIsAnEmptyArray(t *testing.T) {
	h := newObjectsHarness(t)
	list := h.do(t, http.MethodGet, h.path("oregon", ""), "")
	if list.Code != http.StatusOK || !strings.Contains(list.Raw, `"items":[]`) || strings.Contains(list.Raw, `"items":null`) {
		t.Fatalf("empty list = %d %s; Render's lists are arrays, never null", list.Code, list.Raw)
	}
}

func TestRenderObjectsListPagesWithHasNextAndCursor(t *testing.T) {
	h := newObjectsHarness(t)
	h.do(t, http.MethodGet, h.path("oregon", ""), "") // creates the system bucket
	sys, _ := h.repo.EnsureSystemObjectBucket(context.Background(), h.ws, "render-oregon", "oregon")
	for _, k := range []string{"a", "b", "c"} {
		h.fake.Put(objectstorage.Oregon.Bucket(), h.ws+"/"+sys.ID+"/"+k, []byte("v"))
	}
	first := h.do(t, http.MethodGet, h.path("oregon", "")+"?limit=2", "")
	items := first.Body["items"].([]any)
	if len(items) != 2 || first.Body["hasNext"] != true || first.Body["nextCursor"] != items[1].(map[string]any)["cursor"] {
		t.Fatalf("first page = %s", first.Raw)
	}
	second := h.do(t, http.MethodGet, h.path("oregon", "")+"?limit=2&cursor="+first.Body["nextCursor"].(string), "")
	items = second.Body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["object"].(map[string]any)["key"] != "c" || second.Body["hasNext"] != false {
		t.Fatalf("second page = %s", second.Raw)
	}
	// With no ?limit the default is 20, Render's own.
	if all := h.do(t, http.MethodGet, h.path("oregon", ""), ""); len(all.Body["items"].([]any)) != 3 {
		t.Fatalf("default page = %s", all.Raw)
	}
}

func TestRenderObjectsRefusals(t *testing.T) {
	h := newObjectsHarness(t)
	cases := []struct {
		name, method, path, body string
		code                     int
		errCode                  string
	}{
		{"unserved region on put", http.MethodPut, h.path("fsn1", "k"), `{"sizeBytes":1}`, 400, "REGION_NOT_OFFERED"},
		{"render's ohio", http.MethodGet, h.path("ohio", ""), "", 400, "REGION_NOT_OFFERED"},
		{"a location hint is not a region", http.MethodGet, h.path("wnam", "k"), "", 400, "REGION_NOT_OFFERED"},
		{"frankfurt has no store here", http.MethodGet, h.path("frankfurt", "k"), "", 400, "REGION_NOT_OFFERED"},
		{"missing sizeBytes", http.MethodPut, h.path("oregon", "k"), `{}`, 400, "SIZE_BYTES_REQUIRED"},
		{"malformed body", http.MethodPut, h.path("oregon", "k"), `not json`, 400, ""},
		{"a backslash in the key", http.MethodPut, h.path("oregon", "a%5Cb"), `{"sizeBytes":1}`, 400, "OBJECT_KEY_INVALID"},
		{"a control character in the key", http.MethodGet, h.path("oregon", "a%00b"), "", 400, "OBJECT_KEY_INVALID"},
		{"a key over 1024 bytes", http.MethodDelete, h.path("oregon", strings.Repeat("k", 1025)), "", 400, "OBJECT_KEY_INVALID"},
		{"one byte over 5 GiB", http.MethodPut, h.path("oregon", "k"), fmt.Sprintf(`{"sizeBytes":%d}`, objectstorage.MaxObjectBytesPresigned+1), 413, "OBJECT_TOO_LARGE"},
		{"a negative size", http.MethodPut, h.path("oregon", "k"), `{"sizeBytes":-1}`, 413, "OBJECT_TOO_LARGE"},
		{"limit zero", http.MethodGet, h.path("oregon", "") + "?limit=0", "", 400, ""},
		{"limit not a number", http.MethodGet, h.path("oregon", "") + "?limit=many", "", 400, ""},
		{"a cursor it did not mint", http.MethodGet, h.path("oregon", "") + "?cursor=!!!", "", 400, "INVALID_CURSOR"},
	}
	for _, c := range cases {
		got := h.do(t, c.method, c.path, c.body)
		if got.Code != c.code || (c.errCode != "" && got.Body["code"] != c.errCode) {
			t.Errorf("%s: %s %s = %d %s; want %d %s", c.name, c.method, c.path, got.Code, got.Raw, c.code, c.errCode)
		}
	}
	// None of the refusals above reached Wasabi.
	for _, r := range h.fake.Requests() {
		if r.Method == http.MethodPut || r.Method == http.MethodDelete {
			t.Errorf("a refused request reached Wasabi: %s %s", r.Method, r.Key)
		}
	}
}

func TestRenderObjectsQuotaAnswers429(t *testing.T) {
	h := newObjectsHarness(t)
	h.repo.bytes = objectstorage.MaxWorkspaceBytes - 5
	got := h.do(t, http.MethodPut, h.path("oregon", "k"), `{"sizeBytes":6}`)
	params, _ := got.Body["params"].(map[string]any)
	if got.Code != http.StatusTooManyRequests || got.Body["code"] != "OBJECT_STORAGE_QUOTA" || params["resource"] != "bytes" {
		t.Fatalf("a put over the byte cap = %d %s", got.Code, got.Raw)
	}
	if ok := h.do(t, http.MethodPut, h.path("oregon", "k"), `{"sizeBytes":5}`); ok.Code != http.StatusOK {
		t.Fatalf("a put that exactly fits = %d %s", ok.Code, ok.Raw)
	}
}

func TestRenderObjectsMintRateLimitAnswers429WithRetryAfter(t *testing.T) {
	h := newObjectsHarness(t)
	var last objectsReply
	for range objectstorage.MintBurst + 1 {
		last = h.do(t, http.MethodPut, h.path("oregon", "k"), `{"sizeBytes":1}`)
	}
	if last.Code != http.StatusTooManyRequests || last.Body["code"] != "OBJECT_REQUESTS_LIMITED" || last.Header.Get("Retry-After") != "1" {
		t.Fatalf("past the mint rate = %d %s retry-after %q", last.Code, last.Raw, last.Header.Get("Retry-After"))
	}
}

func TestRenderObjectsOfAnotherWorkspaceAreInvisible(t *testing.T) {
	h := newObjectsHarness(t)
	put := h.do(t, http.MethodPut, h.path("oregon", "secret.txt"), `{"sizeBytes":3}`)
	if _, err := http.DefaultClient.Do(mustPut(put.Body["url"].(string), []byte("abc"))); err != nil {
		t.Fatal(err)
	}
	other := &objectsHarness{mux: h.mux, fake: h.fake, repo: h.repo, ws: ids.New(ids.Workspace)}
	if got := other.do(t, http.MethodGet, other.path("oregon", "secret.txt"), ""); got.Code != http.StatusNotFound {
		t.Fatalf("another workspace's GET = %d %s, want 404", got.Code, got.Raw)
	}
	list := other.do(t, http.MethodGet, other.path("oregon", ""), "")
	if items, _ := list.Body["items"].([]any); len(items) != 0 {
		t.Fatalf("another workspace lists %d objects", len(items))
	}
}

// relationChecker is OpenFGA in miniature: it grants a fixed set of relations
// on one workspace and records every check it is asked.
type relationChecker struct {
	mu        sync.Mutex
	workspace string
	held      map[string]bool
	asked     []string
}

func (c *relationChecker) Check(_ context.Context, _, relation, object string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.asked = append(c.asked, relation+" on "+object)
	return object == core.WorkspaceObject(c.workspace) && c.held[relation], nil
}

func TestRenderObjectsAskForTheRelationEachOperationNeeds(t *testing.T) {
	h := newObjectsHarnessAuthz(t, nil)
	c := &relationChecker{workspace: h.ws, held: map[string]bool{
		core.RelCanView: true, core.RelCanViewSensitive: true, core.RelCanCreate: true,
	}}
	h = newObjectsHarnessAuthz(t, c)
	c.workspace = h.ws
	put := h.do(t, http.MethodPut, h.path("oregon", "k"), `{"sizeBytes":1}`)
	if _, err := http.DefaultClient.Do(mustPut(put.Body["url"].(string), []byte("x"))); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct{ method, path, body, relation string }{
		{http.MethodGet, h.path("oregon", ""), "", core.RelCanView},
		{http.MethodPut, h.path("oregon", "k"), `{"sizeBytes":1}`, core.RelCanCreate},
		{http.MethodGet, h.path("oregon", "k"), "", core.RelCanViewSensitive},
		{http.MethodDelete, h.path("oregon", "k"), "", core.RelCanCreate},
	} {
		c.mu.Lock()
		c.asked = nil
		c.mu.Unlock()
		if got := h.do(t, step.method, step.path, step.body); got.Code >= 400 {
			t.Fatalf("%s %s with every relation held = %d %s", step.method, step.path, got.Code, got.Raw)
		}
		want := step.relation + " on " + core.WorkspaceObject(h.ws)
		c.mu.Lock()
		asked := append([]string(nil), c.asked...)
		c.mu.Unlock()
		if len(asked) == 0 || asked[0] != want {
			t.Errorf("%s %s asked %v, want %q first", step.method, step.path, asked, want)
		}
	}
}

func TestRenderObjectsAViewerCanListButNotReadWriteOrDelete(t *testing.T) {
	c := &relationChecker{held: map[string]bool{core.RelCanView: true}}
	h := newObjectsHarnessAuthz(t, c)
	c.workspace = h.ws
	if got := h.do(t, http.MethodGet, h.path("oregon", ""), ""); got.Code != http.StatusOK {
		t.Fatalf("a viewer listing = %d %s", got.Code, got.Raw)
	}
	for _, step := range []struct{ method, path, body string }{
		{http.MethodGet, h.path("oregon", "k"), ""},
		{http.MethodPut, h.path("oregon", "k"), `{"sizeBytes":1}`},
		{http.MethodDelete, h.path("oregon", "k"), ""},
	} {
		if got := h.do(t, step.method, step.path, step.body); got.Code != http.StatusForbidden {
			t.Errorf("a viewer %s %s = %d %s, want 403", step.method, step.path, got.Code, got.Raw)
		}
	}
	for _, r := range h.fake.Requests() {
		if r.Key != "" {
			t.Errorf("a refused caller reached Wasabi: %s %s", r.Method, r.Key)
		}
	}
}

func TestRenderObjectsOfAnotherWorkspaceAreRefusedByRelation(t *testing.T) {
	c := &relationChecker{held: map[string]bool{core.RelCanView: true, core.RelCanViewSensitive: true, core.RelCanCreate: true}}
	h := newObjectsHarnessAuthz(t, c)
	c.workspace = ids.New(ids.Workspace) // the caller belongs to some other workspace
	for _, step := range []struct{ method, path, body string }{
		{http.MethodGet, h.path("oregon", ""), ""},
		{http.MethodGet, h.path("oregon", "k"), ""},
		{http.MethodPut, h.path("oregon", "k"), `{"sizeBytes":1}`},
		{http.MethodDelete, h.path("oregon", "k"), ""},
	} {
		if got := h.do(t, step.method, step.path, step.body); got.Code != http.StatusForbidden {
			t.Errorf("%s %s in a workspace the caller is not in = %d %s, want 403", step.method, step.path, got.Code, got.Raw)
		}
	}
	// Refusal leaves no trace: no bucket row, nothing sent to Wasabi.
	if len(h.repo.buckets) != 0 || len(h.fake.Requests()) != 0 {
		t.Errorf("a refused caller created %d buckets and caused %d Wasabi requests", len(h.repo.buckets), len(h.fake.Requests()))
	}
}
