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
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w9/m521: connection info hands out a public store's external URL from the
// create, so a store created public reads creating until that URL connects,
// as Render's available means every string it handed out connects. Nothing
// else holds: a store that has been available, a failed certificate, a
// private store, or one past the bound.
func TestKeyValueFirstListenerWait(t *testing.T) {
	created := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	failed := []metav1.Condition{{Type: appv1alpha1.ConditionPublicCertificateReady, Status: metav1.ConditionFalse, Reason: appv1alpha1.ReasonPublicCertificateRateLimited}}
	issuing := []metav1.Condition{{Type: appv1alpha1.ConditionPublicCertificateReady, Status: metav1.ConditionFalse, Reason: appv1alpha1.ReasonPublicCertificateIssuing}}
	for _, tc := range []struct {
		name   string
		status appv1alpha1.KeyValueStatus
		after  time.Duration
		want   time.Duration
	}{
		{"created public, certificate issuing", appv1alpha1.KeyValueStatus{ExternalHost: "red-a.kv.bex.co", Conditions: issuing}, time.Minute, kvFirstListenerWait - time.Minute},
		{"created public, no condition yet", appv1alpha1.KeyValueStatus{ExternalHost: "red-a.kv.bex.co"}, 0, kvFirstListenerWait},
		{"listener open", appv1alpha1.KeyValueStatus{ExternalHost: "red-a.kv.bex.co", TLSListener: true}, time.Minute, 0},
		{"available before: a later first enable", appv1alpha1.KeyValueStatus{ExternalHost: "red-a.kv.bex.co", CredentialRevision: "sha256:x", Conditions: issuing}, time.Minute, 0},
		{"certificate failed", appv1alpha1.KeyValueStatus{ExternalHost: "red-a.kv.bex.co", Conditions: failed}, time.Minute, 0},
		{"private", appv1alpha1.KeyValueStatus{}, time.Minute, 0},
		{"past the bound", appv1alpha1.KeyValueStatus{ExternalHost: "red-a.kv.bex.co", Conditions: issuing}, kvFirstListenerWait + time.Second, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-a", CreationTimestamp: metav1.NewTime(created)}, Status: tc.status}
			if got := keyValueFirstListenerWait(kv, created.Add(tc.after)); got != tc.want {
				t.Fatalf("wait = %v, want %v", got, tc.want)
			}
		})
	}
}

// readyKeyValueFixture is a store whose StatefulSet and pod are rolled out and
// Ready, with no internal proxy (a legacy embedding), so only the listener
// decides.
func readyKeyValueFixture(t *testing.T, kv *appv1alpha1.KeyValue) (*KeyValueReconciler, *appsv1.StatefulSet) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{appv1alpha1.AddToScheme, appsv1.AddToScheme, corev1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace, Generation: 1},
		Status:     appsv1.StatefulSetStatus{ObservedGeneration: 1, CurrentRevision: "r", UpdateRevision: "r", CurrentReplicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-0", Namespace: kv.Namespace, Labels: map[string]string{labelKeyValue: kv.Name, appsv1.StatefulSetRevisionLabel: "r"}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, sts, pod).WithStatusSubresource(kv).Build()
	return &KeyValueReconciler{Client: cl, Scheme: scheme}, sts
}

func TestKeyValueCreatedPublicReadsCreatingUntilItsListenerOpens(t *testing.T) {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-pub", Namespace: "default", Generation: 1, CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Minute))},
		Spec:       appv1alpha1.KeyValueSpec{Public: true},
		Status:     appv1alpha1.KeyValueStatus{ExternalHost: "red-pub.kv.bex.co"},
	}
	r, sts := readyKeyValueFixture(t, kv)
	ctx := context.Background()
	result, err := r.updateKeyValueReadiness(ctx, kv, sts, 1, "sha256:rev")
	if err != nil {
		t.Fatal(err)
	}
	ready := meta.FindStatusCondition(kv.Status.Conditions, appv1alpha1.ConditionReady)
	if kv.Status.Phase != appv1alpha1.KVPhaseProvisioning || ready == nil || ready.Message != "waiting for the external TLS listener" ||
		kv.Status.CredentialRevision != "" {
		t.Fatalf("listener closed: phase %s, ready %+v, revision %q; want provisioning (creating) waiting for the listener", kv.Status.Phase, ready, kv.Status.CredentialRevision)
	}
	if result.RequeueAfter <= 0 || result.RequeueAfter > kvStorageRequeue {
		t.Fatalf("requeue = %v, want a pass within %v", result.RequeueAfter, kvStorageRequeue)
	}

	kv.Status.TLSListener = true
	if _, err := r.updateKeyValueReadiness(ctx, kv, sts, 1, "sha256:rev"); err != nil {
		t.Fatal(err)
	}
	if kv.Status.Phase != appv1alpha1.KVPhaseReady || kv.Status.CredentialRevision != "sha256:rev" {
		t.Fatalf("listener open: phase %s, revision %q; want ready", kv.Status.Phase, kv.Status.CredentialRevision)
	}

	// Available once, the store never waits for its listener again: turning
	// access off and on again reads available with the certificate pending.
	kv.Status.TLSListener = false
	if _, err := r.updateKeyValueReadiness(ctx, kv, sts, 1, "sha256:rev"); err != nil {
		t.Fatal(err)
	}
	if kv.Status.Phase != appv1alpha1.KVPhaseReady {
		t.Fatalf("a later first enable: phase %s, want ready", kv.Status.Phase)
	}
}

// A certificate that failed never holds the store: its internal clients get
// it, and connection info says externalCertificate failed.
func TestKeyValueWithAFailedCertificateIsAvailable(t *testing.T) {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-fail", Namespace: "default", Generation: 1, CreationTimestamp: metav1.NewTime(time.Now())},
		Spec:       appv1alpha1.KeyValueSpec{Public: true},
		Status: appv1alpha1.KeyValueStatus{ExternalHost: "red-fail.kv.bex.co", Conditions: []metav1.Condition{{
			Type: appv1alpha1.ConditionPublicCertificateReady, Status: metav1.ConditionFalse, Reason: appv1alpha1.ReasonPublicCertificateIssuanceFailed,
			LastTransitionTime: metav1.Now(),
		}}},
	}
	r, sts := readyKeyValueFixture(t, kv)
	if _, err := r.updateKeyValueReadiness(context.Background(), kv, sts, 1, "sha256:rev"); err != nil {
		t.Fatal(err)
	}
	if kv.Status.Phase != appv1alpha1.KVPhaseReady {
		t.Fatalf("failed certificate: phase %s, want ready", kv.Status.Phase)
	}
}

// keyValueUsernameFixture is a running store whose server was started from
// template, with its pod's process.
func keyValueUsernameFixture(t *testing.T, template []corev1.Container, restarts int32) (*KeyValueReconciler, *appv1alpha1.KeyValue, *appsv1.StatefulSet, *corev1.Pod) {
	t.Helper()
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-held", Namespace: "tea-test", UID: "held"}}
	sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace, UID: "sts"},
		Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: template}}}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-0", Namespace: kv.Namespace, OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "apps/v1", Kind: "StatefulSet", Name: sts.Name, UID: sts.UID, Controller: new(true),
		}}},
		Spec: corev1.PodSpec{Containers: template},
		Status: corev1.PodStatus{PodIP: "10.0.0.9", ContainerStatuses: []corev1.ContainerStatus{{
			Name: keyValueContainerName, ContainerID: "containerd://abc", RestartCount: restarts,
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts, pod).Build()
	return &KeyValueReconciler{Client: cl, Scheme: scheme}, kv, sts, pod
}

// heldTemplate is a server template rendered before w9/m521: no id user.
func heldTemplate(kv *appv1alpha1.KeyValue) []corev1.Container {
	plan, _ := resolveKVPlan(kv.Spec)
	return []corev1.Container{{Name: keyValueContainerName, Args: valkeyArgs(kv.Spec, plan, "")}}
}

// A server started from a template held since before w9/m521 gets the id user
// at runtime, once per process, and status names it only then. A restart
// that comes back without it gets it again.
func TestKeyValueUsernameIsAddedToAHeldServerOncePerProcess(t *testing.T) {
	ctx := context.Background()
	held := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-held"}}
	r, kv, sts, pod := keyValueUsernameFixture(t, heldTemplate(held), 0)
	var calls []string
	failing := true
	r.setValkeyUser = func(_ context.Context, _ *corev1.Pod, _ *corev1.Secret, user, revision string) error {
		calls = append(calls, user+" "+revision)
		if failing {
			return errors.New("LOADING")
		}
		return nil
	}
	intent := keyValueIntent{credentialRevision: "sha256:" + fixtureHash, replicas: 1}

	if !r.reconcileKeyValueUsername(ctx, kv, sts, nil, intent) || kv.Status.Username != "" {
		t.Fatalf("a refused add: pending false or username %q; want pending and default kept", kv.Status.Username)
	}
	failing = false
	if r.reconcileKeyValueUsername(ctx, kv, sts, nil, intent) || kv.Status.Username != kv.Name {
		t.Fatalf("added: username %q, want %q", kv.Status.Username, kv.Name)
	}
	if len(calls) != 2 || calls[1] != kv.Name+" sha256:"+fixtureHash {
		t.Fatalf("adds = %q, want the id user with the credential revision", calls)
	}
	stored := &appsv1.StatefulSet{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(sts), stored); err != nil {
		t.Fatal(err)
	}
	if want := "sha256:" + fixtureHash + "/" + persistenceProcess(pod); stored.Annotations[annotKVUsername] != want {
		t.Fatalf("record = %q, want %q", stored.Annotations[annotKVUsername], want)
	}
	if !equality.Semantic.DeepEqual(stored.Spec.Template, sts.Spec.Template) {
		t.Fatal("the runtime add changed the held template")
	}

	// The same process is not dialed again.
	if r.reconcileKeyValueUsername(ctx, kv, stored, nil, intent); len(calls) != 2 {
		t.Fatalf("a settled server was dialed again: %q", calls)
	}
	// A restart from the held template comes back without the user.
	pod.Status.ContainerStatuses[0].RestartCount = 1
	if err := r.Status().Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
	if r.reconcileKeyValueUsername(ctx, kv, stored, nil, intent); len(calls) != 3 || kv.Status.Username != kv.Name {
		t.Fatalf("after a restart: adds %q, username %q; want the user added again", calls, kv.Status.Username)
	}
}

// A server started from a template carrying the user needs no runtime add,
// and a server too old to have the platform user keeps default.
func TestKeyValueUsernameFollowsTheRunningServersArgs(t *testing.T) {
	ctx := context.Background()
	named := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-held"}}
	plan, _ := resolveKVPlan(named.Spec)
	templated := []corev1.Container{{Name: keyValueContainerName, Args: valkeyArgs(named.Spec, plan, named.Name)}}
	r, kv, sts, _ := keyValueUsernameFixture(t, templated, 0)
	r.setValkeyUser = func(context.Context, *corev1.Pod, *corev1.Secret, string, string) error {
		t.Fatal("a server started with the user was dialed")
		return nil
	}
	if r.reconcileKeyValueUsername(ctx, kv, sts, nil, keyValueIntent{credentialRevision: "sha256:" + fixtureHash}); kv.Status.Username != kv.Name {
		t.Fatalf("templated server: username %q, want %q", kv.Status.Username, kv.Name)
	}

	legacy := []corev1.Container{{Name: keyValueContainerName, Args: []string{"--requirepass", "$(VALKEY_PASSWORD)"}}}
	r, kv, sts, _ = keyValueUsernameFixture(t, legacy, 0)
	r.setValkeyUser = func(context.Context, *corev1.Pod, *corev1.Secret, string, string) error {
		t.Fatal("a server without the platform user was dialed")
		return nil
	}
	if r.reconcileKeyValueUsername(ctx, kv, sts, nil, keyValueIntent{credentialRevision: "sha256:" + fixtureHash}); kv.Status.Username != "" {
		t.Fatalf("pre-platform-user server: username %q, want default kept", kv.Status.Username)
	}
}

// The operator upgrade restarts no store for the user: a running template
// rendered without it keeps exactly its pod template, while a new store's,
// and any template the store's own change rolls anyway, carries it.
func TestKeyValueTemplateGetsTheUsernameOnlyWhenItRollsAnyway(t *testing.T) {
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-roll", Namespace: "tea-test"}, Spec: appv1alpha1.KeyValueSpec{Plan: "starter"}}
	plan, storage := resolveKVPlan(kv.Spec)
	intent := keyValueIntentFor(kv, plan, storage, "", false, true)
	intent.authSecretName, intent.platformSecretName, intent.credentialRevision = "red-roll-auth", "red-roll-platform", "sha256:"+fixtureHash

	fresh := &appsv1.StatefulSet{}
	applyKeyValueStatefulSet(fresh, kv, intent)
	if !valkeyHasUser(fresh.Spec.Template.Spec.Containers, kv.Name) {
		t.Fatal("a new store's template lacks its id user")
	}

	// A running store rendered before w9/m521.
	running := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.Now()}}
	held := intent
	held.withoutUsername = true
	writeKeyValueStatefulSet(running, kv, held)
	before := running.Spec.Template.DeepCopy()
	applyKeyValueStatefulSet(running, kv, intent)
	if !equality.Semantic.DeepEqual(&running.Spec.Template, before) {
		t.Fatal("the id user rolled a running store's template")
	}

	// The store's own change (a plan change) rolls the template anyway.
	kv.Spec.Plan = "standard"
	plan, storage = resolveKVPlan(kv.Spec)
	rolled := keyValueIntentFor(kv, plan, storage, "", false, true)
	rolled.authSecretName, rolled.platformSecretName, rolled.credentialRevision = intent.authSecretName, intent.platformSecretName, intent.credentialRevision
	applyKeyValueStatefulSet(running, kv, rolled)
	if !valkeyHasUser(running.Spec.Template.Spec.Containers, kv.Name) {
		t.Fatal("a template the plan change rolled still lacks the id user")
	}
}

// fixtureHash is a SHA-256 hex digest standing in for a credential revision.
const fixtureHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
