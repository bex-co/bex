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
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestQueryListAcceptsCommaAndRepeatedForms(t *testing.T) {
	q := url.Values{"name": {" alpha,bravo ", "bravo", "", "charlie"}}
	if got := QueryList(q, "name"); !slices.Equal(got, []string{"alpha", "bravo", "charlie"}) {
		t.Fatalf("QueryList = %v", got)
	}
}

func TestQueryTime(t *testing.T) {
	want := time.Date(2026, 7, 15, 10, 11, 12, 0, time.UTC)
	got, err := QueryTime(url.Values{"createdBefore": {want.Format(time.RFC3339)}}, "createdBefore")
	if err != nil || !got.Equal(want) {
		t.Fatalf("QueryTime = %v, %v", got, err)
	}
	_, err = QueryTime(url.Values{"createdBefore": {"yesterday"}}, "createdBefore")
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "createdBefore") {
		t.Fatalf("invalid QueryTime = %v", err)
	}
}

func TestQueryTimeWindow(t *testing.T) {
	before := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	after := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	got, err := QueryTimeWindow(url.Values{
		"createdBefore": {before.Format(time.RFC3339)},
		"createdAfter":  {after.Format(time.RFC3339)},
	}, "createdBefore", "createdAfter")
	if err != nil || !got.Before.Equal(before) || !got.After.Equal(after) {
		t.Fatalf("QueryTimeWindow = %+v, %v", got, err)
	}

	// An absent pair is the zero window, not an error.
	if got, err = QueryTimeWindow(url.Values{}, "createdBefore", "createdAfter"); err != nil || (TimeWindow{}) != got {
		t.Fatalf("absent window = %+v, %v; want zero value", got, err)
	}

	// Either malformed bound is a bad request naming the offending key, so a
	// caller never sees its filter silently ignored.
	for _, key := range []string{"createdBefore", "createdAfter"} {
		_, err := QueryTimeWindow(url.Values{key: {"yesterday"}}, "createdBefore", "createdAfter")
		if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), key) {
			t.Errorf("%s=yesterday = %v, want a bad request naming %s", key, err, key)
		}
	}
}

func TestTimeWindowContains(t *testing.T) {
	at := func(day int) time.Time { return time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC) }
	stamp := func(day int) string { return at(day).Format(time.RFC3339) }

	cases := []struct {
		name   string
		window TimeWindow
		raw    string
		want   bool
	}{
		{"zero window admits everything", TimeWindow{}, stamp(15), true},
		{"before excludes later", TimeWindow{Before: at(10)}, stamp(15), false},
		{"before admits earlier", TimeWindow{Before: at(10)}, stamp(5), true},
		{"after excludes earlier", TimeWindow{After: at(10)}, stamp(5), false},
		{"after admits later", TimeWindow{After: at(10)}, stamp(15), true},
		{"both bounds admit the middle", TimeWindow{Before: at(20), After: at(10)}, stamp(15), true},
		{"both bounds exclude outside", TimeWindow{Before: at(20), After: at(10)}, stamp(25), false},
		// Render's bounds are exclusive on both ends.
		{"equal to before is excluded", TimeWindow{Before: at(10)}, stamp(10), false},
		{"equal to after is excluded", TimeWindow{After: at(10)}, stamp(10), false},
		// A timestamp the window cannot place is admitted, never silently
		// dropped (w2/m51): records pre-dating timestamp stamping have an empty
		// value, and a malformed one is likewise not evidence of exclusion.
		{"empty timestamp is admitted", TimeWindow{Before: at(10)}, "", true},
		{"unparseable timestamp is admitted", TimeWindow{Before: at(10)}, "not-a-time", true},
		{"empty timestamp with zero window", TimeWindow{}, "", true},
	}
	for _, c := range cases {
		if got := c.window.Contains(c.raw); got != c.want {
			t.Errorf("%s: Contains(%q) = %v, want %v", c.name, c.raw, got, c.want)
		}
	}
}

func TestParseEnum(t *testing.T) {
	// Absent means unfiltered, and an allowed value passes through unchanged.
	for _, value := range []string{"", "pending", "verified"} {
		got, err := ParseEnum("verificationStatus", value, "pending", "verified")
		if err != nil || got != value {
			t.Errorf("ParseEnum(%q) = (%q, %v), want (%q, nil)", value, got, err, value)
		}
	}

	// An unrecognized value names both the parameter and the accepted
	// vocabulary, so every surface reports the same 400.
	_, err := ParseEnum("verificationStatus", "maybe", "pending", "verified")
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("unknown value = %v, want ErrBadRequest", err)
	}
	for _, want := range []string{"verificationStatus", `"maybe"`, "pending|verified"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestFilter(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	even := func(n int) bool { return n%2 == 0 }

	if got := Filter(items, even); !slices.Equal(got, []int{2, 4}) {
		t.Errorf("Filter(even) = %v, want [2 4]", got)
	}
	if !slices.Equal(items, []int{1, 2, 3, 4, 5}) {
		t.Errorf("Filter mutated its input: %v", items)
	}
	if got := Filter(items, func(int) bool { return true }); !slices.Equal(got, items) {
		t.Errorf("Filter(always) = %v, want the input unchanged", got)
	}
	if got := Filter(items, func(int) bool { return false }); len(got) != 0 {
		t.Errorf("Filter(never) = %v, want empty", got)
	}
	if got := Filter(nil, even); len(got) != 0 {
		t.Errorf("Filter(nil) = %v, want empty", got)
	}
}

func TestParseDirection(t *testing.T) {
	cases := []struct {
		direction     string
		wantOldest    bool
		wantBadReqest bool
	}{
		{"", false, false},                // absent ⇒ newest-first
		{DirectionBackward, false, false}, // explicit newest-first
		{DirectionForward, true, false},   // oldest-first
		{"sideways", false, true},
	}
	for _, c := range cases {
		oldestFirst, err := ParseDirection(c.direction)
		if c.wantBadReqest {
			if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), c.direction) {
				t.Errorf("ParseDirection(%q) = %v, want a bad request naming the value", c.direction, err)
			}
			continue
		}
		if err != nil || oldestFirst != c.wantOldest {
			t.Errorf("ParseDirection(%q) = (%v, %v), want (%v, nil)", c.direction, oldestFirst, err, c.wantOldest)
		}
	}
}

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

func TestStablePage(t *testing.T) {
	items := []string{"d", "b", "c", "a"}
	id := func(s string) string { return s }

	// No paging args means compatibility mode: the full slice and its original
	// ordering are preserved exactly.
	if got := StablePage(items, "", DefaultPageLimit, false, id); !slices.Equal(got, items) {
		t.Fatalf("omitted paging args = %v, want original %v", got, items)
	}

	// Once paging is requested, the cursor order is deterministic without
	// mutating the backing-store order supplied by the caller.
	if got := StablePage(items, "", 2, true, id); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("first stable page = %v, want [a b]", got)
	}
	if got := StablePage(items, "b", 2, true, id); !slices.Equal(got, []string{"c", "d"}) {
		t.Fatalf("second stable page = %v, want [c d]", got)
	}
	if !slices.Equal(items, []string{"d", "b", "c", "a"}) {
		t.Fatalf("StablePage mutated input: %v", items)
	}
}

// TestPageLimitOrDefaultTreatsZeroAsAbsent pins the distinction that makes this
// a separate function from PageLimit, which is the whole reason it exists: for
// a typed adapter argument 0 means OMITTED, and PageLimit would clamp it UP to
// 1 rather than defaulting it — serving one item per page. list_environments
// shipped that way (it called PageLimit directly), and it is invisible except
// in the page size, so the seam is pinned here for every consumer at once.
func TestPageLimitOrDefaultTreatsZeroAsAbsent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int
		want  int
	}{
		{"absent defaults", 0, DefaultPageLimit},
		{"in range passes through", 7, 7},
		{"oversized clamps", MaxPageLimit + 50, MaxPageLimit},
		{"exactly the max is kept", MaxPageLimit, MaxPageLimit},
		// A negative is a caller error, not an absence: it clamps to the floor
		// like any other out-of-range value rather than silently becoming a
		// default page. Callers that want negatives treated as absent keep
		// their own `< 1` branch (see this function's doc).
		{"negative clamps to the floor", -5, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := PageLimitOrDefault(tc.limit); got != tc.want {
				t.Errorf("PageLimitOrDefault(%d) = %d, want %d", tc.limit, got, tc.want)
			}
		})
	}
	// The contrast that motivates the function at all.
	if PageLimit(0) == PageLimitOrDefault(0) {
		t.Error("PageLimit and PageLimitOrDefault agree on 0; the distinction this function exists for is gone")
	}
}
