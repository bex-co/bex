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
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// chatFake is one session across two turns: turn 1's stream waits for the
// test to release it, a steer starts turn 2, whose stream ends at once.
type chatFake struct {
	*routeFake
	mu        sync.Mutex
	turn      int
	working   bool
	streaming chan struct{} // closed when turn 1's stream has started
	release   chan struct{} // closing it ends turn 1
	steered   chan string
	canceled  chan struct{}
}

func newChatFake(t *testing.T) *chatFake {
	f := &chatFake{turn: 1, working: true, streaming: make(chan struct{}), release: make(chan struct{}),
		steered: make(chan string, 4), canceled: make(chan struct{}, 1)}
	sessionView := func(id string) map[string]any {
		f.mu.Lock()
		defer f.mu.Unlock()
		state := stateWorking
		if !f.working {
			state = stateSucceeded
		}
		v := sessionJSON(id, state)
		v["steerable"], v["turns"] = !f.working, f.turn
		if !f.working {
			v["outcome"] = map[string]any{"turn": f.turn, "summary": "done", "cost": map[string]any{"totalUSD": "0.01"}}
		}
		return v
	}
	turnChunks := func(turn int, text string) string {
		return fmt.Sprintf("data: {\"type\":\"data-user-prompt\",\"data\":{\"turn\":%d,\"text\":\"prompt %d\"}}\n\n"+
			"data: {\"type\":\"text-start\",\"id\":\"t%d\"}\n\ndata: {\"type\":\"text-delta\",\"id\":\"t%d\",\"delta\":%q}\n\ndata: {\"type\":\"text-end\",\"id\":\"t%d\"}\n\n",
			turn, turn, turn, turn, text, turn)
	}
	f.routeFake = newRouteFake(t, map[string]http.HandlerFunc{
		"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, sessionView(r.PathValue("id")))
		},
		"POST /v1/agent-sessions/{id}/attach-ticket": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"ticket": "tkt", "streamUrl": f.srv.URL + "/v1/agent-sessions/" + r.PathValue("id") + "/stream"})
		},
		"GET /v1/agent-sessions/{id}/stream": func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			turn, working := f.turn, f.working
			f.mu.Unlock()
			body := turnChunks(1, "Working.")
			if turn == 1 && working {
				_, _ = io.WriteString(w, body)
				w.(http.Flusher).Flush()
				close(f.streaming)
				<-f.release
				f.mu.Lock()
				f.working = false
				f.mu.Unlock()
			}
			if turn == 2 {
				body += turnChunks(2, "Added a test.")
				f.mu.Lock()
				f.working = false
				f.mu.Unlock()
			}
			if turn != 1 || !working {
				_, _ = io.WriteString(w, body)
			}
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		},
		"POST /v1/agent-sessions/{id}/steer": func(w http.ResponseWriter, r *http.Request) {
			var body steerRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.turn, f.working = 2, true
			f.mu.Unlock()
			f.steered <- body.Prompt
			writeJSON(w, http.StatusOK, sessionView(r.PathValue("id")))
		},
		"POST /v1/agent-sessions/{id}/cancel": func(w http.ResponseWriter, r *http.Request) {
			f.canceled <- struct{}{}
			writeJSON(w, http.StatusOK, sessionJSON(r.PathValue("id"), stateCanceled))
		},
	})
	return f
}

// chatRun starts `bex agent chat` on a terminal whose input the test types.
func chatRun(t *testing.T, f *chatFake, args ...string) (*io.PipeWriter, func() (string, string, int)) {
	in, typed := io.Pipe()
	d := f.deps()
	d.interactive = func() bool { return true }
	d.stdin = in
	type result struct {
		stdout, stderr string
		code           int
	}
	done := make(chan result, 1)
	go func() {
		stdout, stderr, code := run(t, d, append([]string{"-o", "text", "chat"}, args...)...)
		done <- result{stdout, stderr, code}
	}()
	return typed, func() (string, string, int) {
		select {
		case r := <-done:
			return r.stdout, r.stderr, r.code
		case <-time.After(10 * time.Second):
			t.Fatal("chat did not end")
			return "", "", 0
		}
	}
}

func TestChatSendsAMessageTypedDuringATurnWhenTheTurnEnds(t *testing.T) {
	f := newChatFake(t)
	typed, wait := chatRun(t, f, "ags-chat1")
	<-f.streaming
	_, _ = io.WriteString(typed, "Also add a test\n")
	select {
	case prompt := <-f.steered:
		t.Fatalf("%q was sent while the turn still ran", prompt)
	case <-time.After(200 * time.Millisecond):
	}
	close(f.release)
	select {
	case prompt := <-f.steered:
		if prompt != "Also add a test" {
			t.Fatalf("steered %q", prompt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the queued message was never sent")
	}
	time.Sleep(100 * time.Millisecond) // turn 2 renders
	_, _ = io.WriteString(typed, "/quit\n")
	stdout, stderr, code := wait()
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{"ags-chat1 · claude · working", "Working.", "queued — sends when this turn ends", "── turn 1 succeeded · $0.01 ──", "Added a test."} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Count(stdout, "Working.") != 1 {
		t.Fatalf("turn 1's text was printed %d times after reattaching:\n%s", strings.Count(stdout, "Working."), stdout)
	}
	if len(f.steered) != 0 || !strings.Contains(stderr, "keeps running") {
		t.Fatalf("extra steers or no detach note: %d, %s", len(f.steered), stderr)
	}
}

func TestChatLeavingNeverCancels(t *testing.T) {
	f := newChatFake(t)
	typed, wait := chatRun(t, f, "https://dashboard.bex.co/agents/ags-chat1")
	<-f.streaming
	_ = typed.Close() // Ctrl-D
	_, stderr, code := wait()
	close(f.release)
	if code != 0 || !strings.Contains(stderr, "Left ags-chat1; it keeps running") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if len(f.canceled) != 0 {
		t.Fatal("leaving canceled the session")
	}
}

func TestChatCancelAsksFirst(t *testing.T) {
	f := newChatFake(t)
	typed, wait := chatRun(t, f, "ags-chat1")
	<-f.streaming
	_, _ = io.WriteString(typed, "/cancel\nn\n/cancel\ny\n")
	select {
	case <-f.canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("/cancel with y did not cancel")
	}
	_ = typed.Close()
	close(f.release)
	stdout, _, _ := wait()
	if !strings.Contains(stdout, "not canceled") {
		t.Fatalf("the first /cancel (n) was not declined:\n%s", stdout)
	}
}

func TestChatNeedsATerminal(t *testing.T) {
	f := newChatFake(t)
	if _, stderr, code := run(t, f.deps(), "chat", "ags-chat1"); code != exitUsage || !strings.Contains(stderr, "run --follow") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	close(f.release)
}

func TestPickerChoosesARecentSession(t *testing.T) {
	f := newRouteFake(t, map[string]http.HandlerFunc{
		"GET /v1/agent-sessions": answer(http.StatusOK, []any{sessionJSON("ags-first", stateSucceeded), sessionJSON("ags-second", stateWorking)}),
		"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, sessionJSON(r.PathValue("id"), stateSucceeded))
		},
	})
	d := f.deps()
	d.interactive = func() bool { return true }
	d.stdin = strings.NewReader("2\n")
	_, stderr, code := run(t, d, "-o", "text", "get")
	if code != 0 || !strings.Contains(stderr, "2  ags-second") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got := f.callsTo(http.MethodGet); got[len(got)-1].path != "/v1/agent-sessions/ags-second" || !strings.Contains(got[0].query, "mine=true") {
		t.Fatalf("calls %+v", got)
	}
	d.stdin = strings.NewReader("9\n")
	if _, _, code := run(t, d, "get"); code != exitUsage {
		t.Fatalf("an out-of-range pick: exit %d", code)
	}
	d.interactive = func() bool { return false }
	if _, _, code := run(t, d, "get"); code != exitUsage {
		t.Fatalf("no terminal and no session: exit %d", code)
	}
}

func TestSessionArgumentsComplete(t *testing.T) {
	f := newRouteFake(t, map[string]http.HandlerFunc{
		"GET /v1/agent-sessions": answer(http.StatusOK, []any{sessionJSON("ags-first", stateSucceeded)}),
	})
	get, _, err := newCommand(f.deps()).Find([]string{"get"})
	if err != nil {
		t.Fatal(err)
	}
	got, directive := get.ValidArgsFunction(get, nil, "")
	if len(got) != 1 || !strings.HasPrefix(got[0], "ags-first\t") || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completions %v, directive %v", got, directive)
	}
	if more, _ := get.ValidArgsFunction(get, []string{"ags-first"}, ""); len(more) != 0 {
		t.Fatalf("a second argument completed: %v", more)
	}
}
