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

package agentsession

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeNonceClaimer struct {
	seen map[string]bool
	err  error
}

func (f *fakeNonceClaimer) ClaimShellNonce(_ context.Context, nonce string, _ time.Time) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if f.seen[nonce] {
		return false, nil
	}
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	f.seen[nonce] = true
	return true, nil
}

// signedMintRequest builds a validly-signed internal mint request for the given
// body, so tests can replay the identical bytes an on-path attacker would.
func signedMintRequest(t *testing.T, secret, body []byte, now time.Time) func() *http.Request {
	t.Helper()
	ts, sig := Sign(secret, body, now)
	return func() *http.Request {
		hr := httptest.NewRequest(http.MethodPost, InternalMintPath, bytes.NewReader(body))
		hr.Header.Set(TimestampHeader, ts)
		hr.Header.Set(SignatureHeader, sig)
		return hr
	}
}

func TestServeSignedMintNonceReplayGuard(t *testing.T) {
	secret := []byte("gateway-shared-secret")
	now := time.Unix(1_700_000_000, 0)
	nonceOf := func(r MintRequest) string { return r.Nonce }

	newMint := func() (func(context.Context, MintRequest) (MintResponse, error), *int) {
		calls := 0
		return func(context.Context, MintRequest) (MintResponse, error) {
			calls++
			return MintResponse{Token: "tok"}, nil
		}, &calls
	}

	t.Run("replay of identical signed request is rejected", func(t *testing.T) {
		claimer := &fakeNonceClaimer{}
		mint, calls := newMint()
		body, _ := json.Marshal(MintRequest{SessionID: "ags-1", Nonce: newNonce()})
		makeReq := signedMintRequest(t, secret, body, now)

		rec1 := httptest.NewRecorder()
		serveSignedMint(rec1, makeReq(), secret, now, claimer, nonceOf, mint)
		if rec1.Code != http.StatusOK {
			t.Fatalf("first mint code = %d, want 200", rec1.Code)
		}

		rec2 := httptest.NewRecorder()
		serveSignedMint(rec2, makeReq(), secret, now, claimer, nonceOf, mint)
		if rec2.Code != http.StatusUnauthorized {
			t.Fatalf("replay code = %d, want 401", rec2.Code)
		}
		if *calls != 1 {
			t.Fatalf("replay reached the minter: calls = %d, want 1", *calls)
		}
	})

	t.Run("fresh nonce each call succeeds", func(t *testing.T) {
		claimer := &fakeNonceClaimer{}
		mint, calls := newMint()
		for i := 0; i < 3; i++ {
			body, _ := json.Marshal(MintRequest{SessionID: "ags-1", Nonce: newNonce()})
			rec := httptest.NewRecorder()
			serveSignedMint(rec, signedMintRequest(t, secret, body, now)(), secret, now, claimer, nonceOf, mint)
			if rec.Code != http.StatusOK {
				t.Fatalf("call %d code = %d, want 200", i, rec.Code)
			}
		}
		if *calls != 3 {
			t.Fatalf("calls = %d, want 3", *calls)
		}
	})

	t.Run("absent nonce is fail-closed when a claimer is wired", func(t *testing.T) {
		claimer := &fakeNonceClaimer{}
		mint, calls := newMint()
		body, _ := json.Marshal(MintRequest{SessionID: "ags-1"}) // no nonce
		rec := httptest.NewRecorder()
		serveSignedMint(rec, signedMintRequest(t, secret, body, now)(), secret, now, claimer, nonceOf, mint)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("absent-nonce code = %d, want 401", rec.Code)
		}
		if *calls != 0 {
			t.Fatalf("absent-nonce reached the minter: calls = %d", *calls)
		}
	})

	t.Run("store error is fail-closed", func(t *testing.T) {
		claimer := &fakeNonceClaimer{err: errors.New("db down")}
		mint, calls := newMint()
		body, _ := json.Marshal(MintRequest{SessionID: "ags-1", Nonce: newNonce()})
		rec := httptest.NewRecorder()
		serveSignedMint(rec, signedMintRequest(t, secret, body, now)(), secret, now, claimer, nonceOf, mint)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("store-error code = %d, want 401", rec.Code)
		}
		if *calls != 0 {
			t.Fatalf("store-error reached the minter: calls = %d", *calls)
		}
	})

	t.Run("nil claimer keeps prior behavior (no nonce enforcement)", func(t *testing.T) {
		mint, calls := newMint()
		body, _ := json.Marshal(MintRequest{SessionID: "ags-1", Nonce: newNonce()})
		makeReq := signedMintRequest(t, secret, body, now)

		rec1 := httptest.NewRecorder()
		serveSignedMint(rec1, makeReq(), secret, now, nil, nonceOf, mint)
		rec2 := httptest.NewRecorder()
		serveSignedMint(rec2, makeReq(), secret, now, nil, nonceOf, mint)
		if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
			t.Fatalf("nil-claimer codes = %d/%d, want 200/200", rec1.Code, rec2.Code)
		}
		if *calls != 2 {
			t.Fatalf("nil-claimer calls = %d, want 2", *calls)
		}
	})
}
