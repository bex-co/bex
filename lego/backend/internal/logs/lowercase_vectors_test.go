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
	"slices"
	"testing"
	"unicode"
	"unicode/utf8"
)

// lowercaseRunsPath is the one table of the simple lowercase mapping log text
// search folds with: Go's per-rune unicode.ToLower, which strings.ToLower
// applies to both the search term and the message. The dashboard's deploy log
// panel filters already-loaded lines in the browser, where JavaScript's
// toLowerCase folds a final Σ to ς and İ to two code points; it matches through
// the same table instead (dashboard/src/features/logs/lib/__tests__/
// go-lowercase.test.ts), so a deploy search finds what service history finds
// (w4/m216). Regenerate with BEX_UPDATE_LOWERCASE_RUNS=1 after a Go upgrade
// changes the Unicode tables; both suites then pin the new table.
const lowercaseRunsPath = "testdata/lowercase-runs.json"

// lowercaseRun maps every rune start, start+stride, …, end to rune+delta.
type lowercaseRun [4]int // start, end, stride, delta

// goLowercaseRuns compresses unicode.ToLower over every scalar value into runs:
// a stride of 1 for a block of capitals, 2 for the alternating upper/lower
// blocks of the Latin, Greek and Cyrillic extensions.
func goLowercaseRuns() []lowercaseRun {
	var runs []lowercaseRun
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		delta := int(unicode.ToLower(r) - r)
		if delta == 0 {
			continue
		}
		if n := len(runs); n > 0 && runs[n-1][3] == delta {
			last := &runs[n-1]
			gap := int(r) - last[1]
			if (last[0] == last[1] && (gap == 1 || gap == 2)) || (last[0] != last[1] && gap == last[2]) {
				last[2], last[1] = gap, int(r)
				continue
			}
		}
		runs = append(runs, lowercaseRun{int(r), int(r), 1, delta})
	}
	return runs
}

func TestLowercaseRunsMatchGo(t *testing.T) {
	runs := goLowercaseRuns()
	// The runs reproduce unicode.ToLower exactly, for every scalar value.
	mapped := map[rune]rune{}
	for _, run := range runs {
		for r := run[0]; r <= run[1]; r += run[2] {
			mapped[rune(r)] = rune(r + run[3])
		}
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		want := unicode.ToLower(r)
		got, ok := mapped[r]
		if !ok {
			got = r
		}
		if utf8.ValidRune(r) && got != want {
			t.Fatalf("runs map %U to %U, unicode.ToLower gives %U", r, got, want)
		}
	}
	if os.Getenv("BEX_UPDATE_LOWERCASE_RUNS") != "" {
		raw, err := json.Marshal(runs)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lowercaseRunsPath, append(raw, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(lowercaseRunsPath)
	if err != nil {
		t.Fatalf("read %s: %v", lowercaseRunsPath, err)
	}
	var committed []lowercaseRun
	if err := json.Unmarshal(raw, &committed); err != nil {
		t.Fatalf("parse %s: %v", lowercaseRunsPath, err)
	}
	if !slices.Equal(committed, runs) {
		t.Fatalf("%s is not this Go's unicode.ToLower (Unicode %s); regenerate with BEX_UPDATE_LOWERCASE_RUNS=1",
			lowercaseRunsPath, unicode.Version)
	}
}
