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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAutomations is the ADR101 D8 REST surface in memory: automations by
// id, addressable by name in the request's ownerId, with optimistic
// versions, plus just enough of the session routes for `run --wait`.
type fakeAutomations struct {
	t   *testing.T
	mu  sync.Mutex
	srv *httptest.Server

	stored     map[string]map[string]any // id -> automation view
	next       int
	runOutcome string // what POST …/run answers
	conflict   bool   // the next PATCH answers 409 AGENT_AUTOMATION_CONFLICT
	notCreator bool   // PATCH answers 403 AGENT_AUTOMATION_NOT_CREATOR

	requests []capturedCall
}

type capturedCall struct {
	method, path, query string
	body                map[string]any
}

func newFakeAutomations(t *testing.T) *fakeAutomations {
	f := &fakeAutomations{t: t, stored: map[string]map[string]any{}, runOutcome: runStarted}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/", f.serve)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAutomations) deps() deps {
	return stubDeps(func() (*api, error) {
		return &api{server: f.srv.URL + "/v1/", http: f.srv.Client(), download: f.srv.Client(),
			sleep: func(context.Context, time.Duration) error { return nil }}, nil
	})
}

// seed stores an automation as the API would show it.
func (f *fakeAutomations) seed(name string, schedule map[string]any, template map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	id := fmt.Sprintf("aut-%020d", f.next)
	f.stored[id] = map[string]any{
		"id": id, "ownerId": "tea-ws", "name": name, "enabled": true, "createdBy": "usr-me",
		"triggers": []any{map[string]any{"type": "schedule", "schedule": schedule}},
		"template": template, "overlapPolicy": "skip", "nextFireAt": "2026-11-13T08:00:00Z",
		"lastFireAt": nil, "consecutiveFailures": 0, "version": float64(1),
		"createdAt": "2026-11-11T08:00:00Z", "updatedAt": "2026-11-11T08:00:00Z",
	}
	return id
}

func (f *fakeAutomations) find(ref string) map[string]any {
	if a, ok := f.stored[ref]; ok {
		return a
	}
	for _, a := range f.stored {
		if a["name"] == ref {
			return a
		}
	}
	return nil
}

func (f *fakeAutomations) calls(method string) []capturedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []capturedCall
	for _, c := range f.requests {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeAutomations) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			f.t.Errorf("%s %s: body is not a JSON object: %v", r.Method, r.URL.Path, err)
		}
	}
	f.requests = append(f.requests, capturedCall{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body})
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/"), "/")
	if parts[0] == "agent-sessions" && len(parts) == 2 && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"id": parts[1], "state": stateSucceeded, "turns": 1})
		return
	}
	if parts[0] != "agent-automations" {
		http.NotFound(w, r)
		return
	}
	if r.URL.Query().Get("ownerId") != "tea-ws" {
		f.t.Errorf("%s %s without ownerId=tea-ws: %q", r.Method, r.URL.Path, r.URL.RawQuery)
	}
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		list := []any{}
		for _, a := range f.stored {
			list = append(list, a)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ownerId": "tea-ws", "automations": list})
	case len(parts) == 1 && r.Method == http.MethodPost:
		f.next++
		id := fmt.Sprintf("aut-%020d", f.next)
		a := map[string]any{"id": id, "ownerId": "tea-ws", "enabled": true, "createdBy": "usr-me", "version": float64(1)}
		for k, v := range body {
			a[k] = v
		}
		f.stored[id] = a
		writeJSON(w, http.StatusCreated, a)
	default:
		a := f.find(parts[1])
		if a == nil {
			writeAPIError(w, http.StatusNotFound, "AGENT_AUTOMATION_NOT_FOUND", "no workspace automation has this id or name")
			return
		}
		verb := ""
		if len(parts) > 2 {
			verb = parts[2]
		}
		switch {
		case verb == "" && r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, a)
		case verb == "" && r.Method == http.MethodPatch:
			if f.notCreator {
				writeAPIError(w, http.StatusForbidden, codeAutomationNotCreator, "only the member who created an automation can change what it runs")
				return
			}
			if f.conflict || body["ifVersion"] != a["version"] {
				writeAPIError(w, http.StatusConflict, codeAutomationConflict, "the automation changed since it was read")
				return
			}
			for k, v := range body {
				if k != "ifVersion" {
					a[k] = v
				}
			}
			a["version"] = a["version"].(float64) + 1
			writeJSON(w, http.StatusOK, a)
		case verb == "" && r.Method == http.MethodDelete:
			delete(f.stored, a["id"].(string))
			w.WriteHeader(http.StatusNoContent)
		case verb == "pause" || verb == "resume" || verb == "take-over":
			a["enabled"] = verb != "pause"
			writeJSON(w, http.StatusOK, a)
		case verb == "run":
			run := map[string]any{"automationId": a["id"], "fireAt": "2026-11-11T09:00:00Z", "outcome": f.runOutcome, "missedTicks": 0}
			if f.runOutcome == runStarted {
				run["sessionId"] = "ags-fromautomation1"
			} else {
				run["error"] = "the previous run is still working"
			}
			writeJSON(w, http.StatusCreated, run)
		case verb == "runs":
			writeJSON(w, http.StatusOK, map[string]any{"automationId": a["id"], "runs": []any{
				map[string]any{"automationId": a["id"], "fireAt": "2026-11-11T08:00:00Z", "outcome": "started", "sessionId": "ags-one", "missedTicks": 2},
				map[string]any{"automationId": a["id"], "fireAt": "2026-11-10T08:00:00Z", "outcome": "skipped_overlap", "missedTicks": 0},
			}})
		default:
			http.NotFound(w, r)
		}
	}
}

func daily() map[string]any {
	return map[string]any{"frequency": "day", "at": "09:00", "timezone": "Europe/Berlin"}
}

func tpl(task string) map[string]any {
	return map[string]any{"task": task, "repo": "acme/api", "title": task, "agentConfig": map[string]any{"agent": "claude"}, "maxSpendUSD": float64(3)}
}

func asJSON(t *testing.T, v any) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestAutomationCreateSendsTheRESTBody(t *testing.T) {
	f := newFakeAutomations(t)
	_, stderr, code := run(t, f.deps(), "automations", "create", "--name", "error-triage",
		"--cron", "0 9 * * 1-5", "--timezone", "America/New_York",
		"--repo", "acme/api", "--max-spend", "3", "--secret", "SENTRY_TOKEN", "--skill", "triage:fast",
		"--tag", "triage", "Triage", "new", "errors")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	posts := f.calls(http.MethodPost)
	if len(posts) != 1 {
		t.Fatalf("%d POSTs", len(posts))
	}
	want := `{"name":"error-triage","template":{"agentConfig":{"agent":"claude"},"maxSpendUSD":3,"repo":"acme/api",` +
		`"secretNames":["SENTRY_TOKEN"],"skills":[{"arguments":"fast","name":"triage"}],"tags":["triage"],"task":"Triage new errors"},` +
		`"triggers":[{"schedule":{"cron":"0 9 * * 1-5","frequency":"custom","timezone":"America/New_York"},"type":"schedule"}]}`
	if got := asJSON(t, posts[0].body); got != want {
		t.Fatalf("body\n got %s\nwant %s", got, want)
	}
}

func TestAutomationCreateScheduleFlags(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		schedule string
		stderr   string
	}{
		{"daily, machine zone", []string{"--every", "day", "--at", "09:00"},
			`{"at":"09:00","frequency":"day","timezone":"Europe/Berlin"}`, "Time zone: Europe/Berlin (this machine's"},
		{"weekly", []string{"--every", "week", "--at", "08:30", "--on", "Mon,thu", "--timezone", "UTC"},
			`{"at":"08:30","frequency":"week","timezone":"UTC","weekdays":["mon","thu"]}`, ""},
		{"hourly", []string{"--every", "hour", "--minute", "15", "--timezone", "UTC"},
			`{"frequency":"hour","minute":15,"timezone":"UTC"}`, ""},
		{"once, local time read in the zone", []string{"--once", "2026-11-20T09:00", "--timezone", "Europe/Berlin"},
			`{"frequency":"once","runAt":"2026-11-20T09:00:00+01:00","timezone":"Europe/Berlin"}`, ""},
		{"a bare end date includes that day", []string{"--every", "day", "--at", "09:00", "--timezone", "Europe/Berlin", "--ends", "2026-12-31"},
			`{"at":"09:00","endsAt":"2027-01-01T00:00:00+01:00","frequency":"day","timezone":"Europe/Berlin"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAutomations(t)
			args := append([]string{"automations", "create", "--name", "x", "--max-spend", "1"}, tc.args...)
			_, stderr, code := run(t, f.deps(), append(args, "task")...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			triggers := f.calls(http.MethodPost)[0].body["triggers"].([]any)
			if got := asJSON(t, triggers[0].(map[string]any)["schedule"]); got != tc.schedule {
				t.Fatalf("schedule\n got %s\nwant %s", got, tc.schedule)
			}
			if !strings.Contains(stderr, tc.stderr) {
				t.Fatalf("stderr %q lacks %q", stderr, tc.stderr)
			}
		})
	}
}

func TestAutomationCreateUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no schedule", []string{"--name", "x", "t"}, "give a schedule"},
		{"two kinds", []string{"--name", "x", "--every", "day", "--cron", "0 9 * * *", "t"}, "give one of --every, --once or --cron"},
		{"no name", []string{"--every", "day", "--at", "09:00", "t"}, "--name is required"},
		{"minute with day", []string{"--name", "x", "--every", "day", "--minute", "5", "t"}, "--minute goes with --every hour"},
		{"on with day", []string{"--name", "x", "--every", "day", "--on", "mon", "t"}, "--on goes with --every week"},
		{"bad every", []string{"--name", "x", "--every", "month", "t"}, "--every must be hour, day or week"},
		{"bad once", []string{"--name", "x", "--once", "tomorrow", "t"}, `--once "tomorrow" is not a time`},
		{"no task", []string{"--name", "x", "--every", "day", "--at", "09:00"}, "the prompt is empty"},
		{"attach", []string{"--name", "x", "--every", "day", "--attach", "f.txt", "t"}, "a one-off upload does not belong"},
		{"secret from env", []string{"--name", "x", "--every", "day", "--secret-from-env", "TOKEN", "t"}, "bex agent secrets set NAME"},
		{"idempotency key", []string{"--name", "x", "--every", "day", "--idempotency-key", "k", "t"}, "the scheduler sets one"},
		{"parent", []string{"--name", "x", "--every", "day", "--parent", "ags-x", "t"}, "top-level session"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAutomations(t)
			_, stderr, code := run(t, f.deps(), append([]string{"automations", "create"}, tc.args...)...)
			if code != exitUsage || !strings.Contains(stderr, tc.want) {
				t.Fatalf("exit %d, stderr %q; want exit 2 with %q", code, stderr, tc.want)
			}
			if len(f.calls(http.MethodPost)) != 0 {
				t.Fatal("a refused command sent a create")
			}
		})
	}
}

func TestAutomationCreateUnknownMachineZone(t *testing.T) {
	f := newFakeAutomations(t)
	d := f.deps()
	d.localZone = func() string { return "" }
	_, stderr, code := run(t, d, "automations", "create", "--name", "x", "--every", "day", "--at", "09:00", "t")
	if code != exitUsage || !strings.Contains(stderr, "pass --timezone") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestAutomationUpdateChangesOnlyWhatTheFlagsName(t *testing.T) {
	f := newFakeAutomations(t)
	template := tpl("Triage")
	template["egressAllowlist"] = []any{"sentry.io"} // a field the CLI does not model
	f.seed("triage", map[string]any{"frequency": "day", "at": "09:00", "timezone": "Europe/Berlin", "endsAt": "2027-01-01T00:00:00+01:00"}, template)

	_, stderr, code := run(t, f.deps(), "automations", "update", "triage", "--at", "10:00")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	patch := f.calls(http.MethodPatch)[0].body
	if _, ok := patch["template"]; ok {
		t.Fatalf("a schedule-only update resent the template: %v", patch)
	}
	if got, want := asJSON(t, patch["triggers"]), `[{"schedule":{"at":"10:00","endsAt":"2027-01-01T00:00:00+01:00","frequency":"day","timezone":"Europe/Berlin"},"type":"schedule"}]`; got != want {
		t.Fatalf("triggers\n got %s\nwant %s", got, want)
	}
	if patch["ifVersion"] != float64(1) {
		t.Fatalf("ifVersion %v, want the version read", patch["ifVersion"])
	}

	_, stderr, code = run(t, f.deps(), "automations", "update", "triage", "--max-spend", "5", "New", "task")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	patch = f.calls(http.MethodPatch)[1].body
	if _, ok := patch["triggers"]; ok {
		t.Fatalf("a template-only update resent the triggers: %v", patch)
	}
	got := patch["template"].(map[string]any)
	if got["task"] != "New task" || got["maxSpendUSD"] != float64(5) || asJSON(t, got["egressAllowlist"]) != `["sentry.io"]` || got["repo"] != "acme/api" {
		t.Fatalf("template %v: want the new task and spend, every other field kept", got)
	}
}

func TestAutomationUpdateNewFrequencyStartsAFreshSchedule(t *testing.T) {
	f := newFakeAutomations(t)
	f.seed("triage", map[string]any{"frequency": "week", "at": "09:00", "weekdays": []any{"mon"}, "timezone": "Europe/Berlin"}, tpl("t"))
	if _, stderr, code := run(t, f.deps(), "automations", "update", "triage", "--cron", "*/30 * * * *"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got := asJSON(t, f.calls(http.MethodPatch)[0].body["triggers"])
	if want := `[{"schedule":{"cron":"*/30 * * * *","frequency":"custom","timezone":"Europe/Berlin"},"type":"schedule"}]`; got != want {
		t.Fatalf("triggers\n got %s\nwant %s (no stale at/weekdays)", got, want)
	}
}

func TestAutomationUpdateNewFrequencyKeepsTheFieldsItStillUses(t *testing.T) {
	f := newFakeAutomations(t)
	f.seed("triage", daily(), tpl("t"))
	if _, stderr, code := run(t, f.deps(), "automations", "update", "triage", "--every", "week", "--on", "mon,fri"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got := asJSON(t, f.calls(http.MethodPatch)[0].body["triggers"])
	if want := `[{"schedule":{"at":"09:00","frequency":"week","timezone":"Europe/Berlin","weekdays":["mon","fri"]},"type":"schedule"}]`; got != want {
		t.Fatalf("triggers\n got %s\nwant %s (day's --at kept)", got, want)
	}
}

func TestAutomationUpdateConflictIsReportedNotRetried(t *testing.T) {
	f := newFakeAutomations(t)
	f.seed("triage", daily(), tpl("t"))
	f.conflict = true
	_, stderr, code := run(t, f.deps(), "automations", "update", "triage", "--at", "10:00")
	if code != 1 || !strings.Contains(stderr, "re-run to apply on top of the newer version") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if n := len(f.calls(http.MethodPatch)); n != 1 {
		t.Fatalf("%d PATCHes, want exactly one", n)
	}
	if _, stderr, code := run(t, f.deps(), "automations", "update", "triage"); code != exitUsage || !strings.Contains(stderr, "nothing to update") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestAutomationRunExitsByOutcome(t *testing.T) {
	for _, outcome := range []string{"skipped_overlap", "denied", "failed_create"} {
		t.Run(outcome, func(t *testing.T) {
			f := newFakeAutomations(t)
			f.seed("triage", daily(), tpl("t"))
			f.runOutcome = outcome
			stdout, stderr, code := run(t, f.deps(), "automations", "run", "triage", "--wait")
			if code != exitNotStarted || !strings.Contains(stderr, outcome) {
				t.Fatalf("exit %d, stderr %q; want 6 naming %s", code, stderr, outcome)
			}
			if !strings.Contains(stdout, outcome) {
				t.Fatalf("stdout %q does not show the run", stdout)
			}
			if len(f.calls(http.MethodGet)) > 0 {
				t.Fatal("waited on a run that started no session")
			}
		})
	}
	t.Run("started, --wait waits on the session", func(t *testing.T) {
		f := newFakeAutomations(t)
		f.seed("triage", daily(), tpl("t"))
		stdout, stderr, code := run(t, f.deps(), "-o", "json", "automations", "run", "triage", "--wait")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		if !strings.Contains(stdout, `"id": "ags-fromautomation1"`) || !strings.Contains(stderr, "Started ags-fromautomation1") {
			t.Fatalf("stdout %q stderr %q: want the settled session", stdout, stderr)
		}
	})
}

func TestAutomationDestructiveVerbsNeedConfirmation(t *testing.T) {
	f := newFakeAutomations(t)
	f.seed("triage", daily(), tpl("t"))
	for _, verb := range []string{"delete", "take-over"} {
		if _, stderr, code := run(t, f.deps(), "automations", verb, "triage"); code != exitUsage || !strings.Contains(stderr, "pass --confirm") {
			t.Fatalf("%s: exit %d: %s", verb, code, stderr)
		}
	}
	if n := len(f.calls(http.MethodDelete)) + len(f.calls(http.MethodPost)); n != 0 {
		t.Fatalf("%d writes without confirmation", n)
	}
	d := f.deps()
	d.interactive = func() bool { return true }
	d.stdin = strings.NewReader("y\n")
	if _, stderr, code := run(t, d, "automations", "delete", "triage"); code != 0 {
		t.Fatalf("confirmed delete: exit %d: %s", code, stderr)
	}
	if len(f.calls(http.MethodDelete)) != 1 {
		t.Fatal("the confirmed delete was not sent")
	}
}

func TestAutomationTextOutput(t *testing.T) {
	f := newFakeAutomations(t)
	f.seed("triage", map[string]any{"frequency": "week", "at": "08:30", "weekdays": []any{"mon", "thu"}, "timezone": "Europe/Berlin"}, tpl("Triage new errors"))
	stdout, _, code := run(t, f.deps(), "-o", "text", "automations", "list")
	if code != 0 || !strings.Contains(stdout, "every week on Mon, Thu at 08:30 Europe/Berlin") || !strings.Contains(stdout, "2026-11-13 09:00 CET") {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	stdout, _, _ = run(t, f.deps(), "-o", "text", "automations", "get", "triage")
	for _, want := range []string{"· triage · active", "Max spend", "$3 a run", "Triage new errors"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("get lacks %q:\n%s", want, stdout)
		}
	}
	stdout, _, _ = run(t, f.deps(), "-o", "text", "automations", "runs", "triage")
	if !strings.Contains(stdout, "skipped_overlap") || !strings.Contains(stdout, "ags-one") {
		t.Fatalf("runs:\n%s", stdout)
	}
	stdout, _, _ = run(t, f.deps(), "-o", "json", "automations", "get", "triage")
	var view map[string]any
	if err := json.Unmarshal([]byte(stdout), &view); err != nil || view["overlapPolicy"] != "skip" {
		t.Fatalf("-o json is not the API's answer: %v\n%s", err, stdout)
	}
}

func TestScheduleWords(t *testing.T) {
	minute := 5
	ends := time.Date(2027, 1, 1, 0, 0, 0, 0, time.FixedZone("CET", 3600))
	runAt := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	cases := map[string]automationSchedule{
		"every hour at minute 05 UTC":                                    {Frequency: "hour", Minute: &minute, Timezone: "UTC"},
		"every day at 09:00 Europe/Berlin, ends before 2027-01-01 00:00": {Frequency: "day", At: "09:00", Timezone: "Europe/Berlin", EndsAt: &ends},
		"every week on Mon, Thu at 08:30 UTC":                            {Frequency: "week", At: "08:30", Weekdays: []string{"mon", "thu"}, Timezone: "UTC"},
		"once at 2026-11-20 09:00 Europe/Berlin":                         {Frequency: "once", RunAt: &runAt, Timezone: "Europe/Berlin"},
		`cron "0 9 * * 1-5" UTC`:                                         {Frequency: "custom", Cron: "0 9 * * 1-5", Timezone: "UTC"},
	}
	for want, s := range cases {
		if got := scheduleWords(&s); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestApplyPlansCreatesUpdatesAndSkips(t *testing.T) {
	f := newFakeAutomations(t)
	f.seed("same", daily(), tpl("Unchanged task"))
	f.seed("changed", daily(), tpl("Old task"))
	f.seed("absent-from-file", daily(), tpl("Stays"))
	dir := t.TempDir()
	file := filepath.Join(dir, "automations.yaml")
	writeFile(t, file, `name: same
triggers:
  - type: schedule
    schedule: {frequency: day, at: "09:00", timezone: Europe/Berlin}
template:
  task: Unchanged task
  repo: acme/api
  branch: ""
  tags: []
  agentConfig: {agent: claude}
  maxSpendUSD: 3
---
name: changed
template:
  task: New task
  repo: acme/api
  agentConfig: {agent: claude}
  maxSpendUSD: 5
---
name: brand-new
triggers:
  - type: schedule
    schedule: {frequency: hour, minute: 0, timezone: UTC}
template:
  task: Hourly check
  agentConfig: {agent: claude}
  maxSpendUSD: 1
`)
	stdout, stderr, code := run(t, f.deps(), "-o", "text", "automations", "apply", "-f", dir, "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{"unchanged same", "will update changed", "template.maxSpendUSD: 3 → 5", `template.task: "Old task" → "New task"`, "will create brand-new"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("plan lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "absent-from-file") {
		t.Fatalf("the plan mentions an automation the file does not define:\n%s", stdout)
	}
	if n := len(f.calls(http.MethodPost)) + len(f.calls(http.MethodPatch)); n != 0 {
		t.Fatalf("--dry-run wrote %d time(s)", n)
	}
	if gets := f.calls(http.MethodGet); len(gets) != 1 || gets[0].path != "/v1/agent-automations" {
		t.Fatalf("planning three definitions made %d GET(s); want the one list read", len(gets))
	}

	if _, stderr, code := run(t, f.deps(), "automations", "apply", "-f", file); code != exitUsage || !strings.Contains(stderr, "pass --confirm") {
		t.Fatalf("unconfirmed apply: exit %d: %s", code, stderr)
	}
	_, stderr, code = run(t, f.deps(), "-o", "text", "automations", "apply", "-f", file, "--confirm")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	patches, posts := f.calls(http.MethodPatch), f.calls(http.MethodPost)
	if len(patches) != 1 || len(posts) != 1 || len(f.calls(http.MethodDelete)) != 0 {
		t.Fatalf("%d PATCH, %d POST, %d DELETE: want 1, 1, 0", len(patches), len(posts), len(f.calls(http.MethodDelete)))
	}
	if patches[0].body["ifVersion"] != float64(1) || patches[0].body["template"].(map[string]any)["task"] != "New task" {
		t.Fatalf("PATCH body %v", patches[0].body)
	}
	if _, ok := patches[0].body["triggers"]; ok {
		t.Fatal("the update resent triggers the file did not change")
	}
	if posts[0].body["name"] != "brand-new" {
		t.Fatalf("POST body %v", posts[0].body)
	}
}

func TestApplyRefusesWhatIsNotADefinition(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"id: aut-1\nname: x\n":    `"id" is not part of a definition`,
		"template: {task: t}\n":   "has no name",
		"name: a\n---\nname: a\n": `automation "a" is defined twice`,
		"- name: a\n":             "is a mapping of",
	}
	for content, want := range cases {
		file := filepath.Join(dir, "x.yaml")
		writeFile(t, file, content)
		f := newFakeAutomations(t)
		if _, stderr, code := run(t, f.deps(), "automations", "apply", "-f", file, "--dry-run"); code != exitUsage || !strings.Contains(stderr, want) {
			t.Errorf("%q: exit %d, stderr %q; want %q", content, code, stderr, want)
		}
		if len(f.requests) != 0 {
			t.Errorf("%q: sent %d request(s) before refusing", content, len(f.requests))
		}
	}
}

func TestApplyNotCreatorPointsToTakeOver(t *testing.T) {
	f := newFakeAutomations(t)
	f.seed("theirs", daily(), tpl("Old"))
	f.notCreator = true
	file := filepath.Join(t.TempDir(), "a.yaml")
	writeFile(t, file, "name: theirs\ntemplate: {task: New, agentConfig: {agent: claude}}\n")
	_, stderr, code := run(t, f.deps(), "-o", "text", "automations", "apply", "-f", file, "--confirm")
	if code != 1 || !strings.Contains(stderr, "1 of 1 automation(s) could not be applied") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestExportRoundTripsThroughApply(t *testing.T) {
	f := newFakeAutomations(t)
	template := tpl("Triage")
	template["skills"] = []any{map[string]any{"name": "triage"}}
	f.seed("triage", map[string]any{"frequency": "once", "runAt": "2026-11-20T08:00:00Z", "timezone": "Europe/Berlin"}, template)
	f.seed("deps", daily(), tpl("Update deps"))
	exported, stderr, code := run(t, f.deps(), "-o", "text", "automations", "export", "--all")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, field := range []string{"id:", "version:", "nextFireAt:", "createdBy:"} {
		if strings.Contains(exported, field) {
			t.Fatalf("export kept the read-only %s:\n%s", field, exported)
		}
	}
	d := f.deps()
	d.stdin = strings.NewReader(exported)
	stdout, stderr, code := run(t, d, "-o", "text", "automations", "apply", "-f", "-", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if strings.Count(stdout, "unchanged") != 2 || strings.Contains(stdout, "will ") {
		t.Fatalf("export | apply is not a no-op:\n%s", stdout)
	}
}

func TestMachineZone(t *testing.T) {
	for in, want := range map[string]string{
		"Europe/Berlin":                          "Europe/Berlin",
		":America/New_York":                      "America/New_York",
		"/usr/share/zoneinfo/Asia/Tokyo":         "Asia/Tokyo",
		"/var/db/timezone/zoneinfo/Europe/Paris": "Europe/Paris",
		"Not/A_Zone":                             "",
		"Local":                                  "",
	} {
		t.Setenv("TZ", in)
		if got := machineZone(); got != want {
			t.Errorf("TZ=%q: got %q, want %q", in, got, want)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestAutomationContractFixtureDecodes reads the fixture the backend twin
// (TestShapesMatchCLIAutomationContract) checks against the real types.
func TestAutomationContractFixtureDecodes(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/agent-automation-cli-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Automation     automation        `json:"automation"`
		AutomationList automationList    `json:"automationList"`
		Run            automationRun     `json:"run"`
		RunList        automationRunList `json:"runList"`
		Outcomes       []string          `json:"outcomes"`
		ErrorCodes     map[string]string `json:"errorCodes"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	a := golden.Automation
	s := firstSchedule(a)
	if a.ID == "" || a.Name == "" || a.Version != 7 || a.PausedReason == "" || a.NextFireAt == nil || a.LastSessionID == "" || s == nil ||
		s.Timezone == "" || s.At == "" || len(s.Weekdays) != 1 || s.EndsAt == nil || a.Template.MaxSpendUSD == nil || a.Template.AgentConfig.Agent == "" {
		t.Fatalf("automation fields lost: %+v", a)
	}
	if golden.AutomationList.NextCursor == "" || golden.RunList.NextCursor == "" {
		t.Fatal("list cursors lost")
	}
	if r := golden.Run; r.Outcome == "" || r.MissedTicks != 2 || r.Error == "" || r.FireAt.IsZero() {
		t.Fatalf("run fields lost: %+v", r)
	}
	if !slices.Contains(golden.Outcomes, runStarted) {
		t.Fatalf("outcomes %v lack %q", golden.Outcomes, runStarted)
	}
	for _, code := range []string{codeAutomationConflict, codeAutomationNotCreator} {
		if !slices.Contains(slices.Collect(maps.Values(golden.ErrorCodes)), code) {
			t.Fatalf("error codes %v lack %s, which the CLI branches on", golden.ErrorCodes, code)
		}
	}
}

func TestUpdateByAnotherMemberPointsToTakeOver(t *testing.T) {
	f := newFakeAutomations(t)
	f.seed("theirs", daily(), tpl("Old"))
	f.notCreator = true
	_, stderr, code := run(t, f.deps(), "automations", "update", "theirs", "--at", "10:00")
	if code != 1 || !strings.Contains(stderr, "bex agent automations take-over theirs") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestReadPathOrStdinRefusesOversizedInput(t *testing.T) {
	d := stubDeps(nil)
	d.stdin = strings.NewReader("12345")
	if _, err := readPathOrStdin(d, "-", 4); err == nil || exitCode(err) != exitUsage || !strings.Contains(err.Error(), "stdin is larger than") {
		t.Fatalf("an over-limit stdin read gave %v; want a usage error, not a truncated copy", err)
	}
	d.stdin = strings.NewReader("1234")
	if raw, err := readPathOrStdin(d, "-", 4); err != nil || string(raw) != "1234" {
		t.Fatalf("an at-limit read gave %q, %v", raw, err)
	}
	path := filepath.Join(t.TempDir(), "big.yaml")
	writeFile(t, path, "123456")
	if _, err := readPathOrStdin(d, path, 4); err == nil || !strings.Contains(err.Error(), "big.yaml is larger than") {
		t.Fatalf("an over-limit file read gave %v", err)
	}
}
