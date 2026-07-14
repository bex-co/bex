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
	if !Verify(secret, msgID, "1614265330", []byte(payload), want) {
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

	if !Verify(secret, "evt-x", "1750000000", body, sig) {
		t.Fatal("Verify rejected an untampered delivery")
	}
	if Verify(secret, "evt-x", "1750000000", []byte(`{"type":"deploy_started","data":{"id":"evt-y"}}`), sig) {
		t.Error("Verify accepted an altered body")
	}
	if Verify(secret, "evt-other", "1750000000", body, sig) {
		t.Error("Verify accepted an altered message id")
	}
	if Verify(secret, "evt-x", "1750000001", body, sig) {
		t.Error("Verify accepted an altered timestamp")
	}
	otherSecret, _ := NewSecret()
	if Verify(otherSecret, "evt-x", "1750000000", body, sig) {
		t.Error("Verify accepted a signature from a different secret")
	}
	if Verify(secret, "evt-x", "not-a-number", body, sig) {
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
	if !Verify(secret, "evt-x", "1750000000", body, header) {
		t.Error("Verify did not find the valid signature in a multi-signature header")
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
