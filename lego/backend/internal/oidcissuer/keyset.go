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

package oidcissuer

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// KeyStore is the key set's home, the control-plane database (ADR116 D5).
// *store.PGStore implements it.
type KeyStore interface {
	OIDCSigningKeys(ctx context.Context) ([]store.OIDCSigningKey, error)
	BootstrapOIDCSigningKey(ctx context.Context, kid string, privateKey []byte) (bool, error)
	DeleteOIDCSigningKeys(ctx context.Context, kids []string) error
}

var (
	// ErrNotLoaded: this replica has not read the key set yet.
	ErrNotLoaded = errors.New("oidc issuer: the signing key set has not been loaded")
	// ErrStale: the last successful reload is older than MaxStaleness.
	ErrStale = errors.New("oidc issuer: the signing key set has not been reloaded for an hour")
	// ErrNoSigningKey: every stored key is still waiting to start signing.
	ErrNoSigningKey = errors.New("oidc issuer: no signing key is active")
)

// KeySet is one replica's cached view of the stored key set, reloaded every
// ReloadInterval. States are computed at each use from the cached rows and
// the clock, so a rotation takes effect at the same instant on every replica
// that has read its row.
type KeySet struct {
	Store KeyStore
	Now   func() time.Time // nil means time.Now

	mu       sync.RWMutex
	keys     []Key
	loadedAt time.Time
}

func (ks *KeySet) now() time.Time {
	if ks.Now != nil {
		return ks.Now()
	}
	return time.Now()
}

// Load reads the key set, inserting the first key when there is none
// (BootstrapOIDCSigningKey makes that single-writer across replicas), and
// deletes the keys that are no longer published. On any error the cached
// view is kept as it was.
func (ks *KeySet) Load(ctx context.Context) error {
	rows, err := ks.Store.OIDCSigningKeys(ctx)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		kid, der, err := NewKey()
		if err != nil {
			return err
		}
		if _, err := ks.Store.BootstrapOIDCSigningKey(ctx, kid, der); err != nil {
			return err
		}
		if rows, err = ks.Store.OIDCSigningKeys(ctx); err != nil {
			return err
		}
	}
	keys := make([]Key, 0, len(rows))
	for _, row := range rows {
		k, err := parseKey(row)
		if err != nil {
			return err
		}
		keys = append(keys, k)
	}
	now := ks.now()
	var kept []Key
	var removed []string
	for _, c := range Classify(keys, now) {
		if c.State == StateRemoved {
			removed = append(removed, c.ID)
			continue
		}
		kept = append(kept, c.Key)
	}
	ks.mu.Lock()
	ks.keys, ks.loadedAt = kept, now
	ks.mu.Unlock()
	// Pruning is housekeeping: a failed delete leaves a key nobody publishes
	// or signs with, and the next reload tries again.
	return ks.Store.DeleteOIDCSigningKeys(ctx, removed)
}

// Run reloads the key set every ReloadInterval until ctx ends, logging (never
// returning) a failed reload.
func (ks *KeySet) Run(ctx context.Context) {
	for {
		if err := ks.Load(ctx); err != nil && ctx.Err() == nil {
			log.Printf("oidc issuer: key set reload: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(ReloadInterval):
		}
	}
}

// published returns the keys a verifier needs at now: next, current and
// previous, in activation order. ok is false before the first load.
func (ks *KeySet) published(now time.Time) (keys []Key, ok bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	if ks.loadedAt.IsZero() {
		return nil, false
	}
	for _, c := range Classify(ks.keys, now) {
		if c.State != StateRemoved {
			keys = append(keys, c.Key)
		}
	}
	return keys, true
}

// signing returns the key that signs at now. It refuses on a view that was
// never loaded or is older than MaxStaleness: such a replica could have missed
// a rotation and would sign with a key its peers are about to stop
// publishing.
func (ks *KeySet) signing(now time.Time) (Key, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	switch {
	case ks.loadedAt.IsZero():
		return Key{}, ErrNotLoaded
	case now.Sub(ks.loadedAt) > MaxStaleness:
		return Key{}, ErrStale
	}
	for _, c := range Classify(ks.keys, now) {
		if c.State == StateCurrent {
			return c.Key, nil
		}
	}
	return Key{}, ErrNoSigningKey
}
