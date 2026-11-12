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
	return streamFakeWith(t, ticketStatus, false)
}

// streamFakeWith's first connection ends early: cut mid-turn, or (earlyDone)
// closed with [DONE] after only the prompt, as the gateway does while the
// turn's new sandbox does not serve yet.
func streamFakeWith(t *testing.T, ticketStatus []int, earlyDone bool) (*routeFake, *atomic.Int32, *atomic.Int32) {
	var mints, connects atomic.Int32
	var f *routeFake
	f = newRouteFake(t, map[string]http.HandlerFunc{
		"POST /v1/agent-sessions/{id}/attach-ticket": func(w http.ResponseWriter, r *http.Request) {
			n := int(mints.Add(1))
			if r.URL.Query().Get("action") != "read" {
				t.Errorf("attach-ticket action %q, want read", r.URL.Query().Get("action"))
			}
			if n <= len(ticketStatus) {
				writeAPIError(w, ticketStatus[n-1], "AGENT_SESSION_NOT_ATTACHABLE", "no sandbox yet")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "ticket": "tkt-" + string(rune('a'+n)),
				"streamUrl": f.srv.URL + "/v1/agent-sessions/" + r.PathValue("id") + "/stream"})
		},
		"GET /v1/agent-sessions/{id}/stream": func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(ticketHeader) == "" || r.Header.Get("Authorization") != "" {
				t.Errorf("stream request headers: ticket %q, authorization %q", r.Header.Get(ticketHeader), r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fixture := replayFixture(t)
			if connects.Add(1) == 1 {
				if earlyDone {
					_, _ = w.Write([]byte(fixture[:strings.Index(fixture, "data: {\"type\":\"reasoning-start\"")] + "data: [DONE]\n\n"))
					return
				}
				_, _ = w.Write([]byte(fixture[:strings.Index(fixture, "data: {\"type\":\"text-start\"")]))
				return
			}
			_, _ = w.Write([]byte(fixture))
		},
		"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
			state := stateWorking
			if connects.Load() >= 2 {
				state = stateSucceeded
			}
			writeJSON(w, http.StatusOK, sessionJSON(r.PathValue("id"), state))
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

// TestFollowReattachesAfterAnEarlyDone: right after a create or steer the
// gateway can answer before the turn's sandbox serves, replaying and closing
// with [DONE]. That is not the end of the turn; follow must attach again and
// show the turn.
func TestFollowReattachesAfterAnEarlyDone(t *testing.T) {
	f, _, connects := streamFakeWith(t, nil, true)
	stdout, stderr, code := run(t, f.deps(), "-o", "text", "logs", "ags-live1", "--follow")
	if code != 0 || stdout != renderedReplay || connects.Load() != 2 {
		t.Fatalf("exit %d after %d connections\nstdout %s\nstderr %s", code, connects.Load(), stdout, stderr)
	}
}
