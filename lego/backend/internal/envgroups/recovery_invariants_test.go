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
	"maps"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestValueConsumersRefuseAnActiveGroupSaveBeforeReadingOrLinking(t *testing.T) {
	for _, consumer := range []string{"variable reveal", "file reveal", "link", "initial blueprint links", "blueprint apply"} {
		t.Run(consumer, func(t *testing.T) {
			kv := &observedGroupKV{fakeStore: newFakeStore()}
			svc := newService(kv, sampleApp("web"))
			ctx := testCtx()
			g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatal(err)
			}
			row, _ := svc.fakeGroups().Row(g.ID)
			row.Content.State = store.EnvGroupBusy
			row.Content.Op = &store.EnvGroupOp{
				ID: "ego-active", Kind: opKindPatch, Phase: opPhaseEnvWritten,
				LeaseUntil: svc.nowTime().Add(time.Hour), EnvChanged: true, FilesChanged: true,
			}
			svc.fakeGroups().Set(row)
			beforeApp := getApp(t, svc.Client, "web")
			kv.calls = nil
			switch consumer {
			case "variable reveal":
				_, err = svc.GetEnvGroupVar(ctx, g.ID, "TOKEN")
			case "file reveal":
				_, err = svc.GetEnvGroupFile(ctx, g.ID, "cert.pem")
			case "link":
				err = svc.LinkService(ctx, g.ID, "web")
			case "initial blueprint links":
				err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, sampleApp("new"), func() error {
					t.Fatal("an unavailable group reached App creation")
					return nil
				}, nil)
			case "blueprint apply":
				err = svc.ApplyEnvGroup(ctx, "shared", map[string]string{"TOKEN": "new"}, nil)
			}
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != "ENV_GROUP_REVISION_CONFLICT" {
				t.Fatalf("consumer = %v, want ENV_GROUP_REVISION_CONFLICT", err)
			}
			if len(kv.calls) != 0 {
				t.Fatalf("active save's values were touched: %v", kv.calls)
			}
			if after, _ := svc.fakeGroups().Row(g.ID); !reflect.DeepEqual(after, row) {
				t.Fatal("refused consumer changed the active save or its links")
			}
			if after := getApp(t, svc.Client, "web"); !reflect.DeepEqual(after, beforeApp) {
				t.Fatal("refused consumer changed the App")
			}
		})
	}
}

func TestAnotherInstanceRecoversEachSavePhaseBeforeRevealingEitherMap(t *testing.T) {
	for _, phase := range []string{opPhaseAdmitted, opPhaseEnvWritten, opPhaseFilesWritten} {
		for _, reveal := range []string{"variable", "file"} {
			t.Run(phase+"/"+reveal, func(t *testing.T) {
				ctx := testCtx()
				kv := newFakeStore()
				writer := newService(kv)
				g, err := writer.CreateEnvGroup(ctx, CreateEnvGroupRequest{
					Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before-env"}},
					SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "before-file"}},
				})
				if err != nil {
					t.Fatal(err)
				}
				prior := groupContents{env: map[string]string{"TOKEN": "before-env"}, files: map[string]string{"cert.pem": "before-file"}}
				next := groupContents{env: map[string]string{"TOKEN": "after-env"}, files: map[string]string{"cert.pem": "after-file"}}
				row, _ := writer.fakeGroups().Row(g.ID)
				op := store.EnvGroupOp{Kind: opKindPatch, EnvChanged: true, FilesChanged: true}
				op.ID, err = newGroupOpID()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := writer.claimContent(ctx, row, op); err != nil {
					t.Fatal(err)
				}
				if err := writer.preparePatchOperation(ctx, op, row.TenantID, row.ID, prior, next); err != nil {
					t.Fatal(err)
				}
				if phase != opPhaseAdmitted {
					if err := writer.writeSlot(ctx, row.TenantID, row.ID, envSlot, next.env); err != nil {
						t.Fatal(err)
					}
					if phase == opPhaseFilesWritten {
						if err := writer.writeSlot(ctx, row.TenantID, row.ID, filesSlot, next.files); err != nil {
							t.Fatal(err)
						}
					}
					if err := writer.advanceOpPhase(ctx, row.ID, op.ID, phase); err != nil {
						t.Fatal(err)
					}
				}
				reader := newService(kv)
				reader.Client = writer.Client
				reader.Base.Clock = func() time.Time { return writer.nowTime().Add(time.Hour) }
				want, generation := prior, uint64(1)
				if phase == opPhaseFilesWritten {
					want, generation = next, 2
				}
				if reveal == "variable" {
					value, err := reader.GetEnvGroupVar(ctx, row.ID, "TOKEN")
					if err != nil || value.Value != want.env["TOKEN"] {
						t.Fatalf("variable reveal = %+v, %v", value, err)
					}
				} else {
					file, err := reader.GetEnvGroupFile(ctx, row.ID, "cert.pem")
					if err != nil || file.Content != want.files["cert.pem"] {
						t.Fatalf("file reveal = %+v, %v", file, err)
					}
				}
				for slot, expected := range map[contentSlot]map[string]string{envSlot: want.env, filesSlot: want.files} {
					actual, err := reader.getGroupMap(ctx, row.TenantID, slot.path(row.ID))
					if err != nil || !maps.Equal(actual, expected) {
						t.Fatalf("map %d after recovery = %v, %v; want %v", slot, actual, err, expected)
					}
					projection := secret(t, reader.Client, slot.secret(row.ID))
					if len(projection.Data) != len(expected) {
						t.Fatalf("projection %d after recovery has %d keys, want %d", slot, len(projection.Data), len(expected))
					}
					for key, value := range expected {
						if string(projection.Data[key]) != value {
							t.Fatalf("projection %d diverged from recovered source", slot)
						}
					}
				}
				committed, _ := reader.fakeGroups().Row(row.ID)
				if committed.Content.State != store.EnvGroupIdle || committed.Content.Op != nil || committed.Content.Generation != generation {
					t.Fatalf("terminal content state = %+v, want idle generation %d without an operation", committed.Content, generation)
				}
				for key := range kv.m {
					if strings.Contains(key, "/op/") {
						t.Fatalf("terminal recovery left value maps: %s", key)
					}
				}
			})
		}
	}
}

func TestRestoredSaveRemovesAllOperationValueMaps(t *testing.T) {
	kv := &failOneFilesPutStore{fakeStore: newFakeStore()}
	svc := newService(kv)
	ctx := testCtx()
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
		SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	kv.fail = true
	_, err = svc.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
		ExpectedRevision: &g.Revision, SaveMode: SaveModeOnly,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "after"}}, SecretFiles: []SecretFilePatch{{Name: "cert.pem", Content: "after"}},
	})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ENV_GROUP_UPDATE_RESTORED" {
		t.Fatalf("failed mixed save = %v, want restored", err)
	}
	row, _ := svc.fakeGroups().Row(g.ID)
	if row.Content.State != store.EnvGroupIdle || row.Content.Op != nil || row.Content.Generation != 1 {
		t.Fatalf("restored save left content state %+v", row.Content)
	}
	for key := range kv.m {
		if strings.Contains(key, "/op/") {
			t.Fatalf("restored save left recovery value maps: %s", key)
		}
	}
}
