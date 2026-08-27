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
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestPatchEnvironmentStagesMixedOpaqueChangesAndDeploysOnce(t *testing.T) {
	ctx := context.Background()
	web, worker := sampleApp("web"), sampleApp("worker")
	svc := newService(newFakeStore(), web, worker)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name:        "shared",
		EnvVars:     []CreateEnvVarInput{{Key: "KEEP", Value: "opaque"}, {Key: "RENAME", Value: "carry"}, {Key: "DELETE", Value: "gone"}},
		SecretFiles: []SecretFileView{{Name: "old.pem", Content: "file-opaque"}},
		ServiceIDs:  []string{"web", "worker"},
	})
	if err != nil {
		t.Fatal(err)
	}
	beforeWeb, beforeWorker := getApp(t, svc.Client, "web").Spec.RestartedAt, getApp(t, svc.Client, "worker").Spec.RestartedAt
	result, err := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		ExpectedRevision: &group.Revision,
		SaveMode:         SaveModeOnly,
		EnvVars: []EnvVarPatch{
			{Key: "RENAMED", FromKey: "RENAME"},
			{Key: "DELETE", Delete: true},
			{Key: "GENERATED", GenerateValue: true},
		},
		SecretFiles: []SecretFilePatch{{Name: "new.pem", FromName: "old.pem"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.EnvVarKeys, []string{"GENERATED", "KEEP", "RENAMED"}) ||
		!slices.Equal(result.SecretFileNames, []string{"new.pem"}) || result.RolledOut || result.Revision == group.Revision {
		t.Fatalf("save-only result = %+v", result)
	}
	if got := getApp(t, svc.Client, "web").Spec.RestartedAt; got != beforeWeb {
		t.Fatalf("save_only rolled web: before=%q after=%q", beforeWeb, got)
	}
	if got := getApp(t, svc.Client, "worker").Spec.RestartedAt; got != beforeWorker {
		t.Fatalf("save_only rolled worker: before=%q after=%q", beforeWorker, got)
	}
	for key, want := range map[string]string{"KEEP": "opaque", "RENAMED": "carry"} {
		got, revealErr := svc.GetEnvGroupVar(ctx, group.ID, key)
		if revealErr != nil || got.Value != want {
			t.Fatalf("%s reveal = %+v err=%v", key, got, revealErr)
		}
	}
	generated, err := svc.GetEnvGroupVar(ctx, group.ID, "GENERATED")
	if err != nil || len(generated.Value) != 44 {
		t.Fatalf("generated reveal length=%d err=%v", len(generated.Value), err)
	}
	file, err := svc.GetEnvGroupFile(ctx, group.ID, "new.pem")
	if err != nil || file.Content != "file-opaque" {
		t.Fatalf("renamed file = %+v err=%v", file, err)
	}

	svc.Clock = func() time.Time { return time.Unix(2_000_000, 0).UTC() }
	deployed, err := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		ExpectedRevision: &result.Revision, SaveMode: SaveModeDeploy,
	})
	if err != nil || !deployed.RolledOut || len(deployed.FailedServiceIDs) != 0 {
		t.Fatalf("rollout-only retry = %+v err=%v", deployed, err)
	}
	if getApp(t, svc.Client, "web").Spec.RestartedAt == beforeWeb || getApp(t, svc.Client, "worker").Spec.RestartedAt == beforeWorker {
		t.Fatal("deploy did not roll each linked service")
	}
}

func TestPatchEnvironmentRejectsStaleRevisionWithoutLostUpdate(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore())
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		ExpectedRevision: &group.Revision, SaveMode: SaveModeOnly,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "winner"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		ExpectedRevision: &group.Revision, SaveMode: SaveModeOnly,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "loser-secret"}},
	})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ENV_GROUP_REVISION_CONFLICT" || !errors.Is(err, core.ErrConflict) {
		t.Fatalf("stale patch = %v", err)
	}
	if got, _ := svc.GetEnvGroupVar(ctx, group.ID, "TOKEN"); got.Value != "winner" {
		t.Fatalf("winner overwritten: %+v", got)
	}
	if refreshed, _ := svc.GetEnvGroup(ctx, group.ID); refreshed.Revision != first.Revision {
		t.Fatalf("revision = %q, want %q", refreshed.Revision, first.Revision)
	}
}

func TestPatchEnvironmentConcurrentExpectedRevisionHasOneWinner(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	creator := newService(store)
	group, err := creator.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	services := []*Service{newService(store), newService(store)}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for i, service := range services {
		value := []string{"one", "two"}[i]
		go func() {
			<-start
			_, patchErr := service.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
				ExpectedRevision: &group.Revision, SaveMode: SaveModeOnly,
				EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: value}},
			})
			errs <- patchErr
		}()
	}
	close(start)
	var successes, conflicts int
	for range services {
		if err := <-errs; err == nil {
			successes++
		} else if errors.Is(err, core.ErrConflict) {
			conflicts++
		} else {
			t.Fatalf("patch error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

type failOneFilesPutStore struct {
	*fakeStore
	muFail sync.Mutex
	fail   bool
}

func (s *failOneFilesPutStore) Put(ctx context.Context, path string, data map[string]string) error {
	s.muFail.Lock()
	if s.fail && len(path) > len("/files") && path[len(path)-len("/files"):] == "/files" {
		s.fail = false
		s.muFail.Unlock()
		return errors.New("injected file write failure containing must-not-leak")
	}
	s.muFail.Unlock()
	return s.fakeStore.Put(ctx, path, data)
}

func TestPatchEnvironmentCompensatesMixedWriteFailure(t *testing.T) {
	ctx := context.Background()
	store := &failOneFilesPutStore{fakeStore: newFakeStore()}
	svc := newService(store)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "A", Value: "before-env"}},
		SecretFiles: []SecretFileView{{Name: "a.pem", Content: "before-file"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	store.fail = true
	_, err = svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		ExpectedRevision: &group.Revision, SaveMode: SaveModeOnly,
		EnvVars:     []EnvVarPatch{{Key: "A", Value: "failed-env-secret"}},
		SecretFiles: []SecretFilePatch{{Name: "a.pem", Content: "failed-file-secret"}},
	})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ENV_GROUP_UPDATE_RESTORED" {
		t.Fatalf("failure = %v", err)
	}
	if got, _ := svc.GetEnvGroupVar(ctx, group.ID, "A"); got.Value != "before-env" {
		t.Fatalf("env not restored: %+v", got)
	}
	if got, _ := svc.GetEnvGroupFile(ctx, group.ID, "a.pem"); got.Content != "before-file" {
		t.Fatalf("file not restored: %+v", got)
	}
	view, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil || view.Revision != group.Revision {
		t.Fatalf("logical revision changed after restoration: %+v err=%v", view, err)
	}
}

func TestPatchEnvironmentReportsPartialRebuildForRolloutOnlyRetry(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore(), sampleApp("web"), sampleApp("worker"))
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", ServiceIDs: []string{"web", "worker"}})
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	svc.RebuildService = func(_ context.Context, serviceID string) error {
		calls = append(calls, serviceID)
		if serviceID == "worker" {
			return errors.New("rebuild unavailable")
		}
		return nil
	}
	result, err := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		ExpectedRevision: &group.Revision, SaveMode: SaveModeRebuild,
		EnvVars: []EnvVarPatch{{Key: "A", Value: "secret"}},
	})
	if err != nil || result.RolledOut || !slices.Equal(result.FailedServiceIDs, []string{"worker"}) ||
		!slices.Equal(calls, []string{"web", "worker"}) {
		t.Fatalf("partial rebuild result=%+v calls=%v err=%v", result, calls, err)
	}
	calls = nil
	svc.RebuildService = func(_ context.Context, serviceID string) error {
		calls = append(calls, serviceID)
		return nil
	}
	retry, err := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		ExpectedRevision: &result.Revision, SaveMode: SaveModeRebuild,
	})
	if err != nil || !retry.RolledOut || !slices.Equal(calls, []string{"web", "worker"}) {
		t.Fatalf("retry=%+v calls=%v err=%v", retry, calls, err)
	}
}

// A service deleted while still linked to a group must not pin a permanent,
// un-retryable failure on the group's future save-and-deploy/rebuild results
// (w6/m108): the deleted service is neither failed nor affected, the surviving
// services roll out clean, and the group self-heals its stale link.
func TestPatchEnvironmentTolerantOfSinceDeletedLinkedService(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []SaveMode{SaveModeDeploy, SaveModeRebuild} {
		t.Run(string(mode), func(t *testing.T) {
			svc := newService(newFakeStore(), sampleApp("web"), sampleApp("worker"))
			if mode == SaveModeRebuild {
				svc.RebuildService = rebuildStub(svc.Client)
			}
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", ServiceIDs: []string{"web", "worker"}})
			if err != nil {
				t.Fatal(err)
			}
			// worker is deleted out from under the group (its own Settings > Delete
			// Service), leaving a dangling link the group never learned about.
			if err := svc.Client.Delete(ctx, sampleApp("worker")); err != nil {
				t.Fatalf("delete worker: %v", err)
			}
			result, err := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
				ExpectedRevision: &group.Revision, SaveMode: mode,
				EnvVars: []EnvVarPatch{{Key: "A", Value: "secret"}},
			})
			if err != nil {
				t.Fatalf("patch: %v", err)
			}
			if !result.RolledOut || len(result.FailedServiceIDs) != 0 ||
				!slices.Equal(result.AffectedServiceIDs, []string{"web"}) {
				t.Fatalf("stale-link result = %+v, want RolledOut with only web affected and no failures", result)
			}
			// Self-heal: the persisted link set no longer cites the deleted service.
			got, err := svc.GetEnvGroup(ctx, group.ID)
			if err != nil || !slices.Equal(got.ServiceLinks, []string{"web"}) {
				t.Fatalf("serviceLinks after prune = %+v err=%v, want [web]", got.ServiceLinks, err)
			}
			if mode == SaveModeDeploy && getApp(t, svc.Client, "web").Spec.RestartedAt == "" {
				t.Fatal("surviving service was not rolled")
			}
		})
	}
}

// The tolerance above must not swallow a genuine rollout failure on a service
// that still exists: a deleted service is pruned, while a real failure on a
// standing service stays in FailedServiceIDs and retryable (w6/m108).
func TestPatchEnvironmentDistinguishesDeletedServiceFromGenuineFailure(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore(), sampleApp("web"), sampleApp("worker"))
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", ServiceIDs: []string{"web", "worker"}})
	if err != nil {
		t.Fatal(err)
	}
	// web still exists but its rebuild genuinely fails; worker was deleted, so its
	// rebuild surfaces core.ErrNotFound through the faithful client-consulting stub.
	rebuild := rebuildStub(svc.Client)
	svc.RebuildService = func(ctx context.Context, serviceID string) error {
		if serviceID == "web" {
			return errors.New("rebuild unavailable")
		}
		return rebuild(ctx, serviceID)
	}
	if err := svc.Client.Delete(ctx, sampleApp("worker")); err != nil {
		t.Fatalf("delete worker: %v", err)
	}
	result, err := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		ExpectedRevision: &group.Revision, SaveMode: SaveModeRebuild,
		EnvVars: []EnvVarPatch{{Key: "A", Value: "secret"}},
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if result.RolledOut || !slices.Equal(result.FailedServiceIDs, []string{"web"}) ||
		!slices.Equal(result.AffectedServiceIDs, []string{"web"}) {
		t.Fatalf("mixed result = %+v, want web failed+affected, worker pruned", result)
	}
	// worker (deleted) is pruned; web (real failure) is kept so a later retry can succeed.
	got, _ := svc.GetEnvGroup(ctx, group.ID)
	if !slices.Equal(got.ServiceLinks, []string{"web"}) {
		t.Fatalf("serviceLinks = %+v, want [web] (worker pruned, web retained)", got.ServiceLinks)
	}
}
