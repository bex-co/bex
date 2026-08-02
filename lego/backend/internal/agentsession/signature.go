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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

const signatureVersion = "bex-agent-credential-v1"

// Sign authenticates the gateway→bex-api hop with the already-shared sandbox
// exec secret, domain-separated from sandbox-exec tickets. The timestamp bounds
// capture/replay without adding any credential to the guest.
func Sign(secret, body []byte, now time.Time) (timestamp, signature string) {
	timestamp = strconv.FormatInt(now.Unix(), 10)
	return timestamp, hex.EncodeToString(signatureMAC(secret, body, timestamp))
}

func signatureMAC(secret, body []byte, timestamp string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signatureVersion + "\n" + timestamp + "\n"))
	_, _ = mac.Write(body)
	return mac.Sum(nil)
}

// Verify checks a signed internal request within maxSkew of now.
func Verify(secret, body []byte, timestamp, signature string, now time.Time, maxSkew time.Duration) error {
	if len(secret) == 0 || timestamp == "" || signature == "" {
		return ErrForbidden
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrForbidden
	}
	issued := time.Unix(unix, 0)
	if maxSkew <= 0 {
		maxSkew = 30 * time.Second
	}
	if issued.Before(now.Add(-maxSkew)) || issued.After(now.Add(maxSkew)) {
		return fmt.Errorf("%w: stale signature", ErrForbidden)
	}
	got, err := hex.DecodeString(signature)
	if err != nil || !hmac.Equal(got, signatureMAC(secret, body, timestamp)) {
		return ErrForbidden
	}
	return nil
}
