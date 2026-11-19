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
	"maps"
	"math/bits"
	"math/rand/v2"
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
)

// frontendLoki is a Loki whose query frontend merges a multi-stream read the way
// the real one does across a split boundary (w21/063): when the window crosses
// a split cut (every lokiSplitBucket from the epoch, production's 15 minutes),
// the selector matches more than one stream and the window holds more than
// `limit` lines, it deals the page out stream by stream, one line at a time
// from the requested end, in label order, instead of by time. A read of one
// stream, of a window that fits the limit, or of a window inside one split
// bucket (which Loki hands the querier uncut, w5/m503) is exact.
type frontendLoki struct {
	streams []fakeStream
	srv     *httptest.Server

	mu          sync.Mutex
	rangeCalls  []url.Values
	rangeLines  []int // lines each query_range answered
	seriesCalls []url.Values
	onRange     func()      // runs when a query_range arrives
	rangeCode   map[int]int // the status the n-th query_range (from 1) fails with
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
		// Recorded so a test can say no read listed streams (w5/m503 replaced
		// the per-stream re-read of w21/063).
		f.seriesCalls = append(f.seriesCalls, v)
		data := []map[string]string{}
		for _, s := range f.streams {
			if len(s.within(start, end)) > 0 {
				data = append(data, s.labels)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": data})
	case "/loki/api/v1/query_range":
		f.rangeCalls = append(f.rangeCalls, v)
		if f.onRange != nil {
			f.onRange()
		}
		if code := f.rangeCode[len(f.rangeCalls)]; code != 0 {
			f.rangeLines = append(f.rangeLines, 0)
			http.Error(w, "boom", code)
			return
		}
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
		crossesCut := end > splitBucketStart(time.Unix(0, start)).Add(lokiSplitBucket).UnixNano()
		if crossesCut && len(matched) > 1 && total > limit {
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
		answered := 0
		for i := range matched {
			answered += len(take[i])
		}
		f.rangeLines = append(f.rangeLines, answered)
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

// streamsQuery reads the two hours around streamsOrigin.
func streamsQuery(direction string) LogQuery {
	return LogQuery{
		App: "web", Direction: direction, Limit: 10,
		Since: streamsOrigin.Add(-time.Hour), End: streamsOrigin.Add(time.Hour),
	}.normalized()
}

// wideStreamsQuery reads two days either side of streamsOrigin: a full page's
// narrowed window then spans hundreds of split buckets.
func wideStreamsQuery(direction string) LogQuery {
	q := streamsQuery(direction)
	q.Since, q.End = streamsOrigin.Add(-48*time.Hour), streamsOrigin.Add(48*time.Hour)
	return q
}

// readWindow is one query_range call's [start, end) and limit.
func readWindow(v url.Values) (start, end time.Time, limit int) {
	s, _ := strconv.ParseInt(v.Get("start"), 10, 64)
	e, _ := strconv.ParseInt(v.Get("end"), 10, 64)
	limit, _ = strconv.Atoi(v.Get("limit"))
	return time.Unix(0, s).UTC(), time.Unix(0, e).UTC(), limit
}

// fullReadsAcrossACut lists the query_range windows that crossed a split cut
// and came back a full page: the reads whose page fetch cannot trust.
func (f *frontendLoki) fullReadsAcrossACut() []string {
	var out []string
	for i, v := range f.rangeCalls {
		start, end, limit := readWindow(v)
		if !oneBucket(start, end) && f.rangeLines[i] >= limit {
			out = append(out, v.Get("start")+".."+v.Get("end"))
		}
	}
	return out
}

// The live probe: a forward page over two instance streams came back as the
// older instance's first lines plus the newer instance's, skipping the older
// one's lines in between, and the cursor chain never returned them.
//
// The page is right however far the narrowed window reaches: a few buckets
// (the page's own two hours) or hundreds (four days).
func TestAForwardFullPageOverTwoStreamsIsTheOldestLinesNotAShareOfEach(t *testing.T) {
	for name, query := range map[string]func(string) LogQuery{"two hours": streamsQuery, "four days": wideStreamsQuery} {
		t.Run(name, func(t *testing.T) {
			a := secondsFrom(streamsOrigin, 1, 12) // the older instance: 1s..12s
			b := secondsFrom(streamsOrigin, 20, 3) // the newer one: 20s..22s
			f := newFrontendLoki(t, podStream("web-a", a...), podStream("web-b", b...))

			got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", query(DirectionForward))
			if err != nil {
				t.Fatal(err)
			}
			if want := wantPage("web-a", a[:10]); !slices.Equal(entryStamps(got), want) {
				t.Fatalf("forward page = %v\nwant the 10 oldest lines %v", entryStamps(got), want)
			}
		})
	}
}

func TestABackwardFullPageOverTwoStreamsIsTheNewestLinesNotAShareOfEach(t *testing.T) {
	for name, query := range map[string]func(string) LogQuery{"two hours": streamsQuery, "four days": wideStreamsQuery} {
		t.Run(name, func(t *testing.T) {
			a := secondsFrom(streamsOrigin, 11, 12) // the newer instance: 11s..22s
			b := secondsFrom(streamsOrigin, 1, 3)   // the older one: 1s..3s
			f := newFrontendLoki(t, podStream("web-a", a...), podStream("web-b", b...))

			got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", query(DirectionBackward))
			if err != nil {
				t.Fatal(err)
			}
			if want := wantPage("web-a", a[2:]); !slices.Equal(entryStamps(got), want) {
				t.Fatalf("backward page = %v\nwant the 10 newest lines %v", entryStamps(got), want)
			}
		})
	}
}

// A full page is read again in parts that are each exact, never by listing
// and reading its streams, and in few reads: the narrowed window is cut in two
// at a split cut, and only the part the page lies in is cut again. A sparse
// instance spread over days is the case that cost a read per stream across
// every bucket before (w5/m503: 39 to 65 s of Loki time on an idle service).
func TestAFullPageIsReadAgainInAFewExactParts(t *testing.T) {
	for _, direction := range []string{DirectionBackward, DirectionForward} {
		t.Run(direction, func(t *testing.T) {
			var sparse []int64 // one line an hour over the four days
			for h := -47; h < 48; h++ {
				sparse = append(sparse, streamsOrigin.Add(time.Duration(h)*time.Hour+time.Minute).UnixNano())
			}
			dense := podStream("web-b", secondsFrom(streamsOrigin, 0, 30)...)
			f := newFrontendLoki(t, podStream("web-a", sparse...), dense)
			q := wideStreamsQuery(direction)

			got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", q)
			if err != nil {
				t.Fatal(err)
			}
			all := map[int64]string{}
			for _, s := range []fakeStream{podStream("web-a", sparse...), dense} {
				for _, ns := range s.stamps {
					all[ns] = s.message(ns)
				}
			}
			stamps := slices.Sorted(maps.Keys(all))
			if direction == DirectionForward {
				stamps = stamps[:10]
			} else {
				stamps = stamps[len(stamps)-10:]
			}
			var want []string
			for _, ns := range stamps {
				want = append(want, all[ns])
			}
			if !slices.Equal(entryStamps(got), want) {
				t.Fatalf("%s page = %v\nwant %v", direction, entryStamps(got), want)
			}
			if len(f.seriesCalls) != 0 {
				t.Fatalf("%d stream listings, want none", len(f.seriesCalls))
			}
			// The narrowed window spans up to 384 buckets: 2·log2(384), about
			// 18 reads, bounds it, where a bucket at a time would be hundreds.
			if len(f.rangeCalls) > 1+2*bits.Len(384) {
				t.Fatalf("%d reads, want about 2·log2 of the buckets", len(f.rangeCalls))
			}
		})
	}
}

// A full page whose lines lie in the window's newest bucket is that bucket's
// own read, and no read reaches past the suspect page's far line: the page's
// lines are real, so the window's page is no older than it.
func TestAFullPageIsReadAgainNoFurtherThanItsFarLine(t *testing.T) {
	newest := secondsFrom(streamsOrigin.Add(50*time.Minute), 0, 12)  // 10:50:00..10:50:11
	older := podStream("web-b", secondsFrom(streamsOrigin, 1, 3)...) // 10:00:01..10:00:03
	f := newFrontendLoki(t, podStream("web-a", newest...), older)

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", streamsQuery(DirectionBackward))
	if err != nil {
		t.Fatal(err)
	}
	if want := wantPage("web-a", newest[2:]); !slices.Equal(entryStamps(got), want) {
		t.Fatalf("backward page = %v\nwant the 10 newest lines %v", entryStamps(got), want)
	}
	// The dealt page reached back to web-b's 10:00:01 line, and no re-read
	// starts before it.
	floor := streamsOrigin.Add(time.Second)
	for _, v := range f.rangeCalls[1:] {
		if start, _, _ := readWindow(v); start.Before(floor) {
			t.Fatalf("a re-read starts at %s, before the suspect page's far line %s", start, floor)
		}
	}
	if len(f.seriesCalls) != 0 {
		t.Fatalf("%d stream listings, want none", len(f.seriesCalls))
	}
}

// A full page of a window inside one bucket is exact as Loki returned it.
func TestAFullPageInsideOneBucketIsOneRequest(t *testing.T) {
	f := newFrontendLoki(t,
		podStream("web-a", secondsFrom(streamsOrigin, 1, 12)...),
		podStream("web-b", secondsFrom(streamsOrigin, 20, 3)...))
	q := streamsQuery(DirectionForward)
	q.Since, q.End = streamsOrigin, streamsOrigin.Add(10*time.Minute)

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", q)
	if err != nil || len(got) != 10 {
		t.Fatalf("got %d lines, err %v; want a page of 10", len(got), err)
	}
	if len(f.rangeCalls) != 1 || len(f.seriesCalls) != 0 {
		t.Fatalf("%d reads and %d stream listings, want 1 and 0", len(f.rangeCalls), len(f.seriesCalls))
	}
}

// A caller's inclusive End on a cut is read as Loki is sent it, one
// nanosecond on: the instant at End sits in the next bucket, so the window
// crosses that cut, and its line is kept.
func TestTheInclusiveEndOnACutKeepsItsLine(t *testing.T) {
	end := streamsOrigin.Add(15 * time.Minute) // 10:15, on a cut
	a := append(secondsFrom(streamsOrigin, 600, 12), end.UnixNano())
	f := newFrontendLoki(t, podStream("web-a", a...), podStream("web-b", secondsFrom(streamsOrigin, 610, 5)...))
	q := streamsQuery(DirectionBackward)
	q.End = end

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", q)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[len(got)-1].Message != podStream("web-a").message(end.UnixNano()) {
		t.Fatalf("page = %v, want it to end with the line at the inclusive End", entryStamps(got))
	}
}

// A page short of the limit is exact whatever the frontend did, so it costs one
// request.
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

// Every re-read is the query itself over a smaller window: its restrictions
// (the app id here) apply to each part.
func TestEveryReReadKeepsTheQuerysOwnSelector(t *testing.T) {
	older := fakeStream{labels: map[string]string{"namespace": "default", "app": "web", "container": "app", "pod": "web-a"}, stamps: secondsFrom(streamsOrigin, 1, 12)}
	current := fakeStream{labels: map[string]string{"namespace": "default", "app": "web", "app_id": "app-1", "container": "app", "pod": "web-b"}, stamps: secondsFrom(streamsOrigin, 20, 3)}
	f := newFrontendLoki(t, older, current)
	q := streamsQuery(DirectionForward)
	q.appID = "app-1"

	if _, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", q); err != nil {
		t.Fatal(err)
	}
	if len(f.rangeCalls) < 2 {
		t.Fatalf("%d reads, want the full page read again", len(f.rangeCalls))
	}
	for _, v := range f.rangeCalls {
		if got := v.Get("query"); got != `{namespace="default", app="web", app_id=~"app-1|", container=~"app|"}` {
			t.Fatalf("a read asked for %s, want the query's own selector", got)
		}
	}
}

// A page that cannot be proven exact is an error, never the inexact page.
func TestAFailedReReadFailsTheReadInsteadOfReturningTheInexactPage(t *testing.T) {
	f := newFrontendLoki(t,
		podStream("web-a", secondsFrom(streamsOrigin, 1, 12)...),
		podStream("web-b", secondsFrom(streamsOrigin, 20, 3)...))
	f.rangeCode = map[int]int{2: http.StatusServiceUnavailable} // the first re-read

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", streamsQuery(DirectionForward))
	if err == nil {
		t.Fatalf("got %d lines and no error, want the failed re-read surfaced", len(got))
	}
}

// A raw text-search page is the same suspect: its saturation protocol takes the
// page for the window's oldest lines and moves coverage past it, so a match the
// inexact page dropped would never be read.
func TestALocalTextSearchScanReadsAFullRawPageAgainExactly(t *testing.T) {
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

// A text search over a busy window reads it once: after the first full raw page
// every read stays inside one split bucket, which Loki answers exactly, so no
// raw page is read twice and no stream is listed (w5/m503: on production a
// search re-read every 1,000-line page once per stream, 150 to 270 reads in its
// 18 s budget, and covered under an hour).
func TestABusyTextSearchReadsEachRawLineOnceInsideSplitBuckets(t *testing.T) {
	// Three instances logging every second for 40 minutes from 10:00, nine
	// marked lines across them.
	var streams []fakeStream
	var want []string
	for i, pod := range []string{"web-a", "web-b", "web-c"} {
		s := podStream(pod, secondsFrom(streamsOrigin, i, 2400)...)
		s.marked = map[int64]bool{}
		for _, at := range []int{100, 1300, 2300} {
			ns := s.stamps[at]
			s.marked[ns] = true
			want = append(want, s.message(ns))
		}
		streams = append(streams, s)
	}
	f := newFrontendLoki(t, streams...)
	for _, direction := range []string{DirectionBackward, DirectionForward} {
		t.Run(direction, func(t *testing.T) {
			f.mu.Lock()
			f.rangeCalls, f.rangeLines, f.seriesCalls = nil, nil, nil
			f.mu.Unlock()
			q := LogQuery{
				App: "web", Search: []string{"MATCH"}, Direction: direction, Limit: 100,
				Since: streamsOrigin.Add(-time.Hour), End: streamsOrigin.Add(time.Hour),
			}.normalized()

			got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", q)
			if err != nil {
				t.Fatal(err)
			}
			gotMsgs := entryStamps(got)
			slices.Sort(gotMsgs)
			wantMsgs := slices.Sorted(slices.Values(want))
			if !slices.Equal(gotMsgs, wantMsgs) {
				t.Fatalf("matches = %v, want %v", gotMsgs, wantMsgs)
			}
			if len(f.seriesCalls) != 0 {
				t.Fatalf("%d stream listings, want none", len(f.seriesCalls))
			}
			// Only the first slice, and a part of its re-read that overshot the
			// page, come back full across a cut: after a full raw page the scan
			// reads on inside one bucket.
			if crossing := f.fullReadsAcrossACut(); len(crossing) > 2 {
				t.Fatalf("full reads across a split cut = %v, want the first slice's alone", crossing)
			}
			// 7,200 raw lines are 8 raw pages; the first slice's bucket re-reads,
			// the bucket ends and the empty slices around the busy 40 minutes
			// cost a few more, but no raw line is read once per stream.
			if len(f.rangeCalls) > 20 {
				t.Fatalf("%d reads for 7,200 raw lines, want each read about once", len(f.rangeCalls))
			}
		})
	}
}

// A caller that leaves while the buckets are read gets its error, never a page
// cut from the buckets already read.
func TestACallerThatLeavesDuringTheBucketReadsGetsAnError(t *testing.T) {
	f := newFrontendLoki(t,
		podStream("web-a", secondsFrom(streamsOrigin, 1, 12)...),
		podStream("web-b", secondsFrom(streamsOrigin, 20, 3)...))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.onRange = cancel // the first page arrives, then the caller leaves

	got, err := NewLokiSource(f.srv.URL, f.srv.Client())(ctx, "default", streamsQuery(DirectionForward))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %d lines and err %v, want context.Canceled", len(got), err)
	}
}

// Random shapes against the truth: one to five instances, each starting
// anywhere in a three-day span and logging at its own pace, read over a
// random window in either direction at a random limit. Every page is the
// window's oldest or newest lines, and the reads stay within a few times
// log2 of the buckets the window spans, never one a bucket.
func TestExactPagesMatchTheTruthOnRandomShapes(t *testing.T) {
	rng := rand.New(rand.NewPCG(503, 1119))
	for trial := range 150 {
		var streams []fakeStream
		for i := range 1 + rng.IntN(5) {
			first := streamsOrigin.Add(time.Duration(rng.Int64N(int64(72*time.Hour))) - 36*time.Hour)
			step := time.Duration(1+rng.Int64N(int64(2*time.Hour/time.Second))) * time.Second
			var stamps []int64
			for n := range 1 + rng.IntN(60) {
				// The stream index in the nanoseconds keeps every stamp unique.
				stamps = append(stamps, first.Add(time.Duration(n)*step).UnixNano()+int64(i))
			}
			streams = append(streams, podStream(fmt.Sprintf("web-%d", i), stamps...))
		}
		since := streamsOrigin.Add(-time.Duration(rng.Int64N(int64(40 * time.Hour))))
		end := streamsOrigin.Add(time.Duration(rng.Int64N(int64(40 * time.Hour))))
		direction := []string{DirectionBackward, DirectionForward}[rng.IntN(2)]
		limit := []int64{1, 7, 10, 50}[rng.IntN(4)]

		f := newFrontendLoki(t, streams...)
		q := LogQuery{App: "web", Direction: direction, Limit: limit, Since: since, End: end}.normalized()
		got, err := NewLokiSource(f.srv.URL, f.srv.Client())(context.Background(), "default", q)
		if err != nil {
			t.Fatalf("trial %d: %v", trial, err)
		}
		var truth []LogEntry
		for _, s := range streams {
			for _, ns := range s.within(since.UnixNano(), end.UnixNano()+1) {
				truth = append(truth, LogEntry{Timestamp: time.Unix(0, ns).UTC().Format(time.RFC3339Nano), Message: s.message(ns)})
			}
		}
		want := entryStamps(q.capToLimit(sortedEntries(truth)))
		if !slices.Equal(entryStamps(got), want) {
			t.Fatalf("trial %d (%d streams, %s, limit %d): page = %v\nwant %v", trial, len(streams), direction, limit, entryStamps(got), want)
		}
		buckets := int(end.Sub(since)/lokiSplitBucket) + 2
		if reads := len(f.rangeCalls); reads > 3+4*bits.Len(uint(buckets)) {
			t.Fatalf("trial %d: %d reads for a window of %d buckets", trial, reads, buckets)
		}
	}
}
