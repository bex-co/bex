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
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// lokiSplitBucket is a span Loki never cuts a read inside. Its query frontend
// cuts a read at the multiples of its split interval counted from the Unix
// epoch (pkg/util.ForInterval in the pinned 3.6.7), and a read it does not cut
// goes to the querier whole, past the merge that loses lines. The chart bex
// deploys splits at 15 minutes and Loki's own default at an hour; both are
// multiples of this, so a window inside one bucket is one piece under either.
const lokiSplitBucket = 15 * time.Minute

// lokiReader reads windows of one query from Loki.
//
// A Loki query frontend cuts a read at its split interval (15 minutes in the
// chart bex deploys, an hour in Loki's own default) and merges the pieces'
// answers. When a window crosses a cut and holds more than one stream, and the
// pieces together hold more than `limit` lines, the merged page is not the
// window's newest (backward) or oldest (forward) `limit` lines: it deals out a
// share of each stream, and a page cursor that continues from it never reaches
// what was dropped (w21/063; docs/ADR010-observability.md).
//
// Two answers are exact: a page of fewer than `limit` lines (no piece was cut,
// so the merge hands every piece back whole), and a read of a window inside one
// split bucket (nothing is cut, so nothing is merged). A full page of a window
// that crosses a cut is suspect, and exact reads the window again in parts
// that are each exact (w5/m503). The suspect page's `limit` lines are real, so
// the lines the page should hold lie no later (forward) or no earlier
// (backward) than its far end, which narrows the window; the parts then grow
// from the end the direction keeps, a bucket first, until they hold a page.
type lokiReader struct {
	hc   *http.Client
	base string
	// selector is the query's stream selector, `{matcher, ...}`; pipeline is
	// whatever lokiQueryFor appends to it (json and label-filter stages).
	selector string
	pipeline string
	// pageQ is the query with Limit set to one request's line cap (lokiFetchLimit).
	pageQ LogQuery
}

// fetch is one slice's raw page: exactly the newest (backward) or oldest
// (forward) lines of [from, to) that fit the page; to is the query's inclusive
// End when it is the caller's (lokiEnd).
func (r lokiReader) fetch(ctx context.Context, from, to time.Time) ([]LogEntry, error) {
	return r.exact(ctx, from, lokiEnd(r.pageQ, to))
}

// exact is the exact page of [from, end), end exclusive as Loki is sent it.
func (r lokiReader) exact(ctx context.Context, from, end time.Time) ([]LogEntry, error) {
	page, err := r.read(ctx, from, end)
	if err != nil || int64(len(page)) < r.pageQ.Limit || oneBucket(from, end) {
		return page, err
	}
	from, end = r.narrowWindow(page, from, end)
	return r.outward(ctx, from, end)
}

// outward reads [from, end) in parts from the end the direction keeps, one
// split bucket and then twice the last part's buckets each time, until the
// parts read hold a page, and answers that page. Every line of a part lies
// nearer that end than the lines of the parts after it, so the first parts'
// `limit` lines are the window's. A part short of a page or inside one bucket
// is exact as read; a full part across a cut holds the rest of the page and is
// read the same way, narrowed to its own far line. A busy service's page is a
// bucket or two; a quiet one's spans many buckets in a few parts: a window of
// B buckets costs a few times log2(B) reads, never one a bucket.
func (r lokiReader) outward(ctx context.Context, from, end time.Time) ([]LogEntry, error) {
	forward := lokiDirection(r.pageQ) == DirectionForward
	var lines []LogEntry
	for buckets := 1; from.Before(end) && int64(len(lines)) < r.pageQ.Limit; buckets *= 2 {
		lo, hi := from, end
		if forward {
			hi = earlier(end, splitBucketStart(from).Add(time.Duration(buckets)*lokiSplitBucket))
		} else {
			lo = later(from, splitBucketStart(end.Add(-time.Nanosecond)).Add(-time.Duration(buckets-1)*lokiSplitBucket))
		}
		part, err := r.read(ctx, lo, hi)
		if err != nil {
			return nil, err
		}
		if int64(len(part)) >= r.pageQ.Limit && !oneBucket(lo, hi) {
			lo, hi = r.narrowWindow(part, lo, hi)
			if part, err = r.outward(ctx, lo, hi); err != nil {
				return nil, err
			}
			lines = append(lines, part...)
			break
		}
		lines = append(lines, part...)
		if forward {
			from = hi
		} else {
			end = lo
		}
	}
	return r.pageQ.capToLimit(sortedEntries(lines)), nil
}

// read is one query_range over [from, end) for the query's selector and
// pipeline, as raw candidates; end is exclusive and sent as it is.
func (r lokiReader) read(ctx context.Context, from, end time.Time) ([]LogEntry, error) {
	u := fmt.Sprintf("%s/loki/api/v1/query_range?%s", r.base, url.Values{
		"query": {r.selector + r.pipeline},
		"start": {strconv.FormatInt(from.UnixNano(), 10)},
		"end":   {strconv.FormatInt(end.UnixNano(), 10)},
		"limit": {strconv.FormatInt(r.pageQ.Limit, 10)},
		// Render's direction decides which end of the window `limit` keeps:
		// backward (default) the newest lines, forward the oldest.
		"direction": {lokiDirection(r.pageQ)},
	}.Encode())
	var lr lokiRangeResponse
	if err := lokiGet(ctx, r.hc, u, &lr); err != nil {
		return nil, err
	}
	// Raw candidates, unmatched: scanLokiWindow measures coverage by every line
	// read before it keeps the matches (w8/m58).
	return parseLokiStreams(lr, r.pageQ)
}

// narrowWindow bounds [from, end) by a full page's far line: the page's lines
// are real, so the window's page lies no earlier (backward) or no later
// (forward). page is oldest-first and parseLokiStreams formatted its stamps, so
// a stamp that does not parse is not expected; the window then stays as asked,
// which only costs more reads.
func (r lokiReader) narrowWindow(page []LogEntry, from, end time.Time) (time.Time, time.Time) {
	if lokiDirection(r.pageQ) == DirectionForward {
		if far, err := time.Parse(time.RFC3339Nano, page[len(page)-1].Timestamp); err == nil && far.Before(end) {
			end = far.Add(time.Nanosecond)
		}
	} else if far, err := time.Parse(time.RFC3339Nano, page[0].Timestamp); err == nil && far.After(from) {
		from = far
	}
	return from, end
}

// oneBucket reports whether Loki reads [from, end) as one piece: the window
// lies inside one split bucket.
func oneBucket(from, end time.Time) bool {
	return !end.After(splitBucketStart(from).Add(lokiSplitBucket))
}

// splitBucketStart is the start of the split bucket that holds t.
func splitBucketStart(t time.Time) time.Time {
	ns := t.UnixNano()
	return time.Unix(0, ns-ns%int64(lokiSplitBucket)).UTC()
}
