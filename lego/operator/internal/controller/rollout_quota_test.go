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
	"strconv"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// rollout_quota_test.go pins the quota-refused rollout diagnosis: the
// workspace ResourceQuota rejects the new template's pod, the ReplicaSet
// records ReplicaFailure/FailedCreate ("exceeded quota"), and no pod object
// for it exists — so stuckPodMessage is blind and, without this, the App sat
// RolloutProgressing until the backend's health gate failed it with the
// generic timeout line. The verdict lands on the App's Ready condition
// (RolloutBlockedByQuota), which bex-api projects as the open deploy row's
// stall reason.

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

// quotaTestReplicaSet is the Deployment's seq-th ReplicaSet (the controller's
// revision annotation), created for a template of release revision.
func quotaTestReplicaSet(revision string, seq int, replicas int32, conditions ...appsv1.ReplicaSetCondition) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-" + strconv.Itoa(seq), Namespace: "tea-a",
			Labels:      map[string]string{"app": "web", labelRevision: revision},
			Annotations: map[string]string{deploymentRevisionAnnotation: strconv.Itoa(seq)},
		},
		Status: appsv1.ReplicaSetStatus{Replicas: replicas, Conditions: conditions},
	}
}

const (
	quotaVerdict       = `pods "web-new-abc12-" is forbidden: exceeded quota: tenant-quota, requested: pods=1, used: pods=20, limited: pods=20`
	memoryQuotaVerdict = `pods "web-7c9f-x2" is forbidden: exceeded quota: tenant-quota, requested: limits.memory=512Mi,requests.memory=512Mi, ` +
		`used: limits.memory=5888Mi,requests.memory=5888Mi, limited: limits.memory=6Gi,requests.memory=6Gi`
)

func quotaFailedCondition(verdict string) appsv1.ReplicaSetCondition {
	return appsv1.ReplicaSetCondition{
		Type: appsv1.ReplicaSetReplicaFailure, Status: corev1.ConditionTrue,
		Reason: "FailedCreate", Message: verdict,
	}
}

func TestRolloutQuotaBlockMessage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	quotaFailed := quotaFailedCondition(quotaVerdict)
	otherFailure := quotaFailedCondition(`pods "web-new-abc12-" is forbidden: violates PodSecurity`)

	tests := []struct {
		name        string
		replicaSets []*appsv1.ReplicaSet
		mutateDep   func(*appsv1.Deployment)
		wantReason  string
		wantIn      []string
	}{
		{
			name:        "quota FailedCreate with no new pods",
			replicaSets: []*appsv1.ReplicaSet{quotaTestReplicaSet("rev-2", 2, 0, quotaFailed)},
			wantReason:  reasonRolloutBlockedByQuota,
			wantIn: []string{"the workspace's resource quota is full (instances: 20 of 20 in use, the new instance needs 1)",
				appv1alpha1.QuotaStallWaiting, "upgrade the plan"},
		},
		{
			// w8/223: image-port discovery rolled the release's template; the
			// replaced template's ReplicaSet keeps serving while the new one's
			// surge pod is refused by the memory quota.
			name: "surge refused while the release's earlier template serves",
			replicaSets: []*appsv1.ReplicaSet{
				quotaTestReplicaSet("rev-1", 1, 0),
				quotaTestReplicaSet("rev-2", 2, 1),
				quotaTestReplicaSet("rev-2", 3, 0, quotaFailedCondition(memoryQuotaVerdict)),
			},
			wantReason: reasonRolloutBlockedByQuota,
			wantIn: []string{"the workspace's resource quota is full (memory: 5888Mi of 6Gi in use, the new instance needs 512Mi): " +
				appv1alpha1.QuotaStallWaiting},
		},
		{
			name:        "surge refused on a rolling update while the old release serves",
			replicaSets: []*appsv1.ReplicaSet{quotaTestReplicaSet("rev-1", 1, 1), quotaTestReplicaSet("rev-2", 2, 0, quotaFailedCondition(memoryQuotaVerdict))},
			wantReason:  reasonRolloutBlockedByQuota,
			wantIn:      []string{"memory: 5888Mi of 6Gi in use"},
		},
		{
			name:        "some pods created, the rest refused",
			replicaSets: []*appsv1.ReplicaSet{quotaTestReplicaSet("rev-2", 2, 1, quotaFailed)},
			wantReason:  reasonRolloutBlockedByQuota,
		},
		{
			name:        "a stale verdict on the release's replaced template is not this rollout's",
			replicaSets: []*appsv1.ReplicaSet{quotaTestReplicaSet("rev-2", 2, 0, quotaFailed), quotaTestReplicaSet("rev-2", 3, 0)},
			wantReason:  "",
		},
		{
			name:        "no ReplicaFailure condition — merely progressing",
			replicaSets: []*appsv1.ReplicaSet{quotaTestReplicaSet("rev-2", 2, 0)},
			wantReason:  "",
		},
		{
			name:        "non-quota FailedCreate stays generic",
			replicaSets: []*appsv1.ReplicaSet{quotaTestReplicaSet("rev-2", 2, 0, otherFailure)},
			wantReason:  "",
		},
		{
			name:        "quota verdict on the OLD revision is not this rollout's",
			replicaSets: []*appsv1.ReplicaSet{quotaTestReplicaSet("rev-1", 1, 0, quotaFailed)},
			wantReason:  "",
		},
		{
			name:        "no revision label — inconclusive",
			replicaSets: []*appsv1.ReplicaSet{quotaTestReplicaSet("rev-2", 2, 0, quotaFailed)},
			mutateDep:   func(d *appsv1.Deployment) { d.Spec.Template.Labels = map[string]string{"app": "web"} },
			wantReason:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dep := quotaTestDeployment()
			if tc.mutateDep != nil {
				tc.mutateDep(dep)
			}
			objs := make([]client.Object, 0, len(tc.replicaSets))
			for _, rs := range tc.replicaSets {
				objs = append(objs, rs)
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
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
			// The pod and quota object names stay platform detail.
			for _, s := range []string{"tenant-quota", "forbidden", "web-"} {
				if strings.Contains(msg, s) {
					t.Errorf("message %q leaks %q", msg, s)
				}
			}
		})
	}
}

// TestQuotaStallMessageTerminal pins both halves of the quota diagnosis: the
// in-flight line says the instance starts once capacity frees, and the failed
// deploy's record restates it as the outcome, through the operator's
// terminalStallMessage and bex-api's TerminalStallText alike.
func TestQuotaStallMessageTerminal(t *testing.T) {
	msg := quotaStallMessage(memoryQuotaVerdict)
	if !strings.Contains(msg, appv1alpha1.QuotaStallWaiting) {
		t.Fatalf("in-flight message %q lacks the waiting phrase", msg)
	}
	final := terminalStallMessage(reasonRolloutBlockedByQuota, msg)
	if strings.Contains(final, appv1alpha1.QuotaStallWaiting) || !strings.Contains(final, appv1alpha1.QuotaStallFailed) {
		t.Fatalf("terminal message = %q", final)
	}
	if shared := appv1alpha1.TerminalStallText(msg); shared != final {
		t.Fatalf("TerminalStallText = %q, want the operator's %q", shared, final)
	}
	// An unparseable verdict still names the cause, without the raw text.
	if got := quotaStallMessage(`exceeded quota: tenant-quota`); !strings.HasPrefix(got, "the workspace's resource quota is full: ") {
		t.Fatalf("fallback message = %q", got)
	}
}

// TestReportRolloutProgressStampsQuotaBlock pins the wiring: a quota-blocked
// rollout's App Ready condition carries RolloutBlockedByQuota (the reason
// bex-api projects onto the open deploy row), not the opaque
// RolloutProgressing — also while the release's earlier template serves.
func TestReportRolloutProgressStampsQuotaBlock(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)

	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", Generation: 2},
	}
	serving := quotaTestReplicaSet("rev-2", 1, 1)
	surge := quotaTestReplicaSet("rev-2", 2, 0, quotaFailedCondition(memoryQuotaVerdict))
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, serving, surge).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme}

	if _, err := r.reportRolloutProgress(context.Background(), app, quotaTestDeployment(), 1, 3000, "waiting"); err != nil {
		t.Fatalf("reportRolloutProgress: %v", err)
	}
	ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Reason != reasonRolloutBlockedByQuota {
		t.Fatalf("Ready condition = %+v, want RolloutBlockedByQuota", ready)
	}
	if !strings.Contains(ready.Message, "memory: 5888Mi of 6Gi in use") {
		t.Errorf("message does not carry the quota numbers: %q", ready.Message)
	}
}
