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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// cfHarness drives the Cloudflare-shaped routes through the surface's own
// chain (gate, scope-class check, routing) with an identity injected in place
// of the OAuth gate, over a fake S3 and an in-memory bucket registry.
type cfHarness struct {
	srv  *Server
	h    http.Handler
	fake *fakes3.Server
	repo *memObjectRepo
	svc  *objectstorage.Service
	ws   string
}

// cfTestAuth stands in for the OAuth gate: every request is a signed-in human.
func cfTestAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(core.WithIdentity(r.Context(), core.Identity{Subject: "user-1", Method: core.MethodSession}))
		next.ServeHTTP(w, r)
	})
}

func passThrough(next http.Handler) http.Handler { return next }

func newCFHarness(t *testing.T) *cfHarness { return newCFHarnessAuthz(t, nil) }

func newCFHarnessAuthz(t *testing.T, authz core.Checker) *cfHarness {
	t.Helper()
	fake := fakes3.New(objectstorage.Oregon.Bucket(), objectstorage.Frankfurt.Bucket())
	t.Cleanup(fake.Close)
	stores := map[objectstorage.Region]objectstorage.Store{}
	for _, r := range objectstorage.Regions {
		stores[r] = objectstorage.NewStore(objectstorage.Config{
			Endpoint: fake.URL, Region: r.WasabiRegion(), Bucket: r.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1,
		})
	}
	repo := &memObjectRepo{}
	svc := objectstorage.NewService(repo, stores)
	srv := &Server{CloudflareR2: newCloudflareR2(&core.Base{Authz: authz}, svc)}
	return &cfHarness{
		srv: srv, h: srv.cloudflareHandler(cfTestAuth, passThrough), fake: fake, repo: repo, svc: svc, ws: ids.New(ids.Workspace),
	}
}

// cfReply is one answer, with the envelope decoded when the body is one.
type cfReply struct {
	Code   int
	Header http.Header
	Raw    []byte
	Env    struct {
		Success  bool            `json:"success"`
		Errors   []cfErrorEntry  `json:"errors"`
		Messages []string        `json:"messages"`
		Result   json.RawMessage `json:"result"`
	}
}

func (r *cfReply) errCode() int {
	if len(r.Env.Errors) == 0 {
		return 0
	}
	return r.Env.Errors[0].Code
}

// url is a path under the account's r2 tree.
func (h *cfHarness) url(suffix string) string {
	return cloudflareMount + "/accounts/" + h.ws + "/r2" + suffix
}

func (h *cfHarness) do(t *testing.T, method, path string, body []byte, headers ...string) *cfReply {
	t.Helper()
	return serveCF(t, h.h, method, path, body, headers...)
}

func serveCF(t *testing.T, h http.Handler, method, path string, body []byte, headers ...string) *cfReply {
	t.Helper()
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := &cfReply{Code: rec.Code, Header: rec.Header(), Raw: rec.Body.Bytes()}
	_ = json.Unmarshal(out.Raw, &out.Env)
	return out
}

// wantCF fails unless the answer is the given status and Cloudflare code.
func wantCF(t *testing.T, r *cfReply, status, code int) {
	t.Helper()
	if r.Code != status || r.errCode() != code || r.Env.Success {
		t.Fatalf("answer = %d %s, want %d with code %d", r.Code, r.Raw, status, code)
	}
}

func wantOK(t *testing.T, r *cfReply) {
	t.Helper()
	if r.Code != http.StatusOK || !r.Env.Success || len(r.Env.Errors) != 0 {
		t.Fatalf("answer = %d %s, want 200 success", r.Code, r.Raw)
	}
}

// resultOf decodes the envelope's result.
func resultOf[T any](t *testing.T, r *cfReply) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(r.Env.Result, &out); err != nil {
		t.Fatalf("result %s: %v", r.Env.Result, err)
	}
	return out
}

func (h *cfHarness) createBucket(t *testing.T, name string) {
	t.Helper()
	wantOK(t, h.do(t, http.MethodPost, h.url("/buckets"), fmt.Appendf(nil, `{"name":%q}`, name), "Content-Type", "text/plain"))
}

func TestCloudflareEveryAnswerIsTheEnvelope(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")

	list := h.do(t, http.MethodGet, h.url("/buckets"), nil)
	wantOK(t, list)
	if list.Header.Get("Content-Type") != "application/json" || list.Env.Errors == nil || list.Env.Messages == nil {
		t.Fatalf("a success must carry success, errors:[] and messages:[]: %s", list.Raw)
	}

	// Every 2xx has a body: wrangler fails on a bodiless one from an object route.
	for _, step := range []struct {
		method, path string
		body         []byte
	}{
		{http.MethodPut, h.url("/buckets/photos/objects/a.txt"), []byte("hello")},
		{http.MethodDelete, h.url("/buckets/photos/objects/a.txt"), nil},
		{http.MethodDelete, h.url("/buckets/photos/objects/never-existed"), nil},
		{http.MethodDelete, h.url("/buckets/photos"), nil},
	} {
		r := h.do(t, step.method, step.path, step.body)
		wantOK(t, r)
		if len(r.Raw) == 0 || string(r.Env.Result) == "" || string(r.Env.Result) == "null" {
			t.Errorf("%s %s answered %d with result %q; a 2xx needs a body with a result", step.method, step.path, r.Code, r.Env.Result)
		}
	}
}

// TestCloudflareEachErrorCodeIsProduced pins every code of the table to the
// situation that answers it (ADR112 D3).
func TestCloudflareEachErrorCodeIsProduced(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/kept.txt"), []byte("x")))

	// 10005: a name outside wrangler's rule, and the reserved render- prefix.
	wantCF(t, h.do(t, http.MethodPost, h.url("/buckets"), []byte(`{"name":"Bad_Name"}`)), http.StatusBadRequest, 10005)
	wantCF(t, h.do(t, http.MethodPost, h.url("/buckets"), []byte(`{"name":"render-mine"}`)), http.StatusBadRequest, 10005)
	// 10073: a name that is taken.
	wantCF(t, h.do(t, http.MethodPost, h.url("/buckets"), []byte(`{"name":"photos"}`)), http.StatusConflict, 10073)
	// 10006: a bucket that is not there, on every route that names one.
	for _, step := range []struct{ method, suffix string }{
		{http.MethodGet, "/buckets/ghost"}, {http.MethodDelete, "/buckets/ghost"},
		{http.MethodGet, "/buckets/ghost/objects/k"}, {http.MethodPut, "/buckets/ghost/objects/k"},
		{http.MethodGet, "/buckets/ghost/cors"}, {http.MethodGet, "/buckets/ghost/lifecycle"},
	} {
		wantCF(t, h.do(t, step.method, h.url(step.suffix), []byte("x")), http.StatusNotFound, 10006)
	}
	// 10007: a key that is not there.
	wantCF(t, h.do(t, http.MethodGet, h.url("/buckets/photos/objects/ghost.txt"), nil), http.StatusNotFound, 10007)
	// 10008: a bucket that still holds an object.
	wantCF(t, h.do(t, http.MethodDelete, h.url("/buckets/photos"), nil), http.StatusConflict, 10008)
	// 10020: a key bex will not store (a backslash and a control character).
	wantCF(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/a%5Cb"), []byte("x")), http.StatusBadRequest, 10020)
	wantCF(t, h.do(t, http.MethodGet, h.url("/buckets/photos/objects/a%01b"), nil), http.StatusBadRequest, 10020)
	// 10033: a put with no length (chunked).
	chunked := httptest.NewRequest(http.MethodPut, h.url("/buckets/photos/objects/k"), io.NopCloser(strings.NewReader("x")))
	chunked.ContentLength = -1
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, chunked)
	if rec.Code != http.StatusLengthRequired || !strings.Contains(rec.Body.String(), `"code":10033`) {
		t.Fatalf("a put with no length = %d %s, want 411 with 10033", rec.Code, rec.Body)
	}
	// 100100: declared above the 300 MiB cap, refused before a byte is read.
	big := httptest.NewRequest(http.MethodPut, h.url("/buckets/photos/objects/big"), io.NopCloser(strings.NewReader("x")))
	big.ContentLength = objectstorage.MaxObjectBytesStreamed + 1
	rec = httptest.NewRecorder()
	h.h.ServeHTTP(rec, big)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":100100`) {
		t.Fatalf("300 MiB + 1 = %d %s, want 400 with 100100", rec.Code, rec.Body)
	}
	// 10042: a feature bex does not offer, and a region it does not serve.
	wantCF(t, h.do(t, http.MethodPatch, h.url("/buckets/photos"), nil, "cf-r2-storage-class", "InfrequentAccess"), http.StatusForbidden, 10042)
	wantCF(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/cold"), []byte("x"), "cf-r2-storage-class", "InfrequentAccess"), http.StatusForbidden, 10042)
	wantCF(t, h.do(t, http.MethodPost, h.url("/buckets"), []byte(`{"name":"east","locationHint":"enam"}`)), http.StatusBadRequest, 10042)
	wantCF(t, h.do(t, http.MethodPost, h.url("/buckets"), []byte(`{"name":"ia","storageClass":"InfrequentAccess"}`)), http.StatusForbidden, 10042)
	// 7003 and 7000: a URI or a method Cloudflare's own API would not route.
	wantCF(t, h.do(t, http.MethodGet, cloudflareMount+"/nonsense", nil), http.StatusNotFound, 7003)
	wantCF(t, h.do(t, http.MethodPost, h.url("/buckets/photos/objects/k"), nil), http.StatusMethodNotAllowed, 7000)
	// 7400: a body that is not JSON.
	wantCF(t, h.do(t, http.MethodPost, h.url("/buckets"), []byte(`not json`)), http.StatusBadRequest, 7400)
}

func TestCloudflareTooManyBucketsIs10009(t *testing.T) {
	h := newCFHarness(t)
	for i := range objectstorage.MaxUserBuckets {
		if _, err := h.repo.CreateObjectBucket(context.Background(),
			store.ObjectBucket{WorkspaceID: h.ws, Name: fmt.Sprintf("bucket-%04d", i), Region: "oregon"}, objectstorage.MaxUserBuckets); err != nil {
			t.Fatal(err)
		}
	}
	wantCF(t, h.do(t, http.MethodPost, h.url("/buckets"), []byte(`{"name":"one-too-many"}`)), http.StatusBadRequest, 10009)
}

func TestCloudflareAnUnconfiguredStoreAnswers10042OnEveryRoute(t *testing.T) {
	srv := &Server{CloudflareR2: newCloudflareR2(&core.Base{}, nil)}
	h := srv.cloudflareHandler(cfTestAuth, passThrough)
	ws := ids.New(ids.Workspace)
	base := cloudflareMount + "/accounts/" + ws + "/r2"
	for _, step := range []struct{ method, path string }{
		{http.MethodGet, base + "/buckets"}, {http.MethodPost, base + "/buckets"}, {http.MethodGet, base + "/buckets/b"},
		{http.MethodDelete, base + "/buckets/b"}, {http.MethodPut, base + "/buckets/b/objects/k"},
		{http.MethodGet, base + "/buckets/b/objects/k"}, {http.MethodDelete, base + "/buckets/b/objects/k"},
		{http.MethodGet, base + "/buckets/b/cors"}, {http.MethodPut, base + "/buckets/b/lifecycle"},
		{http.MethodPost, cloudflareGraphQLPath},
	} {
		wantCF(t, serveCF(t, h, step.method, step.path, []byte(`{}`)), http.StatusForbidden, 10042)
	}
	// An unknown route is still an unknown route.
	wantCF(t, serveCF(t, h, http.MethodGet, cloudflareMount+"/nonsense", nil), http.StatusNotFound, 7003)
}

// TestCloudflareFailuresBeforeARouteAreEnvelopes goes through the real handler
// chain: the OAuth gate's 401 and the limiter's refusals are written by shared
// middleware in bex's own shape, and must still reach wrangler as Cloudflare's.
func TestCloudflareFailuresBeforeARouteAreEnvelopes(t *testing.T) {
	fake := fakes3.New(objectstorage.Oregon.Bucket())
	t.Cleanup(fake.Close)
	st := objectstorage.NewStore(objectstorage.Config{Endpoint: fake.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1})
	svc := objectstorage.NewService(&memObjectRepo{}, map[objectstorage.Region]objectstorage.Store{objectstorage.Oregon: st})
	srv := NewServer(&core.Base{}, Deps{ObjectStorage: svc})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.RateLimiter = NewRateLimiter(60, 1)
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	ws := ids.New(ids.Workspace)
	list := cloudflareMount + "/accounts/" + ws + "/r2/buckets"

	noToken := do(t, h, http.MethodGet, list, "", "")
	var env cfReply
	_ = json.Unmarshal(noToken.Body.Bytes(), &env.Env)
	if noToken.Code != http.StatusUnauthorized || len(env.Env.Errors) != 1 || env.Env.Errors[0].Code != 10002 || env.Env.Success {
		t.Fatalf("no token = %d %s, want 401 in the envelope with 10002", noToken.Code, noToken.Body)
	}
	if noToken.Header().Get("WWW-Authenticate") == "" {
		t.Errorf("the gate's WWW-Authenticate header was dropped")
	}
	badToken := do(t, h, http.MethodGet, list, "wrong", "")
	if badToken.Code != http.StatusUnauthorized || !strings.Contains(badToken.Body.String(), `"code":10002`) {
		t.Fatalf("bad token = %d %s", badToken.Code, badToken.Body)
	}

	// The limiter's budget of one is spent by the first call; the second is shed.
	first := do(t, h, http.MethodGet, list, testToken, "")
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"success":true`) {
		t.Fatalf("an authenticated list = %d %s", first.Code, first.Body)
	}
	second := do(t, h, http.MethodGet, list, testToken, "")
	if second.Code != http.StatusTooManyRequests || !strings.Contains(second.Body.String(), `"code":10058`) || second.Header().Get("Retry-After") == "" {
		t.Fatalf("a limited call = %d %s retry-after %q, want 429 with 10058 and Retry-After", second.Code, second.Body, second.Header().Get("Retry-After"))
	}
}

func jsonUnmarshal(raw []byte, dst any) error { return json.Unmarshal(raw, dst) }

func contains(raw []byte, s string) bool { return bytes.Contains(raw, []byte(s)) }

func newWS() string { return ids.New(ids.Workspace) }

func storeBucket(ws, name string) store.ObjectBucket {
	return store.ObjectBucket{WorkspaceID: ws, Name: name, Region: "oregon"}
}
