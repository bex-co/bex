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
	"testing"

	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m640: on Render, background workers, cron jobs and free instances "can
// send private network requests, but they can't receive them"
// (render.com/docs/private-network). Every pod that must not receive carries
// the label the platform's deny-send-only-private-ingress policy selects; a
// paid web or private service's pods carry none.

func sendOnly(labels map[string]string) bool {
	return labels[execution.LabelPrivateNetwork] == execution.PrivateNetworkSendOnly
}

func TestPrivateNetworkLabelOnServicePods(t *testing.T) {
	for _, tc := range []struct {
		typ, tier string
		want      bool
	}{
		{appv1alpha1.TypeBackgroundWorker, "starter", true},
		{appv1alpha1.TypeBackgroundWorker, "pro", true},
		{appv1alpha1.TypeWebService, tierFree, true},
		{appv1alpha1.TypeWebService, "", true},
		{"", tierFree, true},
		{appv1alpha1.TypePrivateService, tierFree, true},
		{appv1alpha1.TypeWebService, "starter", false},
		{"", "standard", false},
		{appv1alpha1.TypePrivateService, "starter", false},
		{appv1alpha1.TypePrivateService, "pro-ultra", false},
	} {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Type, a.Spec.Tier = tc.typ, tc.tier })
		labels := project(app, deploymentParams{}).Spec.Template.Labels
		if got := sendOnly(labels); got != tc.want {
			t.Errorf("%q on %q: send-only label = %v, want %v (%v)", tc.typ, tc.tier, got, tc.want, labels)
		}
		if _, present := labels[execution.LabelPrivateNetwork]; !tc.want && present {
			t.Errorf("%q on %q: a receiving service's template carries the label", tc.typ, tc.tier)
		}
	}
}

// A paid web or private service's template stays byte-identical, so the
// operator upgrade rolls none of them.
func TestPrivateNetworkLabelLeavesReceivingServicesUntouched(t *testing.T) {
	base := map[string]string{"k": "v"}
	for _, typ := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService} {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Type, a.Spec.Tier = typ, "starter" })
		if got := appPolicyLabels(app, base); len(got) != 1 || got["k"] != "v" {
			t.Errorf("%s on starter: labels = %v, want the caller's alone", typ, got)
		}
	}
	worker := projectionApp(func(a *appv1alpha1.App) { a.Spec.Type, a.Spec.Tier = appv1alpha1.TypeBackgroundWorker, "starter" })
	got := appPolicyLabels(worker, base)
	if !sendOnly(got) || got["k"] != "v" {
		t.Fatalf("paid worker labels = %v", got)
	}
	if _, ok := got[execution.LabelFreePlan]; ok {
		t.Fatalf("a paid worker gained the free-plan label: %v", got)
	}
	if _, leaked := base[execution.LabelPrivateNetwork]; leaked {
		t.Fatal("the caller's map was mutated")
	}
}

// A cron run sends only on every plan; a pre-deploy Job sends only whatever
// its service's type or plan.
func TestPrivateNetworkLabelOnCronAndPreDeployPods(t *testing.T) {
	r := &AppReconciler{}
	for _, tier := range []string{tierFree, "starter", "pro"} {
		cronApp := projectionApp(func(a *appv1alpha1.App) { a.Spec.Type, a.Spec.Tier = appv1alpha1.TypeCronJob, tier })
		cron := r.cronPodSpec(cronApp, "img:1", 0, map[string]string{labelApp: cronApp.Name}, launchUnknown, "")
		if !sendOnly(cron.Labels) {
			t.Errorf("cron run on %q: labels %v", tier, cron.Labels)
		}
	}
	for _, free := range []bool{false, true} {
		job := predeploy.Job(predeploy.Options{Name: "web", Namespace: "default", Revision: "gen-1", Command: []string{"true"}, FreePlan: free})
		if !sendOnly(job.Spec.Template.Labels) {
			t.Errorf("pre-deploy Job (free=%v): labels %v", free, job.Spec.Template.Labels)
		}
		if _, onJob := job.Labels[execution.LabelPrivateNetwork]; onJob {
			t.Errorf("pre-deploy Job (free=%v): the Job object itself carries the pod label", free)
		}
	}
}

// A cron run is a tenant source like a service's pod: it carries its
// workspace label, by which the send-only and free-plan ingress denies know a
// tenant source, so it cannot reach a background worker or a free web service.
// Without it, a manual run on production opened a worker's port by its pod IP
// and its <slug>-discovery name, and a free web service's internal address
// (w6/m640 t004, 2026-11-26). Manual runs take the CronJob's template.
func TestCronRunPodsCarryTheWorkspaceLabel(t *testing.T) {
	g := NewWithT(t)
	app := activeApp("tea-cron-workspace")
	app.UID = "cron-workspace-uid"
	app.Spec.Type = appv1alpha1.TypeCronJob
	app.Spec.Tier = "starter"
	app.Spec.Expose = false
	app.Spec.Schedule = "* * * * *"
	app.Spec.Command = "echo sends"
	scheme := wakeScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, wakeReconciler(cl, scheme), nn)
	var cron batchv1.CronJob
	g.Expect(cl.Get(t.Context(), nn, &cron)).To(Succeed())
	labels := cron.Spec.JobTemplate.Spec.Template.Labels
	g.Expect(labels).To(HaveKeyWithValue(labelWorkspace, "tea-cron-workspace"))
	g.Expect(sendOnly(labels)).To(BeTrue(), "a cron run still receives nothing: %v", labels)

}

// A held release's served template follows a plan change: a move from Free to
// Starter drops both network policy labels, and a move back restores them. A
// worker's label stays through any plan change.
func TestSyncPolicyLabelsFollowReceiving(t *testing.T) {
	tpl := &corev1.PodTemplateSpec{}
	tpl.Labels = map[string]string{labelApp: "hello"}
	free := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = tierFree })
	paid := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = "starter" })
	if !syncPolicyLabels(tpl, free) || !sendOnly(tpl.Labels) || tpl.Labels[execution.LabelFreePlan] != execution.FreePlanValue {
		t.Fatalf("→ free: labels %v", tpl.Labels)
	}
	if syncPolicyLabels(tpl, free) {
		t.Fatal("an unchanged free plan reported a change")
	}
	if !syncPolicyLabels(tpl, paid) {
		t.Fatal("free → starter reported no change")
	}
	if len(tpl.Labels) != 1 || tpl.Labels[labelApp] != "hello" {
		t.Fatalf("free → starter: labels %v, want only the App label", tpl.Labels)
	}
	if syncPolicyLabels(tpl, paid) {
		t.Fatal("an unchanged starter plan reported a change")
	}

	worker := &corev1.PodTemplateSpec{}
	for _, tier := range []string{tierFree, "starter", "pro"} {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Type, a.Spec.Tier = appv1alpha1.TypeBackgroundWorker, tier })
		syncPolicyLabels(worker, app)
		if !sendOnly(worker.Labels) {
			t.Fatalf("worker on %q: labels %v", tier, worker.Labels)
		}
	}
}

// A served release's template restored over a canceled deploy takes the labels
// of the plan it runs, read from its app container's resources, whatever plan
// the canceled release saved: a record from before w6/m640 has no send-only
// label, and a canceled move from Free to Starter still runs Free.
func TestSyncRestoredPolicyLabelsFollowTheServedPlan(t *testing.T) {
	restored := func(tier string, labels map[string]string) *corev1.PodTemplateSpec {
		tpl := &corev1.PodTemplateSpec{}
		tpl.Labels = labels
		tpl.Spec.Containers = []corev1.Container{{Name: appContainerName, Resources: resourcesForTier(tier)}}
		return tpl
	}
	worker := projectionApp(func(a *appv1alpha1.App) { a.Spec.Type, a.Spec.Tier = appv1alpha1.TypeBackgroundWorker, "starter" })
	tpl := restored("starter", map[string]string{labelApp: "w"})
	if !syncRestoredPolicyLabels(tpl, worker) || !sendOnly(tpl.Labels) {
		t.Fatalf("a worker's pre-label record: labels %v", tpl.Labels)
	}
	if syncRestoredPolicyLabels(tpl, worker) {
		t.Fatal("an already labeled template reported a change")
	}

	toStarter := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = "starter" })
	tpl = restored(tierFree, map[string]string{labelApp: "web", execution.LabelFreePlan: execution.FreePlanValue})
	syncRestoredPolicyLabels(tpl, toStarter)
	if !sendOnly(tpl.Labels) || tpl.Labels[execution.LabelFreePlan] != execution.FreePlanValue {
		t.Fatalf("a canceled Free → Starter still runs Free: labels %v", tpl.Labels)
	}
	if toStarter.Spec.Tier != "starter" {
		t.Fatal("the App itself was changed")
	}

	toFree := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = tierFree })
	tpl = restored("starter", map[string]string{labelApp: "web"})
	if syncRestoredPolicyLabels(tpl, toFree) || len(tpl.Labels) != 1 {
		t.Fatalf("a canceled Starter → Free still runs Starter and receives: labels %v", tpl.Labels)
	}
}
