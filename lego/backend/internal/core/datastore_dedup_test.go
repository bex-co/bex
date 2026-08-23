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

package core

import (
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// datastore_dedup_test.go covers the read-side half of the ADR043 D8 cutover
// window: between Step 5 and Step 9 of
// docs/runbooks/datastore-namespace-cutover.md a workspace legitimately has TWO
// Database (or KeyValue) CRs with the SAME metadata.name — the live one in its
// own `<ws>` namespace, the stale one in the shared namespace — and every
// label-scoped List (DatastoreListOptions) returns both. DedupDatabaseTwins /
// DedupKeyValueTwins collapse that to one item per id, preferring the live
// copy, so list views and Blueprint resolution stop seeing twins. The App side
// already holds this policy in store's indexManagedApps; these tests pin that
// the datastore side agrees.

// TestDedupDatabaseTwinsPrefersTheOwnWorkspaceCopy runs the twin in both list
// orders: the winner must be decided by placement (own workspace namespace),
// never by which copy the API server happened to return first.
func TestDedupDatabaseTwinsPrefersTheOwnWorkspaceCopy(t *testing.T) {
	live := tenantDatabase("tea-a", "dpg-x", "tea-a")
	live.Spec.Plan = "standard"
	stale := tenantDatabase("default", "dpg-x", "tea-a")
	stale.Spec.Plan = "free"

	for _, tc := range []struct {
		name  string
		items []appv1alpha1.Database
	}{
		{"stale listed first", []appv1alpha1.Database{*stale, *live}},
		{"live listed first", []appv1alpha1.Database{*live, *stale}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := DedupDatabaseTwins(tc.items)
			if len(got) != 1 {
				t.Fatalf("DedupDatabaseTwins returned %d items, want 1 — a mid-cutover twin must not surface twice", len(got))
			}
			if got[0].Namespace != "tea-a" || got[0].Spec.Plan != "standard" {
				t.Errorf("kept %s/%s (plan %q), want the live tea-a copy — its fields are what connection details must come from",
					got[0].Namespace, got[0].Name, got[0].Spec.Plan)
			}
		})
	}
}

// TestDedupKeyValueTwinsPrefersTheOwnWorkspaceCopy is
// TestDedupDatabaseTwinsPrefersTheOwnWorkspaceCopy's KeyValue twin.
func TestDedupKeyValueTwinsPrefersTheOwnWorkspaceCopy(t *testing.T) {
	live := tenantKeyValue("tea-a", "red-x", "tea-a")
	live.Spec.Plan = "standard"
	stale := tenantKeyValue("default", "red-x", "tea-a")
	stale.Spec.Plan = "free"

	got := DedupKeyValueTwins([]appv1alpha1.KeyValue{*stale, *live})
	if len(got) != 1 {
		t.Fatalf("DedupKeyValueTwins returned %d items, want 1", len(got))
	}
	if got[0].Namespace != "tea-a" || got[0].Spec.Plan != "standard" {
		t.Errorf("kept %s/%s (plan %q), want the live tea-a copy", got[0].Namespace, got[0].Name, got[0].Spec.Plan)
	}
}

// TestDedupDatabaseTwinsKeepsFirstSeenOtherwise pins the two non-twin arms:
// distinct names are not duplicates at all (an un-migrated datastore in the
// shared namespace must survive — dropping it would strand every pre-cutover
// tenant), and when NEITHER copy is in the canonical namespace there is no
// live copy to prefer, so the first-seen one wins deterministically.
func TestDedupDatabaseTwinsKeepsFirstSeenOtherwise(t *testing.T) {
	own := tenantDatabase("tea-a", "dpg-a", "tea-a")
	unmigrated := tenantDatabase("default", "dpg-b", "tea-a")
	got := DedupDatabaseTwins([]appv1alpha1.Database{*own, *unmigrated})
	if len(got) != 2 || got[0].Name != "dpg-a" || got[1].Name != "dpg-b" {
		t.Fatalf("distinct names = %v, want both kept in order — only same-name pairs are twins", namesOf(got))
	}

	first := tenantDatabase("default", "dpg-x", "tea-a")
	second := tenantDatabase("tea-b", "dpg-x", "tea-a") // in neither's canonical spot
	got = DedupDatabaseTwins([]appv1alpha1.Database{*first, *second})
	if len(got) != 1 || got[0].Namespace != "default" {
		t.Fatalf("two non-canonical copies = %v, want the first-seen one kept", namesOf(got))
	}
}

func namesOf(items []appv1alpha1.Database) []string {
	out := make([]string, len(items))
	for i := range items {
		out[i] = items[i].Namespace + "/" + items[i].Name
	}
	return out
}
