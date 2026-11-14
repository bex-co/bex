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
	"strings"
	"testing"
)

// manifestNaming plans a create of one web service named name.
func manifestNaming(name string) string {
	return strings.Replace(recreateManifest, "name: api", "name: "+name, 1)
}

// w8/m153 (was w2/074's reconnection boundary): a source connected, synced,
// deleted and connected again twice — once through Render's planned create,
// once through bex's one-step create — is three Blueprints, not one row
// revived twice. Each has its own id and only its own syncs in its history, so
// there is no boundary to mark and no run carries a reconnection note.
func TestConnectedThreeTimesIsThreeBlueprintsPG(t *testing.T) {
	svc, _, pool, ctx, wsID := newRecreatePGService(t)
	repo := "https://github.com/acme/reconnect-" + wsID

	connect := func(name string, legacy bool) string {
		t.Helper()
		svc.GitFetcher = fakeBlueprintFetcher{contents: manifestNaming(name)}
		var bpID string
		if legacy {
			v, err := svc.CreateBlueprint(ctx, wsID, CreateBlueprintRequest{Repo: repo, Branch: "main", Name: "bp-" + name})
			if err != nil {
				t.Fatalf("one-step create %s: %v", name, err)
			}
			bpID = v.ID
		} else {
			res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "bp-" + name, OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo}})
			if err != nil || res.Sync == nil {
				t.Fatalf("create %s = %+v, %v; want a planned sync", name, res, err)
			}
			bpID = res.Blueprint.ID
			if _, err := svc.ApproveBlueprint(ctx, bpID, wsID, "", nil); err != nil {
				t.Fatalf("approve %s: %v", name, err)
			}
		}
		// Later syncs of the same Blueprint.
		for range 2 {
			if _, err := svc.SyncBlueprint(ctx, bpID, wsID, "", "", nil); err != nil {
				t.Fatalf("sync %s: %v", name, err)
			}
		}
		history, err := svc.ListBlueprintSyncs(ctx, bpID, wsID, "", 100)
		if err != nil || len(history) != 3 {
			t.Fatalf("history of %s = %d runs (%v), want its own three: the first sync and two more", name, len(history), err)
		}
		for _, run := range history {
			if run.Note != "" {
				t.Fatalf("%s: run %s carries the note %q; a new Blueprint has no earlier connection to mark off", name, run.ID, run.Note)
			}
		}
		return bpID
	}

	seen := map[string]bool{}
	bp := connect("api1", false)
	seen[bp] = true
	for i, legacy := range []bool{false, true} {
		if err := svc.DisconnectBlueprint(ctx, bp, wsID); err != nil {
			t.Fatal(err)
		}
		bp = connect([]string{"api2", "api3"}[i], legacy)
		if seen[bp] {
			t.Fatalf("connection %d reused the Blueprint id %s of a deleted one", i+2, bp)
		}
		seen[bp] = true
	}

	var rows, live int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE status != 'disconnected')
		FROM blueprints WHERE tenant_id = $1 AND repo = $2`, wsID, repo).Scan(&rows, &live); err != nil {
		t.Fatal(err)
	}
	if rows != 3 || live != 1 {
		t.Fatalf("rows for the source = %d (%d live), want three with only the last one live", rows, live)
	}
}
