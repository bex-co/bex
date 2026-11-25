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

	corev1 "k8s.io/api/core/v1"

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
