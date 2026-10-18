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
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// A group's values land under its owning workspace's own OpenBao tenant, and
// nothing else about it is written to OpenBao: its name, workspace and the
// names of its values are its row (w8/m114).
func TestCreateEnvGroupWritesOnlyValuesUnderTheWorkspaceTenant(t *testing.T) {
	store := newFakeStore()
	svc := withGroups(&Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: coretest.Members{"dana": {"tea-a"}}},
		Store: store,
	})
	ctx := core.WithIdentity(testCtx(), core.Identity{Subject: "dana", Method: "session"})

	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name:        "shared",
		EnvVars:     []CreateEnvVarInput{{Key: "K", Value: "v"}},
		SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "pem"}},
	})
	if err != nil {
		t.Fatalf("CreateEnvGroup: %v", err)
	}
	workspace := secrets.WithTenant(testCtx(), "tea-a")
	var paths []string
	for key := range store.m {
		if strings.Contains(key, g.ID) {
			paths = append(paths, key)
		}
	}
	slices.Sort(paths)
	want := []string{store.key(workspace, envPath(g.ID)), store.key(workspace, filesPath(g.ID))}
	slices.Sort(want)
	if !slices.Equal(paths, want) {
		t.Fatalf("OpenBao paths for the group = %v, want only its two maps %v", paths, want)
	}
	row, ok := groupsOf(store).Row(g.ID)
	if !ok || row.TenantID != "tea-a" || row.Name != "shared" ||
		!slices.Equal(row.EnvKeys, []string{"K"}) || !slices.Equal(row.FileNames, []string{"cert.pem"}) {
		t.Fatalf("row = %+v found=%v", row, ok)
	}
}
