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

// w2/074: the reconnection note marks a boundary. A Blueprint connected,
// synced, disconnected and reconnected twice — once through Render's planned
// create, once through bex's legacy create — has exactly two notes in its
// history, each on the first sync of the connection it starts.
func TestReconnectedTwiceRecordsTwoBoundaryNotesPG(t *testing.T) {
	svc, _, _, ctx, wsID := newRecreatePGService(t)
	repo := "https://github.com/acme/reconnect-" + wsID
	var firsts []string // the first sync id of each connection, oldest first

	connect := func(name string, legacy bool) string {
		t.Helper()
		svc.GitFetcher = fakeBlueprintFetcher{contents: manifestNaming(name)}
		var bpID string
		if legacy {
			v, err := svc.CreateBlueprint(ctx, wsID, CreateBlueprintRequest{Repo: repo, Branch: "main", Name: "bp-" + name})
			if err != nil {
				t.Fatalf("legacy create %s: %v", name, err)
			}
			bpID = v.ID
		} else {
			res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "bp-" + name, OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo}})
			if err != nil || res.Sync == nil {
				t.Fatalf("create %s = %+v, %v; want a planned sync", name, res, err)
			}
			bpID = res.Blueprint.ID
			if _, err := svc.ApproveBlueprint(ctx, bpID, wsID, nil); err != nil {
				t.Fatalf("approve %s: %v", name, err)
			}
		}
		history, err := svc.ListBlueprintSyncs(ctx, bpID, wsID, "", 100)
		if err != nil || len(history) == 0 {
			t.Fatalf("history after connecting %s = %v, %v", name, history, err)
		}
		firsts = append(firsts, history[0].ID)
		// Later syncs of the same connection.
		for range 2 {
			if _, err := svc.SyncBlueprint(ctx, bpID, wsID, "", "", nil); err != nil {
				t.Fatalf("sync %s: %v", name, err)
			}
		}
		return bpID
	}

	bp := connect("api1", false)
	for i, legacy := range []bool{false, true} {
		if err := svc.DisconnectBlueprint(ctx, bp, wsID); err != nil {
			t.Fatal(err)
		}
		if again := connect([]string{"api2", "api3"}[i], legacy); again != bp {
			t.Fatalf("reconnect %d = %s, want the revived row %s", i+1, again, bp)
		}
	}

	history, err := svc.ListBlueprintSyncs(ctx, bp, wsID, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	var noted []string
	for _, run := range history {
		t.Logf("%s %s note=%q", run.ID, run.State, run.Note)
		if strings.Contains(run.Note, "re-established a previously disconnected connection") {
			noted = append(noted, run.ID)
		}
	}
	// Newest first: the second reconnection's boundary, then the first's.
	if want := []string{firsts[2], firsts[1]}; strings.Join(noted, ",") != strings.Join(want, ",") {
		t.Fatalf("reconnection notes on %v, want exactly the two boundaries %v", noted, want)
	}
}
