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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// rollout_quota_test.go pins the no-pods-at-all rollout diagnosis: the
// workspace ResourceQuota rejects the surge pod, the ReplicaSet records
// ReplicaFailure/FailedCreate ("exceeded quota"), and no pod object ever
// exists — so stuckPodMessage is blind and, without this, the App sat
// RolloutProgressing until the backend's 18-minute health gate failed it with
// the generic timeout line while the exact quota verdict already existed on
// the ReplicaSet status. The verdict now lands on the App's Ready condition
// (RolloutBlockedByQuota), where the backend's failureReasonFor whitelist
// carries it onto the deploy record.

func quotaTestDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"app": "web", labelRevision: "rev-2",
				}},
			},
		},
	}
}

func quotaTestReplicaSet(revision string, replicas int32, conditions ...appsv1.ReplicaSetCondition) *appsv1.ReplicaSet {
	name := "web-new"
	if revision != "rev-2" {
		name = "web-old"
	}
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "tea-a",
			Labels: map[string]string{"app": "web", labelRevision: revision},
		},
		Status: appsv1.ReplicaSetStatus{Replicas: replicas, Conditions: conditions},
	}
}

const quotaVerdict = `pods "web-new-abc12-" is forbidden: exceeded quota: tenant-quota, requested: pods=1, used: pods=20, limited: pods=20`

func TestRolloutQuotaBlockMessage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	quotaFailed := appsv1.ReplicaSetCondition{
		Type: appsv1.ReplicaSetReplicaFailure, Status: corev1.ConditionTrue,
		Reason: "FailedCreate", Message: quotaVerdict,
	}
	otherFailure := appsv1.ReplicaSetCondition{
		Type: appsv1.ReplicaSetReplicaFailure, Status: corev1.ConditionTrue,
		Reason: "FailedCreate", Message: `pods "web-new-abc12-" is forbidden: violates PodSecurity`,
	}

	tests := []struct {
		name       string
		replicaSet *appsv1.ReplicaSet
		mutateDep  func(*appsv1.Deployment)
		wantReason string
		wantIn     []string
	}{
		{
			name:       "quota FailedCreate with no new pods",
			replicaSet: quotaTestReplicaSet("rev-2", 0, quotaFailed),
			wantReason: "RolloutBlockedByQuota",
			wantIn:     []string{"resource quota", "tenant-quota", "used: pods=20", "quota headroom"},
		},
		{
			name:       "no ReplicaFailure condition — merely progressing",
			replicaSet: quotaTestReplicaSet("rev-2", 0),
			wantReason: "",
		},
		{
			name:       "non-quota FailedCreate stays generic",
			replicaSet: quotaTestReplicaSet("rev-2", 0, otherFailure),
			wantReason: "",
		},
		{
			name:       "new revision already has pods — stuckPodMessage owns it",
			replicaSet: quotaTestReplicaSet("rev-2", 1, quotaFailed),
			wantReason: "",
		},
		{
			name:       "quota verdict on the OLD revision is not this rollout's",
			replicaSet: quotaTestReplicaSet("rev-1", 0, quotaFailed),
			wantReason: "",
		},
		{
			name:       "no revision label — inconclusive",
			replicaSet: quotaTestReplicaSet("rev-2", 0, quotaFailed),
			mutateDep:  func(d *appsv1.Deployment) { d.Spec.Template.Labels = map[string]string{"app": "web"} },
			wantReason: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dep := quotaTestDeployment()
			if tc.mutateDep != nil {
				tc.mutateDep(dep)
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.replicaSet).Build()
			r := &AppReconciler{Client: cl}
			reason, msg := r.rolloutQuotaBlockMessage(context.Background(), dep)
			if reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q (msg %q)", reason, tc.wantReason, msg)
			}
			if tc.wantReason == "" && msg != "" {
				t.Fatalf("msg = %q, want empty", msg)
			}
			for _, s := range tc.wantIn {
				if !strings.Contains(msg, s) {
					t.Errorf("message %q missing %q", msg, s)
				}
			}
		})
	}
}

// TestReportRolloutProgressStampsQuotaBlock pins the wiring: a quota-blocked
// rollout's App Ready condition carries RolloutBlockedByQuota (the reason the
// backend whitelists in failureReasonFor), not the opaque RolloutProgressing.
func TestReportRolloutProgressStampsQuotaBlock(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)

	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", Generation: 2},
	}
	rs := quotaTestReplicaSet("rev-2", 0, appsv1.ReplicaSetCondition{
		Type: appsv1.ReplicaSetReplicaFailure, Status: corev1.ConditionTrue,
		Reason: "FailedCreate", Message: quotaVerdict,
	})
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, rs).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme}

	if _, err := r.reportRolloutProgress(context.Background(), app, quotaTestDeployment(), 1, 3000, "waiting"); err != nil {
		t.Fatalf("reportRolloutProgress: %v", err)
	}
	ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Reason != "RolloutBlockedByQuota" {
		t.Fatalf("Ready condition = %+v, want RolloutBlockedByQuota", ready)
	}
	if !strings.Contains(ready.Message, "tenant-quota") {
		t.Errorf("message does not carry the quota detail: %q", ready.Message)
	}
}
