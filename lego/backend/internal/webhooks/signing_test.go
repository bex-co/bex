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

package webhooks

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestSignMatchesStandardWebhooksReferenceVector pins the signature scheme to
// the Standard Webhooks specification's own published test vector — if this
// fails, receivers verifying with any standard-webhooks library reject every
// bex delivery.
func TestSignMatchesStandardWebhooksReferenceVector(t *testing.T) {
	const (
		secret  = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
		msgID   = "msg_p5jXN8AQM9LWM0D4loKWxJek"
		payload = `{"test": 2432232314}`
		want    = "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="
	)
	at := time.Unix(1614265330, 0)
	if got := Sign(secret, msgID, at, []byte(payload)); got != want {
		t.Errorf("Sign = %q, want %q", got, want)
	}
	// verifyAt pins the receiver clock to the vector's own instant — the
	// vector's timestamp is from 2021 and Verify now (correctly) rejects it as
	// stale, which is the point of the tolerance window, not a broken vector.
	if !verifyAt(at, secret, msgID, "1614265330", []byte(payload), want) {
		t.Error("Verify rejected the reference signature")
	}
}

// TestVerifyRejectsTampering: any change to body, id, timestamp, or secret
// must fail verification.
func TestVerifyRejectsTampering(t *testing.T) {
	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	at := time.Unix(1750000000, 0)
	body := []byte(`{"type":"deploy_started","data":{"id":"evt-x"}}`)
	sig := Sign(secret, "evt-x", at, body)

	if !verifyAt(at, secret, "evt-x", "1750000000", body, sig) {
		t.Fatal("Verify rejected an untampered delivery")
	}
	if verifyAt(at, secret, "evt-x", "1750000000", []byte(`{"type":"deploy_started","data":{"id":"evt-y"}}`), sig) {
		t.Error("Verify accepted an altered body")
	}
	if verifyAt(at, secret, "evt-other", "1750000000", body, sig) {
		t.Error("Verify accepted an altered message id")
	}
	if verifyAt(at, secret, "evt-x", "1750000001", body, sig) {
		t.Error("Verify accepted an altered timestamp")
	}
	otherSecret, _ := NewSecret()
	if verifyAt(at, otherSecret, "evt-x", "1750000000", body, sig) {
		t.Error("Verify accepted a signature from a different secret")
	}
	if verifyAt(at, secret, "evt-x", "not-a-number", body, sig) {
		t.Error("Verify accepted a malformed timestamp")
	}
}

// TestVerifyAcceptsMultiSignatureHeader: Standard Webhooks allows several
// space-delimited signatures (key rotation); ours must be found among them.
func TestVerifyAcceptsMultiSignatureHeader(t *testing.T) {
	secret, _ := NewSecret()
	at := time.Unix(1750000000, 0)
	body := []byte(`{}`)
	sig := Sign(secret, "evt-x", at, body)
	header := "v1,bm90LXRoZS1zaWduYXR1cmU= " + sig
	if !verifyAt(at, secret, "evt-x", "1750000000", body, header) {
		t.Error("Verify did not find the valid signature in a multi-signature header")
	}
}

// TestVerifyEnforcesTheStandardWebhooksTimestampTolerance: a correct signature
// proves authenticity, not freshness. Without a tolerance window one captured
// delivery replays forever, which is exactly what the spec's tolerance check
// exists to stop. Both directions are bounded — a far-future timestamp is as
// wrong as a stale one.
func TestVerifyEnforcesTheStandardWebhooksTimestampTolerance(t *testing.T) {
	secret, _ := NewSecret()
	body := []byte(`{"type":"deploy_started"}`)
	now := time.Unix(1750000000, 0)

	sign := func(at time.Time) (string, string) {
		return strconv.FormatInt(at.Unix(), 10), Sign(secret, "evt-x", at, body)
	}

	for _, tc := range []struct {
		name  string
		at    time.Time
		valid bool
	}{
		{"fresh", now, true},
		{"just inside the window", now.Add(-VerifyTolerance + time.Second), true},
		{"just outside the window", now.Add(-VerifyTolerance - time.Second), false},
		{"captured an hour ago", now.Add(-time.Hour), false},
		{"captured months ago", now.Add(-90 * 24 * time.Hour), false},
		{"far future", now.Add(VerifyTolerance + time.Second), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, sig := sign(tc.at)
			// Every case is a CORRECTLY signed delivery; only its age differs.
			if got := verifyAt(now, secret, "evt-x", ts, body, sig); got != tc.valid {
				t.Errorf("verifyAt(age %s) = %v, want %v", now.Sub(tc.at), got, tc.valid)
			}
		})
	}
}

// TestVerifyUsesTheRealClock: the exported Verify must apply the window against
// time.Now(), not leave it to callers — a fresh delivery passes, a stale one
// signed with the same secret does not.
func TestVerifyUsesTheRealClock(t *testing.T) {
	secret, _ := NewSecret()
	body := []byte(`{}`)

	fresh := time.Now()
	if !Verify(secret, "evt-x", strconv.FormatInt(fresh.Unix(), 10), body, Sign(secret, "evt-x", fresh, body)) {
		t.Error("Verify rejected a fresh delivery")
	}
	stale := time.Now().Add(-VerifyTolerance - time.Minute)
	if Verify(secret, "evt-x", strconv.FormatInt(stale.Unix(), 10), body, Sign(secret, "evt-x", stale, body)) {
		t.Error("Verify accepted a correctly signed but stale delivery")
	}
}

// TestNewSecretShape: the whsec_ serialization is what standard-webhooks
// libraries parse; two mints must differ.
func TestNewSecretShape(t *testing.T) {
	a, err := NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	b, _ := NewSecret()
	if !strings.HasPrefix(a, "whsec_") {
		t.Errorf("secret %q lacks the whsec_ prefix", a)
	}
	if a == b {
		t.Error("two minted secrets are identical")
	}
	if len(signingKey(a)) != secretBytes {
		t.Errorf("signing key length = %d, want %d", len(signingKey(a)), secretBytes)
	}
}
