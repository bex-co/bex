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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

func TestGroupLifecycleWritesOnlyWorkspaceValueMaps(t *testing.T) {
	kv := &observedGroupKV{fakeStore: newFakeStore()}
	svc := newService(kv, ownedApp("web", "tea-a"))
	svc.Workspace = coretest.Workspaces{"tea-a"}
	ctx := testCtx()
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
		SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RenameEnvGroup(ctx, g.ID, "renamed"); err != nil {
		t.Fatal(err)
	}
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
		ExpectedRevision: &g.Revision, SaveMode: SaveModeOnly,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "after"}}, SecretFiles: []SecretFilePatch{{Name: "cert.pem", Content: "after"}},
	}); err != nil {
		t.Fatal(err)
	}
	clone, err := svc.CloneEnvGroup(ctx, g.ID, CloneEnvGroupRequest{Name: "clone"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.UnlinkService(ctx, g.ID, "web"); err != nil {
		t.Fatal(err)
	}
	for _, gid := range []string{g.ID, clone.ID} {
		if err := svc.DeleteEnvGroup(ctx, gid); err != nil {
			t.Fatal(err)
		}
	}
	for _, call := range kv.calls {
		verb, key, _ := strings.Cut(call, " ")
		if verb != "Put" && verb != "Delete" {
			continue
		}
		parts := strings.Split(key, "/")
		if len(parts) < 4 || parts[0] != "tea-a" || parts[1] != "env-groups" || (parts[2] != g.ID && parts[2] != clone.ID) {
			t.Fatalf("lifecycle wrote outside its workspace's group values: %s", call)
		}
		switch {
		case len(parts) == 4 && (parts[3] == "env" || parts[3] == "files"):
		case len(parts) == 6 && parts[3] == "op" && strings.HasPrefix(parts[4], "ego-") &&
			(parts[5] == "old-env" || parts[5] == "new-env" || parts[5] == "old-files" || parts[5] == "new-files"):
		default:
			t.Fatalf("lifecycle wrote non-value OpenBao state: %s", call)
		}
	}
	if len(kv.m) != 0 {
		t.Fatalf("terminal deletes left %d OpenBao maps", len(kv.m))
	}
}
