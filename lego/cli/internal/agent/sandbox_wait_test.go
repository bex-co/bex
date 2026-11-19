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
	"strings"
	"testing"
	"time"
)

// ADR047 § w10/m497: a session the sandbox pool cannot place yet waits for a
// machine, and `bex agent` says so; a start that failed names its cause.

const waitLine = "waiting for a sandbox machine for "

func TestRunSaysOnceThatTheSessionWaitsAndKeepsWaiting(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-new1"] = []string{stateWorking, stateWorking, stateWorking, stateSucceeded}
	f.waiting["ags-new1"] = time.Now().Add(-3 * time.Minute).UTC().Format(time.RFC3339)
	_, stderr, code := run(t, f.deps(t).deps, "run", "--wait", "-o", "json", "--repo", "acme/api", "fix", "the", "crash")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got := strings.Count(stderr, "ags-new1 "+waitLine); got != 1 {
		t.Fatalf("the waiting line printed %d times, want once:\n%s", got, stderr)
	}
	if !strings.Contains(stderr, "starts by itself when a machine frees") {
		t.Fatalf("the waiting line does not say the session starts by itself: %q", stderr)
	}
}

func TestGetShowsTheWait(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-w1"] = []string{stateWorking}
	f.waiting["ags-w1"] = time.Now().Add(-90 * time.Second).UTC().Format(time.RFC3339)
	stdout, stderr, code := run(t, f.deps(t).deps, "-o", "text", "get", "ags-w1")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Waiting") || !strings.Contains(stdout, "for a sandbox machine for 1m3") {
		t.Fatalf("get does not show the wait:\n%s", stdout)
	}
}

func TestLogsSaysAWaitingTurnWaits(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-w2"] = []string{stateWorking}
	f.waiting["ags-w2"] = time.Now().UTC().Format(time.RFC3339)
	_, stderr, code := run(t, f.deps(t).deps, "logs", "ags-w2")
	if code != 0 || !strings.Contains(stderr, "ags-w2 "+waitLine) {
		t.Fatalf("exit %d, stderr %q: logs must say the turn waits for a machine", code, stderr)
	}
}

func TestAStartFailureNamesItsCause(t *testing.T) {
	f := newFakeAPI(t)
	f.raw["ags-f1"] = `{"id":"ags-f1","ownerId":"tea-fake","state":"failed","phase":"failed","turns":1,
		"agentConfig":{"agent":"claude","task":"t"},
		"failureReasonCode":"SANDBOX_START_FAILED","failureCause":"pool_full",
		"failureReason":"No sandbox machine had room for this session for 60 minutes, so it did not start. Send the message again to retry.",
		"outcome":{"state":"failed","turn":1,"failureReasonCode":"SANDBOX_START_FAILED"}}`
	stdout, _, _ := run(t, f.deps(t).deps, "-o", "text", "get", "ags-f1")
	if !strings.Contains(stdout, "SANDBOX_START_FAILED (pool_full): No sandbox machine had room") {
		t.Fatalf("get does not name the cause:\n%s", stdout)
	}
	_, stderr, code := run(t, f.deps(t).deps, "wait", "ags-f1")
	if code != exitFailed || !strings.Contains(stderr, "SANDBOX_START_FAILED (pool_full): No sandbox machine had room") {
		t.Fatalf("wait exit %d, stderr %q, want the failure with its cause", code, stderr)
	}
}
