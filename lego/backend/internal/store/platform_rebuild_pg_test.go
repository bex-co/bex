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
	"testing"
	"time"
)

// Only releases the platform opened (deployed_by_render) are exempt from
// tenant build minutes (w1/m184); a tenant's own deploys are not.
func TestPGPlatformRebuildRevisionsListsOnlyPlatformReleases(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ten := lifecycleTenant(t, st, "platform-rebuild")
	row, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: fmt.Sprintf("web-%d", time.Now().UnixNano()), Repo: "https://example.com/acme/web.git",
		Branch: "main", Port: 80, Replicas: 1, Tier: "free",
	})
	if err != nil {
		t.Fatal(err)
	}
	history, err := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(history) != 1 {
		t.Fatalf("CreateApp history = %+v (err %v)", history, err)
	}
	first := history[0].Generation
	if _, err := st.CreateDeploy(ctx, row.ID, TriggerDeployedByRender, "", first+1, CommitInfo{}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", first+2, CommitInfo{}, ""); err != nil {
		t.Fatal(err)
	}

	got, err := st.PlatformRebuildRevisions(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("gen-%d", first+1)
	if len(got) != 1 || !got[want] {
		t.Errorf("platform rebuild revisions = %v, want only %s", got, want)
	}
}
