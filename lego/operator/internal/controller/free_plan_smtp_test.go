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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m127: every pod a free-plan App runs carries the label the platform's
// deny-free-plan-smtp-egress policy selects; a paid App's pods carry none, so
// their templates stay byte-identical.

func TestFreePlanLabelOnServicePods(t *testing.T) {
	for tier, want := range map[string]bool{tierFree: true, "": true, "starter": false, "pro": false} {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = tier })
		labels := project(app, deploymentParams{}).Spec.Template.Labels
		if got := labels[execution.LabelFreePlan] == execution.FreePlanValue; got != want {
			t.Errorf("tier %q: free-plan label present = %v, want %v (%v)", tier, got, want, labels)
		}
		if !want {
			if _, ok := labels[execution.LabelFreePlan]; ok {
				t.Errorf("tier %q: paid template carries the label", tier)
			}
		}
	}
}

func TestFreePlanLabelLeavesPaidPolicyLabelsUntouched(t *testing.T) {
	base := map[string]string{"k": "v"}
	paid := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = "starter" })
	if got := appPolicyLabels(paid, base); !reflect.DeepEqual(got, base) {
		t.Fatalf("paid labels changed: %v", got)
	}
	free := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = tierFree })
	got := appPolicyLabels(free, base)
	if got[execution.LabelFreePlan] != execution.FreePlanValue || got["k"] != "v" {
		t.Fatalf("free labels = %v", got)
	}
	if _, ok := got[execution.LabelContainerPolicy]; ok {
		t.Fatalf("free strict App gained the container-policy marker: %v", got)
	}
	if _, leaked := base[execution.LabelFreePlan]; leaked {
		t.Fatal("the caller's map was mutated")
	}
}

func TestFreePlanLabelOnCronAndPreDeployPods(t *testing.T) {
	r := &AppReconciler{}
	for tier, want := range map[string]bool{tierFree: true, "starter": false} {
		app := projectionApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeCronJob
			a.Spec.Tier = tier
		})
		cron := r.cronPodSpec(app, "img:1", 0, map[string]string{labelApp: app.Name}, launchUnknown)
		if got := cron.Labels[execution.LabelFreePlan] == execution.FreePlanValue; got != want {
			t.Errorf("cron tier %q: label = %v, want %v", tier, got, want)
		}
		job := predeploy.Job(predeploy.Options{Name: app.Name, Namespace: "default", Revision: "gen-1", Command: "true", FreePlan: tier == tierFree})
		if got := job.Spec.Template.Labels[execution.LabelFreePlan] == execution.FreePlanValue; got != want {
			t.Errorf("pre-deploy tier %q: label = %v, want %v", tier, got, want)
		}
	}
}

// A held release's served template follows a plan change in both directions.
func TestSyncPolicyLabelsFollowsThePlan(t *testing.T) {
	tpl := &corev1.PodTemplateSpec{}
	tpl.Labels = map[string]string{labelApp: "hello"}
	free := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = tierFree })
	paid := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = "starter" })
	if !syncPolicyLabels(tpl, free) || tpl.Labels[execution.LabelFreePlan] != execution.FreePlanValue {
		t.Fatalf("paid → free: labels %v", tpl.Labels)
	}
	if syncPolicyLabels(tpl, free) {
		t.Fatal("an unchanged free plan reported a change")
	}
	if !syncPolicyLabels(tpl, paid) {
		t.Fatal("free → paid reported no change")
	}
	if _, ok := tpl.Labels[execution.LabelFreePlan]; ok || tpl.Labels[labelApp] != "hello" {
		t.Fatalf("free → paid: labels %v", tpl.Labels)
	}
	if syncPolicyLabels(tpl, paid) {
		t.Fatal("an unchanged paid plan reported a change")
	}
}
