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

import "testing"

// appLevelFuzzSeeds are the shipper's severity shapes from appLevelCorpus —
// the JSON and logfmt lines the live tail classifies — plus malformed edges:
// unterminated quotes and escapes, bare `=`, multibyte keys, and the byte
// prefixes that steer the hand-rolled scanner into its slicing branches. Seeds
// live as source so a reviewer sees what the untrusted surface is probed with;
// the on-disk testdata/fuzz cache is never committed.
func appLevelFuzzSeeds(f *testing.F) {
	f.Helper()
	for _, tc := range appLevelCorpus {
		f.Add(tc.line)
	}
	for _, seed := range []string{
		`{"level":`,
		`{"level":"`,
		`{"severity":{"level":"error"}}`,
		`{"level":[{"level":"error"}]}`,
		`{"level":1e999}`,
		`{"level":"error"}\x00`,
		"level=\"",
		"level=\"a\\",
		"level=\"a\\q\"",
		"level=\"a\"\"b\"",
		"level=a=b",
		"level=a\"b",
		"level==x",
		"=level=error",
		"level=é",
		"é=error",
		"\x00level=error",
		"level=\x00",
		"level=error\x00severity=warn",
	} {
		f.Add(seed)
	}
}

// FuzzAppLineLevel drives the live tail's copy of the log shipper's app_logs
// level classifier with arbitrary container output. Every line a tenant
// container writes reaches this parser on the subscribe path, and the scanner
// is hand-rolled over raw bytes (JSON prefix detection, logfmt tokenizing,
// quoted-value unquoting), so it must never panic or loop. Its result must stay
// inside the shipper's fixed bucket vocabulary: the six levels plus the empty
// "no level label" answer. A value outside that set would be stored as a label
// no history query or dashboard filter can ever select.
func FuzzAppLineLevel(f *testing.F) {
	appLevelFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, line string) {
		switch level := appLineLevel(line); level {
		case "", unknownLevel, "error", "warning", "info", "debug":
		default:
			t.Fatalf("appLineLevel(%q) = %q, outside the shipper's bucket vocabulary", line, level)
		}
	})
}
