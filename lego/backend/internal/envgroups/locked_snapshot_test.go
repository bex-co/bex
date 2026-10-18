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
	"maps"
	"testing"
)

type beforeContentLockStore struct {
	GroupStore
	before func()
}

func (s *beforeContentLockStore) WithEnvGroupContentLock(ctx context.Context, gid string, fn func(context.Context) error) error {
	s.before()
	return s.GroupStore.WithEnvGroupContentLock(ctx, gid, fn)
}

func TestReplaceAllEnvVarsRemovesAKeyAddedAfterAuthorization(t *testing.T) {
	ctx := testCtx()
	kv := newFakeStore()
	first := newService(kv)
	g, err := first.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "KEEP", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	second := newService(kv)
	second.Client = first.Client
	first.Groups = &beforeContentLockStore{GroupStore: first.Groups, before: func() {
		// The replace-all caller already authorized from the old row. Another
		// replica commits this new key before it acquires the content lock.
		if _, err := second.SetEnvGroupVar(ctx, g.ID, "LATE", "concurrent"); err != nil {
			t.Fatal(err)
		}
	}}
	keys, err := first.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "KEEP", Value: "after"}})
	if err != nil || len(keys) != 1 || keys[0].Key != "KEEP" {
		t.Fatalf("replacement keys = %+v, %v", keys, err)
	}
	actual, err := kv.Get(groupCtx(ctx, g.OwnerID), envPath(g.ID))
	if err != nil || !maps.Equal(actual, map[string]string{"KEEP": "after"}) {
		t.Fatalf("replace-all preserved a key absent from its desired set: %v, %v", actual, err)
	}
	projection := secret(t, first.Client, envSecretName(g.ID))
	if len(projection.Data) != 1 || string(projection.Data["KEEP"]) != "after" {
		t.Fatal("replace-all projection kept the concurrent key")
	}
}

func TestBlueprintApplyReadsEachNeededValueMapOnceUnderTheContentLock(t *testing.T) {
	kv := &observedGroupKV{fakeStore: newFakeStore()}
	svc := newService(kv)
	ctx := testCtx()
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
		SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "preserved"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"after", "after"} {
		kv.calls = nil
		if err := svc.ApplyEnvGroup(ctx, g.Name, map[string]string{"TOKEN": value}, nil); err != nil {
			t.Fatal(err)
		}
		envReads := 0
		for _, call := range kv.calls {
			if call == "Get "+g.OwnerID+"/"+envPath(g.ID) {
				envReads++
			}
		}
		if envReads != 1 {
			t.Fatalf("Blueprint apply read its locked env snapshot %d times, want one", envReads)
		}
	}
}
