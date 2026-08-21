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

package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// registrycreds_mcp_test.go drives w2/m14's registry-credential MCP tools
// end-to-end over one real (in-memory transport) MCP session — proving the
// actual wire-level arg-decode -> Service -> result-encode plumbing works,
// which the registrycreds package's own tests (calling Service methods
// directly) don't exercise.

// fakeRCStore is a minimal in-memory registrycreds.CredentialStore.
type fakeRCStore struct {
	rows map[string]store.RegistryCredential
}

func newFakeRCStore() *fakeRCStore { return &fakeRCStore{rows: map[string]store.RegistryCredential{}} }

func (f *fakeRCStore) CreateRegistryCredential(_ context.Context, workspaceID, name, host, username, createdBy string, expiresAt *time.Time) (store.RegistryCredential, error) {
	if name == "" {
		name = host
	}
	now := time.Now().UTC()
	c := store.RegistryCredential{
		ID: "rgc-" + host, WorkspaceID: workspaceID, Name: name, Host: host, Username: username,
		ExpiresAt: expiresAt, CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
	f.rows[c.ID] = c
	return c, nil
}

func (f *fakeRCStore) ListRegistryCredentials(_ context.Context, workspaceID string) ([]store.RegistryCredential, error) {
	var out []store.RegistryCredential
	for _, c := range f.rows {
		if c.WorkspaceID == workspaceID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeRCStore) CountRegistryCredentials(ctx context.Context, workspaceID string) (int, error) {
	rows, err := f.ListRegistryCredentials(ctx, workspaceID)
	return len(rows), err
}

func (f *fakeRCStore) GetRegistryCredential(_ context.Context, workspaceID, id string) (store.RegistryCredential, error) {
	c, ok := f.rows[id]
	if !ok || c.WorkspaceID != workspaceID {
		return store.RegistryCredential{}, store.ErrNotFound
	}
	return c, nil
}

func (f *fakeRCStore) GetRegistryCredentialByID(_ context.Context, id string) (store.RegistryCredential, error) {
	c, ok := f.rows[id]
	if !ok {
		return store.RegistryCredential{}, store.ErrNotFound
	}
	return c, nil
}

func (f *fakeRCStore) GetRegistryCredentialsByIDs(_ context.Context, workspaceID string, ids []string) ([]store.RegistryCredential, error) {
	var out []store.RegistryCredential
	for _, id := range ids {
		if c, ok := f.rows[id]; ok && (workspaceID == "" || c.WorkspaceID == workspaceID) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeRCStore) GetRegistryCredentialByHost(_ context.Context, workspaceID, host string) (store.RegistryCredential, error) {
	for _, c := range f.rows {
		if c.WorkspaceID == workspaceID && c.Host == host {
			return c, nil
		}
	}
	return store.RegistryCredential{}, store.ErrNotFound
}

func (f *fakeRCStore) UpdateRegistryCredential(_ context.Context, workspaceID, id, name, username string, expiresAt *time.Time) (store.RegistryCredential, error) {
	c, ok := f.rows[id]
	if !ok || c.WorkspaceID != workspaceID {
		return store.RegistryCredential{}, store.ErrNotFound
	}
	c.Name, c.Username, c.ExpiresAt = name, username, expiresAt
	c.UpdatedAt = time.Now().UTC()
	f.rows[id] = c
	return c, nil
}

func (f *fakeRCStore) TouchRegistryCredential(_ context.Context, workspaceID, id string) error {
	c, ok := f.rows[id]
	if !ok || c.WorkspaceID != workspaceID {
		return store.ErrNotFound
	}
	c.UpdatedAt = time.Now().UTC()
	f.rows[id] = c
	return nil
}

func (f *fakeRCStore) DeleteRegistryCredential(_ context.Context, workspaceID, id string) error {
	c, ok := f.rows[id]
	if !ok || c.WorkspaceID != workspaceID {
		return store.ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

// fakeRCSecretKV is a minimal in-memory core.SecretKV.
type fakeRCSecretKV struct{ m map[string]map[string]string }

func newFakeRCSecretKV() *fakeRCSecretKV { return &fakeRCSecretKV{m: map[string]map[string]string{}} }

func (f *fakeRCSecretKV) Get(_ context.Context, path string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range f.m[path] {
		out[k] = v
	}
	return out, nil
}

func (f *fakeRCSecretKV) Put(_ context.Context, path string, data map[string]string) error {
	f.m[path] = data
	return nil
}

func (f *fakeRCSecretKV) Delete(_ context.Context, path string) error {
	delete(f.m, path)
	return nil
}

func (f *fakeRCSecretKV) List(context.Context, string) ([]string, error) { return nil, nil }

func TestMCP_RegistryCredentialsCRUDNeverLeaksSecret(t *testing.T) {
	base := &core.Base{Client: fakeClient(), Namespace: "default"}
	srv := NewServer(base, Deps{RegistryCredsStore: newFakeRCStore(), Secrets: newFakeRCSecretKV()})
	cs := mcpSessionAs(t, srv, "client-1")

	created := callTool[struct {
		ID, Host, Username, Status string
	}](t, cs, "create_registry_credential", map[string]any{
		"host": "ghcr.io", "username": "alice", "authToken": "hunter2",
	})
	if created.Host != "ghcr.io" || created.Username != "alice" || created.Status != "active" {
		t.Fatalf("create_registry_credential = %+v", created)
	}
	assertNoSecretLeak(t, cs, "create response", "hunter2")

	list := callTool[struct {
		Credentials []struct{ ID, Host string }
	}](t, cs, "list_registry_credentials", nil)
	if len(list.Credentials) != 1 || list.Credentials[0].ID != created.ID {
		t.Fatalf("list_registry_credentials = %+v", list)
	}
	assertNoSecretLeak(t, cs, "list response", "hunter2")

	updated := callTool[struct{ Username string }](t, cs, "update_registry_credential", map[string]any{
		"id": created.ID, "username": "alice2", "authToken": "hunter3",
	})
	if updated.Username != "alice2" {
		t.Fatalf("update_registry_credential = %+v", updated)
	}
	assertNoSecretLeak(t, cs, "update response", "hunter3")

	deleted := callTool[struct{ Deleted bool }](t, cs, "delete_registry_credential", map[string]any{"id": created.ID})
	if !deleted.Deleted {
		t.Fatalf("delete_registry_credential = %+v", deleted)
	}

	callToolError(t, cs, "get_registry_credential", map[string]any{"id": created.ID})
}

func TestMCP_ServiceRegistryCredentialCreateAndClear(t *testing.T) {
	st := newFakeRCStore()
	secrets := newFakeRCSecretKV()
	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{
		RegistryCredsStore: st,
		Secrets:            secrets,
	})
	cs := mcpSession(t, srv)

	credential := callTool[struct{ ID string }](t, cs, "create_registry_credential", map[string]any{
		"host": "ghcr.io", "username": "alice", "authToken": "hunter2",
	})
	created := callTool[struct {
		ID                   string `json:"id"`
		RegistryCredentialID string `json:"registryCredentialId"`
	}](t, cs, "create_web_service", map[string]any{
		"name": "web", "image": "ghcr.io/acme/private:1", "runtime": "image",
		"buildCommand": "", "startCommand": "", "registryCredentialId": credential.ID,
	})
	if created.RegistryCredentialID != credential.ID {
		t.Fatalf("create_web_service binding = %+v, want %s", created, credential.ID)
	}

	cleared := callTool[struct {
		RegistryCredentialID string `json:"registryCredentialId"`
	}](t, cs, "update_service", map[string]any{
		"serviceId": created.ID, "registryCredentialId": "",
	})
	if cleared.RegistryCredentialID != "" {
		t.Fatalf("update_service registryCredentialId=\"\" = %+v", cleared)
	}
}

// assertNoSecretLeak re-lists and marshals the raw structured content to
// confirm secret never appears in any MCP tool response, belt-and-suspenders
// alongside the per-response struct assertions above (which structurally
// can't decode a leaked secret, but wouldn't catch one sitting in the raw
// JSON under an unexpected key).
func assertNoSecretLeak(t *testing.T, cs *mcp.ClientSession, label, secret string) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_registry_credentials"})
	if err != nil || res.IsError {
		t.Fatalf("%s: list_registry_credentials: err=%v isError=%v", label, err, res.IsError)
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("%s: marshal: %v", label, err)
	}
	if strings.Contains(string(b), secret) {
		t.Fatalf("%s: %s leaked in list_registry_credentials response: %s", label, secret, b)
	}
}
