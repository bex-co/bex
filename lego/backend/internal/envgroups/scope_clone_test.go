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

func TestEnvironmentAliasesPreserveGroupMetadataLinksAndWorkspace(t *testing.T) {
	const stored = "env-c185th5c2rvvnhbfiltg"
	const public = "evm-c185th5c2rvvnhbfiltg"
	const otherStored = "env-c185th5c2rvvnhbfilt0"
	const otherPublic = "evm-c185th5c2rvvnhbfilt0"
	const foreignPublic = "evm-c185th5c2rvvnhbfilt1"

	for _, input := range []string{stored, public} {
		t.Run(input, func(t *testing.T) {
			web := sampleApp("web")
			web.Labels = map[string]string{core.LabelEnvironment: stored}
			svc := newService(newFakeStore(), web)
			svc.EnvironmentWorkspace = func(_ context.Context, environmentID string) (string, error) {
				switch environmentID {
				case stored, otherStored:
					return "", nil
				case "env-c185th5c2rvvnhbfilt1":
					return "tea-foreign", nil
				default:
					return "", core.ErrNotFound
				}
			}
			group, err := svc.CreateEnvGroup(t.Context(), CreateEnvGroupRequest{
				Name: "shared", EnvironmentID: input, ServiceIDs: []string{"web"},
			})
			if err != nil || group.EnvironmentID != public || len(group.ServiceLinks) != 1 {
				t.Fatalf("create with %s: %+v, %v", input, group, err)
			}
			storedGroup, err := svc.requireGroup(t.Context(), group.ID)
			if err != nil || storedGroup.environment != stored {
				t.Fatalf("stored group environment=%q, err=%v", storedGroup.environment, err)
			}
			memberships, err := svc.ListEnvironmentMemberships(t.Context(), "")
			if err != nil || len(memberships) != 1 || memberships[0].EnvironmentID != stored {
				t.Fatalf("internal membership = %+v, err=%v", memberships, err)
			}
			for _, selector := range []string{stored, public} {
				filter := EnvGroupListFilter{EnvironmentIDs: []string{selector}}
				groups, err := svc.ListEnvGroupsFiltered(t.Context(), filter)
				if filter.EnvironmentIDs[0] != selector {
					t.Fatal("list mutated the caller's environment selector")
				}
				if err != nil || len(groups) != 1 || groups[0].ID != group.ID || groups[0].EnvironmentID != public {
					t.Fatalf("filter with %s: %+v, %v", selector, groups, err)
				}
			}
			if moved, err := svc.MoveEnvGroup(t.Context(), group.ID, public); err != nil || moved.EnvironmentID != public {
				t.Fatalf("move to same environment via alias: %+v, %v", moved, err)
			}
			if _, err := svc.MoveEnvGroup(t.Context(), group.ID, otherPublic); !errors.Is(err, core.ErrConflict) {
				t.Fatalf("alias bypassed linked-service placement: %v", err)
			} else {
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Params["targetEnvironmentId"] != otherPublic {
					t.Fatalf("move conflict did not expose canonical target: %v", err)
				}
			}
			if _, err := svc.MoveEnvGroup(t.Context(), group.ID, foreignPublic); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("alias bypassed workspace binding: %v", err)
			}
			storedGroup, err = svc.requireGroup(t.Context(), group.ID)
			if err != nil || storedGroup.environment != stored || getApp(t, svc.Client, "web").Labels[core.LabelEnvironment] != stored {
				t.Fatalf("alias or refused move changed durable placement: group=%q, err=%v", storedGroup.environment, err)
			}
			clone, err := svc.CloneEnvGroup(t.Context(), group.ID, CloneEnvGroupRequest{Name: "copy", EnvironmentID: public})
			if err != nil || clone.EnvironmentID != public || len(clone.ServiceLinks) != 0 {
				t.Fatalf("clone into canonical environment: %+v, %v", clone, err)
			}
			moved, err := svc.MoveEnvGroup(t.Context(), clone.ID, otherPublic)
			if err != nil || moved.EnvironmentID != otherPublic {
				t.Fatalf("move unlinked clone: %+v, %v", moved, err)
			}
			storedClone, err := svc.requireGroup(t.Context(), clone.ID)
			if err != nil || storedClone.environment != otherStored {
				t.Fatalf("move persisted public alias: environment=%q, err=%v", storedClone.environment, err)
			}
		})
	}
}
