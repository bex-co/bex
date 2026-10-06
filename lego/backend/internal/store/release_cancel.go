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

package store

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// kpackImageGVK is a buildpack build's resource. Unstructured keeps the
// backend independent of the operator's and kpack's implementation modules.
var kpackImageGVK = schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Image"}

// CancelRelease ends the release a deploy row requested — the half of a cancel
// the row cannot express — for user Cancel and for a rollout a suspend
// interrupted alike (w5/m114), so a resumed service serves its prior live
// release instead of rolling one its history says was canceled.
//
// The stamp lands first. The operator is level-triggered and only this
// annotation makes prepareAppReleaseDecision take its canceled branch and
// settle the App (settleCanceledRelease): the last served release, or
// PhaseCanceled when none ever served. Deleting a deterministic build Job or
// kpack Image without it would only see the next pass recreate it, and an
// image-backed release — which has no build to delete — would roll on
// (w6/m52, w6/m104). A newer deploy stamps a newer release generation and so
// supersedes the marker.
//
// Only the stamp can fail it: once that lands the release is ended — the
// operator settles from it and rewinds the release generation — and a caller
// that failed then would strand its row open (releaseCanceledFor closes it on
// the next pass regardless). The build deletes are best effort: a build left
// running can no longer promote.
func CancelRelease(ctx context.Context, c client.Client, a *appv1alpha1.App, generation int64, buildNamespace string) error {
	if err := markReleaseCanceled(ctx, c, a, generation); err != nil {
		return err
	}
	if err := deleteReleaseBuild(ctx, c, a, generation, buildNamespace); err != nil {
		log.Printf("cancel release %s/%s generation %d: %v", a.Namespace, a.Name, generation, err)
	}
	return nil
}

// markReleaseCanceled stamps generation unless an equal or newer cancel is
// already in place. It re-reads the App and patches under that resourceVersion:
// the caller's copy can be a whole reconcile pass old, and a blind merge could
// overwrite a newer release's cancel that landed since.
func markReleaseCanceled(ctx context.Context, c client.Client, a *appv1alpha1.App, generation int64) error {
	if err := c.Get(ctx, client.ObjectKeyFromObject(a), a); err != nil {
		return fmt.Errorf("mark canceled release: %w", err)
	}
	if marked, ok := canceledReleaseGeneration(a); ok && marked >= generation {
		return nil
	}
	base := a.DeepCopy()
	metav1.SetMetaDataAnnotation(&a.ObjectMeta, appv1alpha1.AnnotationCanceledReleaseGeneration, strconv.FormatInt(generation, 10))
	if err := c.Patch(ctx, a, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})); err != nil {
		return fmt.Errorf("mark canceled release: %w", err)
	}
	return nil
}

// canceledReleaseGeneration is the release generation the App's cancel stamp
// names; ok is false when there is none.
func canceledReleaseGeneration(a *appv1alpha1.App) (int64, bool) {
	generation, err := strconv.ParseInt(a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration], 10, 64)
	return generation, err == nil && generation > 0
}

// releaseCanceledFor reports that the App's cancel stamp names this open row's
// release. Generations only grow, so no later row can share it.
func releaseCanceledFor(open Deploy, a *appv1alpha1.App) bool {
	generation, ok := canceledReleaseGeneration(a)
	return ok && open.Generation == generation && IsOpenDeployStatus(open.Status)
}

// deleteReleaseBuild deletes a canceled repo-backed release's build Job and
// kpack Image; an image-backed release has no build to stop.
func deleteReleaseBuild(ctx context.Context, c client.Client, a *appv1alpha1.App, generation int64, buildNamespace string) error {
	if a.Spec.Repo == "" {
		return nil
	}
	ns := cmp.Or(buildNamespace, a.Namespace)
	name := appv1alpha1.BuildJobName(a.Name, appv1alpha1.BuildRevision(generation))
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}
	if err := deleteBuildArtifact(ctx, c, job); err != nil {
		return fmt.Errorf("cancel build job: %w", err)
	}
	// builder=auto may resolve to either shape, and cancellation must not race
	// that resolution: delete the kpack Image of the same name too.
	image := &unstructured.Unstructured{}
	image.SetGroupVersionKind(kpackImageGVK)
	image.SetName(name)
	image.SetNamespace(ns)
	if err := deleteBuildArtifact(ctx, c, image); err != nil {
		return fmt.Errorf("cancel kpack image: %w", err)
	}
	return nil
}

// deleteBuildArtifact deletes obj, treating an artifact that does not exist —
// or a kind this cluster does not serve, such as kpack's Image where kpack is
// not installed — as already gone.
func deleteBuildArtifact(ctx context.Context, c client.Client, obj client.Object) error {
	err := c.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationForeground))
	if err == nil || apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
		return nil
	}
	return err
}
