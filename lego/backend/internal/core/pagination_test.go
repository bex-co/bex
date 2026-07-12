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

package core

import (
	"net/url"
	"strings"
	"testing"
)

func TestPageParams(t *testing.T) {
	q := func(s string) url.Values { v, _ := url.ParseQuery(s); return v }
	cases := []struct {
		raw       string
		wantAfter string
		wantLimit int
	}{
		{"", "", DefaultPageLimit},                  // absent ⇒ default 20
		{"limit=5", "", 5},                          // honored
		{"limit=0", "", 1},                          // clamp min
		{"limit=-3", "", 1},                         // clamp min
		{"limit=1000", "", MaxPageLimit},            // clamp max
		{"limit=notanint", "", DefaultPageLimit},    // unparseable ⇒ default
		{"cursor=tea-9", "tea-9", DefaultPageLimit}, // cursor passthrough
		{"cursor=tea-9&limit=2", "tea-9", 2},        // both
	}
	for _, c := range cases {
		after, limit := PageParams(q(c.raw))
		if after != c.wantAfter || limit != c.wantLimit {
			t.Errorf("PageParams(%q) = (%q, %d), want (%q, %d)", c.raw, after, limit, c.wantAfter, c.wantLimit)
		}
	}
}

func TestPage(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	id := func(s string) string { return s }
	join := func(ss []string) string { return strings.Join(ss, "") }

	cases := []struct {
		name  string
		after string
		limit int
		want  string
	}{
		{"head within limit", "", 3, "abc"},
		{"head over limit", "", 10, "abcde"},
		{"after cursor", "b", 2, "cd"},
		{"after cursor to end", "c", 10, "de"},
		{"after last ⇒ empty", "e", 5, ""},
		{"unknown cursor ⇒ empty", "zzz", 5, ""},
		{"limit 1", "", 1, "a"},
	}
	for _, c := range cases {
		got := join(Page(items, c.after, c.limit, id))
		if got != c.want {
			t.Errorf("%s: Page(after=%q, limit=%d) = %q, want %q", c.name, c.after, c.limit, got, c.want)
		}
	}

	// nil input is safe and yields an empty page, never a panic.
	if got := Page(nil, "", 10, id); len(got) != 0 {
		t.Errorf("Page(nil) = %v, want empty", got)
	}
}
