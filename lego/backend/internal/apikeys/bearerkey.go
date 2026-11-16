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

package apikeys

import "strings"

// bearerkey.go is the one definition of an API key's bearer form
// (docs/ADR012-auth.md §8e): the string a caller sends as
// `Authorization: Bearer`, as a Render API key is sent. It is the key's id and
// secret behind a fixed prefix, so the holder of an existing key can compose
// it and bex stores nothing new:
//
//	bex_<id>_<secret>
//
// The prefix cannot open a Hydra access token (`ory_at_…`) or a JWT (`eyJ…`),
// which is how the auth gate tells a key from a token without asking Hydra.
// The id is a Hydra client id, a UUID, which holds no underscore, so the first
// underscore after the prefix ends it; the secret may hold more of them.

// bearerKeyPrefix opens every API key sent as a bearer.
const bearerKeyPrefix = "bex_"

const (
	maxBearerKeyIDLen     = 64
	maxBearerKeySecretLen = 256
)

// BearerKey composes the bearer form of the key with this id and secret.
func BearerKey(id, secret string) string {
	return bearerKeyPrefix + id + "_" + secret
}

// IsBearerKey reports whether a bearer claims to be an API key. A string that
// claims it and does not parse is refused without asking Hydra.
func IsBearerKey(bearer string) bool {
	return strings.HasPrefix(bearer, bearerKeyPrefix)
}

// ParseBearerKey splits a bearer key into the id and secret it was composed
// from. ok is false for anything that is not one: a missing prefix, a missing
// part, or a character neither Hydra's client ids (letters, digits, hyphen)
// nor its secrets (those plus `.`, `_`, `~`) can hold.
func ParseBearerKey(bearer string) (id, secret string, ok bool) {
	rest, found := strings.CutPrefix(bearer, bearerKeyPrefix)
	if !found {
		return "", "", false
	}
	id, secret, found = strings.Cut(rest, "_")
	if !found || !bearerKeyPart(id, maxBearerKeyIDLen, "-") || !bearerKeyPart(secret, maxBearerKeySecretLen, "-._~") {
		return "", "", false
	}
	return id, secret, true
}

// bearerKeyPart reports whether s is 1 to limit ASCII letters, digits and
// the given punctuation.
func bearerKeyPart(s string, limit int, punctuation string) bool {
	if s == "" || len(s) > limit {
		return false
	}
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.IndexByte(punctuation, c) >= 0:
		default:
			return false
		}
	}
	return true
}
