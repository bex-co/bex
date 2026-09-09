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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func clearCacheSiblingScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(appv1alpha1.AddToScheme(s))
	return s
}

func TestDeleteSiblingBuildJobsKeepsCurrentClearsActivePeers(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "tea-w1", UID: types.UID("uid-web"),
			Labels: map[string]string{labelWorkspace: "tea-w1"},
		},
	}
	keep := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: build.JobName("web", "gen-3"), Namespace: "tea-w1",
			Labels: map[string]string{
				"app.bex.co/build":    "web",
				execution.LabelAppUID: string(app.UID),
			},
		},
		Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "busybox"}}}}},
	}
	peer := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: build.JobName("web", "gen-2"), Namespace: "tea-w1",
			Labels: map[string]string{
				"app.bex.co/build":    "web",
				execution.LabelAppUID: string(app.UID),
			},
		},
		Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "busybox"}}}}},
	}
	done := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: build.JobName("web", "gen-1"), Namespace: "tea-w1",
			Labels: map[string]string{
				"app.bex.co/build":    "web",
				execution.LabelAppUID: string(app.UID),
			},
		},
		Spec:   batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "busybox"}}}}},
		Status: batchv1.JobStatus{Succeeded: 1},
	}
	otherApp := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: build.JobName("other", "gen-9"), Namespace: "tea-w1",
			Labels: map[string]string{
				"app.bex.co/build":    "other",
				execution.LabelAppUID: "uid-other",
			},
		},
		Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "busybox"}}}}},
	}
	cl := fake.NewClientBuilder().WithScheme(clearCacheSiblingScheme(t)).
		WithObjects(keep, peer, done, otherApp).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}
	if err := r.deleteSiblingBuildJobs(context.Background(), app, "tea-w1", "gen-3"); err != nil {
		t.Fatalf("deleteSiblingBuildJobs: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(keep), &batchv1.Job{}); err != nil {
		t.Fatalf("keep Job missing: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(peer), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("active peer Job should be deleted, got %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(done), &batchv1.Job{}); err != nil {
		t.Fatalf("succeeded Job must remain: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(otherApp), &batchv1.Job{}); err != nil {
		t.Fatalf("other App Job must remain: %v", err)
	}
}
