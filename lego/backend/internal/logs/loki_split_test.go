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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestLokiPagesCrossingASplitBoundaryKeepEveryLine runs w21/063 on the pinned
// Loki (3.6.7, BEX_TEST_LOKI_URL). Its query frontend cuts a read at the split
// interval (an hour by default) and, for a window that crosses a cut and holds
// two instance streams whose lines together outnumber the page, returns a page
// that is not the window's oldest (forward) or newest (backward) lines: a cursor
// chain that follows such pages skipped lines and ended `hasMore: false`. The
// window here straddles the most recent top of the hour, which every split
// interval Loki is run with divides. Both chains must return every line once.
func TestLokiPagesCrossingASplitBoundaryKeepEveryLine(t *testing.T) {
	base := os.Getenv("BEX_TEST_LOKI_URL")
	if base == "" {
		testenv.Skip(t, "BEX_TEST_LOKI_URL not set")
	}
	base = strings.TrimRight(base, "/")
	hc := &http.Client{Timeout: 30 * time.Second}

	// The last top of the hour that is far enough in the past for the whole
	// window, which runs four minutes past it, to be history.
	boundary := time.Now().UTC().Truncate(time.Hour)
	if time.Since(boundary) < 10*time.Minute {
		boundary = boundary.Add(-time.Hour)
	}
	start, end := boundary.Add(-4*time.Minute), boundary.Add(4*time.Minute)

	type instance struct {
		pod         string
		first, last int // seconds from start, every other second
		offset      time.Duration
	}
	// Forward loses lines when the oldest split holds fewer lines than the page;
	// backward when the newest does. Each shape fails one direction on the
	// unfixed read, and both must hold in both directions.
	for _, tc := range []struct {
		name      string
		direction string
		limit     int
		shape     []instance
	}{
		{"forward, page of 100 (the live probe)", DirectionForward, 100,
			[]instance{{"web-old", 57, 383, time.Millisecond}, {"web-new", 302, 478, 500 * time.Millisecond}}},
		{"forward, page of 50", DirectionForward, 50,
			[]instance{{"web-old", 57, 383, time.Millisecond}, {"web-new", 302, 478, 500 * time.Millisecond}}},
		{"backward, page of 100", DirectionBackward, 100,
			[]instance{{"web-old", 57, 299, time.Millisecond}, {"web-new", 100, 280, 500 * time.Millisecond}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ns := fmt.Sprintf("w21-063-%d", time.Now().UnixNano())
			want := map[string]bool{}
			var streams []map[string]any
			for _, in := range tc.shape {
				var values [][2]string
				for s := in.first; s <= in.last; s += 2 {
					at := start.Add(time.Duration(s)*time.Second + in.offset)
					msg := fmt.Sprintf("hb pod=%s at=%d", in.pod, at.UnixNano())
					values = append(values, [2]string{strconv.FormatInt(at.UnixNano(), 10), msg})
					want[msg] = true
				}
				streams = append(streams, map[string]any{
					"stream": map[string]string{"namespace": ns, "app": "web", "container": "app", "pod": in.pod},
					"values": values,
				})
			}
			body, _ := json.Marshal(map[string]any{"streams": streams})
			resp, err := hc.Post(base+"/loki/api/v1/push", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("push: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode/100 != 2 {
				t.Fatalf("push: HTTP %d", resp.StatusCode)
			}

			src := NewLokiSource(base, hc)
			q := LogQuery{App: "web", Direction: tc.direction, Limit: int64(tc.limit), Since: start, End: end}.normalized()
			// The window is read in the same chain a client walks, from the page's
			// own cursors.
			var got []string
			cur := q
			for pages := 0; pages < 200; pages++ {
				entries, err := src(context.Background(), ns, cur)
				if err != nil {
					t.Fatalf("page %d: %v", pages, err)
				}
				for _, e := range entries {
					got = append(got, e.Message)
				}
				if int64(len(entries)) < cur.Limit {
					break
				}
				next, nextEnd := pageCursors(entries, cur.Since, cur.End, tc.direction)
				cur.Since, cur.End = mustParseCursor(t, next), mustParseCursor(t, nextEnd)
			}
			seen := map[string]bool{}
			for _, m := range got {
				if seen[m] {
					t.Errorf("line returned twice: %s", m)
				}
				seen[m] = true
			}
			var missing []string
			for m := range want {
				if !seen[m] {
					missing = append(missing, m)
				}
			}
			slices.Sort(missing)
			if len(missing) > 0 || len(got) != len(want) {
				t.Fatalf("%s chain at limit %d returned %d of %d lines; %d never returned, first: %v",
					tc.direction, tc.limit, len(got), len(want), len(missing), missing[:min(3, len(missing))])
			}
		})
	}
}

func mustParseCursor(t *testing.T, s string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("cursor %q: %v", s, err)
	}
	return at
}
