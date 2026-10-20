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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// imageDigestFromRef is the sha256 digest a reference pins — "repo@sha256:…",
// or a kubelet imageID such as "docker-pullable://repo@sha256:…" — or "". A
// bare "sha256:…" imageID is a local image's config id, not a registry
// manifest digest, so it does not count.
func imageDigestFromRef(ref string) string {
	i := strings.LastIndex(ref, "@")
	if i < 0 || !registry.IsDigest(ref[i+1:]) {
		return ""
	}
	return ref[i+1:]
}

// recordImageDigest keeps status.imageDigest on the release generation now
// serving (w8/m122), for bex-api's deploy image.sha. A digest-pinned
// status.image is its own evidence; a tag is read off a Ready pod of dep's
// current revision (dep nil: no long-running pod to read). Once the
// generation has a digest it stays, so a pod that later re-pulls a moved tag
// never rewrites what the release first ran; another generation's digest is
// dropped, never carried forward.
func (r *AppReconciler) recordImageDigest(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment) {
	generation := releaseGeneration(app)
	if d := app.Status.ImageDigest; d != nil && d.Generation == generation {
		return
	}
	app.Status.ImageDigest = nil
	digest := imageDigestFromRef(app.Status.Image)
	if digest == "" && dep != nil {
		digest = r.readyPodImageDigest(ctx, dep)
	}
	if digest != "" {
		app.Status.ImageDigest = &appv1alpha1.ImageDigestStatus{Generation: generation, Digest: digest}
	}
}

// readyPodImageDigest is the digest the kubelet reports for the app container
// of the earliest-created (then lowest-named) Ready, non-terminating pod of
// dep's current revision, or "" when none reports one.
func (r *AppReconciler) readyPodImageDigest(ctx context.Context, dep *appsv1.Deployment) string {
	revision := dep.Spec.Template.Labels[labelRevision]
	if revision == "" || dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return ""
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		logf.FromContext(ctx).V(1).Info("listing pods for the image digest", "deployment", dep.Name, "error", err.Error())
		return ""
	}
	var chosen *corev1.Pod
	digest := ""
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !pod.DeletionTimestamp.IsZero() || pod.Labels[labelRevision] != revision || !podReady(pod) {
			continue
		}
		if chosen != nil && !podCreatedBefore(pod, chosen) {
			continue
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name != appContainerName {
				continue
			}
			if d := imageDigestFromRef(cs.ImageID); d != "" {
				chosen, digest = pod, d
			}
			break
		}
	}
	return digest
}

// podCreatedBefore orders pods by creation time, then name, since creation
// timestamps have one-second resolution.
func podCreatedBefore(a, b *corev1.Pod) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}
	return a.Name < b.Name
}
