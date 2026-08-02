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

package secrets

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"
)

const envRevisionPrefix = "evr1_"

var errInvalidEnvRevision = errors.New("invalid environment revision")

// encodeEnvRevision keeps the OpenBao KV version opaque at the API boundary.
// It is a concurrency token, not a credential: encoding prevents callers from
// depending on the backend's integer representation without adding false
// cryptographic meaning.
func encodeEnvRevision(version uint64) string {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], version)
	return envRevisionPrefix + base64.RawURLEncoding.EncodeToString(raw[:])
}

// decodeEnvRevision accepts only the one canonical fixed-width encoding. Its
// error is deliberately constant and never echoes the supplied token.
func decodeEnvRevision(token string) (uint64, error) {
	encoded, ok := strings.CutPrefix(token, envRevisionPrefix)
	if !ok {
		return 0, errInvalidEnvRevision
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) != 8 || base64.RawURLEncoding.EncodeToString(raw) != encoded {
		return 0, errInvalidEnvRevision
	}
	return binary.BigEndian.Uint64(raw), nil
}
