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
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func cleanupFixture(uid types.UID) (*corev1.ConfigMap, *batchv1.Job) {
	parent := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "parent", Namespace: "default", UID: uid}}
	labels := map[string]string{"app.bex.co/app": "web", "app.bex.co/app-uid": string(uid), "app.bex.co/component": "static-purge"}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "purge-web", Namespace: "default", Labels: labels},
		Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels},
			Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "purge", Image: "test"}}}}},
	}
	return parent, job
}

func TestCleanupJobCompletionSurvivesManagerRestartAndWaitsForAbsence(t *testing.T) {
	ctx := context.Background()
	parent, desired := cleanupFixture("uid-one")
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(parent).WithStatusSubresource(&batchv1.Job{}).Build()

	if done, err := reconcileCleanupJob(ctx, cl, parent, desired, "test/complete"); err != nil || done {
		t.Fatalf("dispatch = done %v err %v", done, err)
	}
	var current batchv1.Job
	if err := cl.Get(ctx, client.ObjectKeyFromObject(desired), &current); err != nil {
		t.Fatal(err)
	}
	current.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if err := cl.Status().Update(ctx, &current); err != nil {
		t.Fatal(err)
	}
	// This call persists the terminal result on the parent. A new in-memory
	// parent below models a manager restart and cache replay.
	if done, err := reconcileCleanupJob(ctx, cl, parent, desired, "test/complete"); err != nil || done {
		t.Fatalf("persist completion = done %v err %v", done, err)
	}
	restartedParent := &corev1.ConfigMap{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(parent), restartedParent); err != nil {
		t.Fatal(err)
	}
	if restartedParent.Annotations["test/complete"] != "uid-one" {
		t.Fatalf("completion marker = %v", restartedParent.Annotations)
	}
	if done, err := reconcileCleanupJob(ctx, cl, restartedParent, desired, "test/complete"); err != nil || done {
		t.Fatalf("delete terminal Job = done %v err %v", done, err)
	}
	if done, err := reconcileCleanupJob(ctx, cl, restartedParent, desired, "test/complete"); err != nil || !done {
		t.Fatalf("verified absence = done %v err %v", done, err)
	}
}

func TestCleanupJobFailureRetriesAndRejectsOtherLifetime(t *testing.T) {
	ctx := context.Background()
	parent, desired := cleanupFixture("uid-new")
	_, oldJob := cleanupFixture("uid-old")
	oldJob.Name = desired.Name
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(parent, oldJob).WithStatusSubresource(&batchv1.Job{}).Build()
	if _, err := reconcileCleanupJob(ctx, cl, parent, desired, "test/complete"); err == nil || !strings.Contains(err.Error(), "different resource lifetime") {
		t.Fatalf("cross-lifetime adoption = %v", err)
	}

	if err := cl.Delete(ctx, oldJob); err != nil {
		t.Fatal(err)
	}
	if _, err := reconcileCleanupJob(ctx, cl, parent, desired, "test/complete"); err != nil {
		t.Fatal(err)
	}
	var failed batchv1.Job
	if err := cl.Get(ctx, client.ObjectKeyFromObject(desired), &failed); err != nil {
		t.Fatal(err)
	}
	failed.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "Transient", Message: "upstream unavailable"}}
	if err := cl.Status().Update(ctx, &failed); err != nil {
		t.Fatal(err)
	}
	if _, err := reconcileCleanupJob(ctx, cl, parent, desired, "test/complete"); err == nil || !strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatalf("failed Job = %v", err)
	}
	if parent.Annotations["test/complete"] != "" {
		t.Fatal("failed cleanup was acknowledged")
	}
}
