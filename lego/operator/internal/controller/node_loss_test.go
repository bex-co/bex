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
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// w5/m142: an instance lost with its node reports NodeLost, which the control
// plane records as server_hardware_failure instead of server_failed.
func TestNodeLossMessage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	sel := map[string]string{"app": "web"}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-x"},
		Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: sel}},
	}
	pod := func(node string, ready bool, conds ...corev1.PodCondition) *corev1.Pod {
		readyStatus := corev1.ConditionFalse
		if ready {
			readyStatus = corev1.ConditionTrue
		}
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "web-abc", Namespace: "tea-x", Labels: sel},
			Spec:       corev1.PodSpec{NodeName: node},
			Status: corev1.PodStatus{Conditions: append([]corev1.PodCondition{
				{Type: corev1.PodReady, Status: readyStatus},
			}, conds...)},
		}
	}
	node := func(name string, status corev1.ConditionStatus) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}}},
		}
	}
	disrupted := func(reason string) corev1.PodCondition {
		return corev1.PodCondition{Type: corev1.DisruptionTarget, Status: corev1.ConditionTrue, Reason: reason}
	}
	failed := func(p *corev1.Pod) *corev1.Pod {
		p.Status.Phase = corev1.PodFailed
		return p
	}
	for _, tc := range []struct {
		name    string
		objs    []client.Object
		wantIn  string
		wantNil bool
	}{
		{"node not ready", []client.Object{pod("n1", false), node("n1", corev1.ConditionFalse)}, "node not ready", false},
		{"node unreachable", []client.Object{pod("n1", false), node("n1", corev1.ConditionUnknown)}, "node unreachable", false},
		{"node removed", []client.Object{pod("n1", false)}, "node removed", false},
		{"taint-manager eviction", []client.Object{pod("n1", false, disrupted("DeletionByTaintManager")), node("n1", corev1.ConditionTrue)}, "DeletionByTaintManager", false},
		{"pod GC after node deletion", []client.Object{pod("n1", false, disrupted("DeletionByPodGC"))}, "DeletionByPodGC", false},
		// A drain is planned, not a failure; a healthy node means the
		// container's own problem; a ready pod is not lost at all.
		{"drain eviction on a healthy node", []client.Object{pod("n1", false, disrupted("EvictionByEvictionAPI")), node("n1", corev1.ConditionTrue)}, "", true},
		{"unready on a healthy node", []client.Object{pod("n1", false), node("n1", corev1.ConditionTrue)}, "", true},
		{"ready pod on a not-ready node", []client.Object{pod("n1", true), node("n1", corev1.ConditionFalse)}, "", true},
		{"unscheduled pod", []client.Object{pod("", false)}, "", true},
		{"drained pod whose machine was then deleted", []client.Object{pod("n1", false, disrupted("EvictionByEvictionAPI"))}, "", true},
		{"preempted pod on a not-ready node", []client.Object{pod("n1", false, disrupted("PreemptionByScheduler")), node("n1", corev1.ConditionFalse)}, "", true},
		{"kubelet node-pressure eviction", []client.Object{pod("n1", false, disrupted("TerminationByKubelet")), node("n1", corev1.ConditionTrue)}, "", true},
		{"old failed pod from a past node loss", []client.Object{failed(pod("n1", false, disrupted("DeletionByTaintManager"))), node("n1", corev1.ConditionTrue)}, "", true},
		{"pod GC without a node name", []client.Object{pod("", false, disrupted("DeletionByPodGC"))}, "lost with its node (DeletionByPodGC)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.objs...).Build()}
			msg := r.nodeLossMessage(context.Background(), dep)
			if tc.wantNil != (msg == "") || (!tc.wantNil && !strings.Contains(msg, tc.wantIn)) {
				t.Fatalf("nodeLossMessage = %q, want contains %q (empty=%v)", msg, tc.wantIn, tc.wantNil)
			}
		})
	}
}
