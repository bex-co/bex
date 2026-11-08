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

package agentsessions

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func schedLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	l, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("tzdata for %s unavailable: %v", name, err)
	}
	return l
}

func ip(i int) *int { return &i }

func tp(t time.Time) *time.Time { return &t }

func schedCode(t *testing.T, err error) string {
	t.Helper()
	var ce *core.CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("want a coded error, got %v", err)
	}
	return ce.Code
}

const (
	la = "America/Los_Angeles"
	be = "Europe/Berlin"
	ko = "Asia/Kolkata"
)

func TestAutomationScheduleNextTable(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		name  string
		s     AutomationSchedule
		after time.Time
		want  time.Time
		ok    bool
	}{
		{"hour", AutomationSchedule{Frequency: "hour", Minute: ip(15), Timezone: "UTC"},
			time.Date(2026, 3, 1, 10, 20, 0, 0, utc), time.Date(2026, 3, 1, 11, 15, 0, 0, utc), true},
		{"hour strictly after", AutomationSchedule{Frequency: "hour", Minute: ip(15), Timezone: "UTC"},
			time.Date(2026, 3, 1, 10, 15, 0, 0, utc), time.Date(2026, 3, 1, 11, 15, 0, 0, utc), true},
		{"hour kolkata half-hour offset", AutomationSchedule{Frequency: "hour", Minute: ip(0), Timezone: ko},
			time.Date(2026, 3, 1, 10, 0, 0, 0, utc), time.Date(2026, 3, 1, 10, 30, 0, 0, utc), true},
		{"day berlin", AutomationSchedule{Frequency: "day", At: "09:00", Timezone: be},
			time.Date(2026, 1, 10, 9, 0, 0, 0, utc), time.Date(2026, 1, 11, 8, 0, 0, 0, utc), true},
		{"day kolkata", AutomationSchedule{Frequency: "day", At: "09:00", Timezone: ko},
			time.Date(2026, 1, 10, 0, 0, 0, 0, utc), time.Date(2026, 1, 10, 3, 30, 0, 0, utc), true},
		{"week several weekdays", AutomationSchedule{Frequency: "week", At: "08:30", Weekdays: []string{"Fri", "mon", "WED"}, Timezone: "UTC"},
			time.Date(2026, 11, 4, 9, 0, 0, 0, utc), // Wednesday
			time.Date(2026, 11, 6, 8, 30, 0, 0, utc), true},
		{"week wraps to next week", AutomationSchedule{Frequency: "week", At: "08:30", Weekdays: []string{"mon", "wed"}, Timezone: "UTC"},
			time.Date(2026, 11, 4, 9, 0, 0, 0, utc), time.Date(2026, 11, 9, 8, 30, 0, 0, utc), true},
		{"week sunday", AutomationSchedule{Frequency: "week", At: "00:00", Weekdays: []string{"sun"}, Timezone: "UTC"},
			time.Date(2026, 11, 4, 9, 0, 0, 0, utc), time.Date(2026, 11, 8, 0, 0, 0, 0, utc), true},
		{"once future", AutomationSchedule{Frequency: "once", RunAt: tp(time.Date(2026, 5, 1, 12, 0, 0, 0, utc)), Timezone: "UTC"},
			time.Date(2026, 4, 1, 0, 0, 0, 0, utc), time.Date(2026, 5, 1, 12, 0, 0, 0, utc), true},
		{"once exactly at", AutomationSchedule{Frequency: "once", RunAt: tp(time.Date(2026, 5, 1, 12, 0, 0, 0, utc)), Timezone: "UTC"},
			time.Date(2026, 5, 1, 12, 0, 0, 0, utc), time.Time{}, false},
		{"once other offset normalized to UTC", AutomationSchedule{Frequency: "once", RunAt: tp(time.Date(2026, 5, 1, 12, 0, 0, 0, schedLoc(t, be))), Timezone: be},
			time.Date(2026, 4, 1, 0, 0, 0, 0, utc), time.Date(2026, 5, 1, 10, 0, 0, 0, utc), true},
		{"custom 31st skips short months", AutomationSchedule{Frequency: "custom", Cron: "0 12 31 * *", Timezone: "UTC"},
			time.Date(2026, 1, 31, 12, 0, 0, 0, utc), time.Date(2026, 3, 31, 12, 0, 0, 0, utc), true},
		{"custom 31st month end april", AutomationSchedule{Frequency: "custom", Cron: "0 12 31 * *", Timezone: "UTC"},
			time.Date(2026, 3, 31, 12, 0, 0, 0, utc), time.Date(2026, 5, 31, 12, 0, 0, 0, utc), true},
		{"custom feb 29 leap day", AutomationSchedule{Frequency: "custom", Cron: "0 0 29 2 *", Timezone: "UTC"},
			time.Date(2026, 3, 1, 0, 0, 0, 0, utc), time.Date(2028, 2, 29, 0, 0, 0, 0, utc), true},
		{"day 02:30 in a leap year feb", AutomationSchedule{Frequency: "day", At: "23:59", Timezone: "UTC"},
			time.Date(2028, 2, 28, 23, 59, 0, 0, utc), time.Date(2028, 2, 29, 23, 59, 0, 0, utc), true},
		{"custom step", AutomationSchedule{Frequency: "custom", Cron: "*/10 * * * *", Timezone: "UTC"},
			time.Date(2026, 3, 1, 10, 3, 30, 0, utc), time.Date(2026, 3, 1, 10, 10, 0, 0, utc), true},
		{"endsAt exactly at the tick does not fire", AutomationSchedule{Frequency: "day", At: "09:00", Timezone: "UTC", EndsAt: tp(time.Date(2026, 3, 2, 9, 0, 0, 0, utc))},
			time.Date(2026, 3, 1, 9, 0, 0, 0, utc), time.Time{}, false},
		{"endsAt one second later still fires", AutomationSchedule{Frequency: "day", At: "09:00", Timezone: "UTC", EndsAt: tp(time.Date(2026, 3, 2, 9, 0, 1, 0, utc))},
			time.Date(2026, 3, 1, 9, 0, 0, 0, utc), time.Date(2026, 3, 2, 9, 0, 0, 0, utc), true},
		{"endsAt cancels once", AutomationSchedule{Frequency: "once", RunAt: tp(time.Date(2026, 5, 1, 12, 0, 0, 0, utc)), Timezone: "UTC", EndsAt: tp(time.Date(2026, 5, 1, 12, 0, 0, 0, utc))},
			time.Date(2026, 4, 1, 0, 0, 0, 0, utc), time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.s.Next(tc.after)
			if ok != tc.ok || !got.Equal(tc.want) {
				t.Fatalf("Next = %v, %v; want %v, %v", got, ok, tc.want, tc.ok)
			}
			if ok && got.Location() != time.UTC {
				t.Fatalf("Next must return UTC, got %v", got.Location())
			}
		})
	}
}

func TestAutomationScheduleDST(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		name  string
		s     AutomationSchedule
		after time.Time
		want  []time.Time // consecutive fires
	}{
		// Los Angeles: spring forward 2026-03-08 02:00 -> 03:00; fall back 2026-11-01 02:00 -> 01:00.
		{"LA day 02:30 spring forward fires once, shifted", AutomationSchedule{Frequency: "day", At: "02:30", Timezone: la},
			time.Date(2026, 3, 7, 12, 0, 0, 0, utc),
			[]time.Time{
				time.Date(2026, 3, 7, 10, 30, 0, 0, utc), // not before `after`? 02:30 PST = 10:30Z (< after) -> next day below
			}},
		{"LA day 01:30 fall back fires once at the first occurrence", AutomationSchedule{Frequency: "day", At: "01:30", Timezone: la},
			time.Date(2026, 10, 31, 12, 0, 0, 0, utc),
			[]time.Time{
				time.Date(2026, 11, 1, 8, 30, 0, 0, utc), // 01:30 PDT
				time.Date(2026, 11, 2, 9, 30, 0, 0, utc), // 01:30 PST
			}},
		{"LA hour :30 across spring forward", AutomationSchedule{Frequency: "hour", Minute: ip(30), Timezone: la},
			time.Date(2026, 3, 8, 9, 0, 0, 0, utc), // 01:00 PST
			[]time.Time{
				time.Date(2026, 3, 8, 9, 30, 0, 0, utc),  // 01:30 PST
				time.Date(2026, 3, 8, 10, 30, 0, 0, utc), // 03:30 PDT (02:30 never exists; collapses)
				time.Date(2026, 3, 8, 11, 30, 0, 0, utc), // 04:30 PDT
			}},
		{"LA hour :30 across fall back", AutomationSchedule{Frequency: "hour", Minute: ip(30), Timezone: la},
			time.Date(2026, 11, 1, 7, 0, 0, 0, utc), // 00:00 PDT
			[]time.Time{
				time.Date(2026, 11, 1, 7, 30, 0, 0, utc),  // 00:30 PDT
				time.Date(2026, 11, 1, 8, 30, 0, 0, utc),  // 01:30 PDT (first occurrence)
				time.Date(2026, 11, 1, 10, 30, 0, 0, utc), // 02:30 PST (01:30 PST is not repeated)
			}},
		{"LA week sunday 02:30 spring forward", AutomationSchedule{Frequency: "week", At: "02:30", Weekdays: []string{"sun"}, Timezone: la},
			time.Date(2026, 3, 1, 12, 0, 0, 0, utc), // Sunday 2026-03-01 04:00 PST
			[]time.Time{
				time.Date(2026, 3, 8, 10, 30, 0, 0, utc), // 02:30 PST nonexistent -> 03:30 PDT = 10:30Z
				time.Date(2026, 3, 15, 9, 30, 0, 0, utc), // 02:30 PDT
			}},
		{"LA week sunday 01:30 fall back", AutomationSchedule{Frequency: "week", At: "01:30", Weekdays: []string{"sun"}, Timezone: la},
			time.Date(2026, 10, 25, 12, 0, 0, 0, utc),
			[]time.Time{
				time.Date(2026, 11, 1, 8, 30, 0, 0, utc), // first 01:30 (PDT)
				time.Date(2026, 11, 8, 9, 30, 0, 0, utc), // 01:30 PST
			}},
		// Berlin: spring forward 2026-03-29 02:00 -> 03:00; fall back 2026-10-25 03:00 -> 02:00.
		{"Berlin day 02:30 spring forward fires once", AutomationSchedule{Frequency: "day", At: "02:30", Timezone: be},
			time.Date(2026, 3, 28, 12, 0, 0, 0, utc),
			[]time.Time{
				time.Date(2026, 3, 29, 1, 30, 0, 0, utc), // 03:30 CEST (02:30 CET does not exist)
				time.Date(2026, 3, 30, 0, 30, 0, 0, utc), // 02:30 CEST
			}},
		{"Berlin day 02:30 fall back fires once, first occurrence", AutomationSchedule{Frequency: "day", At: "02:30", Timezone: be},
			time.Date(2026, 10, 24, 12, 0, 0, 0, utc),
			[]time.Time{
				time.Date(2026, 10, 25, 0, 30, 0, 0, utc), // 02:30 CEST
				time.Date(2026, 10, 26, 1, 30, 0, 0, utc), // 02:30 CET
			}},
		{"Berlin hour :15 across fall back", AutomationSchedule{Frequency: "hour", Minute: ip(15), Timezone: be},
			time.Date(2026, 10, 25, 0, 0, 0, 0, utc), // 02:00 CEST
			[]time.Time{
				time.Date(2026, 10, 25, 0, 15, 0, 0, utc), // 02:15 CEST
				time.Date(2026, 10, 25, 2, 15, 0, 0, utc), // 03:15 CET (02:15 CET not repeated)
			}},
		{"Berlin hour :15 across spring forward", AutomationSchedule{Frequency: "hour", Minute: ip(15), Timezone: be},
			time.Date(2026, 3, 29, 0, 0, 0, 0, utc), // 01:00 CET
			[]time.Time{
				time.Date(2026, 3, 29, 0, 15, 0, 0, utc), // 01:15 CET
				time.Date(2026, 3, 29, 1, 15, 0, 0, utc), // 03:15 CEST
				time.Date(2026, 3, 29, 2, 15, 0, 0, utc), // 04:15 CEST
			}},
		{"Berlin week monday 02:30 across fall back sunday", AutomationSchedule{Frequency: "week", At: "02:30", Weekdays: []string{"sun"}, Timezone: be},
			time.Date(2026, 10, 18, 12, 0, 0, 0, utc),
			[]time.Time{
				time.Date(2026, 10, 25, 0, 30, 0, 0, utc),
				time.Date(2026, 11, 1, 1, 30, 0, 0, utc),
			}},
		{"Berlin week sunday 02:30 spring forward", AutomationSchedule{Frequency: "week", At: "02:30", Weekdays: []string{"sun"}, Timezone: be},
			time.Date(2026, 3, 22, 12, 0, 0, 0, utc),
			[]time.Time{
				time.Date(2026, 3, 29, 1, 30, 0, 0, utc),
				time.Date(2026, 4, 5, 0, 30, 0, 0, utc),
			}},
		// Kolkata has no DST: wall clock is constant all year.
		{"Kolkata day 02:30 never shifts", AutomationSchedule{Frequency: "day", At: "02:30", Timezone: ko},
			time.Date(2026, 3, 7, 22, 0, 0, 0, utc),
			[]time.Time{
				time.Date(2026, 3, 7, 21, 0, 0, 0, utc), // placeholder replaced below
			}},
	}
	// Row 0 starts after that day's 02:30 so the transition day is next.
	cases[0].after = time.Date(2026, 3, 7, 11, 0, 0, 0, utc) // 03:00 PST on 03-07, after that day's 02:30
	cases[0].want = []time.Time{
		time.Date(2026, 3, 8, 10, 30, 0, 0, utc), // 03:30 PDT
		time.Date(2026, 3, 9, 9, 30, 0, 0, utc),  // 02:30 PDT
	}
	last := len(cases) - 1
	cases[last].want = []time.Time{
		time.Date(2026, 3, 8, 21, 0, 0, 0, utc),
		time.Date(2026, 3, 9, 21, 0, 0, 0, utc),
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.s.Validate(tc.after.Add(-time.Hour)); err != nil {
				t.Fatalf("schedule should validate: %v", err)
			}
			cur := tc.after
			for i, want := range tc.want {
				got, ok := tc.s.Next(cur)
				if !ok || !got.Equal(want) {
					t.Fatalf("fire %d after %v = %v, %v; want %v", i, cur, got, ok, want)
				}
				cur = got
			}
		})
	}
}

// Whichever instant the walk starts from, every fire is seen exactly once and
// in order across a full year in every zone (no skips, no repeats).
func TestAutomationScheduleDSTWalkIsMonotonic(t *testing.T) {
	for _, zone := range []string{la, be, ko} {
		for _, s := range []AutomationSchedule{
			{Frequency: "day", At: "02:30", Timezone: zone},
			{Frequency: "day", At: "01:30", Timezone: zone},
			{Frequency: "hour", Minute: ip(30), Timezone: zone},
			{Frequency: "custom", Cron: "*/15 * * * *", Timezone: zone},
		} {
			cur := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
			days := map[string]int{}
			n := 0
			for {
				nx, ok := s.Next(cur)
				if !ok || !nx.Before(end) {
					break
				}
				if !nx.After(cur) {
					t.Fatalf("%s %s: not strictly increasing: %v then %v", zone, s.CronSpec(), cur, nx)
				}
				l := nx.In(schedLoc(t, zone))
				if s.Frequency == "day" {
					days[l.Format("2006-01-02")]++
				}
				cur = nx
				n++
			}
			if s.Frequency == "day" {
				if len(days) != 365 {
					t.Fatalf("%s day %s: fired on %d distinct days, want 365", zone, s.At, len(days))
				}
				for d, c := range days {
					if c != 1 {
						t.Fatalf("%s day %s: %d fires on %s", zone, s.At, c, d)
					}
				}
			}
			if n == 0 {
				t.Fatalf("%s %s never fired", zone, s.CronSpec())
			}
		}
	}
}

func TestAutomationScheduleCronSpec(t *testing.T) {
	cases := []struct {
		s    AutomationSchedule
		want string
	}{
		{AutomationSchedule{Frequency: "hour", Minute: ip(7)}, "7 * * * *"},
		{AutomationSchedule{Frequency: "day", At: "09:05"}, "5 9 * * *"},
		{AutomationSchedule{Frequency: "week", At: "18:30", Weekdays: []string{"Fri", "mon", "mon", "sun"}}, "30 18 * * 0,1,5"},
		{AutomationSchedule{Frequency: "custom", Cron: "*/10 8-18 * * 1-5"}, "*/10 8-18 * * 1-5"},
		{AutomationSchedule{Frequency: "once", RunAt: tp(time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC))}, ""},
	}
	for _, tc := range cases {
		if got := tc.s.CronSpec(); got != tc.want {
			t.Errorf("CronSpec(%+v) = %q, want %q", tc.s, got, tc.want)
		}
	}
}

func TestAutomationScheduleNormalize(t *testing.T) {
	n, err := AutomationSchedule{Frequency: " Week ", At: "08:05", Weekdays: []string{"WED", "mon", "Mon"}, Timezone: be}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if n.Frequency != "week" || len(n.Weekdays) != 2 || n.Weekdays[0] != "mon" || n.Weekdays[1] != "wed" {
		t.Fatalf("normalized = %+v", n)
	}
}

func TestAutomationScheduleValidate(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(48 * time.Hour)
	cases := []struct {
		name string
		s    AutomationSchedule
		code string // "" means valid
	}{
		{"hour ok", AutomationSchedule{Frequency: "hour", Minute: ip(0), Timezone: be}, ""},
		{"day ok", AutomationSchedule{Frequency: "day", At: "09:00", Timezone: be}, ""},
		{"week ok", AutomationSchedule{Frequency: "week", At: "09:00", Weekdays: []string{"mon"}, Timezone: be}, ""},
		{"once ok", AutomationSchedule{Frequency: "once", RunAt: tp(future), Timezone: be}, ""},
		{"custom ok", AutomationSchedule{Frequency: "custom", Cron: "0 9 * * 1-5", Timezone: be, EndsAt: tp(future)}, ""},
		{"exactly 5 minutes allowed", AutomationSchedule{Frequency: "custom", Cron: "*/5 * * * *", Timezone: "UTC"}, ""},
		{"4 minutes refused", AutomationSchedule{Frequency: "custom", Cron: "0,4 * * * *", Timezone: "UTC"}, CodeAutomationScheduleTooFrequent},
		{"every minute refused", AutomationSchedule{Frequency: "custom", Cron: "* * * * *", Timezone: "UTC"}, CodeAutomationScheduleTooFrequent},
		{"uneven pair refused", AutomationSchedule{Frequency: "custom", Cron: "0,2 * * * *", Timezone: "UTC"}, CodeAutomationScheduleTooFrequent},
		{"pair wrapping the hour refused", AutomationSchedule{Frequency: "custom", Cron: "0,58 * * * *", Timezone: "UTC"}, CodeAutomationScheduleTooFrequent},
		{"close pair at 3am refused", AutomationSchedule{Frequency: "custom", Cron: "0,2 3 * * *", Timezone: "UTC"}, CodeAutomationScheduleTooFrequent},
		{"missing zone", AutomationSchedule{Frequency: "day", At: "09:00"}, CodeAutomationScheduleInvalid},
		{"Local zone", AutomationSchedule{Frequency: "day", At: "09:00", Timezone: "Local"}, CodeAutomationScheduleInvalid},
		{"unknown zone", AutomationSchedule{Frequency: "day", At: "09:00", Timezone: "Mars/Olympus"}, CodeAutomationScheduleInvalid},
		{"bad frequency", AutomationSchedule{Frequency: "month", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"empty frequency", AutomationSchedule{Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"hour minute missing", AutomationSchedule{Frequency: "hour", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"hour minute 60", AutomationSchedule{Frequency: "hour", Minute: ip(60), Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"hour minute -1", AutomationSchedule{Frequency: "hour", Minute: ip(-1), Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"day at 24:00", AutomationSchedule{Frequency: "day", At: "24:00", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"day at 9:00", AutomationSchedule{Frequency: "day", At: "9:00", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"day at missing", AutomationSchedule{Frequency: "day", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"day with runAt", AutomationSchedule{Frequency: "day", At: "09:00", RunAt: tp(future), Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"day with minute", AutomationSchedule{Frequency: "day", At: "09:00", Minute: ip(0), Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"hour with at", AutomationSchedule{Frequency: "hour", Minute: ip(0), At: "09:00", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"day with weekdays", AutomationSchedule{Frequency: "day", At: "09:00", Weekdays: []string{"mon"}, Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"custom with at", AutomationSchedule{Frequency: "custom", Cron: "0 9 * * *", At: "09:00", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"once with cron", AutomationSchedule{Frequency: "once", RunAt: tp(future), Cron: "0 9 * * *", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"week no weekdays", AutomationSchedule{Frequency: "week", At: "09:00", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"week bad weekday", AutomationSchedule{Frequency: "week", At: "09:00", Weekdays: []string{"someday"}, Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"once without runAt", AutomationSchedule{Frequency: "once", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"custom without cron", AutomationSchedule{Frequency: "custom", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"cron six fields", AutomationSchedule{Frequency: "custom", Cron: "0 0 9 * * *", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"cron four fields", AutomationSchedule{Frequency: "custom", Cron: "0 9 * *", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"cron descriptor", AutomationSchedule{Frequency: "custom", Cron: "@daily", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"cron every", AutomationSchedule{Frequency: "custom", Cron: "@every 1m", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"cron TZ prefix", AutomationSchedule{Frequency: "custom", Cron: "TZ=Asia/Tokyo 0 9 * * *", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"cron out of range", AutomationSchedule{Frequency: "custom", Cron: "61 * * * *", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"cron never fires", AutomationSchedule{Frequency: "custom", Cron: "0 0 31 2 *", Timezone: "UTC"}, CodeAutomationScheduleInvalid},
		{"once runAt now", AutomationSchedule{Frequency: "once", RunAt: tp(now), Timezone: "UTC"}, CodeAutomationSchedulePast},
		{"once runAt past", AutomationSchedule{Frequency: "once", RunAt: tp(now.Add(-time.Minute)), Timezone: "UTC"}, CodeAutomationSchedulePast},
		{"endsAt now", AutomationSchedule{Frequency: "day", At: "09:00", Timezone: "UTC", EndsAt: tp(now)}, CodeAutomationSchedulePast},
		{"endsAt past", AutomationSchedule{Frequency: "day", At: "09:00", Timezone: "UTC", EndsAt: tp(now.Add(-time.Hour))}, CodeAutomationSchedulePast},
		{"once endsAt before runAt", AutomationSchedule{Frequency: "once", RunAt: tp(future), Timezone: "UTC", EndsAt: tp(now.Add(time.Hour))}, CodeAutomationScheduleInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.s.Validate(now)
			if tc.code == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want %s, got nil", tc.code)
			}
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("want a 400, got %v", err)
			}
			if got := schedCode(t, err); got != tc.code {
				t.Fatalf("code = %s, want %s (%v)", got, tc.code, err)
			}
		})
	}
}

func TestAutomationScheduleAdvance(t *testing.T) {
	utc := time.UTC
	every5 := AutomationSchedule{Frequency: "custom", Cron: "*/5 * * * *", Timezone: "UTC"}
	day := AutomationSchedule{Frequency: "day", At: "09:00", Timezone: "UTC"}
	d := func(day, h, m int) time.Time { return time.Date(2026, 3, day, h, m, 0, 0, utc) }

	t.Run("only due is overdue", func(t *testing.T) {
		fire, next, missed := every5.Advance(d(1, 10, 0), d(1, 10, 2))
		if !fire.Equal(d(1, 10, 0)) || missed != 0 || next == nil || !next.Equal(d(1, 10, 5)) {
			t.Fatalf("got %v %v %d", fire, next, missed)
		}
	})
	t.Run("now exactly on a later tick", func(t *testing.T) {
		fire, next, missed := every5.Advance(d(1, 10, 0), d(1, 10, 5))
		if !fire.Equal(d(1, 10, 5)) || missed != 1 || next == nil || !next.Equal(d(1, 10, 10)) {
			t.Fatalf("got %v %v %d", fire, next, missed)
		}
	})
	t.Run("one missed", func(t *testing.T) {
		// 10:05 has also passed, so it is the tick to run and 10:00 is skipped.
		fire, next, missed := every5.Advance(d(1, 10, 0), d(1, 10, 7))
		if !fire.Equal(d(1, 10, 5)) || missed != 1 || next == nil || !next.Equal(d(1, 10, 10)) {
			t.Fatalf("got %v %v %d", fire, next, missed)
		}
	})
	t.Run("many missed", func(t *testing.T) {
		fire, next, missed := day.Advance(d(1, 9, 0), d(11, 9, 30))
		if !fire.Equal(d(11, 9, 0)) || missed != 10 || next == nil || !next.Equal(d(12, 9, 0)) {
			t.Fatalf("got %v %v %d", fire, next, missed)
		}
	})
	t.Run("cap", func(t *testing.T) {
		due := time.Date(2025, 1, 1, 0, 0, 0, 0, utc)
		now := time.Date(2026, 3, 1, 10, 3, 0, 0, utc) // > 10000 five-minute ticks later
		fire, next, missed := every5.Advance(due, now)
		if missed != maxMissedTicks {
			t.Fatalf("missed = %d, want cap %d", missed, maxMissedTicks)
		}
		if !fire.Equal(time.Date(2026, 3, 1, 10, 0, 0, 0, utc)) {
			t.Fatalf("fire = %v, want the latest tick 10:00", fire)
		}
		if next == nil || !next.Equal(time.Date(2026, 3, 1, 10, 5, 0, 0, utc)) {
			t.Fatalf("next = %v", next)
		}
	})
	t.Run("once", func(t *testing.T) {
		run := d(1, 10, 0)
		s := AutomationSchedule{Frequency: "once", RunAt: &run, Timezone: "UTC"}
		fire, next, missed := s.Advance(run, d(1, 10, 1))
		if !fire.Equal(run) || next != nil || missed != 0 {
			t.Fatalf("got %v %v %d", fire, next, missed)
		}
	})
	t.Run("endsAt is exclusive and stops next", func(t *testing.T) {
		s := day
		s.EndsAt = tp(d(3, 9, 0))
		fire, next, missed := s.Advance(d(1, 9, 0), d(3, 9, 0))
		if !fire.Equal(d(2, 9, 0)) || missed != 1 || next != nil {
			t.Fatalf("got %v %v %d", fire, next, missed)
		}
		s.EndsAt = tp(d(3, 9, 0).Add(time.Second))
		fire, next, missed = s.Advance(d(1, 9, 0), d(3, 9, 0))
		if !fire.Equal(d(3, 9, 0)) || missed != 2 || next != nil {
			t.Fatalf("got %v %v %d", fire, next, missed)
		}
	})
	t.Run("not yet due", func(t *testing.T) {
		fire, next, missed := day.Advance(d(2, 9, 0), d(1, 9, 0))
		if !fire.IsZero() || next != nil || missed != 0 {
			t.Fatalf("got %v %v %d", fire, next, missed)
		}
	})
	t.Run("across DST the ticks stay one per local day", func(t *testing.T) {
		s := AutomationSchedule{Frequency: "day", At: "02:30", Timezone: be}
		due := time.Date(2026, 3, 28, 1, 30, 0, 0, utc)
		fire, next, missed := s.Advance(due, time.Date(2026, 3, 29, 12, 0, 0, 0, utc))
		if !fire.Equal(time.Date(2026, 3, 29, 1, 30, 0, 0, utc)) || missed != 1 ||
			next == nil || !next.Equal(time.Date(2026, 3, 30, 0, 30, 0, 0, utc)) {
			t.Fatalf("got %v %v %d", fire, next, missed)
		}
	})
}

func TestAutomationScheduleJSONRoundTrip(t *testing.T) {
	in := AutomationSchedule{
		Frequency: "week", At: "09:00", Weekdays: []string{"mon", "fri"}, Timezone: be,
		EndsAt: tp(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"frequency":"week","at":"09:00","weekdays":["mon","fri"],"timezone":"Europe/Berlin","endsAt":"2027-01-01T00:00:00Z"}`
	if string(b) != want {
		t.Fatalf("json = %s\nwant   %s", b, want)
	}
	var out AutomationSchedule
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.CronSpec() != in.CronSpec() || !out.EndsAt.Equal(*in.EndsAt) || out.Timezone != in.Timezone {
		t.Fatalf("round trip = %+v", out)
	}

	var once AutomationSchedule
	if err := json.Unmarshal([]byte(`{"frequency":"once","runAt":"2026-12-24T10:00:00+01:00","timezone":"Europe/Berlin"}`), &once); err != nil {
		t.Fatal(err)
	}
	if got, ok := once.Next(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); !ok || !got.Equal(time.Date(2026, 12, 24, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("once.Next = %v %v", got, ok)
	}
	var hour AutomationSchedule
	if err := json.Unmarshal([]byte(`{"frequency":"hour","minute":0,"timezone":"UTC"}`), &hour); err != nil || hour.Minute == nil || *hour.Minute != 0 {
		t.Fatalf("minute 0 must survive decoding: %+v %v", hour, err)
	}
}
