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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
	}{
		{"text", []string{"send", "ags-run1", "more"}, "ags-run1 · working", queuedNote},
		{"json", []string{"send", "ags-run1", "-o", "json", "more"}, `"queuedMessage"`, queuedNote},
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
			if calls := f.callsTo(http.MethodGet); len(calls) != 0 {
				t.Fatalf("a queued send without --wait or --follow waited: %+v", calls)
			}
			if f.keys[0] != "bex-cli-testkey" {
				t.Fatalf("Idempotency-Key %q", f.keys[0])
			}
		})
	}
}

// sessionStep is one read of the session once the queued message has left the
// queue: its state and how many turns it has had.
type sessionStep struct {
	state string
	turns int
}

// queueWait scripts a session whose turn 1 is running, with a message of ours
// (qm-1, position 1) queued behind it.
type queueWait struct {
	pending     int           // queued-messages lists qm-1 this many times, then no longer
	others      int           // ...and another person's qm-0 this many times
	flaky       int           // queued-messages answers 503 this many times first
	listRefused int           // queued-messages answers this status instead
	reads       []sessionStep // successive session reads; the last repeats
}

// transcriptOf is the one transcript page the follow cases read: a part of
// turn 1, which the follow must not show, and one of turn 2.
const transcriptOf = `{"parts":[` +
	`{"seq":1,"partIndex":0,"turn":1,"part":{"type":"text","text":"old turn output"}},` +
	`{"seq":2,"partIndex":0,"turn":2,"part":{"type":"text","text":"new turn output"}}],` +
	`"turns":[{"turn":1,"prompt":"fix it"},{"turn":2,"prompt":"more"}],"nextAfterSeq":2}`

func newQueueWaitFake(t *testing.T, tc queueWait) *routeFake {
	var lists, reads atomic.Int32
	return newRouteFake(t, map[string]http.HandlerFunc{
		"POST /v1/agent-sessions/{id}/steer": func(w http.ResponseWriter, _ *http.Request) { queuedAnswer(1)(1, w) },
		"GET /v1/agent-sessions/{id}/queued-messages": func(w http.ResponseWriter, _ *http.Request) {
			n := int(lists.Add(1))
			switch {
			case tc.listRefused != 0:
				writeAPIError(w, tc.listRefused, "FORBIDDEN", "forbidden")
				return
			case n <= tc.flaky:
				writeAPIError(w, http.StatusServiceUnavailable, "", "rolling")
				return
			}
			n -= tc.flaky
			list := []map[string]any{}
			if n <= tc.others {
				list = append(list, map[string]any{"id": "qm-0", "position": 1, "prompt": "someone else's", "state": "pending"})
			}
			if n <= tc.pending {
				list = append(list, map[string]any{"id": "qm-1", "position": len(list) + 1, "prompt": "more", "state": "pending"})
			}
			writeJSON(w, http.StatusOK, map[string]any{"queuedMessages": list})
		},
		"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
			if len(tc.reads) == 0 {
				t.Errorf("the session was read: %s", r.URL.Path)
				writeAPIError(w, http.StatusInternalServerError, "", "unexpected read")
				return
			}
			read := tc.reads[min(int(reads.Add(1))-1, len(tc.reads)-1)]
			v := sessionJSON(r.PathValue("id"), read.state)
			v["turns"] = read.turns
			if read.state == stateFailed {
				v["failureReasonCode"], v["failureReason"] = "AGENT_TURN_FAILED", "the tests did not pass"
			}
			writeJSON(w, http.StatusOK, v)
		},
		"GET /v1/agent-sessions/{id}/transcript": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("afterSeq") == "2" {
				writeJSON(w, http.StatusOK, map[string]any{"parts": []any{}, "turns": []any{}, "nextAfterSeq": 2})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(transcriptOf))
		},
	})
}

// pacedDeps is d with a sleep that records each delay and lets a little real
// time pass, so a --timeout deadline can end a wait. The delays are read after
// the command has returned.
func pacedDeps(d deps) (deps, *[]time.Duration) {
	var slept []time.Duration
	open := d.api
	d.api = func() (*api, error) {
		a, err := open()
		if err != nil {
			return nil, err
		}
		a.sleep = func(ctx context.Context, delay time.Duration) error {
			slept = append(slept, delay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Millisecond):
				return nil
			}
		}
		return a, nil
	}
	return d, &slept
}

// paths lists the requests the fake saw, as "METHOD path", in order.
func paths(f *routeFake) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	for i, c := range f.calls {
		out[i] = c.method + " " + c.path
	}
	return out
}

func count(f *routeFake, call string) int {
	n := 0
	for _, p := range paths(f) {
		if p == call {
			n++
		}
	}
	return n
}

const (
	queuePoll  = "GET /v1/agent-sessions/ags-run1/queued-messages"
	sessionGet = "GET /v1/agent-sessions/ags-run1"
)

// ADR047 D12, w22/013: `send --wait` and `--follow` against a running session
// wait for the turn the queued message starts, not the one running. They poll
// the queue until the message (found by its id) is no longer pending, then do
// what the flag does for a direct send: wait for the session to settle and exit
// by the turn's state, or stream the turn. A message that left the queue
// without a turn of its own (the server dropped it, or someone removed it) is
// a failed send, exit 1.
func TestSendWaitsForTheTurnAQueuedMessageStarts(t *testing.T) {
	const (
		queuedNote = "Queued (position 1): it runs when the session's running turn ends. Waiting for it."
		notStarted = "the queued message was not started; see the session"
	)
	started := []sessionStep{{stateWorking, 2}, {stateSucceeded, 2}}
	// The cases that expect a failed send carry a --timeout: a regression that
	// waited on the session instead would spin until it, and fail, not hang.
	for _, tc := range []struct {
		name       string
		args       []string
		fake       queueWait
		wantCode   int
		wantStdout []string
		wantStderr []string
		avoid      []string // not in stdout
		queuePolls int
		sessReads  int // 0: not asserted
	}{
		{name: "started, then succeeds", args: []string{"send", "ags-run1", "--wait", "more"},
			fake: queueWait{pending: 3, reads: started}, wantStdout: []string{"ags-run1 · succeeded"}, wantStderr: []string{queuedNote},
			avoid: []string{"ags-run1 · working"}, queuePolls: 4},
		{name: "json is the settled session, not the 202", args: []string{"send", "ags-run1", "--wait", "-o", "json", "more"},
			fake: queueWait{pending: 1, reads: started}, wantStdout: []string{`"state": "succeeded"`}, wantStderr: []string{queuedNote},
			avoid: []string{`"queuedMessage"`, "Queued ("}, queuePolls: 2},
		{name: "its turn fails", args: []string{"send", "ags-run1", "--wait", "more"},
			fake: queueWait{pending: 1, reads: []sessionStep{{stateWorking, 2}, {stateFailed, 2}}}, wantCode: exitFailed,
			wantStdout: []string{"ags-run1 · failed"}, wantStderr: []string{"AGENT_TURN_FAILED: the tests did not pass"}, queuePolls: 2},
		{name: "its session is canceled", args: []string{"send", "ags-run1", "--wait", "more"},
			fake: queueWait{pending: 1, reads: []sessionStep{{stateWorking, 2}, {stateCanceled, 2}}}, wantCode: exitCanceled,
			wantStderr: []string{"agent session ags-run1 was canceled"}, queuePolls: 2},
		{name: "its turn is over before the queue is read again", args: []string{"send", "ags-run1", "--wait", "more"},
			fake: queueWait{pending: 2, reads: []sessionStep{{stateSucceeded, 2}}}, wantStdout: []string{"ags-run1 · succeeded"}, queuePolls: 3},
		{name: "waits for its own message, not an empty queue", args: []string{"send", "ags-run1", "--wait", "more"},
			fake: queueWait{pending: 2, others: 9, reads: started}, wantStdout: []string{"ags-run1 · succeeded"}, queuePolls: 3},
		{name: "survives transient failures reading the queue", args: []string{"send", "ags-run1", "--wait", "more"},
			fake: queueWait{pending: 1, flaky: 3, reads: started}, wantStdout: []string{"ags-run1 · succeeded"}, queuePolls: 5},
		{name: "follow streams the turn it starts", args: []string{"send", "ags-run1", "--follow", "more"},
			fake: queueWait{pending: 2, reads: started}, wantStdout: []string{"new turn output"}, avoid: []string{"old turn output"},
			wantStderr: []string{"Queued (position 1): it runs when the session's running turn ends. Waiting for it."}, queuePolls: 3},
		{name: "dropped: the session is idle with no new turn", args: []string{"send", "ags-run1", "--wait", "--timeout", "10s", "more"},
			fake: queueWait{pending: 2, reads: []sessionStep{{stateSucceeded, 1}}}, wantCode: 1,
			wantStdout: []string{"ags-run1 · succeeded"}, wantStderr: []string{notStarted}, queuePolls: 3, sessReads: 1},
		{name: "removed while the running turn still runs", args: []string{"send", "ags-run1", "--wait", "--timeout", "10s", "more"},
			fake: queueWait{pending: 1, reads: []sessionStep{{stateWorking, 1}}}, wantCode: 1,
			wantStderr: []string{notStarted}, queuePolls: 2, sessReads: 1},
		{name: "dropped, followed", args: []string{"send", "ags-run1", "--follow", "--timeout", "10s", "more"},
			fake: queueWait{pending: 1, reads: []sessionStep{{stateSucceeded, 1}}}, wantCode: 1,
			wantStderr: []string{notStarted}, avoid: []string{"old turn output"}, queuePolls: 2, sessReads: 1},
		{name: "a refused queue read is a failed send", args: []string{"send", "ags-run1", "--wait", "more"},
			fake: queueWait{listRefused: http.StatusForbidden}, wantCode: 1, wantStderr: []string{"403 FORBIDDEN: forbidden"}, queuePolls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newQueueWaitFake(t, tc.fake)
			stdout, stderr, code := run(t, f.deps(), tc.args...)
			if code != tc.wantCode {
				t.Fatalf("exit %d, want %d\nstdout %s\nstderr %s", code, tc.wantCode, stdout, stderr)
			}
			for _, want := range tc.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout lacks %q:\n%s", want, stdout)
				}
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr)
				}
			}
			for _, bad := range tc.avoid {
				if strings.Contains(stdout, bad) {
					t.Errorf("stdout has %q:\n%s", bad, stdout)
				}
			}
			if got := count(f, queuePoll); got != tc.queuePolls {
				t.Errorf("%d queue reads, want %d: %v", got, tc.queuePolls, paths(f))
			}
			if tc.sessReads != 0 && count(f, sessionGet) != tc.sessReads {
				t.Errorf("%d session reads, want %d: %v", count(f, sessionGet), tc.sessReads, paths(f))
			}
			// The session is not read while the message is still queued: that
			// read would be the running turn's, not the message's.
			if got := paths(f); slices.Index(got, sessionGet) >= 0 && slices.Index(got, sessionGet) < slices.Index(got, queuePoll)+tc.queuePolls {
				t.Errorf("the session was read before the message left the queue: %v", got)
			}
		})
	}
}

// The wait polls the queue as chat's between-turn wait does: 2 s, rising by
// half each time to 15 s.
func TestSendWaitPollsTheQueueWithBackoff(t *testing.T) {
	f := newQueueWaitFake(t, queueWait{pending: 6, reads: []sessionStep{{stateSucceeded, 2}}})
	d, slept := pacedDeps(f.deps())
	if _, stderr, code := run(t, d, "send", "ags-run1", "--wait", "more"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want := []time.Duration{2 * time.Second, 3 * time.Second, 4500 * time.Millisecond, 6750 * time.Millisecond, 10125 * time.Millisecond, 15 * time.Second}
	if !slices.Equal(*slept, want) {
		t.Fatalf("slept %v, want %v", *slept, want)
	}
}

// --timeout covers the whole wait, the queue included. A deadline while the
// message is still queued exits 124, says the message is still queued, and
// leaves it there; stdout keeps the API's answer to the send.
func TestSendWaitTimeoutDuringTheQueueWait(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"text", []string{"send", "ags-run1", "--wait", "--timeout", "50ms", "more"}, "ags-run1 · working"},
		{"follow", []string{"send", "ags-run1", "--follow", "--timeout", "50ms", "more"}, "ags-run1 · working"},
		{"json", []string{"send", "ags-run1", "--wait", "--timeout", "50ms", "-o", "json", "more"}, `"queuedMessage"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newQueueWaitFake(t, queueWait{pending: 1 << 30, reads: []sessionStep{{stateWorking, 1}}})
			d, _ := pacedDeps(f.deps())
			stdout, stderr, code := run(t, d, tc.args...)
			if code != exitTimeout {
				t.Fatalf("exit %d, want %d\nstdout %s\nstderr %s", code, exitTimeout, stdout, stderr)
			}
			if !strings.Contains(stderr, "ags-run1: the message is still queued after 50ms") || !strings.Contains(stderr, "bex agent get ags-run1") {
				t.Errorf("stderr does not say the message is still queued:\n%s", stderr)
			}
			if !strings.Contains(stdout, tc.want) {
				t.Errorf("stdout lacks %q:\n%s", tc.want, stdout)
			}
			if count(f, sessionGet) != 0 || count(f, queuePoll) == 0 {
				t.Errorf("a timeout in the queue wait read the session: %v", paths(f))
			}
		})
	}
}

// Ctrl-C or SIGTERM during the queue wait is not --timeout: exit 130, and the
// message stays queued, which stderr says.
func TestSendWaitInterruptedWhileQueued(t *testing.T) {
	f := newQueueWaitFake(t, queueWait{pending: 1 << 30})
	a, err := f.deps().api()
	if err != nil {
		t.Fatal(err)
	}
	ctx, interrupt := context.WithCancel(context.Background())
	interrupt()
	var out, errOut bytes.Buffer
	inv := &invocation{ctx: ctx, api: a, format: formatText, out: &out, errOut: &errOut}
	queued := session{ID: "ags-run1", State: stateWorking, Turns: 1, Queued: &queuedMessage{ID: "qm-1", Position: 1}}
	err = inv.awaitQueued(queued, []byte(`{"id":"ags-run1"}`), waitFlags{wait: true})
	if exitCode(err) != exitInterrupted || !strings.Contains(err.Error(), "still queued") {
		t.Fatalf("err = %v (exit %d), want exit %d saying the message is still queued", err, exitCode(err), exitInterrupted)
	}
	if out.Len() != 0 {
		t.Fatalf("an interrupted wait printed %q", out.String())
	}
}

// A server from before the queue refuses a message sent during a turn (409
// AGENT_SESSION_TURN_IN_FLIGHT), whatever the wait flags say: exit 1 and
// nothing to wait for.
func TestSendWaitAgainstAnOlderServerIsRefusedAsBefore(t *testing.T) {
	for _, flag := range []string{"--wait", "--follow"} {
		t.Run(flag, func(t *testing.T) {
			f := newSendFake(t, func(_ int, w http.ResponseWriter) {
				writeAPIError(w, http.StatusConflict, "AGENT_SESSION_TURN_IN_FLIGHT", "a turn is already running")
			})
			_, stderr, code := run(t, f.deps(), "send", "ags-run1", flag, "more")
			if code != 1 || !strings.Contains(stderr, "409 AGENT_SESSION_TURN_IN_FLIGHT: a turn is already running") {
				t.Fatalf("exit %d, want 1: %s", code, stderr)
			}
			if calls := f.callsTo(http.MethodGet); len(calls) != 0 {
				t.Fatalf("a refused send waited: %+v", calls)
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

// stopFake is the interrupt route alone: it answers 202 with the session, or
// the scripted refusal.
func stopFake(t *testing.T, refused *refusal) *routeFake {
	return newRouteFake(t, map[string]http.HandlerFunc{
		"POST /v1/agent-sessions/{id}/interrupt": func(w http.ResponseWriter, r *http.Request) {
			if refused != nil {
				writeAPIError(w, refused.status, refused.code, refused.message)
				return
			}
			writeJSON(w, http.StatusAccepted, runningSession(map[string]any{"id": r.PathValue("id")}))
		},
	})
}

// ADR047 D12, w22/013: `bex agent stop` ends the running turn only, as chat's
// /stop does, and says the same words. It asks for no confirmation (even on a
// terminal), unlike `cancel`: the session stays open. The two refusals about
// where the turn stands have their own exit codes so a script can tell them
// apart; any other refusal is exit 1, as for every command.
func TestStopEndsOnlyTheTurn(t *testing.T) {
	const stopping = "Stopping the current turn. The session stays open and any queued messages run next; `bex agent cancel` ends it.\n"
	for _, tc := range []struct {
		name        string
		args        []string
		refusal     *refusal
		interactive bool
		wantCode    int
		wantStdout  string
		wantStderr  string
	}{
		{name: "stops", args: []string{"stop", "ags-run1"}, wantStdout: stopping},
		{name: "stops on a terminal without asking", args: []string{"stop", "ags-run1"}, interactive: true, wantStdout: stopping},
		{name: "a dashboard URL names the session", args: []string{"stop", "https://dashboard.bex.co/agents/ags-run1"}, wantStdout: stopping},
		{name: "no turn is running", args: []string{"stop", "ags-run1"}, wantCode: exitNoTurnRunning,
			refusal:    &refusal{http.StatusConflict, "AGENT_SESSION_NO_TURN_RUNNING", "no turn is running; the session is idle"},
			wantStderr: "stop: no turn is running, so there is nothing to stop"},
		{name: "the turn is still starting", args: []string{"stop", "ags-run1"}, wantCode: exitTurnStarting,
			refusal:    &refusal{http.StatusConflict, "AGENT_SESSION_TURN_STARTING", "the turn is still starting; stop it once it is running"},
			wantStderr: "stop: the turn is still starting; try again once the agent is working"},
		{name: "no turn is running, as json", args: []string{"stop", "ags-run1", "-o", "json"}, wantCode: exitNoTurnRunning,
			refusal:    &refusal{http.StatusConflict, "AGENT_SESSION_NO_TURN_RUNNING", "no turn is running; the session is idle"},
			wantStderr: "stop: no turn is running, so there is nothing to stop"},
		{name: "a canceled session", args: []string{"stop", "ags-run1"}, wantCode: 1,
			refusal:    &refusal{http.StatusConflict, "AGENT_SESSION_NOT_STEERABLE", "the session is canceled"},
			wantStderr: "409 AGENT_SESSION_NOT_STEERABLE: the session is canceled"},
		{name: "not allowed", args: []string{"stop", "ags-run1"}, wantCode: 1,
			refusal: &refusal{http.StatusForbidden, "", "forbidden"}, wantStderr: "stop agent session ags-run1: 403 forbidden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := stopFake(t, tc.refusal)
			d := f.deps()
			d.interactive = func() bool { return tc.interactive }
			stdout, stderr, code := run(t, d, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("exit %d, want %d\nstdout %s\nstderr %s", code, tc.wantCode, stdout, stderr)
			}
			if tc.wantCode == 0 && (stdout != tc.wantStdout || stderr != "") {
				t.Errorf("stdout %q stderr %q, want exactly %q and nothing on stderr", stdout, stderr, tc.wantStdout)
			}
			if tc.wantCode != 0 && (stdout != "" || !strings.Contains(stderr, tc.wantStderr)) {
				t.Errorf("stdout %q stderr %q, want nothing on stdout and %q", stdout, stderr, tc.wantStderr)
			}
			if call := f.only(t); call.method != http.MethodPost || call.path != "/v1/agent-sessions/ags-run1/interrupt" {
				t.Errorf("request %s %s, want the interrupt verb alone", call.method, call.path)
			}
		})
	}
}

// `-o json` is the API's own bytes, with nothing of the CLI's words around
// them.
func TestStopJSONPassesTheAPIAnswerThrough(t *testing.T) {
	f := stopFake(t, nil)
	stdout, _, code := run(t, f.deps(), "stop", "ags-run1", "-o", "json")
	var answer map[string]any
	if err := json.Unmarshal([]byte(stdout), &answer); err != nil || code != 0 || answer["id"] != "ags-run1" || answer["state"] != stateWorking {
		t.Fatalf("exit %d, stdout is not the session the API answered: %v\n%s", code, err, stdout)
	}
}

// A session is needed, and it is named like any other command's: no session is
// a usage error without a terminal.
func TestStopNeedsASession(t *testing.T) {
	f := stopFake(t, nil)
	if _, stderr, code := run(t, f.deps(), "stop"); code != exitUsage || !strings.Contains(stderr, "expected an agent session id") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if len(f.calls) != 0 {
		t.Fatalf("a usage error called the API: %+v", f.calls)
	}
}
