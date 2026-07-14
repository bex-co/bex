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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// signing.go implements the Standard Webhooks signature scheme
// (https://www.standardwebhooks.com/, the scheme Render's outbound webhooks
// use — verified live render.com/docs/webhooks 2026-07-12):
//
//	webhook-id:        the message id (bex: the event's evt-… id)
//	webhook-timestamp: unix epoch seconds of THIS attempt
//	webhook-signature: "v1," + base64(HMAC-SHA256(key, id + "." + timestamp + "." + body))
//
// where key is the raw secret bytes — the base64-decoded remainder of the
// "whsec_…" secret handed out at endpoint creation. A receiver verifies with
// any standard-webhooks library, or the ~5 lines Verify below spells out.

// secretPrefix is Standard Webhooks' secret serialization prefix.
const secretPrefix = "whsec_"

// secretBytes is the signing-key length NewSecret mints — 24 random bytes,
// standard-webhooks' own reference size.
const secretBytes = 24

// NewSecret mints an endpoint signing secret: "whsec_" + base64(24 random
// bytes). Returned to the creating caller exactly once.
func NewSecret() (string, error) {
	raw := make([]byte, secretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return secretPrefix + base64.StdEncoding.EncodeToString(raw), nil
}

// signingKey extracts the raw HMAC key from a serialized secret. A secret
// without the whsec_ prefix (or with an undecodable remainder) signs with its
// raw string bytes — the permissive fallback standard-webhooks libraries
// apply, so an imported foreign secret still verifies symmetrically.
func signingKey(secret string) []byte {
	if rest, ok := strings.CutPrefix(secret, secretPrefix); ok {
		if key, err := base64.StdEncoding.DecodeString(rest); err == nil {
			return key
		}
	}
	return []byte(secret)
}

// Sign computes the webhook-signature header value for one delivery attempt:
// "v1," + base64(HMAC-SHA256(key, msgID + "." + unixTimestamp + "." + body)).
func Sign(secret, msgID string, at time.Time, body []byte) string {
	mac := hmac.New(sha256.New, signingKey(secret))
	fmt.Fprintf(mac, "%s.%d.", msgID, at.Unix())
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Verify reports whether signatureHeader (a space-delimited list of
// "v1,<base64>" entries, per Standard Webhooks) contains a valid signature
// for the message. It is what a receiver does — exported so bex's own tests
// and the verification harness (scripts/webhooks-verify.sh) check deliveries
// with the same code a real consumer would write.
func Verify(secret, msgID, timestamp string, body []byte, signatureHeader string) bool {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	want := Sign(secret, msgID, time.Unix(ts, 0), body)
	for _, sig := range strings.Fields(signatureHeader) {
		if subtle.ConstantTimeCompare([]byte(sig), []byte(want)) == 1 {
			return true
		}
	}
	return false
}
