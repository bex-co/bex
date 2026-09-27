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

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/028: a failing `* * * * *` cron ran its command 7 times over ~11 minutes
// (Kubernetes' default backoffLimit 6) and ForbidConcurrent skipped every tick
// in between. Scheduled and manual runs are one execution each.
func TestCronRunsExecuteOnce(t *testing.T) {
	ctx := context.Background()
	scheme := wakeScheme()
	app := activeApp("tea-cron-once")
	app.UID = "cron-once-uid"
	app.Spec.Type = appv1alpha1.TypeCronJob
	app.Spec.Tier = ""
	app.Spec.Expose = false
	app.Spec.Schedule = "* * * * *"
	app.Spec.Command = "sh -c 'echo qa40-run; exit 3'"
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
	r := wakeReconciler(cl, scheme)
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)

	var cron batchv1.CronJob
	if err := cl.Get(ctx, nn, &cron); err != nil {
		t.Fatalf("CronJob: %v", err)
	}
	if b := cron.Spec.JobTemplate.Spec.BackoffLimit; b == nil || *b != 0 {
		t.Fatalf("scheduled run backoffLimit = %v, want 0", b)
	}

	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.RunAt = "2026-09-27T01:00:00Z"
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	var job batchv1.Job
	if err := cl.Get(ctx, types.NamespacedName{Namespace: nn.Namespace, Name: manualRunJobName(app.Name, live.Spec.RunAt)}, &job); err != nil {
		t.Fatalf("manual run Job: %v", err)
	}
	if b := job.Spec.BackoffLimit; b == nil || *b != 0 {
		t.Fatalf("manual run backoffLimit = %v, want 0", b)
	}
}
