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
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A workspace's private network is split by region (w4/m712), as Render's
// is: bex-api's private-network policies admit a pod's traffic only from and
// to pods of its own region. A pod's region is labelRegion, copied from the
// resource it runs for: every pod of a service, cron run, pre-deploy command,
// edge-cache tier, Key Value (with its internal proxy and backups) and
// Postgres (instances, replicas, poolers, exports) carries it, so a store's
// own clients and helpers stay in its network. A resource created without a
// region gets no label, and bex-api counts its pods in the region it reads
// back. Render's region is fixed at creation, so the label never changes; it
// is still one of the in-place labels (inPlacePodLabels), so a running pod
// gains it without a restart.
const labelRegion = appv1alpha1.LabelRegion

// regionOf is the region obj's pods carry: the Render region the resource
// was created with, or "" when the create named none (or the annotation is
// no valid label value, which only a hand-written CR can hold).
func regionOf(obj metav1.Object) string {
	region := obj.GetAnnotations()[appv1alpha1.AnnotationRegion]
	if len(validation.IsValidLabelValue(region)) > 0 {
		return ""
	}
	return region
}

// inPlaceLabelsOf is what obj's running pods carry in place
// (relabelPodsInPlace, holdIsolationLabel): its environment labels and its
// region, which comes from the annotation alone.
func inPlaceLabelsOf(obj metav1.Object) map[string]string {
	labels := maps.Clone(obj.GetLabels())
	delete(labels, labelRegion)
	return withRegion(labels, obj)
}

// withRegion returns labels with obj's region set, or labels unchanged when
// obj has none. labels may be nil.
func withRegion(labels map[string]string, obj metav1.Object) map[string]string {
	region := regionOf(obj)
	if region == "" {
		return labels
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels[labelRegion] = region
	return labels
}
