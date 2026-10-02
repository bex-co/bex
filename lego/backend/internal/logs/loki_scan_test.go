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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// scanWindow is a fixed week so slice boundaries are exact.
var (
	scanEnd   = time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	scanStart = scanEnd.Add(-7 * 24 * time.Hour)
)

type sliceCall struct{ from, to time.Time }

func lineAt(t time.Time, msg string) LogEntry {
	return LogEntry{Timestamp: t.UTC().Format(time.RFC3339Nano), Message: msg, Labels: map[string]string{LabelInstance: "web-1"}}
}

func TestScanLokiWindowStopsAtTheFirstSliceThatFillsThePage(t *testing.T) {
	var calls []sliceCall
	q := LogQuery{Search: []string{"GET"}, Limit: 2}.normalized()
	got, err := scanLokiWindow(context.Background(), q, scanStart, scanEnd, true, time.Minute,
		func(_ context.Context, from, to time.Time) ([]LogEntry, error) {
			calls = append(calls, sliceCall{from, to})
			return []LogEntry{lineAt(to.Add(-2*time.Minute), "a"), lineAt(to.Add(-time.Minute), "b")}, nil
		})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d lines, err %v", len(got), err)
	}
	if len(calls) != 1 || !calls[0].from.Equal(scanEnd.Add(-lokiFirstSlice)) || !calls[0].to.Equal(scanEnd) {
		t.Fatalf("calls = %+v, want one newest-hour slice", calls)
	}
}

func TestScanLokiWindowCoversANoMatchWeekInDoublingSlicesNewestFirst(t *testing.T) {
	var calls []sliceCall
	q := LogQuery{Search: []string{"zzqqxx-no-such-token"}, Limit: 100}.normalized()
	got, err := scanLokiWindow(context.Background(), q, scanStart, scanEnd, true, time.Minute,
		func(_ context.Context, from, to time.Time) ([]LogEntry, error) {
			calls = append(calls, sliceCall{from, to})
			return nil, nil
		})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, err %v; want an empty, complete answer", got, err)
	}
	// 1h, 2h, 4h, …, contiguous from the end back to the start, never overlapping.
	next := scanEnd
	for i, c := range calls {
		if !c.to.Equal(next) {
			t.Fatalf("slice %d ends %v, want %v (contiguous)", i, c.to, next)
		}
		next = c.from
	}
	if !next.Equal(scanStart) {
		t.Fatalf("slices reach back to %v, want the window start %v", next, scanStart)
	}
	if want := time.Duration(1<<2) * time.Hour; calls[2].to.Sub(calls[2].from) != want {
		t.Fatalf("third slice spans %v, want %v", calls[2].to.Sub(calls[2].from), want)
	}
}

func TestScanLokiWindowReturnsTheCoveredPartWhenTheBudgetRunsOut(t *testing.T) {
	q := LogQuery{Search: []string{"rare"}, Limit: 100}.normalized()
	calls := 0
	_, err := scanLokiWindow(context.Background(), q, scanStart, scanEnd, true, 200*time.Millisecond,
		func(ctx context.Context, from, to time.Time) ([]LogEntry, error) {
			calls++
			if calls <= 2 { // the newest 1h and 2h slices finish, one match in them
				if calls == 1 {
					return []LogEntry{lineAt(to.Add(-time.Minute), "rare hit")}, nil
				}
				return nil, nil
			}
			<-ctx.Done() // the 4h slice outlasts the budget
			return nil, fmt.Errorf("loki: %w", ctx.Err())
		})
	var partial *ScanIncompleteError
	if !errors.As(err, &partial) {
		t.Fatalf("err = %v, want *ScanIncompleteError", err)
	}
	if want := scanEnd.Add(-3 * time.Hour); !partial.ScannedTo.Equal(want) {
		t.Fatalf("ScannedTo = %v, want %v (the 1h+2h slices)", partial.ScannedTo, want)
	}
	if len(partial.Entries) != 1 || partial.Entries[0].Message != "rare hit" {
		t.Fatalf("entries = %+v, want the covered match", partial.Entries)
	}
}

func TestScanLokiWindowNamesATimeoutWhenNothingIsCovered(t *testing.T) {
	q := LogQuery{Search: []string{"rare"}, Limit: 100}.normalized()
	_, err := scanLokiWindow(context.Background(), q, scanStart, scanEnd, true, 50*time.Millisecond,
		func(ctx context.Context, _, _ time.Time) ([]LogEntry, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeQueryTimeout || !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("err = %v, want a QUERY_TIMEOUT 503", err)
	}
}

func TestScanLokiWindowPassesThroughACallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	q := LogQuery{Search: []string{"rare"}, Limit: 100}.normalized()
	calls := 0
	_, err := scanLokiWindow(ctx, q, scanStart, scanEnd, true, time.Minute,
		func(ctx context.Context, _, _ time.Time) ([]LogEntry, error) {
			calls++
			if calls == 2 {
				cancel()
				return nil, ctx.Err()
			}
			return nil, nil
		})
	var partial *ScanIncompleteError
	if errors.As(err, &partial) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the caller's cancellation, not a partial page", err)
	}
}

func TestScanLokiWindowSlicesForwardFromTheStart(t *testing.T) {
	var calls []sliceCall
	q := LogQuery{Search: []string{"x"}, Limit: 100, Direction: DirectionForward}.normalized()
	_, err := scanLokiWindow(context.Background(), q, scanStart, scanEnd, true, time.Minute,
		func(_ context.Context, from, to time.Time) ([]LogEntry, error) {
			calls = append(calls, sliceCall{from, to})
			return nil, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !calls[0].from.Equal(scanStart) || !calls[0].to.Equal(scanStart.Add(lokiFirstSlice)) {
		t.Fatalf("first forward slice = %+v, want the oldest hour", calls[0])
	}
	if last := calls[len(calls)-1]; !last.to.Equal(scanEnd) {
		t.Fatalf("last forward slice ends %v, want %v", last.to, scanEnd)
	}
}

func TestScanLokiWindowKeepsALabelOnlyReadToOneRequest(t *testing.T) {
	var calls []sliceCall
	q := LogQuery{Limit: 100}.normalized()
	if lokiScansLines(q) {
		t.Fatal("a query with no text/path/host must not be sliced")
	}
	_, err := scanLokiWindow(context.Background(), q, scanStart, scanEnd, lokiScansLines(q), time.Minute,
		func(_ context.Context, from, to time.Time) ([]LogEntry, error) {
			calls = append(calls, sliceCall{from, to})
			return nil, nil
		})
	if err != nil || len(calls) != 1 || !calls[0].from.Equal(scanStart) || !calls[0].to.Equal(scanEnd) {
		t.Fatalf("calls = %+v, err %v; want one whole-window request", calls, err)
	}
}

func TestScanLokiWindowCountsABoundaryLineOnce(t *testing.T) {
	boundary := scanEnd.Add(-lokiFirstSlice)
	q := LogQuery{Search: []string{"edge"}, Limit: 100}.normalized()
	got, err := scanLokiWindow(context.Background(), q, scanStart, scanEnd, true, time.Minute,
		func(_ context.Context, from, to time.Time) ([]LogEntry, error) {
			if from.Equal(boundary) || to.Equal(boundary) {
				return []LogEntry{lineAt(boundary, "edge")}, nil
			}
			return nil, nil
		})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %d copies of the boundary line, err %v", len(got), err)
	}
}

// A scan that ran out of time is a page, not an error: the covered matches plus
// hasMore and a cursor at the coverage edge, so a client pages on into the rest
// of the window (w4/m140). Across two resources, the less-covered one bounds
// the page and the other's lines beyond that edge wait for the next page.
func TestQueryLogPageTurnsAnIncompleteScanIntoAnHonestPage(t *testing.T) {
	webEdge := scanEnd.Add(-3 * time.Hour)
	apiEdge := scanEnd.Add(-time.Hour)
	svc := newService(nil, sampleApp("web"), sampleApp("api"))
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		switch q.App {
		case "web":
			return nil, &ScanIncompleteError{
				Entries:   []LogEntry{lineAt(scanEnd.Add(-2*time.Hour), "web older"), lineAt(scanEnd.Add(-30*time.Minute), "web newer")},
				ScannedTo: webEdge,
			}
		default:
			return nil, &ScanIncompleteError{Entries: []LogEntry{lineAt(scanEnd.Add(-10*time.Minute), "api")}, ScannedTo: apiEdge}
		}
	}
	q := LogQuery{Search: []string{"x"}, Since: scanStart, End: scanEnd}

	page, err := svc.queryLogPage(context.Background(), []string{"web"}, q)
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasMore || page.NextStartTime != scanStart.Format(time.RFC3339Nano) ||
		page.NextEndTime != webEdge.Add(-time.Nanosecond).Format(time.RFC3339Nano) || len(page.Entries) != 2 {
		t.Fatalf("single-resource page = %+v", page)
	}

	page, err = svc.queryLogPage(context.Background(), []string{"web", "api"}, q)
	if err != nil {
		t.Fatal(err)
	}
	var msgs []string
	for _, e := range page.Entries {
		msgs = append(msgs, e.Message)
	}
	if fmt.Sprint(msgs) != "[web newer api]" {
		t.Fatalf("entries = %v, want only lines inside the least-covered edge", msgs)
	}
	if !page.HasMore || page.NextEndTime != apiEdge.Add(-time.Nanosecond).Format(time.RFC3339Nano) {
		t.Fatalf("merged page cursor = %+v, want the api edge", page)
	}
}
