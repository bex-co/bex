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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A disk-bearing service's Recreate rollout (w6/m547): its next instance
// prefers the volume's node, and its startup probe runs every second, each
// adopted only on a pass that rolls anyway.

func diskProjectionApp(mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	return projectionApp(append([]func(*appv1alpha1.App){func(a *appv1alpha1.App) {
		a.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1}
	}}, mutate...)...)
}

func restartedAt(at string) func(*appv1alpha1.App) {
	return func(a *appv1alpha1.App) { a.Spec.RestartedAt = at }
}

func diskParams(node string) deploymentParams {
	p := webParams()
	p.replicas = 1
	p.diskNode = node
	return p
}

// preferredNodes is every node a template's node affinity prefers, and
// requiredNode whether its required terms name a node by name.
func preferredNodes(spec corev1.PodSpec) (nodes []string, requiredNode bool) {
	if spec.Affinity == nil || spec.Affinity.NodeAffinity == nil {
		return nil, false
	}
	na := spec.Affinity.NodeAffinity
	for _, term := range na.PreferredDuringSchedulingIgnoredDuringExecution {
		for _, f := range term.Preference.MatchFields {
			nodes = append(nodes, f.Values...)
		}
	}
	if req := na.RequiredDuringSchedulingIgnoredDuringExecution; req != nil {
		for _, term := range req.NodeSelectorTerms {
			requiredNode = requiredNode || len(term.MatchFields) > 0
		}
	}
	return nodes, requiredNode
}

func TestDiskServiceProbesStartupEverySecondWithTheSameBudgets(t *testing.T) {
	disk := appContainerOf(t, project(diskProjectionApp(), diskParams("")))
	if got := disk.StartupProbe.PeriodSeconds; got != 1 {
		t.Errorf("disk startup period = %ds, want 1s: under Recreate every second before the first passing check is downtime", got)
	}
	if got := disk.StartupProbe.PeriodSeconds * disk.StartupProbe.FailureThreshold; got != rolloutBudgetSeconds {
		t.Errorf("disk startup budget = %ds, want the %ds rollout budget", got, rolloutBudgetSeconds)
	}
	if r := disk.ReadinessProbe; r.PeriodSeconds != healthCheckReadinessPeriodSeconds || r.FailureThreshold != healthCheckReadinessFailureThreshold {
		t.Errorf("disk readiness = %ds × %d, want the unchanged %ds × %d", r.PeriodSeconds, r.FailureThreshold,
			healthCheckReadinessPeriodSeconds, healthCheckReadinessFailureThreshold)
	}
	if l := disk.LivenessProbe; l.PeriodSeconds != healthCheckPeriodSeconds || l.PeriodSeconds*l.FailureThreshold != livenessRestartWindowSeconds {
		t.Errorf("disk liveness = %ds × %d, want the unchanged %ds period and %ds window", l.PeriodSeconds, l.FailureThreshold,
			healthCheckPeriodSeconds, livenessRestartWindowSeconds)
	}

	stateless := appContainerOf(t, project(projectionApp(), webParams()))
	if s := stateless.StartupProbe; s.PeriodSeconds != healthCheckPeriodSeconds || s.FailureThreshold != healthCheckStartupFailureThreshold {
		t.Errorf("stateless startup = %ds × %d, want the unchanged %ds × %d", s.PeriodSeconds, s.FailureThreshold,
			healthCheckPeriodSeconds, healthCheckStartupFailureThreshold)
	}
}

// The operator upgrade that ships the 1s period must not Recreate every disk
// service at once: a stored 10s template keeps it until the next roll.
func TestDiskStartupPeriodIsAdoptedLazily(t *testing.T) {
	app := diskProjectionApp()
	stored := project(app, diskParams(""))
	probe := stored.Spec.Template.Spec.Containers[0].StartupProbe
	probe.PeriodSeconds, probe.FailureThreshold = healthCheckPeriodSeconds, healthCheckStartupFailureThreshold

	upgraded := stored.DeepCopy()
	applyDeploymentSpec(upgraded, app, diskParams(""))
	if !equalPodTemplate(upgraded.Spec.Template, stored.Spec.Template) {
		t.Fatal("an unchanged disk App's template changed on upgrade; that would Recreate every disk service at once")
	}

	next := stored.DeepCopy()
	applyDeploymentSpec(next, diskProjectionApp(restartedAt("2026-11-21T00:00:00Z")), diskParams(""))
	if got := appContainerOf(t, next).StartupProbe.PeriodSeconds; got != diskStartupPeriodSeconds {
		t.Errorf("the rolling pass startup period = %ds, want %ds", got, diskStartupPeriodSeconds)
	}

	// Detaching the disk rolls the template anyway and restores 10s.
	detached := next.DeepCopy()
	applyDeploymentSpec(detached, projectionApp(restartedAt("2026-11-21T00:00:00Z")), webParams())
	if got := appContainerOf(t, detached).StartupProbe.PeriodSeconds; got != healthCheckPeriodSeconds {
		t.Errorf("after detaching the disk the startup period = %ds, want %ds", got, healthCheckPeriodSeconds)
	}
}

func TestDiskInstancePrefersItsVolumesNodeOnlyWhenItRolls(t *testing.T) {
	// A first deploy has no volume to follow.
	first := project(diskProjectionApp(), diskParams(""))
	if nodes, _ := preferredNodes(first.Spec.Template.Spec); len(nodes) != 0 {
		t.Fatalf("first deploy prefers %v, want no node", nodes)
	}

	// The volume is on node-a now. A pass that changes nothing else leaves
	// the template alone: a hint that rolled the pod would be the downtime
	// it exists to shorten.
	steady := first.DeepCopy()
	applyDeploymentSpec(steady, diskProjectionApp(), diskParams("node-a"))
	if !equalPodTemplate(steady.Spec.Template, first.Spec.Template) {
		t.Fatal("learning the volume's node changed an unchanged template; that rolls the instance")
	}

	// A deploy, restart or plan change rolls the template, and the next
	// instance prefers node-a, as a preference only.
	rolled := first.DeepCopy()
	applyDeploymentSpec(rolled, diskProjectionApp(restartedAt("2026-11-21T01:00:00Z")), diskParams("node-a"))
	nodes, required := preferredNodes(rolled.Spec.Template.Spec)
	if len(nodes) != 1 || nodes[0] != "node-a" {
		t.Fatalf("rolled template prefers %v, want [node-a]", nodes)
	}
	if required {
		t.Fatal("the volume's node is required; a cordoned, full or gone node would leave the service Pending")
	}
	if storedDiskNode(rolled.Spec.Template.Spec) != "node-a" {
		t.Fatal("storedDiskNode does not read back the preference it wrote")
	}

	// The instance moved to node-b (node-a was cordoned): no roll for that
	// alone, and the next roll follows the volume to node-b.
	moved := rolled.DeepCopy()
	applyDeploymentSpec(moved, diskProjectionApp(restartedAt("2026-11-21T01:00:00Z")), diskParams("node-b"))
	if !equalPodTemplate(moved.Spec.Template, rolled.Spec.Template) {
		t.Fatal("the volume moving changed an unchanged template")
	}
	applyDeploymentSpec(moved, diskProjectionApp(restartedAt("2026-11-21T02:00:00Z")), diskParams("node-b"))
	if nodes, _ := preferredNodes(moved.Spec.Template.Spec); len(nodes) != 1 || nodes[0] != "node-b" {
		t.Fatalf("after the move the rolled template prefers %v, want [node-b]", nodes)
	}

	// Mid-rollout no instance has a node yet: a second deploy keeps the
	// preference the stored template carries instead of dropping it.
	pending := rolled.DeepCopy()
	applyDeploymentSpec(pending, diskProjectionApp(restartedAt("2026-11-21T03:00:00Z")), diskParams(""))
	if nodes, _ := preferredNodes(pending.Spec.Template.Spec); len(nodes) != 1 || nodes[0] != "node-a" {
		t.Fatalf("with no node known the rolled template prefers %v, want the stored [node-a]", nodes)
	}

	// Detaching the disk drops the preference with the volume.
	detached := rolled.DeepCopy()
	applyDeploymentSpec(detached, projectionApp(restartedAt("2026-11-21T04:00:00Z")), diskParams("node-a"))
	if nodes, _ := preferredNodes(detached.Spec.Template.Spec); len(nodes) != 0 {
		t.Fatalf("a diskless App prefers %v, want no node", nodes)
	}
}

// A dedicated serving pool's required term survives beside the preference.
func TestDiskNodePreferenceKeepsTheServingPoolRequirement(t *testing.T) {
	app := diskProjectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = "pro-plus" })
	dep := project(app, diskParams("node-a"))
	na := dep.Spec.Template.Spec.Affinity.NodeAffinity
	if na.RequiredDuringSchedulingIgnoredDuringExecution == nil || len(na.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) != 1 {
		t.Fatalf("required = %+v, want the serving pool's term", na.RequiredDuringSchedulingIgnoredDuringExecution)
	}
	if nodes, _ := preferredNodes(dep.Spec.Template.Spec); len(nodes) != 1 || nodes[0] != "node-a" {
		t.Fatalf("pooled disk App prefers %v, want [node-a]", nodes)
	}
}

func TestDiskNodeReadsTheServingInstancesNode(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	app := diskApp("blog", &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1})
	pod := func(name, node string, owner string, mutate ...func(*corev1.Pod)) *corev1.Pod {
		p := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace, Labels: map[string]string{labelApp: app.Name}},
			Spec:       corev1.PodSpec{NodeName: node},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		}
		if owner != "" {
			p.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: owner, Name: "x", UID: "u", Controller: new(true)}}
		}
		for _, m := range mutate {
			m(p)
		}
		return p
	}
	deleting := func(p *corev1.Pod) {
		now := metav1.Now()
		p.DeletionTimestamp = &now
		p.Finalizers = []string{"test/hold"}
	}
	lookup := func(objs ...client.Object) string {
		r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build(), Scheme: scheme}
		return r.diskNode(context.Background(), app)
	}
	jobPod := pod("predeploy", "node-z", "Job")
	if got := lookup(jobPod, pod("old", "node-b", "ReplicaSet", deleting), pod("serving", "node-a", "ReplicaSet")); got != "node-a" {
		t.Errorf("diskNode = %q, want the serving instance's node-a, not a Job's or a terminating one's", got)
	}
	if got := lookup(jobPod, pod("old", "node-b", "ReplicaSet", deleting), pod("new", "", "ReplicaSet")); got != "node-b" {
		t.Errorf("mid-rollout diskNode = %q, want node-b, where the replaced instance still holds the volume", got)
	}
	if got := lookup(jobPod); got != "" {
		t.Errorf("diskNode with no instance = %q, want none", got)
	}

	stateless := app.DeepCopy()
	stateless.Spec.Disk = nil
	r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod("serving", "node-a", "ReplicaSet")).Build(), Scheme: scheme}
	if got := r.diskNode(context.Background(), stateless); got != "" {
		t.Errorf("diskless diskNode = %q, want none", got)
	}
}
