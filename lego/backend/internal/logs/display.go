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

import "strings"

// Text search modes (w4/m189) — a labeled Bex extension to Render's `text`
// filter. Raw (the default, and the only mode Render documents) matches the
// stored message bytes, ANSI escapes included. Display matches the text a
// terminal — and the dashboard's log row — actually shows, so a colored
// message is found by the words a reader copies off the screen. Either way the
// returned Message is the raw, untouched record.
const (
	TextModeRaw     = "raw"
	TextModeDisplay = "display"
)

// displayText is the terminal-display projection of a log message: the
// server-side twin of the dashboard's stripAnsi (dashboard/src/features/logs/
// lib/ansi.ts), which the log row renders. It must interpret the same way —
// CSI sequences (SGR and the rest) are dropped, erase-in-line (`K`) erases, OSC
// envelopes (titles, hyperlinks) vanish while a hyperlink's label stays, other
// escapes are swallowed, malformed or unterminated ones consume the rest, and a
// carriage return returns the cursor so overwritten text cannot be matched.
// It is used only to decide a display-mode match; Message is never rewritten.
func displayText(s string) string {
	if !strings.ContainsAny(s, "\x1b\r") {
		return s
	}
	in := []rune(s)
	cells := make([]rune, 0, len(in))
	cursor, lineStart := 0, 0
	for i := 0; i < len(in); {
		switch ch := in[i]; ch {
		case '\x1b':
			var final rune
			var params []rune
			i, final, params = skipEscape(in, i)
			if final == 'K' {
				cells, cursor = eraseInLine(cells, cursor, lineStart, params)
			}
		case '\r':
			cursor = lineStart
			i++
		default:
			for len(cells) < cursor {
				cells = append(cells, ' ')
			}
			if cursor < len(cells) {
				cells[cursor] = ch
			} else {
				cells = append(cells, ch)
			}
			cursor++
			if ch == '\n' {
				lineStart = cursor
			}
			i++
		}
	}
	return string(cells)
}

// skipEscape consumes the escape sequence starting at in[i] (an ESC) and
// returns the index after it, plus a complete CSI sequence's final byte and
// parameters (zero for every other escape).
func skipEscape(in []rune, i int) (next int, final rune, params []rune) {
	if i+1 >= len(in) {
		return len(in), 0, nil // trailing lone ESC
	}
	switch in[i+1] {
	case '[':
		// Parameter bytes 0x30–0x3f, intermediates 0x20–0x2f, then the final byte.
		j := i + 2
		for j < len(in) && in[j] >= 0x30 && in[j] <= 0x3f {
			j++
		}
		params = in[i+2 : j]
		for j < len(in) && in[j] >= 0x20 && in[j] <= 0x2f {
			j++
		}
		if j >= len(in) {
			return len(in), 0, nil
		}
		return j + 1, in[j], params
	case ']':
		// OSC: terminated by BEL or ST (ESC \); unterminated swallows the rest.
		for j := i + 2; j < len(in); j++ {
			if in[j] == '\a' {
				return j + 1, 0, nil
			}
			if in[j] == '\x1b' && j+1 < len(in) && in[j+1] == '\\' {
				return j + 2, 0, nil
			}
		}
		return len(in), 0, nil
	case '(', ')', '#', '%':
		return min(i+3, len(in)), 0, nil // charset selection is three bytes
	default:
		return i + 2, 0, nil
	}
}

// eraseInLine applies CSI `K` exactly as the dashboard parser does: 0 clears
// from the cursor to the end, 1 blanks from the line start through the cursor,
// 2 clears the whole current line.
func eraseInLine(cells []rune, cursor, lineStart int, params []rune) ([]rune, int) {
	switch leadingInt(params) {
	case 0:
		cells = cells[:min(cursor, len(cells))]
	case 1:
		for k := lineStart; k <= cursor && k < len(cells); k++ {
			cells[k] = ' '
		}
	case 2:
		cells = cells[:min(lineStart, len(cells))]
		cursor = lineStart
	}
	return cells, cursor
}

// leadingInt mirrors JavaScript's `Number.parseInt(params, 10) || 0`: the
// leading decimal digits, or 0 when there are none.
func leadingInt(s []rune) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		if n > 1<<20 {
			break
		}
	}
	return n
}
