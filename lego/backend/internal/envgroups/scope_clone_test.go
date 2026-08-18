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
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func environmentResolver(ids ...string) func(context.Context, string) (string, error) {
	allowed := make(map[string]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}
	return func(_ context.Context, id string) (string, error) {
		if !allowed[id] {
			return "", core.ErrNotFound
		}
		return "", nil
	}
}

func TestEnvGroupScopeConstrainsLinksAndMoveIsAtomic(t *testing.T) {
	ctx := context.Background()
	web := sampleApp("web")
	web.Labels = map[string]string{core.LabelEnvironment: "env-a"}
	standalone := sampleApp("standalone")
	svc := newService(newFakeStore(), web, standalone)
	svc.EnvironmentWorkspace = environmentResolver("env-a", "env-b")
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "scoped", EnvironmentID: "env-a", ServiceIDs: []string{"web"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LinkService(ctx, group.ID, "standalone"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("cross-scope link = %v", err)
	}
	if _, err := svc.MoveEnvGroup(ctx, group.ID, "env-b"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("incompatible move = %v", err)
	}
	unchanged, _ := svc.GetEnvGroup(ctx, group.ID)
	if unchanged.EnvironmentID != "env-a" || len(unchanged.ServiceLinks) != 1 {
		t.Fatalf("failed move mutated group: %+v", unchanged)
	}
	app := getApp(t, svc.Client, "web")
	base := client.MergeFrom(app.DeepCopy())
	app.Labels[core.LabelEnvironment] = "env-b"
	if err := svc.Client.Patch(ctx, app, base); err != nil {
		t.Fatal(err)
	}
	moved, err := svc.MoveEnvGroup(ctx, group.ID, "env-b")
	if err != nil || moved.EnvironmentID != "env-b" || len(moved.ServiceLinks) != 1 {
		t.Fatalf("compatible move = %+v err=%v", moved, err)
	}
	if _, err := svc.MoveEnvGroup(ctx, group.ID, ""); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("scoped service should block workspace move: %v", err)
	}
}

func TestCloneEnvGroupCopiesContentsWithoutLinksOrValues(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore(), sampleApp("web"))
	source, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "source", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "secret"}},
		SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "pem-secret"}},
		ServiceIDs:  []string{"web"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clone, err := svc.CloneEnvGroup(ctx, source.ID, CloneEnvGroupRequest{Name: "copy"})
	if err != nil {
		t.Fatal(err)
	}
	if clone.ID == source.ID || clone.Name != "copy" || len(clone.ServiceLinks) != 0 ||
		len(clone.EnvVars) != 1 || clone.EnvVars[0].Value != "" ||
		len(clone.SecretFiles) != 1 || clone.SecretFiles[0].Content != "" {
		t.Fatalf("clone response leaked or copied links: %+v", clone)
	}
	if got, _ := svc.GetEnvGroupVar(ctx, clone.ID, "TOKEN"); got.Value != "secret" {
		t.Fatalf("cloned value = %+v", got)
	}
	if got, _ := svc.GetEnvGroupFile(ctx, clone.ID, "cert.pem"); got.Content != "pem-secret" {
		t.Fatalf("cloned file = %+v", got)
	}
	if sourceAfter, _ := svc.GetEnvGroup(ctx, source.ID); len(sourceAfter.ServiceLinks) != 1 {
		t.Fatalf("source mutated: %+v", sourceAfter)
	}
}

func TestCreateEnvGroupRejectsEnvironmentMismatchedInitialServiceWithoutOrphan(t *testing.T) {
	web := &appv1alpha1.App{}
	*web = *sampleApp("web")
	web.Labels = map[string]string{core.LabelEnvironment: "env-b"}
	store := newFakeStore()
	svc := newService(store, web)
	svc.EnvironmentWorkspace = environmentResolver("env-a", "env-b")
	_, err := svc.CreateEnvGroup(context.Background(), CreateEnvGroupRequest{
		Name: "scoped", EnvironmentID: "env-a", ServiceIDs: []string{"web"},
	})
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("create mismatch = %v", err)
	}
	ids, _ := store.List(context.Background(), "env-groups")
	if len(ids) != 0 {
		t.Fatalf("orphan group paths: %v", ids)
	}
}
