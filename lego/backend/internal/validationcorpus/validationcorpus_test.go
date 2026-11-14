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

package validationcorpus

import "testing"

// FormTrim is JavaScript's String.prototype.trim, which is not Go's
// strings.TrimSpace: it strips a byte order mark and leaves a NEL.
func TestFormTrimIsJavaScriptsTrim(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"  a b  ":       "a b",
		"\n\ta\r\n":     "a",
		"\u00a0a\u2028": "a",
		"\u3000a\u2029": "a",
		"\ufeffa\ufeff": "a",
		"\u0085a\u0085": "\u0085a\u0085",
		"a \u0000 b":    "a \u0000 b",
		"\u200ba":       "\u200ba",
		"":              "",
		"   ":           "",
	} {
		if got := FormTrim(in); got != want {
			t.Errorf("FormTrim(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCaseTextAppliesRepeatBetweenPrefixAndSuffix(t *testing.T) {
	t.Parallel()
	for c, want := range map[Case]string{
		{Input: "ab"}:            "ab",
		{Input: "ab", Repeat: 1}: "ab",
		{Input: "ab", Repeat: 3}: "ababab",
		{Prefix: "p/", Input: "a", Repeat: 2, Suffix: ".s"}: "p/aa.s",
		{Prefix: "p", Input: "x", Suffix: "s"}:              "pxs",
	} {
		if got := c.Text(); got != want {
			t.Errorf("%+v.Text() = %q, want %q", c, got, want)
		}
	}
}
