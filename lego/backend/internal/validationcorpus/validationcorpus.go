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

// Package validationcorpus loads the shared validation corpus: one JSON file
// per validator family under lego/backend/testdata/validation, asserted by a Go
// test against the real server validator and by a dashboard test against the
// client validator that mirrors it (dashboard/src/test/validation-corpus.ts),
// so a form and bex-api cannot disagree on an input without a test failing.
//
// A form that trims what it sends is checked on the trimmed text (FormTrim).
//
// Valid is the server's verdict: the corpus pins what the server does today
// and never tightens it. ServerOnly marks a refused input the client may
// still accept, because only the server can judge it (a public suffix, a
// registry policy); the client may never refuse an input the server accepts.
package validationcorpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

// Case is one corpus entry.
type Case struct {
	// Input is the value. Repeat, when above 1, repeats it, and Prefix and
	// Suffix go around it, for lengths no one should type into JSON.
	Prefix string `json:"prefix,omitempty"`
	Input  string `json:"input"`
	Repeat int    `json:"repeat,omitempty"`
	Suffix string `json:"suffix,omitempty"`
	// Key is the variable name of the env-value family.
	Key        string `json:"key,omitempty"`
	Valid      bool   `json:"valid"`
	ServerOnly bool   `json:"serverOnly,omitempty"`
	Why        string `json:"why,omitempty"`
}

// Text is the input with its repeat, prefix and suffix applied.
func (c Case) Text() string {
	input := c.Input
	if c.Repeat > 1 {
		input = strings.Repeat(input, c.Repeat)
	}
	return c.Prefix + input + c.Suffix
}

// FormTrim is what a dashboard form sends for what was typed: JavaScript's
// String.prototype.trim. It differs from Go's strings.TrimSpace at two code
// points, a byte order mark (U+FEFF), which JavaScript strips, and a NEL
// (U+0085), which JavaScript leaves and Go strips.
func FormTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return r == '\uFEFF' || (r != '\u0085' && unicode.IsSpace(r))
	})
}

// Name is the text for a failure message: the input, cut to a readable
// length with its byte count when it is long.
func (c Case) Name() string {
	text := c.Text()
	if len(text) <= 80 {
		return fmt.Sprintf("%q", text)
	}
	return fmt.Sprintf("%.40q… (%d bytes)", text, len(text))
}

// minCases is the fewest entries a family's file may hold: a file that shrinks
// below it has lost the boundary inputs its tests exist for.
const minCases = 20

// Check asserts, for every entry of a family, that accepts, the real server
// validator applied to the entry's text, agrees with the entry's verdict. It
// returns nil for an input the validator accepts. The test fails when the
// file is missing, malformed, short, or marks a valid entry serverOnly.
func Check(t *testing.T, family string, accepts func(c Case) error) {
	t.Helper()
	for _, c := range load(t, family) {
		err := accepts(c)
		if got := err == nil; got != c.Valid {
			t.Errorf("%s: %s accepted = %v (%v), the corpus says %v: %s", family, c.Name(), got, err, c.Valid, c.Why)
		}
	}
}

func load(t testing.TB, family string) []Case {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(here), "..", "..", "testdata", "validation", family+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []Case
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if len(cases) < minCases {
		t.Fatalf("%s holds %d entries, want at least %d", path, len(cases), minCases)
	}
	for _, c := range cases {
		if c.Valid && c.ServerOnly {
			t.Fatalf("%s: %q is valid and serverOnly; serverOnly marks a refused input", path, c.Text())
		}
	}
	return cases
}
