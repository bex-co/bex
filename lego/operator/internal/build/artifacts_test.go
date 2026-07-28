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

package build

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
)

func TestReclaimAppArtifactsInventoriesEveryKindAndIsolatesRecreation(t *testing.T) {
	old := execution.ArtifactIdentity{Name: "hello", UID: "uid-old", Workspace: "tea-one", Namespace: "apps"}
	labels := old.Labels("build")
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "build-old", Namespace: "build", Labels: labels}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "build-old-pod", Namespace: "build", Labels: labels}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "copy-old", Namespace: "build", Labels: old.Labels("copied-secret")}}
	account := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-old", Namespace: "build", Labels: labels}}
	policy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "np-old", Namespace: "build", Labels: old.Labels("execution-network-policy")}}
	image := newKpackImage()
	image.SetName("image-old")
	image.SetNamespace("build")
	image.SetLabels(labels)
	build := newKpackBuild()
	build.SetName("kpack-build-old")
	build.SetNamespace("build")
	build.SetLabels(labels)

	newIdentity := execution.ArtifactIdentity{Name: "hello", UID: "uid-new", Workspace: "tea-one", Namespace: "apps"}
	newJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "build-new", Namespace: "build", Labels: newIdentity.Labels("build")}}
	cl := fakeClient(job, pod, secret, account, policy, image, build, newJob)

	done, inventory, err := ReclaimAppArtifacts(context.Background(), old, "build", cl)
	if err != nil {
		t.Fatal(err)
	}
	if done || inventory != (ArtifactInventory{Jobs: 1, Pods: 1, Secrets: 1, ServiceAccounts: 1, NetworkPolicies: 1, KpackImages: 1, KpackBuilds: 1}) {
		t.Fatalf("first inventory = %+v done=%v", inventory, done)
	}
	done, inventory, err = ReclaimAppArtifacts(context.Background(), old, "build", cl)
	if err != nil || !done || !inventory.Empty() {
		t.Fatalf("verified absence = %+v done=%v err=%v", inventory, done, err)
	}
	var jobs batchv1.JobList
	if err := cl.List(context.Background(), &jobs, client.InNamespace("build")); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 1 || jobs.Items[0].Name != newJob.Name {
		t.Fatalf("same-name recreation artifact was touched: %+v", jobs.Items)
	}
}

func TestReclaimAppArtifactsRequiresAppUID(t *testing.T) {
	done, inventory, err := ReclaimAppArtifacts(context.Background(), execution.ArtifactIdentity{Name: "hello"}, "build", fakeClient())
	if err == nil || done || !inventory.Empty() {
		t.Fatalf("empty UID reclaim = inventory %+v done=%v err=%v", inventory, done, err)
	}
}

func TestReclaimAppArtifactsRequiresCanonicalLifetimeLabels(t *testing.T) {
	identity := execution.ArtifactIdentity{Name: "hello", UID: "uid-current"}
	canonical := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "canonical", Namespace: "build", Labels: identity.Labels("build"),
	}}
	mechanismOnly := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "mechanism-only", Namespace: "build", Labels: map[string]string{"app.bex.co/build": "hello"},
	}}
	missingUID := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "missing-uid", Namespace: "build", Labels: map[string]string{
			execution.LabelApp: "hello", execution.LabelComponent: "build",
		},
	}}
	otherLifetime := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "other-lifetime", Namespace: "build", Labels: execution.ArtifactIdentity{Name: "hello", UID: "uid-old"}.Labels("build"),
	}}
	cl := fakeClient(canonical, mechanismOnly, missingUID, otherLifetime)

	done, inventory, err := ReclaimAppArtifacts(context.Background(), identity, "build", cl)
	if err != nil || done || inventory.Jobs != 1 {
		t.Fatalf("canonical inventory = %+v done=%v err=%v", inventory, done, err)
	}
	done, inventory, err = ReclaimAppArtifacts(context.Background(), identity, "build", cl)
	if err != nil || !done || !inventory.Empty() {
		t.Fatalf("verified canonical absence = %+v done=%v err=%v", inventory, done, err)
	}
	var jobs batchv1.JobList
	if err := cl.List(context.Background(), &jobs, client.InNamespace("build")); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 3 {
		t.Fatalf("non-canonical artifacts were touched: %+v", jobs.Items)
	}
}
