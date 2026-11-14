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

package credpw

import (
	"encoding/base64"
	"regexp"
	"testing"
)

// urlSafe is the shape both callers rely on: 32 RawURLEncoding characters,
// none of which need escaping inside a connection URI.
var urlSafe = regexp.MustCompile(`^[A-Za-z0-9_-]{32}$`)

func TestGenerateIsURLSafeAndFresh(t *testing.T) {
	seen := make(map[string]struct{}, 64)
	for i := 0; i < 64; i++ {
		pw, err := Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if !urlSafe.MatchString(pw) {
			t.Fatalf("Generate() = %q, want 32 URL-safe characters", pw)
		}
		if raw, err := base64.RawURLEncoding.DecodeString(pw); err != nil || len(raw) != passwordBytes {
			t.Fatalf("Generate() = %q does not decode to %d bytes: %v", pw, passwordBytes, err)
		}
		if _, dup := seen[pw]; dup {
			t.Fatalf("Generate() repeated %q across 64 draws", pw)
		}
		seen[pw] = struct{}{}
	}
}
