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

package sandbox

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type memCredentialKV struct {
	mu   sync.Mutex
	data map[string]map[string]string // "<tenant>/<path>" -> map
	err  error
}

func newMemCredentialKV() *memCredentialKV {
	return &memCredentialKV{data: map[string]map[string]string{}}
}

func (m *memCredentialKV) fullKey(ctx context.Context, path string) string {
	return secrets.TenantFromContext(ctx) + "/" + strings.TrimPrefix(path, "/")
}

func (m *memCredentialKV) Get(ctx context.Context, path string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	v, ok := m.data[m.fullKey(ctx, path)]
	if !ok {
		return map[string]string{}, nil
	}
	return maps.Clone(v), nil
}

func (m *memCredentialKV) Put(ctx context.Context, path string, data map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.data[m.fullKey(ctx, path)] = maps.Clone(data)
	return nil
}

func (m *memCredentialKV) Delete(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	delete(m.data, m.fullKey(ctx, path))
	return nil
}

func (m *memCredentialKV) List(ctx context.Context, path string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	prefix := strings.TrimSuffix(m.fullKey(ctx, path), "/") + "/"
	seen := map[string]struct{}{}
	for k := range m.data {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		seg, _, _ := strings.Cut(rest, "/")
		if seg != "" {
			seen[seg] = struct{}{}
		}
	}
	out := slices.Sorted(maps.Keys(seen))
	return out, nil
}

func (m *memCredentialKV) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.data)
}

func TestCredentialVaultPutGetDeleteAndIsolation(t *testing.T) {
	kv := newMemCredentialKV()
	vault := NewCredentialVault(kv)
	ctx := context.Background()

	if err := vault.Put(ctx, "tea-a", "sbg-1", "my-key", "ref-1", "super-secret-value"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := vault.Get(ctx, "tea-a", "sbg-1", "my-key", "ref-1")
	if err != nil || !ok || got != "super-secret-value" {
		t.Fatalf("Get(tea-a) = (%q, %v, %v), want super-secret-value", got, ok, err)
	}
	// Cross-workspace isolation: tea-b cannot read tea-a's secret even with identical group/cred/ref.
	if gotOther, okOther, err := vault.Get(ctx, "tea-b", "sbg-1", "my-key", "ref-1"); err != nil || okOther || gotOther != "" {
		t.Fatalf("Get(tea-b) = (%q, %v, %v), want absent", gotOther, okOther, err)
	}
	// Delete is idempotent.
	if err := vault.Delete(ctx, "tea-a", "sbg-1", "my-key", "ref-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := vault.Delete(ctx, "tea-a", "sbg-1", "my-key", "ref-1"); err != nil {
		t.Fatalf("repeat Delete: %v", err)
	}
	if _, ok, err := vault.Get(ctx, "tea-a", "sbg-1", "my-key", "ref-1"); err != nil || ok {
		t.Fatalf("after Delete, secret still present: ok=%v err=%v", ok, err)
	}
}

func TestCredentialVaultPrefixPurges(t *testing.T) {
	kv := newMemCredentialKV()
	vault := NewCredentialVault(kv)
	ctx := context.Background()

	// Seed two workspaces, two groups in tea-a, two credentials in sbg-1.
	mustPut := func(ws, group, cred, ref, val string) {
		t.Helper()
		if err := vault.Put(ctx, ws, group, cred, ref, val); err != nil {
			t.Fatalf("Put(%s,%s,%s,%s): %v", ws, group, cred, ref, err)
		}
	}
	mustPut("tea-a", "sbg-1", "cred-1", "ref-1", "v1")
	mustPut("tea-a", "sbg-1", "cred-1", "ref-2", "v2")
	mustPut("tea-a", "sbg-1", "cred-2", "ref-3", "v3")
	mustPut("tea-a", "sbg-2", "cred-3", "ref-4", "v4")
	mustPut("tea-b", "sbg-9", "cred-9", "ref-9", "v9")

	// DeleteCredentialPrefix removes all versions of cred-1 (including unlisted ref-2) and leaves cred-2 intact.
	if err := vault.DeleteCredentialPrefix(ctx, "tea-a", "sbg-1", "cred-1", []string{"ref-1"}); err != nil {
		t.Fatalf("DeleteCredentialPrefix: %v", err)
	}
	if _, ok, _ := vault.Get(ctx, "tea-a", "sbg-1", "cred-1", "ref-2"); ok {
		t.Fatal("unlisted ref-2 survived DeleteCredentialPrefix")
	}
	if _, ok, _ := vault.Get(ctx, "tea-a", "sbg-1", "cred-2", "ref-3"); !ok {
		t.Fatal("sibling cred-2 was unexpectedly deleted")
	}

	// DeleteGroupPrefix removes sbg-1 and leaves sbg-2 + tea-b intact.
	if err := vault.DeleteGroupPrefix(ctx, "tea-a", "sbg-1", []store.SandboxCredentialSecretRefEntry{
		{SandboxGroupID: "sbg-1", CredentialID: "cred-2", SecretRef: "ref-3"},
	}); err != nil {
		t.Fatalf("DeleteGroupPrefix: %v", err)
	}
	if _, ok, _ := vault.Get(ctx, "tea-a", "sbg-1", "cred-2", "ref-3"); ok {
		t.Fatal("sbg-1 secret survived DeleteGroupPrefix")
	}
	if _, ok, _ := vault.Get(ctx, "tea-a", "sbg-2", "cred-3", "ref-4"); !ok {
		t.Fatal("sbg-2 secret was unexpectedly deleted")
	}

	// WorkspacePurger purges tea-a's remaining secrets and leaves tea-b intact.
	purger := &WorkspacePurger{
		CredentialVault: vault,
	}
	if err := purger.PurgeWorkspace(ctx, "tea-a"); err != nil {
		t.Fatalf("PurgeWorkspace: %v", err)
	}
	if _, ok, _ := vault.Get(ctx, "tea-a", "sbg-2", "cred-3", "ref-4"); ok {
		t.Fatal("tea-a secret survived PurgeWorkspace")
	}
	if got, ok, err := vault.Get(ctx, "tea-b", "sbg-9", "cred-9", "ref-9"); err != nil || !ok || got != "v9" {
		t.Fatalf("tea-b secret affected by tea-a purge: (%q, %v, %v)", got, ok, err)
	}
}

func TestCredentialVaultSealedFailsClosedAndSanitizesErrors(t *testing.T) {
	kv := newMemCredentialKV()
	kv.err = errors.New("http://openbao-active.secrets.svc:8200/v1/tenants/data/tea-a/secret-path: Vault is sealed (token=s.leak)")
	vault := NewCredentialVault(kv)
	ctx := context.Background()

	assertSanitized503 := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("expected 503 error, got nil")
		}
		if core.ErrorStatus(err) != http.StatusServiceUnavailable {
			t.Fatalf("status = %d (%v), want 503", core.ErrorStatus(err), err)
		}
		var ce *core.CodedError
		if !errors.As(err, &ce) || ce.Code != CodeSandboxCredentialsUnavailable {
			t.Fatalf("code = %v, want %s", err, CodeSandboxCredentialsUnavailable)
		}
		for _, leaked := range []string{"openbao-active", "secret-path", "s.leak", "my-secret"} {
			if strings.Contains(err.Error(), leaked) {
				t.Errorf("sanitized error leaked %q: %s", leaked, err.Error())
			}
		}
	}

	assertSanitized503(t, vault.Put(ctx, "tea-a", "sbg-1", "cred", "ref", "my-secret"))
	_, _, err := vault.Get(ctx, "tea-a", "sbg-1", "cred", "ref")
	assertSanitized503(t, err)
	assertSanitized503(t, vault.Delete(ctx, "tea-a", "sbg-1", "cred", "ref"))
	assertSanitized503(t, vault.DeleteGroupPrefix(ctx, "tea-a", "sbg-1", nil))

	// Nil vault also fails closed with 503 on Put/Get.
	var nilVault *CredentialVault
	assertSanitized503(t, nilVault.Put(ctx, "tea-a", "sbg-1", "cred", "ref", "my-secret"))
	_, _, err = nilVault.Get(ctx, "tea-a", "sbg-1", "cred", "ref")
	assertSanitized503(t, err)
}
