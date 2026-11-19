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
	"strconv"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// sniTestDaemonSet is an SNI proxy DaemonSet at template generation 8, whose
// controller has observed generation <observed>, that should run on <desired>
// nodes.
func sniTestDaemonSet(observed int64, desired int32) *appsv1.DaemonSet {
	const generation = 8
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bex-kv-sni-proxy", Namespace: "bex-system", UID: "ds-uid", Generation: generation,
			Annotations: map[string]string{appsv1.DeprecatedTemplateGeneration: strconv.FormatInt(generation, 10)},
		},
		Status: appsv1.DaemonSetStatus{ObservedGeneration: observed, DesiredNumberScheduled: desired},
	}
}

type sniTestPod struct {
	node        string
	template    string
	ready       bool
	terminating bool
	owner       types.UID
}

func (p sniTestPod) pod() corev1.Pod {
	status := corev1.ConditionFalse
	if p.ready {
		status = corev1.ConditionTrue
	}
	owner := p.owner
	if owner == "" {
		owner = "ds-uid"
	}
	controller := true
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "bex-kv-sni-proxy-" + p.node + "-" + p.template,
			Labels:          map[string]string{extensionsv1beta1.DaemonSetTemplateGenerationKey: p.template},
			OwnerReferences: []metav1.OwnerReference{{Kind: "DaemonSet", Name: "bex-kv-sni-proxy", UID: owner, Controller: &controller}},
		},
		Spec:   corev1.PodSpec{NodeName: p.node},
		Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: status}}},
	}
	if p.terminating {
		now := metav1.Now()
		pod.DeletionTimestamp = &now
	}
	return pod
}

// The rule scripts/operator-pins.sh waits for and Argo's health check reads
// through the annotation (w6/m518): each node needs a Ready pod of the current
// template that is not terminating, whatever old pods still drain beside it.
func TestNewPodsReady(t *testing.T) {
	for _, tc := range []struct {
		name string
		ds   *appsv1.DaemonSet
		pods []sniTestPod
		want bool
	}{
		{
			name: "an old pod draining beside its node's Ready new pod does not count",
			ds:   sniTestDaemonSet(8, 2),
			pods: []sniTestPod{{node: "a", template: "7", ready: true, terminating: true}, {node: "a", template: "8", ready: true}, {node: "b", template: "8", ready: true}},
			want: true,
		},
		{
			name: "a new pod that is not Ready holds its node",
			ds:   sniTestDaemonSet(8, 2),
			pods: []sniTestPod{{node: "a", template: "7", ready: true}, {node: "a", template: "8"}, {node: "b", template: "8", ready: true}},
		},
		{
			name: "a generation the controller has not observed waits",
			ds:   sniTestDaemonSet(7, 2),
			pods: []sniTestPod{{node: "a", template: "8", ready: true}, {node: "b", template: "8", ready: true}},
		},
		{
			name: "a node that still runs only its old pod waits",
			ds:   sniTestDaemonSet(8, 2),
			pods: []sniTestPod{{node: "a", template: "8", ready: true}, {node: "b", template: "7", ready: true}},
		},
		{
			name: "a node whose only current pod is terminating waits",
			ds:   sniTestDaemonSet(8, 2),
			pods: []sniTestPod{{node: "a", template: "8", ready: true, terminating: true}, {node: "b", template: "8", ready: true}},
		},
		{
			name: "a pod another controller owns does not count",
			ds:   sniTestDaemonSet(8, 2),
			pods: []sniTestPod{{node: "a", template: "8", ready: true, owner: "other"}, {node: "b", template: "8", ready: true}},
		},
		{
			name: "two Ready pods on one node count once",
			ds:   sniTestDaemonSet(8, 2),
			pods: []sniTestPod{{node: "a", template: "8", ready: true}, {node: "a", template: "8", ready: true}},
		},
		{
			name: "a node that joins waits for its pod",
			ds:   sniTestDaemonSet(8, 3),
			pods: []sniTestPod{{node: "a", template: "8", ready: true}, {node: "b", template: "8", ready: true}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pods := make([]corev1.Pod, 0, len(tc.pods))
			for _, p := range tc.pods {
				pods = append(pods, p.pod())
			}
			if got := newPodsReady(tc.ds, pods); got != tc.want {
				t.Fatalf("newPodsReady = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("a DaemonSet without its template generation never counts", func(t *testing.T) {
		ds := sniTestDaemonSet(8, 1)
		delete(ds.Annotations, appsv1.DeprecatedTemplateGeneration)
		if newPodsReady(ds, []corev1.Pod{sniTestPod{node: "a", template: "", ready: true}.pod()}) {
			t.Fatal("newPodsReady = true without a template generation")
		}
	})
}
