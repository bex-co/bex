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
	"strings"
	"testing"
	"time"
)

// w2/m358: an import of a name and digest the group already has returns that
// build unless it failed, the in-flight limit counts only pending and running
// builds, name resolution picks the newest succeeded build, a stale import is
// failed, and succeeded images count toward the storage cap.
func TestSandboxBuildsPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-builds").ID
	group, err := st.EnsureDefaultSandboxGroup(ctx, ws, 10)
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateSandboxGroup(ctx, ws, "other", SandboxGroupRegion, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	in := func(groupID, name, digest string) NewSandboxBuild {
		return NewSandboxBuild{WorkspaceID: ws, SandboxGroupID: groupID, Name: name,
			SourceRepository: "docker.io", SourceImage: "library/python", SourceReference: "3.13", SourceDigest: digest}
	}

	first, created, err := st.CreateSandboxBuild(ctx, in(group.ID, "py", "sha256:aa"), 2)
	if err != nil || !created || first.Status != SandboxBuildPending || !strings.HasPrefix(first.ID, "sbb-") {
		t.Fatalf("first import = %+v, created %v, %v", first, created, err)
	}
	again, created, err := st.CreateSandboxBuild(ctx, in(group.ID, "py", "sha256:aa"), 2)
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("same name and digest = %s (created %v), %v; want %s again", again.ID, created, err, first.ID)
	}
	if _, created, err := st.CreateSandboxBuild(ctx, in(group.ID, "py", "sha256:bb"), 2); err != nil || !created {
		t.Fatalf("a moved tag (new digest) = created %v, %v; want a new build", created, err)
	}
	if _, _, err := st.CreateSandboxBuild(ctx, in(group.ID, "node", "sha256:cc"), 2); !errors.Is(err, ErrSandboxBuildLimit) {
		t.Fatalf("third in-flight import = %v, want ErrSandboxBuildLimit", err)
	}
	// Another group has its own limit and its own builds.
	if _, created, err := st.CreateSandboxBuild(ctx, in(other.ID, "py", "sha256:aa"), 2); err != nil || !created {
		t.Fatalf("the same image in another group = created %v, %v", created, err)
	}

	if live, err := st.TouchSandboxBuild(ctx, first.ID); err != nil || !live {
		t.Fatalf("touch a pending build = %v, %v; a queued import stays fresh", live, err)
	}
	if ok, err := st.StartSandboxBuild(ctx, first.ID); err != nil || !ok {
		t.Fatalf("start = %v, %v", ok, err)
	}
	if ok, _ := st.StartSandboxBuild(ctx, first.ID); ok {
		t.Fatal("a running build started twice")
	}
	if err := st.SucceedSandboxBuild(ctx, first.ID, "sha256:stored", 1500); err != nil {
		t.Fatal(err)
	}
	if live, err := st.TouchSandboxBuild(ctx, first.ID); err != nil || live {
		t.Fatalf("touch a settled build = %v, %v; want false so its import stops", live, err)
	}
	newest, err := st.NewestSucceededSandboxBuild(ctx, ws, group.ID, "py")
	if err != nil || newest.ID != first.ID || newest.ImageDigest == nil || *newest.ImageDigest != "sha256:stored" || newest.FinishedAt == nil {
		t.Fatalf("newest succeeded py = %+v, %v", newest, err)
	}
	if _, err := st.NewestSucceededSandboxBuild(ctx, ws, other.ID, "py"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other group's py has not succeeded, got %v", err)
	}
	if _, err := st.SandboxBuild(ctx, ws, other.ID, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a build read under another group = %v, want ErrNotFound", err)
	}

	// A failed build frees the name and digest and its in-flight slot.
	pending, err := st.SandboxBuilds(ctx, ws, group.ID, "py", []string{SandboxBuildPending})
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending py builds = %+v, %v", pending, err)
	}
	if err := st.FailSandboxBuild(ctx, pending[0].ID, "boom"); err != nil {
		t.Fatal(err)
	}
	retry, created, err := st.CreateSandboxBuild(ctx, in(group.ID, "py", "sha256:bb"), 2)
	if err != nil || !created || retry.ID == pending[0].ID {
		t.Fatalf("retry after failure = %+v (created %v), %v; want a new build", retry, created, err)
	}

	// A build no replica advances is failed by the sweep, a fresh one is not.
	n, err := st.FailStaleSandboxBuilds(ctx, -time.Minute, "interrupted") // everything is stale
	if err != nil || n < 2 {
		t.Fatalf("stale sweep failed %d, %v; want the retry and the other group's build", n, err)
	}
	got, _ := st.SandboxBuild(ctx, ws, group.ID, retry.ID)
	if got.Status != SandboxBuildFailed || got.FailureReason == nil || *got.FailureReason != "interrupted" {
		t.Fatalf("swept build = %+v", got)
	}
	if n, err := st.FailStaleSandboxBuilds(ctx, time.Hour, "interrupted"); err != nil || n != 0 {
		t.Fatalf("a sweep with nothing stale failed %d, %v", n, err)
	}

	all, err := st.SandboxBuilds(ctx, ws, group.ID, "", nil)
	if err != nil || len(all) != 3 || all[0].ID != retry.ID {
		t.Fatalf("all builds newest first = %d builds (first %v), %v", len(all), all, err)
	}

	// Succeeded images count toward the storage cap; failed ones do not.
	bytes, err := st.SandboxStorageBytes(ctx, ws)
	if err != nil || bytes != 1500 {
		t.Fatalf("storage bytes = %d, %v; want the one succeeded image's 1500", bytes, err)
	}

	// A group's builds go with the group.
	if _, err := st.Pool.Exec(ctx, `DELETE FROM sandbox_groups WHERE id = $1`, other.ID); err != nil {
		t.Fatal(err)
	}
	if rows, err := st.SandboxBuilds(ctx, ws, other.ID, "", nil); err != nil || len(rows) != 0 {
		t.Fatalf("a deleted group's builds = %d, %v", len(rows), err)
	}
	if _, _, err := st.CreateSandboxBuild(ctx, in(other.ID, "py", "sha256:dd"), 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an import into a deleted group = %v, want ErrNotFound", err)
	}
}
