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

package store

import (
	"context"
	"maps"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w8/382: a Blueprint's env group claims follow a group by id, are not
// exclusive, are replaced whole by each record, keep only groups of the same
// workspace that exist, go with a deleted group, and go on disconnect.
func TestPGBlueprintEnvGroupClaims(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	tenant := lifecycleTenant(t, st, "egclaim")
	a := lifecycleBlueprint(t, st, tenant, "egclaim-a")
	b := lifecycleBlueprint(t, st, tenant, "egclaim-b")
	group := func(tenantID, name string) string {
		t.Helper()
		now := time.Now().UTC()
		g, err := st.InsertEnvGroup(ctx, EnvGroup{ID: ids.New(ids.EnvGroup), TenantID: tenantID, Name: name, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		return g.ID
	}
	claimsOf := func(blueprintID string) map[string]string {
		t.Helper()
		got, err := st.BlueprintEnvGroupClaims(ctx, tenant.ID, blueprintID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	shared, other := group(tenant.ID, "shared"), group(tenant.ID, "other")
	foreign := group(lifecycleTenant(t, st, "egclaim-foreign").ID, "shared")

	if err := st.RecordBlueprintEnvGroupClaims(ctx, tenant.ID, a.ID, map[string]string{"shared": shared, "other": other, "foreign": foreign}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordBlueprintEnvGroupClaims(ctx, tenant.ID, b.ID, map[string]string{"shared": shared}); err != nil {
		t.Fatal(err)
	}
	if got, want := claimsOf(a.ID), map[string]string{"shared": shared, "other": other}; !maps.Equal(got, want) {
		t.Fatalf("a's claims = %v, want %v (another workspace's group is not recorded)", got, want)
	}
	if got := claimsOf(b.ID); !maps.Equal(got, map[string]string{"shared": shared}) {
		t.Fatalf("b's claims = %v, want shared too: claims are not exclusive", got)
	}

	// A record is the whole set: a name the file stopped declaring goes, and
	// a name now naming another group follows it.
	if err := st.RecordBlueprintEnvGroupClaims(ctx, tenant.ID, a.ID, map[string]string{"shared": other}); err != nil {
		t.Fatal(err)
	}
	if got := claimsOf(a.ID); !maps.Equal(got, map[string]string{"shared": other}) {
		t.Fatalf("after the second record a's claims = %v, want shared→%s alone", got, other)
	}

	if _, err := st.DeleteEnvGroup(ctx, shared); err != nil {
		t.Fatal(err)
	}
	if got := claimsOf(b.ID); len(got) != 0 {
		t.Fatalf("after the group's delete b's claims = %v, want none", got)
	}
	if err := st.ReleaseBlueprintResourceClaims(ctx, tenant.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if got := claimsOf(a.ID); len(got) != 0 {
		t.Fatalf("after disconnect a's claims = %v, want none", got)
	}
	if err := st.RecordBlueprintEnvGroupClaims(ctx, tenant.ID, a.ID, nil); err != nil {
		t.Fatalf("an empty record: %v", err)
	}
}
