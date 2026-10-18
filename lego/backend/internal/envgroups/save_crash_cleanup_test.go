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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type interruptedGroupKV struct {
	*fakeStore
	crashPath       string
	failFiles       bool
	failPrepare     bool
	preparationPuts int
	failCleanup     bool
}

func (f *interruptedGroupKV) Put(ctx context.Context, path string, values map[string]string) error {
	if f.failPrepare && strings.Contains(path, "/op/") {
		f.preparationPuts++
		if f.preparationPuts == 2 {
			f.failPrepare = false
			return errors.New("recovery map write refused")
		}
	}
	if f.failFiles && strings.HasSuffix(path, "/files") {
		f.failFiles = false
		return errors.New("live files write refused")
	}
	if err := f.fakeStore.Put(ctx, path, values); err != nil {
		return err
	}
	if path == f.crashPath {
		f.crashPath = ""
		panic("simulated process crash after live map write")
	}
	return nil
}

func (f *interruptedGroupKV) Delete(ctx context.Context, path string) error {
	if f.failCleanup && strings.Contains(path, "/op/") {
		return errors.New("operation map cleanup unavailable")
	}
	return f.fakeStore.Delete(ctx, path)
}

func TestCrashImmediatelyAfterLiveWriteRecoversACoherentConfiguration(t *testing.T) {
	for _, side := range []contentSlot{envSlot, filesSlot} {
		t.Run(side.path("group"), func(t *testing.T) {
			ctx := testCtx()
			kv := &interruptedGroupKV{fakeStore: newFakeStore()}
			writer := newService(kv)
			g, err := writer.CreateEnvGroup(ctx, CreateEnvGroupRequest{
				Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
				SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "before"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			kv.crashPath = side.path(g.ID)
			func() {
				defer func() {
					if recover() == nil {
						t.Fatal("the intended live-write crash did not occur")
					}
				}()
				_, _ = writer.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
					ExpectedRevision: &g.Revision, SaveMode: SaveModeOnly,
					EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "after"}}, SecretFiles: []SecretFilePatch{{Name: "cert.pem", Content: "after"}},
				})
			}()
			reader := newService(kv)
			reader.Client = writer.Client
			reader.Base.Clock = func() time.Time { return writer.nowTime().Add(time.Hour) }
			want, generation := "before", uint64(1)
			if side == filesSlot {
				want, generation = "after", 2
			}
			value, err := reader.GetEnvGroupVar(ctx, g.ID, "TOKEN")
			if err != nil || value.Value != want {
				t.Fatalf("env after live-write crash = %+v, %v; want %q", value, err, want)
			}
			file, err := reader.GetEnvGroupFile(ctx, g.ID, "cert.pem")
			if err != nil || file.Content != want {
				t.Fatalf("files after live-write crash = %+v, %v; want coherent %q", file, err, want)
			}
			if string(secret(t, reader.Client, envSecretName(g.ID)).Data["TOKEN"]) != want ||
				string(secret(t, reader.Client, filesSecretName(g.ID)).Data["cert.pem"]) != want {
				t.Fatal("crash recovery left a source/projection mismatch")
			}
			row, _ := reader.fakeGroups().Row(g.ID)
			if row.Content.Generation != generation || row.Content.State != store.EnvGroupIdle || row.Content.Op != nil {
				t.Fatalf("recovered content = %+v, want idle generation %d without an op", row.Content, generation)
			}
		})
	}
}

func TestTerminalSaveRetainsCleanupEvidenceUntilAnotherInstanceCanDeleteMaps(t *testing.T) {
	for _, terminal := range []string{"committed", "restored", "prepare failed"} {
		t.Run(terminal, func(t *testing.T) {
			ctx := testCtx()
			kv := &interruptedGroupKV{fakeStore: newFakeStore()}
			writer := newService(kv)
			g, err := writer.CreateEnvGroup(ctx, CreateEnvGroupRequest{
				Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
				SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "before"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			kv.failCleanup = true
			kv.failFiles = terminal == "restored"
			kv.failPrepare = terminal == "prepare failed"
			result, err := writer.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
				ExpectedRevision: &g.Revision, SaveMode: SaveModeOnly,
				EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "after"}}, SecretFiles: []SecretFilePatch{{Name: "cert.pem", Content: "after"}},
			})
			generation := uint64(1)
			if terminal == "committed" {
				generation = 2
				if err != nil || result.Revision != encodeEnvGroupRevision(generation) {
					t.Fatalf("committed content became a failed response because cleanup failed: %+v, %v", result, err)
				}
			} else {
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Code != "ENV_GROUP_UPDATE_RESTORED" {
					t.Fatalf("restored attempt = %v, want ENV_GROUP_UPDATE_RESTORED", err)
				}
			}
			row, _ := writer.fakeGroups().Row(g.ID)
			if row.Content.State != store.EnvGroupIdle || row.Content.Op == nil || row.Content.Generation != generation {
				t.Fatalf("terminal save lost cleanup evidence or changed generation: %+v", row.Content)
			}
			retainedOp := row.Content.Op.ID
			view, err := writer.GetEnvGroup(ctx, g.ID)
			if err != nil || view.Availability != "" || view.Revision != encodeEnvGroupRevision(generation) {
				t.Fatalf("committed metadata unavailable during value-map cleanup: %+v, %v", view, err)
			}
			replica := newService(kv)
			replica.Client = writer.Client
			if _, err := replica.SetEnvGroupVar(ctx, g.ID, "TOKEN", "later"); !errors.Is(err, core.ErrSecretsUnavailable) {
				t.Fatalf("next writer replaced uncleared cleanup evidence: %v", err)
			}
			row, _ = writer.fakeGroups().Row(g.ID)
			if row.Content.Generation != generation || row.Content.Op == nil || row.Content.Op.ID != retainedOp {
				t.Fatal("refused writer lost the committed generation or its cleanup evidence")
			}
			kv.failCleanup = false
			if _, err := replica.SetEnvGroupVar(ctx, g.ID, "TOKEN", "later"); err != nil {
				t.Fatal(err)
			}
			row, _ = writer.fakeGroups().Row(g.ID)
			if row.Content.Generation != generation+1 || row.Content.Op != nil || row.Content.State != store.EnvGroupIdle {
				t.Fatalf("cleanup retry recommitted a generation or left an operation: %+v", row.Content)
			}
			for key := range kv.m {
				if strings.Contains(key, "/op/") {
					t.Fatalf("cleanup retry left recovery maps: %s", key)
				}
			}
		})
	}
}
