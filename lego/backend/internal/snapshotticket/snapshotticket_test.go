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

package snapshotticket

import (
	"errors"
	"testing"
	"time"
)

var secret = []byte("disk-snapshot-secret")

func TestMintedKeyRoundTrips(t *testing.T) {
	now := time.Now().UTC()
	key, err := Mint(secret, "dsk-1", "tea-1/dsk-1/2026-08-23T02:00:00Z.tar.gz.age", now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	claims, err := Verify(secret, key, now)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.DiskID != "dsk-1" || claims.Object != "tea-1/dsk-1/2026-08-23T02:00:00Z.tar.gz.age" {
		t.Fatalf("claims = %+v, want the disk and object it was minted for", claims)
	}
}

// Render's keys expire after 24 hours. A key that outlived its window is a
// standing capability to run an irreversible restore, so the boundary matters.
func TestKeyExpiresAfterTwentyFourHours(t *testing.T) {
	now := time.Now().UTC()
	key, err := Mint(secret, "dsk-1", "obj", now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if _, err := Verify(secret, key, now.Add(23*time.Hour)); err != nil {
		t.Fatalf("Verify at 23h: %v, want still valid", err)
	}
	if _, err := Verify(secret, key, now.Add(25*time.Hour)); !errors.Is(err, ErrExpired) {
		t.Fatalf("Verify at 25h = %v, want ErrExpired", err)
	}
}

// The signature is what makes the key unforgeable; without it a caller could
// name any object in the bucket, including another tenant's.
func TestTamperedOrForeignKeysAreRefused(t *testing.T) {
	now := time.Now().UTC()
	key, err := Mint(secret, "dsk-1", "tea-1/dsk-1/snap.tar.gz.age", now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	if _, err := Verify([]byte("a-different-secret"), key, now); !errors.Is(err, ErrSignature) {
		t.Fatalf("Verify with the wrong secret = %v, want ErrSignature", err)
	}
	tampered := key[:len(key)-2] + "xy"
	if _, err := Verify(secret, tampered, now); err == nil {
		t.Fatal("Verify accepted a tampered key")
	}
	for _, garbage := range []string{"", "not-a-ticket", "a.b.c"} {
		if _, err := Verify(secret, garbage, now); err == nil {
			t.Fatalf("Verify accepted %q", garbage)
		}
	}
}

// A key for another disk is well-signed but must not be usable here — the
// caller has to compare the claim, and this pins that the claim survives so it
// can be compared.
func TestClaimsCarryTheDiskSoACrossDiskKeyIsDetectable(t *testing.T) {
	now := time.Now().UTC()
	key, err := Mint(secret, "dsk-other", "tea-2/dsk-other/snap.tar.gz.age", now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	claims, err := Verify(secret, key, now)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.DiskID == "dsk-1" {
		t.Fatal("a key minted for another disk claims to be this one")
	}
}
