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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m136: a datastore in an isolated environment leaves the namespace's
// allow-same-namespace policy (it carries labelNetworkIsolation) and admits only
// its environment's protected Apps and its own Jobs.

func isolatedKV(env string) *appv1alpha1.KeyValue {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cache", Namespace: "default", UID: "kv-iso-uid",
			Labels: map[string]string{labelEnvironment: "env-a", labelWorkspace: "tea-a"},
		},
		Spec: appv1alpha1.KeyValueSpec{Plan: "free", PersistenceMode: appv1alpha1.KeyValuePersistenceOff},
	}
	if env != "" {
		kv.Labels[labelNetworkIsolation] = env
	}
	return kv
}

func TestKeyValueNewbornTemplateCarriesTheIsolationLabel(t *testing.T) {
	for _, env := range []string{"", "env-a"} {
		kv := isolatedKV(env)
		sts := &appsv1.StatefulSet{}
		plan, storage := resolveKVPlan(kv.Spec)
		applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false))
		if got := sts.Spec.Template.Labels[labelNetworkIsolation]; got != env {
			t.Errorf("env %q: newborn template label = %q", env, got)
		}
		if sts.Spec.Selector.MatchLabels[labelNetworkIsolation] != "" {
			t.Errorf("the immutable selector must not carry the isolation label: %v", sts.Spec.Selector)
		}
	}
}

func TestKeyValueIsolationToggleNeverRollsTheTemplate(t *testing.T) {
	kv := isolatedKV("")
	plan, storage := resolveKVPlan(kv.Spec)
	sts := &appsv1.StatefulSet{}
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false))
	sts.CreationTimestamp = metav1.Now() // now an existing store
	settled := sts.Spec.Template.DeepCopy()

	// Isolation on, nothing else changed: the template is held.
	kv.Labels[labelNetworkIsolation] = "env-a"
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false))
	if !templateEqualIgnoringIsolation(settled, &sts.Spec.Template) || sts.Spec.Template.Labels[labelNetworkIsolation] != "" {
		t.Fatalf("an isolation toggle changed the pod template: %v", sts.Spec.Template.Labels)
	}

	// The template changes for another reason (plan): the new label rides along.
	kv.Spec.Plan = "starter"
	plan, storage = resolveKVPlan(kv.Spec)
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false))
	if got := sts.Spec.Template.Labels[labelNetworkIsolation]; got != "env-a" {
		t.Fatalf("a rolling template did not adopt the isolation label: %q", got)
	}

	// Isolation off again: held while the rest is unchanged.
	delete(kv.Labels, labelNetworkIsolation)
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false))
	if got := sts.Spec.Template.Labels[labelNetworkIsolation]; got != "env-a" {
		t.Fatalf("isolation off rolled the template: %q", got)
	}
}

func TestSyncKeyValuePodIsolationRelabelsRunningPodsOnly(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	kv := isolatedKV("env-a")
	owned := func(name string, labels map[string]string, kind string) *corev1.Pod {
		ctrl := true
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "default", Labels: labels,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: kind, Name: "cache", UID: "u", Controller: &ctrl}},
		}}
	}
	server := owned("cache-0", map[string]string{labelKeyValue: "cache"}, "StatefulSet")
	job := owned("cache-backup-x", map[string]string{labelKeyValue: "cache"}, "Job")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, server, job).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme}
	ctx := context.Background()

	if err := r.syncKeyValuePodIsolation(ctx, kv); err != nil {
		t.Fatal(err)
	}
	get := func(name string) *corev1.Pod {
		p := &corev1.Pod{}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: name}, p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if got := get("cache-0").Labels[labelNetworkIsolation]; got != "env-a" {
		t.Fatalf("server pod label = %q", got)
	}
	if _, ok := get("cache-backup-x").Labels[labelNetworkIsolation]; ok {
		t.Fatal("a backup Job pod was labelled; it must stay an ordinary peer")
	}
	delete(kv.Labels, labelNetworkIsolation)
	if err := r.syncKeyValuePodIsolation(ctx, kv); err != nil {
		t.Fatal(err)
	}
	if _, ok := get("cache-0").Labels[labelNetworkIsolation]; ok {
		t.Fatal("isolation off left the label on the running pod")
	}
}

func TestEnvironmentPeerPolicyOnlyWhileIsolatedAndAdmitsOwnJobs(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	kv := isolatedKV("env-a")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).Build()
	ctx := context.Background()
	apply := func(env string) {
		t.Helper()
		if err := reconcileEnvironmentPeerPolicy(ctx, cl, scheme, kv, env, "-environment-ingress", map[string]string{labelKeyValue: kv.Name}, keyValueJobPeer(kv)); err != nil {
			t.Fatal(err)
		}
	}
	key := client.ObjectKey{Namespace: "default", Name: "cache-environment-ingress"}
	np := &networkingv1.NetworkPolicy{}

	apply("env-a")
	if err := cl.Get(ctx, key, np); err != nil {
		t.Fatal(err)
	}
	peers := np.Spec.Ingress[0].From
	if len(peers) != 2 || peers[0].PodSelector.MatchLabels[labelNetworkIsolation] != "env-a" {
		t.Fatalf("peers = %+v, want the environment's Apps and the store's Jobs", peers)
	}
	if peers[1].PodSelector.MatchLabels[labelKeyValue] != "cache" || len(peers[1].PodSelector.MatchExpressions) != 1 {
		t.Fatalf("job peer = %+v", peers[1].PodSelector)
	}

	apply("")
	if err := cl.Get(ctx, key, np); err == nil {
		t.Fatal("the peer policy outlived isolation")
	}
}

func TestKeyValueReconcileProjectsIsolationOntoNewbornStore(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	kv := isolatedKV("env-a")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).
		WithStatusSubresource(&appv1alpha1.KeyValue{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme}
	nn := types.NamespacedName{Name: "cache", Namespace: "default"}
	for range 2 {
		if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}); err != nil {
			t.Fatal(err)
		}
	}
	var sts appsv1.StatefulSet
	if err := cl.Get(context.Background(), nn, &sts); err != nil {
		t.Fatal(err)
	}
	if sts.Spec.Template.Labels[labelNetworkIsolation] != "env-a" {
		t.Fatalf("newborn isolated store template labels = %v", sts.Spec.Template.Labels)
	}
	np := &networkingv1.NetworkPolicy{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "cache-environment-ingress"}, np); err != nil {
		t.Fatalf("isolated store has no peer policy: %v", err)
	}
}

func TestKeyValueInternalProxyLabelsKeepTheBackendSeparate(t *testing.T) {
	kv := isolatedKV("env-a")
	got := keyValueInternalProxyPodLabels(kv)
	for key, want := range map[string]string{
		labelKeyValueInternalProxy: kv.Name,
		labelWorkspace:             "tea-a",
		labelEnvironment:           "env-a",
		labelNetworkIsolation:      "env-a",
	} {
		if got[key] != want {
			t.Fatalf("proxy label %s = %q, want %q", key, got[key], want)
		}
	}
	if _, present := got[labelKeyValue]; present {
		t.Fatal("the internal proxy would be selected by its own backend Service")
	}
	delete(kv.Labels, labelNetworkIsolation)
	if _, present := keyValueInternalProxyPodLabels(kv)[labelNetworkIsolation]; present {
		t.Fatal("an ordinary proxy still carries the isolation label")
	}
}

func TestKeyValueInternalProxyPolicyProtectsBothConnectionHops(t *testing.T) {
	scheme := keyValueBackupTestScheme(t)
	kv := isolatedKV("env-a")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme}
	get := func(suffix string) *networkingv1.NetworkPolicy {
		t.Helper()
		np := &networkingv1.NetworkPolicy{}
		if err := cl.Get(t.Context(), client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + suffix}, np); err != nil {
			t.Fatal(err)
		}
		return np
	}
	for _, env := range []string{"env-a", "env-b", ""} {
		if env == "" {
			delete(kv.Labels, labelNetworkIsolation)
		} else {
			kv.Labels[labelNetworkIsolation] = env
		}
		if err := r.reconcileKeyValueInternalProxyPolicy(t.Context(), kv); err != nil {
			t.Fatal(err)
		}
		front := get("-internal-proxy-environment-ingress")
		back := get("-internal-proxy-backend-ingress")
		if front.Spec.PodSelector.MatchLabels[labelKeyValueInternalProxy] != kv.Name || back.Spec.PodSelector.MatchLabels[labelKeyValue] != kv.Name {
			t.Fatal("the two policies do not select the front and backend separately")
		}
		if len(front.Spec.PolicyTypes) != 2 || len(front.Spec.Egress) != 2 || len(front.Spec.Egress[0].To) != 1 {
			t.Fatalf("proxy needs ingress and exact-backend egress: %+v", front.Spec)
		}
		for _, tc := range []struct {
			name, ns  string
			podLabels map[string]string
			want      bool
		}{
			{"env-a client", kv.Namespace, map[string]string{labelNetworkIsolation: "env-a"}, env == "env-a"},
			{"env-b client", kv.Namespace, map[string]string{labelNetworkIsolation: "env-b"}, env == "env-b"},
			{"ordinary client", kv.Namespace, map[string]string{}, env == ""},
			{"own backup", kv.Namespace, map[string]string{labelKeyValue: kv.Name, "app.bex.co/component": keyValueBackupComponent}, true},
			{"other backup", kv.Namespace, map[string]string{labelKeyValue: "other", "app.bex.co/component": keyValueBackupComponent}, env == ""},
			{"other workspace", "tea-other", map[string]string{labelNetworkIsolation: env}, false},
		} {
			t.Run(env+"/"+tc.name, func(t *testing.T) {
				allowed := false
				for _, peer := range front.Spec.Ingress[0].From {
					allowed = allowed || keyValueInternalPolicyPeerMatches(t, front.Namespace, peer, tc.ns, tc.podLabels)
				}
				if allowed != tc.want {
					t.Fatalf("front ingress = %v, want %v", allowed, tc.want)
				}
			})
		}
		assertKeyValueInternalBridgeBoundaries(t, kv, front, back)
	}
}

// A PodSelector without NamespaceSelector admits only pods from this namespace.
func keyValueInternalPolicyPeerMatches(t *testing.T, policyNS string, peer networkingv1.NetworkPolicyPeer, ns string, podLabels map[string]string) bool {
	t.Helper()
	if peer.NamespaceSelector != nil || peer.IPBlock != nil || peer.PodSelector == nil {
		t.Fatalf("unexpected broad peer: %+v", peer)
	}
	selector, err := metav1.LabelSelectorAsSelector(peer.PodSelector)
	if err != nil {
		t.Fatal(err)
	}
	return policyNS == ns && selector.Matches(k8slabels.Set(podLabels))
}

func assertKeyValueInternalBridgeBoundaries(t *testing.T, kv *appv1alpha1.KeyValue, front, back *networkingv1.NetworkPolicy) {
	t.Helper()
	dns := front.Spec.Egress[1]
	if len(dns.To) != 1 || dns.To[0].PodSelector != nil || dns.To[0].NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] != "kube-system" || len(dns.Ports) != 2 {
		t.Fatalf("DNS is not restricted to kube-system: %+v", dns)
	}
	for _, port := range dns.Ports {
		if port.Port.IntValue() != 53 {
			t.Fatalf("DNS rule opens another port: %+v", port)
		}
	}
	backendPeer := front.Spec.Egress[0].To[0]
	if !keyValueInternalPolicyPeerMatches(t, front.Namespace, backendPeer, kv.Namespace, map[string]string{labelKeyValue: kv.Name}) || keyValueInternalPolicyPeerMatches(t, front.Namespace, backendPeer, kv.Namespace, map[string]string{labelKeyValue: "other"}) || keyValueInternalPolicyPeerMatches(t, front.Namespace, backendPeer, "tea-other", map[string]string{labelKeyValue: kv.Name}) {
		t.Fatal("the proxy can reach a backend outside its own store")
	}
	proxyPeer := back.Spec.Ingress[0].From[0]
	if !keyValueInternalPolicyPeerMatches(t, back.Namespace, proxyPeer, kv.Namespace, map[string]string{labelKeyValueInternalProxy: kv.Name}) || keyValueInternalPolicyPeerMatches(t, back.Namespace, proxyPeer, kv.Namespace, map[string]string{labelKeyValueInternalProxy: "other"}) {
		t.Fatal("backend admission does not restrict the front to this store")
	}
	for _, ports := range [][]networkingv1.NetworkPolicyPort{front.Spec.Ingress[0].Ports, front.Spec.Egress[0].Ports, back.Spec.Ingress[0].Ports} {
		if len(ports) != 2 || ports[0].Port.IntValue() != kvPort || ports[1].Port.IntValue() != kvTLSPort {
			t.Fatalf("bridge opens ports beyond plaintext and TLS Valkey: %+v", ports)
		}
	}
}
