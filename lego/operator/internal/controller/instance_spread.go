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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// spreadsAcrossNodes reports whether app's instances must each run on a node
// of their own, as Render runs a scaled service's instances on different nodes
// (w21/m617, ADR004 § Multiple instances run on different nodes). It holds for
// every service that can run more than one instance, whatever its count today:
// a scale changes only the Deployment's replicas and rolls nothing (ADR018
// Manual scale), so the constraint must already be in the template the scale
// finds. A disk-bearing service runs one instance under Recreate, and a plan
// capped at one instance (Free) cannot scale.
func spreadsAcrossNodes(app *appv1alpha1.App) bool {
	if app.Spec.Disk != nil {
		return false
	}
	limit, capped := tiers.Compute.InstanceCap(app.Spec.Tier)
	return !capped || limit > 1
}

// instanceSpread places a service's instances on distinct nodes:
//
//   - maxSkew 1 on kubernetes.io/hostname, DoNotSchedule. With N instances and
//     at least N nodes the scheduler can use, no node takes a second instance
//     while another holds none; past that, no node holds two more than
//     another. An instance with no such node waits, which makes the Cluster
//     Autoscaler add one, instead of landing beside its sibling as the
//     MostAllocated packing score would prefer.
//   - matchLabelKeys pod-template-hash counts only the instances of the same
//     revision, so a rolling deploy spreads its new pods among themselves and
//     the old pods it is replacing never block it.
//   - nodeTaintsPolicy Honor leaves out the nodes the pod cannot use (the
//     control plane, the platform, build and dedicated serving pools, a
//     cordoned or draining node). Counted as empty nodes, as the default
//     Ignore would count them, they would hold the minimum at zero and refuse
//     every node a second instance.
func instanceSpread(appName string) corev1.TopologySpreadConstraint {
	honor := corev1.NodeInclusionPolicyHonor
	return corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       corev1.LabelHostname,
		WhenUnsatisfiable: corev1.DoNotSchedule,
		LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: appName}},
		MatchLabelKeys:    []string{appsv1.DefaultDeploymentUniqueLabelKey},
		NodeTaintsPolicy:  &honor,
	}
}

// applyInstanceSpread writes the template's whole topology spread: the
// instance spread when spread is set, else none. bex sets no other.
func applyInstanceSpread(spec *corev1.PodSpec, appName string, spread bool) {
	spec.TopologySpreadConstraints = nil
	if spread {
		spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{instanceSpread(appName)}
	}
}

// storedSpread reports whether a stored pod spec carries a topology spread,
// which only applyInstanceSpread writes.
func storedSpread(spec corev1.PodSpec) bool {
	return len(spec.TopologySpreadConstraints) > 0
}
