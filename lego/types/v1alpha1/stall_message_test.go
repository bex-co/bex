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

package v1alpha1

import (
	"slices"
	"strings"
	"testing"
)

func TestTerminalStallTextRestatesEachInFlightPhrase(t *testing.T) {
	for waiting, failed := range map[string]string{
		ProbeStallWaiting:         ProbeStallFailed,
		ImagePullStallWaiting:     ImagePullStallFailed,
		UnschedulableStallWaiting: UnschedulableStallFailed,
		QuotaStallWaiting:         QuotaStallFailed,
		CrashStallWaiting:         CrashStallFailed,
	} {
		if got := TerminalStallText("x " + waiting + " y"); got != "x "+failed+" y" {
			t.Errorf("TerminalStallText(%q) = %q", waiting, got)
		}
	}
	if got := TerminalStallText("build failed: exit 1"); got != "build failed: exit 1" {
		t.Errorf("unrelated text changed: %q", got)
	}
}

// w8/386: a startup crash restates as the instance that stopped, a non-zero
// exit keeps the port-bind hint and exit code 0 gets Render's early-exit
// guidance, and every form parses as its code with its hint as a part.
func TestStartupCrashReasonRestatesAsStopped(t *testing.T) {
	for _, tc := range []struct {
		name              string
		code              int32
		port              int
		waiting, failed   string
		hints             []string
		wantIn, wantNotIn string
	}{
		{"exit 1", 1, 10000, DeployReasonStartupCrashRestarting, DeployReasonStartupCrashStopped,
			[]string{DeployReasonHintCrashPortBind}, "listen on $PORT (10000)", "SIGTERM"},
		{"exit 1, no port", 1, 0, DeployReasonStartupCrashRestarting, DeployReasonStartupCrashStopped, nil, "crash output", "$PORT"},
		{"exit 0", 0, 10000, DeployReasonStartupEarlyExitRestarting, DeployReasonStartupEarlyExitStopped,
			[]string{DeployReasonHintEarlyExitPort}, `exited "successfully" but early`, "port bind"},
		{"exit 0, no port", 0, 0, DeployReasonStartupEarlyExitRestarting, DeployReasonStartupEarlyExitStopped, nil, "SIGTERM", "$PORT"},
	} {
		waiting := StartupCrashReason(tc.code, tc.port)
		failed := TerminalStallText(waiting)
		for text, want := range map[string]string{waiting: tc.waiting, failed: tc.failed} {
			code, params, ok := ParseDeployReason(text)
			var hints []string
			for _, p := range params {
				if p.Name == "hint" {
					parts, rest := ParseDeployReasonParts(p.Value)
					for _, part := range parts {
						hints = append(hints, part.Code)
					}
					if rest != "" {
						t.Errorf("%s: %q leaves %q unparsed", tc.name, text, rest)
					}
				}
			}
			if !ok || code != want || !slices.Equal(hints, tc.hints) {
				t.Errorf("%s: %q parses as %s %v (ok=%v), want %s %v", tc.name, text, code, hints, ok, want, tc.hints)
			}
			if !strings.Contains(text, tc.wantIn) || strings.Contains(text, tc.wantNotIn) {
				t.Errorf("%s: %q, want %q and not %q in it", tc.name, text, tc.wantIn, tc.wantNotIn)
			}
		}
		if !strings.Contains(failed, "its instance has stopped") || strings.Contains(failed, "restarting") {
			t.Errorf("%s: the outcome %q still restarts", tc.name, failed)
		}
	}
}
