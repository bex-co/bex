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

package apps

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestListBlueprintSyncsClampsLikeEverySiblingList pins the page bounds of the
// blueprint-syncs list.
//
// Pins the contract in core.PageLimitOrAbsent's doc for this list: an absent
// or negative limit is the default page, an oversized one clamps to the MAX.
// Before w2/m71 the oversized arm was folded in with the absent one, so asking
// for 500 syncs got the 20-row default — a fifth of the 100 every sibling
// service returns for the same input, and no more than asking for nothing.
func TestListBlueprintSyncsClampsLikeEverySiblingList(t *testing.T) {
	fs := newFakeBlueprintStore(store.Blueprint{ID: "bp-1", TenantID: "tea-a", Name: "stack"})
	svc := newBlueprintService(fs, fakeWorkspace{"user-a": "tea-a"})
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})

	for _, tc := range []struct {
		name  string
		limit int
		want  int
	}{
		{"absent uses the default page", 0, core.DefaultPageLimit},
		{"a negative is treated as absent", -5, core.DefaultPageLimit},
		{"an in-range limit passes through", 50, 50},
		{"exactly the max passes through", core.MaxPageLimit, core.MaxPageLimit},
		{"an oversized limit clamps to the max, not down to the default",
			core.MaxPageLimit + 400, core.MaxPageLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Reset the recorder rather than rebuild the service: newBlueprintService
			// stands up a fake Kubernetes client (a full scheme registration) that this
			// verb never touches, and it dominated the test ~1100:1. The sentinel keeps
			// a missed store call failing — no expected value is 0.
			fs.gotSyncLimit = 0

			if _, err := svc.ListBlueprintSyncs(ctx, "bp-1", "tea-a", "", tc.limit); err != nil {
				t.Fatalf("ListBlueprintSyncs(limit=%d): %v", tc.limit, err)
			}
			if fs.gotSyncLimit != tc.want {
				t.Errorf("store saw limit = %d, want %d (caller asked for %d)",
					fs.gotSyncLimit, tc.want, tc.limit)
			}
		})
	}
}
