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

package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func deferredRestartApp(t *testing.T, st *PGStore) (App, Deploy) {
	t.Helper()
	ctx := context.Background()
	ten := lifecycleTenant(t, st, "deferred-restart")
	row, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: fmt.Sprintf("web-%d", time.Now().UnixNano()), Repo: "https://example.com/acme/web.git",
		Branch: "main", Port: 80, Replicas: 1, Tier: "free",
	})
	if err != nil {
		t.Fatal(err)
	}
	history, err := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(history) != 1 {
		t.Fatalf("CreateApp history = %+v (err %v), want its first deploy", history, err)
	}
	return row, history[0]
}

func readyOf(t *testing.T, st *PGStore, id string) (DeferredRestart, bool) {
	t.Helper()
	waiting, err := st.ListDeferredRestarts(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range waiting {
		if r.ID == id {
			return r, true
		}
	}
	return DeferredRestart{}, false
}

// w5/m284: a Restart behind a deploy in flight waits in the pending slot with
// no release, becomes ready once that deploy settles, and is promoted in place
// to the release opened for it.
func TestPGDeferredRestartWaitsThenPromotes(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	row, executing := deferredRestartApp(t, st)

	// Nothing in flight: no deferral.
	if _, err := st.TransitionDeploy(ctx, executing.ID, DeployLive, RanImage{}, "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, deferred, err := st.CreateDeferredRestart(ctx, row.ID, "usr-a"); err != nil || deferred {
		t.Fatalf("idle app: deferred=%v err=%v, want a restart now", deferred, err)
	}

	inFlight, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", executing.Generation+1, CommitInfo{}, "")
	if err != nil || inFlight.Status != DeployCreated {
		t.Fatalf("in flight = %+v (err %v)", inFlight, err)
	}
	pending, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", executing.Generation+2, CommitInfo{}, "")
	if err != nil || pending.Status != DeployQueued {
		t.Fatalf("pending = %+v (err %v)", pending, err)
	}
	restart, deferred, err := st.CreateDeferredRestart(ctx, row.ID, "usr-a")
	if err != nil || !deferred {
		t.Fatalf("deferred=%v err=%v, want a waiting restart", deferred, err)
	}
	if restart.Status != DeployQueued || restart.Trigger != TriggerRestart || restart.Generation != 0 || !restart.RestartDeferred {
		t.Fatalf("restart = %+v, want a queued generation-0 deferred restart", restart)
	}
	// Wait, not Override: the deploy in flight is untouched, the pending one is
	// replaced (Render skips intermediate deploys).
	if got, _ := st.GetDeploy(ctx, row.ID, inFlight.ID); got.Status != DeployCreated {
		t.Errorf("deploy in flight = %q, want left running", got.Status)
	}
	if got, _ := st.GetDeploy(ctx, row.ID, pending.ID); got.Status != DeployCanceled || !strings.Contains(got.CancelReason, restart.ID) {
		t.Errorf("replaced pending = %q %q, want canceled naming %s", got.Status, got.CancelReason, restart.ID)
	}
	if r, ok := readyOf(t, st, restart.ID); !ok || r.Ready {
		t.Fatalf("listed=%v ready=%v, want waiting and not ready while a deploy is in flight", ok, r.Ready)
	}
	if err := st.CheckDeferredRestart(ctx, restart.ID, row.ID); !errors.Is(err, ErrDeferredRestartNotReady) {
		t.Fatalf("check while in flight: err = %v, want ErrDeferredRestartNotReady", err)
	}
	if _, err := st.CreateRestartDeploy(WithDeferredRestart(ctx, restart.ID), row.ID, "img", executing.Generation+3, CommitInfo{}, ""); !errors.Is(err, ErrDeferredRestartNotReady) {
		t.Fatalf("promote while in flight: err = %v, want ErrDeferredRestartNotReady", err)
	}

	if _, err := st.TransitionDeploy(ctx, inFlight.ID, DeployLive, RanImage{}, "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if r, ok := readyOf(t, st, restart.ID); !ok || !r.Ready {
		t.Fatalf("listed=%v ready=%v, want ready once the deploy settled", ok, r.Ready)
	}
	if err := st.CheckDeferredRestart(ctx, restart.ID, row.ID); err != nil {
		t.Fatalf("check once settled: %v", err)
	}
	promoted, err := st.CreateRestartDeploy(WithDeferredRestart(ctx, restart.ID), row.ID, "registry.example/acme/web:gen-1", executing.Generation+3,
		CommitInfo{Hash: "abc123"}, "someone-else")
	if err != nil {
		t.Fatal(err)
	}
	if promoted.ID != restart.ID || promoted.Status != DeployCreated || promoted.Generation != executing.Generation+3 ||
		promoted.RestartDeferred || promoted.OverlapPending || promoted.Commit != "abc123" || promoted.TriggeredBy != "usr-a" ||
		!promoted.CreatedAt.Equal(restart.CreatedAt) {
		t.Fatalf("promoted = %+v, want the same row, released at the new generation, keeping its caller and trigger time", promoted)
	}
	if _, ok := readyOf(t, st, restart.ID); ok {
		t.Error("a promoted restart is still listed as waiting")
	}
	// A replica that listed it before the promotion can neither run nor
	// cancel it now.
	if err := st.CheckDeferredRestart(ctx, restart.ID, row.ID); !errors.Is(err, ErrDeferredRestartGone) {
		t.Errorf("check after promotion: err = %v, want ErrDeferredRestartGone", err)
	}
	if won, err := st.CancelDeferredRestart(ctx, restart.ID, "late"); err != nil || won {
		t.Errorf("cancel after promotion: won=%v err=%v, want a no-op", won, err)
	}
	if _, err := st.CreateRestartDeploy(WithDeferredRestart(ctx, restart.ID), row.ID, "img", executing.Generation+4, CommitInfo{}, ""); !errors.Is(err, ErrDeferredRestartGone) {
		t.Fatalf("promote twice: err = %v, want ErrDeferredRestartGone", err)
	}
}

// A newer trigger replaces a waiting restart like any pending row, and the
// replaced restart is never promoted.
func TestPGDeferredRestartIsReplacedByANewerTrigger(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	row, executing := deferredRestartApp(t, st)
	restart, deferred, err := st.CreateDeferredRestart(ctx, row.ID, "")
	if err != nil || !deferred {
		t.Fatalf("deferred=%v err=%v", deferred, err)
	}
	push, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", executing.Generation+1, CommitInfo{}, "")
	if err != nil || push.Status != DeployQueued {
		t.Fatalf("push = %+v (err %v), want queued behind the deploy in flight", push, err)
	}
	got, _ := st.GetDeploy(ctx, row.ID, restart.ID)
	if got.Status != DeployCanceled || got.CancelReason != supersededByReason(push.ID) || got.FinishedAt == nil {
		t.Fatalf("restart = %q %q, want superseded by %s", got.Status, got.CancelReason, push.ID)
	}
	if _, err := st.CreateRestartDeploy(WithDeferredRestart(ctx, restart.ID), row.ID, "img", executing.Generation+2, CommitInfo{}, ""); !errors.Is(err, ErrDeferredRestartGone) {
		t.Fatalf("promote replaced restart: err = %v, want ErrDeferredRestartGone", err)
	}
}

// Canceling a waiting restart closes it with the reason; a second cancel is a
// no-op.
func TestPGCancelDeferredRestart(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	row, _ := deferredRestartApp(t, st)
	restart, _, err := st.CreateDeferredRestart(ctx, row.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if won, err := st.CancelDeferredRestart(ctx, restart.ID, "gave up"); err != nil || !won {
		t.Fatalf("cancel: won=%v err=%v", won, err)
	}
	got, _ := st.GetDeploy(ctx, row.ID, restart.ID)
	if got.Status != DeployCanceled || got.CancelReason != "gave up" || got.FinishedAt == nil || got.OverlapPending {
		t.Fatalf("canceled = %+v", got)
	}
	if won, _ := st.CancelDeferredRestart(ctx, restart.ID, "again"); won {
		t.Error("a second cancel landed")
	}
}
