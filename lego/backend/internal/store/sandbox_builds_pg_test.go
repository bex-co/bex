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
	if ok, err := st.SucceedSandboxBuild(ctx, first.ID, "sha256:stored", 1500); err != nil || !ok {
		t.Fatalf("succeed = %v, %v", ok, err)
	}
	if ok, err := st.SucceedSandboxBuild(ctx, first.ID, "sha256:other", 1); err != nil || ok {
		t.Fatalf("succeeding a settled build = %v, %v; want false so its import deletes the orphan", ok, err)
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
	if err := st.FailSandboxBuild(ctx, pending[0].ID, "internal_error", "boom"); err != nil {
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

// w3/m367: Render's delete-sandbox-build. A deleted build leaves reads, name
// resolution, the storage gauge and the dedupe (the same name and digest
// imports again as a new build); a repeated delete is not an error; a build
// of another group is not found. A failed build keeps its failureCode.
func TestSandboxBuildDeletePG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-build-delete").ID
	group, err := st.EnsureDefaultSandboxGroup(ctx, ws, 10)
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateSandboxGroup(ctx, ws, "other", SandboxGroupRegion, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	in := NewSandboxBuild{WorkspaceID: ws, SandboxGroupID: group.ID, Name: "py",
		SourceRepository: "docker.io", SourceImage: "library/python", SourceReference: "3.13", SourceDigest: "sha256:aa"}
	b, _, err := st.CreateSandboxBuild(ctx, in, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.StartSandboxBuild(ctx, b.ID); err != nil || !ok {
		t.Fatalf("start = %v, %v", ok, err)
	}
	if ok, err := st.SucceedSandboxBuild(ctx, b.ID, "sha256:img", 4096); err != nil || !ok {
		t.Fatalf("succeed = %v, %v", ok, err)
	}

	if _, err := st.DeleteSandboxBuild(ctx, ws, other.ID, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete from another group = %v, want ErrNotFound", err)
	}
	if _, err := st.DeleteSandboxBuild(ctx, ws, group.ID, "sbb-00000000000000000000"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete of an unknown build = %v, want ErrNotFound", err)
	}
	before, _ := st.SandboxStorageBytes(ctx, ws)
	if deleted, err := st.DeleteSandboxBuild(ctx, ws, group.ID, b.ID); err != nil || !deleted {
		t.Fatalf("delete = %v, %v; want deleted", deleted, err)
	}
	if deleted, err := st.DeleteSandboxBuild(ctx, ws, group.ID, b.ID); err != nil || deleted {
		t.Fatalf("repeated delete = %v, %v; want no error and nothing newly deleted", deleted, err)
	}
	if _, err := st.SandboxBuild(ctx, ws, group.ID, b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get of a deleted build = %v, want ErrNotFound", err)
	}
	if list, err := st.SandboxBuilds(ctx, ws, group.ID, "", nil); err != nil || len(list) != 0 {
		t.Errorf("list = %+v, %v; want the deleted build gone", list, err)
	}
	if _, err := st.NewestSucceededSandboxBuild(ctx, ws, group.ID, "py"); !errors.Is(err, ErrNotFound) {
		t.Errorf("by-name resolution = %v, want ErrNotFound", err)
	}
	// A group or workspace delete still sweeps its image, in case the build's
	// own registry delete failed.
	if ids, err := st.SandboxBuildIDs(ctx, ws, group.ID); err != nil || len(ids) != 1 || ids[0] != b.ID {
		t.Errorf("group image ids = %v, %v; want the deleted build's", ids, err)
	}
	if after, _ := st.SandboxStorageBytes(ctx, ws); after != before-4096 {
		t.Errorf("storage = %d after delete, want %d", after, before-4096)
	}
	// A refused delete is undone.
	if err := st.RestoreSandboxBuild(ctx, ws, group.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SandboxBuild(ctx, ws, group.ID, b.ID); err != nil {
		t.Fatalf("restored build = %v", err)
	}
	if deleted, err := st.DeleteSandboxBuild(ctx, ws, group.ID, b.ID); err != nil || !deleted {
		t.Fatalf("re-delete = %v, %v", deleted, err)
	}
	again, created, err := st.CreateSandboxBuild(ctx, in, 0)
	if err != nil || !created || again.ID == b.ID {
		t.Fatalf("re-import after delete = %s (created %v), %v; want a new build", again.ID, created, err)
	}

	// A running import of a deleted build stops: it cannot be touched or
	// succeeded, so its image is deleted as orphaned.
	if deleted, err := st.DeleteSandboxBuild(ctx, ws, group.ID, again.ID); err != nil || !deleted {
		t.Fatalf("delete pending = %v, %v", deleted, err)
	}
	if ok, _ := st.TouchSandboxBuild(ctx, again.ID); ok {
		t.Error("a deleted import was still touched alive")
	}
	if ok, _ := st.SucceedSandboxBuild(ctx, again.ID, "sha256:img", 1); ok {
		t.Error("a deleted import still succeeded")
	}

	// failureCode is recorded with the failure.
	failed, _, err := st.CreateSandboxBuild(ctx, NewSandboxBuild{WorkspaceID: ws, SandboxGroupID: group.ID, Name: "bad",
		SourceRepository: "docker.io", SourceImage: "library/nope", SourceReference: "1", SourceDigest: "sha256:ff"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FailSandboxBuild(ctx, failed.ID, "image_not_found", "the image could not be copied: 404"); err != nil {
		t.Fatal(err)
	}
	got, err := st.SandboxBuild(ctx, ws, group.ID, failed.ID)
	if err != nil || got.FailureCode == nil || *got.FailureCode != "image_not_found" || got.Status != SandboxBuildFailed {
		t.Fatalf("failed build = %+v, %v", got, err)
	}
}
