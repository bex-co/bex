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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// adoptedCtx is ctx carrying an App whose operator reports release generation
// adopted.
func adoptedCtx(ctx context.Context, generation int64) context.Context {
	return WithAdoptedRelease(ctx, &appv1alpha1.App{Status: appv1alpha1.AppStatus{ReleaseGeneration: generation}})
}

// w8/396: once the deploy ahead serves, the operator adopts the deploy waiting
// behind it and runs it to the end, but the reconciler frees its pending slot
// only on its next pass. A deploy opened in between, told the App's adopted
// release, waits behind it rather than canceling it; a waiting deploy the
// operator has not adopted is still replaced.
func TestPGAdoptedWaitingDeployIsNotReplaced(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	row, first := deferredRestartApp(t, st)
	if _, err := st.TransitionDeploy(ctx, first.ID, DeployLive, RanImage{}, "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	ahead, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", first.Generation+1, CommitInfo{}, "")
	if err != nil || ahead.Status != DeployCreated {
		t.Fatalf("deploy ahead = %+v (err %v)", ahead, err)
	}
	waiting, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", first.Generation+2, CommitInfo{}, "")
	if err != nil || waiting.Status != DeployQueued || !waiting.OverlapPending {
		t.Fatalf("waiting deploy = %+v (err %v), want the pending slot", waiting, err)
	}
	if _, err := st.TransitionDeploy(ctx, ahead.ID, DeployLive, RanImage{}, "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	stillWaiting := func(when string) {
		t.Helper()
		got, err := st.GetDeploy(ctx, row.ID, waiting.ID)
		if err != nil || got.Status != DeployQueued || got.OverlapPending || got.CancelReason != "" {
			t.Fatalf("%s: adopted deploy = %+v (err %v), want it executing (queued, out of the pending slot)", when, got, err)
		}
	}

	// A Restart (and any deploy that waits at the row level) waits behind it.
	restart, deferred, err := st.CreateDeferredDeploy(adoptedCtx(ctx, waiting.Generation), Deploy{AppID: row.ID, Trigger: TriggerRestart}, DeferredRequest{})
	if err != nil || !deferred {
		t.Fatalf("restart behind an adopted waiting deploy: deferred=%v err=%v, want it to wait", deferred, err)
	}
	stillWaiting("after a restart")

	// A deploy replaces only the Restart waiting behind it.
	next, err := st.CreateDeploy(adoptedCtx(ctx, waiting.Generation), row.ID, TriggerAPI, "", first.Generation+3, CommitInfo{}, "")
	if err != nil || next.Status != DeployQueued || !next.OverlapPending {
		t.Fatalf("deploy behind an adopted waiting deploy = %+v (err %v), want queued in the pending slot", next, err)
	}
	stillWaiting("after a deploy")
	if got, _ := st.GetDeploy(ctx, row.ID, restart.ID); got.Status != DeployCanceled || SupersedingDeployID(got.CancelReason) != next.ID {
		t.Fatalf("waiting restart = %s (%q), want replaced by %s", got.Status, got.CancelReason, next.ID)
	}

	// The control: a waiting deploy the operator has not adopted is replaced.
	last, err := st.CreateDeploy(adoptedCtx(ctx, waiting.Generation), row.ID, TriggerAPI, "", first.Generation+4, CommitInfo{}, "")
	if err != nil || last.Status != DeployQueued {
		t.Fatalf("newest deploy = %+v (err %v), want queued", last, err)
	}
	if got, _ := st.GetDeploy(ctx, row.ID, next.ID); got.Status != DeployCanceled || SupersedingDeployID(got.CancelReason) != last.ID {
		t.Fatalf("unadopted waiting deploy = %s (%q), want replaced by %s", got.Status, got.CancelReason, last.ID)
	}
	stillWaiting("after the control")
}

// w8/396: without the App's adopted release, or while the deploy ahead is
// still recorded executing, the pending slot is left to the reconciler: two
// executing rows would break the one-executing-deploy rule.
func TestPGWaitingDeployAdoptionNeedsAFreeSlot(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	row, first := deferredRestartApp(t, st)
	waiting, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", first.Generation+1, CommitInfo{}, "")
	if err != nil || !waiting.OverlapPending {
		t.Fatalf("waiting deploy = %+v (err %v), want the pending slot", waiting, err)
	}
	next, err := st.CreateDeploy(adoptedCtx(ctx, waiting.Generation), row.ID, TriggerAPI, "", first.Generation+2, CommitInfo{}, "")
	if err != nil || next.Status != DeployQueued {
		t.Fatalf("deploy while the first is still recorded executing = %+v (err %v), want queued", next, err)
	}
	if got, _ := st.GetDeploy(ctx, row.ID, first.ID); got.Status != first.Status || got.OverlapPending {
		t.Fatalf("deploy recorded executing = %+v, want untouched", got)
	}

	if _, err := st.TransitionDeploy(ctx, first.ID, DeployLive, RanImage{}, "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	last, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", first.Generation+3, CommitInfo{}, "")
	if err != nil || last.Status != DeployCreated {
		t.Fatalf("deploy with no adopted release named = %+v (err %v), want it to replace the waiting one and start", last, err)
	}
	if got, _ := st.GetDeploy(ctx, row.ID, next.ID); got.Status != DeployCanceled || SupersedingDeployID(got.CancelReason) != last.ID {
		t.Fatalf("waiting deploy = %s (%q), want replaced by %s", got.Status, got.CancelReason, last.ID)
	}
}
