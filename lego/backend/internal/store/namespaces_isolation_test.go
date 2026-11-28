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
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m342: an environment's network isolation is the workspace's
// NetworkPolicies alone, selected by the pods' stable environment-id, so a
// toggle rewrites policies and never a pod.

// environmentsStore adds the workspace environment list the reconciler reads
// isolation from to the in-memory store.
type environmentsStore struct {
	*memStore
	environments []Environment
	err          error
}

func (s *environmentsStore) ListWorkspaceEnvironments(_ context.Context, tenantID string) ([]Environment, error) {
	var out []Environment
	for _, e := range s.environments {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, s.err
}

// isolationHarness records every NetworkPolicy write, in order, and every
// other write by kind. converge is the isolation converge under test.
type isolationHarness struct {
	r        *NamespaceReconciler
	st       *environmentsStore
	cl       client.Client
	tenant   Tenant
	writes   []string
	others   []string
	converge func(context.Context, string) error
	// afterWrite, when set, runs after every successful NetworkPolicy write,
	// so a test can check what holds between two writes of one converge.
	afterWrite func()
}

func newIsolationHarness(t *testing.T) *isolationHarness {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	h := &isolationHarness{st: &environmentsStore{memStore: newMemStore()}}
	record := func(verb string, obj client.Object) {
		if _, ok := obj.(*networkingv1.NetworkPolicy); ok {
			h.writes = append(h.writes, verb+" "+obj.GetName())
			if h.afterWrite != nil {
				h.afterWrite()
			}
		} else {
			h.others = append(h.others, fmt.Sprintf("%s %T %s", verb, obj, obj.GetName()))
		}
	}
	h.cl = fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			err := c.Create(ctx, obj, opts...)
			if err == nil {
				record("create", obj)
			}
			return err
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			err := c.Update(ctx, obj, opts...)
			if err == nil {
				record("update", obj)
			}
			return err
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			err := c.Delete(ctx, obj, opts...)
			if err == nil {
				record("delete", obj)
			}
			return err
		},
	}).Build()
	h.r = NewNamespaceReconciler(h.cl, h.st)
	// As on production: an unstamped resource reads back frankfurt.
	h.r.Region = "frankfurt"
	h.converge = h.r.EnsureWorkspace
	tenant, err := h.st.CreateTenant(context.Background(), "acme", "pro")
	if err != nil {
		t.Fatal(err)
	}
	h.tenant = tenant
	return h
}

func (h *isolationHarness) setIsolation(id string, isolated bool) {
	for i := range h.st.environments {
		if h.st.environments[i].ID == id {
			h.st.environments[i].NetworkIsolationEnabled = isolated
			return
		}
	}
	h.st.environments = append(h.st.environments, Environment{ID: id, TenantID: h.tenant.ID, NetworkIsolationEnabled: isolated})
}

func (h *isolationHarness) ensure(t *testing.T) {
	t.Helper()
	h.writes, h.others = nil, nil
	if err := h.converge(context.Background(), h.tenant.ID); err != nil {
		t.Fatalf("converge: %v", err)
	}
}

// bothConverges runs an isolation test through the full workspace converge
// and through the policies-only one a toggle uses (w4/648); the latter starts
// from a namespace the full converge already provisioned.
func bothConverges(t *testing.T, test func(t *testing.T, h *isolationHarness)) {
	t.Run("EnsureWorkspace", func(t *testing.T) { test(t, newIsolationHarness(t)) })
	t.Run("EnsureEnvironmentIsolation", func(t *testing.T) {
		h := newIsolationHarness(t)
		h.ensure(t)
		h.converge = h.r.EnsureEnvironmentIsolation
		test(t, h)
	})
}

func (h *isolationHarness) policy(t *testing.T, name string) *networkingv1.NetworkPolicy {
	t.Helper()
	var np networkingv1.NetworkPolicy
	err := h.cl.Get(context.Background(), client.ObjectKey{Namespace: WorkspaceNamespace(h.tenant.ID), Name: name}, &np)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return &np
}

// selects reports whether sel (a valid selector, or the test fails) matches
// a pod carrying podLabels.
func selects(t *testing.T, sel metav1.LabelSelector, podLabels map[string]string) bool {
	t.Helper()
	s, err := metav1.LabelSelectorAsSelector(&sel)
	if err != nil {
		t.Fatalf("invalid selector %+v: %v", sel, err)
	}
	return s.Matches(labels.Set(podLabels))
}

// admits reports whether np selects to and admits ingress from from, by
// its pod selector and its one ingress rule's pod peers.
func admits(t *testing.T, np *networkingv1.NetworkPolicy, from, to map[string]string) bool {
	t.Helper()
	if np == nil || !selects(t, np.Spec.PodSelector, to) {
		return false
	}
	for _, rule := range np.Spec.Ingress {
		for _, peer := range rule.From {
			if peer.PodSelector != nil && selects(t, *peer.PodSelector, from) {
				return true
			}
		}
	}
	return false
}

// reachable is the effective same-namespace answer for pods from → to: some
// allow must admit the ingress and (with the same peer shape) the egress.
func (h *isolationHarness) reachable(t *testing.T, from, to map[string]string) bool {
	t.Helper()
	var list networkingv1.NetworkPolicyList
	if err := h.cl.List(context.Background(), &list, client.InNamespace(WorkspaceNamespace(h.tenant.ID))); err != nil {
		t.Fatal(err)
	}
	ingress, egress := false, false
	for i := range list.Items {
		np := &list.Items[i]
		ingress = ingress || admits(t, np, from, to)
		if selects(t, np.Spec.PodSelector, from) {
			for _, rule := range np.Spec.Egress {
				for _, peer := range rule.To {
					if peer.PodSelector != nil && selects(t, *peer.PodSelector, to) {
						egress = true
					}
				}
			}
		}
	}
	return ingress && egress
}

func TestSameNamespaceAllowWithNothingIsolatedSelectsEveryPodOfItsRegion(t *testing.T) {
	policies := privateNetworkPolicies("tea-x", nil, "frankfurt")
	np := policies[len(policies)-1]
	if np.Name != "allow-same-namespace" {
		t.Fatalf("the unstamped region's allow must keep its name, got %q", np.Name)
	}
	for _, pod := range []map[string]string{
		{},
		{core.LabelEnvironment: "evm-a"},
		// A stale label of an environment no longer isolated is harmless.
		{core.LabelNetworkIsolation: "evm-a"},
		{appv1alpha1.LabelRegion: "frankfurt"},
	} {
		if !admits(t, np, pod, pod) {
			t.Errorf("allow-same-namespace with no isolated environment must admit %v", pod)
		}
	}
	for _, req := range np.Spec.PodSelector.MatchExpressions {
		if req.Key == core.LabelEnvironment {
			t.Errorf("an empty isolated list must not emit a NotIn requirement (NotIn with no values is invalid): %+v", np.Spec.PodSelector)
		}
	}
}

func TestIsolationToggleRewritesPoliciesOnly(t *testing.T) {
	bothConverges(t, testIsolationToggleRewritesPoliciesOnly)
}

func testIsolationToggleRewritesPoliciesOnly(t *testing.T, h *isolationHarness) {
	h.setIsolation("evm-open", false)
	h.ensure(t)
	inA := map[string]string{core.LabelEnvironment: "evm-iso"}
	// The operator still mirrors the isolation label onto running pods; the
	// boundary must not depend on it.
	inA2 := map[string]string{core.LabelEnvironment: "evm-iso", core.LabelNetworkIsolation: "evm-iso"}
	open := map[string]string{core.LabelEnvironment: "evm-open"}
	bare := map[string]string{}

	if !h.reachable(t, open, inA) || !h.reachable(t, inA, open) {
		t.Fatal("an environment never isolated must be reachable from the workspace")
	}

	h.setIsolation("evm-iso", true)
	h.ensure(t)
	if h.policy(t, "isolate-evm-iso") == nil {
		t.Fatal("turning isolation on must create the environment's boundary policy")
	}
	for _, pair := range []struct {
		what     string
		from, to map[string]string
		want     bool
	}{
		{"inside to inside", inA, inA2, true},
		{"mirrored inside to inside", inA2, inA, true},
		{"outside environment to inside", open, inA, false},
		{"inside to outside environment", inA, open, false},
		{"no environment to inside", bare, inA, false},
		{"outside environments still reach each other", open, bare, true},
	} {
		if got := h.reachable(t, pair.from, pair.to); got != pair.want {
			t.Errorf("isolated: %s reachable = %v, want %v", pair.what, got, pair.want)
		}
	}
	createBoundary := slices.Index(h.writes, "create isolate-evm-iso")
	narrow := slices.Index(h.writes, "update allow-same-namespace")
	if createBoundary < 0 || narrow < 0 || createBoundary > narrow {
		t.Errorf("turning on must create the boundary before narrowing allow-same-namespace, writes: %v", h.writes)
	}

	h.setIsolation("evm-iso", false)
	h.ensure(t)
	if h.policy(t, "isolate-evm-iso") != nil {
		t.Fatal("turning isolation off must delete the boundary policy")
	}
	if !h.reachable(t, open, inA) || !h.reachable(t, inA, open) || !h.reachable(t, open, inA2) {
		t.Error("turning isolation off must readmit the environment, stale isolation labels included")
	}
	widen := slices.Index(h.writes, "update allow-same-namespace")
	deleteBoundary := slices.Index(h.writes, "delete isolate-evm-iso")
	if widen < 0 || deleteBoundary < 0 || widen > deleteBoundary {
		t.Errorf("turning off must widen allow-same-namespace before deleting the boundary, writes: %v", h.writes)
	}
	if got := h.policy(t, "allow-same-namespace"); len(got.Spec.PodSelector.MatchExpressions) != 1 || got.Spec.PodSelector.MatchExpressions[0].Key != appv1alpha1.LabelRegion {
		t.Errorf("with nothing isolated allow-same-namespace must select every pod of its region: %+v", got.Spec.PodSelector)
	}
}

// w4/647: the "-legacy" twin w4/m342 kept for pods from before environment-id
// is retired, and a converge deletes one an older bex-api left behind.
func TestLeftoverLegacyBoundaryIsPruned(t *testing.T) {
	bothConverges(t, testLeftoverLegacyBoundaryIsPruned)
}

func testLeftoverLegacyBoundaryIsPruned(t *testing.T, h *isolationHarness) {
	h.setIsolation("evm-iso", true)
	h.ensure(t)
	ns := WorkspaceNamespace(h.tenant.ID)
	meta := npMeta(ns, "isolate-evm-iso-legacy")
	meta.Labels[isolatedEnvironmentLabel] = "evm-iso"
	if err := h.cl.Create(context.Background(), &networkingv1.NetworkPolicy{ObjectMeta: meta}); err != nil {
		t.Fatal(err)
	}
	h.ensure(t)
	if h.policy(t, "isolate-evm-iso-legacy") != nil {
		t.Errorf("a leftover -legacy boundary must be pruned, writes: %v", h.writes)
	}
	if h.policy(t, "isolate-evm-iso") == nil {
		t.Error("the environment's own boundary must stay")
	}
	// Only environment-id decides: the isolation label alone no longer
	// admits a pod to the environment or keeps it out of the workspace.
	isolationOnly := map[string]string{core.LabelNetworkIsolation: "evm-iso"}
	inside := map[string]string{core.LabelEnvironment: "evm-iso"}
	if h.reachable(t, isolationOnly, inside) {
		t.Error("a pod without environment-id must not reach an isolated environment")
	}
}

func TestDeletedIsolatedEnvironmentLeavesTheBoundaryList(t *testing.T) {
	bothConverges(t, testDeletedIsolatedEnvironmentLeavesTheBoundaryList)
}

func testDeletedIsolatedEnvironmentLeavesTheBoundaryList(t *testing.T, h *isolationHarness) {
	h.setIsolation("evm-a", true)
	h.setIsolation("evm-b", true)
	h.ensure(t)
	for _, name := range []string{"allow-same-namespace", "allow-same-region-oregon"} {
		got := h.policy(t, name).Spec.PodSelector.MatchExpressions
		if len(got) != 2 || got[0].Key != core.LabelEnvironment || !slices.Equal(got[0].Values, []string{"evm-a", "evm-b"}) || got[0].Operator != metav1.LabelSelectorOpNotIn {
			t.Fatalf("%s must exclude both isolated environments, got %+v", name, got)
		}
	}

	h.st.environments = h.st.environments[1:] // evm-a deleted
	h.ensure(t)
	for _, name := range []string{"allow-same-namespace", "allow-same-region-oregon"} {
		got := h.policy(t, name).Spec.PodSelector.MatchExpressions
		if len(got) != 2 || !slices.Equal(got[0].Values, []string{"evm-b"}) {
			t.Fatalf("a deleted environment must leave %s's NotIn list, got %+v", name, got)
		}
	}
	if h.policy(t, "isolate-evm-a") != nil || h.policy(t, "isolate-evm-a-oregon") != nil || h.policy(t, "isolate-evm-b") == nil || h.policy(t, "isolate-evm-b-oregon") == nil {
		t.Error("only the deleted environment's boundaries must go")
	}
}

func TestBoundaryPolicyIsWorkspaceScoped(t *testing.T) {
	h := newIsolationHarness(t)
	h.st.environments = append(h.st.environments, Environment{ID: "evm-other", TenantID: "tea-someone-else", NetworkIsolationEnabled: true})
	h.ensure(t)
	if h.policy(t, "isolate-evm-other") != nil {
		t.Error("another workspace's isolated environment must not reach this namespace")
	}
}

func TestEnvironmentReadFailureFailsTheConverge(t *testing.T) {
	bothConverges(t, testEnvironmentReadFailureFailsTheConverge)
}

func testEnvironmentReadFailureFailsTheConverge(t *testing.T, h *isolationHarness) {
	h.setIsolation("evm-a", true)
	h.ensure(t)
	h.st.err = errors.New("db down")
	if err := h.converge(context.Background(), h.tenant.ID); err == nil {
		t.Fatal("a failed environment read must fail the converge")
	}
	if got := h.policy(t, "allow-same-namespace").Spec.PodSelector.MatchExpressions; len(got) == 0 {
		t.Error("a failed read must not widen allow-same-namespace over an isolated environment")
	}
}

// w4/648: a toggle converges the isolation policies alone — no namespace,
// quota, LimitRange or RoleBinding write, and no write to another allow.
func TestEnvironmentIsolationWritesOnlyTheIsolationPolicies(t *testing.T) {
	h := newIsolationHarness(t)
	h.ensure(t)
	h.converge = h.r.EnsureEnvironmentIsolation
	for _, on := range []bool{true, false} {
		h.setIsolation("evm-a", on)
		h.ensure(t)
		if len(h.others) != 0 {
			t.Errorf("isolation %v: a toggle must write only NetworkPolicies, got %v", on, h.others)
		}
		for _, w := range h.writes {
			if !strings.HasSuffix(w, " allow-same-namespace") && !strings.Contains(w, " allow-same-region-") && !strings.Contains(w, " isolate-evm-a") {
				t.Errorf("isolation %v: a toggle must touch only the isolation policies, wrote %q", on, w)
			}
		}
	}
}

// A workspace whose namespace is missing gets the whole namespace, not
// policies in a namespace that does not exist.
func TestEnvironmentIsolationProvisionsAMissingNamespace(t *testing.T) {
	h := newIsolationHarness(t)
	h.setIsolation("evm-a", true)
	h.converge = h.r.EnsureEnvironmentIsolation
	h.ensure(t)
	if h.policy(t, "isolate-evm-a") == nil || h.policy(t, "allow-dns-egress") == nil {
		t.Errorf("a missing namespace must be provisioned in full, writes: %v", h.writes)
	}
}

// w4/648: a converge over objects already in their desired shape issues no
// Update, so the periodic resync stops rewriting every object.
func TestUnchangedObjectsGetNoUpdate(t *testing.T) {
	h := newIsolationHarness(t)
	h.setIsolation("evm-a", true)
	h.ensure(t)
	h.ensure(t)
	for _, w := range slices.Concat(h.writes, h.others) {
		if strings.HasPrefix(w, "update ") {
			t.Errorf("an unchanged object must not be updated, got %q", w)
		}
	}
	// A drifted object still converges.
	np := h.policy(t, "allow-same-namespace")
	np.Spec.PodSelector = metav1.LabelSelector{}
	if err := h.cl.Update(context.Background(), np); err != nil {
		t.Fatal(err)
	}
	h.ensure(t)
	if !slices.Contains(h.writes, "update allow-same-namespace") {
		t.Errorf("a drifted policy must be updated, writes: %v", h.writes)
	}
}

func TestUnchangedIgnoresOnlyTheServerStampedNamespaceLabel(t *testing.T) {
	desired := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tea-x", Labels: map[string]string{"a": "1"}}}
	stamped := desired.DeepCopy()
	stamped.Labels[corev1.LabelMetadataName] = "tea-x"
	stamped.Spec.Finalizers = []corev1.FinalizerName{corev1.FinalizerKubernetes}
	if !unchanged(stamped, desired) {
		t.Error("the API server's name label and finalizer must not count as drift")
	}
	extra := stamped.DeepCopy()
	extra.Labels["stale"] = "x"
	if unchanged(extra, desired) {
		t.Error("a label the desired shape drops is drift")
	}
	changed := stamped.DeepCopy()
	changed.Labels["a"] = "2"
	if unchanged(changed, desired) {
		t.Error("a changed managed label is drift")
	}
}
