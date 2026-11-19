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
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// lokiTextPrefilter is the LogQL line filter a `text` read sends Loki
// (w5/m520): a superset of the lines Go's matcher (LogQuery.matchesText) can
// accept, so Loki drops the lines that cannot match before bex-api reads them.
// Go's matcher still decides every line Loki returns. A day of the busiest
// service is about 300 MB of raw lines, which no scan budget reads; through
// this filter a search for a term it never finds reads none of them.
//
// Each term gives one needle, a run of ASCII text every line it can match must
// contain (termNeedle), and the filter keeps a line holding any needle:
//
//	|~ "(?i)<needle>" or "(?i)<needle>" ...
//
// A display search (textMode=display) also keeps every line carrying an ESC
// or a CR: displayText returns any other message unchanged, so the same needles
// hold for it. There is no filter at all ("", every line read raw) when a term
// has no needle: a `/…/` regex, a term with no usable ASCII run, or one that
// may match only the cap marker or only inside a request line's key names,
// escapes or numbers.
//
// Why a line Go's matcher accepts holds its term's needle:
//
//   - Go lowercases the message with strings.ToLower, which maps exactly two
//     non-ASCII runes to ASCII (U+0130 to `i`, U+212A to `k`) and invalid UTF-8
//     to U+FFFD, so an ASCII run in the lowercase message is a run of the same
//     letters, either case, in the message, with U+0130 for an `i` and U+212A
//     for a `k`. bex-api decodes Loki's answer as JSON, which turns invalid
//     UTF-8 into U+FFFD, so the line Loki filters and the line bex-api reads
//     hold the same ASCII runs and the same U+0130 and U+212A.
//   - Loki answers `(?i)<literal>` with its own lowercase compare (containsLower
//     in pkg/logql/log/filter.go of the pinned 3.6.7) rather than Go's regexp.
//     That compare lowercases U+0130 and U+212A except as the needle's first
//     character, and Go's `(?i)i` does not match U+0130 at all. So a needle
//     holds no `i` and does not start with `k`; then it is a superset under
//     either engine, which the pinned Loki and a port of containsLower check in
//     textprefilter_test.go. Classes such as `[iI\x{0130}]` would be exact, but
//     they keep Loki on Go's regexp: 13 s of Loki time for a day of the busiest
//     service against 2 s (w5/m520 Findings).
//   - A message is the stored line, less a trailing "\n" then "\r"
//     (canonicalHistoryMessage), cut at 64 KiB with logTruncationMarker
//     appended (capLogMessage). A run of the term found in the message lies in
//     the stored line, unless it touches the marker: it then ends in a space
//     (spanning the cut) or lies inside the marker. So trailing spaces are
//     dropped and a run inside the marker gives no needle.
//   - A request line's message is rendered from the stored JSON record
//     (requestLine). See requestNeedle.
func lokiTextPrefilter(q LogQuery) string {
	if len(q.Search) == 0 {
		return ""
	}
	request := q.mayReadRequestLines()
	var needles []string
	for _, term := range q.Search {
		n := termNeedle(term, request)
		if n == "" {
			return ""
		}
		needles = append(needles, n)
	}
	alts := make([]string, 0, len(needles)+2)
	for _, n := range minimalNeedles(needles) {
		alts = append(alts, strconv.Quote("(?i)"+regexp.QuoteMeta(n)))
	}
	if q.TextMode == TextModeDisplay {
		alts = append(alts, strconv.Quote(`\x1b`), strconv.Quote(`\r`))
	}
	return " |~ " + strings.Join(alts, " or ")
}

// mayReadRequestLines reports whether q's selector can return request
// streams, whose messages requestLine renders (lokiTypeMatcher).
func (q LogQuery) mayReadRequestLines() bool {
	return q.Database == "" && q.KeyValue == "" && q.wants(LogTypeRequest)
}

// termNeedle is the needle of one normalized term, or "" when it has none. A
// literal must occur in the lowercase message, and so must every part of a
// wildcard; the longest needle any of them gives is kept. A `/…/` regex runs
// on the message as written and has no needle.
func termNeedle(term string, request bool) string {
	if _, regex := regexFilterBody(term); regex {
		return ""
	}
	best := ""
	for _, literal := range strings.Split(term, "*") {
		if n := literalNeedle(literal, request); len(n) > len(best) {
			best = n
		}
	}
	return best
}

// literalNeedle is the longest needle a literal that occurs in the lowercase
// message gives. Any substring of it occurs too, so it is cut into runs of the
// characters a needle may hold, and each run is narrowed by where it can occur.
func literalNeedle(literal string, request bool) string {
	best := ""
	for _, run := range strings.FieldsFunc(literal, func(r rune) bool { return !needleRune(r, request) }) {
		if strings.Contains(logTruncationMarker, run) {
			continue // every over-cap message holds it
		}
		if request {
			run = requestNeedle(run)
		} else {
			run = strings.TrimRight(run, " ")
		}
		// containsLower misses U+212A as a needle's first character.
		if n := strings.TrimLeft(run, "k"); len(n) > len(best) {
			best = n
		}
	}
	return best
}

// needleRune reports whether r may be part of a needle: printable ASCII other
// than `i` (Go's `(?i)i` does not match U+0130, which lowercases to `i`) and,
// when request lines are read, other than the characters that delimit a
// rendered request line's fields (space, `=`, `"`) or that its stored JSON may
// write escaped (`\`, `<`, `>`, `&`).
func needleRune(r rune, request bool) bool {
	if r < ' ' || r > '~' || r == 'i' {
		return false
	}
	return !request || !strings.ContainsRune(` ="\<>&`, r)
}

// requestNeedle narrows a run of a term to text the stored access record must
// hold whenever the rendered request line holds the run. The rendered message
// is `key=value` fields joined by spaces, each value a decimal number or a
// strconv.Quote'd string from the record. The run holds no space, `=` or `"`,
// so where it occurs it lies inside one key, one number or one quoted value:
//
//   - inside a key it can match every rendered line, so it has no needle;
//   - inside a number it is digits and `-`, and responseTimeMS is the stored
//     nanoseconds divided, so it has no needle;
//   - inside a quoted value it is the value's own characters, perhaps after the
//     tail of an escape strconv.Quote wrote (it holds no `\`, so it cannot
//     span a whole one). Traefik writes its record with Go's encoding/json,
//     which writes printable ASCII other than `"`, `\`, `<`, `>` and `&` and
//     every printable non-ASCII rune as itself, so what follows the escape's
//     tail is in the stored record as written.
//
// A request stream line that is not an access record is returned as stored,
// and the narrowed run, a substring of the term, lies in it too.
func requestNeedle(run string) string {
	if strings.Trim(run, "0123456789-") == "" {
		return ""
	}
	for _, key := range requestLineKeys {
		if strings.Contains(strings.ToLower(key), run) {
			return ""
		}
	}
	return run[quoteEscapeTail(run):]
}

// quoteEscapeTail is the length of the longest prefix of s, lowercase, that
// can end an escape strconv.Quote writes after its backslash: one of
// `abfnrtv`, `x` and up to two hex digits, `u` (or `U`) and up to eight, or up
// to eight hex digits alone (`a`, `b` and `f` among them).
func quoteEscapeTail(s string) int {
	hexRun := func(from, most int) int {
		j := from
		for j < len(s) && j-from < most && strings.IndexByte("0123456789abcdef", s[j]) >= 0 {
			j++
		}
		return j
	}
	if s == "" {
		return 0
	}
	switch s[0] {
	case 'x':
		return hexRun(1, 2)
	case 'u':
		return hexRun(1, 8)
	case 'n', 'r', 't', 'v':
		return 1
	}
	return hexRun(0, 8)
}

// minimalNeedles drops a needle that holds another: a line holding it holds
// the other, which the filter keeps already.
func minimalNeedles(needles []string) []string {
	slices.Sort(needles)
	needles = slices.Compact(needles)
	return slices.DeleteFunc(slices.Clone(needles), func(n string) bool {
		return slices.ContainsFunc(needles, func(m string) bool { return m != n && strings.Contains(n, m) })
	})
}
