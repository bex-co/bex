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
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const (
	// lokiMaxStreamReads bounds the one-per-stream reads a full page may cost.
	// The shipper's cardinality budget keeps a window to a few dozen streams
	// (deploy/gitops/base/log-shipper.yaml); a window past this is refused rather
	// than read inexactly, and the caller narrows it.
	lokiMaxStreamReads = 256
	// lokiStreamReadParallelism is how many of one page's stream reads run at once.
	lokiStreamReadParallelism = 8
	// lokiStreamReadsInFlight is how many run at once across every request, so
	// concurrent readers and long scans cannot multiply the load on Loki.
	lokiStreamReadsInFlight = 32
)

// lokiStreamReads holds one token per stream read in flight, process-wide.
var lokiStreamReads = make(chan struct{}, lokiStreamReadsInFlight)

// errLogStreamFanOut is the answer when a full page's window holds more streams
// than lokiMaxStreamReads: reading them all would outlast any request, and a
// page cut from fewer would be the inexact one this exists to avoid.
var errLogStreamFanOut = core.NewUnavailableError(core.CodeQueryTimeout,
	fmt.Sprintf("the log time range holds more than %d log streams to read exactly; narrow the time range", lokiMaxStreamReads), nil)

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
// so the merge hands every piece back whole), and a read of a single stream (the
// merge has nothing to interleave). A full page is therefore suspect, and fetch
// reads it again one stream at a time. The suspect page's `limit` lines are
// real, so the lines the page should hold lie no later (forward) or no earlier
// (backward) than its far end; that bounds the window whose streams are listed.
// Each stream's own `limit` lines, merged and cut to `limit`, are the window's:
// the window's page is a subset of the union of its streams' pages.
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

// read is one query_range over [from, to) for selector plus the query's pipeline,
// as raw candidates.
func (r lokiReader) read(ctx context.Context, selector string, from, to time.Time) ([]LogEntry, error) {
	u := fmt.Sprintf("%s/loki/api/v1/query_range?%s", r.base, url.Values{
		"query": {selector + r.pipeline},
		"start": {strconv.FormatInt(from.UnixNano(), 10)},
		"end":   {lokiEndParam(r.pageQ, to)},
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

// fetch is one slice's raw page: exactly the newest (backward) or oldest
// (forward) lines of [from, to) that fit the page.
func (r lokiReader) fetch(ctx context.Context, from, to time.Time) ([]LogEntry, error) {
	page, err := r.read(ctx, r.selector, from, to)
	if err != nil || int64(len(page)) < r.pageQ.Limit {
		return page, err
	}
	return r.readByStream(ctx, page, from, to)
}

// readByStream replaces a full page by the same page read stream by stream.
func (r lokiReader) readByStream(ctx context.Context, page []LogEntry, from, to time.Time) ([]LogEntry, error) {
	// page is oldest-first and parseLokiStreams formatted its stamps, so a stamp
	// that does not parse is not expected; the window then stays as asked, which
	// only lists more streams.
	if lokiDirection(r.pageQ) == DirectionForward {
		if far, err := time.Parse(time.RFC3339Nano, page[len(page)-1].Timestamp); err == nil && far.Before(to) {
			to = far.Add(time.Nanosecond)
		}
	} else if far, err := time.Parse(time.RFC3339Nano, page[0].Timestamp); err == nil && far.After(from) {
		from = far
	}
	streams, err := r.streams(ctx, from, to)
	if err != nil {
		return nil, err
	}
	// With no second stream in the narrowed window every line of the page is
	// that one stream's, and the merge deals a stream's lines in order, so the
	// page is its own oldest (newest) lines. No listed stream at all leaves
	// nothing to read by stream.
	if len(streams) <= 1 {
		return page, nil
	}
	selectors, err := r.streamSelectors(streams)
	if err != nil {
		return nil, err
	}
	if len(selectors) > lokiMaxStreamReads {
		return nil, errLogStreamFanOut
	}
	pages := make([][]LogEntry, len(selectors))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(lokiStreamReadParallelism)
	for i, selector := range selectors {
		if gctx.Err() != nil { // a read already failed, or the caller left
			break
		}
		g.Go(func() (err error) {
			select {
			case lokiStreamReads <- struct{}{}:
				defer func() { <-lokiStreamReads }()
			case <-gctx.Done():
				return gctx.Err()
			}
			pages[i], err = r.read(gctx, selector, from, to)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil { // the loop stopped on the caller leaving, with no read failed
		return nil, err
	}
	return r.pageQ.capToLimit(sortedEntries(slices.Concat(pages...))), nil
}

// streams lists the label sets of the streams with lines in [from, to) that the
// query's selector matches.
func (r lokiReader) streams(ctx context.Context, from, to time.Time) ([]map[string]string, error) {
	u := fmt.Sprintf("%s/loki/api/v1/series?%s", r.base, url.Values{
		"match[]": {r.selector},
		"start":   {strconv.FormatInt(from.UnixNano(), 10)},
		"end":     {lokiEndParam(r.pageQ, to)},
	}.Encode())
	var resp struct {
		Status string              `json:"status"`
		Data   []map[string]string `json:"data"`
	}
	if err := lokiGet(ctx, r.hc, u, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "" && resp.Status != "success" {
		return nil, fmt.Errorf("loki status %q", resp.Status)
	}
	return resp.Data, nil
}

// streamSelectors narrows the query's selector to each listed stream, one
// selector per distinct stream. A stream is pinned by an equality matcher for
// every label any listed stream carries, an empty value where it has none, so a
// stream that carries an extra label is not matched along with it. The query's
// own matchers stay in front: its restrictions (app id, excluded jobs) still
// apply to every read.
func (r lokiReader) streamSelectors(streams []map[string]string) ([]string, error) {
	head, ok := strings.CutSuffix(r.selector, "}")
	if !ok {
		return nil, fmt.Errorf("loki selector %q is not a stream selector", r.selector)
	}
	names := map[string]struct{}{}
	for _, labels := range streams {
		for name := range labels {
			if !strings.HasPrefix(name, "__") { // Loki's internal labels (stream shards) are not stream identity
				names[name] = struct{}{}
			}
		}
	}
	sorted := slices.Sorted(maps.Keys(names))
	seen := map[string]struct{}{}
	var out []string
	for _, labels := range streams {
		pins := make([]string, len(sorted))
		for i, name := range sorted {
			pins[i] = fmt.Sprintf("%s=%q", name, labels[name])
		}
		selector := head + ", " + strings.Join(pins, ", ") + "}"
		if _, dup := seen[selector]; !dup { // shards of one stream differ only in internal labels
			seen[selector] = struct{}{}
			out = append(out, selector)
		}
	}
	return out, nil
}
