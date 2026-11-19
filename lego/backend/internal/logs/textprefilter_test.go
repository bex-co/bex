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
	"reflect"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// The w5/m520 prefilter: Loki drops the lines a `text` term cannot match, and
// Go's matcher decides the rest. These tests hold it to being a superset of
// that matcher, line by line, under both ways Loki can evaluate it.

// lokiContainsLower is containsLower from Loki 3.6.7
// (pkg/logql/log/filter.go, Apache-2.0), the compare Loki answers a
// `(?i)<literal>` line filter with: it lowercases ASCII and decodes other
// runes, but its first-byte scan only folds ASCII.
func lokiContainsLower(line, substr []byte) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(line) {
		return false
	}
	firstByte := substr[0]
	maxIndex := len(line) - len(substr)
	for i := 0; i <= maxIndex; i++ {
		c := line[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != firstByte {
			continue
		}
		matched := true
		linePos, substrPos := i, 0
		for linePos < len(line) && substrPos < len(substr) {
			c, s := line[linePos], substr[substrPos]
			if c < utf8.RuneSelf && s < utf8.RuneSelf {
				if c >= 'A' && c <= 'Z' {
					c += 'a' - 'A'
				}
				if c != s {
					matched = false
					break
				}
				linePos++
				substrPos++
				continue
			}
			lr, lineSize := utf8.DecodeRune(line[linePos:])
			if lr == utf8.RuneError && lineSize == 1 {
				if c >= 'A' && c <= 'Z' {
					c += 'a' - 'A'
				}
				if c != s {
					matched = false
					break
				}
				linePos++
				substrPos++
				continue
			}
			mr, substrSize := utf8.DecodeRune(substr[substrPos:])
			if mr == utf8.RuneError && substrSize == 1 {
				matched = false
				break
			}
			if unicode.ToLower(lr) != mr {
				matched = false
				break
			}
			linePos += lineSize
			substrPos += substrSize
		}
		if matched && substrPos == len(substr) {
			return true
		}
	}
	return false
}

// lokiFilterValues are the values of a pipeline's leading ` |~ "a" or "b"`
// line filter, unquoted, or nil for a pipeline that starts with none.
func lokiFilterValues(pipeline string) []string {
	rest, ok := strings.CutPrefix(pipeline, " |~ ")
	if !ok {
		return nil
	}
	var alts []string
	for {
		quoted, err := strconv.QuotedPrefix(rest)
		if err != nil {
			panic(fmt.Sprintf("prefilter %q: %v", pipeline, err))
		}
		alt, _ := strconv.Unquote(quoted)
		alts = append(alts, alt)
		rest = rest[len(quoted):]
		if rest, ok = strings.CutPrefix(rest, " or "); !ok {
			return alts
		}
	}
}

// lokiFilterKeeps reports whether a line filter value keeps a stored line as
// the pinned Loki evaluates it: a value that parses to a literal is a
// substring compare (containsLower when it folds case), anything else Go's
// regexp.
func lokiFilterKeeps(alt, line string) bool {
	re, err := syntax.Parse(alt, syntax.Perl)
	if err != nil {
		panic(fmt.Sprintf("prefilter value %q: %v", alt, err))
	}
	re = re.Simplify()
	if re.Op == syntax.OpLiteral {
		lit := []byte(string(re.Rune))
		if re.Flags&syntax.FoldCase != 0 {
			return lokiContainsLower([]byte(line), bytes.ToLower(lit))
		}
		return bytes.Contains([]byte(line), lit)
	}
	return regexp.MustCompile(alt).MatchString(line)
}

// prefilterKeeps reports whether pipeline's line filter keeps a stored line,
// under Loki's evaluation and under Go's regexp alone (the engine a Loki that
// stopped simplifying would use).
func prefilterKeeps(pipeline, line string) (loki, goRegexp bool) {
	alts := lokiFilterValues(pipeline)
	if alts == nil {
		return true, true
	}
	for _, alt := range alts {
		loki = loki || lokiFilterKeeps(alt, line)
		goRegexp = goRegexp || regexp.MustCompile(alt).MatchString(line)
	}
	return loki, goRegexp
}

// messageOf is the message bex-api makes of one stored line on a stream: the
// line travels in Loki's JSON answer (invalid UTF-8 arrives as U+FFFD) and
// through parseLokiStreams.
func messageOf(t testing.TB, stream map[string]string, stored string, q LogQuery) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"status": "success", "data": map[string]any{
		"resultType": "streams",
		"result":     []any{map[string]any{"stream": stream, "values": [][]string{{"1751673601000000000", stored}}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var lr lokiRangeResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		t.Fatal(err)
	}
	entries, err := parseLokiStreams(lr, q)
	if err != nil || len(entries) != 1 {
		t.Fatalf("parse %q: %v, %d entries", stored, err, len(entries))
	}
	return entries[0].Message
}

// assertPrefilterKeeps fails when Go's matcher accepts a stored line's message
// and the query's prefilter drops the line under either engine.
func assertPrefilterKeeps(t testing.TB, q LogQuery, stream map[string]string, stored string) {
	t.Helper()
	msg := messageOf(t, stream, stored, q)
	if !q.matchesText(msg) {
		return
	}
	pipeline := lokiTextPrefilter(q)
	loki, goRegexp := prefilterKeeps(pipeline, stored)
	if !loki || !goRegexp {
		t.Fatalf("text %q (mode %q) matches message %q, but the prefilter%s drops stored line %q (Loki keeps it: %v, Go's regexp: %v)",
			q.Search, q.TextMode, clip(msg), pipeline, clip(stored), loki, goRegexp)
	}
}

func clip(s string) string {
	if len(s) > 200 {
		return s[:100] + "…" + s[len(s)-100:]
	}
	return s
}

// streamKinds are the stored streams a text search reads, with the query that
// selects each: 0 app, 1 app read with requests, 2 build, 3 Postgres, 4 Key
// Value, 5 request, 6 request read with app.
var streamKinds = []struct {
	stream map[string]string
	query  LogQuery
}{
	{map[string]string{"app": "web", "container": "app", "type": LogTypeApp}, LogQuery{App: "web", Types: []string{LogTypeApp}}},
	{map[string]string{"app": "web", "container": "app", "type": LogTypeApp}, LogQuery{App: "web"}},
	{map[string]string{"app": "web", "type": LogTypeBuild}, LogQuery{App: "web", Types: []string{LogTypeBuild}}},
	{map[string]string{"database": "dpg-x", "type": "postgres"}, LogQuery{App: "dpg-x", Database: "dpg-x"}},
	{map[string]string{"keyvalue": "red-x", "type": "keyvalue"}, LogQuery{App: "red-x", KeyValue: "red-x"}},
	{map[string]string{"app": "web", "type": LogTypeRequest, "status": "200", "method": "GET"}, LogQuery{App: "web", Types: []string{LogTypeRequest}}},
	{map[string]string{"app": "web", "type": LogTypeRequest, "status": "200", "method": "GET"}, LogQuery{App: "web"}},
}

// textQuery is kind's query searching term in mode.
func textQuery(kind int, term string, display bool) LogQuery {
	q := streamKinds[kind].query
	q.Search = []string{term}
	if display {
		q.TextMode = TextModeDisplay
	}
	return q.normalized()
}

// storedAccessRecord is a Traefik access record as Go's encoding/json writes
// it, holding the fields whose bits are set in fields.
func storedAccessRecord(t testing.TB, ip, id, ua string, duration, size int64, fields uint8) string {
	t.Helper()
	record := map[string]any{"RequestHost": "web.onbex.co", "RequestPath": "/x", "level": "info", "msg": ""}
	for bit, f := range []struct {
		key   string
		value any
	}{
		{"request_True-Client-Ip", ip}, {"request_Rndr-Id", id}, {"downstream_Rndr-Id", id},
		{"Duration", duration}, {"DownstreamContentSize", size}, {"request_User-Agent", ua},
	} {
		if fields&(1<<bit) != 0 {
			record[f.key] = f.value
		}
	}
	b, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// requestLineKeys names every key a rendered request line holds, in order:
// a key it missed would let the prefilter drop a line a term matches only in
// that key.
func TestRequestLineKeysAreTheRenderedKeys(t *testing.T) {
	record := storedAccessRecord(t, "203.0.113.7", "req-1", "curl/8.5.0", 12345678, 512, 0b111111)
	rendered := requestLine(record, map[string]string{})
	var keys []string
	for _, field := range regexp.MustCompile(`(?:^| )([A-Za-z]+)=`).FindAllStringSubmatch(rendered, -1) {
		keys = append(keys, field[1])
	}
	if !slices.Equal(keys, requestLineKeys) {
		t.Fatalf("rendered %q has keys %q, want requestLineKeys %q", rendered, keys, requestLineKeys)
	}
}

// overCap pads s so a message holding it is cut by capLogMessage `back` bytes
// before its end, which puts the cut inside s.
func overCap(s string, back uint16) string {
	pad := maxLogMessageBytes - int(back%512)
	return strings.Repeat("~", max(pad, 0)) + s
}

// FuzzTextPrefilterKeepsEveryStoredLineGoMatches: for any stored line and any
// term, a line whose message Go's matcher accepts passes the prefilter, on
// every stream a text search reads except request lines (the next fuzz test).
func FuzzTextPrefilterKeepsEveryStoredLineGoMatches(f *testing.F) {
	for _, seed := range []struct {
		term, line string
		kind       uint8
		display    bool
		cut        uint16
	}{
		{"fix", "f\u0130x it", 0, false, 0},     // U+0130 lowercases to i
		{"kill", "\u212aill -9", 0, false, 0},   // U+212A lowercases to k, here first
		{"kill", "\u212a\u0130LL", 1, false, 0}, // both, read beside requests
		{"KELVIN", "\u212aELV\u0130N", 2, false, 0},
		{"ss", "\u017f\u017f", 0, false, 0},           // long s stays itself in Go
		{"error", "er\xffror E\xffRROR", 0, false, 0}, // invalid UTF-8 is U+FFFD
		{"err", "boot ERR", 3, false, 0},
		{"select 1", "statement: SELECT 1\r\n", 3, false, 0}, // the record's terminator
		{"ok\r", "ok\r", 4, false, 0},
		{"abc ", "abcdef", 0, false, 3},                        // spans the cut into the marker
		{"truncated", "anything", 0, false, 1},                 // only in the marker
		{"ated]", "anything", 4, false, 7},                     // only in the marker
		{"…[", "x", 0, false, 2},                               // only in the marker
		{"q_red_end", "QA_\x1b[31mred\x1b[0m_end", 0, true, 0}, // display over ANSI
		{"qa_red", "QA_\x1b[31mred", 1, true, 0},
		{"red_end", "xx red\rQA_red_end", 0, true, 0}, // a CR overwrites
		{"boot*ok", "BOOT is OK", 0, false, 0},
		{"*", "anything", 2, false, 0},
		{"a*b*c", "A then B then C", 1, false, 0},
		{"/E.R/", "ExR", 0, false, 0},
		{"/(?i)k/", "\u212a", 0, false, 0},
		{"çava", "ÇAVA", 0, false, 0}, // non-ASCII
		{"tab\there", "TAB\tHERE", 0, false, 0},
		{"k", "K", 0, false, 0},
		{"i", "\u0130", 3, false, 0},
	} {
		f.Add(seed.term, seed.line, seed.kind, seed.display, seed.cut)
	}
	nonRequest := []int{0, 1, 2, 3, 4}
	f.Fuzz(func(t *testing.T, term, line string, kind uint8, display bool, cut uint16) {
		k := nonRequest[int(kind)%len(nonRequest)]
		if cut != 0 {
			line = overCap(line, cut)
		}
		assertPrefilterKeeps(t, textQuery(k, term, display), streamKinds[k].stream, line)
	})
}

// FuzzTextPrefilterKeepsEveryRequestLineGoMatches: for any access record (or a
// request stream line that is not one) and any term, a line whose rendered
// message Go's matcher accepts passes the prefilter.
func FuzzTextPrefilterKeepsEveryRequestLineGoMatches(f *testing.F) {
	all := uint8(0b111111)
	for _, seed := range []struct {
		term, ip, id, ua string
		duration, size   int64
		fields           uint8
		display          bool
		cut              uint16
		asStored         bool
	}{
		{"responsetimems=12", "203.0.113.7", "abc", "curl/8.5.0", 12345678, 512, all, false, 0, false},
		{"=12 ", "203.0.113.7", "abc", "curl/8.5.0", 12345678, 512, all, false, 0, false},
		{"12", "203.0.113.7", "abc", "curl/8.5.0", 12345678, 512, all, false, 0, false},
		{"responsetimems=0", "", "", "", 999, 0, 0b001000, false, 0, false}, // under 1 ms
		{"0", "", "", "", 999, 0, 0b001000, false, 0, false},                // no 0 in the record
		{"0 resp", "", "", "", 999, 7, 0b011000, false, 0, false},
		{"clientip", "203.0.113.7", "", "", 0, 0, 0b000001, false, 0, false},
		{"useragent=\"", "", "", "x", 0, 0, 0b100000, false, 0, false},
		{"ua \\\"bot\\\" \\\\", "", "", `ua "bot" \`, 0, 0, 0b100000, false, 0, false},
		{"bot\" \\", "", "", `ua "bot" \`, 0, 0, 0b100000, false, 0, false},
		{"u00a0bar", "", "", "Foo\u00a0Bar", 0, 0, 0b100000, false, 0, false}, // Quote escapes NBSP
		{"0bar", "", "", "Foo\u00a0Bar", 0, 0, 0b100000, false, 0, false},
		{"x07rest", "", "", "a\arest", 0, 0, 0b100000, false, 0, false},
		{"nrest", "", "", "a\nrest", 0, 0, 0b100000, false, 0, false},
		{"&b", "", "", "a&b<c>", 0, 0, 0b100000, false, 0, false}, // JSON writes \u0026
		{"f\u0130x", "", "", "F\u0130X/1.0", 0, 0, 0b100000, false, 0, false},
		{"kit", "", "", "\u212aIT/2", 0, 0, 0b100000, false, 0, false},
		{"mozilla", "", "", "Mozilla/5.0 (X11; Linux x86_64)", 0, 0, all, false, 0, false},
		{"x11; linux", "", "", "Mozilla/5.0 (X11; Linux x86_64)", 0, 0, all, false, 0, false},
		{"\" useragent", "", "", "z", 0, 0, 0b100000, false, 0, false},
		{"203.0.113", "203.0.113.7", "", "", 0, 0, 0b000001, false, 0, false},
		{"req-1\" response", "", "req-1", "", 5, 0, 0b001010, false, 0, false},
		{"tail", "", "", "head tail", 0, 0, 0b100000, false, 5, false}, // the cut inside the user agent
		{"truncated", "", "", "x", 0, 0, 0b100000, false, 9, false},
		{"plain", "", "", "a plain line, not a record", 0, 0, 0, false, 0, true},
		{"{\"msg", "", "", "{\"msg\":\"no fields\"}", 0, 0, 0, false, 0, true},
		{"x", "", "", "x", 0, 0, 0, false, 0, false}, // a record with none of the fields
		{"bot", "", "", "\x1b[1mbot", 0, 0, 0b100000, true, 0, false},
		{"/12/", "", "", "", 12345678, 0, 0b001000, false, 0, false},
	} {
		f.Add(seed.term, seed.ip, seed.id, seed.ua, seed.duration, seed.size, seed.fields, seed.display, seed.cut, seed.asStored)
	}
	f.Fuzz(func(t *testing.T, term, ip, id, ua string, duration, size int64, fields uint8, display bool, cut uint16, asStored bool) {
		if cut != 0 {
			ua = overCap(ua, cut)
		}
		stored := ua
		if !asStored {
			stored = storedAccessRecord(t, ip, id, ua, duration, size, fields)
		}
		for _, k := range []int{5, 6} {
			assertPrefilterKeeps(t, textQuery(k, term, display), streamKinds[k].stream, stored)
		}
	})
}

// What a term sends Loki, by the rule that gives it. Every value is a literal
// Loki answers with its own substring compare, never Go's regexp.
func TestTextPrefilterNeedles(t *testing.T) {
	for _, c := range []struct {
		name, term string
		kind       int // streamKinds
		display    bool
		want       string
	}{
		{"a literal, lowercase, its longest run without i", "Connection Refused", 0, false, ` |~ "(?i)on refused"`},
		{"no i: Go's (?i)i misses U+0130", "timeout", 0, false, ` |~ "(?i)meout"`},
		{"no leading k: Loki misses U+212A first", "kubernetes", 0, false, ` |~ "(?i)ubernetes"`},
		{"a k inside stays", "parking", 0, false, ` |~ "(?i)park"`},
		{"metacharacters are quoted", "a.b+c", 2, false, ` |~ "(?i)a\\.b\\+c"`},
		{"a trailing space may span the cut", "boot ", 3, false, ` |~ "(?i)boot"`},
		{"a term inside the cap marker has none", "trunc", 0, false, ``},
		{"a regex has none", "/err(or)?/", 0, false, ``},
		{"a term of i and k has none", "kik", 0, false, ``},
		{"a wildcard keeps its longest part", "get*/api/v1/*", 0, false, ` |~ "(?i)/v1/"`},
		{"a wildcard of nothing has none", "*", 0, false, ``},
		{"any term without a needle reads raw", "boot", 0, false, ` |~ "(?i)boot"`},
		{"display keeps lines with ESC or CR", "Red_End", 0, true, ` |~ "(?i)red_end" or "\\x1b" or "\\r"`},
		{"terms are ORed, a needle inside another covers it", "error", 0, false, ` |~ "(?i)error"`},
		{"request: escape tails go", "error", 5, false, ` |~ "(?i)rror"`},
		{"request: no spaces", "Mozilla/5.0 (X11", 5, false, ` |~ "(?i)lla/5\\.0"`},
		{"request: a key name has none", "useragent", 5, false, ``},
		{"request: a number has none", "-12", 5, false, ``},
		{"request: hex up to eight goes", "deadbeefcafe", 5, false, ` |~ "(?i)cafe"`},
		{"request: JSON may escape <, > and &", "a<b>&c", 6, false, ``},
		{"request: no quotes", "\"203.0.113.7\" requestid", 6, false, ` |~ "(?i)\\.0\\.113\\.7"`},
		{"datastore lines are not request lines", "error 42", 3, false, ` |~ "(?i)error 42"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			q := textQuery(c.kind, c.term, c.display)
			if c.name == "terms are ORed, a needle inside another covers it" {
				q.Search = normalizeLogText([]string{"error", "ERRORS", "errors and more"})
			}
			got := lokiTextPrefilter(q)
			if got != c.want {
				t.Fatalf("prefilter of %q:\n got %s\nwant %s", c.term, got, c.want)
			}
			for _, alt := range lokiFilterValues(got) {
				re, err := syntax.Parse(alt, syntax.Perl)
				if err != nil || re.Simplify().Op != syntax.OpLiteral {
					t.Fatalf("value %q is not a literal Loki compares without its regexp", alt)
				}
			}
		})
	}
}

// A prefiltered search pages exactly as an unfiltered one: the same lines in
// the same order and the same hasMore and cursors, page by page, in both
// directions, over a window across split cuts whose candidate lines still
// fill raw pages. Only the lines Loki answers with shrink. Three instances log
// every second for 40 minutes; one line in 97 holds the term, one in three
// holds only its needle (`match` of `kmatch`), the rest neither.
func TestAPrefilteredSearchPagesExactlyAsAnUnfilteredOne(t *testing.T) {
	var streams []fakeStream
	for i, pod := range []string{"web-a", "web-b", "web-c"} {
		s := podStream(pod, secondsFrom(streamsOrigin, i, 2400)...)
		s.text = func(ns int64) string {
			n := (ns - streamsOrigin.UnixNano()) / int64(time.Second)
			switch {
			case n%97 == 0:
				return fmt.Sprintf("KMATCH %s@%d", pod, ns)
			case n%3 == 0:
				return fmt.Sprintf("match %s@%d", pod, ns)
			}
			return fmt.Sprintf("%s@%d", pod, ns)
		}
		streams = append(streams, s)
	}
	f := newFrontendLoki(t, streams...)
	svc := newService(nil, sampleApp("web"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	start := streamsOrigin.Add(-time.Hour).Format(time.RFC3339)
	end := streamsOrigin.Add(time.Hour).Format(time.RFC3339)
	walk := func(filter bool, direction string, limit int) (pages []renderLogList, answered int) {
		f.mu.Lock()
		f.filterLines, f.rangeCalls, f.rangeLines = filter, nil, nil
		f.mu.Unlock()
		pages = walkLogPages(t, svc, "resource=web&text=KMATCH", start, end, limit, direction)
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, n := range f.rangeLines {
			answered += n
		}
		if len(f.rangeCalls) == 0 || !strings.Contains(f.rangeCalls[0].Get("query"), `|~ "(?i)match"`) {
			t.Fatalf("the search sent no prefilter: %v", f.rangeCalls)
		}
		return pages, answered
	}
	for _, direction := range []string{DirectionBackward, DirectionForward} {
		for _, limit := range []int{10, 100} {
			raw, rawLines := walk(false, direction, limit)
			filtered, filteredLines := walk(true, direction, limit)
			if !reflect.DeepEqual(filtered, raw) {
				t.Fatalf("%s, limit %d: prefiltered pages\n%v\ndiffer from unfiltered pages\n%v", direction, limit, pageMessages(filtered), pageMessages(raw))
			}
			if matches := len(slices.Concat(pageMessages(raw)...)); matches != 73 {
				t.Fatalf("%s, limit %d: %d matches, want 73", direction, limit, matches)
			}
			if filteredLines*3 > rawLines*2 {
				t.Fatalf("%s, limit %d: Loki answered %d lines prefiltered and %d raw, want under two thirds", direction, limit, filteredLines, rawLines)
			}
		}
	}
}

// On the pinned Loki (3.6.7, BEX_TEST_LOKI_URL), a prefiltered search returns
// exactly the lines Go's matcher finds in the unfiltered read of the same
// window: U+0130 and U+212A, invalid UTF-8, a line cut at the cap, ANSI text
// in display mode, wildcards and regexes, and rendered request lines, read as
// app lines alone, request lines alone and both.
func TestLokiTextPrefilterFindsEveryMatchOnThePinnedLoki(t *testing.T) {
	base := os.Getenv("BEX_TEST_LOKI_URL")
	if base == "" {
		testenv.Skip(t, "BEX_TEST_LOKI_URL not set")
	}
	base = strings.TrimRight(base, "/")
	ns := fmt.Sprintf("m520-%d", time.Now().UnixNano())
	appLines := []string{
		"f\u0130x it", "\u212aill -9", "\u212a\u0130LL", "\u212aELV\u0130N", "er\xffror E\xffRROR boot ERROR\xff",
		"QA_\x1b[31mred\x1b[0m_end", "xx red\rQA_red_end", "BOOT is OK", "connection REFUSED",
		strings.Repeat("~", maxLogMessageBytes-3) + "abcdef", // cut after "abc"
		"statement: SELECT 1\r", "plain noise",
	}
	requestLines := []string{
		storedAccessRecord(t, "203.0.113.7", "req-1", "Mozilla/5.0 (X11; Linux x86_64)", 12345678, 512, 0b111111),
		storedAccessRecord(t, "", "", `ua "bot" \`, 999, 0, 0b101000),
		storedAccessRecord(t, "", "", "Foo\u00a0Bar", 0, 0, 0b100000),
		storedAccessRecord(t, "", "", "a&b<c>", 0, 7, 0b110000),
		storedAccessRecord(t, "", "", "", 0, 0, 0),
		"a plain request line",
	}
	now := time.Now()
	since := now.Add(-time.Minute)
	push := func(labels map[string]string, lines []string) {
		var values [][2]string
		for i, line := range lines {
			values = append(values, [2]string{strconv.FormatInt(since.Add(time.Duration(i+1)*time.Second).UnixNano(), 10), line})
		}
		labels["namespace"], labels["app"] = ns, "web"
		body, _ := json.Marshal(map[string]any{"streams": []any{map[string]any{"stream": labels, "values": values}}})
		resp, err := http.Post(base+"/loki/api/v1/push", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("push: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			t.Fatalf("push: HTTP %d", resp.StatusCode)
		}
	}
	push(map[string]string{"container": "app", "type": LogTypeApp, "pod": "web-1"}, appLines)
	push(map[string]string{"type": LogTypeRequest, "status": "200", "method": "GET"}, requestLines)

	read := func(q LogQuery) []string {
		q.App, q.Since, q.End, q.Limit = "web", since, now, 100
		got, err := NewLokiSource(base, nil)(context.Background(), ns, q.normalized())
		if err != nil {
			t.Fatal(err)
		}
		return entryStamps(got)
	}
	checked := 0 // searches that sent a prefilter and found a line
	for _, types := range [][]string{nil, {LogTypeApp}, {LogTypeRequest}} {
		all := read(LogQuery{Types: types})
		if len(all) == 0 {
			t.Fatal("the corpus is not readable")
		}
		for _, term := range []string{
			"fix", "kill", "KELVIN", "error", "red_end", "abc ", "truncated", "boot*ok", "/E.R/", "connection refused",
			"select 1", "responsetimems=12", "12", "0bar", "u00a0bar", `bot" \`, `\"bot\" \\`, "mozilla", "x11; linux", "&b", "plain",
		} {
			for _, mode := range []string{TextModeRaw, TextModeDisplay} {
				q := LogQuery{Types: types, Search: []string{term}, TextMode: mode}
				var want []string
				for _, msg := range all {
					if q.normalized().matchesText(msg) {
						want = append(want, msg)
					}
				}
				if got := read(q); !slices.Equal(got, want) {
					t.Errorf("types %v, text %q, mode %s: got %q, want %q", types, term, mode, got, want)
				}
				if len(want) > 0 && lokiTextPrefilter(q.normalized()) != "" {
					checked++
				}
			}
		}
	}
	if checked < 50 {
		t.Fatalf("%d searches sent a prefilter and found a line, want 50 or more", checked)
	}
}

// One prefiltered search answers the same on REST, GraphQL and MCP (w4/m378):
// REST's body and MCP's text are the same bytes, and GraphQL lists the same
// lines with the same hasMore and cursors.
func TestAPrefilteredSearchIsOneAnswerOnRESTGraphQLAndMCP(t *testing.T) {
	s := podStream("web-a", secondsFrom(streamsOrigin, 0, 1800)...)
	s.text = func(ns int64) string {
		switch n := (ns - streamsOrigin.UnixNano()) / int64(time.Second); {
		case n%41 == 0:
			return fmt.Sprintf("KMATCH %d", n)
		case n%2 == 0:
			return fmt.Sprintf("match %d", n)
		default:
			return fmt.Sprintf("line %d", n)
		}
	}
	f := newFrontendLoki(t, s)
	f.filterLines = true
	svc := newService(nil, sampleApp("web"))
	svc.PodLogs, svc.PodLogsFollow = nil, nil
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	start, end := streamsOrigin.Format(time.RFC3339), streamsOrigin.Add(time.Hour).Format(time.RFC3339)

	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&text=KMATCH&limit=20&startTime="+start+"&endTime="+end)
	if rec.Code != http.StatusOK {
		t.Fatalf("REST = %d %s", rec.Code, rec.Body)
	}
	rest := decodeLogList(t, rec)
	if len(rest.Logs) != 20 || !rest.HasMore {
		t.Fatalf("REST page = %d lines, hasMore %v; want a full page of 20 and more", len(rest.Logs), rest.HasMore)
	}
	if q := f.rangeCalls[0].Get("query"); !strings.Contains(q, `|~ "(?i)match"`) {
		t.Fatalf("the search sent no prefilter: %s", q)
	}

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
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{"web"}, "text": []string{"KMATCH"}, "limit": 20, "startTime": start, "endTime": end,
	}})
	if err != nil || res.IsError {
		t.Fatalf("list_logs: %v %+v", err, res)
	}
	if text, body := rendertest.MCPText(t, res), strings.TrimSuffix(rec.Body.String(), "\n"); text != body {
		t.Errorf("MCP list_logs text =\n%s\nwant REST's bytes\n%s", text, body)
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", text:"KMATCH", limit:20, startTime:"`+start+`", endTime:"`+end+`") { logs { message } hasMore nextStartTime nextEndTime } }`)
	page, _ := data["logs"].(map[string]any)
	var gqlMessages []string
	for _, line := range page["logs"].([]any) {
		gqlMessages = append(gqlMessages, line.(map[string]any)["message"].(string))
	}
	if !slices.Equal(gqlMessages, slices.Concat(pageMessages([]renderLogList{rest})...)) ||
		page["hasMore"] != rest.HasMore || page["nextStartTime"] != rest.NextStartTime || page["nextEndTime"] != rest.NextEndTime {
		t.Errorf("GraphQL page = %v, want REST's %v (hasMore %v, %s..%s)", page, pageMessages([]renderLogList{rest}), rest.HasMore, rest.NextStartTime, rest.NextEndTime)
	}
}
