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

// Package fakekeys is an in-memory objectstorage.AccessKeyRepository and a
// core.SecretKV, for tests of the S3 surface and its adapters that need access
// keys without Postgres or OpenBao.
package fakekeys

import (
	"context"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Repo is the in-memory key table.
type Repo struct {
	mu   sync.Mutex
	Keys map[string]store.ObjectAccessKey
}

// NewRepo returns an empty table.
func NewRepo() *Repo { return &Repo{Keys: map[string]store.ObjectAccessKey{}} }

func (m *Repo) CreateObjectAccessKey(_ context.Context, in store.ObjectAccessKey, _, _ int) (store.ObjectAccessKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	in.CreatedAt = time.Now()
	m.Keys[in.ID] = in
	return in, nil
}

func (m *Repo) GetObjectAccessKey(_ context.Context, id string) (store.ObjectAccessKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if k, ok := m.Keys[id]; ok {
		return k, nil
	}
	return store.ObjectAccessKey{}, store.ErrNotFound
}

func (m *Repo) ListObjectAccessKeys(_ context.Context, ws string) ([]store.ObjectAccessKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.ObjectAccessKey
	for _, k := range m.Keys {
		if k.WorkspaceID == ws {
			out = append(out, k)
		}
	}
	return out, nil
}

func (m *Repo) RevokeObjectAccessKey(_ context.Context, ws, id string) (store.ObjectAccessKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.Keys[id]
	if !ok || k.WorkspaceID != ws || k.RevokedAt != nil {
		return store.ObjectAccessKey{}, store.ErrNotFound
	}
	now := time.Now()
	k.RevokedAt = &now
	m.Keys[id] = k
	return k, nil
}

func (m *Repo) MarkObjectAccessKeyBroken(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if k, ok := m.Keys[id]; ok {
		now := time.Now()
		k.RevokedAt, k.Broken = &now, true
		m.Keys[id] = k
	}
	return nil
}

func (m *Repo) TouchObjectAccessKey(context.Context, string, time.Time) error { return nil }

func (m *Repo) DeleteObjectAccessKey(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Keys, id)
	return nil
}

func (m *Repo) DeleteObjectAccessKeysOf(_ context.Context, by, ws string) ([]store.ObjectAccessKey, error) {
	return m.deleteWhere(func(k store.ObjectAccessKey) bool { return k.CreatedBy == by && (ws == "" || k.WorkspaceID == ws) }), nil
}

func (m *Repo) DeleteObjectAccessKeysOfWorkspace(_ context.Context, ws string) ([]store.ObjectAccessKey, error) {
	return m.deleteWhere(func(k store.ObjectAccessKey) bool { return k.WorkspaceID == ws }), nil
}

func (m *Repo) DeleteEndedObjectAccessKeys(_ context.Context, cutoff time.Time) ([]store.ObjectAccessKey, error) {
	return m.deleteWhere(func(k store.ObjectAccessKey) bool {
		return (k.RevokedAt != nil && k.RevokedAt.Before(cutoff)) || (k.ExpiresAt != nil && k.ExpiresAt.Before(cutoff))
	}), nil
}

func (m *Repo) deleteWhere(match func(store.ObjectAccessKey) bool) []store.ObjectAccessKey {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.ObjectAccessKey
	for id, k := range m.Keys {
		if match(k) {
			out = append(out, k)
			delete(m.Keys, id)
		}
	}
	return out
}

// Set replaces a stored key, so a test can age or alter one.
func (m *Repo) Set(k store.ObjectAccessKey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Keys[k.ID] = k
}

// KV is an in-memory core.SecretKV keyed by the context's tenant and the path.
type KV struct {
	mu   sync.Mutex
	Data map[string]map[string]string
	// GetErr, when set, fails every Get: OpenBao down.
	GetErr error
	gets   int
}

// NewKV returns an empty store.
func NewKV() *KV { return &KV{Data: map[string]map[string]string{}} }

func slot(ctx context.Context, path string) string {
	return secrets.TenantFromContext(ctx) + "|" + path
}

func (k *KV) Get(ctx context.Context, p string) (map[string]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.gets++
	if k.GetErr != nil {
		return nil, k.GetErr
	}
	return k.Data[slot(ctx, p)], nil
}

func (k *KV) Put(ctx context.Context, p string, d map[string]string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.Data[slot(ctx, p)] = d
	return nil
}

func (k *KV) Delete(ctx context.Context, p string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.Data, slot(ctx, p))
	return nil
}

func (k *KV) List(context.Context, string) ([]string, error) { return nil, nil }

// Gets is how many times OpenBao was read.
func (k *KV) Gets() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.gets
}

// Len is how many secrets are stored.
func (k *KV) Len() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.Data)
}
