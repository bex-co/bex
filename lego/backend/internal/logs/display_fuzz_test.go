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
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// displayFuzzSeeds are the shared display-projection vectors — the same
// testdata/display-text-vectors.json the Go and dashboard renderers both assert
// — plus malformed edges the hand-rolled escape scanner must survive:
// unterminated CSI/OSC, over-long parameters, charset selectors at the buffer
// end, and carriage returns that move the cursor back over erased cells. Seeds
// live as source so a reviewer sees exactly what the untrusted surface is
// probed with; the on-disk testdata/fuzz cache is never committed.
func displayFuzzSeeds(f *testing.F) {
	f.Helper()
	if raw, err := os.ReadFile(displayTextVectorsPath); err == nil {
		var vectors []struct {
			In string `json:"in"`
		}
		if json.Unmarshal(raw, &vectors) == nil {
			for _, vector := range vectors {
				f.Add(vector.In)
			}
		}
	}
	for _, seed := range []string{
		"",
		"\x1b",
		"\x1b[",
		"\x1b[1",
		"\x1b[K",
		"\x1b[0K",
		"\x1b[1K",
		"\x1b[2K",
		"\x1b[99999999999999999999K",
		"\x1b]",
		"\x1b]8;;",
		"\x1b]8;;\a",
		"\x1b]0;title\x1b\\",
		"\x1b(",
		"\x1b(B",
		"\x1b#8",
		"\x1b%G",
		"\r",
		"\r\r\r",
		"a\r",
		"\rabc",
		"abc\n\rX",
		"a\r\x1b[K",
		"a\x1b[1K\rb",
		"a\x1b[2Kb",
		"\x1b[2Ka",
		"\xff\xfe\x1b[31mred",
		"中文\x1b[31m红\x1b[0m字",
	} {
		f.Add(seed)
	}
}

// FuzzDisplayText drives the server-side terminal projection used by
// textMode=display search with arbitrary tenant log bytes. Container output is
// attacker-controlled, and displayText walks it with a rune cursor, a cell
// buffer, and hand-rolled CSI/OSC scanners, so it must never panic or slice out
// of bounds. The projection is what a reader copies off the screen, so two
// invariants bound it: no escape or carriage return survives (they are
// interpreted, never displayed), and the projection never grows past its
// input. A line with neither an escape nor a carriage return takes the
// identity fast path byte for byte — deliberately, since that is still the
// raw text a display-mode search matches — so it may carry the source's
// invalid UTF-8; a projected line is always valid UTF-8 because it is built
// from decoded runes.
func FuzzDisplayText(f *testing.F) {
	displayFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, line string) {
		out := displayText(line)
		if out == line {
			return
		}
		if strings.ContainsAny(out, "\x1b\r") {
			t.Fatalf("displayText(%q) = %q, an escape or carriage return survived", line, out)
		}
		if !utf8.ValidString(out) {
			t.Fatalf("displayText(%q) is not valid UTF-8", line)
		}
		if ur, uo := utf8.RuneCountInString(line), utf8.RuneCountInString(out); uo > ur {
			t.Fatalf("displayText(%q) grew from %d to %d runes", line, ur, uo)
		}
	})
}
