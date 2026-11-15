package main_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ADR112 D2, D11: the unmodified Render CLI's `ea objects` works against bex.
// bex answers the four routes with presigned URLs, so the bytes go from the
// CLI to the URL and never through bex-api. This is the pinned-CLI contract
// test: a stand-in for bex-api that answers in exactly the shapes bex's
// handlers do (lego/backend/internal/api/objects_render.go, whose own tests
// are written against the same generated client types), and a stand-in for
// Wasabi behind the URLs. A pin bump that changes what the CLI sends or reads
// fails here.
type objectsAPI struct {
	srv *httptest.Server

	mu       sync.Mutex
	objects  map[string][]byte // "region/key" -> body
	putSizes map[string]int64  // what each PUT mint declared
	calls    []objectsCall     // every bex-api route call
	chunked  bool              // a PUT to the URL used chunked transfer encoding
	badLen   string            // a PUT to the URL whose length differed from the mint
}

func newObjectsAPI(t *testing.T) *objectsAPI {
	t.Helper()
	a := &objectsAPI{objects: map[string][]byte{}, putSizes: map[string]int64{}}
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /v1/objects/{ownerId}/{region}/{key...}", a.mintPut)
	mux.HandleFunc("GET /v1/objects/{ownerId}/{region}/{key...}", a.mintGet)
	mux.HandleFunc("DELETE /v1/objects/{ownerId}/{region}/{key...}", a.deleteObject)
	mux.HandleFunc("GET /v1/objects/{ownerId}/{region}", a.list)
	mux.HandleFunc("PUT /wasabi/{region}/{key...}", a.upload)
	mux.HandleFunc("GET /wasabi/{region}/{key...}", a.download)
	a.srv = httptest.NewServer(mux)
	t.Cleanup(a.srv.Close)
	return a
}

func objectID(r *http.Request) string { return r.PathValue("region") + "/" + r.PathValue("key") }

func presignExpiry() string { return time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339) }

func (a *objectsAPI) record(r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, objectsCall{Request: r.Method + " " + r.URL.RequestURI(), Auth: r.Header.Get("Authorization")})
}

func objectsWriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func objectsNotFound(w http.ResponseWriter) {
	objectsWriteJSON(w, http.StatusNotFound, map[string]any{"id": "x", "error": "object not found", "message": "object not found", "code": "OBJECT_NOT_FOUND"})
}

func (a *objectsAPI) mintPut(w http.ResponseWriter, r *http.Request) {
	a.record(r)
	var in struct {
		SizeBytes *int64 `json:"sizeBytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.SizeBytes == nil {
		objectsWriteJSON(w, http.StatusBadRequest, map[string]any{"error": "sizeBytes is required", "message": "sizeBytes is required"})
		return
	}
	id := objectID(r)
	a.mu.Lock()
	a.putSizes[id] = *in.SizeBytes
	a.mu.Unlock()
	objectsWriteJSON(w, http.StatusOK, map[string]any{
		"url":          a.srv.URL + "/wasabi/" + id,
		"expiresAt":    presignExpiry(),
		"maxSizeBytes": *in.SizeBytes,
	})
}

func (a *objectsAPI) mintGet(w http.ResponseWriter, r *http.Request) {
	a.record(r)
	id := objectID(r)
	a.mu.Lock()
	_, ok := a.objects[id]
	a.mu.Unlock()
	if !ok {
		objectsNotFound(w)
		return
	}
	objectsWriteJSON(w, http.StatusOK, map[string]any{
		"url": a.srv.URL + "/wasabi/" + id, "expiresAt": presignExpiry(),
	})
}

func (a *objectsAPI) deleteObject(w http.ResponseWriter, r *http.Request) {
	a.record(r)
	id := objectID(r)
	a.mu.Lock()
	_, ok := a.objects[id]
	delete(a.objects, id)
	a.mu.Unlock()
	if !ok {
		objectsNotFound(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *objectsAPI) list(w http.ResponseWriter, r *http.Request) {
	a.record(r)
	region := r.PathValue("region")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 20
	}
	after := ""
	if c := r.URL.Query().Get("cursor"); c != "" {
		raw, _ := base64.RawURLEncoding.DecodeString(c)
		after = string(raw)
	}
	a.mu.Lock()
	var keys []string
	for id := range a.objects {
		if key, ok := strings.CutPrefix(id, region+"/"); ok && key > after {
			keys = append(keys, key)
		}
	}
	sizes := map[string]int{}
	for _, k := range keys {
		sizes[k] = len(a.objects[region+"/"+k])
	}
	a.mu.Unlock()
	sort.Strings(keys)
	hasNext := len(keys) > limit
	if hasNext {
		keys = keys[:limit]
	}
	items := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		items = append(items, map[string]any{
			"cursor": base64.RawURLEncoding.EncodeToString([]byte(k)),
			"object": map[string]any{"key": k, "sizeBytes": sizes[k], "lastModified": "2026-11-15T10:00:00Z"},
		})
	}
	out := map[string]any{"items": items, "hasNext": hasNext}
	if hasNext {
		out["nextCursor"] = items[len(items)-1]["cursor"]
	}
	objectsWriteJSON(w, http.StatusOK, out)
}

// upload is Wasabi behind a presigned PUT: it checks what a signed
// Content-Length would enforce, that the body is exactly what the mint declared.
func (a *objectsAPI) upload(w http.ResponseWriter, r *http.Request) {
	id := objectID(r)
	body, _ := io.ReadAll(r.Body)
	a.mu.Lock()
	defer a.mu.Unlock()
	if slices.Contains(r.TransferEncoding, "chunked") {
		a.chunked = true
	}
	if want, ok := a.putSizes[id]; !ok || int64(len(body)) != want || r.ContentLength != want {
		a.badLen = fmt.Sprintf("%s: minted %d, sent %d with Content-Length %d", id, a.putSizes[id], len(body), r.ContentLength)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	a.objects[id] = body
	w.WriteHeader(http.StatusOK)
}

func (a *objectsAPI) download(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	body, ok := a.objects[r.PathValue("region")+"/"+r.PathValue("key")]
	a.mu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	_, _ = w.Write(body)
}

type objectsCall struct{ Request, Auth string }

func (a *objectsAPI) routeCalls() []objectsCall {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.calls)
}

func TestEaObjectsRoundTripsThroughPresignedURLs(t *testing.T) {
	api := newObjectsAPI(t)
	dir := t.TempDir()
	// Empty, one byte, and 5 MiB: the CLI sends chunked bodies for a zero
	// length unless told otherwise, which a presigned URL cannot take.
	for name, size := range map[string]int{"empty.bin": 0, "one.bin": 1, "five-mib.bin": 5 << 20} {
		content := make([]byte, size)
		_, _ = rand.Read(content)
		src := filepath.Join(dir, "src-"+name)
		if err := os.WriteFile(src, content, 0o600); err != nil {
			t.Fatal(err)
		}
		key := "dumps/" + name
		if _, stderr, err := runBexAgainst(t, api.srv.URL, "ea", "objects", "put", key, "--file", src, "--region", "oregon"); err != nil {
			t.Fatalf("put %s: %v\n%s", name, err, stderr)
		}
		dst := filepath.Join(dir, "dst-"+name)
		if _, stderr, err := runBexAgainst(t, api.srv.URL, "ea", "objects", "get", key, "--file", dst, "--region", "oregon"); err != nil {
			t.Fatalf("get %s: %v\n%s", name, err, stderr)
		}
		got, err := os.ReadFile(dst)
		if err != nil || !bytes.Equal(got, content) {
			t.Fatalf("%s: downloaded %d bytes (%v), want the %d uploaded, byte for byte", name, len(got), err, size)
		}
	}
	if api.badLen != "" {
		t.Fatalf("an upload did not match what the mint declared: %s", api.badLen)
	}
	if api.chunked {
		t.Fatal("an upload used chunked transfer encoding; a presigned URL with a signed Content-Length cannot take it")
	}
	if api.putSizes["oregon/dumps/empty.bin"] != 0 || api.putSizes["oregon/dumps/five-mib.bin"] != 5<<20 {
		t.Fatalf("sizeBytes sent: %v", api.putSizes)
	}
	// Every route call was authenticated as the CLI's configured key.
	for _, call := range api.routeCalls() {
		if call.Auth != "Bearer test-access-token" {
			t.Errorf("route call %q sent Authorization %q", call.Request, call.Auth)
		}
	}
}

func TestEaObjectsListHonorsALimitOf200AndPages(t *testing.T) {
	api := newObjectsAPI(t)
	api.mu.Lock()
	for i := range 250 {
		api.objects[fmt.Sprintf("oregon/k/%04d", i)] = []byte("v")
	}
	api.mu.Unlock()

	stdout, stderr, err := runBexAgainst(t, api.srv.URL, "ea", "objects", "list", "--region", "oregon", "--limit", "200", "-o", "json")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, stderr)
	}
	if n := strings.Count(stdout, `"key"`); n != 200 {
		t.Fatalf("`--limit 200` printed %d keys, want 200 (a lower server cap would truncate silently)\n%.400s", n, stdout)
	}
	calls := api.routeCalls()
	if len(calls) == 0 || !strings.Contains(calls[0].Request, "limit=200") {
		t.Fatalf("the CLI sent %v, want limit=200", calls)
	}
	if !strings.Contains(stdout, "k/0000") || strings.Contains(stdout, "k/0250") {
		t.Fatalf("listing is not the first 200 keys in order\n%.300s", stdout)
	}
}

func TestEaObjectsNeitherGetNorDeleteInventAMissingObject(t *testing.T) {
	api := newObjectsAPI(t)
	dst := filepath.Join(t.TempDir(), "out")
	if _, stderr, err := runBexAgainst(t, api.srv.URL, "ea", "objects", "get", "nope", "--file", dst, "--region", "oregon"); err == nil || !strings.Contains(stderr, "object not found") {
		t.Errorf("get of a missing key: err=%v stderr=%q, want it to say 'object not found'", err, stderr)
	}
	if _, err := os.Stat(dst); err == nil {
		t.Error("get of a missing key left a file behind")
	}
	if _, stderr, err := runBexAgainst(t, api.srv.URL, "ea", "objects", "delete", "nope", "--region", "oregon", "--yes"); err == nil || !strings.Contains(stderr, "object not found") {
		t.Errorf("delete of a missing key: err=%v stderr=%q, want it to say 'object not found'", err, stderr)
	}
}

func TestEaObjectsDeleteRemovesTheObject(t *testing.T) {
	api := newObjectsAPI(t)
	api.mu.Lock()
	api.objects["oregon/a/b"] = []byte("x")
	api.mu.Unlock()
	if _, stderr, err := runBexAgainst(t, api.srv.URL, "ea", "objects", "delete", "a/b", "--region", "oregon", "--yes"); err != nil {
		t.Fatalf("delete: %v\n%s", err, stderr)
	}
	api.mu.Lock()
	_, left := api.objects["oregon/a/b"]
	api.mu.Unlock()
	if left {
		t.Fatal("the object is still there after `delete --yes`")
	}
}

// --local is upstream's own filesystem mode and must keep working without ever
// calling bex-api.
func TestEaObjectsLocalModeNeverReachesTheAPI(t *testing.T) {
	var requests atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.NotFound(w, r)
	}))
	defer api.Close()
	for _, local := range []*exec.Cmd{
		bexAgainst(t, api.URL, "ea", "objects", "list", "--region", "oregon", "--local"),
		bexAgainst(t, api.URL, "ea", "objects", "list", "--region", "oregon"),
	} {
		if !slices.Contains(local.Args, "--local") {
			local.Env = append(local.Env, "RENDER_USE_LOCAL_DEV=true")
		}
		local.Dir = t.TempDir()
		var stderr bytes.Buffer
		local.Stderr = &stderr
		if err := local.Run(); err != nil {
			t.Fatalf("%v (local mode) failed: %v (stderr %q)", local.Args, err, stderr.String())
		}
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("sent %d requests in local mode, want none", n)
	}
}
