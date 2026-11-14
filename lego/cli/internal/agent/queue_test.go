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
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
)

// sendFake is the steer route alone: answer decides each request's reply and
// every request's Idempotency-Key is kept.
type sendFake struct {
	*routeFake
	mu   sync.Mutex
	keys []string
}

func newSendFake(t *testing.T, answer func(n int, w http.ResponseWriter)) *sendFake {
	f := &sendFake{}
	f.routeFake = newRouteFake(t, map[string]http.HandlerFunc{
		"POST /v1/agent-sessions/{id}/steer": func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			f.keys = append(f.keys, r.Header.Get("Idempotency-Key"))
			n := len(f.keys)
			f.mu.Unlock()
			answer(n, w)
		},
		"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, sessionJSON(r.PathValue("id"), stateWorking))
		},
	})
	return f
}

func runningSession(extra map[string]any) map[string]any {
	v := sessionJSON("ags-run1", stateWorking)
	v["turns"] = 1
	for k, val := range extra {
		v[k] = val
	}
	return v
}

func queuedAnswer(position int) func(int, http.ResponseWriter) {
	return func(_ int, w http.ResponseWriter) {
		writeJSON(w, http.StatusAccepted, runningSession(map[string]any{
			"pendingMessages": position,
			"queuedMessage":   map[string]any{"id": "qm-1", "position": position, "prompt": "more", "state": "pending"},
		}))
	}
}

// ADR047 D12: a send to a session whose turn is running is queued by the
// server (202). That is a success: the message is on the server, the exit is
// 0, JSON output is the API's own answer, and stderr says where it stands.
func TestSendToARunningSessionIsQueued(t *testing.T) {
	const queuedNote = "Queued (position 2): it runs when the session's running turn ends."
	for _, tc := range []struct {
		name       string
		args       []string
		wantStdout string
		wantStderr string
		noPolling  bool
	}{
		{"text", []string{"send", "ags-run1", "more"}, "ags-run1 · working", queuedNote, true},
		{"json", []string{"send", "ags-run1", "-o", "json", "more"}, `"queuedMessage"`, queuedNote, true},
		{"wait is not waited for", []string{"send", "ags-run1", "--wait", "more"}, "ags-run1 · working",
			queuedNote + " Not waiting: a queued message runs as its own turn.", true},
		{"follow is not followed", []string{"send", "ags-run1", "--follow", "more"}, "ags-run1 · working",
			"Not waiting: a queued message runs as its own turn.", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSendFake(t, queuedAnswer(2))
			stdout, stderr, code := run(t, f.deps(), tc.args...)
			if code != 0 {
				t.Fatalf("exit %d: a queued send is a success: %s", code, stderr)
			}
			if !strings.Contains(stdout, tc.wantStdout) || !strings.Contains(stderr, tc.wantStderr) {
				t.Fatalf("stdout %q stderr %q", stdout, stderr)
			}
			if tc.name == "json" {
				var answer map[string]any
				if err := json.Unmarshal([]byte(stdout), &answer); err != nil || answer["queuedMessage"].(map[string]any)["id"] != "qm-1" {
					t.Fatalf("stdout is not the API's own answer: %v\n%s", err, stdout)
				}
				if strings.Contains(stdout, "Queued (") {
					t.Fatalf("the note went to stdout:\n%s", stdout)
				}
			}
			if calls := f.callsTo(http.MethodGet); tc.noPolling && len(calls) != 0 {
				t.Fatalf("a queued send waited for the running turn: %+v", calls)
			}
			if f.keys[0] != "bex-cli-testkey" {
				t.Fatalf("Idempotency-Key %q", f.keys[0])
			}
		})
	}
}

// A send that starts a turn is as it was: no queue note.
func TestSendThatStartsATurnSaysNothingOfAQueue(t *testing.T) {
	f := newSendFake(t, func(_ int, w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, runningSession(map[string]any{"turns": 2}))
	})
	stdout, stderr, code := run(t, f.deps(), "send", "ags-run1", "more")
	if code != 0 || !strings.Contains(stdout, "ags-run1 · working") || strings.Contains(stderr, "Queued") {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

// The refusals that say what to do next say it in this command's terms; the
// exit stays 1, as for any refused request, and an older server's 409 for a
// running turn is the error it always was.
func TestSendExplainsWhyItWasRefused(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		status int
		code   string
		api    string
		want   []string
	}{
		{"queue full", []string{"send", "ags-run1", "more"}, http.StatusConflict, "AGENT_SESSION_QUEUE_FULL",
			"the session already has the most messages it can queue; wait for one to run or delete one",
			[]string{"409 AGENT_SESSION_QUEUE_FULL: the session's queue is full; wait for a queued message to run, then send again"}},
		{"inline secrets", []string{"send", "ags-run1", "--secret-from-env", "API_TOKEN", "more"}, http.StatusConflict, "AGENT_SESSION_QUEUE_INLINE_SECRETS",
			"a message that carries inline secrets cannot wait in the queue; name workspace secrets in secretNames",
			[]string{"409 AGENT_SESSION_QUEUE_INLINE_SECRETS: a message with inline secrets cannot wait in the queue, because their values are never stored; name workspace secrets with --secret instead of --secret-from-env, or send it once the session is idle"}},
		{"older server, turn in flight", []string{"send", "ags-run1", "more"}, http.StatusConflict, "AGENT_SESSION_TURN_IN_FLIGHT",
			"a turn is already running",
			[]string{"409 AGENT_SESSION_TURN_IN_FLIGHT: a turn is already running"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSendFake(t, func(_ int, w http.ResponseWriter) { writeAPIError(w, tc.status, tc.code, tc.api) })
			d := f.deps()
			d.lookupEnv = func(string) (string, bool) { return "tok-123", true }
			_, stderr, code := run(t, d, tc.args...)
			if code != 1 {
				t.Fatalf("exit %d, want 1: %s", code, stderr)
			}
			for _, want := range tc.want {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr lacks %q:\n%s", want, stderr)
				}
			}
		})
	}
}

// A send is one attempt. The server records no key for a send that started a
// turn, so repeating one whose answer was lost would run the message twice; a
// transient failure is reported, and the person decides whether to send again.
// The key is still sent, so a message still queued deduplicates against it.
func TestSendReportsATransientFailureAndDoesNotRepeatIt(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			f := newSendFake(t, func(_ int, w http.ResponseWriter) { writeAPIError(w, status, "", "rolling") })
			_, stderr, code := run(t, f.deps(), "send", "ags-run1", "more")
			if code != 1 || !strings.Contains(stderr, "rolling") {
				t.Fatalf("exit %d, want 1 with the failure reported: %s", code, stderr)
			}
			if !slices.Equal(f.keys, []string{"bex-cli-testkey"}) {
				t.Fatalf("the send was attempted %d times (keys %q), want once under its key", len(f.keys), f.keys)
			}
		})
	}
}
