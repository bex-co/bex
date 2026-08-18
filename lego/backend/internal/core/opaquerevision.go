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

package core

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
)

// EncodeOpaqueRevision hides a store's integer version behind a namespaced,
// fixed-width concurrency token. It does not add cryptographic meaning.
func EncodeOpaqueRevision(prefix string, version uint64) string {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], version)
	return prefix + base64.RawURLEncoding.EncodeToString(raw[:])
}

// DecodeOpaqueRevision accepts only the canonical token encoding for prefix.
// The boolean lets each feature retain its own constant, non-echoing error.
func DecodeOpaqueRevision(prefix, token string) (uint64, bool) {
	encoded, ok := strings.CutPrefix(token, prefix)
	if !ok {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) != 8 || base64.RawURLEncoding.EncodeToString(raw) != encoded {
		return 0, false
	}
	return binary.BigEndian.Uint64(raw), true
}
