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
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w9/m388: a workspace's inbound IP rules. No row reads as unset; a save
// creates the row and a second replaces it; a saved empty list stays distinct
// from no row; the batch read answers every workspace at once; the table's
// structured-row rule refuses an entry without a CIDR; and the row goes with
// its workspace.
func TestPGWorkspaceIPAllowList(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	ws, err := st.CreateTenant(ctx, "ws-ip-rules", PlanScale)
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateTenant(ctx, "ws-ip-rules-other", PlanScale)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, other.ID) })

	if entries, set, err := st.WorkspaceIPAllowList(ctx, ws.ID); err != nil || set || entries != nil {
		t.Fatalf("unset = %v, %v, %v; want no row", entries, set, err)
	}
	office := []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}, {CIDRBlock: "2001:db8::/32", Description: ""}}
	if err := st.SetWorkspaceIPAllowList(ctx, ws.ID, office); err != nil {
		t.Fatal(err)
	}
	if entries, set, err := st.WorkspaceIPAllowList(ctx, ws.ID); err != nil || !set || !slices.Equal(entries, office) {
		t.Fatalf("after save = %v, %v, %v; want %v", entries, set, err, office)
	}
	vpn := []core.IPAllowListEntry{{CIDRBlock: "198.51.100.0/24", Description: "vpn"}}
	if err := st.SetWorkspaceIPAllowList(ctx, ws.ID, vpn); err != nil {
		t.Fatal(err)
	}
	if err := st.SetWorkspaceIPAllowList(ctx, other.ID, nil); err != nil {
		t.Fatal(err)
	}
	lists, err := st.WorkspaceIPAllowLists(ctx, []string{ws.ID, other.ID, "tea-absent"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(lists[ws.ID], vpn) {
		t.Fatalf("replaced list = %v, want %v", lists[ws.ID], vpn)
	}
	if deny, ok := lists[other.ID]; !ok || deny == nil || len(deny) != 0 {
		t.Fatalf("saved empty list = %#v (present %v), want a present empty list", deny, ok)
	}
	if _, ok := lists["tea-absent"]; ok || len(lists) != 2 {
		t.Fatalf("batch read = %v, want exactly the two saved workspaces", lists)
	}
	if err := st.SetWorkspaceIPAllowList(ctx, ws.ID, []core.IPAllowListEntry{{Description: "no cidr"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an entry without a CIDR: want ErrInvalid, got %v", err)
	}
	if err := st.SetWorkspaceIPAllowList(ctx, "tea-absent", vpn); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown workspace: want ErrNotFound, got %v", err)
	}
	if err := st.DeleteTenant(ctx, ws.ID); err != nil {
		t.Fatal(err)
	}
	if _, set, err := st.WorkspaceIPAllowList(ctx, ws.ID); err != nil || set {
		t.Fatalf("after the workspace is deleted: set=%v err=%v; want the row gone", set, err)
	}
}
