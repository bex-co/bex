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

package secrets

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"strconv"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestADeadWriteCannotCommitOverItsNamesakeAfterAPurge (w5/156): a service's
// purge deleted its store paths' metadata, so the namesake's create committed
// version 1 again. A write in flight for the deleted service that had read
// version 1 then compare-and-set over the namesake's map, and its compensation
// put the dead service's map back there. The purge now retires the paths, so
// versions keep counting and that write conflicts.
func TestADeadWriteCannotCommitOverItsNamesakeAfterAPurge(t *testing.T) {
	for _, c := range projectionCases {
		t.Run(c.name, func(t *testing.T) {
			svc, store := c.setup(nil, map[string]string{"old": "1"})
			ctx := context.Background()
			name := c.kind.secretName("web")
			prepared := map[string]string{"new": "1"}
			var recreated error
			store.afterGet = func() {
				recreated = func() error {
					dead := sampleApp("web")
					if err := svc.Client.Delete(ctx, dead); err != nil {
						return err
					}
					if err := (&WorkspacePurger{Service: svc}).PurgeApp(ctx, dead); err != nil {
						return err
					}
					if err := svc.Client.Delete(ctx, getSecret(t, svc.Client, name)); err != nil {
						return err
					}
					return svc.prepareProjection(ctx, sampleApp("web"), c.kind, c.path, prepared)
				}()
			}
			err := c.write(svc, "stale")
			if recreated != nil {
				t.Fatalf("the purge and the namesake's create: %v", recreated)
			}
			if err == nil {
				t.Fatal("the deleted service's write succeeded, want it refused")
			}
			if stored := store.m[c.path]; !maps.Equal(stored, prepared) {
				t.Fatalf("the namesake's store map = %v, want the create's %v", stored, prepared)
			}
			if sec := getSecret(t, svc.Client, name); !equalSecretData(sec.Data, prepared) {
				t.Fatalf("the namesake's Secret holds %v, want the create's %v", sec.Data, prepared)
			}
		})
	}
}

// TestAReplacedCASWriteTakesBackItsChangeAndLeavesTheSecret (w5/156): a
// revision-checked save whose service is replaced after its store write used
// to compensate as if it still owned the name, answering
// ENVIRONMENT_RESTORATION_FAILED. It now takes its change back, leaves the
// Secret at the name alone, and answers the revision conflict.
func TestAReplacedCASWriteTakesBackItsChangeAndLeavesTheSecret(t *testing.T) {
	c := projectionCases[0]
	before := map[string]string{"TOKEN": "before"}
	svc, store := c.setup(nil, before)
	ctx := context.Background()
	secretBefore := getSecret(t, svc.Client, envSecretName("web"))
	var replaced error
	store.afterCAS = func(string, uint64) {
		replaced = func() error {
			if err := svc.Client.Delete(ctx, sampleApp("web")); err != nil {
				return err
			}
			namesake := sampleApp("web")
			namesake.UID = "uid-namesake"
			return svc.Client.Create(ctx, namesake)
		}()
	}
	revision := encodeEnvRevision(1)
	_, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision, EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "stale"}}})
	if replaced != nil {
		t.Fatalf("replacing the service: %v", replaced)
	}
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_REVISION_CONFLICT" {
		t.Fatalf("the replaced service's save = %v, want ENVIRONMENT_REVISION_CONFLICT", err)
	}
	if stored := store.m[c.path]; !maps.Equal(stored, before) {
		t.Fatalf("the store map = %v, want the save taken back to %v", stored, before)
	}
	if sec := getSecret(t, svc.Client, envSecretName("web")); sec.ResourceVersion != secretBefore.ResourceVersion {
		t.Fatalf("the Secret at the name changed (resourceVersion %s -> %s), want it left alone", secretBefore.ResourceVersion, sec.ResourceVersion)
	}
}

// TestRetireKeepsCountingAndDestroysTheOldValues (w5/156): against a real
// OpenBao, a retired path reads empty at a version past every earlier one, so
// a compare-and-set from any version read before it conflicts, and none of the
// values it held can be read back. An absent path stays absent.
func TestRetireKeepsCountingAndDestroysTheOldValues(t *testing.T) {
	bao := realBatchBao(t)
	ctx := context.Background()
	path := envPath("web")
	v1, err := bao.PutCAS(ctx, path, map[string]string{"TOKEN": "one"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := bao.PutCAS(ctx, path, map[string]string{"TOKEN": "two"}, v1)
	if err != nil {
		t.Fatal(err)
	}
	if err := bao.Retire(ctx, path); err != nil {
		t.Fatalf("Retire: %v", err)
	}
	retired, err := bao.GetVersioned(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if retired.Version <= v2 || len(retired.Data) != 0 {
		t.Fatalf("retired path = %v at v%d, want an empty map past v%d", retired.Data, retired.Version, v2)
	}
	for _, stale := range []uint64{0, v1, v2} {
		if _, err := bao.PutCAS(ctx, path, map[string]string{"TOKEN": "stale"}, stale); !errors.Is(err, core.ErrConflict) {
			t.Fatalf("PutCAS from v%d after the retire = %v, want a conflict", stale, err)
		}
	}
	for _, old := range []uint64{v1, v2} {
		var out struct {
			Data struct {
				Data map[string]string `json:"data"`
			} `json:"data"`
		}
		err := bao.kv(ctx, http.MethodGet, bao.dataURL(ctx, path)+"?version="+strconv.FormatUint(old, 10), nil, &out)
		var status *core.HTTPStatusError
		if !(errors.As(err, &status) && status.Code == http.StatusNotFound) && len(out.Data.Data) != 0 {
			t.Fatalf("version %d still reads %v (err %v), want it destroyed", old, out.Data.Data, err)
		}
	}
	if _, err := bao.PutCAS(ctx, path, map[string]string{"TOKEN": "namesake"}, retired.Version); err != nil {
		t.Fatalf("the namesake's write from the retired version: %v", err)
	}

	absent := envPath("never-written")
	if err := bao.Retire(ctx, absent); err != nil {
		t.Fatalf("Retire of an absent path: %v", err)
	}
	if snapshot, err := bao.GetVersioned(ctx, absent); err != nil || snapshot.Version != 0 {
		t.Fatalf("absent path after Retire = v%d (err %v), want still absent", snapshot.Version, err)
	}
}

// TestAWorkspacePurgeSweepsItsRetiredPaths (w5/156): a deleted service's paths
// keep their metadata once retired, and no live App names them any more. The
// workspace's own purge deletes them, and leaves another workspace's alone.
func TestAWorkspacePurgeSweepsItsRetiredPaths(t *testing.T) {
	bao := realBatchBao(t)
	ctx := context.Background()
	gone, other := withTenant(ctx, "tea-gone"), withTenant(ctx, "tea-other")
	for _, c := range []context.Context{gone, other} {
		if _, err := bao.PutCAS(c, envPath("old"), map[string]string{"K": "v"}, 0); err != nil {
			t.Fatal(err)
		}
		if err := bao.Retire(c, envPath("old")); err != nil {
			t.Fatal(err)
		}
	}
	svc := newService(bao)
	if err := (&WorkspacePurger{Service: svc}).PurgeWorkspace(ctx, "tea-gone"); err != nil {
		t.Fatalf("PurgeWorkspace: %v", err)
	}
	if left, err := bao.List(gone, "services"); err != nil || len(left) != 0 {
		t.Fatalf("the deleted workspace still lists %v (err %v), want nothing", left, err)
	}
	if kept, err := bao.List(other, "services"); err != nil || len(kept) != 1 {
		t.Fatalf("the other workspace lists %v (err %v), want its retired path kept", kept, err)
	}
}
