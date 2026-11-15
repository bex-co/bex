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

package objectgw

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

// TestAPublicObjectIsServedByteIdentical: small objects are read through the
// cache path and large ones streamed, and either way the bytes, the stored
// Content-Type and Cache-Control, and the validators are what was stored.
func TestAPublicObjectIsServedByteIdentical(t *testing.T) {
	for _, size := range []int{0, 1, 1000, 300 << 10, CacheMaxObjectBytes, CacheMaxObjectBytes + 1, 3 << 20} {
		t.Run(fmt.Sprintf("%d bytes", size), func(t *testing.T) {
			e := newGW(t)
			body := randomBytes(t, size)
			e.put("photos/cat.bin", body, objectstorage.ObjectHeaders{ContentType: "image/png", CacheControl: "public, max-age=600", ContentLanguage: "en"})
			rec := e.do(http.MethodGet, "/photos/cat.bin")
			if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), body) {
				t.Fatalf("GET = %d with %d bytes, want 200 with the %d stored", rec.Code, rec.Body.Len(), size)
			}
			h := rec.Header()
			if h.Get("Content-Type") != "image/png" || h.Get("Cache-Control") != "public, max-age=600" || h.Get("Content-Language") != "en" {
				t.Errorf("stored headers not served: %v", h)
			}
			if h.Get("Content-Length") != strconv.Itoa(size) || h.Get("ETag") == "" || h.Get("Last-Modified") == "" {
				t.Errorf("length or validators missing: %v", h)
			}
		})
	}
}

// TestObjectKeysAreServedAsStored: S3 keys are opaque, so a doubled slash and
// unicode reach the object that was stored under exactly that key.
func TestObjectKeysAreServedAsStored(t *testing.T) {
	e := newGW(t)
	e.put("a//b", []byte("double"), objectstorage.ObjectHeaders{})
	e.put("héllo wörld.txt", []byte("unicode"), objectstorage.ObjectHeaders{})
	e.put("trailing/", []byte("dir"), objectstorage.ObjectHeaders{})
	for target, want := range map[string]string{
		"/a//b": "double", "/h%C3%A9llo%20w%C3%B6rld.txt": "unicode", "/trailing/": "dir",
	} {
		if rec := e.do(http.MethodGet, target); rec.Code != http.StatusOK || rec.Body.String() != want {
			t.Errorf("GET %s = %d %q, want %q", target, rec.Code, rec.Body, want)
		}
	}
}

func TestRangeRequestsAreServedPartial(t *testing.T) {
	// A small object takes the cache path (the gateway evaluates the range) and a
	// large one is passed to Wasabi; both answer the same.
	for name, size := range map[string]int{"small": 5000, "large": CacheMaxObjectBytes + 5000} {
		t.Run(name, func(t *testing.T) {
			e := newGW(t)
			body := randomBytes(t, size)
			e.put("o", body, objectstorage.ObjectHeaders{ContentType: "application/octet-stream"})
			if name == "small" {
				e.do(http.MethodGet, "/o") // fill the cache; the ranged read below is a hit
			}
			for _, c := range []struct {
				rng        string
				start, end int
			}{
				{"bytes=0-9", 0, 9}, {"bytes=100-199", 100, 199}, {fmt.Sprintf("bytes=%d-", size-10), size - 10, size - 1},
				{"bytes=-25", size - 25, size - 1}, {fmt.Sprintf("bytes=%d-%d", size-5, size+500), size - 5, size - 1},
			} {
				rec := e.do(http.MethodGet, "/o", "Range", c.rng)
				want := body[c.start : c.end+1]
				if rec.Code != http.StatusPartialContent || !bytes.Equal(rec.Body.Bytes(), want) {
					t.Fatalf("%s: %d with %d bytes, want 206 with %d", c.rng, rec.Code, rec.Body.Len(), len(want))
				}
				if got, wantCR := rec.Header().Get("Content-Range"), fmt.Sprintf("bytes %d-%d/%d", c.start, c.end, size); got != wantCR {
					t.Errorf("%s: Content-Range = %q, want %q", c.rng, got, wantCR)
				}
				if rec.Header().Get("Content-Length") != strconv.Itoa(len(want)) {
					t.Errorf("%s: Content-Length = %q for %d bytes", c.rng, rec.Header().Get("Content-Length"), len(want))
				}
			}
			// A range past the end is 416, with a plain body and never the store's XML.
			rec := e.do(http.MethodGet, "/o", "Range", fmt.Sprintf("bytes=%d-", size+10))
			if rec.Code != http.StatusRequestedRangeNotSatisfiable || strings.Contains(rec.Body.String(), "<") {
				t.Fatalf("an unsatisfiable range = %d %q, want 416 with no XML", rec.Code, rec.Body)
			}
		})
	}
}

// TestIfRangeServesTheWholeObject: Wasabi does not take If-Range, and a ranged
// answer for an object that changed would corrupt a resumed download, so the
// gateway serves the whole object whenever the header is present.
func TestIfRangeServesTheWholeObject(t *testing.T) {
	e := newGW(t)
	body := randomBytes(t, CacheMaxObjectBytes+100) // large: not served from the cache
	e.put("o", body, objectstorage.ObjectHeaders{})
	rec := e.do(http.MethodGet, "/o", "Range", "bytes=0-9", "If-Range", `"anything"`)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), body) {
		t.Fatalf("Range with If-Range = %d with %d bytes, want 200 with the whole object", rec.Code, rec.Body.Len())
	}
}

// TestConditionalRequests: a validator the visitor holds is checked, by Wasabi
// for what is not cached and by the gateway for what is, and answers 304 or 412
// with no body either way.
func TestConditionalRequests(t *testing.T) {
	for name, size := range map[string]int{"cached": 2000, "streamed": CacheMaxObjectBytes + 2000} {
		t.Run(name, func(t *testing.T) {
			e := newGW(t)
			e.put("o", randomBytes(t, size), objectstorage.ObjectHeaders{CacheControl: "max-age=60"})
			first := e.do(http.MethodGet, "/o")
			etag, modified := first.Header().Get("ETag"), first.Header().Get("Last-Modified")
			if etag == "" || modified == "" {
				t.Fatalf("no validators: %v", first.Header())
			}
			if name == "cached" {
				e.do(http.MethodGet, "/o") // make the next one a hit
			}
			for _, c := range []struct {
				name    string
				headers []string
				want    int
			}{
				{"If-None-Match matches", []string{"If-None-Match", etag}, 304},
				{"If-None-Match does not", []string{"If-None-Match", `"other"`}, 200},
				{"If-Modified-Since the same time", []string{"If-Modified-Since", modified}, 304},
				{"If-Modified-Since long ago", []string{"If-Modified-Since", "Mon, 02 Jan 2006 15:04:05 GMT"}, 200},
				{"If-Match matches", []string{"If-Match", etag}, 200},
				{"If-Match does not", []string{"If-Match", `"other"`}, 412},
				{"If-Unmodified-Since long ago", []string{"If-Unmodified-Since", "Mon, 02 Jan 2006 15:04:05 GMT"}, 412},
			} {
				rec := e.do(http.MethodGet, "/o", c.headers...)
				if rec.Code != c.want {
					t.Errorf("%s: %d, want %d", c.name, rec.Code, c.want)
				}
				if c.want != 200 && strings.Contains(rec.Body.String(), "<") {
					t.Errorf("%s: the store's XML reached the visitor: %q", c.name, rec.Body)
				}
				if c.want == 304 && (rec.Body.Len() != 0 || rec.Header().Get("ETag") != etag) {
					t.Errorf("%s: a 304 carries no body and keeps the ETag: %q %v", c.name, rec.Body, rec.Header())
				}
			}
		})
	}
}

func TestHeadAnswersHeadersAndNoBody(t *testing.T) {
	for name, size := range map[string]int{"cached": 3000, "streamed": CacheMaxObjectBytes + 3000} {
		t.Run(name, func(t *testing.T) {
			e := newGW(t)
			e.put("o", randomBytes(t, size), objectstorage.ObjectHeaders{ContentType: "text/plain"})
			if name == "cached" {
				e.do(http.MethodGet, "/o")
			}
			rec := e.do(http.MethodHead, "/o")
			if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
				t.Fatalf("HEAD = %d with %d body bytes", rec.Code, rec.Body.Len())
			}
			if rec.Header().Get("Content-Length") != strconv.Itoa(size) || rec.Header().Get("Content-Type") != "text/plain" {
				t.Errorf("HEAD headers: %v", rec.Header())
			}
			if rec := e.do(http.MethodHead, "/missing"); rec.Code != http.StatusNotFound {
				t.Errorf("HEAD of a missing object = %d, want 404", rec.Code)
			}
		})
	}
}

// TestAStoredEncodingIsNotDecoded: an object stored gzipped is served as stored,
// with its Content-Encoding, on both paths.
func TestAStoredEncodingIsNotDecoded(t *testing.T) {
	for name, size := range map[string]int{"cached": 100, "streamed": CacheMaxObjectBytes + 100} {
		t.Run(name, func(t *testing.T) {
			e := newGW(t)
			body := append([]byte{0x1f, 0x8b, 0x08}, randomBytes(t, size)...) // gzip magic, not a valid stream
			e.put("o.gz", body, objectstorage.ObjectHeaders{ContentType: "text/css", ContentEncoding: "gzip"})
			rec := e.do(http.MethodGet, "/o.gz")
			if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), body) || rec.Header().Get("Content-Encoding") != "gzip" {
				t.Fatalf("GET = %d, encoding %q, %d bytes; want the stored bytes untouched", rec.Code, rec.Header().Get("Content-Encoding"), rec.Body.Len())
			}
		})
	}
}

// TestNothingOfTheStoresIsRelayedButTheObjectsHeaders: no x-amz header and no
// request id reaches a visitor.
func TestNothingOfTheStoresIsRelayedButTheObjectsHeaders(t *testing.T) {
	e := newGW(t)
	e.put("o", randomBytes(t, CacheMaxObjectBytes+1), objectstorage.ObjectHeaders{ContentType: "text/plain"})
	rec := e.do(http.MethodGet, "/o")
	for name := range rec.Header() {
		switch http.CanonicalHeaderKey(name) {
		case "Content-Type", "Content-Length", "Etag", "Last-Modified", "X-Content-Type-Options", "Accept-Ranges", "Date":
		default:
			t.Errorf("unexpected header %q: %v", name, rec.Header()[name])
		}
	}
}

// TestWhatIsNotAPublicObjectIs404: a host that is not a public bucket's (private,
// deleted, unknown or malformed), the root, and every key that is not valid
// answer 404, and none of them reaches Wasabi for anything but a valid key.
func TestWhatIsNotAPublicObjectIs404(t *testing.T) {
	e := newGW(t)
	e.put("secret/own.txt", []byte("mine"), objectstorage.ObjectHeaders{})

	t.Run("hosts", func(t *testing.T) {
		before := e.resolver.lookups()
		for name, host := range map[string]string{
			"unknown bucket":    "pub-" + strings.Repeat("0", 20) + "." + testBaseDomain,
			"not a bucket":      "www." + testBaseDomain,
			"the apex":          testBaseDomain,
			"empty host":        "",
			"wrong shape":       "pub-short." + testBaseDomain,
			"id of a workspace": "pub-" + strings.TrimPrefix(e.bucket.WorkspaceID, "tea-") + "tea." + testBaseDomain,
		} {
			if rec := e.request(host, http.MethodGet, "/secret/own.txt"); rec.Code != http.StatusNotFound {
				t.Errorf("%s (%q): %d, want 404", name, host, rec.Code)
			}
		}
		// Only a well-formed bucket host costs a lookup (the unknown bucket's).
		if got := e.resolver.lookups() - before; got > 1 {
			t.Errorf("%d lookups for hosts that are not bucket hosts, want at most the one well-formed guess", got)
		}
	})

	t.Run("private and deleted", func(t *testing.T) {
		// bex-api answers both as not found, and the gateway turns that into 404.
		e.resolver.mu.Lock()
		delete(e.resolver.buckets, e.host)
		e.resolver.mu.Unlock()
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPost} {
			if rec := e.do(method, "/secret/own.txt", "Origin", "https://x.example", "Access-Control-Request-Method", "GET"); rec.Code != http.StatusNotFound {
				t.Errorf("%s on a private bucket: %d, want 404", method, rec.Code)
			}
		}
	})
}

func TestTheRootAndInvalidKeysNeverListOrEscape(t *testing.T) {
	e := newGW(t)
	e.put("a/b", []byte("ok"), objectstorage.ObjectHeaders{})
	other := newGW(t) // another workspace's bucket in its own fake
	other.put("a/b", []byte("theirs"), objectstorage.ObjectHeaders{})

	before := len(e.fake.Requests())
	for _, target := range []string{
		"/", "//", "/?list-type=2", "/?prefix=a/", "/?versions", "/?uploads",
		"/../" + e.bucket.WorkspaceID + "/" + e.bucket.ID + "/a/b", "/%2e%2e/x", "/%2E%2E/x", "/a/../a/b", "/a/./b", "/./a",
		"/a/%2e%2e/b", "/%2e/x", "//a/b", "/a%00b", "/a%0Ab", "/a%5Cb", "/" + strings.Repeat("k", 1025),
	} {
		rec := e.do(http.MethodGet, target)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %q = %d %q, want 404", target, rec.Code, rec.Body)
		}
	}
	if got := len(e.fake.Requests()); got != before {
		t.Errorf("%d requests reached Wasabi for roots and invalid keys, want 0: %v", got-before, e.wasabiKeys()[before:])
	}
	e.noListing()
	// A query on a real key is ignored, never passed on.
	rec := e.do(http.MethodGet, "/a/b?versionId=x&response-content-disposition=attachment&list-type=2")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" || rec.Header().Get("Content-Disposition") != "" {
		t.Errorf("GET with a query = %d %q %v, want the object and nothing the query asked for", rec.Code, rec.Body, rec.Header())
	}
	for _, r := range e.fake.Requests() {
		if r.Query.Has("versionId") || r.Query.Has("list-type") || r.Query.Has("response-content-disposition") {
			t.Errorf("a visitor's query reached Wasabi: %v", r.Query)
		}
	}
}

// TestHostsAreCaseInsensitive: DNS is, so the same bucket answers whichever way
// the visitor's client capitalizes the name.
func TestHostsAreCaseInsensitive(t *testing.T) {
	e := newGW(t)
	e.put("o", []byte("x"), objectstorage.ObjectHeaders{})
	for _, host := range []string{strings.ToUpper(e.host), e.host + ":443", e.host + "."} {
		if rec := e.request(host, http.MethodGet, "/o"); rec.Code != http.StatusOK {
			t.Errorf("host %q = %d, want 200", host, rec.Code)
		}
	}
}

func TestOtherMethodsAre405AndOnlyForARealBucket(t *testing.T) {
	e := newGW(t)
	e.put("o", []byte("x"), objectstorage.ObjectHeaders{})
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, "PROPFIND", "TRACE"} {
		rec := e.do(method, "/o")
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, OPTIONS" {
			t.Errorf("%s = %d (Allow %q), want 405 naming GET, HEAD, OPTIONS", method, rec.Code, rec.Header().Get("Allow"))
		}
		// An unknown host is 404 whatever the method: nothing about it is revealed.
		if rec := e.request("pub-"+strings.Repeat("1", 20)+"."+testBaseDomain, method, "/o"); rec.Code != http.StatusNotFound {
			t.Errorf("%s on an unknown host = %d, want 404", method, rec.Code)
		}
	}
	// Nothing was written or deleted.
	for _, r := range e.fake.Requests() {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPut {
			t.Errorf("a %s reached Wasabi", r.Method)
		}
	}
	if keys := e.fake.Keys(objectstorage.Oregon.Bucket(), ""); len(keys) != 1 {
		t.Errorf("the bucket holds %v after the writes were refused", keys)
	}
}

func TestOriginFailures(t *testing.T) {
	e := newGW(t)
	if rec := e.do(http.MethodGet, "/missing"); rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "<") {
		t.Errorf("a missing object = %d %q, want a plain 404", rec.Code, rec.Body)
	}
	e.fake.SetDown(true)
	rec := e.do(http.MethodGet, "/anything")
	if rec.Code != http.StatusBadGateway && rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Wasabi down = %d, want 502 or 503", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "fake") || strings.Contains(rec.Body.String(), "<") {
		t.Errorf("the store's own error text reached the visitor: %q", rec.Body)
	}
	// The gateway's own lookup failing is 503 with Retry-After, never 404 (that
	// would be cached by a visitor's browser as "this bucket does not exist").
	e.resolver.mu.Lock()
	e.resolver.err = io.ErrUnexpectedEOF
	e.resolver.mu.Unlock()
	if rec := e.do(http.MethodGet, "/o"); rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Errorf("a failed lookup = %d (Retry-After %q), want 503 with one", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestNosniffOnEveryAnswer(t *testing.T) {
	e := newGW(t, objectstorage.CORSRule{Allowed: objectstorage.CORSAllowed{Origins: []string{"*"}, Methods: []string{"GET"}}})
	e.put("small", []byte("x"), objectstorage.ObjectHeaders{ContentType: "text/html"})
	e.put("big", randomBytes(t, CacheMaxObjectBytes+1), objectstorage.ObjectHeaders{ContentType: "text/html"})
	for name, rec := range map[string]interface{ Header() http.Header }{
		"cached object":     e.do(http.MethodGet, "/small"),
		"cache hit":         e.do(http.MethodGet, "/small"),
		"streamed object":   e.do(http.MethodGet, "/big"),
		"partial":           e.do(http.MethodGet, "/big", "Range", "bytes=0-4"),
		"not modified":      e.do(http.MethodGet, "/small", "If-None-Match", e.do(http.MethodGet, "/small").Header().Get("ETag")),
		"unsatisfiable":     e.do(http.MethodGet, "/big", "Range", "bytes=99999999-"),
		"missing":           e.do(http.MethodGet, "/nope"),
		"root":              e.do(http.MethodGet, "/"),
		"method":            e.do(http.MethodPost, "/small"),
		"preflight":         e.do(http.MethodOptions, "/small", "Origin", "https://a.example", "Access-Control-Request-Method", "GET"),
		"denied preflight":  e.do(http.MethodOptions, "/small", "Origin", "https://a.example", "Access-Control-Request-Method", "PUT"),
		"unknown host":      e.request("nope.example", http.MethodGet, "/"),
		"unknown bucket id": e.request("pub-"+strings.Repeat("2", 20)+"."+testBaseDomain, http.MethodGet, "/x"),
	} {
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", name, got)
		}
	}
}

// TestNoObjectIsReadFromAnotherBucketsPrefix: two buckets with the same key never
// see each other's bytes, because the key is built from the resolved ids.
func TestNoObjectIsReadFromAnotherBucketsPrefix(t *testing.T) {
	e := newGW(t)
	e.put("same", []byte("mine"), objectstorage.ObjectHeaders{})
	// The same key in another workspace's and another bucket's prefix, in the same store.
	e.fake.Put(objectstorage.Oregon.Bucket(), "tea-other/obk-other/same", []byte("not yours"))
	e.fake.Put(objectstorage.Oregon.Bucket(), e.bucket.WorkspaceID+"/obk-"+strings.Repeat("3", 20)+"/same", []byte("sibling"))
	if rec := e.do(http.MethodGet, "/same"); rec.Body.String() != "mine" {
		t.Fatalf("GET = %q, want mine", rec.Body)
	}
	if rec := e.do(http.MethodGet, "/"+e.bucket.ID+"/same"); rec.Code != http.StatusNotFound {
		t.Errorf("a path naming the bucket id = %d, want 404", rec.Code)
	}
}

// deadlineWriter is a ResponseWriter that takes write deadlines, as the server's
// does, and records them.
type deadlineWriter struct {
	http.ResponseWriter
	deadlines []time.Time
}

func (d *deadlineWriter) SetWriteDeadline(t time.Time) error {
	d.deadlines = append(d.deadlines, t)
	return nil
}

// TestAStreamsWriteDeadlineFollowsTheReader: a streamed response is not cut at
// the server's 120 s while its reader keeps taking bytes, but a reader that stops
// is cut a minute later, and nothing outlives the hard stop.
func TestAStreamsWriteDeadlineFollowsTheReader(t *testing.T) {
	rec := httptest.NewRecorder()
	dw := &deadlineWriter{ResponseWriter: rec}
	w := &countingWriter{ResponseWriter: dw}
	before := time.Now()
	w.stream()
	if len(dw.deadlines) != 1 || dw.deadlines[0].Before(before.Add(WriteStall-time.Second)) || dw.deadlines[0].After(before.Add(WriteStall+5*time.Second)) {
		t.Fatalf("the first deadline is %v, want about %s ahead", dw.deadlines, WriteStall)
	}
	// Writes inside the first half of the window do not touch the deadline.
	for range 20 {
		_, _ = w.Write([]byte("x"))
	}
	if len(dw.deadlines) != 1 {
		t.Errorf("%d deadline updates for 20 small writes; a fast stream must not set a timer per write", len(dw.deadlines))
	}
	// Past half of it, a write pushes the deadline out again.
	w.deadline = time.Now().Add(WriteStall/2 - time.Second)
	_, _ = w.Write([]byte("x"))
	if len(dw.deadlines) != 2 || !dw.deadlines[1].After(dw.deadlines[0]) {
		t.Errorf("deadlines %v: a reader still taking bytes must get more time", dw.deadlines)
	}
	// Never past the hard stop.
	w.hardStop = time.Now().Add(10 * time.Second)
	w.deadline = time.Time{}
	_, _ = w.Write([]byte("x"))
	if last := dw.deadlines[len(dw.deadlines)-1]; last.After(w.hardStop) {
		t.Errorf("deadline %s is past the hard stop %s", last, w.hardStop)
	}
	if StreamDeadline < 10*time.Minute || WriteStall >= StreamDeadline {
		t.Errorf("StreamDeadline %s, WriteStall %s", StreamDeadline, WriteStall)
	}
	// A writer that takes no deadlines (the recorder) must not stop a response.
	plainRec := httptest.NewRecorder()
	pw := &countingWriter{ResponseWriter: plainRec}
	pw.stream()
	if _, err := pw.Write([]byte("ok")); err != nil || plainRec.Body.String() != "ok" {
		t.Errorf("a writer without deadlines: %v %q", err, plainRec.Body)
	}
}

// TestMissingKeysReuseTheConnectionToWasabi: probing keys that do not exist costs
// one connection to Wasabi, not a TLS handshake each. It guards the pool, not the
// drain: Go reuses a connection whose small error body is closed unread, and
// would not reuse one whose body is larger than the drain reads.
func TestMissingKeysReuseTheConnectionToWasabi(t *testing.T) {
	e := newGW(t)
	fresh := 0
	trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
		if !info.Reused {
			fresh++
		}
	}}
	for i := range 8 {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/missing-%d", i), nil)
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
		req.Host = e.host
		rec := httptest.NewRecorder()
		e.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET = %d", rec.Code)
		}
	}
	if fresh > 1 {
		t.Errorf("8 requests for missing keys opened %d connections to Wasabi, want 1", fresh)
	}
}

func TestAFailedPreconditionIsCountedAsOne(t *testing.T) {
	e := newGW(t)
	e.put("o", randomBytes(t, CacheMaxObjectBytes+1), objectstorage.ObjectHeaders{})
	if rec := e.do(http.MethodGet, "/o", "If-Match", `"nope"`); rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("= %d", rec.Code)
	}
	if got := e.requests(resultPreconditionFailed); got != 1 {
		t.Errorf("requests{result=precondition_failed} = %v, want 1: a 412 is not an ok", got)
	}
	if got := e.requests(resultOK); got != 0 {
		t.Errorf("requests{result=ok} = %v, want 0", got)
	}
}

// noTypeOrigin answers every read with a 200 that names no Content-Type, which
// Wasabi never does (it types an untyped object binary/octet-stream) but a
// misbehaving store could.
type noTypeOrigin struct{ body string }

func (o noTypeOrigin) Fetch(_ context.Context, _ objectstorage.Region, _, method string, _ http.Header) (*http.Response, error) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, ContentLength: int64(len(o.body)), Body: io.NopCloser(strings.NewReader(o.body))}
	if method == http.MethodHead {
		resp.Body = http.NoBody
	}
	return resp, nil
}

// TestAnObjectWithNoContentTypeIsNotSniffed: the gateway never lets the standard
// library guess a type from a tenant's bytes; what the store did not say, the
// gateway answers as an opaque download.
func TestAnObjectWithNoContentTypeIsNotSniffed(t *testing.T) {
	e := newGW(t)
	e.handler.Origin = noTypeOrigin{body: "<html><script>alert(1)</script></html>"}
	if got := e.do(http.MethodGet, "/page").Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", got)
	}
	if got := e.do(http.MethodGet, "/page").Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("a cached read: Content-Type = %q", got)
	}
}

// TestAnUntypedObjectIsServedAsTheStoreTypesIt: Wasabi types an object stored
// with no Content-Type as binary/octet-stream, and the gateway relays that.
func TestAnUntypedObjectIsServedAsTheStoreTypesIt(t *testing.T) {
	e := newGW(t)
	e.fake.Put(objectstorage.Oregon.Bucket(), e.bucket.WorkspaceID+"/"+e.bucket.ID+"/page", []byte("<html><script>alert(1)</script></html>"))
	if got := e.do(http.MethodGet, "/page").Header().Get("Content-Type"); got != "binary/octet-stream" {
		t.Errorf("Content-Type = %q, want binary/octet-stream", got)
	}
}
