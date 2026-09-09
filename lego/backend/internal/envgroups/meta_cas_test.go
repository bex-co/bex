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
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestContentSavePreservesConcurrentRename(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc := newService(store)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name:    "shared",
		EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var armed atomic.Bool
	entered, proceed := make(chan struct{}), make(chan struct{})
	store.afterGetVersioned = func(path string, _ core.SecretKVSnapshot) {
		if !strings.HasSuffix(path, "/revision") || !armed.CompareAndSwap(false, true) {
			return
		}
		close(entered)
		<-proceed
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
	ctx := context.Background()
	store := &failOneFilesPutStore{fakeStore: newFakeStore()}
	svc := newService(store)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name:        "shared",
		EnvVars:     []CreateEnvVarInput{{Key: "A", Value: "before-env"}},
		SecretFiles: []SecretFileView{{Name: "a.pem", Content: "before-file"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var armed atomic.Bool
	entered, proceed := make(chan struct{}), make(chan struct{})
	store.afterGetVersioned = func(path string, _ core.SecretKVSnapshot) {
		if !strings.HasSuffix(path, "/revision") || !armed.CompareAndSwap(false, true) {
			return
		}
		close(entered)
		<-proceed
	}
	store.fail = true

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
}

func TestConcurrentLinksPreserveBothMemberships(t *testing.T) {
	ctx := context.Background()
	web, worker := sampleApp("web"), sampleApp("worker")
	store := newFakeStore()
	cl := fakeClient(web, worker)
	base := func() *Service {
		return &Service{
			Base:  &core.Base{Client: cl, Namespace: "default", Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }},
			Store: store,
		}
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
	ctx := context.Background()
	web, gone := sampleApp("web"), sampleApp("gone")
	store := newFakeStore()
	svc := newService(store, web, gone)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", ServiceIDs: []string{"web", "gone"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Client.Delete(ctx, gone); err != nil {
		t.Fatal(err)
	}

	var armed atomic.Bool
	entered, proceed := make(chan struct{}), make(chan struct{})
	store.afterGetVersioned = func(path string, _ core.SecretKVSnapshot) {
		if !strings.HasSuffix(path, "/meta") || !armed.CompareAndSwap(false, true) {
			return
		}
		close(entered)
		<-proceed
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.pruneStaleLinks(ctx, group.ID, meta{
			workspace: "",
			links:     []string{"web", "gone"},
		}, []string{"gone"})
	}()
	<-entered
	worker := sampleApp("worker")
	if err := svc.Client.Create(ctx, worker); err != nil {
		t.Fatal(err)
	}
	linker := newService(store, web, worker)
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

func TestDelayedMetaMutationDoesNotResurrectDeletedGroup(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc := newService(store, sampleApp("web"))
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "ephemeral"})
	if err != nil {
		t.Fatal(err)
	}
	gid, workspace := group.ID, group.OwnerID
	if err := svc.DeleteEnvGroup(ctx, gid); err != nil {
		t.Fatal(err)
	}
	_, err = svc.mutateMetaCAS(ctx, gid, workspace, func(cur meta) (meta, error) {
		cur.updatedAt = "should-not-land"
		return cur, nil
	})
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("mutate after delete = %v, want ErrNotFound", err)
	}
	if err := svc.writeMeta(ctx, gid, meta{name: "zombie", workspace: workspace}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("writeMeta after delete = %v, want ErrNotFound", err)
	}
	_, err = svc.GetEnvGroup(ctx, gid)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetEnvGroup after delete = %v, want ErrNotFound", err)
	}
}

func TestConcurrentRenamesLeaveOneCommittedName(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	creator := newService(store)
	group, err := creator.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "original"})
	if err != nil {
		t.Fatal(err)
	}
	arrived := make(chan struct{}, 2)
	goon := make(chan struct{})
	store.afterGetVersioned = func(path string, _ core.SecretKVSnapshot) {
		if !strings.HasSuffix(path, "/meta") {
			return
		}
		select {
		case arrived <- struct{}{}:
			<-goon
		default:
			// Retries after the initial overlapping snapshot proceed.
		}
	}
	services := []*Service{newService(store), newService(store)}
	start := make(chan struct{})
	errs := make(chan error, 2)
	names := []string{"alpha", "bravo"}
	for i, svc := range services {
		name := names[i]
		go func() {
			<-start
			_, renameErr := svc.RenameEnvGroup(ctx, group.ID, name)
			errs <- renameErr
		}()
	}
	close(start)
	<-arrived
	<-arrived
	close(goon)
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
	if err := creator.claimGroupName(ctx, got.OwnerID, other, "evg-other0000000000001"); err != nil {
		t.Fatalf("losing name should be free: %v", err)
	}
}

func TestMutateMetaCASRejectsHardConflictWithoutClobber(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc := newService(store)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	var attempts atomic.Int32
	entered, proceed := make(chan struct{}), make(chan struct{})
	store.afterGetVersioned = func(path string, _ core.SecretKVSnapshot) {
		if !strings.HasSuffix(path, "/meta") || attempts.Add(1) != 1 {
			return
		}
		close(entered)
		<-proceed
	}
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.mutateMetaCAS(ctx, group.ID, group.OwnerID, func(cur meta) (meta, error) {
			if cur.name != "shared" {
				return meta{}, envGroupMetadataConflict()
			}
			cur.name = "from-stale"
			return cur, nil
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
