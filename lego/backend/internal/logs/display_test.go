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
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The colored line from the w4/m189 finding, byte for byte.
const ansiSample = "QA_A31_ANSI_\x1b[31mred\x1b[0m_end"

func TestDisplayTextMatchesTheDashboardRenderer(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"plain", "plain, 中文 .* | text", "plain, 中文 .* | text"},
		{"sgr", ansiSample, "QA_A31_ANSI_red_end"},
		{"extended sgr", "\x1b[38;2;1;2;3mtrue\x1b[38;5;196mcolor\x1b[m", "truecolor"},
		{"cr overwrite", "secret\rSHOWN", "SHOWNt"},
		{"progress bar", "10%\r50%\r100%", "100%"},
		{"cr per line", "abc\ndef\rX", "abc\nXef"},
		{"erase to end", "abcdef\r\x1b[Kxy", "xy"},
		{"erase to start", "abcdef\x1b[3D\x1b[1Kz", "      z"},
		{"erase line", "old\x1b[2Knew", "new"},
		{"osc hyperlink bel", "\x1b]8;;https://x.test\alabel\x1b]8;;\a!", "label!"},
		{"osc title st", "\x1b]0;title\x1b\\body", "body"},
		{"charset", "\x1b(Bok", "ok"},
		{"two byte escape", "a\x1b7b", "ab"},
		{"private csi", "\x1b[?25lhidden-cursor", "hidden-cursor"},
		{"unterminated csi", "keep\x1b[31", "keep"},
		{"unterminated osc", "keep\x1b]0;never ends", "keep"},
		{"lone esc", "keep\x1b", "keep"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := displayText(tc.in); got != tc.want {
				t.Fatalf("displayText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTextModeMatchesDisplayedTextOnlyWhenAsked(t *testing.T) {
	for _, tc := range []struct {
		mode, term string
		want       bool
	}{
		{"", "QA_A31_ANSI_red_end", false}, // the finding's raw probe: unchanged
		{TextModeRaw, "qa_a31_ansi_red_end", false},
		{"", "QA_A31_ANSI_", true},
		{"", "\x1b[31mred", true},
		{TextModeDisplay, "QA_A31_ANSI_red_end", true},
		{TextModeDisplay, "qa_a31_ansi_RED_END", true},
		{TextModeDisplay, "\x1b[31mred", false}, // escapes are not displayed text
		{TextModeDisplay, "31m", false},
	} {
		q := LogQuery{Search: []string{tc.term}, TextMode: tc.mode}.normalized()
		if got := q.keep(LogEntry{Message: ansiSample}); got != tc.want {
			t.Errorf("mode %q term %q: keep = %v, want %v", tc.mode, tc.term, got, tc.want)
		}
	}
	overwritten := LogQuery{Search: []string{"secret"}, TextMode: TextModeDisplay}.normalized()
	if overwritten.keep(LogEntry{Message: "secret\rSHOWN"}) {
		t.Error("display search matched text a carriage return overwrote")
	}
}

func TestDisplayModeAcrossAdaptersKeepsRawBytesAndRawDefault(t *testing.T) {
	svc := newService(map[string][]string{"web-1": {
		"2026-07-05T00:00:01Z " + ansiSample,
		"2026-07-05T00:00:02Z secret\rSHOWN",
		"2026-07-05T00:00:03Z plain control",
	}}, sampleApp("web"), podFor("web", "web-1"))
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	cs := mcpLogsClient(t, svc)

	messages := func(t *testing.T, term, mode string) map[string][]string {
		t.Helper()
		out := map[string][]string{}
		params := url.Values{"resource": {"web"}, "text": {term}}
		args := map[string]any{"resource": []string{"web"}, "text": []string{term}}
		gqlMode := ""
		if mode != "" {
			params.Set("textMode", mode)
			args["textMode"] = mode
			gqlMode = fmt.Sprintf(", textMode:%q", mode)
		}
		rec := serveREST(svc, http.MethodGet, "/v1/logs?"+params.Encode())
		if rec.Code != http.StatusOK {
			t.Fatalf("REST %d: %s", rec.Code, rec.Body.String())
		}
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_logs", Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("MCP: %v %+v", err, res)
		}
		data := runQuery(t, schema, fmt.Sprintf(`{ logs(resource:"web", text:%q%s) { logs { message } } }`, term, gqlMode))
		gqlBody, err := json.Marshal(data["logs"])
		if err != nil {
			t.Fatal(err)
		}
		for adapter, body := range map[string][]byte{
			"REST": rec.Body.Bytes(), "MCP": []byte(res.Content[0].(*mcp.TextContent).Text), "GraphQL": gqlBody,
		} {
			var page wirePage
			if err := json.Unmarshal(body, &page); err != nil {
				t.Fatalf("%s: %v", adapter, err)
			}
			out[adapter] = []string{}
			for _, row := range page.Logs {
				out[adapter] = append(out[adapter], row.Message)
			}
		}
		return out
	}

	for _, tc := range []struct {
		term, mode string
		want       []string
	}{
		{"QA_A31_ANSI_red_end", "", []string{}},
		{"QA_A31_ANSI_red_end", TextModeRaw, []string{}},
		{"QA_A31_ANSI_", "", []string{ansiSample}},
		{"red", "", []string{ansiSample}},
		{"QA_A31_ANSI_red_end", TextModeDisplay, []string{ansiSample}},
		{"red", TextModeDisplay, []string{ansiSample}},
		{"secret", "", []string{"secret\rSHOWN"}},
		{"secret", TextModeDisplay, []string{}},
		{"SHOWNt", TextModeDisplay, []string{"secret\rSHOWN"}},
	} {
		t.Run(tc.mode+"/"+tc.term, func(t *testing.T) {
			for adapter, got := range messages(t, tc.term, tc.mode) {
				if !slices.Equal(got, tc.want) {
					t.Errorf("%s = %q, want %q (raw bytes returned untouched)", adapter, got, tc.want)
				}
			}
		})
	}
}

func TestUnknownTextModeIsANamedBadRequestOnEverySurface(t *testing.T) {
	svc := newService(map[string][]string{"web-1": {"2026-07-05T00:00:01Z x"}}, sampleApp("web"), podFor("web", "web-1"))
	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&text=x&textMode=regex")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `unknown textMode \"regex\"`) {
		t.Fatalf("REST = %d %s, want a 400 naming the mode", rec.Code, rec.Body.String())
	}
	rec = serveREST(svc, http.MethodGet, "/v1/logs/subscribe?resource=web&textMode=Display")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("subscribe = %d %s, want 400", rec.Code, rec.Body.String())
	}
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ logs(resource:"web", textMode:"regex") { hasMore } }`})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "textMode") {
		t.Fatalf("GraphQL errors = %v, want the named refusal", res.Errors)
	}
	out, err := mcpLogsClient(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "list_logs",
		Arguments: map[string]any{"resource": []string{"web"}, "textMode": "regex"}})
	if err == nil && (out == nil || !out.IsError) {
		t.Fatalf("MCP accepted an unknown textMode: %+v", out)
	}
}

func TestLiveTailAppliesDisplayMatching(t *testing.T) {
	stream := newScriptedStream(map[string][]string{"web-1": {strings.Join([]string{
		"2026-09-14T10:00:00Z " + ansiSample,
		"2026-09-14T10:00:01Z secret\rSHOWN",
		"2026-09-14T10:00:02Z zz-done",
	}, "\n")}})
	svc := tailService(stream.follow, sampleApp("web"), appPodIn("web", "web-1", "Running", containerRunning, 0))
	run := startTail(t, svc, LogQuery{App: "web", Search: []string{"qa_a31_ansi_red_end", "secret", "zz-done"}, TextMode: TextModeDisplay})
	for _, want := range []string{ansiSample, "zz-done"} {
		select {
		case got := <-run.messages:
			if got != want {
				t.Fatalf("tail emitted %q, want %q (overwritten text must not match)", got, want)
			}
		case err := <-run.done:
			t.Fatalf("tail ended: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %q", want)
		}
	}
}

func TestLokiTextSearchesKeepTheLokiQueryBroad(t *testing.T) {
	raw := lokiQueryFor("default", LogQuery{App: "web", Search: []string{"a.b"}, Path: []string{"/x"}}.normalized())
	if strings.Contains(raw, " |~ ") {
		t.Fatalf("raw text search must not delegate matching to Loki: %s", raw)
	}
	if !strings.Contains(raw, `request_path="/x"`) {
		t.Fatalf("raw query lost the path filter: %s", raw)
	}
	q := LogQuery{App: "web", Search: []string{"QA_A31_ANSI_red_end", "a.b"}, Path: []string{"/x"}, TextMode: TextModeDisplay}.normalized()
	query := lokiQueryFor("default", q)
	if strings.Contains(query, " |~ ") {
		t.Fatalf("display text search must not delegate matching to Loki: %s", query)
	}
	if !strings.Contains(query, `request_path="/x"`) {
		t.Fatalf("display query lost a filter: %s", query)
	}
}

// A caller End is inclusive. When more than a raw page sits on and just
// before it, the scan must still progress past that instant (not re-read one
// full page until the budget dies) and reach an older match.
func TestLokiDisplayScanProgressesPastAFullPageAtTheInclusiveEnd(t *testing.T) {
	end := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	var stamps []time.Time
	for i := 0; i < 1500; i++ {
		stamps = append(stamps, end)
	}
	for i := 0; i < 1200; i++ {
		stamps = append(stamps, end.Add(-time.Nanosecond))
	}
	older := end.Add(-time.Second)
	stamps = append(stamps, older)
	srv := exclusiveEndLoki(t, stamps)
	defer srv.Close()
	src := NewLokiSource(srv.URL, srv.Client())
	needle := "line-" + older.Format(time.RFC3339Nano)
	start := time.Now()
	got, err := src(context.Background(), "default", LogQuery{
		App: "web", Search: []string{needle}, TextMode: TextModeDisplay, Since: end.Add(-time.Hour), End: end, Limit: 100,
	}.normalized())
	if err != nil {
		t.Fatalf("err = %v after %v", err, time.Since(start))
	}
	if len(got) != 1 || got[0].Message != needle {
		t.Fatalf("got %+v, want the older match", got)
	}
}

// rangeLoki answers like query_range over data (oldest-first): [from, to) with
// the newest (backward) or oldest (forward) `limit` lines, returned oldest-first.
func rangeLoki(data []LogEntry, limit int, forward bool, calls *int) func(context.Context, time.Time, time.Time) ([]LogEntry, error) {
	return func(_ context.Context, from, to time.Time) ([]LogEntry, error) {
		*calls++
		var in []LogEntry
		for _, e := range data {
			ts, _ := time.Parse(time.RFC3339Nano, e.Timestamp)
			if !ts.Before(from) && ts.Before(to) {
				in = append(in, e)
			}
		}
		if len(in) > limit {
			if forward {
				in = in[:limit]
			} else {
				in = in[len(in)-limit:]
			}
		}
		return slices.Clone(in), nil
	}
}

// A display search must reach matches behind far more than a page of
// unmatched raw lines, and lose nothing where Loki's limit cuts a group of
// lines sharing one timestamp.
func TestScanLokiWindowDisplayCoversRawLinesNotMatches(t *testing.T) {
	const n = 3000
	base := scanEnd.Add(-30 * time.Minute)
	var data []LogEntry
	var want []string
	for i := 0; i < n; i++ {
		ts := base.Add(time.Duration(i/3) * time.Millisecond) // groups of three share an instant
		msg := fmt.Sprintf("noise %d", i)
		// 1998/1999 share an instant with 2000, the line where the first backward
		// raw page is cut; 999-1001 sit at the forward cut. Line 0 is the oldest
		// match, behind 2999 unmatched-or-sparse lines.
		if i%250 == 0 || i == 1998 || i == 1999 || i == 1001 {
			msg = fmt.Sprintf("hit-\x1b[1m%d\x1b[0m-end", i)
			want = append(want, fmt.Sprintf("hit-\x1b[1m%d\x1b[0m-end", i))
		}
		data = append(data, LogEntry{Timestamp: ts.UTC().Format(time.RFC3339Nano), Message: msg, Labels: map[string]string{LabelInstance: fmt.Sprint("web-", i%3)}})
	}
	for _, direction := range []string{DirectionBackward, DirectionForward} {
		t.Run(direction, func(t *testing.T) {
			calls := 0
			q := LogQuery{Search: []string{"-end"}, TextMode: TextModeDisplay, Limit: 100, Direction: direction}.normalized()
			got, err := scanLokiWindow(context.Background(), q, scanStart, scanEnd, true, time.Minute,
				rangeLoki(data, lokiDisplayScanPage, direction == DirectionForward, &calls))
			if err != nil {
				t.Fatal(err)
			}
			var messages []string
			for _, e := range got {
				messages = append(messages, e.Message)
			}
			if !slices.Equal(messages, want) {
				t.Fatalf("got %d matches %q, want %d %q", len(messages), messages, len(want), want)
			}
			if calls < 3 {
				t.Fatalf("%d requests cannot have read %d raw lines", calls, n)
			}
		})
	}
	// The raw default is untouched: the same visible needle finds nothing.
	calls := 0
	raw := LogQuery{Search: []string{"hit-0-end"}, Limit: 100}.normalized()
	got, err := scanLokiWindow(context.Background(), raw, scanStart, scanEnd, true, time.Minute,
		func(ctx context.Context, from, to time.Time) ([]LogEntry, error) {
			page, err := rangeLoki(data, 100, false, &calls)(ctx, from, to)
			return slices.DeleteFunc(page, func(e LogEntry) bool { return !raw.matchesText(e.Message) }), err
		})
	if err != nil || len(got) != 0 {
		t.Fatalf("raw scan = %v, %v; want the existing empty answer", got, err)
	}
}

// When the budget runs out mid-scan, coverage stops at the raw lines actually
// read — never at the slice start — so the page continues instead of
// declaring unscanned history empty.
func TestScanLokiWindowDisplayBudgetReportsRawCoverage(t *testing.T) {
	var data []LogEntry
	for i := 0; i < 20000; i++ {
		data = append(data, lineAt(scanEnd.Add(-10*time.Minute+time.Duration(i)*30*time.Millisecond), "noise"))
	}
	calls := 0
	slow := rangeLoki(data, lokiDisplayScanPage, false, &calls)
	q := LogQuery{Search: []string{"absent"}, TextMode: TextModeDisplay, Limit: 100}.normalized()
	_, err := scanLokiWindow(context.Background(), q, scanStart, scanEnd, true, 150*time.Millisecond,
		func(ctx context.Context, from, to time.Time) ([]LogEntry, error) {
			time.Sleep(60 * time.Millisecond)
			return slow(ctx, from, to)
		})
	var partial *ScanIncompleteError
	if !errors.As(err, &partial) {
		t.Fatalf("err = %v, want *ScanIncompleteError", err)
	}
	if len(partial.Entries) != 0 {
		t.Fatalf("entries = %v", partial.Entries)
	}
	// Each raw page of 1000 lines spans 30s; coverage must sit inside the data,
	// not at the 1h/2h slice starts a matched-only scanner would claim.
	if !partial.ScannedTo.After(scanEnd.Add(-10*time.Minute)) || !partial.ScannedTo.Before(scanEnd) {
		t.Fatalf("ScannedTo = %v, want inside the raw lines read", partial.ScannedTo)
	}
}

func mcpLogsClient(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}
