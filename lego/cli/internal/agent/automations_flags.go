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

package agent

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Schedule frequencies (ADR101 D2).
const (
	frequencyHour   = "hour"
	frequencyDay    = "day"
	frequencyWeek   = "week"
	frequencyOnce   = "once"
	frequencyCustom = "custom"
)

// frequencyFields are the schedule fields each frequency uses, so a new
// frequency keeps the stored ones that still apply and drops the rest.
var frequencyFields = map[string][]string{
	frequencyHour:   {"minute"},
	frequencyDay:    {"at"},
	frequencyWeek:   {"at", "weekdays"},
	frequencyOnce:   {"runAt"},
	frequencyCustom: {"cron"},
}

// anyChanged reports whether the user set any flag of fs. The flags are
// shared with the command's own set, so parsing marks them here too.
func anyChanged(fs *pflag.FlagSet) bool {
	changed := false
	fs.VisitAll(func(f *pflag.Flag) { changed = changed || f.Changed })
	return changed
}

// scheduleFlags build an automation's one schedule trigger (ADR101 D2). The
// CLI checks only which flags go together; the API owns every rule about
// the schedule itself (minimum interval, past times, cron syntax).
type scheduleFlags struct {
	every, at, once, cron, timezone, ends string
	minute                                int
	on                                    []string
	noEnd                                 bool
	fs                                    *pflag.FlagSet
}

func (s *scheduleFlags) bind(c *cobra.Command) {
	f := pflag.NewFlagSet("schedule", pflag.ContinueOnError)
	f.StringVar(&s.every, "every", "", "repeat every hour, day or week")
	f.StringVar(&s.at, "at", "", "time of day HH:MM (24h), with --every day or week")
	f.IntVar(&s.minute, "minute", 0, "minute past the hour, with --every hour")
	f.StringSliceVar(&s.on, "on", nil, "weekdays, with --every week: mon,tue,…")
	f.StringVar(&s.once, "once", "", "fire once at this time: RFC 3339, or YYYY-MM-DDTHH:MM in --timezone")
	f.StringVar(&s.cron, "cron", "", "a five-field cron expression: \"M H DoM Mon DoW\"")
	f.StringVar(&s.timezone, "timezone", "", "IANA time zone (default: this machine's)")
	f.StringVar(&s.ends, "ends", "", "stop firing at this time (exclusive); a bare date YYYY-MM-DD includes that whole day")
	f.BoolVar(&s.noEnd, "no-end", false, "remove the end date")
	c.Flags().AddFlagSet(f)
	s.fs = f
}

func (s *scheduleFlags) changed() bool { return anyChanged(s.fs) }

// build returns the schedule the flags describe. current is the stored
// schedule on update (nil on create): flags it does not name keep their
// value. A new --every, --once or --cron keeps only the stored fields its
// frequency still uses (with the time zone and end), so a day schedule
// switched to week keeps its --at.
func (s *scheduleFlags) build(c *cobra.Command, d deps, current map[string]any) (map[string]any, error) {
	f := s.fs
	kinds := 0
	for _, name := range []string{"every", "once", "cron"} {
		if f.Changed(name) {
			kinds++
		}
	}
	switch {
	case kinds > 1:
		return nil, usageError("give one of --every, --once or --cron")
	case kinds == 0 && current == nil:
		return nil, usageError("give a schedule: --every hour|day|week, --once TIME or --cron EXPR")
	case f.Changed("ends") && s.noEnd:
		return nil, usageError("use --ends or --no-end, not both")
	}
	out := map[string]any{}
	if kinds == 0 {
		out = maps.Clone(current)
	} else if current != nil {
		for _, keep := range []string{"timezone", "endsAt"} {
			if v, ok := current[keep]; ok && v != nil {
				out[keep] = v
			}
		}
	}
	switch {
	case f.Changed("every"):
		every := strings.ToLower(strings.TrimSpace(s.every))
		if every != frequencyHour && every != frequencyDay && every != frequencyWeek {
			return nil, usageError("--every must be hour, day or week, not %q", s.every)
		}
		out["frequency"] = every
	case f.Changed("once"):
		out["frequency"] = frequencyOnce
	case f.Changed("cron"):
		out["frequency"], out["cron"] = frequencyCustom, s.cron
	}
	if kinds == 1 && current != nil {
		for _, field := range frequencyFields[out["frequency"].(string)] {
			if _, set := out[field]; !set && current[field] != nil {
				out[field] = current[field]
			}
		}
	}
	if f.Changed("timezone") {
		out["timezone"] = s.timezone
	}
	zone, _ := out["timezone"].(string)
	if zone == "" {
		if zone = d.localZone(); zone == "" {
			return nil, usageError("this machine's time zone is unknown: pass --timezone, an IANA name such as Europe/Berlin")
		}
		out["timezone"] = zone
		_, _ = fmt.Fprintf(c.ErrOrStderr(), "Time zone: %s (this machine's; --timezone chooses another)\n", zone)
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, usageError("--timezone %q is not an IANA time zone", zone)
	}
	frequency, _ := out["frequency"].(string)
	if f.Changed("minute") {
		if frequency != frequencyHour {
			return nil, usageError("--minute goes with --every hour")
		}
		out["minute"] = s.minute
	}
	if f.Changed("at") {
		if frequency != frequencyDay && frequency != frequencyWeek {
			return nil, usageError("--at goes with --every day or --every week")
		}
		out["at"] = s.at
	}
	if f.Changed("on") {
		if frequency != frequencyWeek {
			return nil, usageError("--on goes with --every week")
		}
		days := make([]string, 0, len(s.on))
		for _, day := range s.on {
			days = append(days, strings.ToLower(strings.TrimSpace(day)))
		}
		out["weekdays"] = days
	}
	if f.Changed("once") {
		at, err := parseWhen("--once", s.once, loc, false)
		if err != nil {
			return nil, err
		}
		out["runAt"] = at.Format(time.RFC3339)
	}
	if f.Changed("ends") {
		at, err := parseWhen("--ends", s.ends, loc, true)
		if err != nil {
			return nil, err
		}
		out["endsAt"] = at.Format(time.RFC3339)
	}
	if s.noEnd {
		out["endsAt"] = nil
	}
	return out, nil
}

// parseWhen reads RFC 3339, or a local time in loc. With wholeDay, a bare
// date means the end of that day, so `--ends 2026-12-31` still fires on the
// 31st.
func parseWhen(flag, value string, loc *time.Location, wholeDay bool) (time.Time, error) {
	value = strings.TrimSpace(value)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", minuteLayout, "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, nil
		}
	}
	if wholeDay {
		if t, err := time.ParseInLocation(time.DateOnly, value, loc); err == nil {
			return t.AddDate(0, 0, 1), nil
		}
	}
	return time.Time{}, usageError("%s %q is not a time: use RFC 3339 or YYYY-MM-DDTHH:MM", flag, value)
}

// machineZone is this machine's IANA time zone: $TZ, else the
// /etc/localtime link (Linux and macOS); "" when neither names one.
func machineZone() string {
	if tz, ok := os.LookupEnv("TZ"); ok {
		if tz = strings.TrimPrefix(tz, ":"); tz == "" {
			return "UTC"
		}
		return ianaName(tz)
	}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		return ianaName(target)
	}
	return ""
}

func ianaName(v string) string {
	if i := strings.LastIndex(v, "zoneinfo/"); i >= 0 {
		v = v[i+len("zoneinfo/"):]
	}
	if v == "" || v == "Local" || strings.HasPrefix(v, "/") {
		return ""
	}
	if _, err := time.LoadLocation(v); err != nil {
		return ""
	}
	return v
}

// sessionOnly are flags of a single session that a recurring template
// cannot carry, each refused with the reason.
var sessionOnly = []struct{ name, why string }{
	{"attach", "a one-off upload does not belong in a recurring template"},
	{"secret-from-env", "it sends a value from this machine, which a recurring template cannot hold: store it with `bex agent secrets set NAME` and pass --secret NAME"},
	{"idempotency-key", "the scheduler sets one for each fire"},
	{"parent", "each fire starts a top-level session"},
}

func refuseSessionOnly(c *cobra.Command) error {
	for _, flag := range sessionOnly {
		if c.Flags().Changed(flag.name) {
			return usageError("--%s does not fit an automation: %s", flag.name, flag.why)
		}
	}
	return nil
}

// templateFlags build an automation's session template (ADR101 D5): the
// flags `run` takes, with the session-only ones refused.
type templateFlags struct {
	session sessionFlags
	turn    turnFlags
	fs      *pflag.FlagSet
}

func (t *templateFlags) bind(c *cobra.Command) {
	f := pflag.NewFlagSet("template", pflag.ContinueOnError)
	t.session.bind(f)
	t.turn.bind(f)
	f.Lookup("skill").Usage = "NAME or NAME:ARGUMENTS: a workspace skill each run uses; repeatable, up to 5"
	f.Lookup("secret").Usage = "workspace agent secret each run gets as an env var; repeatable"
	f.Lookup("output-schema").Usage = "JSON Schema file each run's result must satisfy"
	for _, flag := range sessionOnly {
		if f.Lookup(flag.name) == nil {
			f.StringArray(flag.name, nil, "")
		}
		_ = f.MarkHidden(flag.name)
	}
	c.Flags().AddFlagSet(f)
	t.fs = f
}

func (t *templateFlags) changed() bool { return anyChanged(t.fs) }

// template returns the template the flags describe. current is the stored
// template on update (nil on create): only the flags given change it, and
// every field the CLI does not model is kept.
func (t *templateFlags) template(c *cobra.Command, d deps, task []string, current map[string]any) (map[string]any, error) {
	f := t.fs
	creating := current == nil
	out := maps.Clone(current)
	if out == nil {
		out = map[string]any{}
	}
	if creating || len(task) > 0 || t.turn.prompt.file != "" {
		prompt, err := t.turn.prompt.read(d, task)
		if err != nil {
			return nil, err
		}
		out["task"] = prompt
	}
	agent, _ := out["agentConfig"].(map[string]any)
	agent = maps.Clone(agent)
	if agent == nil {
		agent = map[string]any{}
	}
	// set writes a flag's value on create (when given a value) or when the
	// flag was given; an emptied flag removes the field.
	set := func(into map[string]any, key, flag, value string) {
		switch {
		case f.Changed(flag) && value == "":
			delete(into, key)
		case f.Changed(flag) || (creating && value != ""):
			into[key] = value
		}
	}
	set(out, "repo", "repo", t.session.repo)
	set(out, "branch", "branch", t.session.branch)
	set(out, "title", "title", t.session.title)
	set(agent, "agent", "agent", t.session.agent)
	set(agent, "model", "model", t.session.model)
	out["agentConfig"] = agent
	if f.Changed("tag") {
		out["tags"] = t.session.tags
	}
	spend, err := t.session.maxSpend(c)
	if err != nil {
		return nil, err
	}
	if spend != nil {
		out["maxSpendUSD"] = *spend
	}
	if f.Changed("skill") {
		skills, err := parseSkills(t.turn.skills)
		if err != nil {
			return nil, err
		}
		out["skills"] = skills
	}
	if f.Changed("secret") {
		out["secretNames"] = t.turn.secrets.names
	}
	if f.Changed("output-schema") {
		schema, err := readSchema(t.turn.schema)
		if err != nil {
			return nil, err
		}
		out["outputSchema"] = json.RawMessage(schema)
	}
	return out, nil
}
