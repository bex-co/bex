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
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR047 D12 against the real store: a message sent while a turn runs is
// queued, managed, and delivered as its own turn when the turn settles, with
// the in-memory authorization and sandbox fakes the other session tests use.

type queueEnv struct {
	lineageEnv
	roles map[string]string // subject -> "contrib" (operate only) or "viewer" (view only)
	id    string            // a session whose first turn is running
}

func newQueueEnv(t *testing.T) queueEnv {
	t.Helper()
	e := queueEnv{lineageEnv: newLineageEnv(t), roles: map[string]string{}}
	e.svc.Queue = e.pg
	for _, subject := range []string{"contrib", "viewer", "bob"} {
		e.fga.members[subject] = e.ws
	}
	e.svc.Authz = checkerFunc(func(ctx context.Context, subject, relation, object string) (bool, error) {
		switch e.roles[strings.TrimPrefix(subject, "user:")] {
		case "contrib":
			if relation == core.RelCanCreate {
				return false, nil
			}
		case "viewer":
			if relation == core.RelCanOperate || relation == core.RelCanCreate {
				return false, nil
			}
		}
		return e.fga.Check(ctx, subject, relation, object)
	})
	e.roles["contrib"], e.roles["viewer"] = "contrib", "viewer"
	e.id = e.create(t, "").ID
	if got := e.phase(t, e.id); got != PhaseRunning {
		t.Fatalf("the session's first turn is %s, want running", got)
	}
	return e
}

// settle ends the session's running turn as the Completer does for one that
// finished on its own.
func (e queueEnv) settle(t *testing.T, id string) {
	t.Helper()
	if _, _, err := e.pg.FinalizeAgentSession(context.Background(), id, PhaseCompleted, "", "", ""); err != nil {
		t.Fatal(err)
	}
}

func (e queueEnv) send(t *testing.T, as, prompt string) View {
	t.Helper()
	view, err := e.svc.Steer(caller(as), SteerRequest{SessionID: e.id, Prompt: prompt})
	if err != nil {
		t.Fatalf("send %q: %v", prompt, err)
	}
	return view
}

func (e queueEnv) pending(t *testing.T) []string {
	t.Helper()
	list, err := e.svc.QueuedMessages(caller("alice"), e.id)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(list.QueuedMessages))
	for _, m := range list.QueuedMessages {
		out = append(out, m.Prompt)
	}
	return out
}

func (e queueEnv) prompts(t *testing.T, id string) []string {
	t.Helper()
	turns, err := e.pg.AgentSessionTurns(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(turns))
	for _, turn := range turns {
		out = append(out, turn.Prompt)
	}
	return out
}

func (e queueEnv) dispatcher() *MessageQueue {
	return &MessageQueue{Sessions: e.svc, Store: e.pg}
}

func TestSteerDuringATurnQueuesInsteadOfRefusing(t *testing.T) {
	e := newQueueEnv(t)
	for _, phase := range []string{PhaseCreating, PhaseRunning, PhaseResuming, PhaseRedispatching} {
		e.setPhase(t, e.id, phase)
		view, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "while " + phase})
		if isCode(err, codeTurnInFlight) {
			t.Fatalf("a steer in %s was refused as in flight", phase)
		}
		if err != nil || view.Queued == nil || view.Queued.State != QueuedPending || view.Queued.Prompt != "while "+phase {
			t.Fatalf("steer in %s = %+v, %v; want the queued message", phase, view.Queued, err)
		}
		if view.Phase != phase || view.Turns != 1 {
			t.Fatalf("a queued steer changed the session: phase=%s turns=%d", view.Phase, view.Turns)
		}
	}
	// They wait in order, each with its place and its author.
	list, err := e.svc.QueuedMessages(caller("alice"), e.id)
	if err != nil || len(list.QueuedMessages) != 4 {
		t.Fatalf("queue = %+v, %v", list, err)
	}
	for i, m := range list.QueuedMessages {
		if m.Position != i+1 || m.Author != "alice" || m.SessionID != e.id || !strings.HasPrefix(m.ID, "aqm-") {
			t.Fatalf("message %d = %+v", i, m)
		}
	}
	if got := e.prompts(t, e.id); len(got) != 1 {
		t.Fatalf("a queued message started a turn: %v", got)
	}
	// The single read and the list both say how many wait. The single read
	// counts them and leaves the messages to the surface that returns them
	// (TestQueuedMessagesAreListedOnlyWhereASurfaceReturnsThem).
	got, err := e.svc.Get(caller("alice"), e.id)
	if err != nil || got.PendingMessages != 4 || got.QueuedMessages != nil {
		t.Fatalf("get = pending %d, queued %d, %v", got.PendingMessages, len(got.QueuedMessages), err)
	}
	if listed := got.queued(caller("alice")); len(listed) != 4 {
		t.Fatalf("the read's messages = %d, want 4", len(listed))
	}
	listed, err := e.svc.List(caller("alice"), ListRequest{OwnerID: e.ws})
	if err != nil || len(listed) == 0 {
		t.Fatalf("list = %v, %v", listed, err)
	}
	for _, v := range listed {
		if v.ID == e.id && v.PendingMessages != 4 {
			t.Fatalf("listed pendingMessages = %d, want 4", v.PendingMessages)
		}
	}
	// The same send is idempotent and the first answer's shape is stable.
	a, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "once", IdempotencyKey: "retry-1"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "once", IdempotencyKey: "retry-1"})
	if err != nil || a.Queued.ID != b.Queued.ID {
		t.Fatalf("a retried key queued twice: %v %v", a.Queued, b.Queued)
	}
	if len(e.pending(t)) != 5 {
		t.Fatalf("the retry queued again: %v", e.pending(t))
	}
	if _, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "other", IdempotencyKey: "retry-1"}); !isCode(err, codeIdempotencyKeyReused) {
		t.Fatalf("a key reused for another message = %v", err)
	}
	if _, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "bad", IdempotencyKey: "has space"}); !isCode(err, "AGENT_SESSION_IDEMPOTENCY_KEY_INVALID") {
		t.Fatalf("an invalid key = %v", err)
	}
}

// countingQueue counts the reads of the queue a single-session read makes.
type countingQueue struct {
	QueueStore
	mu            sync.Mutex
	lists, counts int
}

func (c *countingQueue) ListQueuedAgentMessages(ctx context.Context, sessionID string) ([]store.QueuedAgentMessage, error) {
	c.mu.Lock()
	c.lists++
	c.mu.Unlock()
	return c.QueueStore.ListQueuedAgentMessages(ctx, sessionID)
}

func (c *countingQueue) CountQueuedAgentMessages(ctx context.Context, sessionID string) (int, error) {
	c.mu.Lock()
	c.counts++
	c.mu.Unlock()
	return c.QueueStore.CountQueuedAgentMessages(ctx, sessionID)
}

func (c *countingQueue) reset() (lists, counts int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	lists, counts = c.lists, c.counts
	c.lists, c.counts = 0, 0
	return lists, counts
}

// A prompt can be 100 KB, so a read of a session lists the queue's rows only
// where its surface returns them: Get counts, GraphQL lists when queuedMessages
// is selected, REST and MCP inline them.
func TestQueuedMessagesAreListedOnlyWhereASurfaceReturnsThem(t *testing.T) {
	a, e := newQueueAdapters(t)
	e.send(t, "alice", "first")
	e.send(t, "alice", "second")
	counting := &countingQueue{QueueStore: e.pg}
	e.svc.Queue = counting
	counting.reset()

	got, err := e.svc.Get(caller("alice"), e.id)
	if lists, counts := counting.reset(); err != nil || lists != 0 || counts != 1 || got.PendingMessages != 2 {
		t.Fatalf("Get listed %d times and counted %d, pending %d, %v; want one count and no list", lists, counts, got.PendingMessages, err)
	}

	data, code := a.gql(`{ agentSession(id: "`+e.id+`") { pendingMessages } }`, "alice")
	if lists, _ := counting.reset(); code != "" || lists != 0 || data["agentSession"].(map[string]any)["pendingMessages"] != 2 {
		t.Fatalf("a GraphQL read that did not select queuedMessages listed %d times: %v %s", lists, data, code)
	}
	data, code = a.gql(`{ agentSession(id: "`+e.id+`") { queuedMessages { prompt } } }`, "alice")
	if lists, _ := counting.reset(); code != "" || lists != 1 || !slices.Equal(promptsOf(data["agentSession"].(map[string]any)["queuedMessages"]), []string{"first", "second"}) {
		t.Fatalf("a GraphQL read that selected queuedMessages listed %d times: %v %s", lists, data, code)
	}

	status, body := a.rest(http.MethodGet, "/v1/agent-sessions/"+e.id, nil, "alice")
	if lists, _ := counting.reset(); status != http.StatusOK || lists != 1 || !slices.Equal(promptsOf(body["queuedMessages"]), []string{"first", "second"}) || body["pendingMessages"] != float64(2) {
		t.Fatalf("the REST read listed %d times: %d %v; want the messages inlined", lists, status, body)
	}
	out, code := a.mcp("get_agent_session", map[string]any{"id": e.id}, "alice")
	if lists, _ := counting.reset(); code != "" || lists != 1 || !slices.Equal(promptsOf(out["queuedMessages"]), []string{"first", "second"}) {
		t.Fatalf("the MCP read listed %d times: %v %s; want the messages inlined", lists, out, code)
	}
}

func TestQueueCapAndInlineSecrets(t *testing.T) {
	e := newQueueEnv(t)
	for i := 1; i <= maxQueuedMessages; i++ {
		e.send(t, "alice", fmt.Sprintf("m%d", i))
	}
	_, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "one too many"})
	if !isCode(err, codeQueueFull) || !errors.Is(err, core.ErrConflict) {
		t.Fatalf("an eleventh message = %v, want a coded 409 %s", err, codeQueueFull)
	}
	// An inline secret's value is never stored in the queue.
	if _, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "with a key",
		SessionSecretsInput: SessionSecretsInput{Secrets: []SecretInput{{Name: "API_KEY", Value: "sk-live-123"}}}}); !isCode(err, codeQueueInlineSecrets) {
		t.Fatalf("a queued message with an inline secret = %v", err)
	}
	var stored int
	if err := e.pg.Pool.QueryRow(context.Background(), `SELECT count(*) FROM agent_session_queued_messages WHERE session_id=$1 AND (prompt LIKE '%sk-live%' OR request::text LIKE '%sk-live%')`, e.id).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("an inline secret reached the queue table: %d %v", stored, err)
	}
}

func TestSteerOnAnIdleSessionQueuesBehindPendingMessages(t *testing.T) {
	e := newQueueEnv(t)
	queued := e.send(t, "alice", "first in line").Queued
	e.settle(t, e.id)
	// Idle with a message waiting: a direct send cannot jump the queue.
	view := e.send(t, "alice", "second in line")
	if view.Queued == nil || view.Queued.Position != 2 || view.Turns != 1 || view.Phase != PhaseCompleted {
		t.Fatalf("a send on an idle session with pending messages = %+v phase=%s turns=%d", view.Queued, view.Phase, view.Turns)
	}
	// Once the queue is empty an idle session starts the turn at once, as before.
	for _, m := range []string{queued.ID, view.Queued.ID} {
		if err := e.svc.DeleteQueuedMessage(caller("alice"), e.id, m); err != nil {
			t.Fatal(err)
		}
	}
	started, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "straight away"})
	if err != nil || started.Queued != nil || started.Turns != 2 {
		t.Fatalf("a send on an idle session with an empty queue = %+v, %v", started, err)
	}
}

func TestQueueManagementRespectsTheRoleLadder(t *testing.T) {
	e := newQueueEnv(t)
	first := e.send(t, "alice", "one").Queued
	second := e.send(t, "alice", "two").Queued

	// A viewer reads nothing and changes nothing.
	for name, run := range map[string]func() error{
		"list":      func() error { _, err := e.svc.QueuedMessages(caller("viewer"), e.id); return err },
		"edit":      func() error { _, err := e.svc.EditQueuedMessage(caller("viewer"), e.id, first.ID, "x"); return err },
		"delete":    func() error { return e.svc.DeleteQueuedMessage(caller("viewer"), e.id, first.ID) },
		"move":      func() error { _, err := e.svc.MoveQueuedMessage(caller("viewer"), e.id, second.ID, 1); return err },
		"interrupt": func() error { _, err := e.svc.Interrupt(caller("viewer"), e.id); return err },
		"send": func() error {
			_, err := e.svc.Steer(caller("viewer"), SteerRequest{SessionID: e.id, Prompt: "x"})
			return err
		},
	} {
		if err := run(); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("a viewer's %s = %v, want forbidden", name, err)
		}
	}
	// Another workspace's member reaches nothing of this one.
	e.fga.members["mallory"] = "tea-elsewhere"
	if _, err := e.svc.QueuedMessages(caller("mallory"), e.id); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("a foreign member's list = %v", err)
	}
	if len(e.pending(t)) != 2 || e.lc.interrupts != nil {
		t.Fatalf("a refused call changed something: %v %v", e.pending(t), e.lc.interrupts)
	}

	// A contributor (can_operate, not can_create) reads, moves and deletes but
	// cannot author the text a turn runs as someone: no edit, no send.
	if list, err := e.svc.QueuedMessages(caller("contrib"), e.id); err != nil || len(list.QueuedMessages) != 2 {
		t.Fatalf("a contributor's list = %+v, %v", list, err)
	}
	if _, err := e.svc.EditQueuedMessage(caller("contrib"), e.id, first.ID, "x"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a contributor's edit = %v", err)
	}
	moved, err := e.svc.MoveQueuedMessage(caller("contrib"), e.id, second.ID, 1)
	if err != nil || moved.QueuedMessages[0].Prompt != "two" || moved.QueuedMessages[1].Prompt != "one" {
		t.Fatalf("a contributor's move = %+v, %v", moved, err)
	}

	// A developer edits: the text and the member it runs as are now theirs.
	edited, err := e.svc.EditQueuedMessage(caller("bob"), e.id, first.ID, "one, reworded")
	if err != nil || edited.Prompt != "one, reworded" || edited.Author != "bob" || edited.Position != 2 {
		t.Fatalf("edit = %+v, %v", edited, err)
	}
	if err := e.svc.DeleteQueuedMessage(caller("contrib"), e.id, second.ID); err != nil {
		t.Fatal(err)
	}
	if got := e.pending(t); !slices.Equal(got, []string{"one, reworded"}) {
		t.Fatalf("queue after the edits = %v", got)
	}
	if _, err := e.svc.EditQueuedMessage(caller("alice"), e.id, first.ID, "   "); !isCode(err, "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("an empty edit = %v", err)
	}
	if _, err := e.svc.MoveQueuedMessage(caller("alice"), e.id, first.ID, 0); !isCode(err, "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("a move to place 0 = %v", err)
	}
	if err := e.svc.DeleteQueuedMessage(caller("alice"), e.id, "not-an-id"); !isCode(err, "AGENT_QUEUED_MESSAGE_ID_INVALID") {
		t.Fatalf("a malformed message id = %v", err)
	}
	if err := e.svc.DeleteQueuedMessage(caller("alice"), e.id, second.ID); !isCode(err, codeQueuedMessageNotFound) {
		t.Fatalf("deleting a message twice = %v, want %s", err, codeQueuedMessageNotFound)
	}
}

func TestADispatchedMessageCannotBeEditedOrDeleted(t *testing.T) {
	e := newQueueEnv(t)
	msg := e.send(t, "alice", "runs next").Queued
	e.settle(t, e.id)
	if err := e.dispatcher().RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]func() error{
		"edit":   func() error { _, err := e.svc.EditQueuedMessage(caller("alice"), e.id, msg.ID, "late"); return err },
		"delete": func() error { return e.svc.DeleteQueuedMessage(caller("alice"), e.id, msg.ID) },
		"move":   func() error { _, err := e.svc.MoveQueuedMessage(caller("alice"), e.id, msg.ID, 1); return err },
	} {
		err := run()
		if !isCode(err, codeQueuedMessageGone) || !errors.Is(err, core.ErrConflict) {
			t.Errorf("%s of a dispatched message = %v, want a coded 409 %s", name, err, codeQueuedMessageGone)
		}
	}
	// A retried send of the dispatched message returns it, now dispatched.
	state, err := e.pg.QueuedAgentMessageState(context.Background(), e.id, msg.ID)
	if err != nil || state.DeliveredTurn == nil || *state.DeliveredTurn != 2 {
		t.Fatalf("message = %+v, %v", state, err)
	}
}

func TestACanceledOrArchivedSessionTakesAndKeepsNoMessages(t *testing.T) {
	e := newQueueEnv(t)
	e.send(t, "alice", "pending")
	if _, err := e.svc.Archive(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "more"}); !isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("a send to an archived session = %v", err)
	}
	if n, _ := e.pg.CountQueuedAgentMessages(context.Background(), e.id); n != 0 {
		t.Fatalf("archiving left %d messages", n)
	}
	if _, err := e.svc.Unarchive(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	e.send(t, "alice", "pending again")
	if _, err := e.svc.Cancel(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.pg.CountQueuedAgentMessages(context.Background(), e.id); n != 0 {
		t.Fatalf("canceling left %d messages", n)
	}
	if _, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "more"}); !isCode(err, "AGENT_SESSION_NOT_STEERABLE") {
		t.Fatalf("a send to a canceled session = %v", err)
	}
	// A canceled session's messages never run, whatever the dispatcher claims.
	if err := e.dispatcher().RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 1 {
		t.Fatalf("a canceled session ran a queued message: %v", got)
	}
	// Deleting a finished session takes its queue with it.
	other := queueEnv{lineageEnv: e.lineageEnv, id: e.create(t, "").ID}
	other.roles = e.roles
	other.send(t, "alice", "doomed")
	other.settle(t, other.id)
	if err := e.svc.Delete(caller("alice"), other.id); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.pg.CountQueuedAgentMessages(context.Background(), other.id); n != 0 {
		t.Fatalf("deleting left %d messages", n)
	}
}

func TestDispatcherRunsOneMessagePerSettledTurnInOrder(t *testing.T) {
	e := newQueueEnv(t)
	e.send(t, "alice", "one")
	e.send(t, "alice", "two")
	e.send(t, "alice", "three")
	d := e.dispatcher()
	ctx := context.Background()

	// A running turn holds the queue.
	if err := d.RunOnce(ctx); err != nil || len(e.prompts(t, e.id)) != 1 {
		t.Fatalf("the queue ran during a turn: %v %v", err, e.prompts(t, e.id))
	}
	// When it settles, the head runs and only the head.
	e.settle(t, e.id)
	if err := d.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "one" {
		t.Fatalf("turns after the first dispatch = %v", got)
	}
	if got := e.pending(t); !slices.Equal(got, []string{"two", "three"}) {
		t.Fatalf("queue after the first dispatch = %v", got)
	}
	// Another pass while that turn runs starts nothing.
	if err := d.RunOnce(ctx); err != nil || len(e.prompts(t, e.id)) != 2 {
		t.Fatalf("a second pass started another turn: %v %v", err, e.prompts(t, e.id))
	}
	e.settle(t, e.id)
	if err := d.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	e.settle(t, e.id)
	if err := d.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	want := []string{e.prompts(t, e.id)[0], "one", "two", "three"}
	if got := e.prompts(t, e.id); !slices.Equal(got, want) {
		t.Fatalf("turns = %v, want %v", got, want)
	}
	if len(e.pending(t)) != 0 {
		t.Fatalf("messages left over: %v", e.pending(t))
	}
	// A failed turn settles the queue too.
	e.settle(t, e.id)
	e.send(t, "alice", "after the failure")
	if _, err := e.pg.SetAgentSessionFailure(ctx, e.id, "", "boom"); err != nil {
		t.Fatal(err)
	}
	if err := d.RunOnce(ctx); err != nil || len(e.prompts(t, e.id)) != 5 {
		t.Fatalf("a failed turn did not release the queue: %v %v", err, e.prompts(t, e.id))
	}
}

func TestDispatcherSurvivesARestartAndTwoReplicas(t *testing.T) {
	e := newQueueEnv(t)
	e.send(t, "alice", "only once")
	e.send(t, "alice", "after")
	e.settle(t, e.id)
	ctx := context.Background()

	// Two replicas race for the one settled session: one turn starts.
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.dispatcher().RunOnce(ctx); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "only once" {
		t.Fatalf("racing replicas ran %v", got)
	}
	// A restart, a new dispatcher with nothing in memory, loses and repeats
	// nothing: the message recorded as delivered stays delivered.
	if err := e.dispatcher().RunOnce(ctx); err != nil || len(e.prompts(t, e.id)) != 2 {
		t.Fatalf("a restarted dispatcher repeated work: %v %v", err, e.prompts(t, e.id))
	}
	if got := e.pending(t); !slices.Equal(got, []string{"after"}) {
		t.Fatalf("queue after the restart = %v", got)
	}
}

func TestDispatcherReclaimsAnExpiredLease(t *testing.T) {
	e := newQueueEnv(t)
	e.send(t, "alice", "orphaned")
	e.settle(t, e.id)
	ctx := context.Background()
	// A replica claimed the head and crashed before starting the turn.
	now := time.Now()
	claims, err := e.pg.ClaimQueuedAgentMessages(ctx, now, queueLease, 200, []string{PhaseCompleted, PhaseFailed})
	if err != nil || len(claims) == 0 {
		t.Fatalf("claim = %v, %v", claims, err)
	}
	d := e.dispatcher()
	d.Now = func() time.Time { return now }
	if err := d.RunOnce(ctx); err != nil || len(e.prompts(t, e.id)) != 1 {
		t.Fatalf("a held lease was taken: %v %v", err, e.prompts(t, e.id))
	}
	d.Now = func() time.Time { return now.Add(queueLease + time.Second) }
	if err := d.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "orphaned" {
		t.Fatalf("an expired lease was not reclaimed: %v", got)
	}
}

func TestABlockedDispatchKeepsTheMessageAndSaysWhy(t *testing.T) {
	e := newQueueEnv(t)
	e.send(t, "alice", "waits for capacity")
	e.send(t, "alice", "behind it")
	e.settle(t, e.id)
	// The workspace is at its live-sandbox limit: another session holds the one slot.
	blocker := e.create(t, "")
	e.svc.MaxLiveSandboxes = 1
	ctx := context.Background()
	metrics := NewQueueMetrics(prometheus.NewRegistry())
	d := e.dispatcher()
	d.Metrics = metrics
	if err := d.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	list, err := e.svc.QueuedMessages(caller("alice"), e.id)
	if err != nil || len(list.QueuedMessages) != 2 || !strings.Contains(list.QueuedMessages[0].BlockedReason, "live agent-session sandboxes") {
		t.Fatalf("a blocked message = %+v, %v", list, err)
	}
	if list.QueuedMessages[1].BlockedReason != "" {
		t.Fatalf("the message behind the head was marked blocked: %+v", list.QueuedMessages[1])
	}
	if got := e.prompts(t, e.id); len(got) != 1 {
		t.Fatalf("a blocked message started a turn: %v", got)
	}
	if got := testutil.ToFloat64(metrics.blocked.WithLabelValues(blockedLiveLimit)); got != 1 {
		t.Fatalf("blocked live_limit = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.depth); got != 2 {
		t.Fatalf("queue depth = %v, want 2", got)
	}
	if histogramCount(t, metrics.dispatchLatency) != 0 {
		t.Fatalf("a blocked dispatch observed a latency sample")
	}
	// Capacity frees up; the next pass retries and the head runs. The pass
	// reads the clock a minute on, so the wait it reports is positive even when
	// the database's clock runs a little ahead of this one's.
	e.settle(t, blocker.ID)
	d.Now = func() time.Time { return time.Now().Add(time.Minute) }
	if err := d.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "waits for capacity" {
		t.Fatalf("a freed message did not run: %v", got)
	}
	if histogramCount(t, metrics.dispatchLatency) != 1 || testutil.ToFloat64(metrics.depth) != 1 {
		t.Fatalf("a started dispatch left depth %v and %d latency samples", testutil.ToFloat64(metrics.depth), histogramCount(t, metrics.dispatchLatency))
	}
	if got := e.pending(t); !slices.Equal(got, []string{"behind it"}) {
		t.Fatalf("queue = %v", got)
	}
}

func TestAMemberWhoLostAccessHasTheirMessageDropped(t *testing.T) {
	e := newQueueEnv(t)
	msg := e.send(t, "bob", "from a leaver").Queued
	e.send(t, "alice", "stays")
	e.settle(t, e.id)
	delete(e.fga.members, "bob") // no longer a member of the workspace
	d := e.dispatcher()
	d.Metrics = NewQueueMetrics(prometheus.NewRegistry())
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err := e.pg.QueuedAgentMessageState(context.Background(), e.id, msg.ID)
	if err != nil || state.DroppedReason == "" {
		t.Fatalf("the leaver's message = %+v, %v", state, err)
	}
	if got := e.prompts(t, e.id); len(got) != 1 {
		t.Fatalf("a message ran as a member who lost access: %v", got)
	}
	// The message behind it is not held up: the next pass runs it.
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "stays" {
		t.Fatalf("the next message = %v", got)
	}
}

// twoFactorGate refuses the subjects it blocks, as a workspace that requires a
// second factor refuses a member who has none.
type twoFactorGate struct {
	ws      string
	mu      sync.Mutex
	blocked map[string]bool
}

func (g *twoFactorGate) Check(_ context.Context, workspaceID string, id core.Identity) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.blocked[id.Subject] {
		return core.NewWorkspaceTwoFactorRequiredError(workspaceID)
	}
	return nil
}

func (g *twoFactorGate) ObjectWorkspace(context.Context, string) (string, error) { return g.ws, nil }

// A refusal whose cause a person can fix (no model key, an agent the workspace
// cannot run, a member without the second factor the workspace requires) does
// not cost the person their message: it stays queued, says why, and runs once.
func TestAMessageWaitsOutARefusalAPersonCanFix(t *testing.T) {
	for _, tc := range []struct {
		name   string
		says   string
		agent  string // the session's agent; claude when empty
		breaks func(e queueEnv, gate *twoFactorGate)
		fixes  func(e queueEnv, gate *twoFactorGate)
	}{
		{name: "the workspace has no model key", says: "model API key",
			breaks: func(e queueEnv, _ *twoFactorGate) {
				e.svc.ModelKeys = &fakeModelKeys{data: map[string]map[string]string{}}
			},
			fixes: func(e queueEnv, _ *twoFactorGate) { e.svc.ModelKeys = fixtureModelKeys(e.ws) }},
		{name: "the author has no second factor", says: "two-factor",
			breaks: func(_ queueEnv, gate *twoFactorGate) { gate.blocked["alice"] = true },
			fixes:  func(_ queueEnv, gate *twoFactorGate) { delete(gate.blocked, "alice") }},
		{name: "the agent is not available to the workspace", says: "not available", agent: "muse",
			breaks: func(_ queueEnv, _ *twoFactorGate) { agentsession.EnablePlatformMetaRoute("", nil) },
			fixes: func(e queueEnv, _ *twoFactorGate) {
				agentsession.EnablePlatformMetaRoute("meta-PLATFORM-SECRET", []string{e.ws})
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newQueueEnv(t)
			gate := &twoFactorGate{ws: e.ws, blocked: map[string]bool{}}
			e.svc.Base.TwoFactor = gate
			if tc.agent != "" {
				enableMetaRoute(t, e.ws)
				enableMuse(e.svc, e.ws)
				in := e.input("")
				in.AgentConfig = AgentConfig{Agent: tc.agent, Task: "fix the tests"}
				view, err := e.svc.Create(caller("alice"), in)
				if err != nil {
					t.Fatalf("create %s: %v", tc.agent, err)
				}
				e.id = view.ID
			}
			e.send(t, "alice", "waits for the fix")
			e.send(t, "alice", "behind it")
			e.settle(t, e.id)
			tc.breaks(e, gate)

			d := e.dispatcher()
			d.Metrics = NewQueueMetrics(prometheus.NewRegistry())
			ctx := context.Background()
			for pass := 1; pass <= 2; pass++ {
				if err := d.RunOnce(ctx); err != nil {
					t.Fatal(err)
				}
				list, err := e.svc.QueuedMessages(caller("bob"), e.id) // alice may be the blocked one
				if err != nil || len(list.QueuedMessages) != 2 || !strings.Contains(list.QueuedMessages[0].BlockedReason, tc.says) {
					t.Fatalf("pass %d: the queue = %+v, %v; want both messages, the head saying why (%q)", pass, list, err, tc.says)
				}
				if got := e.prompts(t, e.id); len(got) != 1 {
					t.Fatalf("pass %d: a refused message started a turn: %v", pass, got)
				}
			}
			if got := testutil.ToFloat64(d.Metrics.dropped); got != 0 {
				t.Fatalf("a fixable refusal was counted as a drop: %v", got)
			}

			tc.fixes(e, gate)
			if err := d.RunOnce(ctx); err != nil {
				t.Fatal(err)
			}
			if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "waits for the fix" {
				t.Fatalf("turns after the fix = %v; want the head started once", got)
			}
			if got := e.pending(t); !slices.Equal(got, []string{"behind it"}) {
				t.Fatalf("queue after the fix = %v", got)
			}
		})
	}
}

func TestFeedbackYieldsToQueuedMessagesAndIsNotStarved(t *testing.T) {
	e := newQueueEnv(t)
	ctx := context.Background()
	if _, err := e.pg.QueueAgentSessionFeedback(ctx, store.AgentSessionFeedback{SessionID: e.id, SourceKey: "comment:1", Kind: store.FeedbackPRComment, Actor: "alice", Author: "rev", Body: "> please rename"}); err != nil {
		t.Fatal(err)
	}
	e.send(t, "alice", "a person goes first")
	e.settle(t, e.id)
	feedback := &PRFeedback{Sessions: e.svc, Store: e.pg}
	if err := feedback.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 1 {
		t.Fatalf("feedback ran ahead of a person's message: %v", got)
	}
	// Even a feedback steer that slipped past the claim queues nothing and starts nothing.
	if _, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "feedback", mode: steerYields}); !isCode(err, codeTurnInFlight) {
		t.Fatalf("a yielding steer = %v, want the in-flight refusal", err)
	}
	if got := e.pending(t); !slices.Equal(got, []string{"a person goes first"}) {
		t.Fatalf("a feedback steer changed the queue: %v", got)
	}
	// The person's message runs; when that turn settles the feedback is delivered.
	if err := e.dispatcher().RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	e.settle(t, e.id)
	if err := feedback.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	got := e.prompts(t, e.id)
	if len(got) != 3 || got[1] != "a person goes first" || !strings.Contains(got[2], "please rename") {
		t.Fatalf("turns = %v; want the message, then the feedback", got)
	}
}

func TestInterruptAsksTheDriverToStopTheRunningTurnOnly(t *testing.T) {
	e := newQueueEnv(t)
	e.send(t, "alice", "queued behind it")
	row, _ := e.pg.GetAgentSession(context.Background(), e.id)
	view, err := e.svc.Interrupt(caller("alice"), e.id)
	if err != nil || view.ID != e.id {
		t.Fatalf("interrupt = %+v, %v", view, err)
	}
	if want := fmt.Sprintf("%s/%s/1", e.id, row.SandboxID); !slices.Equal(e.lc.interrupts, []string{want}) {
		t.Fatalf("driver asked %v, want %q", e.lc.interrupts, want)
	}
	// Only a request: the turn settles when the Completer sees the driver's status.
	if got := e.phase(t, e.id); got != PhaseRunning {
		t.Fatalf("interrupt moved the session to %s", got)
	}
	if got := e.pending(t); !slices.Equal(got, []string{"queued behind it"}) {
		t.Fatalf("interrupt changed the queue: %v", got)
	}

	e.lc.interruptErr = errors.New("gateway down")
	if _, err := e.svc.Interrupt(caller("alice"), e.id); err == nil {
		t.Fatal("a failed request to the driver reported success")
	}
	e.lc.interruptErr = nil

	// What is not a running turn is refused with its own code.
	for phase, code := range map[string]string{
		PhaseCompleted:     codeNoTurnRunning,
		PhaseFailed:        codeNoTurnRunning,
		PhaseCanceled:      "AGENT_SESSION_NOT_STEERABLE",
		PhaseCanceling:     "AGENT_SESSION_NOT_STEERABLE",
		PhaseCreating:      codeTurnStarting,
		PhaseRedispatching: codeTurnStarting,
	} {
		e.setPhase(t, e.id, phase)
		if _, err := e.svc.Interrupt(caller("alice"), e.id); !isCode(err, code) || !errors.Is(err, core.ErrConflict) {
			t.Errorf("interrupt in %s = %v, want a coded 409 %s", phase, err, code)
		}
	}
	e.setPhase(t, e.id, PhaseRunning)
	if _, err := e.svc.Archive(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Interrupt(caller("alice"), e.id); !isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("interrupt of an archived session = %v", err)
	}
	if len(e.lc.interrupts) != 2 {
		t.Fatalf("a refused interrupt reached the driver: %v", e.lc.interrupts)
	}
}

// Two sends racing for one idle session: one starts the turn, the other finds
// the turn started (at the read or at the write) and queues behind it, so no
// send is refused as in flight.
func TestConcurrentSendsToAnIdleSessionStartOneTurnAndQueueTheRest(t *testing.T) {
	e := newQueueEnv(t)
	e.settle(t, e.id)
	const senders = 5
	results := make([]View, senders)
	errs := make([]error, senders)
	var wg sync.WaitGroup
	for i := range senders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: fmt.Sprintf("send %d", i)})
		}()
	}
	wg.Wait()
	started, queued := 0, 0
	for i := range senders {
		switch {
		case errs[i] != nil:
			t.Fatalf("send %d: %v", i, errs[i])
		case results[i].Queued != nil:
			queued++
		default:
			started++
		}
	}
	if started != 1 || queued != senders-1 {
		t.Fatalf("%d started and %d queued, want 1 and %d", started, queued, senders-1)
	}
	if got := len(e.prompts(t, e.id)); got != 2 {
		t.Fatalf("turns = %d, want the original and one more", got)
	}
	if got := len(e.pending(t)); got != senders-1 {
		t.Fatalf("pending = %d, want %d", got, senders-1)
	}
}

// Each refusal the queue's store raises reaches the client as its own code.
func TestQueueRefusalsMapToTheirCodes(t *testing.T) {
	record := store.AgentSession{ID: "ags-x", Phase: PhaseRunning}
	archived := store.AgentSession{ID: "ags-x", Phase: PhaseRunning, ArchivedAt: &time.Time{}}
	for _, tc := range []struct {
		err    error
		record store.AgentSession
		code   string
		want   error
	}{
		{store.ErrAgentSessionQueueFull, record, codeQueueFull, core.ErrConflict},
		{store.ErrQueuedMessageKeyReused, record, codeIdempotencyKeyReused, core.ErrConflict},
		{store.ErrQueuedMessageNotPending, record, codeQueuedMessageGone, core.ErrConflict},
		{store.ErrAgentSessionNotSteerable, record, "AGENT_SESSION_NOT_STEERABLE", core.ErrConflict},
		{store.ErrAgentSessionNotSteerable, archived, "AGENT_SESSION_ARCHIVED", core.ErrConflict},
		{store.ErrQueuedMessageAttachmentsOwned, record, "AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED", core.ErrForbidden},
		{store.ErrNotFound, record, codeQueuedMessageNotFound, core.ErrNotFound},
	} {
		got := mapQueueError(tc.record, tc.err)
		if !isCode(got, tc.code) || !errors.Is(got, tc.want) {
			t.Errorf("%v mapped to %v, want %s wrapping %v", tc.err, got, tc.code, tc.want)
		}
	}
}

// histogramCount is how many samples a histogram holds.
func histogramCount(t *testing.T, h prometheus.Histogram) uint64 {
	t.Helper()
	var m dto.Metric
	if err := h.Write(&m); err != nil {
		t.Fatal(err)
	}
	return m.GetHistogram().GetSampleCount()
}
