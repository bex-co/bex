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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPreDeployCancelsAJobRecordedUnderADifferentName covers the window a
// change to the Job-name derivation opens. A migration is running under the
// name the previous derivation produced; the operator rolls; this revision now
// computes a different name. Nothing else would sweep the old Job — the
// newest-wins cancel is normally skipped once a revision has started — so
// Ensure would create a second Job and run the migration a second time,
// concurrently with the first. That is precisely what BackoffLimit 0 exists to
// prevent ("a partial re-run can corrupt data").
//
// App CR names are tenant-prefixed (tea-<xid>-<service>), so names long enough
// to be truncated are ordinary, not exotic.
func TestPreDeployCancelsAJobRecordedUnderADifferentName(t *testing.T) {
	// The pre-deploy Job runs in the App's OWN namespace (ADR043 D8), so the
	// stale run the sweep must find lives there too. BuildNamespace stays set to
	// prove it no longer routes the step anywhere.
	const ns = "default"
	appName := "tea-" + strings.Repeat("x", 20) + "-orders-migration-service"
	rev := appv1alpha1.BuildRevision(2)

	currentName := predeploy.JobName(appName, rev)
	staleName := legacySlicedJobName(appName, rev)
	if staleName == currentName {
		t.Fatalf("test app name %q does not exercise truncation: both derivations give %q", appName, currentName)
	}

	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: "default", Generation: 2, UID: "app-uid-1"},
		Spec: appv1alpha1.AppSpec{
			Image: "registry.example/app:gen-2", Port: 3000, PreDeployCommand: "migrate",
		},
		Status: appv1alpha1.AppStatus{
			ReleaseGeneration: 2,
			PreDeploy: &appv1alpha1.PreDeployStatus{
				Job: staleName, Generation: 2, Status: appv1alpha1.PreDeployRunning,
			},
		},
	}

	stale := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      staleName,
			Namespace: ns,
			Labels: map[string]string{
				predeploy.LabelService:   appName,
				execution.LabelAppUID:    "app-uid-1",
				predeploy.LabelComponent: predeploy.ComponentValue,
			},
		},
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(app, stale).
		WithStatusSubresource(&appv1alpha1.App{}).
		Build()

	r := &AppReconciler{
		Client: cl, BuildClient: cl, Scheme: scheme,
		Mode: ModeKubernetes, BuildNamespace: "bex-build",
	}
	if _, _, err := r.reconcilePreDeploy(context.Background(), app, "registry.example/app:gen-2", 3000); err != nil {
		t.Fatalf("reconcilePreDeploy: %v", err)
	}

	var leftover batchv1.Job
	err := cl.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: staleName}, &leftover)
	if err == nil {
		var jobs batchv1.JobList
		_ = cl.List(context.Background(), &jobs, client.InNamespace(ns))
		t.Fatalf("the Job recorded under the previous name survived; %d pre-deploy Jobs now exist and the migration runs twice", len(jobs.Items))
	}

	// The replacement Job for this revision runs beside the App, not in the
	// build namespace.
	var current batchv1.Job
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: currentName}, &current); err != nil {
		t.Fatalf("current revision's pre-deploy Job not in the App namespace: %v", err)
	}
}

// legacySlicedJobName is the plain-slice truncation pre-deploy Job names used
// before they were bound to a hash of the whole name. Two revisions of a
// long-named App collapse to one name under it, which is why it was replaced:
// the completed Job for one revision was read as the next revision's, skipping
// that migration entirely.
func legacySlicedJobName(name, revision string) string {
	n := strings.ToLower("predeploy-" + name + "-" + revision)
	if len(n) > 63 {
		n = n[:63]
	}
	return n
}

// TestLegacySlicedNameCollapsesRevisions documents the defect the hash-bound
// truncation fixes, so the reason the rename was worth its migration window
// stays visible.
func TestLegacySlicedNameCollapsesRevisions(t *testing.T) {
	appName := "tea-" + strings.Repeat("x", 20) + "-orders-migration-service"

	if a, b := legacySlicedJobName(appName, "gen-1"), legacySlicedJobName(appName, "gen-2"); a != b {
		t.Skipf("this app name does not reach the collapse (%q vs %q)", a, b)
	}
	if predeploy.JobName(appName, "gen-1") == predeploy.JobName(appName, "gen-2") {
		t.Fatal("two revisions still share one pre-deploy Job name: the second revision's migration would be skipped")
	}
}
