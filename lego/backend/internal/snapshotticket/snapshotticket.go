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

// Package snapshotticket signs the `snapshotKey` a disk-snapshot listing hands
// out and a restore hands back (docs/ADR082-persistent-disks.md D5).
//
// Render's own keys "expire after 24 hours", and the reason to sign rather than
// return a bare object path is what the key is FOR: it is the argument to an
// irreversible restore. An unsigned path would let a caller name any object in
// the bucket — including another tenant's — and would make the API's job
// "validate that this path is yours" on every request instead of "verify that
// we issued this". The signature binds the object to one disk, so a key from
// another workspace fails verification rather than being range-checked.
package snapshotticket

import (
	"time"

	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
)

var codec = hmacticket.New("disk snapshot key")

var (
	// ErrMalformed is returned when a key is structurally invalid or missing a
	// required claim.
	ErrMalformed = codec.Malformed()
	// ErrSignature is returned when a key's HMAC does not verify (tampered, or
	// minted with a different secret).
	ErrSignature = codec.Signature()
	// ErrExpired is returned when a key is past its 24-hour window.
	ErrExpired = codec.Expired()
)

// TTL is how long a snapshot key stays usable — Render's documented 24 hours.
// A listing is cheap to repeat, so a short window costs a client nothing and
// keeps a leaked key from being a standing restore capability.
const TTL = 24 * time.Hour

// Claims binds one stored object to one disk. It carries no credentials and no
// bucket: the restore Job already knows the store it reads from, and naming it
// here would put infrastructure detail in a value clients hold.
type Claims struct {
	DiskID    string `json:"dsk"` // the disk this snapshot belongs to
	Object    string `json:"obj"` // object key, relative to the store prefix
	IssuedAt  int64  `json:"iat"` // unix seconds
	ExpiresAt int64  `json:"exp"` // unix seconds
}

// Mint signs a key for one snapshot of one disk, expiring TTL after now.
func Mint(secret []byte, diskID, object string, now time.Time) (string, error) {
	return codec.Sign(secret, Claims{
		DiskID:    diskID,
		Object:    object,
		IssuedAt:  now.UTC().Unix(),
		ExpiresAt: now.UTC().Add(TTL).Unix(),
	})
}

// Verify checks the signature and time bounds and returns the claims. The
// caller must still confirm the claims name the disk it is acting on — a
// well-signed key for a DIFFERENT disk is valid, just not valid here.
func Verify(secret []byte, token string, now time.Time) (Claims, error) {
	var claims Claims
	if err := codec.Open(secret, token, &claims); err != nil {
		return Claims{}, err
	}
	if claims.DiskID == "" || claims.Object == "" || claims.ExpiresAt == 0 {
		return Claims{}, ErrMalformed
	}
	if err := codec.CheckBounds(now, claims.IssuedAt, claims.ExpiresAt); err != nil {
		return Claims{}, err
	}
	return claims, nil
}
