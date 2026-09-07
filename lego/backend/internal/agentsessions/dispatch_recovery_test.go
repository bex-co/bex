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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type recoveryLifecycle struct {
	*fakeLifecycle
	cleanup func(context.Context, []store.AgentDispatch) error
}

func (l *recoveryLifecycle) CleanupAgentDispatches(ctx context.Context, d []store.AgentDispatch) error {
	return l.cleanup(ctx, d)
}

func TestDispatchRecoverySurvivesRestartAndLateRemoteCreate(t *testing.T) {
	c, st, lc, _, _ := completerFixture("", nil)
	row, _ := st.CreateAgentSession(context.Background(), store.AgentSession{WorkspaceID: "tea-a", InitialPrompt: "keep my task"})
	now := st.now.Add(dispatchTimeout - time.Second)
	c.Now = func() time.Time { return now }
	calls := 0
	orphan := false
	unavailable := true
	recovery := &recoveryLifecycle{fakeLifecycle: lc, cleanup: func(_ context.Context, ds []store.AgentDispatch) error {
		calls++
		if unavailable {
			return errors.New("upstream down")
		}
		orphan = false
		return nil
	}}
	c.Sandbox = recovery
	c.recoverDispatches(context.Background())
	if calls != 0 || st.rows[row.ID].Phase != PhaseCreating {
		t.Fatal("healthy provision interrupted early")
	}
	now = now.Add(time.Second)
	c.recoverDispatches(context.Background())
	if st.rows[row.ID].Phase != PhaseFailed || st.rows[row.ID].FailureReason != interruptedDispatchReason {
		t.Fatalf("stuck row: %+v", st.rows[row.ID])
	}
	// New worker has no in-memory failure streak or dispatch goroutine.
	restarted := &Completer{Store: st, Sandbox: recovery, Now: func() time.Time { return now }}
	now = now.Add(time.Minute)
	unavailable = false
	restarted.recoverDispatches(context.Background())
	if calls != 2 {
		t.Fatalf("failed cleanup not retried: %d", calls)
	}
	orphan = true // remote create materializes after the successful empty sweep
	now = now.Add(time.Hour)
	restarted.recoverDispatches(context.Background())
	if orphan || calls != 3 {
		t.Fatal("late remote sandbox escaped tombstone cleanup")
	}
	turn := st.turns[row.ID][row.Turns]
	if turn.CompletedAt == nil || turn.TranscriptComplete || turn.Prompt != "keep my task" {
		t.Fatalf("turn=%+v", turn)
	}
}

func TestDispatchRecoveryGroupsWorkspaceDiscoveryAndRotatesBeforeIO(t *testing.T) {
	c, st, lc, _, _ := completerFixture("", nil)
	for range 3 {
		_, _ = st.CreateAgentSession(context.Background(), store.AgentSession{WorkspaceID: "tea-a", InitialPrompt: "task"})
	}
	now := st.now.Add(dispatchTimeout)
	c.Now = func() time.Time { return now }
	calls := 0
	c.Sandbox = &recoveryLifecycle{fakeLifecycle: lc, cleanup: func(ctx context.Context, ds []store.AgentDispatch) error {
		calls++
		if len(ds) != 3 {
			t.Fatalf("batch=%d", len(ds))
		}
		due, _ := st.ListAgentDispatchesDue(ctx, now)
		if len(due) != 0 {
			t.Fatal("failed workspace can starve following tick")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 10*time.Second {
			t.Fatal("recovery has no bounded I/O deadline")
		}
		return errors.New("offline")
	}}
	c.recoverDispatches(context.Background())
	if calls != 1 {
		t.Fatalf("workspace listings=%d", calls)
	}
}
