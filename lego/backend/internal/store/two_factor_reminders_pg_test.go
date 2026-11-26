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
	"fmt"
	"sync"
	"testing"
	"time"
)

// w1/m647 (migration 0347): a workspace's batch of 2FA reminders claims its
// UTC day once; a later day claims again, an earlier or the same one does not,
// a release gives back only the day it names, and the record goes with the
// workspace.
func TestPGTwoFactorReminderDay(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()
	ws, err := st.CreateTenantWithMember(ctx, fmt.Sprintf("tfr-owner-%d", time.Now().UnixNano()), PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })

	claim := func(at time.Time) bool {
		t.Helper()
		ok, err := st.ClaimTwoFactorReminderDay(ctx, ws.ID, at)
		if err != nil {
			t.Fatalf("claim %s: %v", at, err)
		}
		return ok
	}
	noon := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	if !claim(noon) {
		t.Fatal("the first batch of a day was refused")
	}
	// 23:59Z is the same UTC day, though it is already the 27th in UTC+8.
	if claim(time.Date(2026, 11, 27, 7, 59, 0, 0, time.FixedZone("UTC+8", 8*3600))) {
		t.Fatal("a second batch the same UTC day was claimed")
	}
	if claim(noon.AddDate(0, 0, -1)) {
		t.Fatal("an earlier day was claimed after a later one")
	}

	// Releasing another day leaves today's claim; releasing today's frees it.
	if err := st.ReleaseTwoFactorReminderDay(ctx, ws.ID, noon.AddDate(0, 0, -1)); err != nil {
		t.Fatal(err)
	}
	if claim(noon) {
		t.Fatal("releasing yesterday freed today")
	}
	if err := st.ReleaseTwoFactorReminderDay(ctx, ws.ID, noon); err != nil {
		t.Fatal(err)
	}
	if !claim(noon) {
		t.Fatal("a released day could not be claimed again")
	}

	// 00:01Z the next day claims.
	if !claim(time.Date(2026, 11, 27, 0, 1, 0, 0, time.UTC)) {
		t.Fatal("the next UTC day was refused")
	}

	// Two admins at once: one claim between them.
	day := time.Date(2026, 11, 28, 9, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	results := make([]bool, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := st.ClaimTwoFactorReminderDay(ctx, ws.ID, day)
			if err != nil {
				t.Error(err)
			}
			results[i] = ok
		}()
	}
	wg.Wait()
	won := 0
	for _, ok := range results {
		if ok {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("concurrent claims won = %d, want 1", won)
	}

	if err := st.DeleteTenant(ctx, ws.ID); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM workspace_two_factor_reminders WHERE workspace_id = $1`, ws.ID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("rows after the workspace's delete = %d, %v; want none", left, err)
	}
}
