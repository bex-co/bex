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

package egressmeter

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/cilium/ebpf"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"
)

func TestReconcilePodsListsAllNamespacesOnNode(t *testing.T) {
	client := fake.NewClientset()
	meter := &Meter{config: Config{NodeName: "node-a"}, client: client}
	meter.config.defaults()

	// The fake client records the list action before reconcile reaches the
	// deliberately unavailable BPF maps in this unit test.
	_ = meter.reconcilePods(context.Background())
	actions := client.Actions()
	if len(actions) == 0 {
		t.Fatal("reconcile did not list Pods")
	}
	list, ok := actions[0].(ktesting.ListAction)
	if !ok {
		t.Fatalf("first action = %T, want ListAction", actions[0])
	}
	if list.GetNamespace() != "" {
		t.Fatalf("Pod list namespace = %q, want cluster-wide", list.GetNamespace())
	}
}

// w22/m414: App pods and sandbox pods are told apart by different labels, and
// a label selector cannot OR two of them. One node-scoped list, filtered in
// code, finds both without a second request every ReconcileEvery per node.
func TestReconcilePodsMakesOneNodeScopedListWithoutALabelSelector(t *testing.T) {
	client := fake.NewClientset()
	meter := &Meter{config: Config{NodeName: "node-a"}, client: client}
	meter.config.defaults()

	_ = meter.reconcilePods(context.Background())
	var lists []ktesting.ListAction
	for _, action := range client.Actions() {
		if list, ok := action.(ktesting.ListAction); ok && list.GetResource().Resource == "pods" {
			lists = append(lists, list)
		}
	}
	if len(lists) != 1 {
		t.Fatalf("reconcile made %d Pod lists, want exactly 1 (flat API load)", len(lists))
	}
	restrictions := lists[0].GetListRestrictions()
	if got := restrictions.Fields.String(); got != "spec.nodeName=node-a" {
		t.Fatalf("Pod list field selector = %q, want the node", got)
	}
	if !restrictions.Labels.Empty() {
		t.Fatalf("Pod list label selector = %q, want none: a sandbox pod carries no App label", restrictions.Labels)
	}
}

func TestProgramUsesPostPolicyNetfilterAndPodIPMaps(t *testing.T) {
	spec, err := loadBpf()
	if err != nil {
		t.Fatal(err)
	}
	if got := spec.Programs["bex_count_public_egress"].Type; got != ebpf.Netfilter {
		t.Fatalf("program type = %s, want netfilter", got)
	}
	for _, name := range []string{"pod_v4_resources", "pod_v6_resources"} {
		if got := spec.Maps[name].Type; got != ebpf.Hash {
			t.Fatalf("%s type = %s, want hash", name, got)
		}
	}
}

func TestResourceKeyRoundTripAndBounds(t *testing.T) {
	key, err := NewResourceKey("default", "srv-c185th5c2rvvnhbfiltg")
	if err != nil {
		t.Fatal(err)
	}
	namespace, appID := key.Labels()
	if namespace != "default" || appID != "srv-c185th5c2rvvnhbfiltg" {
		t.Fatalf("labels = %q/%q", namespace, appID)
	}
	if _, err := NewResourceKey("", appID); err == nil {
		t.Fatal("empty namespace must fail")
	}
	tooLong := string(make([]byte, labelBytes))
	if _, err := NewResourceKey("default", tooLong); err == nil {
		t.Fatal("overlong id must fail instead of truncating into another resource")
	}
}

func TestParsePrefixesIncludesPrivateAndConfiguredNetworks(t *testing.T) {
	prefixes, err := parsePrefixes([]string{"10.244.0.0/16", "10.96.0.0/12"})
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, prefix := range prefixes {
		have[prefix.String()] = true
	}
	for _, want := range []string{"10.0.0.0/8", "169.254.0.0/16", "fc00::/7", "10.244.0.0/16", "10.96.0.0/12"} {
		if !have[want] {
			t.Errorf("missing excluded prefix %s", want)
		}
	}
	if _, err := parsePrefixes([]string{"not-a-cidr"}); err == nil {
		t.Fatal("invalid configured CIDR must fail source health")
	}
}

func TestCheckpointAtomicRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "counters.json")
	want := []checkpointRow{{Namespace: "default", AppID: "srv-one", Bytes: 1234}}
	if err := writeCheckpoint(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("checkpoint = %#v, want %#v", got, want)
	}
}

func TestDesiredPodResourcesUsesUIDToRejectStaleIPReuse(t *testing.T) {
	pod := func(uid types.UID, appID string) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-" + appID, Namespace: "default", UID: uid,
				Labels: map[string]string{PodLabelApp: "app", PodLabelAppID: appID},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIPs: []corev1.PodIP{{IP: "10.244.1.9"}}},
		}
	}

	if _, _, _, err := desiredPodResources([]corev1.Pod{
		pod("old-uid", "srv-old"),
		pod("new-uid", "srv-new"),
	}); err == nil {
		t.Fatal("concurrent IP reuse by different immutable UIDs must fail closed")
	}

	v4, _, expected, err := desiredPodResources([]corev1.Pod{pod("new-uid", "srv-new")})
	if err != nil || expected != 1 {
		t.Fatalf("replacement attribution = (%d, %v), want (1, nil)", expected, err)
	}
	key := v4[[4]byte{10, 244, 1, 9}]
	_, appID := key.Labels()
	if appID != "srv-new" {
		t.Fatalf("reused IP attributed to %q, want srv-new", appID)
	}
}

// w4/452: a platform Job pod carries the App label without its id. It is
// skipped, never an attribution failure that turns the node's meter unhealthy.
func TestDesiredPodResourcesSkipsJobPodsWithoutAnAppID(t *testing.T) {
	app := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-1", Namespace: "default", UID: "app-uid",
			Labels: map[string]string{PodLabelApp: "web", PodLabelAppID: "srv-one"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIPs: []corev1.PodIP{{IP: "10.244.1.11"}}},
	}
	job := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pub-web-rev-1-x", Namespace: "default", UID: "job-uid",
			Labels: map[string]string{PodLabelApp: "web"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIPs: []corev1.PodIP{{IP: "10.244.1.12"}}},
	}
	v4, _, expected, err := desiredPodResources([]corev1.Pod{app, job})
	if err != nil || expected != 1 || len(v4) != 1 {
		t.Fatalf("attribution = (%v, expected %d, %v), want only the App pod", v4, expected, err)
	}
	if _, ok := v4[[4]byte{10, 244, 1, 12}]; ok {
		t.Fatal("a Job pod without an app id was attributed")
	}
}

// w4/453: a pre-deploy pod runs the service's own command, so its egress is
// the service's, through the egress-only label.
func TestDesiredPodResourcesAttributesPreDeployEgress(t *testing.T) {
	pre := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "predeploy-web-x", Namespace: "default", UID: "pre-uid",
			Labels: map[string]string{PodLabelApp: "web", PodLabelEgressAppID: "srv-one"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIPs: []corev1.PodIP{{IP: "10.244.1.13"}}},
	}
	v4, _, expected, err := desiredPodResources([]corev1.Pod{pre})
	if err != nil || expected != 1 {
		t.Fatalf("attribution = (%d, %v), want (1, nil)", expected, err)
	}
	if _, appID := v4[[4]byte{10, 244, 1, 13}].Labels(); appID != "srv-one" {
		t.Fatalf("pre-deploy egress attributed to %q, want srv-one", appID)
	}
}

func TestDesiredPodResourcesRejectsMissingStableIdentity(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "no-uid", Namespace: "default", Labels: map[string]string{PodLabelApp: "web", PodLabelAppID: "srv-one"}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning, PodIPs: []corev1.PodIP{{IP: "10.244.1.10"}}},
	}
	if _, _, _, err := desiredPodResources([]corev1.Pod{pod}); err == nil {
		t.Fatal("pod without UID must fail source health")
	}
}

func appPod(name, namespace string, uid types.UID, appID, ip string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: namespace, UID: uid,
			Labels: map[string]string{PodLabelApp: "web", PodLabelAppID: appID},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIPs: []corev1.PodIP{{IP: ip}}},
	}
}

func sandboxPod(name, namespace string, uid types.UID, ip string, extra map[string]string) corev1.Pod {
	labels := map[string]string{PodLabelSandboxID: "sbx-" + name, "app.bex.co/regime": "sandbox"}
	maps.Copy(labels, extra)
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: uid, Labels: labels},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning, PodIPs: []corev1.PodIP{{IP: ip}}},
	}
}

// w22/m414: a /v1/sandboxes sandbox pod carries neither App label. Its egress
// is the workspace's, keyed by the sandbox namespace and one constant app id so
// the series count follows workspaces, not short-lived sandboxes.
func TestDesiredPodResourcesAttributesSandboxPodToItsWorkspace(t *testing.T) {
	v4, _, expected, err := desiredPodResources([]corev1.Pod{
		sandboxPod("one", "tea-aaa-sandbox", "uid-1", "10.244.5.7", nil),
	})
	if err != nil || expected != 1 {
		t.Fatalf("attribution = (%d, %v), want (1, nil)", expected, err)
	}
	namespace, appID := v4[[4]byte{10, 244, 5, 7}].Labels()
	if namespace != "tea-aaa-sandbox" || appID != SandboxEgressAppID {
		t.Fatalf("sandbox attributed to %q/%q, want tea-aaa-sandbox/%s", namespace, appID, SandboxEgressAppID)
	}
}

func TestDesiredPodResourcesSandboxesOfAWorkspaceShareOneKey(t *testing.T) {
	v4, _, expected, err := desiredPodResources([]corev1.Pod{
		sandboxPod("one", "tea-aaa-sandbox", "uid-1", "10.244.5.7", nil),
		sandboxPod("two", "tea-aaa-sandbox", "uid-2", "10.244.5.8", nil),
		sandboxPod("three", "tea-aaa-sandbox", "uid-3", "10.244.5.9", nil),
		sandboxPod("four", "tea-bbb-sandbox", "uid-4", "10.244.5.10", nil),
	})
	if err != nil || expected != 4 || len(v4) != 4 {
		t.Fatalf("attribution = (%d pods, %d IPs, %v), want 4 pods and 4 IPs", expected, len(v4), err)
	}
	keys := map[ResourceKey]bool{}
	for _, key := range v4 {
		keys[key] = true
	}
	if len(keys) != 2 {
		t.Fatalf("4 sandboxes in 2 workspaces made %d counter keys, want 2 (bounded by workspaces)", len(keys))
	}
}

// An App pod is attributed exactly as before, next to a sandbox pod.
func TestDesiredPodResourcesKeepsAppPodsBesideSandboxPods(t *testing.T) {
	v4, _, expected, err := desiredPodResources([]corev1.Pod{
		appPod("web-1", "tea-aaa", "uid-app", "srv-one", "10.244.1.20"),
		sandboxPod("one", "tea-aaa-sandbox", "uid-sbx", "10.244.5.7", nil),
	})
	if err != nil || expected != 2 {
		t.Fatalf("attribution = (%d, %v), want (2, nil)", expected, err)
	}
	if namespace, appID := v4[[4]byte{10, 244, 1, 20}].Labels(); namespace != "tea-aaa" || appID != "srv-one" {
		t.Fatalf("App pod attributed to %q/%q, want tea-aaa/srv-one", namespace, appID)
	}
	if namespace, appID := v4[[4]byte{10, 244, 5, 7}].Labels(); namespace != "tea-aaa-sandbox" || appID != SandboxEgressAppID {
		t.Fatalf("sandbox pod attributed to %q/%q", namespace, appID)
	}
}

func TestDesiredPodResourcesSkipsPodsThatAreNeitherAppNorSandbox(t *testing.T) {
	bare := func(name, namespace string, uid types.UID, ip string, labels map[string]string) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: uid, Labels: labels},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning, PodIPs: []corev1.PodIP{{IP: ip}}},
		}
	}
	pods := []corev1.Pod{
		// No label at all, in a sandbox namespace: a snapshot Job or any
		// platform pod there is not a sandbox.
		bare("snapshot-job", "tea-aaa-sandbox", "uid-1", "10.244.5.20", nil),
		bare("coredns", "kube-system", "uid-2", "10.244.0.5", map[string]string{"k8s-app": "kube-dns"}),
		// The regime label alone does not make a /v1/sandboxes sandbox.
		bare("regime-only", "tea-aaa-sandbox", "uid-3", "10.244.5.21", map[string]string{"app.bex.co/regime": "sandbox"}),
		// An empty id is not an id.
		bare("empty-id", "tea-aaa-sandbox", "uid-4", "10.244.5.22", map[string]string{PodLabelSandboxID: ""}),
		// The sandbox id outside a sandbox namespace never opens a series for
		// a hosting namespace.
		bare("misplaced", "tea-aaa", "uid-5", "10.244.1.30", map[string]string{PodLabelSandboxID: "sbx-x"}),
	}
	v4, v6, expected, err := desiredPodResources(pods)
	if err != nil || expected != 0 || len(v4) != 0 || len(v6) != 0 {
		t.Fatalf("attribution = (%v, %v, expected %d, %v), want nothing attributed and no error", v4, v6, expected, err)
	}
}

// An agent session's sandbox goes through the session egress policy and the
// model proxy (ADR047 D5); its bytes are not a workspace's /v1/sandboxes egress.
func TestDesiredPodResourcesSkipsAgentSessionSandboxes(t *testing.T) {
	// The session sandbox as OpenSandbox labels it: the session label and no
	// sandbox id.
	session := sandboxPod("session-only", "tea-aaa-sandbox", "uid-1", "10.244.5.30", map[string]string{PodLabelAgentSession: "ses-0"})
	delete(session.Labels, PodLabelSandboxID)
	// Belt and braces: the session label wins even beside a sandbox id.
	both := sandboxPod("session-and-id", "tea-aaa-sandbox", "uid-2", "10.244.5.31", map[string]string{PodLabelAgentSession: "ses-1"})
	v4, _, expected, err := desiredPodResources([]corev1.Pod{session, both})
	if err != nil || expected != 0 || len(v4) != 0 {
		t.Fatalf("attribution = (%v, expected %d, %v), want agent-session pods skipped", v4, expected, err)
	}
}

func TestDesiredPodResourcesSkipsTerminalSandboxPods(t *testing.T) {
	for _, phase := range []corev1.PodPhase{corev1.PodSucceeded, corev1.PodFailed} {
		pod := sandboxPod("done", "tea-aaa-sandbox", "uid-1", "10.244.5.40", nil)
		pod.Status.Phase = phase
		app := appPod("web-1", "tea-aaa", "uid-2", "srv-one", "10.244.1.40")
		app.Status.Phase = phase
		v4, _, expected, err := desiredPodResources([]corev1.Pod{pod, app})
		if err != nil || expected != 0 || len(v4) != 0 {
			t.Fatalf("%s pods attribution = (%v, expected %d, %v), want none", phase, v4, expected, err)
		}
	}
	// A terminal pod's IP may already belong to its successor: it must not
	// count as a second owner.
	gone := sandboxPod("old", "tea-aaa-sandbox", "uid-old", "10.244.5.41", nil)
	gone.Status.Phase = corev1.PodFailed
	live := sandboxPod("new", "tea-aaa-sandbox", "uid-new", "10.244.5.41", nil)
	if _, _, expected, err := desiredPodResources([]corev1.Pod{gone, live}); err != nil || expected != 1 {
		t.Fatalf("IP reuse after a terminal sandbox = (%d, %v), want (1, nil)", expected, err)
	}
}

// One IP for two UIDs stops accounting whichever kinds the two pods are, so a
// sandbox cannot be cross-attributed to an App (or another workspace) in churn.
func TestDesiredPodResourcesSandboxIPReuseByTwoUIDsFailsClosed(t *testing.T) {
	cases := map[string][]corev1.Pod{
		"two sandboxes of one workspace": {
			sandboxPod("a", "tea-aaa-sandbox", "uid-a", "10.244.5.50", nil),
			sandboxPod("b", "tea-aaa-sandbox", "uid-b", "10.244.5.50", nil),
		},
		"sandboxes of two workspaces": {
			sandboxPod("a", "tea-aaa-sandbox", "uid-a", "10.244.5.50", nil),
			sandboxPod("b", "tea-bbb-sandbox", "uid-b", "10.244.5.50", nil),
		},
		"a sandbox and an App": {
			sandboxPod("a", "tea-aaa-sandbox", "uid-a", "10.244.5.50", nil),
			appPod("web-1", "tea-aaa", "uid-b", "srv-one", "10.244.5.50"),
		},
	}
	for name, pods := range cases {
		if _, _, _, err := desiredPodResources(pods); err == nil {
			t.Errorf("%s: one IP for two UIDs must fail closed", name)
		}
	}
	// The same pod listed twice is not a conflict.
	same := sandboxPod("a", "tea-aaa-sandbox", "uid-a", "10.244.5.50", nil)
	if _, _, expected, err := desiredPodResources([]corev1.Pod{same, same}); err != nil || expected != 2 {
		t.Fatalf("one UID twice = (%d, %v), want (2, nil)", expected, err)
	}
}

func TestDesiredPodResourcesRejectsSandboxPodWithoutStableIdentity(t *testing.T) {
	pod := sandboxPod("no-uid", "tea-aaa-sandbox", "", "10.244.5.60", nil)
	if _, _, _, err := desiredPodResources([]corev1.Pod{pod}); err == nil {
		t.Fatal("sandbox pod without UID must fail source health")
	}
}

func TestDesiredPodResourcesAttributesDualStackSandboxPod(t *testing.T) {
	pod := sandboxPod("dual", "tea-aaa-sandbox", "uid-1", "10.244.5.70", nil)
	pod.Status.PodIPs = append(pod.Status.PodIPs, corev1.PodIP{IP: "fd00::70"})
	v4, v6, expected, err := desiredPodResources([]corev1.Pod{pod})
	if err != nil || expected != 1 || len(v4) != 1 || len(v6) != 1 {
		t.Fatalf("attribution = (%d v4, %d v6, expected %d, %v)", len(v4), len(v6), expected, err)
	}
}

// The sandbox pool is tainted bex.co/sandbox, so a meter that does not tolerate
// it never runs where sandbox pods do and no sandbox byte is counted (w22/m414).
// EgressMeterTargetMissing's expected-node expression (deploy/gitops/base/rules)
// names exactly these two pool taints, and the Grafana "Egress meter fleet"
// panel mirrors it: change all three together. Platform, build-only and
// control-plane nodes stay un-metered by design.
func TestDaemonSetToleratesTheServingAndSandboxPoolsOnly(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "egress-meter", "daemonset.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var daemonSet appsv1.DaemonSet
	if err := yaml.Unmarshal(raw, &daemonSet); err != nil {
		t.Fatal(err)
	}
	got := map[string]corev1.TaintEffect{}
	for _, toleration := range daemonSet.Spec.Template.Spec.Tolerations {
		if toleration.Operator != corev1.TolerationOpExists {
			t.Errorf("toleration %q uses operator %q, want Exists", toleration.Key, toleration.Operator)
		}
		got[toleration.Key] = toleration.Effect
	}
	want := map[string]corev1.TaintEffect{
		"bex.co/serving-pool": corev1.TaintEffectNoSchedule,
		"bex.co/sandbox":      corev1.TaintEffectNoSchedule,
	}
	if len(got) != len(want) {
		t.Fatalf("tolerations = %v, want exactly %v", got, want)
	}
	for key, effect := range want {
		if got[key] != effect {
			t.Fatalf("tolerations = %v, want exactly %v", got, want)
		}
	}
}

func TestSourceInstancePersistsAcrossProcessRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "source-id")
	first := persistentSourceInstance(path)
	second := persistentSourceInstance(path)
	if first == "" || second != first {
		t.Fatalf("persistent source ids = %q then %q", first, second)
	}
}

func TestCounterDecreaseFailsClosed(t *testing.T) {
	previous := []checkpointRow{{Namespace: "default", AppID: "srv-one", Bytes: 100}}
	current := []checkpointRow{{Namespace: "default", AppID: "srv-one", Bytes: 99}}
	resource, oldBytes, newBytes, decreased := counterDecrease(previous, current)
	if !decreased || resource != "default/srv-one" || oldBytes != 100 || newBytes != 99 {
		t.Fatalf("decrease = (%q, %d, %d, %v)", resource, oldBytes, newBytes, decreased)
	}
	if _, _, _, decreased := counterDecrease(previous, []checkpointRow{{Namespace: "default", AppID: "srv-one", Bytes: 101}}); decreased {
		t.Fatal("monotonic counter was treated as a reset")
	}
}

func TestCounterLossStatePersistsAcrossProcessRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "counter-loss.json")
	first := New(Config{LossStatePath: path}, nil)
	if err := first.recordCounterLoss(); err != nil {
		t.Fatal(err)
	}
	if first.lossEvents.Load() != 1 || first.lastLossUnix.Load() <= 0 {
		t.Fatalf("first loss state = events %d timestamp %d", first.lossEvents.Load(), first.lastLossUnix.Load())
	}
	second := New(Config{LossStatePath: path}, nil)
	if second.stateErr != nil || second.lossEvents.Load() != 1 || second.lastLossUnix.Load() != first.lastLossUnix.Load() {
		t.Fatalf("reloaded loss state = events %d timestamp %d err %v", second.lossEvents.Load(), second.lastLossUnix.Load(), second.stateErr)
	}
	if err := second.recordCounterLoss(); err != nil {
		t.Fatal(err)
	}
	third := New(Config{LossStatePath: path}, nil)
	if third.stateErr != nil || third.lossEvents.Load() != 2 {
		t.Fatalf("second persisted event = events %d err %v", third.lossEvents.Load(), third.stateErr)
	}
}

func TestCorruptCounterLossStateFailsMeterLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "counter-loss.json")
	if err := os.WriteFile(path, []byte(`{"events":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	meter := New(Config{NodeName: "node-one", LossStatePath: path}, fake.NewClientset())
	if err := meter.Load(); err == nil {
		t.Fatal("meter loaded with corrupt counter-loss state")
	}
}
