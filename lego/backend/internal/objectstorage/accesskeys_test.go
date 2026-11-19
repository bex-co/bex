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

package objectstorage_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeKV is an in-memory core.SecretKV that records the tenant each call carried
// (the store prepends it, and falls back to "default").
type fakeKV struct {
	mu     sync.Mutex
	data   map[string]map[string]string // tenant/path -> secret
	gets   int
	putErr error
	getErr error
	delErr error
}

func newFakeKV() *fakeKV { return &fakeKV{data: map[string]map[string]string{}} }

func (f *fakeKV) slot(ctx context.Context, path string) string {
	return secrets.TenantFromContext(ctx) + "|" + path
}

func (f *fakeKV) Get(ctx context.Context, path string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.data[f.slot(ctx, path)], nil
}

func (f *fakeKV) Put(ctx context.Context, path string, d map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putErr != nil {
		return f.putErr
	}
	f.data[f.slot(ctx, path)] = d
	return nil
}

func (f *fakeKV) Delete(ctx context.Context, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.data, f.slot(ctx, path))
	return nil
}

func (f *fakeKV) List(context.Context, string) ([]string, error) { return nil, nil }

func (f *fakeKV) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.data)
}

func (f *fakeKV) getCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gets
}

// countingRepo counts the reads a replica makes and can fail an insert.
type countingRepo struct {
	objectstorage.AccessKeyRepository
	mu         sync.Mutex
	gets       int
	failCreate error
}

func (c *countingRepo) GetObjectAccessKey(ctx context.Context, id string) (store.ObjectAccessKey, error) {
	c.mu.Lock()
	c.gets++
	c.mu.Unlock()
	return c.AccessKeyRepository.GetObjectAccessKey(ctx, id)
}

func (c *countingRepo) CreateObjectAccessKey(ctx context.Context, in store.ObjectAccessKey, p, e int) (store.ObjectAccessKey, error) {
	if c.failCreate != nil {
		return store.ObjectAccessKey{}, c.failCreate
	}
	return c.AccessKeyRepository.CreateObjectAccessKey(ctx, in, p, e)
}

func (c *countingRepo) lookups() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gets
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type keyEnv struct {
	WS   string
	PG   *store.PGStore
	KV   *fakeKV
	Repo *countingRepo
	Keys *objectstorage.AccessKeys
	Clk  *clock
}

func newKeyEnv(t *testing.T, name string) *keyEnv {
	t.Helper()
	pg := newPGEnv(t, name)
	t.Cleanup(func() {
		_, _ = pg.Pool.Exec(context.Background(), `DELETE FROM object_access_keys WHERE workspace_id=$1`, pg.WS)
	})
	e := &keyEnv{WS: pg.WS, PG: pg.PG, KV: newFakeKV(), Clk: &clock{t: time.Now().UTC()}}
	e.Repo = &countingRepo{AccessKeyRepository: pg.PG}
	e.Keys = objectstorage.NewAccessKeys(e.Repo, e.KV)
	e.Keys.Now = e.Clk.now
	return e
}

// replica is a second bex-s3 pod: its own caches over the same database and OpenBao.
func (e *keyEnv) replica() (*objectstorage.AccessKeys, *countingRepo) {
	repo := &countingRepo{AccessKeyRepository: e.PG}
	k := objectstorage.NewAccessKeys(repo, e.KV)
	k.Now = e.Clk.now
	return k, repo
}

func (e *keyEnv) create(t *testing.T, creator, perm string, life time.Duration) objectstorage.CreatedKey {
	t.Helper()
	c, err := e.Keys.Create(context.Background(), objectstorage.CreateKeyInput{
		WorkspaceID: e.WS, CreatedBy: creator, Name: "ci", Permission: perm, Lifetime: life, SignIn: core.SignInGoogle,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return c
}

func TestCreateReturnsTheSecretOnceAndKeepsItInOpenBaoUnderTheWorkspace(t *testing.T) {
	e := newKeyEnv(t, "s3k-create")
	c := e.create(t, "usr-a", objectstorage.KeyPermissionReadWrite, 0)
	if !strings.HasPrefix(c.Key.ID, "s3k-") || !objectstorage.KeyShape(c.Key.ID) || !ids.WellFormed(c.Key.ID) {
		t.Errorf("id = %q, want an s3k- id", c.Key.ID)
	}
	if len(c.Secret) != objectstorage.KeySecretLength {
		t.Errorf("secret length = %d, want %d", len(c.Secret), objectstorage.KeySecretLength)
	}
	slot := e.WS + "|object-storage/keys/" + c.Key.ID
	if got := e.KV.data[slot]["secret"]; got != c.Secret {
		t.Errorf("OpenBao holds %q at %s, want the returned secret", got, slot)
	}
	if c.Key.CreatedSignIn != core.SignInGoogle || c.Key.ExpiresAt != nil || c.Key.Ephemeral() {
		t.Errorf("key = %+v, want a persistent key carrying its sign-in", c.Key)
	}
	// A read never carries the secret.
	listed, err := e.Keys.List(context.Background(), e.WS)
	if err != nil || len(listed) != 1 || listed[0].ID != c.Key.ID {
		t.Fatalf("List = %+v, %v", listed, err)
	}
	cred, err := e.Keys.Authenticate(context.Background(), c.Key.ID)
	if err != nil || cred.Secret != c.Secret || cred.Key.WorkspaceID != e.WS || cred.Key.CreatedBy != "usr-a" {
		t.Fatalf("Authenticate = %+v, %v", cred, err)
	}
	// Two keys never share a secret.
	d := e.create(t, "usr-a", objectstorage.KeyPermissionRead, time.Hour)
	if d.Secret == c.Secret || d.Key.ID == c.Key.ID {
		t.Error("two keys share an id or a secret")
	}
}

func TestCreateRefusesWhatItCannotKeep(t *testing.T) {
	e := newKeyEnv(t, "s3k-create-bad")
	ctx := context.Background()
	for name, in := range map[string]objectstorage.CreateKeyInput{
		"no creator":        {WorkspaceID: e.WS, Name: "x", Permission: "read"},
		"empty name":        {WorkspaceID: e.WS, CreatedBy: "usr-a", Name: "  ", Permission: "read"},
		"control character": {WorkspaceID: e.WS, CreatedBy: "usr-a", Name: "a\nb", Permission: "read"},
		"manage permission": {WorkspaceID: e.WS, CreatedBy: "usr-a", Name: "x", Permission: "admin"},
		"no permission":     {WorkspaceID: e.WS, CreatedBy: "usr-a", Name: "x"},
		"13 hours":          {WorkspaceID: e.WS, CreatedBy: "usr-a", Name: "x", Permission: "read", Lifetime: 13 * time.Hour},
		"negative life":     {WorkspaceID: e.WS, CreatedBy: "usr-a", Name: "x", Permission: "read", Lifetime: -time.Hour},
	} {
		if _, err := e.Keys.Create(ctx, in); !errors.Is(err, objectstorage.ErrAccessKeyInvalidInput) {
			t.Errorf("%s: Create = %v, want ErrAccessKeyInvalidInput", name, err)
		}
	}
	if e.KV.count() != 0 {
		t.Errorf("a refused create left %d secrets in OpenBao", e.KV.count())
	}
}

func TestAFailedRowInsertLeavesNoSecretInOpenBao(t *testing.T) {
	e := newKeyEnv(t, "s3k-rollback")
	boom := errors.New("insert failed")
	e.Repo.failCreate = boom
	_, err := e.Keys.Create(context.Background(), objectstorage.CreateKeyInput{WorkspaceID: e.WS, CreatedBy: "usr-a", Name: "x", Permission: "read"})
	if !errors.Is(err, boom) {
		t.Fatalf("Create = %v, want the insert error", err)
	}
	if e.KV.count() != 0 {
		t.Errorf("OpenBao holds %d secrets after a failed insert, want none", e.KV.count())
	}
	if keys, _ := e.Keys.List(context.Background(), e.WS); len(keys) != 0 {
		t.Errorf("List = %+v, want none", keys)
	}
}

func TestAnOpenBaoFailureOnCreateStoresNoRow(t *testing.T) {
	e := newKeyEnv(t, "s3k-bao-create")
	e.KV.putErr = errors.New("sealed")
	_, err := e.Keys.Create(context.Background(), objectstorage.CreateKeyInput{WorkspaceID: e.WS, CreatedBy: "usr-a", Name: "x", Permission: "read"})
	if !errors.Is(err, objectstorage.ErrSecretStoreUnavailable) {
		t.Fatalf("Create = %v, want ErrSecretStoreUnavailable", err)
	}
	if keys, _ := e.Keys.List(context.Background(), e.WS); len(keys) != 0 {
		t.Errorf("a key row exists without its secret: %+v", keys)
	}
}

func TestRevokeLeavesNoUsableKeyAndDeletesTheSecret(t *testing.T) {
	e := newKeyEnv(t, "s3k-revoke")
	ctx := context.Background()
	c := e.create(t, "usr-a", objectstorage.KeyPermissionRead, 0)
	if _, err := e.Keys.Authenticate(ctx, c.Key.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.Keys.Revoke(ctx, "tea-other00000000000000", c.Key.ID); !errors.Is(err, objectstorage.ErrAccessKeyNotFound) {
		t.Errorf("Revoke in another workspace = %v, want ErrAccessKeyNotFound", err)
	}
	if _, err := e.Keys.Authenticate(ctx, c.Key.ID); err != nil {
		t.Errorf("a foreign revoke ended the key: %v", err)
	}
	if err := e.Keys.Revoke(ctx, e.WS, c.Key.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Keys.Authenticate(ctx, c.Key.ID); !errors.Is(err, objectstorage.ErrInvalidAccessKey) {
		t.Errorf("Authenticate after revoke = %v, want ErrInvalidAccessKey on this replica at once", err)
	}
	if e.KV.count() != 0 {
		t.Errorf("OpenBao still holds %d secrets after a revoke", e.KV.count())
	}
	if err := e.Keys.Revoke(ctx, e.WS, c.Key.ID); !errors.Is(err, objectstorage.ErrAccessKeyNotFound) {
		t.Errorf("a second revoke = %v, want ErrAccessKeyNotFound", err)
	}
	// The row stays listed, revoked, until the sweep deletes it.
	listed, _ := e.Keys.List(ctx, e.WS)
	if len(listed) != 1 || listed[0].RevokedAt == nil {
		t.Errorf("List = %+v, want the revoked key", listed)
	}
}

func TestARevokeReachesAnotherReplicaWithinTheRowCacheWindow(t *testing.T) {
	e := newKeyEnv(t, "s3k-replicas")
	ctx := context.Background()
	c := e.create(t, "usr-a", objectstorage.KeyPermissionRead, 0)
	other, _ := e.replica()
	if _, err := other.Authenticate(ctx, c.Key.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.Keys.Revoke(ctx, e.WS, c.Key.ID); err != nil {
		t.Fatal(err)
	}
	// The secret was deleted, but the other replica's caches still answer inside the window.
	e.Clk.advance(objectstorage.AccessKeyRowCacheTTL - time.Second)
	if _, err := other.Authenticate(ctx, c.Key.ID); err != nil {
		t.Errorf("inside the row window the other replica answered %v, want the cached key", err)
	}
	e.Clk.advance(2 * time.Second)
	if _, err := other.Authenticate(ctx, c.Key.ID); !errors.Is(err, objectstorage.ErrInvalidAccessKey) {
		t.Errorf("past the row window the other replica answered %v, want ErrInvalidAccessKey", err)
	}
}

func TestASecretIsReadFromOpenBaoAtMostOncePerCacheWindow(t *testing.T) {
	e := newKeyEnv(t, "s3k-secret-cache")
	ctx := context.Background()
	c := e.create(t, "usr-a", objectstorage.KeyPermissionRead, 0)
	base := e.KV.getCalls()
	for i := 0; i < 50; i++ {
		if _, err := e.Keys.Authenticate(ctx, c.Key.ID); err != nil {
			t.Fatal(err)
		}
	}
	if got := e.KV.getCalls() - base; got != 1 {
		t.Errorf("50 requests read OpenBao %d times, want once", got)
	}
	e.Clk.advance(objectstorage.AccessKeySecretCacheTTL + time.Second)
	if _, err := e.Keys.Authenticate(ctx, c.Key.ID); err != nil {
		t.Fatal(err)
	}
	if got := e.KV.getCalls() - base; got != 2 {
		t.Errorf("after the window OpenBao was read %d times in all, want 2", got)
	}
}

func TestAnUnknownKeyCostsOneLookupPer30SecondsAndAMalformedIDNone(t *testing.T) {
	e := newKeyEnv(t, "s3k-miss")
	ctx := context.Background()
	unknown := ids.New(ids.ObjectAccessKey)
	for i := 0; i < 20; i++ {
		if _, err := e.Keys.Authenticate(ctx, unknown); !errors.Is(err, objectstorage.ErrInvalidAccessKey) {
			t.Fatalf("Authenticate(unknown) = %v", err)
		}
	}
	if got := e.Repo.lookups(); got != 1 {
		t.Errorf("20 requests for an unknown id made %d lookups, want 1 (a miss is cached)", got)
	}
	e.Clk.advance(objectstorage.AccessKeyRowCacheTTL + time.Second)
	_, _ = e.Keys.Authenticate(ctx, unknown)
	if got := e.Repo.lookups(); got != 2 {
		t.Errorf("after the window: %d lookups, want 2", got)
	}
	before := e.Repo.lookups()
	for _, bad := range []string{"", "AKIAIOSFODNN7EXAMPLE", "usr-c185th5c2rvvnhbfiltg", "s3k-short", "s3k-" + strings.Repeat("z", 20), "S3K-" + strings.Repeat("a", 20), ids.New(ids.ObjectBucket)} {
		if _, err := e.Keys.Authenticate(ctx, bad); !errors.Is(err, objectstorage.ErrInvalidAccessKey) {
			t.Errorf("Authenticate(%q) = %v, want ErrInvalidAccessKey", bad, err)
		}
	}
	if e.Repo.lookups() != before {
		t.Error("a malformed key id reached the database")
	}
}

func TestAnEphemeralKeyExpires(t *testing.T) {
	e := newKeyEnv(t, "s3k-expiry")
	ctx := context.Background()
	c := e.create(t, "usr-a", objectstorage.KeyPermissionRead, 2*time.Hour)
	if c.Key.ExpiresAt == nil || !c.Key.Ephemeral() {
		t.Fatalf("key = %+v, want an expiry", c.Key)
	}
	if _, err := e.Keys.Authenticate(ctx, c.Key.ID); err != nil {
		t.Fatal(err)
	}
	e.Clk.advance(2*time.Hour + time.Second)
	if _, err := e.Keys.Authenticate(ctx, c.Key.ID); !errors.Is(err, objectstorage.ErrInvalidAccessKey) {
		t.Errorf("Authenticate past the expiry = %v, want ErrInvalidAccessKey", err)
	}
}

func TestAnOpenBaoErrorIsServiceUnavailableNeverAnAuthenticationFailure(t *testing.T) {
	e := newKeyEnv(t, "s3k-bao-down")
	ctx := context.Background()
	c := e.create(t, "usr-a", objectstorage.KeyPermissionRead, 0)
	other, _ := e.replica() // a replica with nothing cached
	e.KV.getErr = errors.New("connection refused")
	if _, err := other.Authenticate(ctx, c.Key.ID); !errors.Is(err, objectstorage.ErrSecretStoreUnavailable) || errors.Is(err, objectstorage.ErrInvalidAccessKey) {
		t.Fatalf("Authenticate with OpenBao down = %v, want ErrSecretStoreUnavailable and not ErrInvalidAccessKey", err)
	}
	// The failure is not cached: the next request asks again and succeeds.
	e.KV.getErr = nil
	if _, err := other.Authenticate(ctx, c.Key.ID); err != nil {
		t.Errorf("Authenticate after OpenBao recovered = %v", err)
	}
}

func TestAKeyWhoseSecretIsMissingIsRevokedAndListedAsBroken(t *testing.T) {
	e := newKeyEnv(t, "s3k-broken")
	ctx := context.Background()
	c := e.create(t, "usr-a", objectstorage.KeyPermissionRead, 0)
	e.KV.mu.Lock()
	delete(e.KV.data, e.WS+"|object-storage/keys/"+c.Key.ID) // OpenBao restored from an older snapshot
	e.KV.mu.Unlock()
	fresh, _ := e.replica()
	if _, err := fresh.Authenticate(ctx, c.Key.ID); !errors.Is(err, objectstorage.ErrInvalidAccessKey) {
		t.Fatalf("Authenticate = %v, want ErrInvalidAccessKey", err)
	}
	listed, _ := e.Keys.List(ctx, e.WS)
	if len(listed) != 1 || !listed[0].Broken || listed[0].RevokedAt == nil {
		t.Errorf("List = %+v, want the key revoked and broken", listed)
	}
}

func TestTheKeyCapsHoldPerWorkspaceAndPerCreator(t *testing.T) {
	e := newKeyEnv(t, "s3k-caps")
	ctx := context.Background()
	for i := 0; i < objectstorage.MaxPersistentAccessKeys; i++ {
		// Different creators: the persistent cap is the workspace's.
		if _, err := e.Keys.Create(ctx, objectstorage.CreateKeyInput{WorkspaceID: e.WS, CreatedBy: fmt.Sprintf("usr-%d", i), Name: "p", Permission: "read"}); err != nil {
			t.Fatalf("persistent key %d: %v", i, err)
		}
	}
	if _, err := e.Keys.Create(ctx, objectstorage.CreateKeyInput{WorkspaceID: e.WS, CreatedBy: "usr-x", Name: "p", Permission: "read"}); !errors.Is(err, objectstorage.ErrAccessKeyLimit) {
		t.Errorf("persistent key %d = %v, want ErrAccessKeyLimit", objectstorage.MaxPersistentAccessKeys+1, err)
	}
	// The cap counts live keys: revoking one frees a place.
	listed, _ := e.Keys.List(ctx, e.WS)
	if err := e.Keys.Revoke(ctx, e.WS, listed[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Keys.Create(ctx, objectstorage.CreateKeyInput{WorkspaceID: e.WS, CreatedBy: "usr-x", Name: "p", Permission: "read"}); err != nil {
		t.Errorf("a persistent key after a revoke: %v", err)
	}
	// Ephemeral keys are capped per creator and do not eat the persistent cap.
	for i := 0; i < objectstorage.MaxEphemeralAccessKeys; i++ {
		e.create(t, "usr-eph", objectstorage.KeyPermissionRead, time.Hour)
	}
	if _, err := e.Keys.Create(ctx, objectstorage.CreateKeyInput{WorkspaceID: e.WS, CreatedBy: "usr-eph", Name: "e", Permission: "read", Lifetime: time.Hour}); !errors.Is(err, objectstorage.ErrAccessKeyLimit) {
		t.Errorf("ephemeral key %d = %v, want ErrAccessKeyLimit", objectstorage.MaxEphemeralAccessKeys+1, err)
	}
	if _, err := e.Keys.Create(ctx, objectstorage.CreateKeyInput{WorkspaceID: e.WS, CreatedBy: "usr-other", Name: "e", Permission: "read", Lifetime: time.Hour}); err != nil {
		t.Errorf("another creator's ephemeral key: %v", err)
	}
}

func TestExpiredEphemeralKeysFreeTheirPlaceInTheCap(t *testing.T) {
	e := newKeyEnv(t, "s3k-caps-expiry")
	ctx := context.Background()
	past := time.Now().Add(-time.Minute)
	for i := 0; i < objectstorage.MaxEphemeralAccessKeys; i++ {
		_, err := e.PG.CreateObjectAccessKey(ctx, store.ObjectAccessKey{
			ID: ids.New(ids.ObjectAccessKey), WorkspaceID: e.WS, CreatedBy: "usr-eph", Name: "old", Permission: "read", ExpiresAt: &past,
		}, objectstorage.MaxPersistentAccessKeys, objectstorage.MaxEphemeralAccessKeys)
		if err != nil {
			t.Fatalf("expired key %d: %v (an expired key must not count against the cap)", i, err)
		}
	}
	if _, err := e.Keys.Create(ctx, objectstorage.CreateKeyInput{WorkspaceID: e.WS, CreatedBy: "usr-eph", Name: "e", Permission: "read", Lifetime: time.Minute}); err != nil {
		t.Errorf("Create beside %d expired keys = %v, want success", objectstorage.MaxEphemeralAccessKeys, err)
	}
}

func TestDisposeDeletesAPersonsKeysAndSecretsAndNobodyElses(t *testing.T) {
	e := newKeyEnv(t, "s3k-dispose")
	ctx := context.Background()
	mine := e.create(t, "usr-leaving", objectstorage.KeyPermissionReadWrite, 0)
	eph := e.create(t, "usr-leaving", objectstorage.KeyPermissionRead, time.Hour)
	theirs := e.create(t, "usr-staying", objectstorage.KeyPermissionRead, 0)
	gone, err := e.Keys.Dispose(ctx, "usr-leaving", e.WS)
	if err != nil || len(gone) != 2 {
		t.Fatalf("Dispose = %v, %v, want the two ids it deleted", gone, err)
	}
	for _, id := range []string{mine.Key.ID, eph.Key.ID} {
		if _, err := e.Keys.Authenticate(ctx, id); !errors.Is(err, objectstorage.ErrInvalidAccessKey) {
			t.Errorf("a disposed key %s still authenticates: %v", id, err)
		}
	}
	if _, err := e.Keys.Authenticate(ctx, theirs.Key.ID); err != nil {
		t.Errorf("another member's key was disposed: %v", err)
	}
	if e.KV.count() != 1 {
		t.Errorf("OpenBao holds %d secrets, want only the staying member's", e.KV.count())
	}
	if _, err := e.PG.GetObjectAccessKey(ctx, mine.Key.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a disposed row is still there: %v (it must be deleted, not revoked)", err)
	}
}

func TestSweepEndedDeletesRevokedAndExpiredKeysAfterSevenDays(t *testing.T) {
	e := newKeyEnv(t, "s3k-sweep")
	ctx := context.Background()
	revoked := e.create(t, "usr-a", objectstorage.KeyPermissionRead, 0)
	expired := e.create(t, "usr-a", objectstorage.KeyPermissionRead, time.Hour)
	live := e.create(t, "usr-a", objectstorage.KeyPermissionRead, 0)
	if err := e.Keys.Revoke(ctx, e.WS, revoked.Key.ID); err != nil {
		t.Fatal(err)
	}
	e.Clk.advance(6 * 24 * time.Hour)
	if n, err := e.Keys.SweepEnded(ctx); err != nil || n != 0 {
		t.Fatalf("SweepEnded inside the retention = %d, %v, want 0", n, err)
	}
	e.Clk.advance(2*24*time.Hour + time.Hour)
	n, err := e.Keys.SweepEnded(ctx)
	if err != nil || n != 2 {
		t.Fatalf("SweepEnded after the retention = %d, %v, want 2", n, err)
	}
	listed, _ := e.Keys.List(ctx, e.WS)
	if len(listed) != 1 || listed[0].ID != live.Key.ID {
		t.Errorf("List = %+v, want only the live key", listed)
	}
	_ = expired
}

func TestPurgeWorkspaceDeletesItsKeysAndSecretsAndAnotherWorkspacesAreUntouched(t *testing.T) {
	e := newKeyEnv(t, "s3k-purge")
	ctx := context.Background()
	a := e.create(t, "usr-a", objectstorage.KeyPermissionReadWrite, 0)
	b := e.create(t, "usr-b", objectstorage.KeyPermissionRead, time.Hour)
	// A key of another workspace, with its own secret.
	other := newKeyEnv(t, "s3k-purge-other")
	other.KV = e.KV
	other.Keys = objectstorage.NewAccessKeys(other.Repo, e.KV)
	keep := other.create(t, "usr-c", objectstorage.KeyPermissionRead, 0)
	if e.KV.count() != 3 {
		t.Fatalf("OpenBao holds %d secrets before the purge, want 3", e.KV.count())
	}
	if err := e.Keys.PurgeWorkspace(ctx, e.WS); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a.Key.ID, b.Key.ID} {
		if _, err := e.PG.GetObjectAccessKey(ctx, id); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("key %s survived its workspace's purge: %v", id, err)
		}
		if _, err := e.Keys.Authenticate(ctx, id); !errors.Is(err, objectstorage.ErrInvalidAccessKey) {
			t.Errorf("a purged key %s still authenticates: %v", id, err)
		}
	}
	if e.KV.count() != 1 {
		t.Errorf("OpenBao holds %d secrets after the purge, want only the other workspace's", e.KV.count())
	}
	if _, err := other.Keys.Authenticate(ctx, keep.Key.ID); err != nil {
		t.Errorf("another workspace's key was purged: %v", err)
	}
}

func TestDisabledServicesRefuseEverything(t *testing.T) {
	k := objectstorage.NewAccessKeys(nil, nil)
	if k.Enabled() {
		t.Error("a service with no store is enabled")
	}
	if _, err := k.Create(context.Background(), objectstorage.CreateKeyInput{WorkspaceID: "tea-x", CreatedBy: "u", Name: "x", Permission: "read"}); !errors.Is(err, objectstorage.ErrSecretStoreUnavailable) {
		t.Errorf("Create = %v", err)
	}
	if _, err := k.Authenticate(context.Background(), ids.New(ids.ObjectAccessKey)); !errors.Is(err, objectstorage.ErrSecretStoreUnavailable) {
		t.Errorf("Authenticate = %v", err)
	}
}
