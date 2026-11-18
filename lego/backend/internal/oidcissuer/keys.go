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

// Package oidcissuer is bex's Managed OIDC identity provider
// (docs/ADR116-managed-oidc.md): one issuer per workspace at
// <BEX_OIDC_ISSUER_URL>/<tea-…>, Render's discovery document and JWKS, an
// ES256 key set shared by every workspace, its rotation, and the signer that
// mints Render's claims. Delivering tokens into services is w2/m477.
package oidcissuer

import (
	"cmp"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The numbers of ADR116 D6.
const (
	// TokenLifetime is exp - iat, Render's 60 minutes.
	TokenLifetime = time.Hour
	// PublishLead is how long a rotated-in key is published before it signs.
	PublishLead = time.Hour
	// RetireAfter is how long a superseded key stays published after its
	// successor starts signing: the token lifetime plus an hour of margin.
	RetireAfter = TokenLifetime + time.Hour
	// ReloadInterval is how often each replica re-reads the key set.
	ReloadInterval = time.Minute
	// MaxStaleness is how long a replica may sign without a successful reload:
	// past it, it could have missed a rotation.
	MaxStaleness = PublishLead
)

// Key is one signing key of the set.
type Key struct {
	ID          string // RFC 7638 thumbprint of the public key
	Private     *ecdsa.PrivateKey
	CreatedAt   time.Time // published from here on
	ActivatesAt time.Time // signs from here on, until a newer key does
	public      jwk       // the JWKS entry, computed once at load
}

// jwk is one public key with Render's members in Render's order.
type jwk struct {
	Use string `json:"use"`
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Crv string `json:"crv"`
	Alg string `json:"alg"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// NewKey generates a P-256 key and returns its kid and PKCS#8 DER encoding,
// the two values the store keeps.
func NewKey() (kid string, der []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", nil, err
	}
	der, err = x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", nil, err
	}
	pub, err := publicJWK(&priv.PublicKey)
	return pub.Kid, der, err
}

// parseKey turns a stored row back into a Key, refusing anything but a P-256
// ECDSA key whose thumbprint is the row's kid.
func parseKey(row store.OIDCSigningKey) (Key, error) {
	parsed, err := x509.ParsePKCS8PrivateKey(row.PrivateKey)
	if err != nil {
		return Key{}, fmt.Errorf("oidc signing key %s: %w", row.KID, err)
	}
	priv, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || priv.Curve != elliptic.P256() {
		return Key{}, fmt.Errorf("oidc signing key %s: not an ECDSA P-256 key", row.KID)
	}
	pub, err := publicJWK(&priv.PublicKey)
	if err != nil {
		return Key{}, err
	}
	if pub.Kid != row.KID {
		return Key{}, fmt.Errorf("oidc signing key %s: the key's thumbprint is %s", row.KID, pub.Kid)
	}
	return Key{ID: pub.Kid, Private: priv, CreatedAt: row.CreatedAt, ActivatesAt: row.ActivatesAt, public: pub}, nil
}

// publicJWK is a P-256 public key's JWKS entry. x and y are the 32-byte
// big-endian coordinates JWK requires (RFC 7518 §6.2.1.2), and the kid is the
// RFC 7638 thumbprint: the SHA-256 of the required members in lexicographic
// order with no whitespace, base64url without padding.
func publicJWK(key *ecdsa.PublicKey) (jwk, error) {
	point, err := key.Bytes() // 0x04 || X || Y
	if err != nil {
		return jwk{}, err
	}
	if len(point) != 65 {
		return jwk{}, errors.New("oidc signing key: not a P-256 point")
	}
	enc := base64.RawURLEncoding
	x, y := enc.EncodeToString(point[1:33]), enc.EncodeToString(point[33:])
	sum := sha256.Sum256([]byte(`{"crv":"P-256","kty":"EC","x":"` + x + `","y":"` + y + `"}`))
	return jwk{Use: "sig", Kty: "EC", Kid: enc.EncodeToString(sum[:]), Crv: "P-256", Alg: "ES256", X: x, Y: y}, nil
}

// State is a key's place in the rotation of ADR116 D6.
type State string

const (
	StateNext     State = "next"     // published, not signing yet
	StateCurrent  State = "current"  // published and signing
	StatePrevious State = "previous" // superseded, still published
	StateRemoved  State = "removed"  // superseded long enough ago to delete
)

// Classify returns each key's state at now, in activation order. The state is
// a function of the rows and the time alone, so replicas agree on it without
// coordinating: the latest key whose activation has passed signs, keys not
// active yet are next, and a superseded key is previous until RetireAfter has
// passed since its successor started signing.
func Classify(keys []Key, now time.Time) []ClassifiedKey {
	sorted := slices.Clone(keys)
	slices.SortFunc(sorted, func(a, b Key) int {
		if c := a.ActivatesAt.Compare(b.ActivatesAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	current := -1
	for i, k := range sorted {
		if !k.ActivatesAt.After(now) {
			current = i
		}
	}
	out := make([]ClassifiedKey, len(sorted))
	for i, k := range sorted {
		state := StateNext
		switch {
		case i == current:
			state = StateCurrent
		case i < current:
			state = StatePrevious
			if !now.Before(sorted[i+1].ActivatesAt.Add(RetireAfter)) {
				state = StateRemoved
			}
		}
		out[i] = ClassifiedKey{Key: k, State: state}
	}
	return out
}

// ClassifiedKey is a key with its state at one instant.
type ClassifiedKey struct {
	Key
	State State
}
