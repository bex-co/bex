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
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// w1/m657 (ADR115 D11): an org's audit-log key rows. Concurrent mints cannot
// pass the quota together; a revoked row stays as a tombstone that still
// holds the key to the org grant but makes it read nothing; the org's end
// leaves no live key; and a key with no row is not an org key.
func TestOrganizationAPIKeysPG(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	primary, err := st.CreateWorkspace(ctx, "orgkeys-"+stamp, PlanPro, "owner-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), primary.ID) })
	_, org, _, err := st.UpgradeToOrganization(ctx, primary.ID, PlanScale, []string{"owner-" + stamp})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM organization_api_keys WHERE organization_id = $1`, org.ID)
	})
	client := func(i int) string { return fmt.Sprintf("orgkey-%s-%d", stamp, i) }

	// Eight mints race for a quota of three: exactly three rows land.
	const limit = 3
	var wg sync.WaitGroup
	var mu sync.Mutex
	landed, refused := 0, 0
	for i := range 8 {
		wg.Go(func() {
			err := st.InsertOrganizationAPIKey(ctx, OrganizationAPIKey{ClientID: client(i), OrganizationID: org.ID, Label: "k", CreatedBy: "owner-" + stamp}, limit)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				landed++
			case errors.Is(err, ErrOrganizationAPIKeyLimit):
				refused++
			default:
				t.Error(err)
			}
		})
	}
	wg.Wait()
	live, err := st.ListOrganizationAPIKeys(ctx, org.ID)
	if landed != limit || refused != 8-limit || len(live) != limit {
		t.Fatalf("landed %d refused %d live %d, want %d live", landed, refused, len(live), limit)
	}
	key := live[0]
	if key.Type != "AUDIT_LOG" || key.Label != "k" || key.CreatedAt.IsZero() {
		t.Fatalf("row = %+v", key)
	}

	// The key answers its org's grant and is live, for its org only.
	if g, bound, err := st.BoundAPIKeyGrant(ctx, key.ClientID); err != nil || !bound || g.Organization != org.ID || g.All {
		t.Fatalf("grant = %+v bound=%v %v", g, bound, err)
	}
	if got, isLive, err := st.LiveOrganizationAPIKey(ctx, key.ClientID); err != nil || !isLive || got != org.ID {
		t.Fatalf("live = %q %v %v", got, isLive, err)
	}
	if got, err := st.WorkspaceOrganizationID(ctx, primary.ID); err != nil || got != org.ID {
		t.Fatalf("primary's org = %q %v", got, err)
	}
	if got, err := st.WorkspaceOrganizationID(ctx, "tea-none-"+stamp); err != nil || got != "" {
		t.Fatalf("an unknown workspace's org = %q %v", got, err)
	}
	if g, bound, err := st.BoundAPIKeyGrant(ctx, "no-row-"+stamp); err != nil || bound || !g.All || g.Organization != "" {
		t.Fatalf("a client with no row = %+v bound=%v %v", g, bound, err)
	}

	// Revoked: another org cannot revoke it, it stops being live and listed,
	// but its tombstone still holds it to the org grant.
	if err := st.RevokeOrganizationAPIKey(ctx, "org-other", key.ClientID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoke from another org = %v", err)
	}
	if err := st.RevokeOrganizationAPIKey(ctx, org.ID, key.ClientID); err != nil {
		t.Fatal(err)
	}
	if err := st.RevokeOrganizationAPIKey(ctx, org.ID, key.ClientID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a second revoke = %v", err)
	}
	if _, isLive, _ := st.LiveOrganizationAPIKey(ctx, key.ClientID); isLive {
		t.Fatal("a revoked key is live")
	}
	if g, _, _ := st.BoundAPIKeyGrant(ctx, key.ClientID); g.Organization != org.ID || g.All || len(g.Grants) != 0 {
		t.Fatalf("a revoked key's grant = %+v, want its org's", g)
	}
	if live, _ := st.ListOrganizationAPIKeys(ctx, org.ID); len(live) != limit-1 {
		t.Fatalf("after one revoke %d live", len(live))
	}
	// A freed slot takes a new key.
	if err := st.InsertOrganizationAPIKey(ctx, OrganizationAPIKey{ClientID: client(100), OrganizationID: org.ID, Label: "again"}, limit); err != nil {
		t.Fatal(err)
	}

	// The creator's account deletion revokes the keys they made and leaves
	// no trace of them on the row.
	byOther := client(200)
	if err := st.InsertOrganizationAPIKey(ctx, OrganizationAPIKey{ClientID: byOther, OrganizationID: org.ID, Label: "other", CreatedBy: "leaver-" + stamp}, limit+1); err != nil {
		t.Fatal(err)
	}
	if err := st.CleanupAccountSubject(ctx, "leaver-"+stamp, "deleted-"+stamp); err != nil {
		t.Fatal(err)
	}
	var createdBy string
	var tombstoned bool
	if err := pool.QueryRow(ctx, `SELECT created_by, revoked_at IS NOT NULL FROM organization_api_keys WHERE client_id = $1`, byOther).Scan(&createdBy, &tombstoned); err != nil || createdBy != "deleted-"+stamp || !tombstoned {
		t.Fatalf("after the creator's deletion: created_by %q revoked %v (%v)", createdBy, tombstoned, err)
	}

	// The org's end: its keys are not live once the org is gone, and the
	// purge retires every live row.
	other := live[1].ClientID
	if _, err := pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, org.ID); err != nil {
		t.Fatal(err)
	}
	if _, isLive, _ := st.LiveOrganizationAPIKey(ctx, other); isLive {
		t.Fatal("a key of a deleted org is live")
	}
	revoked, err := st.RevokeOrganizationAPIKeys(ctx, org.ID)
	if err != nil || len(revoked) != limit {
		t.Fatalf("purge revoked %v %v, want %d", revoked, err, limit)
	}
	if live, _ := st.ListOrganizationAPIKeys(ctx, org.ID); len(live) != 0 {
		t.Fatalf("after the purge %d live", len(live))
	}
}
