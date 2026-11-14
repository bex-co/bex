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

package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// frontendLoki is a Loki whose query frontend merges a multi-stream read the way
// the real one does across a split boundary (w21/063): when the selector matches
// more than one stream and the window holds more than `limit` lines, it deals
// the page out stream by stream, one line at a time from the requested end, in
// label order, instead of by time. A read of one stream, or of a window that
// fits the limit, is exact.
type frontendLoki struct {
	streams []fakeStream
	srv     *httptest.Server

	mu          sync.Mutex
	rangeCalls  []url.Values
	seriesCalls []url.Values
	seriesCode  int
	seriesExtra int    // streams appended to the series answer that hold no line
	onSeries    func() // runs when a stream listing arrives
}

type fakeStream struct {
	labels map[string]string
	stamps []int64        // ascending
	marked map[int64]bool // stamps whose line reads "MATCH ..."
}

var equalityMatcher = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)=("(?:[^"\\]|\\.)*")`)

func newFrontendLoki(t *testing.T, streams ...fakeStream) *frontendLoki {
	t.Helper()
	f := &frontendLoki{streams: streams}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *frontendLoki) handle(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	f.mu.Lock()
	defer f.mu.Unlock()
	start, _ := strconv.ParseInt(v.Get("start"), 10, 64)
	end, _ := strconv.ParseInt(v.Get("end"), 10, 64)
	switch r.URL.Path {
	case "/loki/api/v1/series":
		f.seriesCalls = append(f.seriesCalls, v)
		if f.onSeries != nil {
			f.onSeries()
		}
		if f.seriesCode != 0 {
			http.Error(w, "boom", f.seriesCode)
			return
		}
		data := []map[string]string{}
		for _, s := range f.streams {
			if len(s.within(start, end)) > 0 {
				data = append(data, s.labels)
			}
		}
		for i := range f.seriesExtra {
			data = append(data, map[string]string{"app": "web", "pod": fmt.Sprintf("ghost-%d", i)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": data})
	case "/loki/api/v1/query_range":
		f.rangeCalls = append(f.rangeCalls, v)
		limit, _ := strconv.Atoi(v.Get("limit"))
		forward := v.Get("direction") == DirectionForward
		var matched []fakeStream
		for _, s := range f.streams {
			if s.matches(v.Get("query")) {
				matched = append(matched, s)
			}
		}
		sort.Slice(matched, func(i, j int) bool { return matched[i].labels["pod"] < matched[j].labels["pod"] })
		type result struct {
			Stream map[string]string `json:"stream"`
			Values [][2]string       `json:"values"`
		}
		var results []result
		total := 0
		for _, s := range matched {
			total += len(s.within(start, end))
		}
		take := make([][]int64, len(matched))
		if len(matched) > 1 && total > limit {
			// The frontend's inexact merge: one line per stream per round.
			next := make([]int, len(matched))
			for n := 0; n < limit; {
				progressed := false
				for i, s := range matched {
					in := s.within(start, end)
					if next[i] >= len(in) || n == limit {
						continue
					}
					if forward {
						take[i] = append(take[i], in[next[i]])
					} else {
						take[i] = append(take[i], in[len(in)-1-next[i]])
					}
					next[i]++
					n++
					progressed = true
				}
				if !progressed {
					break
				}
			}
		} else {
			type picked struct {
				ns     int64
				stream int
			}
			var all []picked
			for i, s := range matched {
				for _, ns := range s.within(start, end) {
					all = append(all, picked{ns, i})
				}
			}
			sort.Slice(all, func(i, j int) bool { return all[i].ns < all[j].ns })
			if len(all) > limit {
				if forward {
					all = all[:limit]
				} else {
					all = all[len(all)-limit:]
				}
			}
			for _, p := range all {
				take[p.stream] = append(take[p.stream], p.ns)
			}
		}
		for i, s := range matched {
			if len(take[i]) == 0 {
				continue
			}
			slices.Sort(take[i])
			res := result{Stream: s.labels}
			for _, ns := range take[i] {
				res.Values = append(res.Values, [2]string{strconv.FormatInt(ns, 10), s.message(ns)})
			}
			results = append(results, res)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "streams", "result": results}})
	default:
		http.NotFound(w, r)
	}
}

// message is the line stored at ns.
func (s fakeStream) message(ns int64) string {
	msg := s.labels["pod"] + "@" + strconv.FormatInt(ns, 10)
	if s.marked[ns] {
		return "MATCH " + msg
	}
	return msg
}

// within is the stream's stamps in [start, end).
func (s fakeStream) within(start, end int64) []int64 {
	var out []int64
	for _, ns := range s.stamps {
		if ns >= start && ns < end {
			out = append(out, ns)
		}
	}
	return out
}

// matches applies a query's equality matchers; a regexp matcher is not modelled.
func (s fakeStream) matches(query string) bool {
	selector, _, _ := strings.Cut(query, "}")
	for _, m := range equalityMatcher.FindAllStringSubmatch(selector, -1) {
		want, err := strconv.Unquote(m[2])
		if err != nil || s.labels[m[1]] != want {
			return false
		}
	}
	return true
}

func podStream(pod string, stamps ...int64) fakeStream {
	return fakeStream{labels: map[string]string{"namespace": "default", "app": "web", "container": "app", "pod": pod}, stamps: stamps}
}

// secondsFrom lists n stamps one second apart, the first at origin + first seconds.
func secondsFrom(origin time.Time, first, n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = origin.Add(time.Duration(first+i) * time.Second).UnixNano()
	}
	return out
}

func entryStamps(entries []LogEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Message
	}
	return out
}

func wantPage(pod string, stamps []int64) []string {
	out := make([]string, len(stamps))
	for i, ns := range stamps {
		out[i] = podStream(pod).message(ns)
	}
	return out
}

var streamsOrigin = time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)

func streamsQuery(direction string) LogQuery {
	return LogQuery{
		App: "web", Direction: direction, Limit: 10,
		Since: streamsOrigin.Add(-time.Hour), End: streamsOrigin.Add(time.Hour),
	}.normalized()
}

// The live probe: a forward page over two instance streams came back as the
// older instance's first lines plus the newer instance's, skipping the older
// one's lines in between, and the cursor chain never returned them.
func TestAForwardFullPageOverTwoStreamsIsTheOldestLinesNotAShareOfEach(t *testing.T) {
	a := secondsFrom(streamsOrigin, 1, 12) // the older instance: 1s..12s
	b := secondsFrom(streamsOrigin, 20, 3) // the newer one: 20s..22s
	f := newFrontendLoki(t, podStream("web-a", a...), podStream("web-b", b...))

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", streamsQuery(DirectionForward))
	if err != nil {
		t.Fatal(err)
	}
	if want := wantPage("web-a", a[:10]); !slices.Equal(entryStamps(got), want) {
		t.Fatalf("forward page = %v\nwant the 10 oldest lines %v", entryStamps(got), want)
	}
}

func TestABackwardFullPageOverTwoStreamsIsTheNewestLinesNotAShareOfEach(t *testing.T) {
	a := secondsFrom(streamsOrigin, 11, 12) // the newer instance: 11s..22s
	b := secondsFrom(streamsOrigin, 1, 3)   // the older one: 1s..3s
	f := newFrontendLoki(t, podStream("web-a", a...), podStream("web-b", b...))

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", streamsQuery(DirectionBackward))
	if err != nil {
		t.Fatal(err)
	}
	if want := wantPage("web-a", a[2:]); !slices.Equal(entryStamps(got), want) {
		t.Fatalf("backward page = %v\nwant the 10 newest lines %v", entryStamps(got), want)
	}
}

// A page short of the limit is exact whatever the frontend did, so it costs one
// request: no stream listing, no per-stream reads.
func TestAPageShortOfTheLimitIsOneRequest(t *testing.T) {
	f := newFrontendLoki(t,
		podStream("web-a", secondsFrom(streamsOrigin, 1, 4)...),
		podStream("web-b", secondsFrom(streamsOrigin, 20, 3)...))

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", streamsQuery(DirectionForward))
	if err != nil || len(got) != 7 {
		t.Fatalf("got %d lines, err %v; want all 7", len(got), err)
	}
	if len(f.rangeCalls) != 1 || len(f.seriesCalls) != 0 {
		t.Fatalf("%d reads and %d stream listings, want 1 and 0", len(f.rangeCalls), len(f.seriesCalls))
	}
}

// A full page whose window holds one stream is that stream's own oldest lines,
// so it costs the page and one stream listing, and no stream read.
func TestAFullPageWithOneStreamInItsWindowIsKept(t *testing.T) {
	a := secondsFrom(streamsOrigin, 1, 25)
	f := newFrontendLoki(t, podStream("web-a", a...))

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", streamsQuery(DirectionForward))
	if err != nil {
		t.Fatal(err)
	}
	if want := wantPage("web-a", a[:10]); !slices.Equal(entryStamps(got), want) {
		t.Fatalf("page = %v, want %v", entryStamps(got), want)
	}
	if len(f.rangeCalls) != 1 || len(f.seriesCalls) != 1 {
		t.Fatalf("%d reads and %d stream listings, want the page and one listing", len(f.rangeCalls), len(f.seriesCalls))
	}
}

// The stream listing covers only the lines the page can hold: every line of a
// suspect page is real, so the oldest `limit` lines end no later than the page's
// newest line.
func TestTheForwardStreamListingEndsAtTheSuspectPagesNewestLine(t *testing.T) {
	f := newFrontendLoki(t,
		podStream("web-a", secondsFrom(streamsOrigin, 1, 12)...),
		podStream("web-b", secondsFrom(streamsOrigin, 20, 3)...))
	if _, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", streamsQuery(DirectionForward)); err != nil {
		t.Fatal(err)
	}
	if len(f.seriesCalls) != 1 {
		t.Fatalf("%d stream listings, want 1", len(f.seriesCalls))
	}
	// The dealt page ends at web-b's 22s line; the end bound is exclusive.
	want := streamsOrigin.Add(22*time.Second).UnixNano() + 1
	if got, _ := strconv.ParseInt(f.seriesCalls[0].Get("end"), 10, 64); got != want {
		t.Fatalf("listing end = %d, want %d", got, want)
	}
}

// ... and the newest `limit` lines begin no earlier than the page's oldest line.
func TestTheBackwardStreamListingStartsAtTheSuspectPagesOldestLine(t *testing.T) {
	f := newFrontendLoki(t,
		podStream("web-a", secondsFrom(streamsOrigin, 11, 12)...),
		podStream("web-b", secondsFrom(streamsOrigin, 1, 3)...))
	if _, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", streamsQuery(DirectionBackward)); err != nil {
		t.Fatal(err)
	}
	if len(f.seriesCalls) != 1 {
		t.Fatalf("%d stream listings, want 1", len(f.seriesCalls))
	}
	// The dealt page begins at web-b's 1s line; the start bound is inclusive.
	want := streamsOrigin.Add(time.Second).UnixNano()
	if got, _ := strconv.ParseInt(f.seriesCalls[0].Get("start"), 10, 64); got != want {
		t.Fatalf("listing start = %d, want %d", got, want)
	}
}

// A stream is pinned inside the query's own selector: its restrictions stay, and
// a stream that lacks a label another listed stream carries is pinned to the
// empty value, so the stream that has the label is not read twice.
func TestEachStreamReadKeepsTheQuerysRestrictionsAndPinsEveryLabel(t *testing.T) {
	older := fakeStream{labels: map[string]string{"namespace": "default", "app": "web", "container": "app", "pod": "web-a"}, stamps: secondsFrom(streamsOrigin, 1, 12)}
	current := fakeStream{labels: map[string]string{"namespace": "default", "app": "web", "app_id": "app-1", "container": "app", "pod": "web-b"}, stamps: secondsFrom(streamsOrigin, 20, 3)}
	f := newFrontendLoki(t, older, current)
	q := streamsQuery(DirectionForward)
	q.appID = "app-1"

	if _, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", q); err != nil {
		t.Fatal(err)
	}
	var pinned []string
	for _, v := range f.rangeCalls[1:] {
		pinned = append(pinned, v.Get("query"))
	}
	sort.Strings(pinned)
	want := []string{
		`{namespace="default", app="web", app_id=~"app-1|", container=~"app|", app="web", app_id="", container="app", namespace="default", pod="web-a"}`,
		`{namespace="default", app="web", app_id=~"app-1|", container=~"app|", app="web", app_id="app-1", container="app", namespace="default", pod="web-b"}`,
	}
	if !slices.Equal(pinned, want) {
		t.Fatalf("per-stream queries =\n%s\nwant\n%s", strings.Join(pinned, "\n"), strings.Join(want, "\n"))
	}
	if got := f.seriesCalls[0].Get("match[]"); got != `{namespace="default", app="web", app_id=~"app-1|", container=~"app|"}` {
		t.Fatalf("the listing matched %s, want the query's own selector", got)
	}
}

// A page that cannot be proven exact is an error, never the inexact page.
func TestAFailedStreamListingFailsTheReadInsteadOfReturningTheInexactPage(t *testing.T) {
	f := newFrontendLoki(t,
		podStream("web-a", secondsFrom(streamsOrigin, 1, 12)...),
		podStream("web-b", secondsFrom(streamsOrigin, 20, 3)...))
	f.seriesCode = http.StatusServiceUnavailable

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", streamsQuery(DirectionForward))
	if err == nil {
		t.Fatalf("got %d lines and no error, want the failed listing surfaced", len(got))
	}
}

func TestAWindowWithTooManyStreamsIsRefusedNotReadInexactly(t *testing.T) {
	f := newFrontendLoki(t,
		podStream("web-a", secondsFrom(streamsOrigin, 1, 12)...),
		podStream("web-b", secondsFrom(streamsOrigin, 20, 3)...))
	f.seriesExtra = lokiMaxStreamReads

	_, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", streamsQuery(DirectionForward))
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeQueryTimeout {
		t.Fatalf("err = %v, want a %s asking the caller to narrow the range", err, core.CodeQueryTimeout)
	}
	if len(f.rangeCalls) != 1 {
		t.Fatalf("%d reads, want only the page: no stream was read", len(f.rangeCalls))
	}
}

// A raw text-search page is the same suspect: its saturation protocol takes the
// page for the window's oldest lines and moves coverage past it, so a match the
// inexact page dropped would never be read.
func TestALocalTextSearchScanReadsAFullRawPageStreamByStream(t *testing.T) {
	older := podStream("web-a", secondsFrom(streamsOrigin, 1, 1200)...)
	older.marked = map[int64]bool{}
	var want []int64
	for _, ns := range older.stamps[1100:1106] { // after the 980 lines the inexact page keeps of this stream
		older.marked[ns] = true
		want = append(want, ns)
	}
	newer := podStream("web-b", secondsFrom(streamsOrigin, 2000, 20)...)
	f := newFrontendLoki(t, older, newer)
	q := LogQuery{
		App: "web", Search: []string{"MATCH"}, Direction: DirectionForward, Limit: 100,
		Since: streamsOrigin.Add(-time.Hour), End: streamsOrigin.Add(time.Hour),
	}.normalized()

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", q)
	if err != nil {
		t.Fatal(err)
	}
	wantMsgs := make([]string, len(want))
	for i, ns := range want {
		wantMsgs[i] = older.message(ns)
	}
	if !slices.Equal(entryStamps(got), wantMsgs) {
		t.Fatalf("matches = %v, want the six marked lines %v", entryStamps(got), wantMsgs)
	}
}

// Shards of one stream differ only in Loki's internal labels: they are one
// stream to read, and one toward the cap.
func TestStreamSelectorsTreatShardsAsOneStreamAndNeverPinInternalLabels(t *testing.T) {
	r := lokiReader{selector: `{namespace="default", app="web"}`}
	got, err := r.streamSelectors([]map[string]string{
		{"namespace": "default", "app": "web", "pod": "web-a", "__stream_shard__": "0"},
		{"namespace": "default", "app": "web", "pod": "web-a", "__stream_shard__": "1"},
		{"namespace": "default", "app": "web", "pod": "web-b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`{namespace="default", app="web", app="web", namespace="default", pod="web-a"}`,
		`{namespace="default", app="web", app="web", namespace="default", pod="web-b"}`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("selectors = %v, want %v", got, want)
	}
}

func TestStreamSelectorsRefuseASelectorThatIsNotBracketed(t *testing.T) {
	r := lokiReader{selector: `namespace="default"`}
	if got, err := r.streamSelectors([]map[string]string{{"pod": "a"}}); err == nil {
		t.Fatalf("selectors = %v, want an error", got)
	}
}

// A caller that leaves mid-read gets its error, never a page cut from the reads
// that happened to finish.
func TestACallerThatLeavesDuringTheStreamReadsGetsAnError(t *testing.T) {
	f := newFrontendLoki(t,
		podStream("web-a", secondsFrom(streamsOrigin, 1, 12)...),
		podStream("web-b", secondsFrom(streamsOrigin, 20, 3)...))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.onSeries = cancel

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(ctx, "default", streamsQuery(DirectionForward))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %d lines and err %v, want context.Canceled", len(got), err)
	}
}
