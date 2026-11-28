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
	"slices"
	"strings"

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

// renderRegions is Render's region enum, the same five bex-api validates and
// reads back (resourcemeta.RenderRegions, w9/105). The build plane keeps its
// own copy because lego/backend/internal does not cross into the operator;
// keep the two lists identical.
var renderRegions = []string{"frankfurt", "ohio", "oregon", "singapore", "virginia"}

// PlatformRenderRegion normalizes raw (BEX_REGION) to the Render spelling the
// private network selects by: a Render region passes through, BEX_REGION's
// own "fsn1" reads back "frankfurt", and anything else reads back nothing —
// the same mapping as resourcemeta.RenderRegion, whose twin this is. Empty
// behaves exactly like bex-api with BEX_REGION unset: every stamped region
// keeps its own scope, and unlabeled pods share none with a Render region.
func PlatformRenderRegion(raw string) string {
	switch region := strings.TrimSpace(raw); region {
	case "fsn1":
		return "frankfurt"
	default:
		if slices.Contains(renderRegions, region) {
			return region
		}
		return ""
	}
}

// buildRegionRequirements scopes a build's workspace targets to the App's
// private-network region (w4/m712): a stamped App reaches only pods carrying
// its region; an App of the platform region — stamped with it or not —
// reaches the pods without a region label and the pods carrying the platform
// region. It admits exactly the pods bex-api's per-region allows admit
// beside the App's own (regionScopes there), so the build and the runtime
// refuse the same remotes.
func buildRegionRequirements(appRegion, platformRegion string) []metav1.LabelSelectorRequirement {
	// Only a Render region stamps its own scope: a hand-written annotation no
	// region list carries (regionOf passes it through as a label value) reads
	// back the platform region, so it shares the platform branch — the same
	// way bex-api's NotIn scopes take a pod whose label is no Render region.
	if appRegion != "" && appRegion != platformRegion && slices.Contains(renderRegions, appRegion) {
		return []metav1.LabelSelectorRequirement{{
			Key: labelRegion, Operator: metav1.LabelSelectorOpIn, Values: []string{appRegion},
		}}
	}
	others := make([]string, 0, len(renderRegions))
	for _, r := range renderRegions {
		if r != platformRegion {
			others = append(others, r)
		}
	}
	return []metav1.LabelSelectorRequirement{{
		Key: labelRegion, Operator: metav1.LabelSelectorOpNotIn, Values: others,
	}}
}
