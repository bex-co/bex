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
	"slices"
	"testing"
)

// w32/m567: the live close writes the URLs the deploy went live at, once. A
// failed close writes none, a row with none reads NULL, and neither another
// close nor the deactivation a newer live deploy brings rewrites them; the
// deploy reads carry them back.
func TestPGLiveURLsAreWrittenOnceByTheLiveClose(t *testing.T) {
	ctx := context.Background()
	pool, st, appID := startedAtPGApp(t, "live-urls")
	isNull := func(id string) bool {
		t.Helper()
		var null bool
		if err := pool.QueryRow(ctx, `SELECT live_urls IS NULL FROM deploys WHERE id = $1`, id).Scan(&null); err != nil {
			t.Fatal(err)
		}
		return null
	}
	first, ok, err := openDeployFor(ctx, st, appID)
	if err != nil || !ok {
		t.Fatalf("open = %v %v", ok, err)
	}
	if !isNull(first.ID) || first.LiveURLs != nil {
		t.Fatalf("an open row records %q", first.LiveURLs)
	}
	went := []string{"https://shop.example", "https://web.onbex.co"}
	if changed, err := st.TransitionDeploy(ctx, first.ID, DeployLive, RanImage{Ref: "nginx@sha256:a", URLs: went}, "", "", "", nil); err != nil || !changed {
		t.Fatalf("live = %v %v", changed, err)
	}
	if got := mustGetDeploy(t, st, appID, first.ID).LiveURLs; !slices.Equal(got, went) {
		t.Fatalf("live_urls = %q, want %q in order", got, went)
	}
	if changed, err := st.TransitionDeploy(ctx, first.ID, DeployLive, RanImage{URLs: []string{"https://other.example"}}, "", "", "", nil); err != nil || changed {
		t.Fatalf("a second live close = %v %v", changed, err)
	}

	// A newer deploy that goes live with no public URL records none, and
	// deactivates the first without touching its record.
	second, err := st.CreateDeploy(ctx, appID, TriggerAPI, "nginx:alpine", first.Generation+1, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := st.TransitionDeploy(ctx, second.ID, DeployLive, RanImage{Ref: "nginx@sha256:b", URLs: []string{}}, "", "", "", nil); err != nil || !changed {
		t.Fatalf("second live = %v %v", changed, err)
	}
	if !isNull(second.ID) {
		t.Fatal("a live close with no URLs stored an empty record, want NULL")
	}
	history, err := st.ListDeploys(ctx, appID, DeployFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range history {
		switch d.ID {
		case first.ID:
			if d.Status != DeployDeactivated || !slices.Equal(d.LiveURLs, went) {
				t.Fatalf("the replaced deploy = %s, %q, want its own record", d.Status, d.LiveURLs)
			}
		case second.ID:
			if d.LiveURLs != nil {
				t.Fatalf("the second deploy reads %q, want none", d.LiveURLs)
			}
		}
	}

	third, err := st.CreateDeploy(ctx, appID, TriggerAPI, "broken:latest", second.Generation+1, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := st.TransitionDeploy(ctx, third.ID, DeployUpdateFailed, RanImage{URLs: went}, "failed", "", "", nil); err != nil || !changed {
		t.Fatalf("failed close = %v %v", changed, err)
	}
	if !isNull(third.ID) {
		t.Fatal("a failed close recorded where it went live")
	}
}
