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
	"slices"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPruneKpackRevisions (w5/129): once gen-4's artifact is stored, every
// other revision's Image, service account and credential Secret goes, whatever
// its build is doing: finished, rebuilt by kpack after a branch push, or still
// on a first build no release will use. gen-4's own stay, as do those of a
// same-named App in another workspace.
func TestPruneKpackRevisions(t *testing.T) {
	ctx := context.Background()
	at := func(appUID, revision string) Options {
		o := opts()
		o.AppUID, o.Revision = appUID, revision
		return o
	}
	artifacts := func(o Options, image *unstructured.Unstructured) []client.Object {
		return []client.Object{
			image,
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: kpackServiceAccountName(o), Namespace: o.Namespace, Labels: kpackArtifactLabels(o, kpackServiceAccountPurpose)}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kpackArtifactName(o, "bld-"+o.Name+"-kpack-registry", kpackRegistrySecretPurpose), Namespace: o.Namespace, Labels: kpackArtifactLabels(o, kpackRegistrySecretPurpose)}},
		}
	}
	finished := func(o Options) []client.Object {
		return artifacts(o, kpackImageWithCondition(o, corev1.ConditionTrue, "Build", "", "zot/web@sha256:"+o.Revision))
	}
	uid := opts().AppUID
	rebuilding := at(uid, "gen-2")
	firstBuild := at(uid, "gen-1")
	spent := slices.Concat(
		finished(at(uid, "gen-3")),
		artifacts(rebuilding, kpackImageWithCondition(rebuilding, corev1.ConditionUnknown, "", "", "zot/web@sha256:gen-2")),
		artifacts(firstBuild, KpackImage(firstBuild)),
	)
	kept := finished(at(uid, "gen-4"))
	namesake := finished(at("uid-namesake", "gen-3"))
	cl := fakeClient(slices.Concat(spent, kept, namesake)...)

	identity := execution.ArtifactIdentity{Name: opts().Name, UID: uid}
	// keep matches gen-4 as the labels spell it (lower-case), not as passed.
	if err := PruneKpackRevisions(ctx, cl, opts().Namespace, identity, "GEN-4"); err != nil {
		t.Fatalf("prune: %v", err)
	}
	present := func(obj client.Object) bool {
		t.Helper()
		probe := obj.DeepCopyObject().(client.Object)
		err := cl.Get(ctx, client.ObjectKeyFromObject(obj), probe)
		if err != nil && !apierrors.IsNotFound(err) {
			t.Fatal(err)
		}
		return err == nil
	}
	for _, obj := range spent {
		if present(obj) {
			t.Errorf("%T %s (revision %s) outlived gen-4's stored build", obj, obj.GetName(), obj.GetLabels()[appv1alpha1.LabelBuildRevision])
		}
	}
	for name, set := range map[string][]client.Object{"gen-4's": kept, "the namesake App's": namesake} {
		for _, obj := range set {
			if !present(obj) {
				t.Errorf("%s %T %s was pruned", name, obj, obj.GetName())
			}
		}
	}
}

// TestStopRevision (w5/149): a canceled release's build ran to the end unless
// bex-api's one delete caught it. StopRevision deletes that revision's build
// Job while it runs and its kpack Image, and nothing else: not a finished Job,
// not a later revision's build, not a namesake App's build of the revision.
func TestStopRevision(t *testing.T) {
	ctx := context.Background()
	at := func(appUID, revision string) Options {
		o := opts()
		o.AppUID, o.Revision = appUID, revision
		return o
	}
	uid := opts().AppUID
	finished := BuildJob(at(uid, "gen-3"), "zot/hello:gen-3")
	finished.Status.Succeeded = 1
	finished.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	// A first attempt that failed leaves the Job retrying, not finished: it
	// still holds a build-cap slot.
	retrying := BuildJob(at(uid, "gen-3"), "zot/hello:gen-3")
	retrying.Status.Failed = 1
	for name, tc := range map[string]struct {
		job     *batchv1.Job
		stopped bool
	}{
		"its running Job":          {BuildJob(at(uid, "gen-3"), "zot/hello:gen-3"), true},
		"its retrying Job":         {retrying, true},
		"its finished Job":         {finished, false},
		"a namesake's running Job": {BuildJob(at("uid-namesake", "gen-3"), "zot/hello:gen-3"), false},
	} {
		t.Run(name, func(t *testing.T) {
			canceled, later, namesakes := KpackImage(at(uid, "gen-3")), KpackImage(at(uid, "gen-4")), KpackImage(at("uid-namesake", "gen-3"))
			cl := fakeClient(tc.job, canceled, later, namesakes)
			if err := StopRevision(ctx, cl, opts().Namespace, execution.ArtifactIdentity{Name: opts().Name, UID: uid}, "gen-3"); err != nil {
				t.Fatalf("stop: %v", err)
			}
			present := func(obj client.Object) bool {
				t.Helper()
				err := cl.Get(ctx, client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
				if err != nil && !apierrors.IsNotFound(err) {
					t.Fatal(err)
				}
				return err == nil
			}
			if present(tc.job) == tc.stopped {
				t.Errorf("the Job is present = %t after the stop, want stopped = %t", present(tc.job), tc.stopped)
			}
			if present(canceled) {
				t.Error("the canceled revision's kpack Image survived the stop")
			}
			for which, obj := range map[string]client.Object{"gen-4's": later, "the namesake's": namesakes} {
				if !present(obj) {
					t.Errorf("%s kpack Image was stopped", which)
				}
			}
		})
	}
}
