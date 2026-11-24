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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/operator/internal/drainguard"
)

// spreadOf returns the template's one topology spread, or nil for none.
func spreadOf(t *testing.T, dep *appsv1.Deployment) *corev1.TopologySpreadConstraint {
	t.Helper()
	switch got := dep.Spec.Template.Spec.TopologySpreadConstraints; len(got) {
	case 0:
		return nil
	case 1:
		return &got[0]
	default:
		t.Fatalf("topology spreads = %d; bex writes at most one", len(got))
		return nil
	}
}

func withDisk(a *appv1alpha1.App) {
	a.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 1}
}

// w21/m617: a service that can run more than one instance carries the spread
// whatever its count today, so a later scale needs no template change. A disk
// service (one instance, Recreate) and a Free one (capped at one) carry none.
func TestInstanceSpreadIsCarriedByEveryServiceThatCanScale(t *testing.T) {
	for _, tc := range []struct {
		name     string
		replicas int32
		worker   bool
		mutate   func(*appv1alpha1.App)
		want     bool
	}{
		{"paid web service at two instances", 2, false, func(a *appv1alpha1.App) { a.Spec.Tier = "standard" }, true},
		{"paid web service at one instance", 1, false, func(a *appv1alpha1.App) { a.Spec.Tier = "starter" }, true},
		{"autoscaled service", 1, false, func(a *appv1alpha1.App) {
			a.Spec.Tier = "pro"
			a.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: 1, MaxReplicas: 3, TargetCPUPercent: new(int32(60))}
		}, true},
		{"private service", 2, false, func(a *appv1alpha1.App) { a.Spec.Tier = "standard"; a.Spec.Type = appv1alpha1.TypePrivateService }, true},
		{"background worker", 2, true, func(a *appv1alpha1.App) { a.Spec.Tier = "standard"; a.Spec.Type = appv1alpha1.TypeBackgroundWorker }, true},
		{"dedicated-pool plan", 2, false, func(a *appv1alpha1.App) { a.Spec.Tier = "pro-plus" }, true},
		{"untiered App", 2, false, func(*appv1alpha1.App) {}, true},
		{"Free plan", 1, false, func(a *appv1alpha1.App) { a.Spec.Tier = "free" }, false},
		{"service with a disk", 1, false, func(a *appv1alpha1.App) { a.Spec.Tier = "standard"; withDisk(a) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := webParams()
			p.replicas, p.worker = tc.replicas, tc.worker
			if got := spreadOf(t, project(projectionApp(tc.mutate), p)) != nil; got != tc.want {
				t.Fatalf("carries the instance spread = %v, want %v", got, tc.want)
			}
		})
	}
}

// The spread must count exactly this service's pods of one revision: its
// selector selects the template's own labels and no other service's, and the
// revision comes from pod-template-hash, which the ReplicaSet adds to each pod.
func TestInstanceSpreadCountsOnlyThisServicesPodsOfOneRevision(t *testing.T) {
	app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = "standard" })
	dep := project(app, webParams())
	spread := spreadOf(t, dep)
	if spread == nil {
		t.Fatal("a paid web service carries no instance spread")
	}
	selector, err := metav1.LabelSelectorAsSelector(spread.LabelSelector)
	if err != nil {
		t.Fatal(err)
	}
	if !selector.Matches(labels.Set(dep.Spec.Template.Labels)) {
		t.Errorf("selector %s does not select the service's own pods %v", selector, dep.Spec.Template.Labels)
	}
	other := project(projectionApp(func(a *appv1alpha1.App) { a.Name = "other" }), webParams())
	if selector.Matches(labels.Set(other.Spec.Template.Labels)) {
		t.Errorf("selector %s also selects another service's pods %v", selector, other.Spec.Template.Labels)
	}
	if got := spread.MatchLabelKeys; len(got) != 1 || got[0] != appsv1.DefaultDeploymentUniqueLabelKey {
		t.Errorf("matchLabelKeys = %v, want [%s] so a rolling deploy's old pods never block its new ones", got, appsv1.DefaultDeploymentUniqueLabelKey)
	}
	if spread.TopologyKey != corev1.LabelHostname || spread.MaxSkew != 1 || spread.WhenUnsatisfiable != corev1.DoNotSchedule {
		t.Errorf("spread = %s maxSkew %d %s, want a hard one-per-node spread on %s",
			spread.TopologyKey, spread.MaxSkew, spread.WhenUnsatisfiable, corev1.LabelHostname)
	}
	if p := spread.NodeTaintsPolicy; p == nil || *p != corev1.NodeInclusionPolicyHonor {
		t.Errorf("nodeTaintsPolicy = %v, want Honor: counted as empty, the tainted nodes would refuse every node a second instance", p)
	}
}

// DoD 3: the operator upgrade that ships the spread rolls nothing. A template
// stored without it keeps it absent through the upgrade and through a scale
// (replicas are not in the template), and gains it on the next pass that
// changes the template for its own reason.
func TestInstanceSpreadIsAdoptedOnTheNextRollNotOnUpgradeOrScale(t *testing.T) {
	app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = "standard" })
	p := webParams()
	p.replicas = 1
	// What the operator before w21/m617 stored: today's projection, no spread.
	stored := project(app, p)
	stored.Spec.Template.Spec.TopologySpreadConstraints = nil

	upgraded := stored.DeepCopy()
	applyDeploymentSpec(upgraded, app, p)
	if !equalPodTemplate(upgraded.Spec.Template, stored.Spec.Template) {
		t.Fatal("an unchanged service's template changed on the upgrade; that would roll every service at once")
	}

	scaled := stored.DeepCopy()
	p.replicas = 3
	applyDeploymentSpec(scaled, app, p)
	if !equalPodTemplate(scaled.Spec.Template, stored.Spec.Template) {
		t.Fatal("a scale changed the template; a scale must roll nothing (ADR018 Manual scale)")
	}
	if got := *scaled.Spec.Replicas; got != 3 {
		t.Fatalf("replicas = %d, want the scale's 3", got)
	}

	restarted := projectionApp(func(a *appv1alpha1.App) {
		a.Spec.Tier = "standard"
		a.Spec.RestartedAt = "2026-11-24T00:00:00Z"
	})
	next := scaled.DeepCopy()
	applyDeploymentSpec(next, restarted, p)
	if spreadOf(t, next) == nil {
		t.Fatal("the next roll must adopt the instance spread")
	}

	again := next.DeepCopy()
	applyDeploymentSpec(again, restarted, p)
	if !equalPodTemplate(again.Spec.Template, next.Spec.Template) {
		t.Error("once adopted, the spread template must re-project unchanged")
	}
}

// A template already carrying the spread keeps it through a pass that does
// not change the service, and loses it with the change that makes the service
// unable to scale: the disk attach or the move to Free, each of which rolls the
// template anyway.
func TestInstanceSpreadLeavesWithTheChangeThatEndsScaling(t *testing.T) {
	p := webParams()
	p.replicas = 1
	stored := project(projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = "standard" }), p)
	if spreadOf(t, stored) == nil {
		t.Fatal("a paid web service carries no instance spread")
	}

	for name, mutate := range map[string]func(*appv1alpha1.App){
		"disk attached": func(a *appv1alpha1.App) { a.Spec.Tier = "standard"; withDisk(a) },
		"moved to Free": func(a *appv1alpha1.App) { a.Spec.Tier = "free" },
	} {
		t.Run(name, func(t *testing.T) {
			next := stored.DeepCopy()
			applyDeploymentSpec(next, projectionApp(mutate), p)
			if spreadOf(t, next) != nil {
				t.Error("a service that can no longer scale still carries the instance spread")
			}
		})
	}
}

// drain-guard evacuates a service off a node by writing its annotation into
// the stored template, which rolls it once (ADR004 § Node rolls). The
// operator's next pass must keep that template as drain-guard wrote it,
// whether or not it carries the spread yet: reverting the annotation, or
// adding the spread on top of it, would roll the service a second time.
func TestDrainGuardsEvacuationKeepsItsTemplate(t *testing.T) {
	app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = "starter" })
	p := webParams()
	p.replicas = 1
	for name, strip := range map[string]bool{"stored before the spread": true, "carrying the spread": false} {
		t.Run(name, func(t *testing.T) {
			stored := project(app, p)
			if strip {
				stored.Spec.Template.Spec.TopologySpreadConstraints = nil
			}
			if stored.Spec.Template.Annotations == nil {
				stored.Spec.Template.Annotations = map[string]string{}
			}
			stored.Spec.Template.Annotations[drainguard.EvacuatedFromAnnotation] = "bex-tenant-0-a"

			next := stored.DeepCopy()
			applyDeploymentSpec(next, app, p)
			if !equalPodTemplate(next.Spec.Template, stored.Spec.Template) {
				t.Fatal("the pass after drain-guard's evacuation changed its template; that rolls the service twice")
			}
		})
	}
}
