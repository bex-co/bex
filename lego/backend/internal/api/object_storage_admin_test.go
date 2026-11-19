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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// memKeyRepo is an in-memory objectstorage.AccessKeyRepository.
type memKeyRepo struct {
	mu   sync.Mutex
	keys map[string]store.ObjectAccessKey
}

func newMemKeyRepo() *memKeyRepo { return &memKeyRepo{keys: map[string]store.ObjectAccessKey{}} }

func (m *memKeyRepo) CreateObjectAccessKey(_ context.Context, in store.ObjectAccessKey, _, _ int) (store.ObjectAccessKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	in.CreatedAt = time.Now()
	m.keys[in.ID] = in
	return in, nil
}

func (m *memKeyRepo) GetObjectAccessKey(_ context.Context, id string) (store.ObjectAccessKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if k, ok := m.keys[id]; ok {
		return k, nil
	}
	return store.ObjectAccessKey{}, store.ErrNotFound
}

func (m *memKeyRepo) ListObjectAccessKeys(_ context.Context, ws string) ([]store.ObjectAccessKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.ObjectAccessKey
	for _, k := range m.keys {
		if k.WorkspaceID == ws {
			out = append(out, k)
		}
	}
	return out, nil
}

func (m *memKeyRepo) RevokeObjectAccessKey(_ context.Context, ws, id string) (store.ObjectAccessKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.keys[id]
	if !ok || k.WorkspaceID != ws || k.RevokedAt != nil {
		return store.ObjectAccessKey{}, store.ErrNotFound
	}
	now := time.Now()
	k.RevokedAt = &now
	m.keys[id] = k
	return k, nil
}

func (m *memKeyRepo) MarkObjectAccessKeyBroken(context.Context, string) error       { return nil }
func (m *memKeyRepo) TouchObjectAccessKey(context.Context, string, time.Time) error { return nil }
func (m *memKeyRepo) DeleteObjectAccessKey(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.keys, id)
	return nil
}

func (m *memKeyRepo) DeleteObjectAccessKeysOf(_ context.Context, by, ws string) ([]store.ObjectAccessKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.ObjectAccessKey
	for id, k := range m.keys {
		if k.CreatedBy == by && (ws == "" || k.WorkspaceID == ws) {
			out = append(out, k)
			delete(m.keys, id)
		}
	}
	return out, nil
}

func (m *memKeyRepo) DeleteObjectAccessKeysOfWorkspace(_ context.Context, ws string) ([]store.ObjectAccessKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.ObjectAccessKey
	for id, k := range m.keys {
		if k.WorkspaceID == ws {
			out = append(out, k)
			delete(m.keys, id)
		}
	}
	return out, nil
}

func (m *memKeyRepo) DeleteEndedObjectAccessKeys(context.Context, time.Time) ([]store.ObjectAccessKey, error) {
	return nil, nil
}

type memKV struct {
	mu sync.Mutex
	m  map[string]map[string]string
}

func (k *memKV) slot(ctx context.Context, p string) string {
	return secrets.TenantFromContext(ctx) + "|" + p
}
func (k *memKV) Get(ctx context.Context, p string) (map[string]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.m[k.slot(ctx, p)], nil
}
func (k *memKV) Put(ctx context.Context, p string, d map[string]string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[k.slot(ctx, p)] = d
	return nil
}
func (k *memKV) Delete(ctx context.Context, p string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, k.slot(ctx, p))
	return nil
}
func (k *memKV) List(context.Context, string) ([]string, error) { return nil, nil }

func newAdminForTest() (*objectStorageAdmin, *memKeyRepo, *memKV) {
	repo, kv := newMemKeyRepo(), &memKV{m: map[string]map[string]string{}}
	keys := objectstorage.NewAccessKeys(repo, kv)
	return newObjectStorageAdmin(&core.Base{}, objectstorage.NewService(nil, nil), keys, "onbex.co"), repo, kv
}

func sessionCtx(workspace string) context.Context {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "usr-alice", Method: core.MethodSession, SignIn: core.SignInGoogle})
	return core.WithWorkspace(ctx, workspace)
}

func TestObjectAdminCreatesListsAndRevokesAKeyAndShowsTheSecretOnce(t *testing.T) {
	a, repo, kv := newAdminForTest()
	ctx := sessionCtx("tea-aaaaaaaaaaaaaaaaaaaa")
	created, err := a.CreateAccessKey(ctx, "tea-aaaaaaaaaaaaaaaaaaaa", createAccessKeyRequest{Name: "ci", Permission: "readwrite"})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Secret) != objectstorage.KeySecretLength || created.CreatedBy != "usr-alice" || created.Status != "active" {
		t.Fatalf("created = %+v", created)
	}
	if row := repo.keys[created.ID]; row.CreatedSignIn != core.SignInGoogle {
		t.Errorf("the key carries the sign-in %q, want the creator's Google stamp", row.CreatedSignIn)
	}
	listed, err := a.ListAccessKeys(ctx, "tea-aaaaaaaaaaaaaaaaaaaa")
	if err != nil || len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("list = %+v, %v", listed, err)
	}
	raw, _ := json.Marshal(listed)
	if strings.Contains(string(raw), created.Secret) || strings.Contains(strings.ToLower(string(raw)), "secret") {
		t.Errorf("a listing carries the secret: %s", raw)
	}
	if err := a.RevokeAccessKey(ctx, "tea-aaaaaaaaaaaaaaaaaaaa", created.ID); err != nil {
		t.Fatal(err)
	}
	listed, _ = a.ListAccessKeys(ctx, "tea-aaaaaaaaaaaaaaaaaaaa")
	if len(listed) != 1 || listed[0].Status != "revoked" {
		t.Errorf("list after revoke = %+v, want the key revoked", listed)
	}
	if len(kv.m) != 0 {
		t.Errorf("OpenBao still holds %d secrets after the revoke", len(kv.m))
	}
}

func TestAnAPIKeyAndAThirdPartyTokenCannotCreateOrRevokeAKey(t *testing.T) {
	a, repo, _ := newAdminForTest()
	for name, id := range map[string]core.Identity{
		// A machine API key: an OAuth2 token with no end user behind it.
		"api key": {Subject: "client-1", ClientID: "client-1", Method: core.MethodOAuth2},
		// A consented third-party client's human token.
		"third-party token": {Subject: "usr-alice", ClientID: "dcr-client", Method: core.MethodOAuth2, Human: true, CanonicalScopes: "bex.read bex.write"},
		// An S3 key's own identity, which no route authenticates, still cannot mint.
		"s3 key": {Subject: "usr-alice", Method: core.MethodS3Key},
	} {
		ctx := core.WithWorkspace(core.WithIdentity(context.Background(), id), "tea-aaaaaaaaaaaaaaaaaaaa")
		_, err := a.CreateAccessKey(ctx, "tea-aaaaaaaaaaaaaaaaaaaa", createAccessKeyRequest{Name: "x", Permission: "read"})
		if !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s: CreateAccessKey = %v, want ErrForbidden", name, err)
		}
		if err := a.RevokeAccessKey(ctx, "tea-aaaaaaaaaaaaaaaaaaaa", "s3k-"+strings.Repeat("a", 20)); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s: RevokeAccessKey = %v, want ErrForbidden", name, err)
		}
	}
	if len(repo.keys) != 0 {
		t.Errorf("a refused caller left %d keys", len(repo.keys))
	}
}

func TestObjectAdminRefusesWhatItCannotKeep(t *testing.T) {
	a, _, _ := newAdminForTest()
	ctx := sessionCtx("tea-aaaaaaaaaaaaaaaaaaaa")
	for name, req := range map[string]createAccessKeyRequest{
		"admin permission": {Name: "x", Permission: "manage"},
		"13 hours":         {Name: "x", Permission: "read", ExpiresInSeconds: 13 * 3600},
		"negative":         {Name: "x", Permission: "read", ExpiresInSeconds: -1},
		"no name":          {Permission: "read"},
	} {
		_, err := a.CreateAccessKey(ctx, "tea-aaaaaaaaaaaaaaaaaaaa", req)
		if err == nil {
			t.Errorf("%s: CreateAccessKey succeeded", name)
		}
	}
	// No secret store: 503, not a 500.
	off := newObjectStorageAdmin(&core.Base{}, objectstorage.NewService(nil, nil), nil, "onbex.co")
	if _, err := off.CreateAccessKey(ctx, "tea-aaaaaaaaaaaaaaaaaaaa", createAccessKeyRequest{Name: "x", Permission: "read"}); !errors.Is(adminError(err), core.ErrUnavailable) {
		t.Errorf("without OpenBao CreateAccessKey = %v, want a 503", adminError(err))
	}
}

func TestObjectAdminListsNoKeysAsAnEmptyArray(t *testing.T) {
	a, _, _ := newAdminForTest()
	mux := http.NewServeMux()
	a.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodGet, "/v1/object-storage/tea-aaaaaaaaaaaaaaaaaaaa/access-keys", nil).WithContext(sessionCtx(""))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("GET access-keys = %d %q, want 200 []", rec.Code, rec.Body.String())
	}
}

// fakeAPIKeys is the API-key half of credentialDisposal.
func TestCredentialDisposalDisposesS3KeysWithTheMembersOtherKeys(t *testing.T) {
	repo, kv := newMemKeyRepo(), &memKV{m: map[string]map[string]string{}}
	keys := objectstorage.NewAccessKeys(repo, kv)
	ctx := context.Background()
	mine, err := keys.Create(ctx, objectstorage.CreateKeyInput{WorkspaceID: "tea-a", CreatedBy: "usr-bob", Name: "m", Permission: "read"})
	if err != nil {
		t.Fatal(err)
	}
	other, _ := keys.Create(ctx, objectstorage.CreateKeyInput{WorkspaceID: "tea-b", CreatedBy: "usr-bob", Name: "o", Permission: "read"})
	theirs, _ := keys.Create(ctx, objectstorage.CreateKeyInput{WorkspaceID: "tea-a", CreatedBy: "usr-carol", Name: "t", Permission: "read"})

	d := newCredentialDisposal(nil, keys) // a deployment with no Hydra has no API keys
	if d == nil {
		t.Fatal("no disposal with S3 keys alone")
	}
	got, err := d.CleanupSubjectInWorkspace(ctx, "usr-bob", "tea-a")
	if err != nil || len(got) != 1 || got[0] != mine.Key.ID {
		t.Fatalf("CleanupSubjectInWorkspace = %v, %v, want only bob's key in tea-a", got, err)
	}
	if _, ok := repo.keys[other.Key.ID]; !ok {
		t.Error("leaving one workspace disposed of the person's key in another")
	}
	if _, ok := repo.keys[theirs.Key.ID]; !ok {
		t.Error("another member's key was disposed of")
	}
	if err := d.CleanupSubject(ctx, "usr-bob"); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.keys[other.Key.ID]; ok {
		t.Error("an account's deletion left the person's key in another workspace")
	}
	if len(kv.m) != 1 {
		t.Errorf("OpenBao holds %d secrets, want only carol's", len(kv.m))
	}
	if newCredentialDisposal(nil, nil) != nil {
		t.Error("a disposal with nothing to dispose of must be nil")
	}
}

func TestPublishingABucketNeedsCanManageAndABucketIsResolvedInsideTheCallersWorkspace(t *testing.T) {
	fake := fakes3.New(objectstorage.Oregon.Bucket())
	t.Cleanup(fake.Close)
	st := objectstorage.NewStore(objectstorage.Config{
		Endpoint: fake.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1,
	})
	repo := &memObjectRepo{}
	svc := objectstorage.NewService(repo, map[objectstorage.Region]objectstorage.Store{objectstorage.Oregon: st})
	ws, other := ids.New(ids.Workspace), ids.New(ids.Workspace)
	ctx := context.Background()
	if _, err := svc.CreateBucket(ctx, ws, "photos", objectstorage.Oregon, ""); err != nil {
		t.Fatal(err)
	}
	callCtx := sessionCtx(ws)

	// A developer: can view and create, cannot manage.
	dev := newObjectStorageAdmin(&core.Base{Authz: &relationChecker{workspace: ws, held: map[string]bool{core.RelCanView: true, core.RelCanCreate: true, core.RelCanViewSensitive: true}}}, svc, nil, "onbex.co")
	if _, err := dev.SetBucketPublic(callCtx, ws, "photos", true); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a developer publishing a bucket = %v, want ErrForbidden", err)
	}
	if b, _ := svc.GetBucket(ctx, ws, "photos"); b.Public {
		t.Fatal("a refused publish made the bucket public")
	}
	// They can read whether it is public, and its facts.
	if got, err := dev.BucketPublic(callCtx, ws, "photos"); err != nil || got.Public || !strings.HasPrefix(got.Host, "pub-obk-") && !strings.HasPrefix(got.Host, "pub-") {
		t.Errorf("BucketPublic = %+v, %v", got, err)
	}
	info, err := dev.BucketInfo(callCtx, ws, "photos")
	if err != nil || info.Name != "photos" || info.Region != "oregon" || info.Objects != 0 {
		t.Errorf("BucketInfo = %+v, %v", info, err)
	}

	// An admin may publish and unpublish.
	admin := newObjectStorageAdmin(&core.Base{Authz: &relationChecker{workspace: ws, held: map[string]bool{core.RelCanView: true, core.RelCanCreate: true, core.RelCanManage: true}}}, svc, nil, "onbex.co")
	got, err := admin.SetBucketPublic(callCtx, ws, "photos", true)
	if err != nil || !got.Public || got.Host == "" {
		t.Fatalf("an admin publishing = %+v, %v", got, err)
	}
	if b, _ := svc.GetBucket(ctx, ws, "photos"); !b.Public {
		t.Error("the bucket is not public after an admin's publish")
	}
	if got, err := admin.SetBucketPublic(callCtx, ws, "photos", false); err != nil || got.Public {
		t.Errorf("an admin unpublishing = %+v, %v", got, err)
	}
	// A deployment with no hosting domain cannot publish at all.
	noDomain := newObjectStorageAdmin(&core.Base{}, svc, nil, "")
	if _, err := noDomain.SetBucketPublic(callCtx, ws, "photos", true); !errors.Is(err, objectstorage.ErrNoHostingDomain) {
		t.Errorf("publishing with no hosting domain = %v, want ErrNoHostingDomain", err)
	}
	// A bucket is resolved inside the named workspace, after the caller is
	// authorized on it: another workspace's is forbidden, never read, and a name
	// the workspace does not hold is not found.
	if _, err := admin.BucketInfo(sessionCtx(other), other, "photos"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("another workspace's bucket = %v, want ErrForbidden", err)
	}
	if _, err := admin.BucketInfo(callCtx, ws, "nope"); !errors.Is(err, objectstorage.ErrBucketNotFound) {
		t.Errorf("a missing bucket = %v, want ErrBucketNotFound", err)
	}
}
