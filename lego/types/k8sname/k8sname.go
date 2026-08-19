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

// Package k8sname derives Kubernetes object names that stay within the
// 63-character DNS-1123 label limit. A leaf utility shared by both sides of the
// App CR boundary; it carries no bex domain types.
package k8sname

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// MaxLabel is Kubernetes' DNS-1123 label limit.
const MaxLabel = 63

// Stable preserves a readable prefix while binding every truncated name to the
// complete identity tuple, so names that differ only past the cut cannot
// collide. Truncation must never discard the revision or purpose that
// distinguishes two resources: a plain slice makes every revision of a
// long-named object resolve to ONE name, which silently turns "reuse the
// existing object for this revision" into "serve the wrong revision".
func Stable(raw string, parts ...string) string {
	const hashLength = 12
	raw = strings.ToLower(raw)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	suffix := fmt.Sprintf("%x", sum[:hashLength/2])
	maxBase := MaxLabel - 1 - len(suffix)
	if len(raw) > maxBase {
		raw = raw[:maxBase]
	}
	raw = strings.TrimRight(raw, "-.")
	return raw + "-" + suffix
}

// Fit returns raw unchanged when it fits a DNS-1123 label, and its Stable
// truncation otherwise.
func Fit(raw string) string {
	raw = strings.ToLower(raw)
	if len(raw) <= MaxLabel {
		return raw
	}
	return Stable(raw, raw)
}
