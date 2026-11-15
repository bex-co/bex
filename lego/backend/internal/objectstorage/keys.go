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

package objectstorage

import (
	"errors"
	"strings"
	"unicode/utf8"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// ErrInvalidKey refuses an object key bex will not store (ADR112 D7).
var ErrInvalidKey = errors.New("invalid object key")

// ValidateKey checks a key as the caller gave it, after the HTTP layer has
// decoded it. It rejects an empty key, one over MaxKeyBytes, one that is not
// UTF-8, one that starts with "/", one with a "." or ".." segment, and one
// with a backslash or a control character.
//
// The "." and ".." rule is the one that matters for isolation: Wasabi's
// handling of ".." in a key is unproven on a live deployment, and a ".."
// segment is the one input that could step out of <workspace>/<bucket>/. Keys
// are otherwise opaque: "a//b" and a trailing "/" are valid S3 keys and stay
// valid here (http.ServeMux cleans such paths before a handler sees them).
func ValidateKey(key string) error {
	if key == "" || len(key) > MaxKeyBytes || !utf8.ValidString(key) || key[0] == '/' {
		return ErrInvalidKey
	}
	for _, r := range key {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return ErrInvalidKey
		}
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "." || segment == ".." {
			return ErrInvalidKey
		}
	}
	return nil
}

// ObjectKey is the Wasabi key of key in a bucket: <workspace id>/<bucket id>/<key>.
// The bucket's id, not its name, is the second segment, so a bucket deleted and
// recreated under the same name never sees the old objects. It validates all
// three parts, so nothing that reaches Wasabi can leave the prefix.
func ObjectKey(workspaceID, bucketID, key string) (string, error) {
	if err := validateScope(workspaceID, bucketID); err != nil {
		return "", err
	}
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	return Prefix(workspaceID, bucketID) + key, nil
}

// Prefix is the Wasabi key prefix of a bucket, with its trailing slash.
func Prefix(workspaceID, bucketID string) string {
	return workspaceID + "/" + bucketID + "/"
}

// WorkspacePrefix is the Wasabi key prefix of every bucket of a workspace.
func WorkspacePrefix(workspaceID string) string { return workspaceID + "/" }

// ParseObjectKey splits a Wasabi key back into its workspace id, bucket id and
// the customer's key. ok is false for anything that is not a well-formed
// <workspace id>/<bucket id>/<key>, which the sweep treats as junk.
func ParseObjectKey(wasabiKey string) (workspaceID, bucketID, key string, ok bool) {
	parts := strings.SplitN(wasabiKey, "/", 3)
	if len(parts) != 3 || validateScope(parts[0], parts[1]) != nil || parts[2] == "" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func validateScope(workspaceID, bucketID string) error {
	if k, ok := ids.KindOf(workspaceID); !ok || k != ids.Workspace {
		return ErrInvalidKey
	}
	if k, ok := ids.KindOf(bucketID); !ok || k != ids.ObjectBucket {
		return ErrInvalidKey
	}
	return nil
}
