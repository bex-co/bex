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

package postgres

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestARestartFreePostgresChangeReadsAvailable (w4/364, mirroring w4/m233's
// Key Value rule): an external-access or display-name save restarts nothing —
// CNPG re-signs the server certificate and reloads — so the status must not
// read config_restart while the operator catches up to that generation.
// Before the fix every Ready database read config_restart from the save's own
// response until the operator observed the new generation.
func TestARestartFreePostgresChangeReadsAvailable(t *testing.T) {
	ready := func(observed int64) []metav1.Condition {
		return []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: appv1alpha1.ReasonProvisioned, ObservedGeneration: observed}}
	}
	db := func(generation, observed int64, marked string) *appv1alpha1.Database {
		d := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders", Generation: generation},
			Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "free"},
		}
		d.Status.Phase = appv1alpha1.DBPhaseReady
		d.Status.Provisioned = true
		d.Status.Conditions = ready(observed)
		if marked != "" {
			d.Annotations = map[string]string{annotationRestartFreeGeneration: marked}
		}
		return d
	}
	for name, tc := range map[string]struct {
		d    *appv1alpha1.Database
		want string
	}{
		"marked, one generation pending":   {db(5, 4, "5"), core.DatastoreAvailable},
		"unmarked, one generation pending": {db(5, 4, ""), core.DatastoreConfigRestart},
		"marked, two generations pending":  {db(5, 3, "5"), core.DatastoreConfigRestart},
		"marked for an older generation":   {db(5, 4, "4"), core.DatastoreConfigRestart},
		"observed":                         {db(5, 5, ""), core.DatastoreAvailable},
	} {
		if got := dbStatus(tc.d); got != tc.want {
			t.Errorf("%s: status %q, want %q", name, got, tc.want)
		}
	}

	// The write decides: only the display name, the allow-list layers and
	// external access are restart-free. The PostgresPatch IPAllowList field
	// also flips Public (non-empty = on), which is the same restart-free
	// external-access change.
	rules := []core.IPAllowListEntry{{CIDRBlock: "203.0.113.7/32"}}
	empty := []core.IPAllowListEntry{}
	for name, tc := range map[string]struct {
		patch  PostgresPatch
		marked bool
	}{
		"empty the allow-list": {PostgresPatch{IPAllowList: &empty}, true},
		"change the rules":     {PostgresPatch{IPAllowList: &[]core.IPAllowListEntry{{CIDRBlock: "203.0.113.9/32"}}}, true},
		"rename":               {PostgresPatch{Name: new("orders-2")}, true},
		"public off":           {PostgresPatch{Public: new(false)}, true},
		"plan change":          {PostgresPatch{Plan: new("basic-256mb")}, false},
		"storage change":       {PostgresPatch{DiskSizeGB: new(int32(5))}, false},
		"parameter change":     {PostgresPatch{ParameterOverrides: &map[string]string{"work_mem": "8MB"}}, false},
		"pooler change":        {PostgresPatch{Pooler: new(true)}, false},
		"rules plus plan":      {PostgresPatch{IPAllowList: &empty, Plan: new("basic-256mb")}, false},
	} {
		before := db(7, 7, "")
		before.Spec.Public = true
		before.Spec.IPAllowList = core.AllowListToSpec(rules)
		after := before.DeepCopy()
		tc.patch.apply(after)
		markRestartFree(before, after)
		if got := after.Annotations[annotationRestartFreeGeneration] == "8"; got != tc.marked {
			t.Errorf("%s: marked %v, want %v (%v)", name, got, tc.marked, after.Annotations)
		}
	}

	// Saving the same rules bumps no generation: marking it would let the
	// next, restarting write read available.
	same := db(7, 7, "")
	same.Spec.Public = true
	same.Spec.IPAllowList = core.AllowListToSpec(rules)
	after := same.DeepCopy()
	PostgresPatch{IPAllowList: &rules}.apply(after)
	markRestartFree(same, after)
	if marked, ok := after.Annotations[annotationRestartFreeGeneration]; ok {
		t.Errorf("an unchanged save marked generation %s restart-free", marked)
	}
}

// TestUpdatePostgresMarksRestartFreeWrites: the annotation lands on the
// stored Database through the real update seam for a restart-free patch, and
// never for a restarting one.
func TestUpdatePostgresMarksRestartFreeWrites(t *testing.T) {
	rules := []core.IPAllowListEntry{{CIDRBlock: "203.0.113.7/32"}}
	for name, tc := range map[string]struct {
		patch  PostgresPatch
		marked bool
	}{
		"allow-list": {PostgresPatch{IPAllowList: &rules}, true},
		"plan":       {PostgresPatch{Plan: new("free")}, false},
	} {
		d := databaseForProtection("dpg-orders", "orders", false)
		d.Generation = 3
		svc, cl := newService(d)
		if _, err := svc.UpdatePostgres(context.Background(), d.Name, tc.patch); err != nil {
			t.Fatalf("%s: UpdatePostgres: %v", name, err)
		}
		stored := &appv1alpha1.Database{}
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(d), stored); err != nil {
			t.Fatalf("%s: get stored: %v", name, err)
		}
		if got := stored.Annotations[annotationRestartFreeGeneration] == "4"; got != tc.marked {
			t.Errorf("%s: stored marked %v, want %v (%v)", name, got, tc.marked, stored.Annotations)
		}
	}
}
