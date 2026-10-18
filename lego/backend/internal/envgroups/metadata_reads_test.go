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
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// observedGroupKV records all four OpenBao verbs, including writes a read might
// accidentally perform while trying to recover an interrupted operation.
type observedGroupKV struct {
	*fakeStore
	mu    sync.Mutex
	calls []string
}

func (f *observedGroupKV) record(ctx context.Context, verb, path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, verb+" "+secrets.TenantFromContext(ctx)+"/"+path)
}

func (f *observedGroupKV) Get(ctx context.Context, path string) (map[string]string, error) {
	f.record(ctx, "Get", path)
	return f.fakeStore.Get(ctx, path)
}

func (f *observedGroupKV) Put(ctx context.Context, path string, values map[string]string) error {
	f.record(ctx, "Put", path)
	return f.fakeStore.Put(ctx, path, values)
}

func (f *observedGroupKV) Delete(ctx context.Context, path string) error {
	f.record(ctx, "Delete", path)
	return f.fakeStore.Delete(ctx, path)
}

func (f *observedGroupKV) List(ctx context.Context, path string) ([]string, error) {
	f.record(ctx, "List", path)
	return f.fakeStore.List(ctx, path)
}

func TestEnvGroupMetadataReadsNeverTouchOpenBaoInAnyContentState(t *testing.T) {
	states := []struct {
		name, state, phase string
		lease              time.Duration
	}{
		{name: "idle", state: store.EnvGroupIdle},
		{name: "idle awaiting cleanup", state: store.EnvGroupIdle, phase: opPhaseFilesWritten, lease: -time.Hour},
		{name: "active save", state: store.EnvGroupBusy, phase: opPhaseAdmitted, lease: time.Hour},
		{name: "expired before writes", state: store.EnvGroupBusy, phase: opPhaseAdmitted, lease: -time.Hour},
		{name: "expired after env", state: store.EnvGroupBusy, phase: opPhaseEnvWritten, lease: -time.Hour},
		{name: "expired after files", state: store.EnvGroupBusy, phase: opPhaseFilesWritten, lease: -time.Hour},
		{name: "repair required", state: store.EnvGroupRepairRequired, phase: opPhaseEnvWritten, lease: -time.Hour},
	}
	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			for _, read := range []string{"get", "list", "membership", "blueprint names", "blueprint ids", "blueprint export"} {
				t.Run(read, func(t *testing.T) {
					kv := &observedGroupKV{fakeStore: newFakeStore()}
					svc := newService(kv)
					row := store.EnvGroup{
						ID: id.New(id.EnvGroup), TenantID: core.DefaultTenant, Name: "shared",
						EnvironmentID: id.New(id.Environment), ServiceLinks: []string{"srv-web"},
						EnvKeys: []string{"TOKEN"}, FileNames: []string{"cert.pem"},
						CreatedAt: svc.nowTime(), UpdatedAt: svc.nowTime(),
						Content: store.EnvGroupContent{State: state.state, Generation: 4, Version: 7},
					}
					if state.phase != "" {
						row.Content.Op = &store.EnvGroupOp{
							ID: "ego-interrupted", Kind: opKindPatch, Phase: state.phase,
							LeaseUntil: svc.nowTime().Add(state.lease), EnvChanged: true, FilesChanged: true,
						}
					}
					row = svc.fakeGroups().Seed(row)
					// A foreign row must stay out of workspace-scoped inventories.
					svc.fakeGroups().Seed(store.EnvGroup{ID: id.New(id.EnvGroup), TenantID: "tea-other", Name: "foreign"})
					ctx := testCtx()
					var view EnvGroupView
					switch read {
					case "get":
						var err error
						view, err = svc.GetEnvGroup(ctx, row.ID)
						if err != nil {
							t.Fatal(err)
						}
					case "list":
						views, err := svc.ListEnvGroups(ctx, "")
						if err != nil || len(views) != 1 {
							t.Fatalf("list = %+v, %v; want the workspace's group", views, err)
						}
						view = views[0]
					case "membership":
						memberships, err := svc.ListEnvironmentMemberships(ctx, "")
						if err != nil || !slices.Equal(memberships, []EnvironmentMembership{{ID: row.ID, EnvironmentID: row.EnvironmentID}}) {
							t.Fatalf("membership = %+v, %v", memberships, err)
						}
					case "blueprint names":
						names, err := svc.GroupNames(ctx)
						if err != nil || !slices.Equal(names, []string{row.Name}) {
							t.Fatalf("names = %v, %v", names, err)
						}
					case "blueprint ids":
						ids, err := svc.GroupIDsByName(ctx)
						if err != nil || len(ids) != 1 || ids[row.Name] != row.ID {
							t.Fatalf("ids = %v, %v", ids, err)
						}
					case "blueprint export":
						name, keys, err := svc.ExportEnvGroup(ctx, row.ID)
						if state.state == store.EnvGroupIdle {
							if err != nil || name != row.Name || !slices.Equal(keys, row.EnvKeys) {
								t.Fatalf("export = %q, %v, %v", name, keys, err)
							}
						} else {
							code := "ENV_GROUP_REVISION_CONFLICT"
							if state.state == store.EnvGroupRepairRequired {
								code = "ENV_GROUP_RESTORATION_FAILED"
							}
							var coded *core.CodedError
							if !errors.As(err, &coded) || coded.Code != code || name != "" || keys != nil {
								t.Fatalf("unavailable export = %q, %v, %v; want %s without an incomplete declaration", name, keys, err, code)
							}
						}
					}
					if view.ID != "" {
						if view.ID != row.ID || view.Name != row.Name || view.OwnerID != row.TenantID ||
							view.EnvironmentID != id.EnvironmentPublicID(row.EnvironmentID) || !slices.Equal(view.ServiceLinks, row.ServiceLinks) {
							t.Fatalf("metadata view = %+v, want row identity, ownership, membership and links", view)
						}
						if state.state == store.EnvGroupIdle {
							if len(view.EnvVars) != 1 || view.EnvVars[0].Key != "TOKEN" || len(view.SecretFiles) != 1 ||
								view.SecretFiles[0].Name != "cert.pem" || view.Revision != encodeEnvGroupRevision(4) || view.Availability != "" {
								t.Fatalf("idle content view = %+v", view)
							}
						} else if view.Availability != state.state || len(view.EnvVars) != 0 || len(view.SecretFiles) != 0 || view.Revision != "" {
							t.Fatalf("unavailable content view = %+v; want state %q without names or revision", view, state.state)
						}
					}
					if len(kv.calls) != 0 {
						t.Fatalf("metadata read touched OpenBao: %v", kv.calls)
					}
					if after, _ := svc.fakeGroups().Row(row.ID); !reflect.DeepEqual(after, row) {
						t.Fatalf("metadata read changed the content state: before=%+v after=%+v", row, after)
					}
				})
			}
		})
	}
}
