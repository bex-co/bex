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
	"maps"
	"reflect"
	"testing"

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m712: every pod a service, Key Value or Postgres runs carries the region
// the resource was created with, so bex-api's per-region private-network
// policies keep it in its own region's network, and a resource created
// without one carries none (bex-api counts it in the platform's region).

func regionAnnotated(obj metav1.Object, region string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[appv1alpha1.AnnotationRegion] = region
	obj.SetAnnotations(annotations)
}

func TestRegionComesFromTheAnnotationAlone(t *testing.T) {
	app := &appv1alpha1.App{}
	if got := regionOf(app); got != "" {
		t.Fatalf("an unstamped App's region = %q, want none", got)
	}
	// A label on the CR is not the region: bex-api stamps the annotation.
	app.Labels = map[string]string{labelRegion: "oregon", labelEnvironment: "evm-a"}
	if got := inPlaceLabelsOf(app); got[labelRegion] != "" || got[labelEnvironment] != "evm-a" {
		t.Fatalf("in-place labels of an unstamped App = %v", got)
	}
	regionAnnotated(app, "frankfurt")
	if got := inPlaceLabelsOf(app); got[labelRegion] != "frankfurt" || got[labelEnvironment] != "evm-a" {
		t.Fatalf("in-place labels of a frankfurt App = %v", got)
	}
	if app.Labels[labelRegion] != "oregon" {
		t.Fatal("inPlaceLabelsOf mutated the CR's labels")
	}
	// Only a hand-written CR can hold a value no label may carry.
	regionAnnotated(app, "not a region!")
	if got := regionOf(app); got != "" {
		t.Fatalf("an invalid annotation became region %q", got)
	}
}

// BEX_REGION arrives in its placement spelling; the private network selects
// by Render regions. The mapping is resourcemeta.RenderRegion's twin: Render
// regions pass through, "fsn1" reads back "frankfurt", and anything else
// reads back nothing (bex-api with BEX_REGION unset).
func TestPlatformRenderRegionNormalization(t *testing.T) {
	for raw, want := range map[string]string{
		"frankfurt": "frankfurt", "ohio": "ohio", "oregon": "oregon", "singapore": "singapore", "virginia": "virginia",
		"fsn1": "frankfurt", "  fsn1  ": "frankfurt",
		"": "", "fra1": "", "mars": "",
	} {
		if got := PlatformRenderRegion(raw); got != want {
			t.Errorf("PlatformRenderRegion(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestServicePodTemplatesCarryTheRegion(t *testing.T) {
	stamped := projectionApp(func(a *appv1alpha1.App) { regionAnnotated(a, "oregon") })
	dep := project(stamped, deploymentParams{})
	if got := dep.Spec.Template.Labels[labelRegion]; got != "oregon" {
		t.Errorf("an oregon service's template labels = %v", dep.Spec.Template.Labels)
	}
	if _, ok := dep.Spec.Selector.MatchLabels[labelRegion]; ok {
		t.Error("the region must stay out of the immutable selector")
	}
	if got := appTemplateEnvironmentLabels(stamped); got[labelRegion] != "oregon" {
		t.Errorf("the held template labels lack the region: %v", got)
	}
	// An unstamped service's template is what it was before the region, so
	// the operator upgrade rolls none of production's services.
	unstamped := project(projectionApp(), deploymentParams{})
	if _, ok := unstamped.Spec.Template.Labels[labelRegion]; ok {
		t.Errorf("an unstamped service's template carries a region: %v", unstamped.Spec.Template.Labels)
	}

	for _, region := range []string{"", "singapore"} {
		job := predeploy.Job(predeploy.Options{Name: "web", Namespace: "default", Revision: "gen-1", Command: []string{"true"}, Region: region})
		got, ok := job.Spec.Template.Labels[labelRegion]
		if region == "" && ok || region != "" && got != region {
			t.Errorf("pre-deploy pod for region %q: labels %v", region, job.Spec.Template.Labels)
		}
	}
}

func TestCronRunPodsCarryTheRegion(t *testing.T) {
	g := NewWithT(t)
	app := activeApp("tea-cron-region")
	app.UID = "cron-region-uid"
	app.Spec.Type = appv1alpha1.TypeCronJob
	app.Spec.Tier = "starter"
	app.Spec.Expose = false
	app.Spec.Schedule = "* * * * *"
	app.Spec.Command = "echo sends"
	regionAnnotated(app, "virginia")
	scheme := wakeScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, wakeReconciler(cl, scheme), nn)
	var cron batchv1.CronJob
	g.Expect(cl.Get(t.Context(), nn, &cron)).To(Succeed())
	g.Expect(cron.Spec.JobTemplate.Spec.Template.Labels).To(HaveKeyWithValue(labelRegion, "virginia"))
}

// A service whose Deployment predates the operator that reads its region (an
// App created in a region between w9/105 and w4/m712) keeps its template and
// its pods: the running pod gains the label in place, with its UID unchanged.
func TestAnExistingServiceGainsItsRegionInPlace(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	app := activeApp("tea-region-inplace")
	app.Spec.Tier = "starter"
	scheme := wakeScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := wakeReconciler(cl, scheme)
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)
	dep := &appsv1.Deployment{}
	g.Expect(cl.Get(ctx, nn, dep)).To(Succeed())
	if dep.CreationTimestamp.IsZero() { // the fake client does not stamp it
		dep.CreationTimestamp = metav1.Now()
		g.Expect(cl.Update(ctx, dep)).To(Succeed())
	}
	settled := dep.Spec.Template.DeepCopy()
	g.Expect(settled.Labels).NotTo(HaveKey(labelRegion))

	ctrl := true
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: app.Name + "-abc", Namespace: app.Namespace, UID: "running-pod",
		Labels:          maps.Clone(settled.Labels),
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: app.Name + "-rs", UID: "rs", Controller: &ctrl}},
	}, Spec: settled.Spec}
	g.Expect(cl.Create(ctx, pod)).To(Succeed())

	live := &appv1alpha1.App{}
	g.Expect(cl.Get(ctx, nn, live)).To(Succeed())
	regionAnnotated(live, "oregon")
	g.Expect(cl.Update(ctx, live)).To(Succeed())
	reconcileTwice(t, r, nn)

	g.Expect(cl.Get(ctx, nn, dep)).To(Succeed())
	g.Expect(dep.Spec.Template.Labels).To(Equal(settled.Labels), "the region alone must not roll the service")
	running := &corev1.Pod{}
	g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(pod), running)).To(Succeed())
	g.Expect(running.UID).To(Equal(types.UID("running-pod")))
	g.Expect(running.Labels).To(HaveKeyWithValue(labelRegion, "oregon"), "the running pod must gain its region in place")
}

func TestKeyValuePodsCarryTheRegion(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{
			Name: "red-region", Namespace: "tea-a", UID: "region-uid",
			Labels: map[string]string{labelWorkspace: "tea-a"},
		},
		Spec: appv1alpha1.KeyValueSpec{Plan: "starter"},
	}
	regionAnnotated(kv, "ohio")
	r := &KeyValueReconciler{Scheme: scheme, Backup: testKeyValueBackupStore, InternalProxyImage: "bex@sha256:fixture"}
	plan, storage := resolveKVPlan(kv.Spec)
	intent := keyValueIntentFor(kv, plan, storage, "", false, false)
	intent.authSecretName = "auth"
	sts := &appsv1.StatefulSet{}
	applyKeyValueStatefulSet(sts, kv, intent)
	for what, labels := range map[string]map[string]string{
		"server":         sts.Spec.Template.Labels,
		"internal proxy": keyValueInternalProxyPodLabels(kv),
		"backup":         r.keyValueBackupCronJobSpec(kv, plan, "platform").JobTemplate.Spec.Template.Labels,
		"backup purge":   r.keyValueBackupPurgeJob(kv).Spec.Template.Labels,
	} {
		if labels[labelRegion] != "ohio" {
			t.Errorf("%s pod labels = %v, want region ohio", what, labels)
		}
	}

	// A running server and proxy gain it in place.
	ctrl := true
	server := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: kv.Name + "-0", Namespace: kv.Namespace, UID: "server-pod",
		Labels:          map[string]string{labelKeyValue: kv.Name},
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "StatefulSet", Name: kv.Name, UID: "sts", Controller: &ctrl}},
	}}
	proxy := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: kv.Name + "-internal-abc", Namespace: kv.Namespace, UID: "proxy-pod",
		Labels:          map[string]string{labelKeyValueInternalProxy: kv.Name},
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "rs", UID: "rs", Controller: &ctrl}},
	}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, server, proxy).Build()
	r.Client = cl
	if err := r.syncKeyValuePodIsolation(ctx, kv); err != nil {
		t.Fatal(err)
	}
	for _, pod := range []*corev1.Pod{server, proxy} {
		got := &corev1.Pod{}
		if err := cl.Get(ctx, client.ObjectKeyFromObject(pod), got); err != nil {
			t.Fatal(err)
		}
		if got.Labels[labelRegion] != "ohio" || got.UID != pod.UID {
			t.Errorf("%s: labels %v, UID %s; want region ohio in place", pod.Name, got.Labels, got.UID)
		}
	}

	unstamped := kv.DeepCopy()
	unstamped.Annotations = nil
	if labels := keyValueIntentFor(unstamped, plan, storage, "", false, false).labels; labels[labelRegion] != "" {
		t.Errorf("an unstamped Key Value's pods carry a region: %v", labels)
	}
}

func TestPostgresPodsCarryTheRegion(t *testing.T) {
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-region", Namespace: "tea-a", Labels: map[string]string{labelWorkspace: "tea-a"}}}
	if _, ok := exportLabels(db, "exp-1")[labelRegion]; ok {
		t.Error("an unstamped database's export carries a region")
	}
	regionAnnotated(db, "singapore")
	if got := exportLabels(db, "exp-1")[labelRegion]; got != "singapore" {
		t.Errorf("export pod region = %q, want singapore: pg_dump dials the database", got)
	}
}

// A new Pooler's PgBouncer pods are born with the region. An existing one
// keeps its template, since a template change rolls PgBouncer, and its pods
// gain the label in place instead.
func TestPoolerRegionIsHeldOnAnExistingPooler(t *testing.T) {
	templateRegion := func(spec map[string]any) string {
		region, _, _ := unstructured.NestedString(spec, "template", "metadata", "labels", labelRegion)
		return region
	}
	newPooler := &unstructured.Unstructured{Object: map[string]any{}}
	spec := poolerSpec("dpg-a", resolvePlanForTest(t))
	holdPoolerRegion(spec, newPooler, "oregon")
	if got := templateRegion(spec); got != "oregon" {
		t.Errorf("a new Pooler's template region = %q", got)
	}

	existing := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{}}}}}
	existing.SetResourceVersion("7")
	spec = poolerSpec("dpg-a", resolvePlanForTest(t))
	holdPoolerRegion(spec, existing, "oregon")
	if got := templateRegion(spec); got != "" {
		t.Errorf("an existing Pooler without the label gained it in its template (a PgBouncer roll): %q", got)
	}
	delete(spec, "template") // a Pooler that does not adopt the template keeps its own
	holdPoolerRegion(spec, existing, "oregon")
	if _, ok := spec["template"]; ok {
		t.Error("holding the region wrote a template the Pooler did not have")
	}

	labelled := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"template": map[string]any{
		"metadata": map[string]any{"labels": map[string]any{labelRegion: "oregon"}},
	}}}}
	labelled.SetResourceVersion("8")
	spec = poolerSpec("dpg-a", resolvePlanForTest(t))
	holdPoolerRegion(spec, labelled, "oregon")
	if got := templateRegion(spec); got != "oregon" {
		t.Errorf("a Pooler that carries the region must keep it, or the projection drops it and rolls PgBouncer: %q", got)
	}

	unstamped := poolerSpec("dpg-a", resolvePlanForTest(t))
	before := unstamped["template"]
	holdPoolerRegion(unstamped, newPooler, "")
	if !reflect.DeepEqual(unstamped["template"], before) {
		t.Error("an unstamped database's Pooler template changed")
	}
}

func TestPoolerPodsGainTheRegionInPlace(t *testing.T) {
	ctx := context.Background()
	ctrl := true
	pod := func(name, pooler, ownerKind string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "tea-a", UID: types.UID(name),
			Labels:          map[string]string{cnpgPoolerNameLabel: pooler, labelEnvironment: "evm-a"},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: ownerKind, Name: "owner", UID: "owner", Controller: &ctrl}},
		}}
	}
	mine, other, unowned := pod("dpg-a-pooler-1", "dpg-a-pooler", kindReplicaSet), pod("dpg-b-pooler-1", "dpg-b-pooler", kindReplicaSet), pod("dpg-a-pooler-x", "dpg-a-pooler", "StatefulSet")
	cl := fake.NewClientBuilder().WithObjects(mine, other, unowned).Build()
	if err := relabelPoolerPods(ctx, cl, "tea-a", "dpg-a-pooler", "oregon"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		pod  *corev1.Pod
		want string
	}{{mine, "oregon"}, {other, ""}, {unowned, ""}} {
		got := &corev1.Pod{}
		if err := cl.Get(ctx, client.ObjectKeyFromObject(tc.pod), got); err != nil {
			t.Fatal(err)
		}
		if got.Labels[labelRegion] != tc.want || got.UID != tc.pod.UID || got.Labels[labelEnvironment] != "evm-a" {
			t.Errorf("%s: labels %v, want region %q and its other labels kept", tc.pod.Name, got.Labels, tc.want)
		}
	}
}

func resolvePlanForTest(t *testing.T) tiers.PostgresTier {
	t.Helper()
	plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-1gb"})
	return plan
}
