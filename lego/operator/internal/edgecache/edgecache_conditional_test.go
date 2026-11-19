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

package edgecache

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// w3/m488. The cache stripped a client's validators before the fill and
// always wrote the stored page in full, so turning caching on turned an app's
// 304s into full 200s, bytes the plan's included bandwidth pays for, and no
// answer from the cache said how old it was.

const lastModified = "Tue, 17 Nov 2026 10:00:00 GMT"

// page is an origin serving a cacheable stylesheet with the given ETag (none
// when empty) and lastModified.
func page(etag string) *origin {
	header := http.Header{
		"Cache-Control":    {"max-age=600"},
		"Content-Type":     {"text/css"},
		"Content-Encoding": {"identity"},
		"Last-Modified":    {lastModified},
	}
	if etag != "" {
		header.Set("ETag", etag)
	}
	return &origin{header: header}
}

// assertNotModified checks a 304 the way RFC 9110 §15.4.5 shapes it: no body
// and no representation metadata, but the validators and caching headers a
// 200 would carry.
func assertNotModified(t *testing.T, w *httptest.ResponseRecorder, cacheStatus, etag string) {
	t.Helper()
	if w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("got %d with %d body bytes, want an empty 304", w.Code, w.Body.Len())
	}
	h := w.Header()
	if got := h.Get(StatusHeader); got != cacheStatus {
		t.Errorf("%s = %q, want %q", StatusHeader, got, cacheStatus)
	}
	if h.Get("ETag") != etag || h.Get("Cache-Control") != "max-age=600" {
		t.Errorf("304 ETag %q Cache-Control %q, want %q and the page's", h.Get("ETag"), h.Get("Cache-Control"), etag)
	}
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Encoding"} {
		if v := h.Get(name); v != "" {
			t.Errorf("304 carries %s: %q", name, v)
		}
	}
	// Last-Modified guides the client's update only when there is no ETag.
	if got := h.Get("Last-Modified"); (etag == "") != (got == lastModified) {
		t.Errorf("304 Last-Modified %q with ETag %q", got, etag)
	}
}

func TestAHitAnswersAMatchingValidatorWith304(t *testing.T) {
	const later, earlier = "Wed, 18 Nov 2026 10:00:00 GMT", "Mon, 16 Nov 2026 10:00:00 GMT"
	for _, tc := range []struct {
		name, method, etag string
		header             map[string]string
		want               int
	}{
		{"its ETag", "GET", `"v1"`, map[string]string{"If-None-Match": `"v1"`}, 304},
		{"a weak validator for a strong ETag", "GET", `"v1"`, map[string]string{"If-None-Match": `W/"v1"`}, 304},
		{"a strong validator for a weak ETag", "GET", `W/"v1"`, map[string]string{"If-None-Match": `"v1"`}, 304},
		{"its ETag in a list", "GET", `"v1"`, map[string]string{"If-None-Match": `"v0", W/"v1"`}, 304},
		{"*", "GET", `"v1"`, map[string]string{"If-None-Match": "*"}, 304},
		{"an ETag holding a comma", "GET", `"a,b"`, map[string]string{"If-None-Match": `"a,b"`}, 304},
		{"HEAD", "HEAD", `"v1"`, map[string]string{"If-None-Match": `"v1"`}, 304},
		{"another ETag", "GET", `"v1"`, map[string]string{"If-None-Match": `"v0"`}, 200},
		{"half of an ETag holding a comma", "GET", `"a,b"`, map[string]string{"If-None-Match": `"a"`}, 200},
		{"an unquoted tag", "GET", `"v1"`, map[string]string{"If-None-Match": "v1"}, 200},
		{"a page without an ETag", "GET", "", map[string]string{"If-None-Match": `"v1"`}, 200},
		{"If-Modified-Since its Last-Modified", "GET", `"v1"`, map[string]string{"If-Modified-Since": lastModified}, 304},
		{"If-Modified-Since later", "GET", "", map[string]string{"If-Modified-Since": later}, 304},
		{"If-Modified-Since earlier", "GET", `"v1"`, map[string]string{"If-Modified-Since": earlier}, 200},
		{"If-Modified-Since not a date", "GET", `"v1"`, map[string]string{"If-Modified-Since": "yesterday"}, 200},
		{"If-Modified-Since beside a non-matching If-None-Match", "GET", `"v1"`, map[string]string{"If-None-Match": `"v0"`, "If-Modified-Since": later}, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := page(tc.etag)
			h := newCache(t, ProfileAll, o, nil)
			get(h, "GET", "http://app.test/x.css", nil)
			w := get(h, tc.method, "http://app.test/x.css", tc.header)
			if o.hits.Load() != 1 {
				t.Fatalf("origin hits = %d, want 1: the conditional is the cache's to answer", o.hits.Load())
			}
			if tc.want == http.StatusNotModified {
				assertNotModified(t, w, statusHit, tc.etag)
				return
			}
			wantBody := "body:/x.css"
			if tc.method == "HEAD" {
				wantBody = ""
			}
			if w.Code != tc.want || w.Body.String() != wantBody || w.Header().Get(StatusHeader) != statusHit || w.Header().Get("Content-Type") != "text/css" {
				t.Fatalf("got %d %s %q, want the full page as a HIT", w.Code, w.Header().Get(StatusHeader), w.Body.String())
			}
		})
	}
}

func TestOnlyAStored2xxIsAnsweredWith304(t *testing.T) {
	o := &origin{status: http.StatusNotFound, header: http.Header{"ETag": {`"v1"`}}}
	h := newCache(t, ProfileAll, o, nil)
	get(h, "GET", "http://app.test/missing", nil)
	w := get(h, "GET", "http://app.test/missing", map[string]string{"If-None-Match": "*"})
	if w.Code != http.StatusNotFound || w.Body.String() != "body:/missing" || w.Header().Get(StatusHeader) != statusHit {
		t.Fatalf("a stored 404 asked If-None-Match: * = %d %q, want the 404 itself (RFC 9110 §13.2.1)", w.Code, w.Body.String())
	}
}

func TestTheFillAnswersTheLeadersMatchingValidatorWith304AndStoresThePage(t *testing.T) {
	clock := time.Unix(1_800_000_000, 0)
	now := func() time.Time { return clock }
	o := page(`"v1"`)
	h := newCache(t, ProfileAll, o, now)
	matching := map[string]string{"If-None-Match": `"v1"`}

	assertNotModified(t, get(h, "GET", "http://app.test/x.css", matching), statusMiss, `"v1"`)
	if o.validators.Load() != 0 {
		t.Fatal("the fill must not send the client's validators upstream: the stored body has to suit every client")
	}
	if w := get(h, "GET", "http://app.test/x.css", nil); w.Code != http.StatusOK || w.Body.String() != "body:/x.css" || w.Header().Get(StatusHeader) != statusHit {
		t.Fatalf("the GET after a 304 fill = %d %s %q, want the full page as a HIT", w.Code, w.Header().Get(StatusHeader), w.Body.String())
	}

	clock = clock.Add(601 * time.Second)
	assertNotModified(t, get(h, "GET", "http://app.test/x.css", map[string]string{"If-Modified-Since": lastModified}), statusExpired, `"v1"`)
	if w := get(h, "GET", "http://app.test/x.css", nil); w.Body.String() != "body:/x.css" || w.Header().Get(StatusHeader) != statusHit {
		t.Fatalf("the GET after an EXPIRED 304 fill = %s %q, want the refreshed page as a HIT", w.Header().Get(StatusHeader), w.Body.String())
	}
	if o.hits.Load() != 2 {
		t.Fatalf("origin hits = %d, want 2 (the fill and the refill)", o.hits.Load())
	}

	other := newCache(t, ProfileAll, page(`"v1"`), nil)
	if w := get(other, "GET", "http://app.test/x.css", map[string]string{"If-None-Match": `"v0"`}); w.Code != http.StatusOK || w.Body.String() != "body:/x.css" || w.Header().Get(StatusHeader) != statusMiss {
		t.Fatalf("a fill for a stale validator = %d %s %q, want the full page as a MISS", w.Code, w.Header().Get(StatusHeader), w.Body.String())
	}
}

func TestAPageTheCacheDoesNotKeepStillAnswersAMatchingValidatorWith304(t *testing.T) {
	// Rails' default for every page: revalidate each time, with an ETag the
	// app answers 304 for. The cache keeps none of it, and must not cost the
	// client the 304 the app would have sent.
	o := &origin{header: http.Header{"Cache-Control": {"max-age=0, private, must-revalidate"}, "ETag": {`W/"v1"`}}}
	h := newCache(t, ProfileAll, o, nil)
	w := get(h, "GET", "http://app.test/dashboard", map[string]string{"If-None-Match": `W/"v1"`})
	if w.Code != http.StatusNotModified || w.Body.Len() != 0 || w.Header().Get(StatusHeader) != statusBypass {
		t.Fatalf("a matching validator on a private page = %d %s with %d body bytes, want an empty BYPASS 304", w.Code, w.Header().Get(StatusHeader), w.Body.Len())
	}
	if w := get(h, "GET", "http://app.test/dashboard", nil); w.Code != http.StatusOK || w.Body.String() != "body:/dashboard" || w.Header().Get(StatusHeader) == statusHit {
		t.Fatalf("the next GET = %d %s %q, want the page from the app, never stored", w.Code, w.Header().Get(StatusHeader), w.Body.String())
	}
}

func TestAFillTooLargeToStoreAnswers304AndStoresNothing(t *testing.T) {
	big := bytes.Repeat([]byte("x"), 1<<17) // twice the test cache's object cap
	var hits atomic.Int64
	h := newCacheOver(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("ETag", `"big"`)
		w.Header().Set("Cache-Control", "max-age=600")
		// Unannounced length: only reading the body finds it too large.
		for chunk := range slices.Chunk(big, 4096) {
			_, _ = w.Write(chunk)
			w.(http.Flusher).Flush()
		}
	}), Config{Profile: ProfileAll, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	w := get(h, "GET", "http://app.test/big.zip", map[string]string{"If-None-Match": `"big"`})
	if w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("got %d with %d body bytes, want an empty 304", w.Code, w.Body.Len())
	}
	w = get(h, "GET", "http://app.test/big.zip", nil)
	if w.Header().Get(StatusHeader) != statusMiss || w.Body.Len() != len(big) || hits.Load() != 2 {
		t.Fatalf("the next GET = %s with %d bytes after %d origin hits, want the whole body as a MISS", w.Header().Get(StatusHeader), w.Body.Len(), hits.Load())
	}
}

func TestA304FillStopsReadingABodyItCannotStore(t *testing.T) {
	// The leader is sent none of the body, so nothing past the object cap is
	// worth reading: a long stream must not hold the fill for its whole length.
	const total = 64 << 20
	var written atomic.Int64
	done := make(chan struct{})
	h := newCacheOver(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		defer close(done)
		w.Header().Set("ETag", `"stream"`)
		w.Header().Set("Cache-Control", "max-age=600")
		chunk := bytes.Repeat([]byte("x"), 32<<10)
		for written.Load() < total {
			if _, err := w.Write(chunk); err != nil {
				return
			}
			written.Add(int64(len(chunk)))
		}
	}), Config{Profile: ProfileAll, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	if w := get(h, "GET", "http://app.test/stream.bin", map[string]string{"If-None-Match": `"stream"`}); w.Code != http.StatusNotModified {
		t.Fatalf("got %d, want 304", w.Code)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the origin is still writing after the 304")
	}
	if written.Load() >= total {
		t.Fatalf("the origin wrote all %d bytes for a 304 whose body could not be stored", total)
	}
}

func TestRequestsWaitingOnAFillAnswerTheirValidatorsFromTheStoredPage(t *testing.T) {
	o := page(`"v1"`)
	o.delay = 50 * time.Millisecond
	h := newCache(t, ProfileAll, o, nil)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		statuses = map[string]int{}
	)
	for range 6 {
		wg.Go(func() {
			w := get(h, "GET", "http://app.test/x.css", map[string]string{"If-None-Match": `"v1"`})
			if w.Code != http.StatusNotModified || w.Body.Len() != 0 {
				t.Errorf("got %d with %d body bytes, want an empty 304", w.Code, w.Body.Len())
			}
			mu.Lock()
			statuses[w.Header().Get(StatusHeader)]++
			mu.Unlock()
		})
	}
	wg.Wait()
	if o.hits.Load() != 1 || statuses[statusMiss] != 1 || statuses[statusHit] != 5 {
		t.Fatalf("origin hits %d, statuses %v; want one MISS fill and five HITs", o.hits.Load(), statuses)
	}
}

func TestA304CrossesARealServerAndKeepsTheConnection(t *testing.T) {
	o := page(`"v1"`)
	srv := httptest.NewServer(newCache(t, ProfileAll, o, nil))
	t.Cleanup(srv.Close)
	client := srv.Client()
	do := func(path, inm string) (*http.Response, []byte) {
		t.Helper()
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		if inm != "" {
			req.Header.Set("If-None-Match", inm)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp, body
	}
	for _, step := range []struct {
		path, inm, cacheStatus string
		code                   int
	}{
		{"/a.css", `"v1"`, statusMiss, http.StatusNotModified}, // the fill
		{"/a.css", `"v1"`, statusHit, http.StatusNotModified},
		{"/a.css", "", statusHit, http.StatusOK},
		{"/b.css", "", statusMiss, http.StatusOK},
		{"/b.css", `"v1"`, statusHit, http.StatusNotModified},
	} {
		resp, body := do(step.path, step.inm)
		if resp.StatusCode != step.code || resp.Header.Get(StatusHeader) != step.cacheStatus {
			t.Fatalf("GET %s If-None-Match %q = %d %s, want %d %s", step.path, step.inm, resp.StatusCode, resp.Header.Get(StatusHeader), step.code, step.cacheStatus)
		}
		if step.code == http.StatusNotModified && (len(body) != 0 || resp.Header.Get("Content-Length") != "") {
			t.Fatalf("a 304 on the wire carried %d body bytes, Content-Length %q", len(body), resp.Header.Get("Content-Length"))
		}
		if step.code == http.StatusOK && !strings.HasPrefix(string(body), "body:") {
			t.Fatalf("a 200 on the wire carried %q", body)
		}
	}
	if o.hits.Load() != 2 {
		t.Fatalf("origin hits = %d, want one fill per path", o.hits.Load())
	}
}

func TestEveryAnswerFromAStoredPageCarriesItsAge(t *testing.T) {
	clock := time.Unix(1_800_000_000, 0)
	now := func() time.Time { return clock }
	age := func(w *httptest.ResponseRecorder) []string { return w.Header().Values("Age") }

	o := page(`"v1"`)
	o.header.Set("Age", "30") // an upstream that is itself a cache
	h := newCache(t, ProfileAll, o, now)
	if got := age(get(h, "GET", "http://app.test/x.css", nil)); len(got) != 1 || got[0] != "30" {
		t.Fatalf("the fill's Age = %v, want the upstream's own 30", got)
	}
	clock = clock.Add(5900 * time.Millisecond)
	for name, header := range map[string]map[string]string{"a HIT": nil, "a 304": {"If-None-Match": `"v1"`}} {
		if got := age(get(h, "GET", "http://app.test/x.css", header)); len(got) != 1 || got[0] != "35" {
			t.Errorf("%s 5.9s after storing a page of Age 30 carries Age %v, want [35]", name, got)
		}
	}

	h = newCache(t, ProfileAll, page(`"v1"`), now)
	get(h, "GET", "http://app.test/x.css", nil)
	if got := age(get(h, "HEAD", "http://app.test/x.css", nil)); len(got) != 1 || got[0] != "0" {
		t.Fatalf("a HIT at once = Age %v, want [0]", got)
	}
	clock = clock.Add(12 * time.Second)
	if got := age(get(h, "GET", "http://app.test/x.css", nil)); len(got) != 1 || got[0] != "12" {
		t.Fatalf("a HIT 12s after storing = Age %v, want [12]", got)
	}
}

func TestUpstreamAgeIsWholeSeconds(t *testing.T) {
	for value, want := range map[string]time.Duration{
		"":            0,
		" 7 ":         7 * time.Second,
		"-5":          0,
		"+5":          0,
		"1.5":         0,
		"soon":        0,
		"99999999999": (1<<31 - 1) * time.Second, // RFC 9111 §1.2.2
	} {
		if got := upstreamAge(http.Header{"Age": {value}}); got != want {
			t.Errorf("Age %q = %s, want %s", value, got, want)
		}
	}
}
