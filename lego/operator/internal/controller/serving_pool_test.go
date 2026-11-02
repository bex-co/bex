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
	"bytes"
	"os"
	"reflect"
	"regexp"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
)

// wantServingPool is the w2/m250 routing: the tiers no standard bex-tenant-0
// node (cx33) can hold run on their dedicated pool, every other tier — and an
// untiered App — keeps today's placement.
var wantServingPool = map[string]string{
	"": "", "free": "", "starter": "", "standard": "", "pro": "",
	"pro-plus": "xl", "pro-max": "xl", "pro-ultra": "xxl",
}

// assertServingPlacement checks a pod spec carries exactly the pool's
// toleration and required node affinity, or neither for the standard pool.
func assertServingPlacement(t *testing.T, where, tier string, spec corev1.PodSpec) {
	t.Helper()
	pool := wantServingPool[tier]
	if pool == "" {
		if spec.Tolerations != nil || spec.Affinity != nil {
			t.Errorf("%s tier %q: want no toleration/affinity, got %v / %v", where, tier, spec.Tolerations, spec.Affinity)
		}
		return
	}
	wantTol := []corev1.Toleration{{Key: "bex.co/serving-pool", Operator: corev1.TolerationOpEqual, Value: pool, Effect: corev1.TaintEffectNoSchedule}}
	if !equality.Semantic.DeepEqual(spec.Tolerations, wantTol) {
		t.Errorf("%s tier %q: tolerations = %v, want %v", where, tier, spec.Tolerations, wantTol)
	}
	na := spec.Affinity
	if na == nil || na.NodeAffinity == nil || na.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil ||
		na.PodAffinity != nil || na.PodAntiAffinity != nil || na.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
		t.Fatalf("%s tier %q: want only a required node affinity, got %v", where, tier, na)
	}
	terms := na.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	want := []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{{
		Key: "bex.co/serving-pool", Operator: corev1.NodeSelectorOpIn, Values: []string{pool},
	}}}}
	if !equality.Semantic.DeepEqual(terms, want) {
		t.Errorf("%s tier %q: node affinity = %v, want %v", where, tier, terms, want)
	}
}

// Every execution mode sized by the tier — the serving Deployment, the cron /
// one-off run template and the pre-deploy Job — is placed by it.
func TestTierServingPoolPlacement(t *testing.T) {
	if len(wantServingPool) != len(tiers.Compute.IDs())+1 {
		t.Fatalf("wantServingPool covers %d tiers, the catalog has %d", len(wantServingPool)-1, len(tiers.Compute.IDs()))
	}
	for tier := range wantServingPool {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = tier })
		assertServingPlacement(t, "deployment", tier, project(app, webParams()).Spec.Template.Spec)

		cron := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = tier; a.Spec.Type = appv1alpha1.TypeCronJob })
		assertServingPlacement(t, "cron", tier, (&AppReconciler{}).cronPodSpec(cron, "image", 0, nil, launchUnknown, "").Spec)

		job := predeploy.Job(predeploy.Options{Name: "api", Namespace: "default", Revision: "gen-1",
			Command: []string{"/bin/sh", "-c", "true"}, Resources: resourcesForTier(tier), ServingPool: servingPoolForTier(tier)})
		assertServingPlacement(t, "predeploy", tier, job.Spec.Template.Spec)
		// HardenPod's pool selector still applies: the dedicated pools'
		// nodes are tenant nodes too.
		if got := job.Spec.Template.Spec.NodeSelector[execution.NodePoolLabel]; got != execution.UntrustedNodePool {
			t.Errorf("predeploy tier %q: nodeSelector %s=%q, want %q", tier, execution.NodePoolLabel, got, execution.UntrustedNodePool)
		}
	}
}

// A disk snapshot Job mounts the App's ReadWriteOnce volume, so it must be
// able to follow the App onto its serving pool — tolerate, never require, and
// only for the mounting phases (purge mounts nothing).
func TestDiskSnapshotToleratesTheAppServingPool(t *testing.T) {
	for tier, pool := range wantServingPool {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = tier })
		r := &AppReconciler{}
		for _, command := range []string{"backup", "restore", "purge"} {
			spec := r.diskSnapshotJobSpec(app, nil, command, nil).Template.Spec
			var want []corev1.Toleration
			if pool != "" && command != "purge" {
				want = []corev1.Toleration{{Key: execution.ServingPoolKey, Operator: corev1.TolerationOpEqual, Value: pool, Effect: corev1.TaintEffectNoSchedule}}
			}
			if !equality.Semantic.DeepEqual(spec.Tolerations, want) {
				t.Errorf("tier %q %s: tolerations = %v, want %v", tier, command, spec.Tolerations, want)
			}
			if spec.Affinity != nil && spec.Affinity.NodeAffinity != nil {
				t.Errorf("tier %q %s: a snapshot must not require the pool: %v", tier, command, spec.Affinity.NodeAffinity)
			}
		}
	}
}

// A plan change in either direction re-projects the placement: moving down to
// pro leaves no stale toleration that would let it keep (and pin) an xl node,
// and moving from xl to xxl never keeps the xl affinity alongside.
func TestServingPoolPlacementFollowsTierChanges(t *testing.T) {
	app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Tier = "pro-plus" })
	dep := project(app, webParams())
	for _, tier := range []string{"pro-ultra", "pro", "pro-max"} {
		app.Spec.Tier = tier
		applyDeploymentSpec(dep, app, webParams())
		assertServingPlacement(t, "re-projected deployment", tier, dep.Spec.Template.Spec)
	}
}

// The workload admission policy cannot import the tier catalog, so its
// servingPools list is pinned to it here: a pool added to tiers.yaml and not
// to admission would refuse every Deployment of that tier at the API server.
func TestAdmissionServingPoolsMatchTheCatalog(t *testing.T) {
	raw, err := os.ReadFile("../../../../deploy/gitops/base/operator-workload-admission.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var expression string
	for doc := range bytes.SplitSeq(raw, []byte("\n---")) {
		var policy admissionregistrationv1.ValidatingAdmissionPolicy
		if err := sigsyaml.Unmarshal(doc, &policy); err != nil {
			t.Fatal(err)
		}
		if policy.Kind != "ValidatingAdmissionPolicy" || policy.Name != "bex-operator-workloads" {
			continue
		}
		for _, v := range policy.Spec.Variables {
			if v.Name == "servingPools" {
				expression = v.Expression
			}
		}
	}
	matches := regexp.MustCompile(`'([a-z0-9-]+)'`).FindAllStringSubmatch(expression, -1)
	admitted := make([]string, 0, len(matches))
	for _, m := range matches {
		admitted = append(admitted, m[1])
	}
	if want := tiers.Compute.ServingPools(); !reflect.DeepEqual(admitted, want) {
		t.Errorf("bex-operator-workloads admits serving pools %v, the tier catalog names %v", admitted, want)
	}
}
