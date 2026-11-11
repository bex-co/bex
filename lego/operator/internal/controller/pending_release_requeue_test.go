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

package controller

import (
	"context"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// pending_release_requeue_test.go is w8/270. A deploy that arrives while
// another release is in flight waits in the pending slot. The pass that
// settles the running release writes status only, which the App watch ignores,
// so nothing brought the reconcile back: the waiting deploy started up to a
// minute later, at the next unrelated event. Render starts it at once.
func TestTheSettlingPassComesStraightBackForAWaitingDeploy(t *testing.T) {
	r := &AppReconciler{}
	waiting := &appv1alpha1.App{}
	waiting.Status.ReleaseGeneration, waiting.Status.PendingReleaseGeneration = 4, 5

	res, err := r.runningRequeue(context.Background(), waiting, false)
	if err != nil || res.RequeueAfter <= 0 || res.RequeueAfter > time.Second {
		t.Fatalf("a Running app with a deploy waiting requeues after %v (err %v), want at most 1s", res.RequeueAfter, err)
	}
	// The autoscaling poll does not postpone it either.
	if res, _ := r.runningRequeue(context.Background(), waiting, true); res.RequeueAfter > time.Second {
		t.Errorf("an autoscaled app with a deploy waiting requeues after %v, want at most 1s", res.RequeueAfter)
	}
	if res, owed := owedReleaseRequeue(waiting); !owed || res.RequeueAfter != pendingReleaseRequeue {
		t.Errorf("owedReleaseRequeue = %v, %v", res, owed)
	}

	// With nothing waiting, the steady-state beat stands, and a release that
	// has already been adopted is not owed.
	settled := &appv1alpha1.App{}
	settled.Status.ReleaseGeneration, settled.Status.PendingReleaseGeneration = 5, 5
	if res, err := r.runningRequeue(context.Background(), settled, false); err != nil || res.RequeueAfter != childHealthRequeue {
		t.Errorf("a settled app requeues after %v (err %v), want the %v health beat", res.RequeueAfter, err, childHealthRequeue)
	}
	if _, owed := owedReleaseRequeue(settled); owed {
		t.Error("an adopted release reads as still owed")
	}
}
