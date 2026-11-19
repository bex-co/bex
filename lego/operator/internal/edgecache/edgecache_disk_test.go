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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// w3/m489. A service's cache tier keeps bodies on its pod's bounded disk, so
// it can hold far more, and far larger objects, than a sidecar's memory; only
// the metadata stays in memory, under its own bound.

// sized serves a cacheable body of the size its query names, every byte the
// last of its path.
func sized(hits *atomic.Int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		n, _ := strconv.Atoi(r.URL.Query().Get("size"))
		w.Header().Set("Cache-Control", "max-age=600")
		_, _ = w.Write(bytes.Repeat([]byte{r.URL.Path[len(r.URL.Path)-1]}, n))
	})
}

func newDiskCache(t *testing.T, upstream http.Handler, cfg Config) (*Handler, string) {
	t.Helper()
	if cfg.Dir == "" {
		cfg.Dir = t.TempDir()
	}
	if cfg.Profile == "" {
		cfg.Profile = ProfileAll
	}
	return newCacheOver(t, upstream, cfg), cfg.Dir
}

// files lists what dir holds.
func files(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// storedFile is the body file of the entry stored for target, or "".
func storedFile(h *Handler, target string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if el, ok := h.entries[cacheKey(httptest.NewRequest("GET", target, nil))]; ok {
		return el.Value.(*entry).file
	}
	return ""
}

func TestDiskModeStoresTheBodyAsAFileAndServesHitsFromIt(t *testing.T) {
	var hits atomic.Int64
	h, dir := newDiskCache(t, sized(&hits), Config{MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	const target = "http://app.test/a.css?size=10000"
	want := bytes.Repeat([]byte("s"), 10000)
	if w := get(h, "GET", target, nil); w.Header().Get(StatusHeader) != statusMiss || !bytes.Equal(w.Body.Bytes(), want) {
		t.Fatalf("the fill = %s with %d bytes, want the whole body as a MISS", w.Header().Get(StatusHeader), w.Body.Len())
	}
	stored := files(t, dir)
	if len(stored) != 1 {
		t.Fatalf("%s holds %v, want the one body file", dir, stored)
	}
	if info, err := os.Stat(filepath.Join(dir, stored[0])); err != nil || info.Size() != 10000 {
		t.Fatalf("the body file: %v, %v; want 10000 bytes", info, err)
	}
	h.mu.Lock()
	for el := h.lru.Front(); el != nil; el = el.Next() {
		if e := el.Value.(*entry); e.body != nil || e.file != filepath.Join(dir, stored[0]) {
			t.Errorf("the entry holds %d body bytes in memory and file %q", len(e.body), e.file)
		}
	}
	if h.size != 3*diskBlock || h.index <= 0 {
		t.Errorf("charged %d disk bytes and %d index bytes, want the file's three blocks and its metadata", h.size, h.index)
	}
	h.mu.Unlock()

	w := get(h, "GET", target, nil)
	if w.Header().Get(StatusHeader) != statusHit || !bytes.Equal(w.Body.Bytes(), want) || w.Header().Get("Content-Length") != "10000" {
		t.Fatalf("the HIT = %s with %d bytes, Content-Length %q", w.Header().Get(StatusHeader), w.Body.Len(), w.Header().Get("Content-Length"))
	}
	if w := get(h, "HEAD", target, nil); w.Header().Get(StatusHeader) != statusHit || w.Body.Len() != 0 || w.Header().Get("Content-Length") != "10000" {
		t.Fatalf("the HEAD = %s with %d bytes, Content-Length %q", w.Header().Get(StatusHeader), w.Body.Len(), w.Header().Get("Content-Length"))
	}
	if hits.Load() != 1 {
		t.Fatalf("origin hits = %d, want 1", hits.Load())
	}
}

func TestDiskModeEvictsByDiskBytesFromTheColdEnd(t *testing.T) {
	// Each 10000-byte body takes three blocks: the cache holds two.
	h, dir := newDiskCache(t, sized(nil), Config{MaxBytes: 2*3*diskBlock + 100, MaxIndexBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	a, b, c := "http://app.test/a?size=10000", "http://app.test/b?size=10000", "http://app.test/c?size=10000"
	get(h, "GET", a, nil)
	get(h, "GET", b, nil)
	get(h, "GET", a, nil) // b is now the least recently used
	evicted := storedFile(h, b)
	get(h, "GET", c, nil)
	if got := files(t, dir); len(got) != 2 || slices.Contains(got, filepath.Base(evicted)) {
		t.Fatalf("%s holds %v, want two files and not the evicted %s", dir, got, filepath.Base(evicted))
	}
	for _, tc := range []struct{ target, want string }{{a, statusHit}, {c, statusHit}, {b, statusMiss}} {
		if s := get(h, "GET", tc.target, nil).Header().Get(StatusHeader); s != tc.want {
			t.Errorf("%s = %s, want %s", tc.target, s, tc.want)
		}
	}
	if h.size > h.cfg.MaxBytes {
		t.Fatalf("charged %d disk bytes, over the %d bound", h.size, h.cfg.MaxBytes)
	}
}

func TestDiskModeEvictsByIndexBytesFromTheColdEnd(t *testing.T) {
	probe, _ := newDiskCache(t, sized(nil), Config{MaxBytes: 1 << 30, MaxObjectBytes: 1 << 16})
	get(probe, "GET", "http://app.test/a?size=100", nil)
	one := probe.index
	if one < entryOverhead {
		t.Fatalf("an entry's metadata is charged %d, want at least its structure's %d", one, entryOverhead)
	}

	bound := 2*one + one/2
	h, dir := newDiskCache(t, sized(nil), Config{MaxBytes: 1 << 30, MaxIndexBytes: bound, MaxObjectBytes: 1 << 16})
	get(h, "GET", "http://app.test/a?size=100", nil)
	evicted := storedFile(h, "http://app.test/a?size=100")
	get(h, "GET", "http://app.test/b?size=100", nil)
	get(h, "GET", "http://app.test/c?size=100", nil)
	if got := files(t, dir); len(got) != 2 || slices.Contains(got, filepath.Base(evicted)) {
		t.Fatalf("%s holds %v, want two files and not the evicted %s", dir, got, filepath.Base(evicted))
	}
	if h.index > bound {
		t.Fatalf("charged %d index bytes, over the %d bound", h.index, bound)
	}
	if s := get(h, "GET", "http://app.test/a?size=100", nil).Header().Get(StatusHeader); s != statusMiss {
		t.Fatalf("the coldest entry after the index filled = %s, want MISS", s)
	}
}

func TestDiskModeABodyOverTheObjectCapIsServedButNeverStored(t *testing.T) {
	big := bytes.Repeat([]byte("x"), 1<<17) // twice the object cap
	for name, announce := range map[string]bool{"announced": true, "unannounced": false} {
		t.Run(name, func(t *testing.T) {
			var hits atomic.Int64
			h, dir := newDiskCache(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.Header().Set("Cache-Control", "max-age=600")
				if announce {
					w.Header().Set("Content-Length", strconv.Itoa(len(big)))
				}
				for chunk := range slices.Chunk(big, 4096) {
					_, _ = w.Write(chunk)
					w.(http.Flusher).Flush()
				}
			}), Config{MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
			for range 2 {
				// An announced length over the cap is refused before the body,
				// and reported as the memory cache reports it.
				w := get(h, "GET", "http://app.test/big.zip", nil)
				if s := w.Header().Get(StatusHeader); s == statusHit || s == "" || !bytes.Equal(w.Body.Bytes(), big) {
					t.Fatalf("got %s with %d bytes, want the whole body from the app", s, w.Body.Len())
				}
				if got := files(t, dir); len(got) != 0 {
					t.Fatalf("%s holds %v after a body over the cap", dir, got)
				}
			}
			if hits.Load() != 2 {
				t.Fatalf("origin hits = %d, want 2", hits.Load())
			}
		})
	}
}

func TestDiskModeAPurgeRemovesEveryFile(t *testing.T) {
	h, dir := newDiskCache(t, sized(nil), Config{MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	for _, p := range []string{"a", "b", "c"} {
		get(h, "GET", "http://app.test/"+p+"?size=500", nil)
	}
	if got := files(t, dir); len(got) != 3 {
		t.Fatalf("%s holds %v, want three body files", dir, got)
	}
	h.Purge()
	if got := files(t, dir); len(got) != 0 {
		t.Fatalf("%s holds %v after the purge", dir, got)
	}
	if h.size != 0 || h.index != 0 {
		t.Fatalf("charged %d disk and %d index bytes after the purge", h.size, h.index)
	}
	if s := get(h, "GET", "http://app.test/a?size=500", nil).Header().Get(StatusHeader); s != statusMiss {
		t.Fatalf("after the purge = %s, want MISS", s)
	}
}

func TestDiskModeAFillInFlightAcrossAPurgeLeavesNoFile(t *testing.T) {
	half := bytes.Repeat([]byte("p"), 8192)
	release := make(chan struct{})
	var hits atomic.Int64
	h, dir := newDiskCache(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "max-age=600")
		_, _ = w.Write(half)
		w.(http.Flusher).Flush()
		if hits.Add(1) == 1 {
			<-release
		}
		_, _ = w.Write(half)
	}), Config{MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})

	var wg sync.WaitGroup
	var first *httptest.ResponseRecorder
	wg.Go(func() { first = get(h, "GET", "http://app.test/slow.css", nil) })
	deadline := time.Now().Add(5 * time.Second)
	for len(files(t, dir)) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the fill never started its temporary file")
		}
		time.Sleep(time.Millisecond)
	}
	h.Purge()
	close(release)
	wg.Wait()
	if first.Header().Get(StatusHeader) != statusMiss || first.Body.Len() != 2*len(half) {
		t.Fatalf("the fill's own client got %s with %d bytes, want the whole body as a MISS", first.Header().Get(StatusHeader), first.Body.Len())
	}
	if got := files(t, dir); len(got) != 0 {
		t.Fatalf("%s holds %v after a fill that crossed a purge", dir, got)
	}
	if s := get(h, "GET", "http://app.test/slow.css", nil).Header().Get(StatusHeader); s != statusMiss {
		t.Fatalf("after the purge = %s, want MISS: a pre-purge response was stored", s)
	}
	if s := get(h, "GET", "http://app.test/slow.css", nil).Header().Get(StatusHeader); s != statusHit {
		t.Fatalf("the post-purge fill = %s, want HIT", s)
	}
}

// A restarted cache process cannot know which purges it missed, so whatever a
// previous one left on the volume goes.
func TestNewStartsEmptyOnADirectoryHoldingLeftovers(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fill-123", "stale", filepath.Join("nested", "deeper")} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	o := &origin{status: http.StatusOK}
	h, _ := newDiskCache(t, o, Config{Dir: dir, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	if got := files(t, dir); len(got) != 0 {
		t.Fatalf("%s holds %v after New, want it emptied", dir, got)
	}
	if s := status(h, "/app.css"); s != statusMiss {
		t.Fatalf("the first request = %s, want MISS", s)
	}

	missing := filepath.Join(t.TempDir(), "not", "yet")
	newDiskCache(t, o, Config{Dir: missing, MaxBytes: 1 << 20})
	if info, err := os.Stat(missing); err != nil || !info.IsDir() {
		t.Fatalf("New left no directory at %s: %v", missing, err)
	}
}

func TestDiskModeABodyThatWillNotOpenIsAMiss(t *testing.T) {
	var hits atomic.Int64
	h, dir := newDiskCache(t, sized(&hits), Config{MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	get(h, "GET", "http://app.test/a.css?size=100", nil)
	for _, name := range files(t, dir) {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	w := get(h, "GET", "http://app.test/a.css?size=100", nil)
	if w.Code != http.StatusOK || w.Header().Get(StatusHeader) != statusMiss || w.Body.Len() != 100 {
		t.Fatalf("with its file gone = %d %s with %d bytes, want the page refetched as a MISS", w.Code, w.Header().Get(StatusHeader), w.Body.Len())
	}
	if s := get(h, "GET", "http://app.test/a.css?size=100", nil).Header().Get(StatusHeader); s != statusHit || hits.Load() != 2 {
		t.Fatalf("after the refill = %s with %d origin hits, want a HIT after 2", s, hits.Load())
	}
}

// stallingWriter holds its first Write until release closes.
type stallingWriter struct {
	*httptest.ResponseRecorder
	once             sync.Once
	started, release chan struct{}
}

func (s *stallingWriter) Write(p []byte) (int, error) {
	s.once.Do(func() {
		close(s.started)
		<-s.release
	})
	return s.ResponseRecorder.Write(p)
}

func TestDiskModeAHitEvictedWhileItStreamsDeliversTheWholeBody(t *testing.T) {
	const size = 256 << 10
	h, dir := newDiskCache(t, sized(nil), Config{MaxBytes: size + size/2, MaxObjectBytes: size})
	a, b := fmt.Sprintf("http://app.test/a.bin?size=%d", size), fmt.Sprintf("http://app.test/b.bin?size=%d", size)
	get(h, "GET", a, nil)
	evicted := storedFile(h, a)

	w := &stallingWriter{ResponseRecorder: httptest.NewRecorder(), started: make(chan struct{}), release: make(chan struct{})}
	var wg sync.WaitGroup
	wg.Go(func() { h.ServeHTTP(w, httptest.NewRequest("GET", a, nil)) })
	<-w.started
	get(h, "GET", b, nil) // room for one: a is evicted while its HIT streams
	if got := files(t, dir); len(got) != 1 || got[0] == filepath.Base(evicted) {
		t.Fatalf("%s holds %v, want only b's file once a is evicted", dir, got)
	}
	close(w.release)
	wg.Wait()
	if w.Header().Get(StatusHeader) != statusHit || !bytes.Equal(w.Body.Bytes(), bytes.Repeat([]byte("n"), size)) {
		t.Fatalf("the HIT = %s with %d bytes, want the whole %d-byte body", w.Header().Get(StatusHeader), w.Body.Len(), size)
	}
}

func TestDiskModeAnswers304sFromTheFillAndTheStoredPage(t *testing.T) {
	o := page(`"v1"`)
	o.status = http.StatusOK
	h, dir := newDiskCache(t, o, Config{MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	matching := map[string]string{"If-None-Match": `"v1"`}
	assertNotModified(t, get(h, "GET", "http://app.test/x.css", matching), statusMiss, `"v1"`)
	if got := files(t, dir); len(got) != 1 {
		t.Fatalf("%s holds %v after a 304 fill, want the page's body file", dir, got)
	}
	if w := get(h, "GET", "http://app.test/x.css", nil); w.Code != http.StatusOK || w.Body.String() != "body:/x.css" || w.Header().Get(StatusHeader) != statusHit {
		t.Fatalf("the GET after a 304 fill = %d %s %q, want the full page as a HIT", w.Code, w.Header().Get(StatusHeader), w.Body.String())
	}
	assertNotModified(t, get(h, "GET", "http://app.test/x.css", matching), statusHit, `"v1"`)
	if o.hits.Load() != 1 {
		t.Fatalf("origin hits = %d, want 1", o.hits.Load())
	}

	big := bytes.Repeat([]byte("x"), 1<<17)
	h, dir = newDiskCache(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"big"`)
		w.Header().Set("Cache-Control", "max-age=600")
		for chunk := range slices.Chunk(big, 4096) {
			_, _ = w.Write(chunk)
			w.(http.Flusher).Flush()
		}
	}), Config{MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	if w := get(h, "GET", "http://app.test/big.zip", map[string]string{"If-None-Match": `"big"`}); w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("got %d with %d body bytes, want an empty 304", w.Code, w.Body.Len())
	}
	if got := files(t, dir); len(got) != 0 {
		t.Fatalf("%s holds %v after a 304 fill too large to store", dir, got)
	}
}
