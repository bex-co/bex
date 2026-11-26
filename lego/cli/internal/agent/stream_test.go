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
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

const renderedReplay = "── turn 1 ──\n> Fix the crash in crash.log\n▸ Bash go test ./...\n  ✗ Bash: exit status 1\n▸ edit main.go\nFixed the nil dereference; tests pass.\n"

func replayFixture(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/agent-stream-replay.sse")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestReplayRendersLikeTheTranscript(t *testing.T) {
	var out bytes.Buffer
	r := newChunkRenderer(newPartPrinter(&out, false, false), 0)
	done, err := readSSE(strings.NewReader(replayFixture(t)), r.chunk)
	if !done || err != nil {
		t.Fatalf("done %v, err %v", done, err)
	}
	if out.String() != renderedReplay {
		t.Fatalf("rendered\n%s\nwant\n%s", out.String(), renderedReplay)
	}
	out.Reset()
	r = newChunkRenderer(newPartPrinter(&out, false, true), 0)
	_, _ = readSSE(strings.NewReader(replayFixture(t)), r.chunk)
	if !strings.Contains(out.String(), "(thinking) Read the log first.\n▸ Bash") {
		t.Fatalf("--reasoning:\n%s", out.String())
	}
}

func TestReconnectPrintsEachItemOnce(t *testing.T) {
	fixture := replayFixture(t)
	cut := strings.Index(fixture, "data: {\"type\":\"tool-input-available\",\"toolCallId\":\"call-2\"")
	var out bytes.Buffer
	r := newChunkRenderer(newPartPrinter(&out, false, false), 0)
	if done, err := readSSE(strings.NewReader(fixture[:cut]), r.chunk); done || err == nil {
		t.Fatalf("a cut stream reported done %v, err %v", done, err)
	}
	r.reconnect()
	if done, _ := readSSE(strings.NewReader(fixture), r.chunk); !done {
		t.Fatal("the replay did not end with [DONE]")
	}
	if out.String() != renderedReplay {
		t.Fatalf("after a reconnect\n%s\nwant each item once\n%s", out.String(), renderedReplay)
	}
}

func TestFromTurnSkipsEarlierTurns(t *testing.T) {
	two := replayFixture(t) // turn 1, then a second turn's prompt and text
	two = strings.Replace(two, "data: [DONE]\n", `data: {"type":"data-user-prompt","data":{"turn":2,"text":"Add a test"}}

data: {"type":"text-start","id":"t2"}

data: {"type":"text-delta","id":"t2","delta":"Added."}

data: {"type":"text-end","id":"t2"}

data: [DONE]
`, 1)
	var out bytes.Buffer
	r := newChunkRenderer(newPartPrinter(&out, false, false), 2)
	_, _ = readSSE(strings.NewReader(two), r.chunk)
	if out.String() != "── turn 2 ──\n> Add a test\nAdded.\n" {
		t.Fatalf("from turn 2:\n%s", out.String())
	}
}

// streamFake serves attach-ticket, a stream that drops its first connection
// halfway, and a session that is settled once the stream has ended.
func streamFake(t *testing.T, ticketStatus []int) (*routeFake, *atomic.Int32, *atomic.Int32) {
	fixture := replayFixture(t)
	cut := fixture[:strings.Index(fixture, "data: {\"type\":\"text-start\"")]
	return scriptedStream(t, streamScript{ticketStatus: ticketStatus, bodies: []string{cut, fixture},
		state: func(connects int32) (string, int) {
			if connects >= 2 {
				return stateSucceeded, 1
			}
			return stateWorking, 1
		}})
}

// streamScript is what scriptedStream serves: refusals before the first
// ticket, each connection's body in turn (the last repeats), and the session's
// state and turns (which a ticket reports too) by how many connections the
// stream has had.
type streamScript struct {
	ticketStatus []int
	bodies       []string
	state        func(connects int32) (state string, turns int)
}

func scriptedStream(t *testing.T, sc streamScript) (*routeFake, *atomic.Int32, *atomic.Int32) {
	var mints, connects atomic.Int32
	var f *routeFake
	f = newRouteFake(t, map[string]http.HandlerFunc{
		"POST /v1/agent-sessions/{id}/attach-ticket": func(w http.ResponseWriter, r *http.Request) {
			n := int(mints.Add(1))
			if r.URL.Query().Get("action") != "read" {
				t.Errorf("attach-ticket action %q, want read", r.URL.Query().Get("action"))
			}
			if n <= len(sc.ticketStatus) {
				writeAPIError(w, sc.ticketStatus[n-1], "AGENT_SESSION_NOT_ATTACHABLE", "no sandbox yet")
				return
			}
			_, turns := sc.state(connects.Load())
			writeJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "ticket": "tkt-" + string(rune('a'+n)), "turns": turns,
				"streamUrl": f.srv.URL + "/v1/agent-sessions/" + r.PathValue("id") + "/stream"})
		},
		"GET /v1/agent-sessions/{id}/stream": func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(ticketHeader) == "" || r.Header.Get("Authorization") != "" {
				t.Errorf("stream request headers: ticket %q, authorization %q", r.Header.Get(ticketHeader), r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "text/event-stream")
			n := int(connects.Add(1))
			_, _ = w.Write([]byte(sc.bodies[min(n, len(sc.bodies))-1]))
		},
		"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
			state, turns := sc.state(connects.Load())
			s := sessionJSON(r.PathValue("id"), state)
			s["turns"] = turns
			writeJSON(w, http.StatusOK, s)
		},
	})
	return f, &mints, &connects
}

func TestLogsFollowStreamsAndReconnects(t *testing.T) {
	f, mints, connects := streamFake(t, []int{http.StatusConflict, http.StatusConflict})
	stdout, stderr, code := run(t, f.deps(), "-o", "text", "logs", "ags-live1", "--follow")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if stdout != renderedReplay {
		t.Fatalf("stdout\n%s\nwant\n%s", stdout, renderedReplay)
	}
	if mints.Load() != 4 || connects.Load() != 2 {
		t.Fatalf("%d ticket mints and %d connections: want 2 refusals while provisioning, then a fresh ticket per connection", mints.Load(), connects.Load())
	}
}

func TestFollowFallsBackToPollingWithoutAStream(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-t1"] = []string{stateWorking, stateSucceeded}
	f.transcript["ags-t1"] = []string{`{"seq":1,"partIndex":0,"turn":1,"part":{"type":"text","text":"Polled."}}`}
	stdout, stderr, code := run(t, f.deps(t).deps, "-o", "text", "logs", "ags-t1", "--follow")
	if code != 0 || !strings.Contains(stdout, "Polled.") {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, stdout, stderr)
	}
}

func TestErrorLineSkipsFences(t *testing.T) {
	for in, want := range map[string]string{
		"```\nexit status 1\n```": "exit status 1",
		"\n  \nboom":              "boom",
		"plain":                   "plain",
		"```":                     "```",
	} {
		if got := errorLine(in); got != want {
			t.Errorf("errorLine(%q) = %q, want %q", in, got, want)
		}
	}
}

// `[DONE]` ends a stream only once its turn has settled (w3/m406), though the
// session reads working until the Completer's next poll (about 4.5 s on
// production, 2026-11-26). follow waits for it without attaching again.
func TestFollowWaitsForTheSessionAfterDone(t *testing.T) {
	var reads atomic.Int32
	f, _, connects := scriptedStream(t, streamScript{bodies: []string{replayFixture(t)},
		state: func(int32) (string, int) {
			if reads.Add(1) <= 2 {
				return stateWorking, 1
			}
			return stateSucceeded, 1
		}})
	stdout, stderr, code := run(t, f.deps(), "-o", "text", "logs", "ags-live1", "--follow")
	if code != 0 || stdout != renderedReplay || connects.Load() != 1 {
		t.Fatalf("exit %d after %d connections, want one\nstdout %s\nstderr %s", code, connects.Load(), stdout, stderr)
	}
}

// A message queued behind the streamed turn starts the next one, which has a
// stream of its own: follow attaches again, printing only the new turn.
func TestFollowAttachesAgainForAQueuedTurn(t *testing.T) {
	turn1 := replayFixture(t)
	both := strings.Replace(turn1, "data: [DONE]\n", `data: {"type":"data-user-prompt","data":{"turn":2,"text":"Add a test"}}

data: {"type":"text-start","id":"t2"}

data: {"type":"text-delta","id":"t2","delta":"Added."}

data: {"type":"text-end","id":"t2"}

data: [DONE]
`, 1)
	f, _, connects := scriptedStream(t, streamScript{bodies: []string{turn1, both},
		state: func(connects int32) (string, int) {
			if connects >= 2 {
				return stateSucceeded, 2
			}
			return stateWorking, int(min(connects+1, 2)) // the ticket is minted at turn 1
		}})
	stdout, stderr, code := run(t, f.deps(), "-o", "text", "logs", "ags-live1", "--follow")
	if code != 0 || stdout != renderedReplay+"── turn 2 ──\n> Add a test\nAdded.\n" || connects.Load() != 2 {
		t.Fatalf("exit %d after %d connections\nstdout %s\nstderr %s", code, connects.Load(), stdout, stderr)
	}
}

// A stream that closes without `[DONE]` was cut before its turn settled: it
// is attached again even when the session has settled meanwhile, so the rest
// of the turn still prints.
func TestFollowAttachesAgainAfterACutEvenOnceSettled(t *testing.T) {
	fixture := replayFixture(t)
	cut := fixture[:strings.Index(fixture, "data: {\"type\":\"text-start\"")]
	f, _, connects := scriptedStream(t, streamScript{bodies: []string{cut, fixture},
		state: func(int32) (string, int) { return stateSucceeded, 1 }})
	stdout, stderr, code := run(t, f.deps(), "-o", "text", "logs", "ags-live1", "--follow")
	if code != 0 || stdout != renderedReplay || connects.Load() != 2 {
		t.Fatalf("exit %d after %d connections, want the cut attached again\nstdout %s\nstderr %s", code, connects.Load(), stdout, stderr)
	}
}
