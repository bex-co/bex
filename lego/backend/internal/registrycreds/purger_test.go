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

package registrycreds

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestWorkspacePurgerDeletesSecretsButLeavesRowsForCascade(t *testing.T) {
	s, st, kv := newTestService()
	ctx := context.Background()
	first, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "one"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Create(ctx, CreateRequest{Host: "docker.io", Username: "alice", Secret: "two"})
	if err != nil {
		t.Fatal(err)
	}

	if err := (&WorkspacePurger{Service: s}).PurgeWorkspace(ctx, core.DefaultTenant); err != nil {
		t.Fatalf("PurgeWorkspace: %v", err)
	}
	for _, id := range []string{first.ID, second.ID} {
		if secret, _ := kv.Get(ctx, secretPath(core.DefaultTenant, id)); len(secret) != 0 {
			t.Errorf("secret %s survives purge: %+v", id, secret)
		}
		if _, err := st.GetRegistryCredential(ctx, core.DefaultTenant, id); err != nil {
			t.Errorf("metadata row %s must remain for FK cascade: %v", id, err)
		}
	}
}

func TestWorkspacePurgerSurfacesOpenBaoFailure(t *testing.T) {
	s, _, kv := newTestService()
	if _, err := s.Create(context.Background(), CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "one"}); err != nil {
		t.Fatal(err)
	}
	kv.failDelete = errors.New("openbao down")
	if err := (&WorkspacePurger{Service: s}).PurgeWorkspace(context.Background(), core.DefaultTenant); err == nil {
		t.Fatal("PurgeWorkspace should surface OpenBao failure")
	}
}

func TestWorkspacePurgerDisabledIsNoOp(t *testing.T) {
	if err := (&WorkspacePurger{}).PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("disabled PurgeWorkspace: %v", err)
	}
}
