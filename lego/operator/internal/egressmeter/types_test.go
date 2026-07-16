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
	"os"
	"path/filepath"
	"testing"

	"github.com/cilium/ebpf"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

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

func TestDesiredPodResourcesRejectsMissingStableIdentity(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "no-uid", Namespace: "default", Labels: map[string]string{PodLabelAppID: "srv-one"}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning, PodIPs: []corev1.PodIP{{IP: "10.244.1.10"}}},
	}
	if _, _, _, err := desiredPodResources([]corev1.Pod{pod}); err == nil {
		t.Fatal("pod without UID must fail source health")
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
