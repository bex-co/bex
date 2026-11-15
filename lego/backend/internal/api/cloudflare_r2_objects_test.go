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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestCloudflareObjectRoundTripKeepsBytesAndStoredHeaders(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	body := bytes.Repeat([]byte("0123456789abcdef"), 1024)

	put := h.do(t, http.MethodPut, h.url("/buckets/photos/objects/dir/cat%20pic.jpg"), body,
		"Content-Type", "image/jpeg", "Cache-Control", "max-age=60", "Content-Disposition", `inline; filename="cat.jpg"`,
		"Content-Encoding", "identity", "Content-Language", "en", "Expires", "Wed, 21 Oct 2037 07:28:00 GMT",
		"cf-r2-data-catalog-check", "true", "cf-r2-storage-class", "Standard")
	wantOK(t, put)

	got := h.do(t, http.MethodGet, h.url("/buckets/photos/objects/dir/cat%20pic.jpg"), nil)
	if got.Code != http.StatusOK || !bytes.Equal(got.Raw, body) {
		t.Fatalf("get = %d, %d bytes, want the %d bytes put", got.Code, len(got.Raw), len(body))
	}
	for name, want := range map[string]string{
		"Content-Type": "image/jpeg", "Cache-Control": "max-age=60", "Content-Disposition": `inline; filename="cat.jpg"`,
		"Content-Encoding": "identity", "Content-Language": "en", "Expires": "Wed, 21 Oct 2037 07:28:00 GMT",
		"Content-Length": fmt.Sprint(len(body)),
	} {
		if got.Header.Get(name) != want {
			t.Errorf("%s = %q, want the stored %q", name, got.Header.Get(name), want)
		}
	}
	if got.Header.Get("ETag") == "" || got.Header.Get("Last-Modified") == "" {
		t.Errorf("ETag %q and Last-Modified %q are missing", got.Header.Get("ETag"), got.Header.Get("Last-Modified"))
	}
	// A browser that opens this URL must not render a tenant's bytes on bex's origin.
	if got.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(got.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("object bytes are served without nosniff and a sandboxing CSP: %v", got.Header)
	}

	// The key in Wasabi is <workspace>/<bucket id>/<key>, bucket id not name.
	row, _ := h.repo.GetObjectBucket(t.Context(), h.ws, "photos")
	if _, ok := h.fake.Get(objectstorage.Oregon.Bucket(), h.ws+"/"+row.ID+"/dir/cat pic.jpg"); !ok {
		t.Fatalf("the object is not under <workspace>/<bucket id>/<key>: %v", h.fake.Keys(objectstorage.Oregon.Bucket(), ""))
	}

	// A put overwrites, an empty object is a valid object, and a delete of a
	// missing key succeeds as S3 and Cloudflare answer.
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/dir/cat%20pic.jpg"), []byte("v2")))
	if again := h.do(t, http.MethodGet, h.url("/buckets/photos/objects/dir/cat%20pic.jpg"), nil); string(again.Raw) != "v2" {
		t.Fatalf("after an overwrite = %q", again.Raw)
	}
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/empty"), []byte{}))
	e := h.do(t, http.MethodGet, h.url("/buckets/photos/objects/empty"), nil)
	if e.Code != 200 || len(e.Raw) != 0 {
		t.Fatalf("an empty object = %d %q", e.Code, e.Raw)
	}
	// An object stored with no Content-Disposition is still served as a download.
	if got := e.Header.Get("Content-Disposition"); got != "attachment" {
		t.Errorf("Content-Disposition = %q, want attachment for an object that stored none", got)
	}
	wantOK(t, h.do(t, http.MethodDelete, h.url("/buckets/photos/objects/dir/cat%20pic.jpg"), nil))
	wantCF(t, h.do(t, http.MethodGet, h.url("/buckets/photos/objects/dir/cat%20pic.jpg"), nil), http.StatusNotFound, 10007)
	wantOK(t, h.do(t, http.MethodDelete, h.url("/buckets/photos/objects/dir/cat%20pic.jpg"), nil))
}

// TestCloudflareHeadAsksWasabiForHeadersOnly: a HEAD answers the stored
// headers and length with no body, by a HEAD to Wasabi and not a GET that is
// then abandoned, and a missing key is 404 with no body.
func TestCloudflareHeadAsksWasabiForHeadersOnly(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/k"), []byte("12345"), "Content-Type", "text/plain", "Cache-Control", "max-age=5", "Expires", "Wed, 21 Oct 2037 07:28:00 GMT"))
	before := len(h.fake.Requests())

	head := h.do(t, http.MethodHead, h.url("/buckets/photos/objects/k"), nil)
	if head.Code != http.StatusOK || len(head.Raw) != 0 {
		t.Fatalf("HEAD = %d with %d body bytes", head.Code, len(head.Raw))
	}
	for name, want := range map[string]string{"Content-Length": "5", "Content-Type": "text/plain", "Cache-Control": "max-age=5", "Expires": "Wed, 21 Oct 2037 07:28:00 GMT"} {
		if got := head.Header.Get(name); got != want {
			t.Errorf("HEAD %s = %q, want %q", name, got, want)
		}
	}
	for _, r := range h.fake.Requests()[before:] {
		if r.Key != "" && r.Method != http.MethodHead {
			t.Errorf("a HEAD reached Wasabi as %s %s", r.Method, r.Key)
		}
	}
	if missing := h.do(t, http.MethodHead, h.url("/buckets/photos/objects/ghost"), nil); missing.Code != http.StatusNotFound {
		t.Errorf("HEAD of a missing key = %d", missing.Code)
	}
}

// TestCloudflareAnObjectIsSentToWasabiWithItsRealLength: Wasabi refuses a
// chunked PUT (ADR112 § Not verified), so the request that leaves bex-api must
// carry the caller's Content-Length. The fake refuses a chunked body the way
// Wasabi does.
func TestCloudflareAnObjectIsSentToWasabiWithItsRealLength(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	for _, size := range []int{0, 1, 5 << 20} {
		wantOK(t, h.do(t, http.MethodPut, h.url(fmt.Sprintf("/buckets/photos/objects/o%d", size)), make([]byte, size)))
	}
	var puts int
	for _, r := range h.fake.Requests() {
		if r.Method == http.MethodPut && r.Key != "" {
			puts++
			if r.Header.Get("Content-Length") == "" {
				t.Errorf("PUT %s reached Wasabi without a Content-Length", r.Key)
			}
		}
	}
	if puts != 3 {
		t.Fatalf("Wasabi saw %d PUTs, want 3", puts)
	}
}

func TestCloudflareAShortBodyStoresNothing(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	req := httptest.NewRequest(http.MethodPut, h.url("/buckets/photos/objects/short"), io.NopCloser(strings.NewReader("12345")))
	req.ContentLength = 10 // declares ten bytes, delivers five
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":10013`) {
		t.Fatalf("a short body = %d %s, want 400 with 10013", rec.Code, rec.Body)
	}
	wantCF(t, h.do(t, http.MethodGet, h.url("/buckets/photos/objects/short"), nil), http.StatusNotFound, 10007)
}

func TestCloudflareAQuotaBreachNamesTheCap(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	h.repo.bytes = objectstorage.MaxWorkspaceBytes - 3
	r := h.do(t, http.MethodPut, h.url("/buckets/photos/objects/k"), []byte("four"))
	wantCF(t, r, http.StatusForbidden, 10042)
	if !strings.Contains(r.Env.Errors[0].Message, "bytes") || !strings.Contains(r.Env.Errors[0].Message, fmt.Sprint(objectstorage.MaxWorkspaceBytes)) {
		t.Errorf("the refusal must name the cap it hit: %q", r.Env.Errors[0].Message)
	}
	// A write that fits is still taken, and the refused one left nothing.
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/k"), []byte("abc")))
	h.repo.bytes, h.repo.object = 0, objectstorage.MaxWorkspaceObjects
	if r := h.do(t, http.MethodPut, h.url("/buckets/photos/objects/k2"), []byte("a")); r.errCode() != 10042 || !strings.Contains(r.Env.Errors[0].Message, "objects") {
		t.Errorf("the object cap = %d %s", r.Code, r.Raw)
	}
}

func TestCloudflareBucketsAndObjectsOfAnotherWorkspaceAreUnreachable(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/secret.txt"), []byte("mine")))

	other := &cfHarness{srv: h.srv, h: h.h, fake: h.fake, repo: h.repo, svc: h.svc, ws: ids.New(ids.Workspace)}
	wantCF(t, other.do(t, http.MethodGet, other.url("/buckets/photos/objects/secret.txt"), nil), http.StatusNotFound, 10006)
	if list := resultOf[struct{ Buckets []cfBucket }](t, other.do(t, http.MethodGet, other.url("/buckets"), nil)); len(list.Buckets) != 0 {
		t.Fatalf("another workspace lists %d buckets", len(list.Buckets))
	}
	// The same name in another workspace is another bucket with another key prefix.
	other.createBucket(t, "photos")
	wantCF(t, other.do(t, http.MethodGet, other.url("/buckets/photos/objects/secret.txt"), nil), http.StatusNotFound, 10007)
}

// TestCloudflareTheSystemBucketIsSharedWithTheRenderRoutes: an object put with
// wrangler into render-oregon is the one Render's routes list, and the other
// way round (ADR112 D1).
func TestCloudflareTheSystemBucketIsSharedWithTheRenderRoutes(t *testing.T) {
	h := newCFHarness(t)
	// Neither side has touched it yet: wrangler creates it on first use.
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/render-oregon/objects/from-wrangler.txt"), []byte("hi")))
	page, err := h.svc.ListObjects(t.Context(), h.ws, objectstorage.Oregon, "", 100)
	if err != nil || len(page.Items) != 1 || page.Items[0].Key != "from-wrangler.txt" || page.Items[0].SizeBytes != 2 {
		t.Fatalf("Render's list of render-oregon = %+v, %v", page, err)
	}
	// And what the Render side put is readable through wrangler's route.
	put, err := h.svc.MintPut(t.Context(), h.ws, objectstorage.Oregon, "from-render.txt", 3)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(mustPut(put.URL, []byte("abc")))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("uploading to Render's presigned URL: %v %v", resp, err)
	}
	if got := h.do(t, http.MethodGet, h.url("/buckets/render-oregon/objects/from-render.txt"), nil); string(got.Raw) != "abc" {
		t.Fatalf("wrangler's read of an object Render put = %d %q", got.Code, got.Raw)
	}
	// A look-alike that is not a system bucket does not exist.
	wantCF(t, h.do(t, http.MethodGet, h.url("/buckets/render-elsewhere/objects/k"), nil), http.StatusNotFound, 10006)
}

// streamServer is the full chain on a real listener whose own timeouts are
// tiny, the way bex-api's are 30 s: only a handler that lifts them lets a slow
// or long body through.
func streamServer(t *testing.T, h *cfHarness, timeout time.Duration) *httptest.Server {
	t.Helper()
	ts := httptest.NewUnstartedServer(withGzip(h.h))
	ts.Config.ReadTimeout, ts.Config.WriteTimeout = timeout, timeout
	ts.Start()
	t.Cleanup(ts.Close)
	return ts
}

// slowBody delivers n bytes in chunks, pausing between them.
type slowBody struct {
	chunks int
	chunk  []byte
	pause  time.Duration
}

func (s *slowBody) Read(p []byte) (int, error) {
	if s.chunks == 0 {
		return 0, io.EOF
	}
	s.chunks--
	time.Sleep(s.pause)
	return copy(p, s.chunk), nil
}

func TestCloudflareASlowBodyOutlivesTheServerTimeouts(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	ts := streamServer(t, h, 150*time.Millisecond)

	chunk := bytes.Repeat([]byte("x"), 1000)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+h.url("/buckets/photos/objects/slow"), &slowBody{chunks: 8, chunk: chunk, pause: 60 * time.Millisecond})
	req.ContentLength = 8000
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("a body slower than the server's timeouts: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("slow put = %d %s", resp.StatusCode, raw)
	}
	// The same server cuts a route that does not extend its deadlines.
	cut, _ := http.NewRequest(http.MethodPost, ts.URL+h.url("/buckets"), &slowBody{chunks: 8, chunk: []byte(`{"name":"x"}`), pause: 60 * time.Millisecond})
	cut.ContentLength = 96
	if resp, err := http.DefaultClient.Do(cut); err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("the control request was not cut by the server's timeouts: the test proves nothing")
		}
	}

	// A slow download, too: the write deadline is lifted for the stream.
	body := bytes.Repeat([]byte("y"), 1<<20)
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/big"), body))
	dl, err := http.Get(ts.URL + h.url("/buckets/photos/objects/big"))
	if err != nil {
		t.Fatal(err)
	}
	defer dl.Body.Close()
	time.Sleep(300 * time.Millisecond) // longer than the write timeout before reading
	got, err := io.ReadAll(dl.Body)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("a download read after the server's write timeout: %d bytes, %v", len(got), err)
	}
}

// TestCloudflareObjectBytesAreNotCompressed: a client that accepts gzip gets
// the object as stored, with its length, not a gzip of it.
func TestCloudflareObjectBytesAreNotCompressed(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	body := bytes.Repeat([]byte("compressible "), 4096)
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/text.txt"), body, "Content-Type", "text/plain"))
	ts := streamServer(t, h, 5*time.Second)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+h.url("/buckets/photos/objects/text.txt"), nil)
	req.Header.Set("Accept-Encoding", "gzip")
	tr := &http.Transport{DisableCompression: true}
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if resp.Header.Get("Content-Encoding") != "" || !bytes.Equal(got, body) || resp.Header.Get("Content-Length") != fmt.Sprint(len(body)) {
		t.Fatalf("object served with Content-Encoding %q, %d bytes, Content-Length %q", resp.Header.Get("Content-Encoding"), len(got), resp.Header.Get("Content-Length"))
	}
	// A JSON route is still compressed for a client that accepts gzip.
	jr, _ := http.NewRequest(http.MethodGet, ts.URL+h.url("/buckets"), nil)
	jr.Header.Set("Accept-Encoding", "gzip")
	jresp, err := (&http.Client{Transport: tr}).Do(jr)
	if err != nil {
		t.Fatal(err)
	}
	jresp.Body.Close()
	if jresp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("a JSON answer lost its compression: %q", jresp.Header.Get("Content-Encoding"))
	}
}

// zeros is an endless reader of zero bytes, so a 300 MiB body costs no memory
// on the sending side.
type zeros struct{ left int64 }

func (z *zeros) Read(p []byte) (int, error) {
	if z.left <= 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), z.left)
	clear(p[:n])
	z.left -= n
	return int(n), nil
}

// TestCloudflareThreeHundredMiBStreamsWithFlatMemory: the cap is exactly 300
// MiB, the body goes through to Wasabi as it arrives, and the heap does not
// grow with the object. Wasabi is a server that counts and discards, because
// the in-memory fake would hold the whole body itself.
func TestCloudflareThreeHundredMiBStreamsWithFlatMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("moves 300 MiB over loopback")
	}
	var stored atomic.Int64
	wasabi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.ContentLength < 0 {
			http.Error(w, "chunked bodies are refused", http.StatusForbidden)
			return
		}
		n, _ := io.Copy(io.Discard, r.Body)
		stored.Store(n)
		w.Header().Set("ETag", `"discarded"`)
	}))
	t.Cleanup(wasabi.Close)
	st := objectstorage.NewStore(objectstorage.Config{Endpoint: wasabi.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1})
	repo := &memObjectRepo{}
	svc := objectstorage.NewService(repo, map[objectstorage.Region]objectstorage.Store{objectstorage.Oregon: st})
	srv := &Server{CloudflareR2: newCloudflareR2(&core.Base{}, svc)}
	h := &cfHarness{srv: srv, h: srv.cloudflareHandler(cfTestAuth, passThrough), repo: repo, svc: svc, ws: ids.New(ids.Workspace)}
	h.createBucket(t, "big")
	ts := streamServer(t, h, 30*time.Second)

	upload := func(size int64) *http.Response {
		req, _ := http.NewRequest(http.MethodPut, ts.URL+h.url("/buckets/big/objects/blob"), &zeros{left: size})
		req.ContentLength = size
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("uploading %d bytes: %v", size, err)
		}
		return resp
	}

	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var peak atomic.Uint64
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > peak.Load() {
				peak.Store(m.HeapAlloc)
			}
			select {
			case <-done:
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	resp := upload(objectstorage.MaxObjectBytesStreamed)
	close(done)
	wg.Wait()
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || stored.Load() != objectstorage.MaxObjectBytesStreamed {
		t.Fatalf("300 MiB upload = %d, Wasabi stored %d bytes", resp.StatusCode, stored.Load())
	}
	if growth := int64(peak.Load()) - int64(base.HeapAlloc); growth > 64<<20 {
		t.Fatalf("the heap grew by %d MiB moving a 300 MiB object: the body is being buffered", growth>>20)
	}

	over := upload(objectstorage.MaxObjectBytesStreamed + 1)
	over.Body.Close()
	if over.StatusCode != http.StatusBadRequest {
		t.Fatalf("300 MiB + 1 = %d, want 400", over.StatusCode)
	}
}

// TestCloudflareTheNinthConcurrentStreamIsRefused: eight uploads in flight is
// a workspace's limit; the ninth answers 429 with 10058, and a slot freed by a
// finished upload is usable again.
func TestCloudflareTheNinthConcurrentStreamIsRefused(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	ts := streamServer(t, h, 10*time.Second)

	type inflight struct {
		w    *io.PipeWriter
		done chan *http.Response
	}
	start := func(key string) inflight {
		pr, pw := io.Pipe()
		req, _ := http.NewRequest(http.MethodPut, ts.URL+h.url("/buckets/photos/objects/"+key), pr)
		req.ContentLength = 4
		done := make(chan *http.Response, 1)
		go func() {
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("stream %s: %v", key, err)
				done <- nil
				return
			}
			done <- resp
		}()
		_, _ = pw.Write([]byte("ab")) // half the body: the stream is now holding its slot
		return inflight{w: pw, done: done}
	}
	var held []inflight
	// A failed assertion must not leave eight uploads blocked on their pipes,
	// which would hang the server's Close.
	t.Cleanup(func() {
		for _, s := range held {
			_ = s.w.CloseWithError(io.ErrClosedPipe)
		}
	})
	for i := range objectstorage.MaxConcurrentStreamsPerWorkspace {
		held = append(held, start(fmt.Sprintf("s%d", i)))
	}
	// Each held stream has reached Wasabi, which is waiting on the rest of its body.
	waitFor(t, func() bool {
		n := 0
		for _, r := range h.fake.Requests() {
			if r.Method == http.MethodPut && strings.Contains(r.Key, "/") {
				n++
			}
		}
		return n == objectstorage.MaxConcurrentStreamsPerWorkspace
	})

	req, _ := http.NewRequest(http.MethodPut, ts.URL+h.url("/buckets/photos/objects/ninth"), strings.NewReader("abcd"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || !bytes.Contains(raw, []byte(`"code":10058`)) || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("the ninth stream = %d %s, want 429 with 10058 and Retry-After", resp.StatusCode, raw)
	}
	// Another workspace is not held back by this one's streams.
	other := &cfHarness{srv: h.srv, h: h.h, fake: h.fake, repo: h.repo, svc: h.svc, ws: ids.New(ids.Workspace)}
	other.createBucket(t, "photos")
	wantOK(t, other.do(t, http.MethodPut, other.url("/buckets/photos/objects/k"), []byte("ok")))

	for _, s := range held {
		_, _ = s.w.Write([]byte("cd"))
		_ = s.w.Close()
		if r := <-s.done; r != nil {
			r.Body.Close()
			if r.StatusCode != http.StatusOK {
				t.Errorf("a held stream finished with %d", r.StatusCode)
			}
		}
	}
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/after"), []byte("ok")))
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached in 5s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestCloudflareWasabiDownIsAnInternalError: a store that cannot be reached is
// not a missing bucket or a missing key, and no raw error text escapes.
func TestCloudflareWasabiDownIsAnInternalError(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/k"), []byte("x")))
	h.fake.SetDown(true)
	for _, step := range []struct {
		method, suffix string
		body           []byte
	}{
		{http.MethodGet, "/buckets/photos/objects/k", nil},
		{http.MethodPut, "/buckets/photos/objects/k2", []byte("y")},
		{http.MethodDelete, "/buckets/photos", nil},
	} {
		r := h.do(t, step.method, h.url(step.suffix), step.body)
		wantCF(t, r, http.StatusInternalServerError, 10001)
		if strings.Contains(string(r.Raw), h.fake.URL) || strings.Contains(string(r.Raw), "wasabi") {
			t.Errorf("a raw store error reached the customer: %s", r.Raw)
		}
	}
	h.fake.SetDown(false)
}

// usageSink collects what the transfer meters flush.
type usageSink struct {
	mu     sync.Mutex
	totals map[string]int64
}

func (u *usageSink) AddObjectStorageUsage(_ context.Context, deltas []store.ObjectStorageUsageDelta) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.totals == nil {
		u.totals = map[string]int64{}
	}
	for _, d := range deltas {
		u.totals[d.WorkspaceID+" "+d.Kind] += d.Quantity
	}
	return nil
}

// TestCloudflareTransferIsMeteredExactly: a completed put counts its bytes as
// ingress, a get counts the bytes it streamed as egress, and a refused or
// missing one counts nothing (ADR112 D8; shadow meters, never billed).
func TestCloudflareTransferIsMeteredExactly(t *testing.T) {
	h := newCFHarness(t)
	sink := &usageSink{}
	h.svc.SetUsageSink(sink)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.svc.RunMeter(ctx, time.Hour); close(done) }()

	h.createBucket(t, "photos")
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/k"), []byte("12345")))
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/k2"), []byte("123")))
	h.do(t, http.MethodGet, h.url("/buckets/photos/objects/k"), nil)
	h.do(t, http.MethodGet, h.url("/buckets/photos/objects/k"), nil)
	h.do(t, http.MethodGet, h.url("/buckets/photos/objects/ghost"), nil)                // 404: nothing read
	h.do(t, http.MethodPut, h.url("/buckets/photos/objects/bad%5Ckey"), []byte("zzzz")) // refused: nothing sent

	cancel()
	<-done // RunMeter flushes once more when it stops
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if got := sink.totals[h.ws+" "+store.UsageKindObjectStorageIngressBytes]; got != 8 {
		t.Errorf("ingress = %d, want 8 (5 + 3)", got)
	}
	if got := sink.totals[h.ws+" "+store.UsageKindObjectStorageEgressBytes]; got != 10 {
		t.Errorf("egress = %d, want 10 (two reads of 5)", got)
	}
}
