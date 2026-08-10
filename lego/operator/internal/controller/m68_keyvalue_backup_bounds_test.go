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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/m68 F10 — a tenant backup cannot spend unbounded node disk.
//
// The KeyValue backup Job snapshots the whole dataset into an EmptyDir, then
// gzips it (both files coexist during compression) and optionally age-encrypts
// it. That volume had no SizeLimit and its containers declared no
// ephemeral-storage, so a large or incompressible dataset could push a shared
// node into disk pressure and evict unrelated workloads — while the sibling
// pg_dump export path (database_exports.go) has bounded its work volume all
// along.

func starterValkeyTier() tiers.ValkeyTier {
	plan, ok := tiers.Valkey.ByID("starter")
	if !ok {
		panic("starter Valkey tier missing from the catalog")
	}
	return plan
}

func valkeyTier(t *testing.T, id string) tiers.ValkeyTier {
	t.Helper()
	plan, ok := tiers.Valkey.ByID(id)
	if !ok {
		t.Fatalf("Valkey tier %q missing from the catalog", id)
	}
	return plan
}

// TestKeyValueBackupWorkVolumeIsBounded is the core assertion: every generated
// backup Job carries a bounded work volume AND ephemeral-storage on every
// container that can write to it. Both halves matter — an EmptyDir's usage
// counts against the pod's ephemeral-storage limit, so a SizeLimit without
// container limits still leaves the node's eviction manager as the only guard.
func TestKeyValueBackupWorkVolumeIsBounded(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		r := &KeyValueReconciler{Backup: testKeyValueBackupStore}
		if encrypted {
			r.Backup.AgePublicKey = "age1testkeytestkeytestkeytestkeytestkeytestkeytestkey"
		}
		kv := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: "kv-bounded", Namespace: "default", UID: "kv-uid"},
			Spec:       appv1alpha1.KeyValueSpec{Plan: "starter", Version: "8"},
		}
		pod := r.keyValueBackupCronJobSpec(kv, starterValkeyTier(), "kv-auth").JobTemplate.Spec.Template.Spec

		var work *corev1.Volume
		for i := range pod.Volumes {
			if pod.Volumes[i].Name == "backup" {
				work = &pod.Volumes[i]
			}
		}
		if work == nil || work.EmptyDir == nil {
			t.Fatalf("encrypted=%v: no backup EmptyDir on the Job", encrypted)
		}
		if work.EmptyDir.SizeLimit == nil || work.EmptyDir.SizeLimit.IsZero() {
			t.Errorf("encrypted=%v: backup EmptyDir has no SizeLimit — a tenant dataset can fill the node", encrypted)
		}

		all := append(append([]corev1.Container{}, pod.InitContainers...), pod.Containers...)
		if len(all) < 3 {
			t.Fatalf("encrypted=%v: expected at least snapshot+compress+upload, got %d", encrypted, len(all))
		}
		for _, c := range all {
			req, hasReq := c.Resources.Requests[corev1.ResourceEphemeralStorage]
			lim, hasLim := c.Resources.Limits[corev1.ResourceEphemeralStorage]
			if !hasReq || req.IsZero() || !hasLim || lim.IsZero() {
				t.Errorf("encrypted=%v: container %q declares no ephemeral-storage request/limit", encrypted, c.Name)
			}
			// The limit must cover the volume, or the pod is evicted before the
			// volume bound it was sized for is ever reached.
			if hasLim && lim.Cmp(*work.EmptyDir.SizeLimit) < 0 {
				t.Errorf("encrypted=%v: container %q ephemeral limit %s < volume SizeLimit %s",
					encrypted, c.Name, lim.String(), work.EmptyDir.SizeLimit.String())
			}
		}
	}
}

// TestKeyValueBackupBudgetScalesWithTheInstance pins the derivation rather than
// a magic number: a bigger instance gets a bigger budget, and a small one does
// NOT get the large one's ceiling — which is the point of deriving per instance
// instead of picking one global constant.
func TestKeyValueBackupBudgetScalesWithTheInstance(t *testing.T) {
	small := &appv1alpha1.KeyValue{Spec: appv1alpha1.KeyValueSpec{Plan: "starter"}}
	large := &appv1alpha1.KeyValue{Spec: appv1alpha1.KeyValueSpec{Plan: "standard"}}

	smallBudget := keyValueBackupWorkBudget(small, valkeyTier(t, "starter"))
	largeBudget := keyValueBackupWorkBudget(large, valkeyTier(t, "standard"))

	if smallBudget.Cmp(largeBudget) >= 0 {
		t.Errorf("starter budget %s >= standard budget %s — the bound is not instance-derived",
			smallBudget.String(), largeBudget.String())
	}

	// 2*S + 1 GiB headroom, S = the plan's storage.
	wantSmall := int64(2*1+1) * (1 << 30)
	if got := smallBudget.Value(); got != wantSmall {
		t.Errorf("starter budget = %d bytes, want %d (2*storage + headroom)", got, wantSmall)
	}
	wantLarge := int64(2*5+1) * (1 << 30)
	if got := largeBudget.Value(); got != wantLarge {
		t.Errorf("standard budget = %d bytes, want %d", got, wantLarge)
	}
}

// TestKeyValueBackupBudgetFollowsGrownStorage covers disk autoscaling: an
// instance whose PVC has grown past its plan's base size must get a budget for
// the storage it ACTUALLY has, or its backups start failing the moment the
// dataset outgrows the plan default.
func TestKeyValueBackupBudgetFollowsGrownStorage(t *testing.T) {
	grown := &appv1alpha1.KeyValue{
		Spec:   appv1alpha1.KeyValueSpec{Plan: "starter", StorageGB: 20},
		Status: appv1alpha1.KeyValueStatus{AllocatedStorageGB: 40},
	}
	budget := keyValueBackupWorkBudget(grown, valkeyTier(t, "starter"))
	if want := int64(2*40+1) * (1 << 30); budget.Value() != want {
		t.Errorf("grown-instance budget = %d, want %d (allocated storage wins over spec and plan)",
			budget.Value(), want)
	}
}

// The pipeline's own shape (stages, hardening, auth) stays covered by
// TestKeyValueBackupCronJobSpec, which now runs through this budget path — no
// second copy of those assertions here.
