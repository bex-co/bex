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

package sshgateway

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubNonceStore struct {
	claimed map[string]bool
	err     error
}

func (s *stubNonceStore) ClaimShellNonce(_ context.Context, nonce string, _ time.Time) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if s.claimed[nonce] {
		return false, nil
	}
	if s.claimed == nil {
		s.claimed = map[string]bool{}
	}
	s.claimed[nonce] = true
	return true, nil
}

func TestNonceGuardConsume(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	exp := now.Add(time.Minute)

	t.Run("single use across replays", func(t *testing.T) {
		g := &NonceGuard{Store: &stubNonceStore{}}
		if !g.Consume(context.Background(), "n1", exp, now) {
			t.Fatal("first Consume should win")
		}
		if g.Consume(context.Background(), "n1", exp, now) {
			t.Fatal("replay of a claimed nonce must lose")
		}
		if g.Consume(context.Background(), "", exp, now) {
			t.Fatal("empty nonce must lose")
		}
	})

	t.Run("store error rolls back the in-memory mark so a retry can win", func(t *testing.T) {
		store := &stubNonceStore{err: errors.New("db down")}
		g := &NonceGuard{Store: store}

		// Transient store error: this attempt fails closed.
		if g.Consume(context.Background(), "n2", exp, now) {
			t.Fatal("Consume must fail closed on a store error")
		}
		// Store recovers. Because the in-memory mark was rolled back, the same
		// ticket's nonce can now be claimed — it was never actually consumed.
		store.err = nil
		if !g.Consume(context.Background(), "n2", exp, now) {
			t.Fatal("after the store recovers, the un-consumed nonce must be claimable")
		}
		// And it is now genuinely single-use.
		if g.Consume(context.Background(), "n2", exp, now) {
			t.Fatal("nonce must be single-use once durably claimed")
		}
	})

	t.Run("genuine durable conflict stays rejected", func(t *testing.T) {
		// The nonce is already claimed durably (e.g. by another replica).
		store := &stubNonceStore{claimed: map[string]bool{"n3": true}}
		g := &NonceGuard{Store: store}
		if g.Consume(context.Background(), "n3", exp, now) {
			t.Fatal("a nonce already claimed durably must lose")
		}
	})
}
