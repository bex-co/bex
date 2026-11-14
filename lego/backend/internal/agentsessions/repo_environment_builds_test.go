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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The build status file is untrusted: only ADR111 D2's codes are believed,
// strings are bounded, and an absent or torn file is still building.
func TestParseBuildStatus(t *testing.T) {
	long := strings.Repeat("x", maxBuildStepOutput+100) + "tail"
	cases := []struct {
		name   string
		raw    string
		done   bool
		code   string
		head   string
		reason string
	}{
		{name: "absent", raw: "", done: false},
		{name: "torn", raw: `{"state":"succ`, done: false},
		{name: "no verdict", raw: `{"state":"running"}`, done: false},
		{name: "success", raw: `{"state":"succeeded","build":{"head":"0123456789abcdef0123456789abcdef01234567","steps":[]}}`, done: true, head: "0123456789abcdef0123456789abcdef01234567"},
		{name: "success with a bad head", raw: `{"state":"succeeded","build":{"head":"main; rm -rf /","steps":[]}}`, done: true},
		{name: "setup failed", raw: `{"state":"failed","code":"setup_failed","error":"setup step \"deps\"\nexited with code 2"}`, done: true, code: buildFailedSetup, reason: `setup step "deps" exited with code 2`},
		{name: "step timeout", raw: `{"state":"failed","code":"step_timeout","error":"3h"}`, done: true, code: buildFailedStepTimeout, reason: "3h"},
		{name: "a build timeout the driver no longer reports", raw: `{"state":"failed","code":"build_timeout","error":"3h"}`, done: true, code: buildFailedInternalError, reason: "3h"},
		{name: "a code the driver may not claim", raw: `{"state":"failed","code":"storage_limit_reached"}`, done: true, code: buildFailedInternalError, reason: "the build failed"},
		{name: "session code", raw: `{"state":"failed","code":"AGENT_SESSION_SETUP_FAILED","error":"x"}`, done: true, code: buildFailedInternalError, reason: "x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, failure, done := parseBuildStatus(tc.raw)
			if done != tc.done {
				t.Fatalf("done = %v", done)
			}
			if !done {
				return
			}
			if tc.code == "" {
				if failure != nil || report.head != tc.head {
					t.Fatalf("report = %+v failure = %+v", report, failure)
				}
				return
			}
			if failure == nil || failure.code != tc.code || failure.reason != tc.reason {
				t.Fatalf("failure = %+v, want %s %q", failure, tc.code, tc.reason)
			}
		})
	}
	steps := make([]string, 0, maxBuildSteps+5)
	for range maxBuildSteps + 5 {
		steps = append(steps, `{"name":"`+strings.Repeat("n", 100)+`","exitCode":0,"durationMs":-5,"output":"`+long+`"}`)
	}
	report, _, done := parseBuildStatus(`{"state":"succeeded","build":{"steps":[` + strings.Join(steps, ",") + `]}}`)
	var got []RepoEnvironmentBuildStep
	if !done || json.Unmarshal(report.steps, &got) != nil || len(got) != maxBuildSteps {
		t.Fatalf("bounded steps = %d", len(got))
	}
	if r := []rune(got[0].Name); len(r) != maxBuildStepName+1 || got[0].DurationMs != 0 || len(got[0].Output) != maxBuildStepOutput || !strings.HasSuffix(got[0].Output, "tail") {
		t.Fatalf("bounded step = name %d runes, duration %d, output %d bytes", len(r), got[0].DurationMs, len(got[0].Output))
	}
}

// ADR111 D3: the daily refresh rebuilds a repository that never built, whose
// newest build is a day old (less its jitter), of another version, or a
// settled success on an older agent image in its slot, and retries a build
// bex failed after a while; it leaves a fresh build, and a fresh tenant
// failure, alone.
func TestRefreshDue(t *testing.T) {
	now := slotStart("tea-a", "acme/api")
	recent := now.Add(-time.Hour)
	build := func(status, code, version, base string, created time.Time) *store.AgentRepoEnvironmentBuild {
		finished := created.Add(time.Minute)
		return &store.AgentRepoEnvironmentBuild{Status: status, FailureCode: code, Version: version, BaseImage: base, CreatedAt: created, FinishedAt: &finished}
	}
	cases := []struct {
		name   string
		newest *store.AgentRepoEnvironmentBuild
		due    bool
	}{
		{"never built", nil, true},
		{"fresh success", build("success", "", "v1", "agent:1", recent), false},
		{"a day old", build("success", "", "v1", "agent:1", now.Add(-24*time.Hour)), true},
		{"within the spread", build("success", "", "v1", "agent:1", now.Add(-20*time.Hour)), false},
		{"another version", build("failed", "setup_failed", "v0", "agent:1", recent), true},
		{"older agent image, settled", build("success", "", "v1", "agent:0", recent), true},
		{"older agent image, minutes old", build("success", "", "v1", "agent:0", now.Add(-10*time.Minute)), false},
		{"tenant failure", build("failed", "setup_failed", "v1", "agent:0", recent), false},
		{"bex failure just now", build("failed", "internal_error", "v1", "agent:1", now.Add(-2*time.Minute)), false},
		{"bex failure a while ago", build("failed", "internal_error", "v1", "agent:1", now.Add(-20*time.Minute)), true},
	}
	for _, tc := range cases {
		r := store.AgentRepoEnvironmentRefresh{WorkspaceID: "tea-a", Repo: "acme/api", Version: "v1", Newest: tc.newest}
		if got := refreshDue(r, "agent:1", now); got != tc.due {
			t.Errorf("%s: due = %v, want %v", tc.name, got, tc.due)
		}
	}
	if j := refreshJitter("tea-a", "acme/api"); j < 0 || j >= repoEnvironmentRefreshSpread || j != refreshJitter("tea-a", "acme/api") {
		t.Fatalf("jitter = %s, want a fixed share of the spread", j)
	}
	// Outside its slot a settled build of an older agent image waits, so an
	// image release spreads its rebuilds over the spread.
	other := now.Add(buildRefreshSlot)
	r := store.AgentRepoEnvironmentRefresh{WorkspaceID: "tea-a", Repo: "acme/api", Version: "v1", Newest: build("success", "", "v1", "agent:0", other.Add(-time.Hour))}
	if refreshDue(r, "agent:1", other) {
		t.Fatal("a build of an older agent image was due outside its slot")
	}
	slots := map[bool]int{}
	for i := range int(repoEnvironmentRefreshSpread / buildRefreshSlot) {
		slots[inRefreshSlot("tea-a", "acme/api", now.Add(time.Duration(i)*buildRefreshSlot))]++
	}
	if slots[true] != 1 {
		t.Fatalf("a repository falls in %d slots of one spread, want 1", slots[true])
	}
}

// slotStart is a time inside the repository's refresh slot.
func slotStart(workspaceID, repo string) time.Time {
	spread := int64(repoEnvironmentRefreshSpread)
	base := time.Unix(0, time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC).UnixNano()/spread*spread).UTC()
	return base.Add(refreshJitter(workspaceID, repo).Truncate(buildRefreshSlot) + time.Minute)
}
