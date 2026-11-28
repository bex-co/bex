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
	"context"
	"fmt"
	"strings"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m712: a workspace's private network is split by region, as Render's is
// (render.com/docs/regions: "services in different regions can't communicate
// directly over a private network"). A pod's region is its
// app.bex.co/region label, or the region an unstamped resource reads back
// (frankfurt on production) when it carries none.

// regionPod is one pod of the matrix: its kind (an App's instance, a Key
// Value's or a Postgres's), its region label ("" for a resource created
// without one) and its environment ("" for none).
type regionPod struct {
	kind, region, env string
}

func (p regionPod) labels() map[string]string {
	labels := map[string]string{core.LabelWorkspace: "tea-acme"}
	switch p.kind {
	case "app":
		labels["bex.co/app"] = "srv-a"
	case "keyvalue":
		labels[appv1alpha1.LabelKeyValue] = "red-a"
	case "postgres":
		labels[core.PodLabelCNPGCluster] = "dpg-a"
	}
	if p.region != "" {
		labels[appv1alpha1.LabelRegion] = p.region
	}
	if p.env != "" {
		labels[core.LabelEnvironment] = p.env
	}
	return labels
}

func (p regionPod) String() string {
	region := p.region
	if region == "" {
		region = "unstamped"
	}
	env := p.env
	if env == "" {
		env = "no environment"
	}
	return fmt.Sprintf("%s (%s, %s)", p.kind, region, env)
}

// The platform region on the harness (newIsolationHarness), the one an
// unstamped resource reads back.
const harnessRegion = "frankfurt"

func effectivePodRegion(p regionPod) string {
	if p.region == "" {
		return harnessRegion
	}
	return p.region
}

// TestPrivateNetworkIsSplitByRegionAndEnvironment checks every pair of the
// matrix against the rule: two pods reach each other privately exactly when
// they are in the same region and in the same environment scope (the same
// isolated environment, or both outside every isolated one).
func TestPrivateNetworkIsSplitByRegionAndEnvironment(t *testing.T) {
	bothConverges(t, testPrivateNetworkIsSplitByRegionAndEnvironment)
}

func testPrivateNetworkIsSplitByRegionAndEnvironment(t *testing.T, h *isolationHarness) {
	h.setIsolation("evm-open", false)
	h.setIsolation("evm-iso", true)
	h.setIsolation("evm-iso2", true)
	h.ensure(t)
	isolated := map[string]bool{"evm-iso": true, "evm-iso2": true}
	scope := func(p regionPod) string {
		if isolated[p.env] {
			return p.env
		}
		return ""
	}
	var pods []regionPod
	for _, kind := range []string{"app", "keyvalue", "postgres"} {
		for _, region := range []string{"", harnessRegion, "oregon", "singapore"} {
			for _, env := range []string{"", "evm-open", "evm-iso", "evm-iso2"} {
				pods = append(pods, regionPod{kind, region, env})
			}
		}
	}
	for _, from := range pods {
		if from.kind != "app" {
			continue // a datastore only answers; its clients are services
		}
		for _, to := range pods {
			want := effectivePodRegion(from) == effectivePodRegion(to) && scope(from) == scope(to)
			if got := h.reachable(t, from.labels(), to.labels()); got != want {
				t.Errorf("%v -> %v reachable = %v, want %v", from, to, got, want)
			}
		}
	}
}

// The definition of done's own pairs, by name.
func TestOregonCannotReachFrankfurtPrivately(t *testing.T) {
	h := newIsolationHarness(t)
	h.ensure(t)
	for _, pair := range []struct {
		what     string
		from, to regionPod
		want     bool
	}{
		{"oregon service to frankfurt service", regionPod{"app", "oregon", ""}, regionPod{"app", harnessRegion, ""}, false},
		{"oregon service to frankfurt Key Value", regionPod{"app", "oregon", ""}, regionPod{"keyvalue", harnessRegion, ""}, false},
		{"oregon service to frankfurt Postgres", regionPod{"app", "oregon", ""}, regionPod{"postgres", harnessRegion, ""}, false},
		{"frankfurt service to oregon service", regionPod{"app", harnessRegion, ""}, regionPod{"app", "oregon", ""}, false},
		{"oregon service to an unstamped (frankfurt) Key Value", regionPod{"app", "oregon", ""}, regionPod{"keyvalue", "", ""}, false},
		{"two frankfurt services", regionPod{"app", harnessRegion, ""}, regionPod{"app", harnessRegion, ""}, true},
		{"frankfurt service to frankfurt Key Value", regionPod{"app", harnessRegion, ""}, regionPod{"keyvalue", harnessRegion, ""}, true},
		{"an unstamped service counts as frankfurt", regionPod{"app", "", ""}, regionPod{"postgres", harnessRegion, ""}, true},
		{"two oregon services", regionPod{"app", "oregon", ""}, regionPod{"app", "oregon", ""}, true},
		{"oregon service to oregon Postgres", regionPod{"app", "oregon", ""}, regionPod{"postgres", "oregon", ""}, true},
	} {
		if got := h.reachable(t, pair.from.labels(), pair.to.labels()); got != pair.want {
			t.Errorf("%s: reachable = %v, want %v", pair.what, got, pair.want)
		}
	}
}

// Every Render region has its network from the start, whatever the
// workspace's resources: a service's first pod in a new region must not wait
// for a resync to reach its own region's datastore.
func TestEveryRenderRegionHasItsPrivateNetwork(t *testing.T) {
	h := newIsolationHarness(t)
	h.setIsolation("evm-iso", true)
	h.ensure(t)
	for _, region := range resourcemeta.RenderRegions {
		for _, env := range []string{"", "evm-iso"} {
			pod := regionPod{"app", region, env}
			if !h.reachable(t, pod.labels(), regionPod{"keyvalue", region, env}.labels()) {
				t.Errorf("%v cannot reach its own region's Key Value", pod)
			}
		}
	}
}

// With no platform region every Render region is its own network, and an
// unstamped resource (which then reads back no region) shares none of them.
func TestNoPlatformRegionKeepsUnstampedPodsApart(t *testing.T) {
	h := newIsolationHarness(t)
	h.r.Region = ""
	h.ensure(t)
	unstamped := regionPod{"app", "", ""}
	if !h.reachable(t, unstamped.labels(), regionPod{"postgres", "", ""}.labels()) {
		t.Error("two unstamped pods must still reach each other")
	}
	for _, region := range resourcemeta.RenderRegions {
		if h.reachable(t, unstamped.labels(), regionPod{"postgres", region, ""}.labels()) {
			t.Errorf("an unstamped pod reaches a %s one with no platform region", region)
		}
		if !h.reachable(t, regionPod{"app", region, ""}.labels(), regionPod{"postgres", region, ""}.labels()) {
			t.Errorf("two %s pods must reach each other with no platform region", region)
		}
	}
}

// preRegionPolicies is the private network as bex-api wrote it before the
// split (w4/m342): one allow-same-namespace outside the isolated
// environments, and one isolate-<env> per isolated environment, neither
// naming a region.
func preRegionPolicies(namespace string, isolated []string) []*networkingv1.NetworkPolicy {
	ordinary := metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
		Key: core.LabelEnvironment, Operator: metav1.LabelSelectorOpNotIn, Values: isolated,
	}}}
	policies := []*networkingv1.NetworkPolicy{privateNetworkPolicy(npMeta(namespace, "allow-same-namespace"), ordinary)}
	for _, env := range isolated {
		meta := npMeta(namespace, "isolate-"+env)
		meta.Labels[isolatedEnvironmentLabel] = env
		policies = append(policies, privateNetworkPolicy(meta, metav1.LabelSelector{MatchLabels: map[string]string{core.LabelEnvironment: env}}))
	}
	return policies
}

// Converging from the policies production ran before the split never drops
// a same-region path, between any two writes: the region unstamped resources
// read back keeps its policies' names, so allow-same-namespace and
// isolate-<env> only narrow away from pods of other regions, and each other
// region's policy is written before they do. Pods the operator already
// labelled (it ships in the same image) keep their path too.
func TestConvergingFromThePreRegionPoliciesDropsNoSameRegionPath(t *testing.T) {
	h := newIsolationHarness(t)
	h.setIsolation("evm-iso", true)
	h.ensure(t)
	ns := WorkspaceNamespace(h.tenant.ID)
	var current networkingv1.NetworkPolicyList
	if err := h.cl.List(context.Background(), &current); err != nil {
		t.Fatal(err)
	}
	for i := range current.Items {
		np := &current.Items[i]
		if _, boundary := np.Labels[isolatedEnvironmentLabel]; boundary || np.Name == "allow-same-namespace" || strings.HasPrefix(np.Name, "allow-same-region-") {
			if err := h.cl.Delete(context.Background(), np); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, np := range preRegionPolicies(ns, []string{"evm-iso"}) {
		if err := h.cl.Create(context.Background(), np); err != nil {
			t.Fatal(err)
		}
	}
	samePairs := [][2]regionPod{
		{{"app", "", ""}, {"postgres", "", ""}},
		{{"app", "", "evm-open"}, {"keyvalue", "", ""}},
		{{"app", "", "evm-iso"}, {"keyvalue", "", "evm-iso"}},
		{{"app", harnessRegion, ""}, {"keyvalue", "", ""}},
		{{"app", "oregon", ""}, {"keyvalue", "oregon", ""}},
		{{"app", "oregon", "evm-iso"}, {"postgres", "oregon", "evm-iso"}},
	}
	check := func(when string) {
		for _, pair := range samePairs {
			if !h.reachable(t, pair[0].labels(), pair[1].labels()) {
				t.Errorf("%s: %v lost its path to %v", when, pair[0], pair[1])
			}
		}
	}
	check("before the converge")
	h.afterWrite = func() { check("after " + h.writes[len(h.writes)-1]) }
	h.ensure(t)
	h.afterWrite = nil
	if h.reachable(t, regionPod{"app", "oregon", ""}.labels(), regionPod{"keyvalue", "", ""}.labels()) {
		t.Error("after the converge an oregon pod must not reach a frankfurt one")
	}
	for _, w := range h.writes {
		if w == "delete allow-same-namespace" || w == "delete isolate-evm-iso" {
			t.Errorf("the converge must keep the existing policies, not replace them: %v", h.writes)
		}
	}
}
