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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- fakes ---

// fakeStore is an in-memory CredentialStore, workspace-scoped like the real
// Postgres rows (a lookup for the wrong workspace is ErrNotFound).
type fakeStore struct {
	rows map[string]store.RegistryCredential // by id
}

func newFakeStore() *fakeStore { return &fakeStore{rows: map[string]store.RegistryCredential{}} }

func (f *fakeStore) CreateRegistryCredential(_ context.Context, workspaceID, name, host, username, createdBy string, expiresAt *time.Time) (store.RegistryCredential, error) {
	if name == "" {
		name = host
	}
	now := time.Now().UTC()
	c := store.RegistryCredential{
		ID: ids.New(ids.RegistryCredential), WorkspaceID: workspaceID, Name: name, Host: host, Username: username,
		ExpiresAt: expiresAt, CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
	f.rows[c.ID] = c
	return c, nil
}

func (f *fakeStore) ListRegistryCredentials(_ context.Context, workspaceID string) ([]store.RegistryCredential, error) {
	var out []store.RegistryCredential
	for _, c := range f.rows {
		if c.WorkspaceID == workspaceID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeStore) GetRegistryCredential(_ context.Context, workspaceID, id string) (store.RegistryCredential, error) {
	c, ok := f.rows[id]
	if !ok || c.WorkspaceID != workspaceID {
		return store.RegistryCredential{}, store.ErrNotFound
	}
	return c, nil
}

func (f *fakeStore) GetRegistryCredentialByID(_ context.Context, id string) (store.RegistryCredential, error) {
	c, ok := f.rows[id]
	if !ok {
		return store.RegistryCredential{}, store.ErrNotFound
	}
	return c, nil
}

func (f *fakeStore) GetRegistryCredentialsByIDs(_ context.Context, ids []string) ([]store.RegistryCredential, error) {
	var out []store.RegistryCredential
	for _, id := range ids {
		if c, ok := f.rows[id]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeStore) GetRegistryCredentialByHost(_ context.Context, workspaceID, host string) (store.RegistryCredential, error) {
	for _, c := range f.rows {
		if c.WorkspaceID == workspaceID && c.Host == host {
			return c, nil
		}
	}
	return store.RegistryCredential{}, store.ErrNotFound
}

func (f *fakeStore) UpdateRegistryCredential(_ context.Context, workspaceID, id, name, username string, expiresAt *time.Time) (store.RegistryCredential, error) {
	c, ok := f.rows[id]
	if !ok || c.WorkspaceID != workspaceID {
		return store.RegistryCredential{}, store.ErrNotFound
	}
	c.Name = name
	c.Username = username
	c.ExpiresAt = expiresAt
	c.UpdatedAt = time.Now().UTC()
	f.rows[id] = c
	return c, nil
}

func (f *fakeStore) TouchRegistryCredential(_ context.Context, workspaceID, id string) error {
	c, ok := f.rows[id]
	if !ok || c.WorkspaceID != workspaceID {
		return store.ErrNotFound
	}
	c.UpdatedAt = time.Now().UTC()
	f.rows[id] = c
	return nil
}

func (f *fakeStore) DeleteRegistryCredential(_ context.Context, workspaceID, id string) error {
	c, ok := f.rows[id]
	if !ok || c.WorkspaceID != workspaceID {
		return store.ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

// fakeSecretKV is a minimal in-memory core.SecretKV.
type fakeSecretKV struct {
	m       map[string]map[string]string
	failGet error
	failPut error
}

func newFakeSecretKV() *fakeSecretKV { return &fakeSecretKV{m: map[string]map[string]string{}} }

func (f *fakeSecretKV) Get(_ context.Context, path string) (map[string]string, error) {
	if f.failGet != nil {
		return nil, f.failGet
	}
	out := map[string]string{}
	for k, v := range f.m[path] {
		out[k] = v
	}
	return out, nil
}

func (f *fakeSecretKV) Put(_ context.Context, path string, data map[string]string) error {
	if f.failPut != nil {
		return f.failPut
	}
	cp := map[string]string{}
	for k, v := range data {
		cp[k] = v
	}
	f.m[path] = cp
	return nil
}

func (f *fakeSecretKV) Delete(_ context.Context, path string) error {
	delete(f.m, path)
	return nil
}

func (f *fakeSecretKV) List(context.Context, string) ([]string, error) { return nil, nil }

func newTestService() (*Service, *fakeStore, *fakeSecretKV) {
	st := newFakeStore()
	kv := newFakeSecretKV()
	return &Service{Base: &core.Base{Namespace: "default", Client: fakeK8sClient()}, Store: st, Secret: kv}, st, kv
}

func ptrTime(t time.Time) *time.Time { return &t }

// --- tests ---

func TestCreateStoresMetadataInPostgresAndSecretInOpenBao(t *testing.T) {
	s, st, kv := newTestService()
	ctx := context.Background()

	v, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.Host != "ghcr.io" || v.Username != "alice" || v.Status != "active" {
		t.Errorf("view = %+v", v)
	}

	row, err := st.GetRegistryCredential(ctx, core.DefaultTenant, v.ID)
	if err != nil {
		t.Fatalf("row not in store: %v", err)
	}
	if row.Host != "ghcr.io" {
		t.Errorf("stored row host = %q", row.Host)
	}

	secret, err := kv.Get(ctx, secretPath(core.DefaultTenant, v.ID))
	if err != nil || secret["password"] != "hunter2" {
		t.Errorf("secret in OpenBao = %+v (err %v), want password=hunter2", secret, err)
	}
}

func TestCreateRejectsMissingFields(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()

	for _, tc := range []struct{ host, user, secret string }{
		{"", "alice", "s"}, {"ghcr.io", "", "s"}, {"ghcr.io", "alice", ""},
	} {
		if _, err := s.Create(ctx, CreateRequest{Host: tc.host, Username: tc.user, Secret: tc.secret}); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("Create(%q,%q,%q) = %v, want ErrBadRequest", tc.host, tc.user, tc.secret, err)
		}
	}
}

func TestCreateRollsBackRowOnSecretWriteFailure(t *testing.T) {
	s, st, kv := newTestService()
	kv.failPut = errors.New("openbao down")
	ctx := context.Background()

	if _, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"}); err == nil {
		t.Fatal("Create should fail when the secret write fails")
	}
	rows, _ := st.ListRegistryCredentials(ctx, core.DefaultTenant)
	if len(rows) != 0 {
		t.Errorf("a failed secret write must not leave an orphaned metadata row: %+v", rows)
	}
}

func TestListAndGetNeverReturnTheSecret(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()
	created, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// CredentialView has no Secret field at all — the compiler enforces this
	// structurally, but assert the fields present carry no secret-shaped data.
	list, err := s.List(ctx, "")
	if err != nil || len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("List = %+v (err %v)", list, err)
	}
	got, err := s.Get(ctx, created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("Get = %+v (err %v)", got, err)
	}
}

func TestGetScopedToWorkspaceCrossTenantIsNotFound(t *testing.T) {
	s, st, kv := newTestService()
	s.Workspace = fakeWorkspaceResolver{"tea-mine"}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "u1", Method: "session"})

	created, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	other := &Service{Base: &core.Base{Namespace: "default", Workspace: fakeWorkspaceResolver{"tea-other"}}, Store: st, Secret: kv}
	if _, err := other.Get(ctx, created.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("cross-workspace get = %v, want ErrNotFound", err)
	}
}

func TestUpdateUsernameAndExpiryAndRotateSecret(t *testing.T) {
	s, _, kv := newTestService()
	ctx := context.Background()
	created, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newUser := "alice2"
	newSecret := "hunter3"
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	updated, err := s.Update(ctx, created.ID, UpdateRequest{
		Username: &newUser, Secret: &newSecret, ExpiresAtSet: true, ExpiresAt: &expires,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Username != "alice2" || updated.ExpiresAt == "" {
		t.Errorf("updated view = %+v", updated)
	}
	secret, _ := kv.Get(ctx, secretPath(core.DefaultTenant, created.ID))
	if secret["password"] != "hunter3" {
		t.Errorf("secret not rotated: %+v", secret)
	}

	// ExpiresAtSet=false leaves the expiry untouched.
	untouched, err := s.Update(ctx, created.ID, UpdateRequest{})
	if err != nil || untouched.ExpiresAt != updated.ExpiresAt {
		t.Errorf("no-op update changed expiry: %+v (err %v)", untouched, err)
	}

	// ExpiresAtSet=true with a nil ExpiresAt clears it.
	cleared, err := s.Update(ctx, created.ID, UpdateRequest{ExpiresAtSet: true, ExpiresAt: nil})
	if err != nil || cleared.ExpiresAt != "" {
		t.Errorf("clear expiry = %+v (err %v), want empty ExpiresAt", cleared, err)
	}
}

func TestUpdateRejectsEmptySecretOrUsername(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()
	created, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	empty := ""
	if _, err := s.Update(ctx, created.ID, UpdateRequest{Secret: &empty}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("Update secret=\"\" = %v, want ErrBadRequest", err)
	}
	if _, err := s.Update(ctx, created.ID, UpdateRequest{Username: &empty}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("Update username=\"\" = %v, want ErrBadRequest", err)
	}
}

func TestDeleteRemovesRowAndSecret(t *testing.T) {
	s, st, kv := newTestService()
	ctx := context.Background()
	created, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := st.GetRegistryCredential(ctx, core.DefaultTenant, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("row survives delete: %v", err)
	}
	if secret, _ := kv.Get(ctx, secretPath(core.DefaultTenant, created.ID)); len(secret) != 0 {
		t.Errorf("secret survives delete: %+v", secret)
	}
}

func TestDeleteUnknownIsNotFound(t *testing.T) {
	s, _, _ := newTestService()
	if err := s.Delete(context.Background(), "rgc-doesnotexist0000"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("delete unknown id = %v, want ErrNotFound", err)
	}
}

// TestDeleteRefusesWhileExplicitlyBound proves w4/029.md #12's fix: deleting a
// credential an App still names via spec.registryCredentialId must not orphan
// that App's derived <app>-registry-pull Secret with a now-unrecoverable
// password.
func TestDeleteRefusesWhileExplicitlyBound(t *testing.T) {
	st := newFakeStore()
	kv := newFakeSecretKV()
	created, err := (&Service{Base: &core.Base{Namespace: "default"}, Store: st, Secret: kv}).Create(
		context.Background(), CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	credID := created.ID
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "bound-app", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{RegistryCredentialID: &credID},
	}
	s := &Service{Base: &core.Base{Namespace: "default", Client: fakeK8sClient(app)}, Store: st, Secret: kv}

	if err := s.Delete(context.Background(), credID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("Delete while explicitly bound = %v, want ErrConflict", err)
	}
	if _, err := st.GetRegistryCredential(context.Background(), core.DefaultTenant, credID); err != nil {
		t.Errorf("row must survive a refused delete: %v", err)
	}
}

// TestDeleteRefusesWhileImplicitlyBoundByHostMatch covers the legacy
// host-match binding path (no explicit registryCredentialId), which
// materializePullSecret also resolves credentials through.
func TestDeleteRefusesWhileImplicitlyBoundByHostMatch(t *testing.T) {
	st := newFakeStore()
	kv := newFakeSecretKV()
	created, err := (&Service{Base: &core.Base{Namespace: "default"}, Store: st, Secret: kv}).Create(
		context.Background(), CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "host-match-app", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "ghcr.io/acme/private-app:1.0"},
	}
	s := &Service{Base: &core.Base{Namespace: "default", Client: fakeK8sClient(app)}, Store: st, Secret: kv}

	if err := s.Delete(context.Background(), created.ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("Delete while host-matched = %v, want ErrConflict", err)
	}
}

// TestDeleteProceedsWhenExplicitBindingPointsElsewhere proves an App whose
// explicit binding was cleared or points at a different credential does not
// spuriously block deletion via host matching (explicit overrides implicit).
func TestDeleteProceedsWhenExplicitBindingPointsElsewhere(t *testing.T) {
	st := newFakeStore()
	kv := newFakeSecretKV()
	svc := &Service{Base: &core.Base{Namespace: "default"}, Store: st, Secret: kv}
	target, err := svc.Create(context.Background(), CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"})
	if err != nil {
		t.Fatalf("Create target: %v", err)
	}
	other, err := svc.Create(context.Background(), CreateRequest{Host: "ghcr.io", Username: "bob", Secret: "hunter3"})
	if err != nil {
		t.Fatalf("Create other: %v", err)
	}
	otherID := other.ID
	// Same host as target, but explicitly bound elsewhere — must not count as a
	// host-match binding to target.
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "elsewhere-app", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "ghcr.io/acme/app:1.0", RegistryCredentialID: &otherID},
	}
	s := &Service{Base: &core.Base{Namespace: "default", Client: fakeK8sClient(app)}, Store: st, Secret: kv}

	if err := s.Delete(context.Background(), target.ID); err != nil {
		t.Fatalf("Delete unbound-in-practice credential = %v, want nil", err)
	}
}

func TestExpiryStatusComputation(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	s := &Service{Base: &core.Base{Namespace: "default", Clock: func() time.Time { return now }}, Store: newFakeStore(), Secret: newFakeSecretKV()}
	ctx := context.Background()

	noExpiry, err := s.Create(ctx, CreateRequest{Host: "docker.io", Username: "bob", Secret: "s3cr3t"})
	if err != nil || noExpiry.Status != "active" {
		t.Errorf("no expiry = %+v (err %v), want active", noExpiry, err)
	}

	farFuture, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "bob", Secret: "s3cr3t", ExpiresAt: ptrTime(now.Add(24 * time.Hour))})
	if err != nil || farFuture.Status != "active" {
		t.Errorf("far-future expiry = %+v (err %v), want active", farFuture, err)
	}

	soon, err := s.Create(ctx, CreateRequest{Host: "docker.pkg.github.com", Username: "bob", Secret: "s3cr3t", ExpiresAt: ptrTime(now.Add(30 * time.Minute))})
	if err != nil || soon.Status != "expiring_soon" {
		t.Errorf("30m-out expiry = %+v (err %v), want expiring_soon", soon, err)
	}

	expired, err := s.Create(ctx, CreateRequest{Host: "registry.gitlab.com", Username: "bob", Secret: "s3cr3t", ExpiresAt: ptrTime(now.Add(-time.Minute))})
	if err != nil || expired.Status != "expired" {
		t.Errorf("past expiry = %+v (err %v), want expired", expired, err)
	}
}

// fakeWorkspaceResolver resolves every caller to a fixed tenant, for the
// cross-workspace scoping test.
type fakeWorkspaceResolver struct{ tenant string }

func (f fakeWorkspaceResolver) Tenant(context.Context, core.Identity) (string, bool) {
	return f.tenant, true
}

func (f fakeWorkspaceResolver) IsMember(context.Context, core.Identity, string) (bool, error) {
	return true, nil
}
