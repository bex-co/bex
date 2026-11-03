package agentsessions

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A mutex-guarded phase change, the way the completer moves a session on.
func setPhase(st *fakeStore, _ *sync.Mutex, id, phase string) {
	st.rowMu.Lock()
	defer st.rowMu.Unlock()
	row := st.rows[id]
	row.Phase = phase
	st.rows[id] = row
}

func waitFixture(t *testing.T, phases ...string) (*Service, *fakeStore, []string) {
	t.Helper()
	svc, st, fga, _ := fixture()
	svc.WaitPoll = time.Hour // only notifications may wake a waiter, unless a test says otherwise
	ids := make([]string, 0, len(phases))
	for _, phase := range phases {
		phase := phase
		ids = append(ids, seedSession(st, fga, func(r *store.AgentSession) { r.Phase = phase }))
	}
	return svc, st, ids
}

func TestWaitReturnsAtOnceWhenEverythingIsSettled(t *testing.T) {
	svc, st, ids := waitFixture(t, PhaseCompleted, PhaseFailed)
	seedCompletedTurn(st, ids[0], 1, `{"type":"text","text":"done"}`)

	start := time.Now()
	got, err := svc.Wait(caller("alice"), WaitRequest{IDs: ids}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second || got.TimedOut || len(got.Settled) != 2 {
		t.Fatalf("settled wait = %+v after %s", got, time.Since(start))
	}
	if got.Sessions[0].State != StateSucceeded || got.Sessions[1].State != StateFailed {
		t.Fatalf("states = %+v", got.Sessions)
	}
	if got.Sessions[0].Outcome.Summary != "done" {
		t.Fatalf("settled session carries no summary: %+v", got.Sessions[0].Outcome)
	}
}

func TestWaitWakesOnAPhaseChangeNotification(t *testing.T) {
	svc, st, ids := waitFixture(t, PhaseRunning)
	var mu sync.Mutex
	done := make(chan WaitResult, 1)
	go func() {
		res, err := svc.Wait(caller("alice"), WaitRequest{IDs: ids, TimeoutSeconds: 20}, nil)
		if err != nil {
			t.Error(err)
		}
		done <- res
	}()
	// Wait until the listener exists, then move the session on and notify.
	deadline := time.Now().Add(5 * time.Second)
	for {
		st.listenMu.Lock()
		ready := len(st.stateListeners) > 0
		st.listenMu.Unlock()
		if ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // let the waiter reach its select
	setPhase(st, &mu, ids[0], PhaseCompleted)
	st.notifyState(ids[0])
	select {
	case res := <-done:
		if res.TimedOut || len(res.Settled) != 1 || res.Sessions[0].State != StateSucceeded {
			t.Fatalf("woken wait = %+v", res)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waiter was not woken by the notification (poll interval is an hour)")
	}
}

func TestWaitRecoversAMissedNotificationOnTheSlowTimer(t *testing.T) {
	svc, st, ids := waitFixture(t, PhaseRunning)
	svc.WaitPoll = 20 * time.Millisecond
	var mu sync.Mutex
	go func() {
		time.Sleep(100 * time.Millisecond)
		setPhase(st, &mu, ids[0], PhaseFailed) // no notification sent
	}()
	res, err := svc.Wait(caller("alice"), WaitRequest{IDs: ids, TimeoutSeconds: 10}, nil)
	if err != nil || res.TimedOut || res.Sessions[0].State != StateFailed {
		t.Fatalf("wait = %+v err=%v, want failed via re-read", res, err)
	}
}

func TestWaitAnyVersusAll(t *testing.T) {
	svc, _, ids := waitFixture(t, PhaseCompleted, PhaseRunning)

	any, err := svc.Wait(caller("alice"), WaitRequest{IDs: ids, Until: "any"}, nil)
	if err != nil || any.TimedOut || len(any.Settled) != 1 || any.Settled[0] != ids[0] {
		t.Fatalf("until=any = %+v err=%v, want only %s settled, no timeout", any, err, ids[0])
	}
	all, err := svc.Wait(caller("alice"), WaitRequest{IDs: ids, Until: "all", TimeoutSeconds: 1}, nil)
	if err != nil || !all.TimedOut || len(all.Settled) != 1 {
		t.Fatalf("until=all with one still working = %+v err=%v, want timedOut", all, err)
	}
	if all.Sessions[1].State != StateWorking {
		t.Fatalf("the working session reads %q", all.Sessions[1].State)
	}
}

func TestWaitReportsProgressOnEachChange(t *testing.T) {
	svc, st, ids := waitFixture(t, PhaseRunning)
	svc.WaitPoll = 10 * time.Millisecond
	var mu sync.Mutex
	go func() {
		time.Sleep(60 * time.Millisecond)
		setPhase(st, &mu, ids[0], PhaseCompleted)
	}()
	var lines []string
	if _, err := svc.Wait(caller("alice"), WaitRequest{IDs: ids, TimeoutSeconds: 10}, func(l string) { lines = append(lines, l) }); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0] != "0 of 1 agent sessions settled" || lines[1] != "1 of 1 agent sessions settled" {
		t.Fatalf("progress lines = %q, want one per change, not one per poll", lines)
	}
}

func TestWaitRefusesBadInputAndForeignSessions(t *testing.T) {
	svc, st, fga, _ := fixture()
	foreign := seedLiveSession(st, "tea-b") // bob's workspace, alice has no tuple on it
	_ = fga
	own := seedSession(st, fga, func(*store.AgentSession) {})

	for name, req := range map[string]WaitRequest{
		"no ids":        {},
		"too many ids":  {IDs: make([]string, 21)},
		"malformed id":  {IDs: []string{"srv-nope"}},
		"bad until":     {IDs: []string{own}, Until: "some"},
		"timeout zero+": {IDs: []string{own}, TimeoutSeconds: 26},
		"timeout neg":   {IDs: []string{own}, TimeoutSeconds: -1},
	} {
		if name == "too many ids" {
			for i := range req.IDs {
				req.IDs[i] = own
			}
		}
		if _, err := svc.Wait(caller("alice"), req, nil); !isCode(err, "AGENT_SESSION_INPUT_INVALID") && !isCode(err, "AGENT_SESSION_ID_INVALID") {
			t.Errorf("%s: err = %v, want a coded 400", name, err)
		}
	}
	if _, err := svc.Wait(caller("alice"), WaitRequest{IDs: []string{foreign}}, nil); err == nil || !isForbidden(err) {
		t.Errorf("foreign session = %v, want forbidden", err)
	}
}

func isForbidden(err error) bool {
	return err != nil && (err == core.ErrForbidden || err.Error() == core.ErrForbidden.Error())
}

func TestWaitAgentSessionsOverMCPIsReadOnlyAndReturnsOutcomes(t *testing.T) {
	svc, st, ids := waitFixture(t, PhaseCompleted)
	seedCompletedTurn(st, ids[0], 1, `{"type":"text","text":"shipped"}`)
	ctx := caller("alice")
	client := newMCPClient(t, ctx, svc)

	tools, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "wait_agent_sessions" {
			if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
				t.Fatalf("wait_agent_sessions annotations = %+v, want read-only and non-destructive", tool.Annotations)
			}
		}
	}
	res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "wait_agent_sessions", Arguments: map[string]any{"ids": ids}})
	if err != nil || res.IsError {
		t.Fatalf("call = %+v err=%v", res, err)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out WaitResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.TimedOut || len(out.Settled) != 1 || out.Sessions[0].Outcome.Summary != "shipped" {
		t.Fatalf("result = %+v", out)
	}
}
