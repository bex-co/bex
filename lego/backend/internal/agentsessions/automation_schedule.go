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

// The schedule model of an agent automation (ADR101 D2, D3): pure functions,
// no database, no clock. Callers pass every instant explicitly.
//
// Time zones. robfig/cron's own Next mishandles both daylight-saving
// transitions (a skipped local time can be dropped for the day and a repeated
// one can fire twice), so this file uses robfig only to PARSE a five-field
// expression into its field bit sets and to search wall-clock matches in UTC
// (which has no transitions). Each wall-clock match is then mapped to a real
// instant by wallToInstant, which pins the two rules:
//
//   - a local time that does not exist (spring forward) fires once, shifted
//     forward by the length of the gap (02:30 becomes 03:30 when the clocks
//     jump 02:00 to 03:00); when that coincides with a real tick of the same
//     schedule, the two collapse into one fire;
//   - a local time that occurs twice (fall back) fires once, at the first
//     occurrence.

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const (
	// minScheduleInterval is the shortest gap allowed between two fires
	// (ADR101 D2): a cost-control limit of bex, not a Devin one.
	minScheduleInterval = 5 * time.Minute

	// maxMissedTicks caps the loop that counts ticks skipped during an
	// outage, so a year of downtime on a per-5-minute schedule cannot spin.
	maxMissedTicks = 10_000

	// frequencyScanFires is how many consecutive fires from "now" the
	// too-frequent check inspects.
	frequencyScanFires = 64

	// Named 400 codes.
	CodeAutomationScheduleInvalid     = "AGENT_AUTOMATION_SCHEDULE_INVALID"
	CodeAutomationSchedulePast        = "AGENT_AUTOMATION_SCHEDULE_PAST"
	CodeAutomationScheduleTooFrequent = "AGENT_AUTOMATION_SCHEDULE_TOO_FREQUENT"
	automationFrequencyHour           = "hour"
	automationFrequencyDay            = "day"
	automationFrequencyWeek           = "week"
	automationFrequencyOnce           = "once"
	automationFrequencyCustom         = "custom"
	maxDSTShift                       = 3 * time.Hour
)

var automationWeekdays = map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}

var automationCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// AutomationSchedule is the schedule trigger of an automation (ADR101 D2).
type AutomationSchedule struct {
	Frequency string     `json:"frequency"`
	Minute    *int       `json:"minute,omitempty"`   // hour
	At        string     `json:"at,omitempty"`       // day, week: "HH:MM" 24h
	Weekdays  []string   `json:"weekdays,omitempty"` // week: mon..sun
	RunAt     *time.Time `json:"runAt,omitempty"`    // once
	Cron      string     `json:"cron,omitempty"`     // custom: five fields
	Timezone  string     `json:"timezone"`           // IANA name, required
	EndsAt    *time.Time `json:"endsAt,omitempty"`   // exclusive
}

func scheduleInvalid(field, msg string) error {
	return core.NewBadRequestError(CodeAutomationScheduleInvalid, msg, map[string]any{"field": field})
}

// compiledSchedule is a validated schedule ready to compute fires.
type compiledSchedule struct {
	s    AutomationSchedule // normalized
	loc  *time.Location
	spec *cron.SpecSchedule // nil for once
}

// Normalize returns the canonical form of the schedule (lowercase frequency,
// zero-padded "at", sorted unique lowercase weekdays) or the 400 INVALID error
// naming the offending field. It does not look at the clock.
func (s AutomationSchedule) Normalize() (AutomationSchedule, error) {
	c, err := s.compile()
	if err != nil {
		return AutomationSchedule{}, err
	}
	return c.s, nil
}

func (s AutomationSchedule) compile() (*compiledSchedule, error) {
	n := s
	n.Frequency = strings.ToLower(strings.TrimSpace(s.Frequency))
	n.Weekdays = nil

	tz := s.Timezone
	if tz == "" || tz == "Local" {
		return nil, scheduleInvalid("timezone", "timezone is required and must be an IANA name such as Europe/Berlin")
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, scheduleInvalid("timezone", "timezone is not a known IANA zone")
	}

	// Which fields each frequency owns; everything else must be empty.
	var allowed string
	switch n.Frequency {
	case automationFrequencyHour:
		allowed = "minute"
	case automationFrequencyDay:
		allowed = "at"
	case automationFrequencyWeek:
		allowed = "at,weekdays"
	case automationFrequencyOnce:
		allowed = "runAt"
	case automationFrequencyCustom:
		allowed = "cron"
	default:
		return nil, scheduleInvalid("frequency", "frequency must be one of hour, day, week, once, custom")
	}
	present := map[string]bool{
		"minute":   s.Minute != nil,
		"at":       s.At != "",
		"weekdays": len(s.Weekdays) > 0,
		"runAt":    s.RunAt != nil,
		"cron":     s.Cron != "",
	}
	for _, f := range []string{"minute", "at", "weekdays", "runAt", "cron"} {
		if present[f] && !slices.Contains(strings.Split(allowed, ","), f) {
			return nil, scheduleInvalid(f, fmt.Sprintf("%s does not apply to a %s schedule", f, n.Frequency))
		}
	}
	for _, f := range strings.Split(allowed, ",") {
		if !present[f] {
			return nil, scheduleInvalid(f, fmt.Sprintf("%s is required for a %s schedule", f, n.Frequency))
		}
	}

	c := &compiledSchedule{s: n, loc: loc}
	n.Timezone = tz
	c.s = n
	switch n.Frequency {
	case automationFrequencyOnce:
		return c, nil
	case automationFrequencyHour:
		if *s.Minute < 0 || *s.Minute > 59 {
			return nil, scheduleInvalid("minute", "minute must be between 0 and 59")
		}
	case automationFrequencyDay, automationFrequencyWeek:
		h, m, ok := parseClock(s.At)
		if !ok {
			return nil, scheduleInvalid("at", `at must be a 24-hour "HH:MM" time`)
		}
		c.s.At = fmt.Sprintf("%02d:%02d", h, m)
		if n.Frequency == automationFrequencyWeek {
			days := map[int]string{}
			for _, d := range s.Weekdays {
				name := strings.ToLower(strings.TrimSpace(d))
				num, ok := automationWeekdays[name]
				if !ok {
					return nil, scheduleInvalid("weekdays", "weekdays must be mon, tue, wed, thu, fri, sat or sun")
				}
				days[num] = name
			}
			nums := make([]int, 0, len(days))
			for k := range days {
				nums = append(nums, k)
			}
			sort.Ints(nums)
			for _, k := range nums {
				c.s.Weekdays = append(c.s.Weekdays, days[k])
			}
		}
	case automationFrequencyCustom:
		if strings.Contains(s.Cron, "TZ=") {
			return nil, scheduleInvalid("cron", "cron must not carry a TZ= prefix; set timezone instead")
		}
	}

	parsed, err := automationCronParser.Parse(c.s.CronSpec())
	if err != nil {
		return nil, scheduleInvalid("cron", "cron must be a valid five-field expression: "+err.Error())
	}
	spec, ok := parsed.(*cron.SpecSchedule)
	if !ok {
		return nil, scheduleInvalid("cron", "cron must be a valid five-field expression")
	}
	c.spec = spec
	return c, nil
}

// parseClock parses a strict "HH:MM".
func parseClock(at string) (h, m int, ok bool) {
	if len(at) != 5 || at[2] != ':' {
		return 0, 0, false
	}
	for _, i := range []int{0, 1, 3, 4} {
		if at[i] < '0' || at[i] > '9' {
			return 0, 0, false
		}
	}
	h = int(at[0]-'0')*10 + int(at[1]-'0')
	m = int(at[3]-'0')*10 + int(at[4]-'0')
	return h, m, h <= 23 && m <= 59
}

// CronSpec is the five-field cron expression the schedule compiles to; for
// custom the expression as given, for once (or an unusable schedule) "".
func (s AutomationSchedule) CronSpec() string {
	switch strings.ToLower(strings.TrimSpace(s.Frequency)) {
	case automationFrequencyHour:
		if s.Minute == nil {
			return ""
		}
		return fmt.Sprintf("%d * * * *", *s.Minute)
	case automationFrequencyDay:
		h, m, ok := parseClock(s.At)
		if !ok {
			return ""
		}
		return fmt.Sprintf("%d %d * * *", m, h)
	case automationFrequencyWeek:
		h, m, ok := parseClock(s.At)
		if !ok {
			return ""
		}
		set := map[int]bool{}
		for _, d := range s.Weekdays {
			if n, ok := automationWeekdays[strings.ToLower(strings.TrimSpace(d))]; ok {
				set[n] = true
			}
		}
		nums := make([]string, 0, len(set))
		for n := 0; n <= 6; n++ {
			if set[n] {
				nums = append(nums, fmt.Sprint(n))
			}
		}
		if len(nums) == 0 {
			return ""
		}
		return fmt.Sprintf("%d %d * * %s", m, h, strings.Join(nums, ","))
	case automationFrequencyCustom:
		return s.Cron
	}
	return ""
}

// Validate refuses a schedule that cannot be stored: 400
// AGENT_AUTOMATION_SCHEDULE_INVALID for a bad field, _PAST for a once or
// endsAt that is not after now, _TOO_FREQUENT when two consecutive fires are
// closer than minScheduleInterval.
func (s AutomationSchedule) Validate(now time.Time) error {
	c, err := s.compile()
	if err != nil {
		return err
	}
	if c.s.EndsAt != nil && !c.s.EndsAt.After(now) {
		return core.NewBadRequestError(CodeAutomationSchedulePast, "endsAt must be in the future", map[string]any{"field": "endsAt"})
	}
	if c.s.Frequency == automationFrequencyOnce {
		if !c.s.RunAt.After(now) {
			return core.NewBadRequestError(CodeAutomationSchedulePast, "runAt must be in the future", map[string]any{"field": "runAt"})
		}
		if c.s.EndsAt != nil && !c.s.RunAt.Before(*c.s.EndsAt) {
			return scheduleInvalid("endsAt", "endsAt must be after runAt")
		}
		return nil
	}
	prev, ok := c.next(now)
	for i := 1; ok && i < frequencyScanFires; i++ {
		var cur time.Time
		if cur, ok = c.next(prev); !ok {
			break
		}
		if cur.Sub(prev) < minScheduleInterval {
			return core.NewBadRequestError(CodeAutomationScheduleTooFrequent,
				"the schedule fires more often than every 5 minutes",
				map[string]any{"field": "cron", "minIntervalSeconds": int(minScheduleInterval / time.Second)})
		}
		prev = cur
	}
	if _, ok := c.next(now); !ok && c.spec != nil {
		return scheduleInvalid("cron", "the schedule never fires")
	}
	return nil
}

// Next is the first fire strictly after the given instant, in UTC; false when
// the schedule is finished (once already past, or the next fire is at or after
// EndsAt) or unusable.
func (s AutomationSchedule) Next(after time.Time) (time.Time, bool) {
	c, err := s.compile()
	if err != nil {
		return time.Time{}, false
	}
	return c.next(after)
}

func (c *compiledSchedule) next(after time.Time) (time.Time, bool) {
	var out time.Time
	if c.spec == nil { // once
		if !c.s.RunAt.After(after) {
			return time.Time{}, false
		}
		out = c.s.RunAt.UTC()
	} else {
		w := wallOf(after, c.loc).Add(-maxDSTShift)
		for range 400 { // up to maxDSTShift of per-minute walls, then the answer
			w = c.spec.Next(w)
			if w.IsZero() {
				return time.Time{}, false
			}
			if t := wallToInstant(w, c.loc); t.After(after) {
				out = t.UTC()
				break
			}
		}
		if out.IsZero() {
			return time.Time{}, false
		}
	}
	if c.s.EndsAt != nil && !out.Before(*c.s.EndsAt) {
		return time.Time{}, false
	}
	return out, true
}

// wallOf renders t's wall clock in loc as a UTC time (seconds precision).
func wallOf(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), l.Minute(), l.Second(), 0, time.UTC)
}

// wallToInstant maps a wall-clock time (given as a UTC time) to the instant it
// names in loc: the first occurrence when it occurs twice, and the time shifted
// forward by the gap when it does not exist.
func wallToInstant(w time.Time, loc *time.Location) time.Time {
	_, before := w.Add(-24 * time.Hour).In(loc).Zone()
	_, after := w.Add(24 * time.Hour).In(loc).Zone()
	var best time.Time
	for _, off := range []int{before, after} {
		c := w.Add(-time.Duration(off) * time.Second)
		if wallOf(c, loc).Equal(w) && (best.IsZero() || c.Before(best)) {
			best = c
		}
	}
	if best.IsZero() { // in the gap
		return w.Add(-time.Duration(before) * time.Second)
	}
	return best
}

// Advance is called when a stored next_fire_at (due) is <= now. It returns the
// latest tick <= now (the one to run), how many earlier ticks between due and
// it are skipped (capped at maxMissedTicks), and the first tick strictly after
// now (nil when the schedule is finished). When due is not yet reached, or it
// is a tick that EndsAt has cancelled, fireAt is the zero time and nothing runs.
func (s AutomationSchedule) Advance(due, now time.Time) (fireAt time.Time, next *time.Time, missed int) {
	c, err := s.compile()
	if err != nil || due.After(now) {
		return time.Time{}, nil, 0
	}
	if c.s.EndsAt != nil && !due.Before(*c.s.EndsAt) {
		return time.Time{}, nil, 0
	}
	cur := due.UTC()
	for {
		n, ok := c.next(cur)
		if !ok {
			return cur, nil, missed
		}
		if n.After(now) {
			return cur, &n, missed
		}
		if missed == maxMissedTicks {
			// Too many to walk: find the latest tick directly.
			latest := c.latestAtOrBefore(now)
			if latest.Before(cur) {
				latest = cur
			}
			nn, ok := c.next(now)
			if !ok {
				return latest, nil, missed
			}
			return latest, &nn, missed
		}
		missed++
		cur = n
	}
}

// latestAtOrBefore finds the last tick <= now by looking back over doubling
// windows, so the walk stays short however long the outage was.
func (c *compiledSchedule) latestAtOrBefore(now time.Time) time.Time {
	for w := minScheduleInterval; w < 400*24*time.Hour; w *= 2 {
		t, ok := c.next(now.Add(-w))
		if !ok || t.After(now) {
			continue
		}
		for range maxMissedTicks {
			n, ok := c.next(t)
			if !ok || n.After(now) {
				break
			}
			t = n
		}
		return t
	}
	return time.Time{}
}
