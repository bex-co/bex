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
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// pendingBytes is what the meter has counted and not yet reported.
func (e *gwEnv) pendingBytes() int64 {
	e.handler.Meter.mu.Lock()
	defer e.handler.Meter.mu.Unlock()
	return e.handler.Meter.pending[e.bucket.WorkspaceID]
}

func (e *gwEnv) counter(origin responseOrigin) float64 {
	return testutil.ToFloat64(e.handler.Metrics.responseBytes.WithLabelValues(string(origin)))
}

func (e *gwEnv) requests(result requestResult) float64 {
	return testutil.ToFloat64(e.handler.Metrics.requests.WithLabelValues(string(result)))
}

// TestTheBytesOutOfARequestAreCountedExactly: what the visitor received, from
// Wasabi or from the cache, whole or partial, and nothing for a HEAD, a 304 or an
// error.
func TestTheBytesOutOfARequestAreCountedExactly(t *testing.T) {
	e := newGW(t)
	small := randomBytes(t, 3000)
	big := randomBytes(t, CacheMaxObjectBytes+777)
	e.put("small", small, objectstorage.ObjectHeaders{})
	e.put("big", big, objectstorage.ObjectHeaders{})

	step := func(name string, rec *httptest.ResponseRecorder, wasabi, cache int) {
		t.Helper()
		if int(e.counter(originWasabi)) != wasabi || int(e.counter(originCache)) != cache {
			t.Errorf("%s: counted wasabi %v and cache %v, want %d and %d (answered %d with %d bytes)",
				name, e.counter(originWasabi), e.counter(originCache), wasabi, cache, rec.Code, rec.Body.Len())
		}
		if int(e.pendingBytes()) != wasabi+cache {
			t.Errorf("%s: the meter holds %d, want %d", name, e.pendingBytes(), wasabi+cache)
		}
	}
	step("a miss, filled into the cache", e.do(http.MethodGet, "/small"), 3000, 0)
	step("a cache hit", e.do(http.MethodGet, "/small"), 3000, 3000)
	step("a ranged cache hit", e.do(http.MethodGet, "/small", "Range", "bytes=0-99"), 3000, 3100)
	step("a streamed object", e.do(http.MethodGet, "/big"), 3000+len(big), 3100)
	step("a ranged stream", e.do(http.MethodGet, "/big", "Range", "bytes=10-19"), 3010+len(big), 3100)
	etag := e.do(http.MethodGet, "/small").Header().Get("ETag") // another hit: +3000 cache
	step("a 304", e.do(http.MethodGet, "/small", "If-None-Match", etag), 3000+len(big)+10, 6100)
	step("a HEAD", e.do(http.MethodHead, "/big"), 3000+len(big)+10, 6100)
	step("a 404", e.do(http.MethodGet, "/nope"), 3000+len(big)+10, 6100)
	step("a 405", e.do(http.MethodPost, "/small"), 3000+len(big)+10, 6100)
	step("a refused 416", e.do(http.MethodGet, "/big", "Range", "bytes=999999999-"), 3000+len(big)+10, 6100)

	for result, want := range map[requestResult]float64{resultOK: 6, resultNotModified: 1, resultNotFound: 1, resultMethodNotAllowed: 1, resultRangeNotSatisfiable: 1} {
		if e.requests(result) < want {
			t.Errorf("requests{result=%s} = %v, want at least %v", result, e.requests(result), want)
		}
	}
}

func TestTheMeterReportsPerWorkspaceAndKeepsWhatFailed(t *testing.T) {
	reporter := &flakyReporter{}
	m := NewMeter(reporter)
	m.Add("tea-a", 100)
	m.Add("tea-b", 50)
	m.Add("tea-a", 25)
	m.Add("", 999)     // no workspace: nothing to attribute
	m.Add("tea-c", 0)  // nothing sent
	m.Add("tea-c", -5) // never negative
	reporter.fail = true
	if err := m.Flush(context.Background()); err == nil {
		t.Fatal("a failed report should be an error")
	}
	m.Add("tea-a", 5)
	reporter.fail = false
	if err := m.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(reporter.batches) != 1 || reporter.batches[0]["tea-a"] != 130 || reporter.batches[0]["tea-b"] != 50 || len(reporter.batches[0]) != 2 {
		t.Errorf("reported %v, want tea-a 130 and tea-b 50 once: a failed report is kept, not lost or doubled", reporter.batches)
	}
	if err := m.Flush(context.Background()); err != nil || len(reporter.batches) != 1 {
		t.Errorf("an empty flush reported again: %v %v", err, reporter.batches)
	}
}

type flakyReporter struct {
	mu      sync.Mutex
	fail    bool
	batches []map[string]int64
}

func (f *flakyReporter) Report(_ context.Context, counts map[string]int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("bex-api is unreachable")
	}
	cp := map[string]int64{}
	for k, v := range counts {
		cp[k] = v
	}
	f.batches = append(f.batches, cp)
	return nil
}

func TestTheMeterIsBoundedWhileBexAPIIsAway(t *testing.T) {
	m := NewMeter(&flakyReporter{fail: true})
	for i := range MaxReportWorkspaces + 50 {
		m.Add("tea-"+strings.Repeat("x", 3)+string(rune('a'+i%26))+string(rune('a'+(i/26)%26))+string(rune('a'+(i/676)%26)), 1)
	}
	if len(m.pending) > MaxReportWorkspaces {
		t.Errorf("%d workspaces held, bound %d", len(m.pending), MaxReportWorkspaces)
	}
	// A workspace already held still counts: undercounting is for new keys only.
	var held string
	for k := range m.pending {
		held = k
		break
	}
	before := m.pending[held]
	m.Add(held, 7)
	if m.pending[held] != before+7 {
		t.Error("a held workspace stopped counting once the bound was reached")
	}
}

func TestTheHTTPReporterPostsTheCountsWithTheBearer(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		if r.Method != http.MethodPost || r.URL.Path != EgressPath {
			http.Error(w, "wrong route", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"accepted":1}`))
	}))
	defer srv.Close()
	rep := &HTTPReporter{APIClient{URL: srv.URL, Token: "t0ken"}}
	if err := rep.Report(context.Background(), map[string]int64{"tea-a": 42}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer t0ken" || gotBody != `{"bytes":{"tea-a":42}}` {
		t.Errorf("sent %q %q", gotAuth, gotBody)
	}
	srv.Close()
	if err := rep.Report(context.Background(), map[string]int64{"tea-a": 1}); err == nil {
		t.Error("an unreachable bex-api should be an error")
	}
}
