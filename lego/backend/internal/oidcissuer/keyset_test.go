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

package oidcissuer_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/oidcissuer"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// memStore is an in-memory KeyStore/RotationStore whose times come from the
// test's clock, as the database's now() does in production.
type memStore struct {
	mu      sync.Mutex
	now     func() time.Time
	rows    []store.OIDCSigningKey
	deleted []string
	fail    error
}

func (m *memStore) OIDCSigningKeys(context.Context) ([]store.OIDCSigningKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return nil, m.fail
	}
	out := slices.Clone(m.rows)
	slices.SortFunc(out, func(a, b store.OIDCSigningKey) int { return a.ActivatesAt.Compare(b.ActivatesAt) })
	return out, nil
}

func (m *memStore) BootstrapOIDCSigningKey(_ context.Context, kid string, der []byte) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.rows) > 0 {
		return false, nil
	}
	now := m.now()
	m.rows = append(m.rows, store.OIDCSigningKey{KID: kid, PrivateKey: der, CreatedAt: now, ActivatesAt: now})
	return true, nil
}

func (m *memStore) RotateOIDCSigningKey(_ context.Context, kid string, der []byte, lead time.Duration, replace bool) (store.OIDCSigningKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if replace {
		m.rows = nil
	}
	for _, r := range m.rows {
		if r.ActivatesAt.After(now) {
			return store.OIDCSigningKey{}, store.ErrOIDCRotationPending
		}
	}
	row := store.OIDCSigningKey{KID: kid, PrivateKey: der, CreatedAt: now, ActivatesAt: now.Add(lead)}
	m.rows = append(m.rows, row)
	return row, nil
}

func (m *memStore) DeleteOIDCSigningKeys(_ context.Context, kids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleted = append(m.deleted, kids...)
	m.rows = slices.DeleteFunc(m.rows, func(r store.OIDCSigningKey) bool { return slices.Contains(kids, r.KID) })
	return nil
}

func (m *memStore) kids() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, r := range m.rows {
		out = append(out, r.KID)
	}
	return out
}

// clock is a settable time source shared by the store, the key set and the
// verifier.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// fixture is an issuer over an in-memory store, loaded once.
type fixture struct {
	clock  *clock
	store  *memStore
	keys   *oidcissuer.KeySet
	issuer *oidcissuer.Issuer
}

func newFixture(t *testing.T, baseURL string, start time.Time) *fixture {
	t.Helper()
	c := &clock{t: start}
	st := &memStore{now: c.Now}
	keys := &oidcissuer.KeySet{Store: st, Now: c.Now}
	if err := keys.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	iss, err := oidcissuer.New(baseURL, keys)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &fixture{clock: c, store: st, keys: keys, issuer: iss}
}

// tokenParts decodes a compact JWS's header and payload without verifying it.
func tokenParts(t *testing.T, raw string) (header, payload map[string]any) {
	t.Helper()
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts", len(parts))
	}
	decode := func(s string) map[string]any {
		b, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	return decode(parts[0]), decode(parts[1])
}

const (
	wsA  = "tea-c0000000000000000000"
	wsB  = "tea-d98210cbbpdc73dcrkvg"
	srvA = "srv-c0000000000000000000"
	evmA = "evm-c0000000000000000000"
)

// TestSubjectIsRendersTemplate pins Render's sub claim and its refusals.
func TestSubjectIsRendersTemplate(t *testing.T) {
	for _, tc := range []struct {
		sub  oidcissuer.Subject
		want string
	}{
		{oidcissuer.Subject{Workspace: wsA, Service: srvA}, "workspace:" + wsA + ":environment:default:service:" + srvA},
		{oidcissuer.Subject{Workspace: wsA, Environment: "default", Service: srvA}, "workspace:" + wsA + ":environment:default:service:" + srvA},
		{oidcissuer.Subject{Workspace: wsA, Environment: evmA, Service: srvA}, "workspace:" + wsA + ":environment:" + evmA + ":service:" + srvA},
		// The storage spelling of an environment id is written as its public one.
		{oidcissuer.Subject{Workspace: wsA, Environment: "env-c0000000000000000000", Service: srvA}, "workspace:" + wsA + ":environment:" + evmA + ":service:" + srvA},
	} {
		if err := tc.sub.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v", tc.sub, err)
		}
		if got := tc.sub.String(); got != tc.want {
			t.Errorf("String(%+v) = %q, want %q", tc.sub, got, tc.want)
		}
	}
	for _, bad := range []oidcissuer.Subject{
		{Workspace: "tea-d0000000000000000001", Service: srvA}, // not an xid
		{Workspace: srvA, Service: srvA},
		{Workspace: wsA, Service: "web"},
		{Workspace: wsA, Environment: "production", Service: srvA},
		{Workspace: wsA, Environment: "evm-d0000000000000000001", Service: srvA},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want a refusal", bad)
		}
	}
}

// TestMintCarriesRendersClaims checks the claims and header byte-level: iss
// is the workspace's issuer, exp is iat + 3600, aud is one string, the header
// names the current key, and nothing else is claimed.
func TestMintCarriesRendersClaims(t *testing.T) {
	start := time.Date(2026, 11, 18, 9, 30, 15, 500_000_000, time.UTC)
	f := newFixture(t, "https://oidc.bex.co", start)
	tok, err := f.issuer.Mint(oidcissuer.Subject{Workspace: wsA, Service: srvA}, "sts.amazonaws.com")
	if err != nil {
		t.Fatal(err)
	}
	header, payload := tokenParts(t, tok.Raw)
	if header["alg"] != "ES256" || header["typ"] != "JWT" || header["kid"] != f.store.kids()[0] || len(header) != 3 {
		t.Errorf("header = %v", header)
	}
	want := map[string]any{
		"iss": "https://oidc.bex.co/" + wsA,
		"sub": "workspace:" + wsA + ":environment:default:service:" + srvA,
		"aud": "sts.amazonaws.com",
		"iat": float64(start.Truncate(time.Second).Unix()),
		"exp": float64(start.Truncate(time.Second).Unix() + 3600),
	}
	if len(payload) != len(want) {
		t.Errorf("payload has claims %v, want exactly %v", payload, want)
	}
	for k, v := range want {
		if payload[k] != v {
			t.Errorf("claim %s = %v, want %v", k, payload[k], v)
		}
	}
	if !tok.ExpiresAt.Equal(tok.IssuedAt.Add(time.Hour)) || tok.KeyID != header["kid"] {
		t.Errorf("Token = %+v", tok)
	}
	if _, err := f.issuer.Mint(oidcissuer.Subject{Workspace: wsA, Service: srvA}, " "); err == nil {
		t.Error("Mint with an empty audience succeeded")
	}
	if _, err := f.issuer.Mint(oidcissuer.Subject{Workspace: "tea-abc", Service: srvA}, "sts.amazonaws.com"); err == nil {
		t.Error("Mint for a malformed workspace succeeded")
	}
}

// TestSigningRefusesAStaleOrUnloadedKeySet is the staleness guard of ADR116
// D6: a replica that has not reloaded for MaxStaleness stops signing (it may
// have missed a rotation) but keeps publishing; one that never loaded does
// neither.
func TestSigningRefusesAStaleOrUnloadedKeySet(t *testing.T) {
	f := newFixture(t, "https://oidc.bex.co", time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC))
	sub := oidcissuer.Subject{Workspace: wsA, Service: srvA}
	f.clock.Advance(oidcissuer.MaxStaleness)
	if _, err := f.issuer.Mint(sub, "aud"); err != nil {
		t.Fatalf("Mint at exactly MaxStaleness = %v, want success", err)
	}
	f.clock.Advance(time.Second)
	if _, err := f.issuer.Mint(sub, "aud"); !errors.Is(err, oidcissuer.ErrStale) {
		t.Fatalf("Mint past MaxStaleness = %v, want ErrStale", err)
	}
	f.store.fail = errors.New("database down")
	if err := f.keys.Load(context.Background()); err == nil {
		t.Fatal("Load over a failing store succeeded")
	}
	if _, err := f.issuer.Mint(sub, "aud"); !errors.Is(err, oidcissuer.ErrStale) {
		t.Fatalf("a failed reload refreshed the view: Mint = %v", err)
	}
	f.store.fail = nil
	if err := f.keys.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.issuer.Mint(sub, "aud"); err != nil {
		t.Fatalf("Mint after a successful reload = %v", err)
	}

	never, err := oidcissuer.New("https://oidc.bex.co", &oidcissuer.KeySet{Store: &memStore{now: time.Now}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := never.Mint(sub, "aud"); !errors.Is(err, oidcissuer.ErrNotLoaded) {
		t.Fatalf("Mint before the first load = %v, want ErrNotLoaded", err)
	}
}

// TestClassifyFollowsTheRotationWindows is the state table of ADR116 D6 at
// the edges of each window.
func TestClassifyFollowsTheRotationWindows(t *testing.T) {
	t0 := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	old := oidcissuer.Key{ID: "old", ActivatesAt: t0}
	next := oidcissuer.Key{ID: "new", CreatedAt: t0.Add(time.Hour), ActivatesAt: t0.Add(2 * time.Hour)}
	states := func(now time.Time) string {
		var out []string
		for _, c := range oidcissuer.Classify([]oidcissuer.Key{next, old}, now) {
			out = append(out, c.ID+"="+string(c.State))
		}
		return strings.Join(out, " ")
	}
	for _, tc := range []struct {
		at   time.Duration
		want string
	}{
		{90 * time.Minute, "old=current new=next"},
		{2*time.Hour - time.Second, "old=current new=next"},
		{2 * time.Hour, "old=previous new=current"},
		{4*time.Hour - time.Second, "old=previous new=current"}, // successor + RetireAfter - 1s
		{4 * time.Hour, "old=removed new=current"},
	} {
		if got := states(t0.Add(tc.at)); got != tc.want {
			t.Errorf("at t0+%v: %s, want %s", tc.at, got, tc.want)
		}
	}
	if got := states(t0.Add(-time.Minute)); got != "old=next new=next" {
		t.Errorf("before any activation: %s", got)
	}
}

// TestLoadBootstrapsOnceAndPrunesRemovedKeys: an empty store gets one key, a
// second replica's load adds none, and a key past its retention is deleted
// from the store and dropped from the JWKS.
func TestLoadBootstrapsOnceAndPrunesRemovedKeys(t *testing.T) {
	f := newFixture(t, "https://oidc.bex.co", time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC))
	second := &oidcissuer.KeySet{Store: f.store, Now: f.clock.Now}
	if err := second.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := f.store.kids()
	if len(first) != 1 {
		t.Fatalf("two loads stored %d keys, want 1", len(first))
	}
	if _, err := oidcissuer.Rotate(context.Background(), f.store, f.clock.Now(), false, true); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(oidcissuer.PublishLead + oidcissuer.RetireAfter)
	if err := f.keys.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := f.store.kids(); len(got) != 1 || got[0] == first[0] {
		t.Fatalf("after retention the store holds %v; want only the rotated-in key", got)
	}
	if !slices.Equal(f.store.deleted, first) {
		t.Errorf("deleted %v, want %v", f.store.deleted, first)
	}
}

// TestRotateRefusesAPendingRotationAndPlansWithoutWriting covers the
// command's guard rails.
func TestRotateRefusesAPendingRotationAndPlansWithoutWriting(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, "https://oidc.bex.co", time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC))
	plan, err := oidcissuer.Rotate(ctx, f.store, f.clock.Now(), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.store.kids()) != 1 || plan.Applied || plan.NewKey == nil ||
		!plan.NewKey.ActivatesAt.Equal(f.clock.Now().Add(oidcissuer.PublishLead)) ||
		!plan.SupersededRemovedAt.Equal(plan.NewKey.ActivatesAt.Add(oidcissuer.RetireAfter)) ||
		len(plan.Keys) != 1 || plan.Keys[0].State != oidcissuer.StateCurrent {
		t.Fatalf("dry run = %+v (store %v)", plan, f.store.kids())
	}
	if _, err := oidcissuer.Rotate(ctx, f.store, f.clock.Now(), false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := oidcissuer.Rotate(ctx, f.store, f.clock.Now(), false, true); !errors.Is(err, store.ErrOIDCRotationPending) {
		t.Fatalf("second rotation = %v, want ErrOIDCRotationPending", err)
	}
	if got := len(f.store.kids()); got != 2 {
		t.Fatalf("store holds %d keys after a refused rotation, want 2", got)
	}
	emergency, err := oidcissuer.Rotate(ctx, f.store, f.clock.Now(), true, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.store.kids(); len(got) != 1 || got[0] != emergency.NewKey.KID || emergency.NewKey.State != oidcissuer.StateCurrent {
		t.Fatalf("emergency rotation left %v (report %+v)", got, emergency.NewKey)
	}
}
