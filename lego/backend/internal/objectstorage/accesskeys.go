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

package objectstorage

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// S3 access keys (ADR112 D3). A key is a Postgres row that names the workspace it
// reaches and the person it acts as; its 40-character secret is in OpenBao at
// object-storage/keys/<id>, because SigV4 needs the secret itself to verify a
// request and a hash would not do. The service holds no HTTP and no
// authorization: the adapters authorize (the mint class and can_create to create
// or revoke, can_view to list), and the S3 surface authenticates with Authenticate.

// The permissions a key carries (ADR112 D3). Neither includes can_manage, so no
// key can publish a bucket.
const (
	KeyPermissionRead      = "read"
	KeyPermissionReadWrite = "readwrite"
)

// Errors of the key service.
var (
	// ErrInvalidAccessKey is a key id that names no usable key: malformed,
	// unknown, revoked or expired. The S3 surface answers all of them
	// InvalidAccessKeyId, so existence leaks no more than it does on S3.
	ErrInvalidAccessKey = errors.New("the access key id is not valid")
	// ErrSecretStoreUnavailable is an OpenBao failure on the key path. It is
	// never an authentication failure: SDKs retry a 503 and do not retry a 403.
	ErrSecretStoreUnavailable = core.Unavailable("the secret store is unavailable")
	// ErrAccessKeyLimit refuses a key past its cap (MaxPersistentAccessKeys per
	// workspace, MaxEphemeralAccessKeys per creator).
	ErrAccessKeyLimit = errors.New("access key limit reached")
	// ErrAccessKeyInvalidInput is a name, permission or lifetime the service refuses.
	ErrAccessKeyInvalidInput = errors.New("invalid access key")
	// ErrAccessKeyNotFound is a key that is missing or in another workspace.
	ErrAccessKeyNotFound = errors.New("access key not found")
)

// AccessKeyRepository is the part of the store the key service reads and
// writes. *store.PGStore implements it.
type AccessKeyRepository interface {
	CreateObjectAccessKey(ctx context.Context, in store.ObjectAccessKey, maxPersistent, maxEphemeral int) (store.ObjectAccessKey, error)
	GetObjectAccessKey(ctx context.Context, id string) (store.ObjectAccessKey, error)
	ListObjectAccessKeys(ctx context.Context, workspaceID string) ([]store.ObjectAccessKey, error)
	RevokeObjectAccessKey(ctx context.Context, workspaceID, id string) (store.ObjectAccessKey, error)
	MarkObjectAccessKeyBroken(ctx context.Context, id string) error
	TouchObjectAccessKey(ctx context.Context, id string, at time.Time) error
	DeleteObjectAccessKey(ctx context.Context, id string) error
	DeleteObjectAccessKeysOf(ctx context.Context, createdBy, workspaceID string) ([]store.ObjectAccessKey, error)
	DeleteObjectAccessKeysOfWorkspace(ctx context.Context, workspaceID string) ([]store.ObjectAccessKey, error)
	DeleteEndedObjectAccessKeys(ctx context.Context, cutoff time.Time) ([]store.ObjectAccessKey, error)
}

// AccessKeys is the key service.
type AccessKeys struct {
	repo    AccessKeyRepository
	secrets core.SecretKV
	// Now supplies the clock; nil is time.Now. Injectable so a test crosses an
	// expiry or a cache window without sleeping.
	Now func() time.Time

	rows    *core.TTLCache[keyLookup]
	secret  *core.TTLCache[string]
	touched sync.Map // key id -> time.Time of the last last_used_at write on this replica
}

// keyLookup is one cached answer about a key id: the row, or that none exists
// (a miss is cached too, so a flood of unknown ids costs one lookup each per 30 s).
type keyLookup struct {
	key   store.ObjectAccessKey
	found bool
}

// NewAccessKeys returns the service. A nil secret store leaves it disabled
// (every method answers ErrSecretStoreUnavailable), as an installation without
// OpenBao has no way to keep a secret.
func NewAccessKeys(repo AccessKeyRepository, kv core.SecretKV) *AccessKeys {
	a := &AccessKeys{repo: repo, secrets: kv, rows: core.NewTTLCache[keyLookup](), secret: core.NewTTLCache[string]()}
	// The caches read the service's clock, so a test that sets Now crosses a
	// window without sleeping.
	a.rows.Now, a.secret.Now = a.now, a.now
	return a
}

func (a *AccessKeys) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// Enabled reports whether keys can be kept.
func (a *AccessKeys) Enabled() bool { return a != nil && a.repo != nil && a.secrets != nil }

// secretPath is a key's secret in OpenBao, below the workspace's tenant (the
// store prepends the context's tenant and falls back to "default", so every
// call below carries WithTenant).
func secretPath(id string) string { return "object-storage/keys/" + id }

func (a *AccessKeys) tenantCtx(ctx context.Context, workspaceID string) context.Context {
	return secrets.WithTenant(ctx, workspaceID)
}

// KeyShape reports whether id looks like an access key id, before any lookup:
// s3k- and 20 base32-hex characters.
func KeyShape(id string) bool {
	return len(id) == 24 && strings.HasPrefix(id, ids.ObjectAccessKey.Prefix()+"-") && ids.WellFormed(id)
}

// CreateKeyInput names the key to make. Lifetime zero is a persistent key;
// otherwise the key expires after it, at most MaxEphemeralKeyLife.
type CreateKeyInput struct {
	WorkspaceID string
	CreatedBy   string
	Name        string
	Permission  string
	Lifetime    time.Duration
	SignIn      core.SignIn
}

// CreatedKey is a new key with the one-time secret.
type CreatedKey struct {
	Key store.ObjectAccessKey
	// Secret is shown once, here, and never returned by a read.
	Secret string
}

// Create makes a key. OpenBao is written first and rolled back if the row cannot
// be inserted, so a failed create leaves no secret behind.
func (a *AccessKeys) Create(ctx context.Context, in CreateKeyInput) (CreatedKey, error) {
	if !a.Enabled() {
		return CreatedKey{}, ErrSecretStoreUnavailable
	}
	if in.WorkspaceID == "" || in.CreatedBy == "" {
		return CreatedKey{}, fmt.Errorf("%w: a workspace and a creator are required", ErrAccessKeyInvalidInput)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 200 || strings.ContainsFunc(name, func(r rune) bool { return r < ' ' || r == 0x7f }) {
		return CreatedKey{}, fmt.Errorf("%w: a name is 1 to 200 characters with no control character", ErrAccessKeyInvalidInput)
	}
	if in.Permission != KeyPermissionRead && in.Permission != KeyPermissionReadWrite {
		return CreatedKey{}, fmt.Errorf("%w: permission must be %q or %q", ErrAccessKeyInvalidInput, KeyPermissionRead, KeyPermissionReadWrite)
	}
	if in.Lifetime < 0 || in.Lifetime > MaxEphemeralKeyLife {
		return CreatedKey{}, fmt.Errorf("%w: an ephemeral key lives at most %s", ErrAccessKeyInvalidInput, MaxEphemeralKeyLife)
	}
	secret, err := newKeySecret()
	if err != nil {
		return CreatedKey{}, err
	}
	row := store.ObjectAccessKey{
		ID: ids.New(ids.ObjectAccessKey), WorkspaceID: in.WorkspaceID, CreatedBy: in.CreatedBy,
		Name: name, Permission: in.Permission, CreatedSignIn: in.SignIn,
	}
	if in.Lifetime > 0 {
		exp := a.now().Add(in.Lifetime).UTC()
		row.ExpiresAt = &exp
	}
	tctx := a.tenantCtx(ctx, in.WorkspaceID)
	if err := a.secrets.Put(tctx, secretPath(row.ID), map[string]string{"secret": secret}); err != nil {
		log.Printf("objectstorage: write access key secret: %v", err)
		return CreatedKey{}, ErrSecretStoreUnavailable
	}
	created, err := a.repo.CreateObjectAccessKey(ctx, row, MaxPersistentAccessKeys, MaxEphemeralAccessKeys)
	if err != nil {
		// Roll the secret back whatever the cause; a failure to do so leaves an
		// unreferenced secret under a never-used id, which nothing reads.
		if delErr := a.secrets.Delete(tctx, secretPath(row.ID)); delErr != nil {
			log.Printf("objectstorage: roll back access key secret %s: %v", row.ID, delErr)
		}
		if errors.Is(err, store.ErrObjectAccessKeyQuota) {
			return CreatedKey{}, ErrAccessKeyLimit
		}
		return CreatedKey{}, err
	}
	return CreatedKey{Key: created, Secret: secret}, nil
}

// List returns a workspace's keys, revoked and expired ones included until the
// sweep deletes them. No secret is ever returned.
func (a *AccessKeys) List(ctx context.Context, workspaceID string) ([]store.ObjectAccessKey, error) {
	if a == nil || a.repo == nil {
		return nil, ErrSecretStoreUnavailable
	}
	return a.repo.ListObjectAccessKeys(ctx, workspaceID)
}

// Revoke ends a key of the workspace. The row is marked first and the secret
// deleted after, so a failure between the two leaves a key that is already
// unusable. A key that is missing or in another workspace is ErrAccessKeyNotFound.
func (a *AccessKeys) Revoke(ctx context.Context, workspaceID, id string) error {
	if !a.Enabled() {
		return ErrSecretStoreUnavailable
	}
	k, err := a.repo.RevokeObjectAccessKey(ctx, workspaceID, id)
	if errors.Is(err, store.ErrNotFound) {
		return ErrAccessKeyNotFound
	}
	if err != nil {
		return err
	}
	a.forget(k.ID)
	if err := a.secrets.Delete(a.tenantCtx(ctx, k.WorkspaceID), secretPath(k.ID)); err != nil {
		// The key is unusable already (the row is read first on every request);
		// the sweep deletes the row and retries the secret seven days on.
		log.Printf("objectstorage: delete revoked access key secret %s: %v", k.ID, err)
	}
	return nil
}

// forget drops a key from this replica's caches: a revoke here takes effect on
// the next request, on another replica within the row cache's 30 seconds.
func (a *AccessKeys) forget(id string) {
	a.rows.Delete(id)
	a.secret.Delete(id)
	a.touched.Delete(id)
}

// Dispose deletes the keys a person made, in one workspace or (workspaceID "")
// in all of them, with their secrets, and returns the ids it deleted. A member's
// removal and an account's deletion call it; the rows are deleted and not
// revoked, so re-adding the creator never revives a key.
func (a *AccessKeys) Dispose(ctx context.Context, createdBy, workspaceID string) ([]string, error) {
	if a == nil || a.repo == nil {
		return nil, nil
	}
	keys, err := a.repo.DeleteObjectAccessKeysOf(ctx, createdBy, workspaceID)
	if err != nil {
		return nil, err
	}
	deleted := make([]string, 0, len(keys))
	for _, k := range keys {
		deleted = append(deleted, k.ID)
	}
	return deleted, a.deleteSecrets(ctx, keys)
}

// PurgeWorkspace deletes every key of a workspace being deleted, with its
// secret. It satisfies workspaces.WorkspacePurger, which this package does not
// import, and runs after the tenant cascade like the objects' purge.
func (a *AccessKeys) PurgeWorkspace(ctx context.Context, workspaceID string) error {
	if a == nil || a.repo == nil {
		return nil
	}
	keys, err := a.repo.DeleteObjectAccessKeysOfWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	return a.deleteSecrets(ctx, keys)
}

// SweepEnded deletes the keys that were revoked or expired at least
// RetiredKeyRetention ago, with their secrets (the hourly sweep calls it). It
// returns how many rows it deleted.
func (a *AccessKeys) SweepEnded(ctx context.Context) (int, error) {
	if a == nil || a.repo == nil {
		return 0, nil
	}
	keys, err := a.repo.DeleteEndedObjectAccessKeys(ctx, a.now().Add(-RetiredKeyRetention))
	if err != nil {
		return 0, err
	}
	return len(keys), a.deleteSecrets(ctx, keys)
}

func (a *AccessKeys) deleteSecrets(ctx context.Context, keys []store.ObjectAccessKey) error {
	var firstErr error
	for _, k := range keys {
		a.forget(k.ID)
		if a.secrets == nil {
			continue
		}
		if err := a.secrets.Delete(a.tenantCtx(ctx, k.WorkspaceID), secretPath(k.ID)); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("delete the secret of access key %s: %w", k.ID, err)
		}
	}
	return firstErr
}

// Credential is what the S3 surface needs to verify one request: the live key
// and its secret.
type Credential struct {
	Key    store.ObjectAccessKey
	Secret string
}

// Authenticate resolves the access key id a request names. The row is read first
// (cached 30 seconds, a miss too), so a revocation reaches every replica within
// that window and a secret never has to be refetched; the secret is read from
// OpenBao at most once per SecretCacheTTL on each replica, whatever is sent. An
// unknown, revoked or expired key is ErrInvalidAccessKey, an OpenBao failure
// ErrSecretStoreUnavailable, and a live key whose secret is gone is revoked and
// marked broken.
func (a *AccessKeys) Authenticate(ctx context.Context, keyID string) (Credential, error) {
	if !a.Enabled() {
		return Credential{}, ErrSecretStoreUnavailable
	}
	if !KeyShape(keyID) {
		return Credential{}, ErrInvalidAccessKey
	}
	look, err := a.rows.Load(ctx, keyID, AccessKeyRowCacheTTL, func(ctx context.Context) (keyLookup, error) {
		k, err := a.repo.GetObjectAccessKey(ctx, keyID)
		if errors.Is(err, store.ErrNotFound) {
			return keyLookup{}, nil
		}
		return keyLookup{key: k, found: err == nil}, err
	})
	if err != nil {
		return Credential{}, err
	}
	if !look.found || !look.key.Live(a.now()) {
		return Credential{}, ErrInvalidAccessKey
	}
	secret, err := a.secret.Load(ctx, keyID, AccessKeySecretCacheTTL, func(ctx context.Context) (string, error) {
		got, err := a.secrets.Get(a.tenantCtx(ctx, look.key.WorkspaceID), secretPath(keyID))
		if err != nil {
			return "", fmt.Errorf("%w: %w", ErrSecretStoreUnavailable, err)
		}
		return got["secret"], nil
	})
	if err != nil {
		if errors.Is(err, ErrSecretStoreUnavailable) {
			log.Printf("objectstorage: read access key secret %s: %v", keyID, err)
			return Credential{}, ErrSecretStoreUnavailable
		}
		return Credential{}, err
	}
	if secret == "" {
		// OpenBao answers an empty map for a path never written: the secret is
		// gone (restored from an older snapshot, say). The key cannot sign
		// again, so it is revoked and listed as broken.
		a.secret.Delete(keyID)
		if err := a.repo.MarkObjectAccessKeyBroken(context.WithoutCancel(ctx), keyID); err != nil {
			log.Printf("objectstorage: mark access key %s broken: %v", keyID, err)
		}
		a.rows.Delete(keyID)
		return Credential{}, ErrInvalidAccessKey
	}
	a.touch(ctx, look.key)
	return Credential{Key: look.key, Secret: secret}, nil
}

// Resolve is the live key a key id names, from the row cache, without its secret.
// A CORS preflight carries no signature, so it can only be routed by the key id of
// a presigned URL; the answer is a hint that routes it, never an authentication.
// An unknown, revoked or expired key is ErrInvalidAccessKey.
func (a *AccessKeys) Resolve(ctx context.Context, keyID string) (store.ObjectAccessKey, error) {
	if !a.Enabled() || !KeyShape(keyID) {
		return store.ObjectAccessKey{}, ErrInvalidAccessKey
	}
	look, err := a.rows.Load(ctx, keyID, AccessKeyRowCacheTTL, func(ctx context.Context) (keyLookup, error) {
		k, err := a.repo.GetObjectAccessKey(ctx, keyID)
		if errors.Is(err, store.ErrNotFound) {
			return keyLookup{}, nil
		}
		return keyLookup{key: k, found: err == nil}, err
	})
	if err != nil {
		return store.ObjectAccessKey{}, err
	}
	if !look.found || !look.key.Live(a.now()) {
		return store.ObjectAccessKey{}, ErrInvalidAccessKey
	}
	return look.key, nil
}

// ErrNotCached is a key that AuthenticateCached could not answer from memory.
var ErrNotCached = errors.New("the access key is not in this replica's caches")

// AuthenticateCached is Authenticate from this replica's caches alone: it never
// reads the database or OpenBao. The S3 surface uses it for a client address that
// has failed too often, so a flood of unknown key ids from one address stops
// reaching either, while a key it is already serving keeps working. A key id the
// caches do not hold is ErrNotCached.
func (a *AccessKeys) AuthenticateCached(keyID string) (Credential, error) {
	if !a.Enabled() || !KeyShape(keyID) {
		return Credential{}, ErrInvalidAccessKey
	}
	look, ok := a.rows.Get(keyID)
	if !ok {
		return Credential{}, ErrNotCached
	}
	if !look.found || !look.key.Live(a.now()) {
		return Credential{}, ErrInvalidAccessKey
	}
	secret, ok := a.secret.Get(keyID)
	if !ok || secret == "" {
		return Credential{}, ErrNotCached
	}
	return Credential{Key: look.key, Secret: secret}, nil
}

// touch records the key's last use, at most once per KeyTouchEvery on a replica.
func (a *AccessKeys) touch(ctx context.Context, k store.ObjectAccessKey) {
	now := a.now()
	if last, ok := a.touched.Load(k.ID); ok && now.Sub(last.(time.Time)) < KeyTouchEvery {
		return
	}
	a.touched.Store(k.ID, now)
	if err := a.repo.TouchObjectAccessKey(context.WithoutCancel(ctx), k.ID, now); err != nil {
		log.Printf("objectstorage: record access key use %s: %v", k.ID, err)
	}
}

// keySecretAlphabet is what a secret is drawn from: 62 letters and digits, so
// the 40 characters hold about 238 bits and no client mangles one.
const keySecretAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// KeySecretLength is the length of an access key's secret, as AWS's are.
const KeySecretLength = 40

// newKeySecret draws a secret from the system's random source, rejecting the
// bytes that would bias the choice of a character.
func newKeySecret() (string, error) {
	const unbiased = 256 - 256%len(keySecretAlphabet)
	out := make([]byte, 0, KeySecretLength)
	buf := make([]byte, 64)
	for len(out) < KeySecretLength {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("draw an access key secret: %w", err)
		}
		for _, b := range buf {
			if int(b) >= unbiased {
				continue
			}
			out = append(out, keySecretAlphabet[int(b)%len(keySecretAlphabet)])
			if len(out) == KeySecretLength {
				break
			}
		}
	}
	return string(out), nil
}
