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

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w2/m359: Render's `sandbox-groups default set` and `sandbox-groups delete`,
// against a real control-plane Postgres.

func (f *groupFixture) listGroups(t *testing.T) []Group {
	t.Helper()
	code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups?ownerId="+f.ws, "")
	var out []GroupEnvelope
	if err := json.Unmarshal([]byte(body), &out); code != http.StatusOK || err != nil {
		t.Fatalf("list groups = %d %s", code, body)
	}
	groups := make([]Group, 0, len(out))
	for _, e := range out {
		groups = append(groups, e.SandboxGroup)
	}
	return groups
}

func TestSetDefaultGroupMovesTheDefault(t *testing.T) {
	f := newGroupFixture(t)
	a := f.createGroup(t, "a")
	b := f.createGroup(t, "b")
	if !a.IsDefault || b.IsDefault {
		t.Fatalf("first group a must be the default: a=%v b=%v", a.IsDefault, b.IsDefault)
	}

	code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups/"+b.ID+"/set-default", "")
	var got Group
	if err := json.Unmarshal([]byte(body), &got); code != http.StatusOK || err != nil || got.ID != b.ID || !got.IsDefault {
		t.Fatalf("set-default b = %d %s", code, body)
	}
	defaults := 0
	for _, g := range f.listGroups(t) {
		if g.IsDefault {
			defaults++
			if g.ID != b.ID {
				t.Fatalf("default is %s, want %s", g.ID, b.ID)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("%d default groups, want exactly 1", defaults)
	}
	// A create that names no group lands in the new default.
	if sb := f.mustCreate(t, ""); sb.SandboxGroupID != b.ID {
		t.Fatalf("create without a group landed in %s, want %s", sb.SandboxGroupID, b.ID)
	}
	// Setting the default again is a no-op that returns it.
	if code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups/"+b.ID+"/set-default", ""); code != http.StatusOK {
		t.Fatalf("set-default on the default = %d %s", code, body)
	}
	for _, tc := range []struct {
		path   string
		status int
		code   string
	}{
		{"/v1/sandbox-groups/nope/set-default", 400, CodeInvalidSandboxGroupID},
		{"/v1/sandbox-groups/sbg-aaaaaaaaaaaaaaaaaaaa/set-default", 404, CodeSandboxGroupNotFound},
	} {
		if code, body := f.do(t, http.MethodPost, tc.path, ""); code != tc.status || errCode(body) != tc.code {
			t.Errorf("%s = %d %s, want %d %s", tc.path, code, body, tc.status, tc.code)
		}
	}
}

func TestDeleteGroupTerminatesItsSandboxesAndDeletesItsImages(t *testing.T) {
	f := newImageFixture(t) // f.group is the workspace's first, so default, group
	f.svc.Images.Reaper = &RegistryReaper{RegistryConfig: RegistryConfig{BaseURL: f.sink.URL, Prefix: DefaultSnapshotRepoPrefix}}
	doomed := f.createGroup(t, "doomed")

	ref := hostOf(f.source) + "/library/py:3.13"
	pushImage(t, ref, linuxAMD64, 256)
	code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups/"+doomed.ID+"/builds", fmt.Sprintf(`{"name":"py","sourceImage":%q}`, ref))
	var b Build
	if err := json.Unmarshal([]byte(body), &b); code != http.StatusCreated || err != nil {
		t.Fatalf("import = %d %s", code, body)
	}
	waitSucceeded(t, f.st, f.ws, doomed.ID, b.ID)
	stored, err := name.ParseReference(fmt.Sprintf("%s/snapshots/%s/images/%s:image", hostOf(f.sink), store.SandboxNamespace(f.ws), b.ID), name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := remote.Head(stored); err != nil {
		t.Fatalf("the image is not stored: %v", err)
	}
	running := f.mustCreate(t, doomed.ID)
	suspended := f.mustCreate(t, doomed.ID)
	f.suspend(t, suspended)
	keep := f.mustCreate(t, "") // in the default group

	if code, body := f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+f.group.ID, ""); code != http.StatusConflict || errCode(body) != CodeDefaultSandboxGroupNotDeletable {
		t.Fatalf("delete the default group = %d %s, want 409 %s", code, body, CodeDefaultSandboxGroupNotDeletable)
	}
	if code, body := f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+doomed.ID, ""); code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", code, body)
	}

	if code, _ := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+doomed.ID, ""); code != http.StatusNotFound {
		t.Fatalf("a deleted group reads %d, want 404", code)
	}
	if slices.ContainsFunc(f.listGroups(t), func(g Group) bool { return g.ID == doomed.ID }) {
		t.Fatal("a deleted group is still listed")
	}
	for _, sb := range []Sandbox{running, suspended} {
		if _, live := f.os.boxes[f.os.idOf(sb.ID)]; live {
			t.Fatalf("sandbox %s of the deleted group was not terminated", sb.ID)
		}
	}
	if _, live := f.os.boxes[f.os.idOf(keep.ID)]; !live {
		t.Fatal("a sandbox of another group was terminated")
	}
	if rows, err := f.st.SandboxBuilds(context.Background(), f.ws, doomed.ID, "", nil); err != nil || len(rows) != 0 {
		t.Fatalf("the deleted group's builds = %d, %v", len(rows), err)
	}
	if _, err := remote.Head(stored); err == nil {
		t.Fatal("the deleted group's image is still in the registry")
	}
	if code, body := f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+doomed.ID, ""); code != http.StatusNotFound {
		t.Fatalf("a second delete = %d %s, want 404", code, body)
	}
	if code, body := f.create(t, doomed.ID); code != http.StatusNotFound || errCode(body) != CodeSandboxGroupNotFound {
		t.Fatalf("a create into the deleted group = %d %s", code, body)
	}
}

// An OpenSandbox outage during the delete still hides the group at once; the
// sweeper finishes the teardown once OpenSandbox answers again.
func TestInterruptedGroupDeleteIsFinishedBySweeper(t *testing.T) {
	f := newGroupFixture(t)
	f.createGroup(t, "default")
	doomed := f.createGroup(t, "doomed")
	sb := f.mustCreate(t, doomed.ID)

	f.os.mu.Lock()
	f.os.terminateStatus = http.StatusServiceUnavailable
	f.os.mu.Unlock()
	if code, body := f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+doomed.ID, ""); code != http.StatusNoContent {
		t.Fatalf("delete during an outage = %d %s", code, body)
	}
	if code, _ := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+doomed.ID, ""); code != http.StatusNotFound {
		t.Fatalf("a group being deleted reads %d, want 404", code)
	}
	if _, live := f.os.boxes[f.os.idOf(sb.ID)]; !live {
		t.Fatal("the outage should have kept the sandbox")
	}

	f.os.mu.Lock()
	f.os.terminateStatus = 0
	f.os.mu.Unlock()
	if err := f.svc.sweepGroupDeletes(context.Background(), -time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, live := f.os.boxes[f.os.idOf(sb.ID)]; live {
		t.Fatal("the sweeper did not terminate the deleted group's sandbox")
	}
	var rows int
	if err := f.st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM sandbox_groups WHERE id = $1`, doomed.ID).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("the sweeper left %d group rows, %v", rows, err)
	}
	// The deleted group's name is free at once.
	if g := f.createGroup(t, "doomed"); g.ID == doomed.ID {
		t.Fatal("re-creating the name returned the deleted group")
	}
}

// waitSucceeded polls the store until a build settles succeeded.
func waitSucceeded(t *testing.T, st *store.PGStore, ws, groupID, id string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		b, err := st.SandboxBuild(context.Background(), ws, groupID, id)
		if err != nil {
			t.Fatal(err)
		}
		switch b.Status {
		case store.SandboxBuildSucceeded:
			return
		case store.SandboxBuildFailed:
			t.Fatalf("build %s failed: %v", id, *b.FailureReason)
		}
		if time.Now().After(deadline) {
			t.Fatalf("build %s still %s", id, b.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A workspace delete removes the workspace's imported images from the
// registry before its rows cascade away.
func TestWorkspacePurgeDeletesImportedImages(t *testing.T) {
	f := newImageFixture(t)
	f.svc.Images.Reaper = &RegistryReaper{RegistryConfig: RegistryConfig{BaseURL: f.sink.URL, Prefix: DefaultSnapshotRepoPrefix}}
	ref := hostOf(f.source) + "/library/py:3.13"
	pushImage(t, ref, linuxAMD64, 256)
	b := f.mustImport(t, fmt.Sprintf(`{"name":"py","sourceImage":%q}`, ref))
	waitSucceeded(t, f.st, f.ws, f.group.ID, b.ID)
	stored, err := name.ParseReference(fmt.Sprintf("%s/snapshots/%s/images/%s:image", hostOf(f.sink), store.SandboxNamespace(f.ws), b.ID), name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&WorkspacePurger{Images: f.svc.Images}).PurgeWorkspace(context.Background(), f.ws); err != nil {
		t.Fatal(err)
	}
	if _, err := remote.Head(stored); err == nil {
		t.Fatal("the deleted workspace's image is still in the registry")
	}
}
