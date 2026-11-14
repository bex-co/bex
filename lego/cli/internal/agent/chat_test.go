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
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// refusal is a scripted answer to a request, before the fake does anything.
type refusal struct {
	status        int
	code, message string
}

// chatFake is one session across turns, as bex-api serves it (ADR047 D12).
// Turn 1's stream waits for the test to release it; a later turn's ends at
// once. A message sent while a turn runs, or behind messages already waiting,
// is queued (202 with the message, honoring Idempotency-Key). When a turn ends
// the next queued message starts as a turn, after dispatchAfter, as the
// dispatcher does. A message sent to an idle session with nothing waiting
// starts a turn (200). With older set it is a server from before the queue: a
// message during a turn is refused 409 AGENT_SESSION_TURN_IN_FLIGHT.
type chatFake struct {
	*routeFake
	mu            sync.Mutex
	turn          int
	working       bool
	prompts       map[int]string
	streaming     chan struct{} // closed when turn 1's stream has started
	release       chan struct{} // closing it ends turn 1
	releaseOnce   sync.Once
	streamingOnce sync.Once
	steered       chan string // the prompt of every turn a message started
	canceled      chan struct{}

	queue        []map[string]any // pending messages, next first
	byKey        map[string]map[string]any
	nextMessage  int
	queueCap     int
	dispatchWait time.Duration
	older        bool

	sent        []steerRequest // every steer the fake received
	keys        []string       // the Idempotency-Key of each
	refusals    []refusal      // answers to the next steers
	lostAnswers int            // the next steers are processed, then answered 503
	stops       int            // interrupts accepted
	stopRefused []refusal      // answers to the next interrupts
}

func newChatFake(t *testing.T) *chatFake {
	f := &chatFake{
		turn: 1, working: true, prompts: map[int]string{},
		streaming: make(chan struct{}), release: make(chan struct{}),
		steered: make(chan string, 16), canceled: make(chan struct{}, 1),
		byKey: map[string]map[string]any{}, queueCap: 10,
	}
	turnChunks := func(turn int, text string) string {
		return fmt.Sprintf("data: {\"type\":\"data-user-prompt\",\"data\":{\"turn\":%d,\"text\":%q}}\n\n"+
			"data: {\"type\":\"text-start\",\"id\":\"t%d\"}\n\ndata: {\"type\":\"text-delta\",\"id\":\"t%d\",\"delta\":%q}\n\ndata: {\"type\":\"text-end\",\"id\":\"t%d\"}\n\n",
			turn, f.prompts[turn], turn, turn, text, turn)
	}
	f.routeFake = newRouteFake(t, map[string]http.HandlerFunc{
		"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			v := f.view(r.PathValue("id"))
			f.mu.Unlock()
			writeJSON(w, http.StatusOK, v)
		},
		"POST /v1/agent-sessions/{id}/attach-ticket": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"ticket": "tkt", "streamUrl": f.srv.URL + "/v1/agent-sessions/" + r.PathValue("id") + "/stream"})
		},
		"GET /v1/agent-sessions/{id}/stream": func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			turn, working := f.turn, f.working
			var body strings.Builder
			for n := 1; n <= turn; n++ {
				text := "Working."
				if n > 1 {
					text = "Done: " + f.prompts[n]
				}
				body.WriteString(turnChunks(n, text))
			}
			f.mu.Unlock()
			_, _ = io.WriteString(w, body.String())
			if working {
				w.(http.Flusher).Flush()
				if turn == 1 {
					f.streamingOnce.Do(func() { close(f.streaming) })
					<-f.release
				}
				f.mu.Lock()
				if f.working && f.turn == turn {
					f.finish()
				}
				f.mu.Unlock()
			}
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		},
		"POST /v1/agent-sessions/{id}/steer": func(w http.ResponseWriter, r *http.Request) {
			var body steerRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			key := r.Header.Get("Idempotency-Key")
			f.mu.Lock()
			f.sent, f.keys = append(f.sent, body), append(f.keys, key)
			if len(f.refusals) > 0 {
				ref := f.refusals[0]
				f.refusals = f.refusals[1:]
				f.mu.Unlock()
				writeAPIError(w, ref.status, ref.code, ref.message)
				return
			}
			status, v, ref := f.steer(r.PathValue("id"), body, key)
			lost := f.lostAnswers > 0
			if lost {
				f.lostAnswers--
			}
			f.mu.Unlock()
			switch {
			case ref != nil:
				writeAPIError(w, ref.status, ref.code, ref.message)
			case lost:
				writeAPIError(w, http.StatusServiceUnavailable, "", "rolling")
			default:
				writeJSON(w, status, v)
			}
		},
		"GET /v1/agent-sessions/{id}/queued-messages": func(w http.ResponseWriter, _ *http.Request) {
			f.mu.Lock()
			list := f.pending()
			f.mu.Unlock()
			writeJSON(w, http.StatusOK, map[string]any{"queuedMessages": list})
		},
		"POST /v1/agent-sessions/{id}/interrupt": func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			if len(f.stopRefused) > 0 {
				ref := f.stopRefused[0]
				f.stopRefused = f.stopRefused[1:]
				f.mu.Unlock()
				writeAPIError(w, ref.status, ref.code, ref.message)
				return
			}
			f.stops++
			v := f.view(r.PathValue("id"))
			f.mu.Unlock()
			f.releaseTurn() // the turn settles shortly after the 202
			writeJSON(w, http.StatusAccepted, v)
		},
		"POST /v1/agent-sessions/{id}/cancel": func(w http.ResponseWriter, r *http.Request) {
			f.canceled <- struct{}{}
			writeJSON(w, http.StatusOK, sessionJSON(r.PathValue("id"), stateCanceled))
		},
	})
	t.Cleanup(f.releaseTurn)
	return f
}

// releaseTurn ends turn 1.
func (f *chatFake) releaseTurn() { f.releaseOnce.Do(func() { close(f.release) }) }

// view is the session as a single-session read serves it; f.mu is held.
func (f *chatFake) view(id string) map[string]any {
	state := stateWorking
	if !f.working {
		state = stateSucceeded
	}
	v := sessionJSON(id, state)
	v["steerable"], v["turns"] = !f.working, f.turn
	if !f.working {
		v["outcome"] = map[string]any{"turn": f.turn, "summary": "done", "cost": map[string]any{"totalUSD": "0.01"}}
	}
	if len(f.queue) > 0 {
		v["pendingMessages"], v["queuedMessages"] = len(f.queue), f.pending()
	}
	return v
}

// pending is a copy of the queue, safe to encode while the fake moves on.
func (f *chatFake) pending() []map[string]any {
	out := make([]map[string]any, 0, len(f.queue))
	for _, m := range f.queue {
		out = append(out, maps.Clone(m))
	}
	return out
}

// steer is the steer route; f.mu is held. A non-nil refusal is its answer.
func (f *chatFake) steer(id string, body steerRequest, key string) (int, map[string]any, *refusal) {
	switch {
	case f.older && f.working:
		return 0, nil, &refusal{http.StatusConflict, codeSessionTurnInFlight, "a turn is already running"}
	case !f.older && (f.working || len(f.queue) > 0):
		message, replay := f.byKey[key]
		if !replay || key == "" {
			if len(f.queue) >= f.queueCap {
				return 0, nil, &refusal{http.StatusConflict, codeQueueFull, "the session already has the most messages it can queue; wait for one to run or delete one"}
			}
			message = f.enqueue(id, body.Prompt)
			if key != "" {
				f.byKey[key] = message
			}
		}
		v := f.view(id)
		v["queuedMessage"] = maps.Clone(message)
		return http.StatusAccepted, v, nil
	}
	f.start(body.Prompt)
	return http.StatusOK, f.view(id), nil
}

func (f *chatFake) enqueue(id, prompt string) map[string]any {
	f.nextMessage++
	message := map[string]any{"id": fmt.Sprintf("qm-%d", f.nextMessage), "sessionId": id, "position": len(f.queue) + 1, "prompt": prompt, "state": "pending"}
	f.queue = append(f.queue, message)
	return message
}

func (f *chatFake) start(prompt string) {
	f.turn++
	f.working = true
	f.prompts[f.turn] = prompt
	f.steered <- prompt
}

// finish ends the running turn; f.mu is held. The next queued message starts
// as a turn right away, or after dispatchWait.
func (f *chatFake) finish() {
	f.working = false
	switch {
	case len(f.queue) == 0:
	case f.dispatchWait == 0:
		f.dispatch()
	default:
		time.AfterFunc(f.dispatchWait, func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.dispatch()
		})
	}
}

func (f *chatFake) dispatch() {
	if f.working || len(f.queue) == 0 {
		return
	}
	head := f.queue[0]
	f.queue = f.queue[1:]
	for i := range f.queue {
		f.queue[i]["position"] = i + 1
	}
	f.start(head["prompt"].(string))
}

// waitQueued waits until n messages are queued on the server.
func (f *chatFake) waitQueued(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		f.mu.Lock()
		got := len(f.queue)
		f.mu.Unlock()
		if got == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d messages queued, want %d", got, n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitIdle waits until turn has ended and nothing runs.
func (f *chatFake) waitIdle(t *testing.T, turn int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		f.mu.Lock()
		done := f.turn == turn && !f.working
		f.mu.Unlock()
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("turn %d did not end", turn)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// started is the next prompt a turn started with.
func (f *chatFake) started(t *testing.T) string {
	t.Helper()
	select {
	case prompt := <-f.steered:
		return prompt
	case <-time.After(5 * time.Second):
		t.Fatal("no turn started")
		return ""
	}
}

func (f *chatFake) queuedPrompts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, m := range f.queue {
		out = append(out, m["prompt"].(string))
	}
	return out
}

// chatDeps is a terminal whose input the test types, minting a key per call.
func chatDeps(f *chatFake, in io.Reader) deps {
	d := f.deps()
	d.interactive = func() bool { return true }
	d.stdin = in
	var (
		mu sync.Mutex
		n  int
	)
	d.newKey = func() string {
		mu.Lock()
		defer mu.Unlock()
		n++
		return fmt.Sprintf("bex-cli-line%d", n)
	}
	return d
}

// chatRun starts `bex agent chat` on a terminal whose input the test types.
func chatRun(t *testing.T, f *chatFake, args ...string) (*io.PipeWriter, func() (string, string, int)) {
	in, typed := io.Pipe()
	d := chatDeps(f, in)
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

func typeLines(t *testing.T, typed io.Writer, lines ...string) {
	t.Helper()
	for _, line := range lines {
		if _, err := io.WriteString(typed, line+"\n"); err != nil {
			t.Fatal(err)
		}
	}
}

func mustContain(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("output lacks %q:\n%s", want, got)
		}
	}
}

// ADR047 D12: a message typed while a turn runs goes to the server's queue at
// once, typing again adds another, and the server runs them in order when the
// turn ends: the chat keeps showing the turns it did not start.
func TestChatQueuesMessagesTypedDuringATurnOnTheServer(t *testing.T) {
	f := newChatFake(t)
	typed, wait := chatRun(t, f, "ags-chat1")
	<-f.streaming
	typeLines(t, typed, "Also add a test", "Update the docs")
	f.waitQueued(t, 2)
	if got := f.queuedPrompts(); !slices.Equal(got, []string{"Also add a test", "Update the docs"}) {
		t.Fatalf("the server queue holds %q: the second message replaced the first", got)
	}
	select {
	case prompt := <-f.steered:
		t.Fatalf("%q started a turn while turn 1 still ran", prompt)
	case <-time.After(100 * time.Millisecond):
	}
	f.mu.Lock()
	keys := slices.Clone(f.keys)
	f.mu.Unlock()
	if len(keys) != 2 || keys[0] == "" || keys[0] == keys[1] {
		t.Fatalf("idempotency keys %q: want one per typed line", keys)
	}

	f.releaseTurn()
	if first, second := f.started(t), f.started(t); first != "Also add a test" || second != "Update the docs" {
		t.Fatalf("the server ran %q then %q", first, second)
	}
	f.waitIdle(t, 3)
	time.Sleep(200 * time.Millisecond) // turn 3 renders
	typeLines(t, typed, "/quit")
	stdout, stderr, code := wait()
	if code != 0 || !strings.Contains(stderr, "Left ags-chat1; it keeps running") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	mustContain(t, stdout, "ags-chat1 · claude · working", "Working.",
		"queued — sends when this turn ends (position 1)", "queued — sends when this turn ends (position 2)",
		"Done: Also add a test", "Done: Update the docs", "── turn 3 succeeded · $0.01 ──")
	if strings.Count(stdout, "Working.") != 1 || strings.Index(stdout, "Done: Also add a test") > strings.Index(stdout, "Done: Update the docs") {
		t.Fatalf("turns printed twice or out of order:\n%s", stdout)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) != 2 {
		t.Fatalf("the chat sent %d messages, want 2 (the server runs the queue)", len(f.sent))
	}
}

// Between turns the chat waits for the turn the server starts from its queue
// instead of sitting at the prompt, and shows it without being asked.
func TestChatFollowsATurnTheServerStartsFromItsQueue(t *testing.T) {
	f := newChatFake(t)
	f.dispatchWait = 100 * time.Millisecond
	typed, wait := chatRun(t, f, "ags-chat1")
	<-f.streaming
	typeLines(t, typed, "Also add a test")
	f.waitQueued(t, 1)
	f.releaseTurn()
	if prompt := f.started(t); prompt != "Also add a test" {
		t.Fatalf("started %q", prompt)
	}
	f.waitIdle(t, 2)
	time.Sleep(200 * time.Millisecond)
	typeLines(t, typed, "/quit")
	stdout, _, code := wait()
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, stdout, "── turn 1 succeeded · $0.01 ──", "Done: Also add a test", "── turn 2 succeeded · $0.01 ──")
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) != 1 {
		t.Fatalf("%d sends: the chat sent more than the one typed message", len(f.sent))
	}
}

// A message typed when no turn runs and nothing waits is sent at once and
// starts a turn.
func TestChatSendsAMessageTypedBetweenTurnsAtOnce(t *testing.T) {
	f := newChatFake(t)
	typed, wait := chatRun(t, f, "ags-chat1")
	<-f.streaming
	f.releaseTurn()
	f.waitIdle(t, 1)
	time.Sleep(200 * time.Millisecond) // the chat is at its prompt
	typeLines(t, typed, "Add a test")
	if prompt := f.started(t); prompt != "Add a test" {
		t.Fatalf("started %q", prompt)
	}
	f.waitIdle(t, 2)
	time.Sleep(200 * time.Millisecond)
	typeLines(t, typed, "/quit")
	stdout, _, _ := wait()
	mustContain(t, stdout, "Done: Add a test")
	if strings.Contains(stdout, "queued") {
		t.Fatalf("a message sent at once was reported queued:\n%s", stdout)
	}
}

// A send whose answer is lost is reported, not repeated: the server records no
// key for a send that started a turn, so a repeat could run the message twice.
// The chat goes on, and the next line gets a key of its own.
func TestChatReportsALostSendAndDoesNotRepeatIt(t *testing.T) {
	f := newChatFake(t)
	f.lostAnswers = 1
	typed, wait := chatRun(t, f, "ags-chat1")
	<-f.streaming
	typeLines(t, typed, "Also add a test", "Update the docs")
	f.waitQueued(t, 2)
	_ = typed.Close()
	stdout, _, _ := wait()
	f.mu.Lock()
	keys := slices.Clone(f.keys)
	f.mu.Unlock()
	if len(keys) != 2 || keys[0] == keys[1] {
		t.Fatalf("keys %q: want one attempt per line, each under its own key", keys)
	}
	// The first line reached the server before its answer was lost; it is held
	// once, not once per attempt.
	if got := f.queuedPrompts(); !slices.Equal(got, []string{"Also add a test", "Update the docs"}) {
		t.Fatalf("the server queue holds %q, want each line once", got)
	}
	mustContain(t, stdout, "not sent: 503", "queued — sends when this turn ends (position 2)")
	if strings.Contains(stdout, "(position 1)") {
		t.Fatalf("the lost send was reported queued:\n%s", stdout)
	}
}

// An older server refuses a message during a turn: the chat holds it, then
// sends each held message in order as the turns end, one key per line.
func TestChatHoldsMessagesAnOlderServerRefuses(t *testing.T) {
	f := newChatFake(t)
	f.older = true
	typed, wait := chatRun(t, f, "ags-chat1")
	<-f.streaming
	typeLines(t, typed, "Also add a test", "Update the docs")
	deadline := time.Now().Add(5 * time.Second)
	for {
		f.mu.Lock()
		n := len(f.sent)
		f.mu.Unlock()
		if n == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the chat sent %d messages, want 2 attempts", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
	select {
	case prompt := <-f.steered:
		t.Fatalf("%q was sent while the turn still ran", prompt)
	case <-time.After(100 * time.Millisecond):
	}

	f.releaseTurn()
	if first, second := f.started(t), f.started(t); first != "Also add a test" || second != "Update the docs" {
		t.Fatalf("sent %q then %q", first, second)
	}
	f.waitIdle(t, 3)
	time.Sleep(200 * time.Millisecond)
	typeLines(t, typed, "/quit")
	stdout, _, code := wait()
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, stdout, "queued — sends when this turn ends", "Done: Also add a test", "Done: Update the docs")
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := f.keys
	// Both lines were refused during turn 1, then each was sent once more,
	// the second after the first's turn: a line's key never changes.
	if len(keys) != 4 || keys[0] != keys[2] || keys[1] != keys[3] || keys[0] == keys[1] {
		t.Fatalf("keys %q: a held line must be resent under its own key", keys)
	}
}

// What the server refuses is said in words the user can act on, and the chat
// goes on.
func TestChatSaysWhyAMessageWasNotSent(t *testing.T) {
	for _, tc := range []struct {
		name string
		ref  refusal
		want []string
	}{
		{"queue full", refusal{http.StatusConflict, codeQueueFull, "the session already has the most messages it can queue; wait for one to run or delete one"},
			[]string{"not sent: 409 AGENT_SESSION_QUEUE_FULL", "the session's queue is full; wait for a queued message to run, then send again"}},
		{"inline secrets", refusal{http.StatusConflict, codeQueueInlineSecrets, "a message that carries inline secrets cannot wait in the queue; name workspace secrets in secretNames"},
			[]string{"not sent: 409 AGENT_SESSION_QUEUE_INLINE_SECRETS", "name workspace secrets with --secret instead of --secret-from-env"}},
		{"archived", refusal{http.StatusConflict, "AGENT_SESSION_ARCHIVED", "the session is archived"},
			[]string{"not sent: 409 AGENT_SESSION_ARCHIVED: the session is archived"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newChatFake(t)
			f.refusals = []refusal{tc.ref}
			typed, wait := chatRun(t, f, "ags-chat1")
			<-f.streaming
			typeLines(t, typed, "Also add a test", "Update the docs") // the chat is still there for the second
			f.waitQueued(t, 1)
			_ = typed.Close()
			stdout, _, code := wait()
			if code != 0 {
				t.Fatalf("exit %d: a refusal must not end the chat", code)
			}
			mustContain(t, stdout, tc.want...)
			if got := f.queuedPrompts(); !slices.Equal(got, []string{"Update the docs"}) {
				t.Fatalf("queue %q", got)
			}
		})
	}
}

// ADR047 D12: a reattached chat lists what is still queued, next first.
func TestChatListsTheQueuedMessagesOnAttach(t *testing.T) {
	f := newChatFake(t)
	f.enqueue("ags-chat1", "Add a test")
	f.enqueue("ags-chat1", "Update the docs\nand the changelog")
	f.queue[1]["blockedReason"] = "the workspace is at its live sandbox limit"
	typed, wait := chatRun(t, f, "ags-chat1")
	<-f.streaming
	_ = typed.Close()
	stdout, _, code := wait()
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, stdout, "2 messages queued, next first:", "  1. Add a test\n",
		"  2. Update the docs (waiting: the workspace is at its live sandbox limit)\n")
	if got := f.callsTo(http.MethodGet); !slices.ContainsFunc(got, func(c capturedCall) bool { return c.path == "/v1/agent-sessions/ags-chat1/queued-messages" }) {
		t.Fatalf("the queue was not read: %+v", got)
	}
}

// A session with nothing queued costs no extra read and prints no list.
func TestChatPrintsNoQueueWhenNothingIsQueued(t *testing.T) {
	f := newChatFake(t)
	typed, wait := chatRun(t, f, "ags-chat1")
	<-f.streaming
	_ = typed.Close()
	stdout, _, _ := wait()
	if strings.Contains(stdout, "queued") {
		t.Fatalf("stdout mentions a queue:\n%s", stdout)
	}
	for _, c := range f.callsTo(http.MethodGet) {
		if strings.HasSuffix(c.path, "/queued-messages") {
			t.Fatalf("read the queue of a session that has none: %+v", c)
		}
	}
}

// Detaching (Ctrl-D, or Ctrl-C ending the context) leaves the queue where it
// is: the session keeps running and the server still runs what was queued.
func TestChatLeavingKeepsTheQueue(t *testing.T) {
	t.Run("Ctrl-D", func(t *testing.T) {
		f := newChatFake(t)
		typed, wait := chatRun(t, f, "ags-chat1")
		<-f.streaming
		typeLines(t, typed, "Also add a test")
		f.waitQueued(t, 1)
		_ = typed.Close()
		_, stderr, code := wait()
		if code != 0 || !strings.Contains(stderr, "Left ags-chat1; it keeps running") {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		assertQueueKept(t, f)
	})
	t.Run("Ctrl-C", func(t *testing.T) {
		f := newChatFake(t)
		in, typed := io.Pipe()
		d := chatDeps(f, in)
		client, err := d.api()
		if err != nil {
			t.Fatal(err)
		}
		ctx, interrupt := context.WithCancel(context.Background())
		defer interrupt()
		var stdout, stderr bytes.Buffer
		a := workspaceAPI{&invocation{ctx: ctx, api: client, out: &stdout, errOut: &stderr}, "tea-ws"}
		done := make(chan error, 1)
		go func() { done <- d.chat(a, "ags-chat1", chatOptions{}) }()
		<-f.streaming
		typeLines(t, typed, "Also add a test")
		f.waitQueued(t, 1)
		interrupt()
		select {
		case err := <-done:
			if err != nil || !strings.Contains(stderr.String(), "Left ags-chat1; it keeps running") {
				t.Fatalf("chat returned %v: %s", err, stderr.String())
			}
		case <-time.After(5 * time.Second):
			t.Fatal("chat did not end")
		}
		assertQueueKept(t, f)
	})
}

func assertQueueKept(t *testing.T, f *chatFake) {
	t.Helper()
	if got := f.queuedPrompts(); !slices.Equal(got, []string{"Also add a test"}) {
		t.Fatalf("the queue holds %q after leaving", got)
	}
	if len(f.canceled) != 0 {
		t.Fatal("leaving canceled the session")
	}
	if len(f.callsTo(http.MethodDelete)) != 0 {
		t.Fatal("leaving deleted a queued message")
	}
	for _, c := range f.callsTo(http.MethodPost) {
		if strings.HasSuffix(c.path, "/cancel") || strings.HasSuffix(c.path, "/interrupt") {
			t.Fatalf("leaving called POST %s", c.path)
		}
	}
}

// /stop ends the running turn only, asks nothing, and says so; the two
// refusals are explained.
func TestChatStopEndsOnlyTheTurn(t *testing.T) {
	t.Run("stops", func(t *testing.T) {
		f := newChatFake(t)
		typed, wait := chatRun(t, f, "ags-chat1")
		<-f.streaming
		typeLines(t, typed, "/stop") // no confirmation is asked
		f.waitIdle(t, 1)
		time.Sleep(200 * time.Millisecond)
		typeLines(t, typed, "/quit")
		stdout, stderr, code := wait()
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		mustContain(t, stdout, "Stopping the current turn. The session stays open and any queued messages run next; /cancel ends it.")
		if strings.Contains(stdout, "[y/N]") {
			t.Fatalf("/stop asked for a confirmation:\n%s", stdout)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.stops != 1 || len(f.canceled) != 0 {
			t.Fatalf("%d stops, %d cancels", f.stops, len(f.canceled))
		}
	})
	for _, tc := range []struct {
		name string
		ref  refusal
		want string
	}{
		{"no turn running", refusal{http.StatusConflict, "AGENT_SESSION_NO_TURN_RUNNING", "no turn is running; the session is idle"},
			"stop: no turn is running, so there is nothing to stop"},
		{"turn starting", refusal{http.StatusConflict, "AGENT_SESSION_TURN_STARTING", "the turn is still starting; stop it once it is running"},
			"stop: the turn is still starting; try again once the agent is working"},
		{"other", refusal{http.StatusForbidden, "", "forbidden"}, "stop: 403 forbidden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newChatFake(t)
			f.stopRefused = []refusal{tc.ref}
			typed, wait := chatRun(t, f, "ags-chat1")
			<-f.streaming
			typeLines(t, typed, "/stop")
			f.releaseTurn()
			f.waitIdle(t, 1)
			time.Sleep(100 * time.Millisecond)
			_ = typed.Close()
			stdout, _, code := wait()
			if code != 0 {
				t.Fatalf("exit %d: a refused /stop must not end the chat", code)
			}
			mustContain(t, stdout, tc.want)
			if strings.Contains(stdout, "Stopping") {
				t.Fatalf("a refused /stop said it was stopping:\n%s", stdout)
			}
		})
	}
}

// /attach and /skill stage what goes with the next message, which is sent at
// once; the line after it carries nothing staged.
func TestChatSendsStagedSkillsWithTheNextMessage(t *testing.T) {
	f := newChatFake(t)
	typed, wait := chatRun(t, f, "ags-chat1")
	<-f.streaming
	typeLines(t, typed, "/skill lint:strict", "Also add a test", "Update the docs")
	f.waitQueued(t, 2)
	_ = typed.Close()
	_, _, _ = wait()
	f.mu.Lock()
	defer f.mu.Unlock()
	if !slices.Equal(f.sent[0].Skills, []skillInvocation{{Name: "lint", Arguments: "strict"}}) || len(f.sent[1].Skills) != 0 {
		t.Fatalf("skills sent: %+v then %+v", f.sent[0].Skills, f.sent[1].Skills)
	}
}

func TestChatHelpListsTheQueueAndStop(t *testing.T) {
	for _, want := range []string{"is queued on", "type more to queue more", "/stop", "/cancel", "after a confirmation"} {
		if !strings.Contains(chatHelp, want) {
			t.Errorf("/help lacks %q:\n%s", want, chatHelp)
		}
	}
}

func TestChatLeavingNeverCancels(t *testing.T) {
	f := newChatFake(t)
	typed, wait := chatRun(t, f, "https://dashboard.bex.co/agents/ags-chat1")
	<-f.streaming
	_ = typed.Close() // Ctrl-D
	_, stderr, code := wait()
	f.releaseTurn()
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
	f.releaseTurn()
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
