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
