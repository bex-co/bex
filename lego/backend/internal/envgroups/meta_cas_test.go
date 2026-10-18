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

package envgroups

import (
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A rename that commits while a content save is between writing its maps and
// releasing them survives the save: the release owns content columns only.
func TestContentSavePreservesConcurrentRename(t *testing.T) {
	ctx := testCtx()
	kv := newFakeStore()
	svc := newService(kv)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name:    "shared",
		EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	groups := svc.fakeGroups()
	var armed atomic.Bool
	entered, proceed := make(chan struct{}), make(chan struct{})
	groups.OnRelease = func(string) {
		if armed.CompareAndSwap(false, true) {
			close(entered)
			<-proceed
		}
	}

	errCh := make(chan error, 1)
	go func() {
		_, patchErr := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
			ExpectedRevision: &group.Revision,
			SaveMode:         SaveModeOnly,
			EnvVars:          []EnvVarPatch{{Key: "TOKEN", Value: "after"}},
		})
		errCh <- patchErr
	}()
	<-entered
	if _, err := svc.RenameEnvGroup(ctx, group.ID, "renamed"); err != nil {
		t.Fatalf("rename during content save: %v", err)
	}
	close(proceed)
	if err := <-errCh; err != nil {
		t.Fatalf("content save: %v", err)
	}
	got, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "renamed" {
		t.Fatalf("name = %q, want renamed (content save must not restore stale meta)", got.Name)
	}
	env, err := svc.GetEnvGroupVar(ctx, group.ID, "TOKEN")
	if err != nil || env.Value != "after" {
		t.Fatalf("TOKEN = %+v err=%v", env, err)
	}
}

func TestContentCompensationPreservesConcurrentRename(t *testing.T) {
	ctx := testCtx()
	kv := &failOneFilesPutStore{fakeStore: newFakeStore()}
	svc := newService(kv)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name:        "shared",
		EnvVars:     []CreateEnvVarInput{{Key: "A", Value: "before-env"}},
		SecretFiles: []SecretFileView{{Name: "a.pem", Content: "before-file"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	groups := svc.fakeGroups()
	var armed atomic.Bool
	entered, proceed := make(chan struct{}), make(chan struct{})
	groups.OnRelease = func(string) {
		if armed.CompareAndSwap(false, true) {
			close(entered)
			<-proceed
		}
	}
	kv.fail = true

	errCh := make(chan error, 1)
	go func() {
		_, patchErr := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
			ExpectedRevision: &group.Revision,
			SaveMode:         SaveModeOnly,
			EnvVars:          []EnvVarPatch{{Key: "A", Value: "failed-env"}},
			SecretFiles:      []SecretFilePatch{{Name: "a.pem", Content: "failed-file"}},
		})
		errCh <- patchErr
	}()
	<-entered
	if _, err := svc.RenameEnvGroup(ctx, group.ID, "kept-name"); err != nil {
		t.Fatalf("rename during failing save: %v", err)
	}
	close(proceed)
	if err := <-errCh; !errors.Is(err, core.ErrConflict) {
		t.Fatalf("patch error = %v, want conflict after restore", err)
	}
	got, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "kept-name" {
		t.Fatalf("name = %q after compensation, want kept-name", got.Name)
	}
	value, err := svc.GetEnvGroupVar(ctx, group.ID, "A")
	if err != nil || value.Value != "before-env" {
		t.Fatalf("A after compensation = %+v err=%v, want the restored before-env", value, err)
	}
}

func TestConcurrentLinksPreserveBothMemberships(t *testing.T) {
	ctx := testCtx()
	web, worker := sampleApp("web"), sampleApp("worker")
	kv := newFakeStore()
	cl := fakeClient(web, worker)
	base := func() *Service {
		return withGroups(&Service{
			Base:  &core.Base{Client: cl, Namespace: "default", Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }},
			Store: kv,
		})
	}
	creator := base()
	group, err := creator.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	services := []*Service{base(), base()}
	start := make(chan struct{})
	errs := make(chan error, 2)
	targets := []string{"web", "worker"}
	for i, svc := range services {
		serviceID := targets[i]
		go func() {
			<-start
			errs <- svc.LinkService(ctx, group.ID, serviceID)
		}()
	}
	close(start)
	for range services {
		if err := <-errs; err != nil {
			t.Fatalf("link: %v", err)
		}
	}
	got, err := creator.GetEnvGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	links := append([]string{}, got.ServiceLinks...)
	slices.Sort(links)
	if !slices.Equal(links, []string{"web", "worker"}) {
		t.Fatalf("links = %v, want both web and worker", got.ServiceLinks)
	}
	for _, name := range targets {
		app := getApp(t, cl, name)
		if !slices.Contains(app.Spec.EnvFromSecrets, envSecretName(group.ID)) {
			t.Fatalf("%s missing env secret ref", name)
		}
	}
}

func TestPruneStaleLinksPreservesConcurrentLink(t *testing.T) {
	ctx := testCtx()
	web, gone := sampleApp("web"), sampleApp("gone")
	kv := newFakeStore()
	svc := newService(kv, web, gone)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", ServiceIDs: []string{"web", "gone"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Client.Delete(ctx, gone); err != nil {
		t.Fatal(err)
	}

	groups := svc.fakeGroups()
	var armed atomic.Bool
	entered, proceed := make(chan struct{}), make(chan struct{})
	groups.AfterGet = func(string) {
		if armed.CompareAndSwap(false, true) {
			close(entered)
			<-proceed
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.pruneStaleLinks(ctx, group.ID, []string{"gone"})
	}()
	<-entered
	worker := sampleApp("worker")
	if err := svc.Client.Create(ctx, worker); err != nil {
		t.Fatal(err)
	}
	linker := newService(kv, web, worker)
	if err := linker.LinkService(ctx, group.ID, "worker"); err != nil {
		t.Fatalf("concurrent link: %v", err)
	}
	close(proceed)
	<-done

	got, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	links := append([]string{}, got.ServiceLinks...)
	slices.Sort(links)
	if !slices.Equal(links, []string{"web", "worker"}) {
		t.Fatalf("links after prune = %v, want web+worker (gone removed, worker kept)", got.ServiceLinks)
	}
}

// A deleted group's id cannot be written again: its row is gone, every later
// metadata or content write is a compare-and-set that matches nothing, and no
// tombstone is needed (w4/m97, w8/m114).
func TestDelayedWritesDoNotResurrectDeletedGroup(t *testing.T) {
	ctx := testCtx()
	kv := newFakeStore()
	svc := newService(kv, sampleApp("web"))
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "ephemeral"})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := svc.fakeGroups().Row(group.ID)
	if err := svc.DeleteEnvGroup(ctx, group.ID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.mutateGroup(ctx, group.ID, func(cur *store.EnvGroup) error {
		cur.Name = "zombie"
		return nil
	})
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("metadata write after delete = %v, want ErrNotFound", err)
	}
	if _, err := svc.claimContent(ctx, before, store.EnvGroupOp{ID: "ego-late", Kind: opKindPatch}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("content claim after delete = %v, want ErrNotFound", err)
	}
	if _, err := svc.GetEnvGroup(ctx, group.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetEnvGroup after delete = %v, want ErrNotFound", err)
	}
}

func TestConcurrentRenamesLeaveOneCommittedName(t *testing.T) {
	ctx := testCtx()
	kv := newFakeStore()
	creator := newService(kv)
	group, err := creator.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "original"})
	if err != nil {
		t.Fatal(err)
	}
	// Both renames read the row before either writes; the second write then
	// finds the name it read already changed.
	var reads atomic.Int32
	both := make(chan struct{})
	creator.fakeGroups().AfterGet = func(string) {
		if n := reads.Add(1); n == 2 {
			close(both)
		} else if n < 2 {
			<-both
		}
	}
	services := []*Service{newService(kv), newService(kv)}
	for _, svc := range services {
		svc.cutoverDone.Store(true)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	names := []string{"alpha", "bravo"}
	for i, svc := range services {
		name := names[i]
		go func() {
			<-start
			_, renameErr := svc.mutateGroup(ctx, group.ID, func(cur *store.EnvGroup) error {
				if cur.Name != "original" {
					return envGroupMetadataConflict()
				}
				cur.Name = name
				return nil
			})
			errs <- renameErr
		}()
	}
	close(start)
	var successes, conflicts int
	for range services {
		err := <-errs
		switch {
		case err == nil:
			successes++
		case errors.Is(err, core.ErrConflict):
			conflicts++
		default:
			t.Fatalf("rename error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
	creator.fakeGroups().AfterGet = nil
	got, err := creator.GetEnvGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "alpha" && got.Name != "bravo" {
		t.Fatalf("committed name = %q", got.Name)
	}
	other := "alpha"
	if got.Name == "alpha" {
		other = "bravo"
	}
	if _, err := creator.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: other}); err != nil {
		t.Fatalf("losing name should be free: %v", err)
	}
	if _, err := creator.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: got.Name}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("committed name should be taken: %v", err)
	}
}

func TestMutateGroupRejectsHardConflictWithoutClobber(t *testing.T) {
	ctx := testCtx()
	kv := newFakeStore()
	svc := newService(kv)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	var attempts atomic.Int32
	entered, proceed := make(chan struct{}), make(chan struct{})
	svc.fakeGroups().AfterGet = func(string) {
		if attempts.Add(1) == 1 {
			close(entered)
			<-proceed
		}
	}
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.mutateGroup(ctx, group.ID, func(cur *store.EnvGroup) error {
			if cur.Name != "shared" {
				return envGroupMetadataConflict()
			}
			cur.Name = "from-stale"
			return nil
		})
		errCh <- err
	}()
	<-entered
	if _, err := svc.RenameEnvGroup(ctx, group.ID, "winner"); err != nil {
		t.Fatal(err)
	}
	close(proceed)
	if err := <-errCh; !errors.Is(err, core.ErrConflict) {
		t.Fatalf("stale rename = %v, want metadata conflict", err)
	}
	got, _ := svc.GetEnvGroup(ctx, group.ID)
	if got.Name != "winner" {
		t.Fatalf("name = %q, want winner", got.Name)
	}
}
