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

// Package credpw generates the random passwords bex writes into Kubernetes
// Secrets for platform-managed credentials: a managed KeyValue instance's
// password (the operator, internal/controller/keyvalue_controller.go) and an
// added Postgres role's password (bex-api, internal/postgres/access.go).
//
// It is a leaf utility shared by both sides of the App CR boundary on
// purpose. The two callers used to carry byte-identical copies, and nothing
// tied them together: changing one (the byte length, the encoding) would
// silently diverge the other. One mint path keeps the scheme they promise in
// their comments true.
package credpw

import (
	"crypto/rand"
	"encoding/base64"
)

// passwordBytes is the entropy behind one password: 24 bytes = 192 bits.
// RawURLEncoding then yields 32 URL-safe characters with no `+`, `/`, or `=`,
// so a password embeds cleanly in a `redis://default:<password>@host`
// connection URI (and a Postgres URI) without escaping.
const passwordBytes = 24

// Generate returns a fresh URL-safe random password.
func Generate() (string, error) {
	b := make([]byte, passwordBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
