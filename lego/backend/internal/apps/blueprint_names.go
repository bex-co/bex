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

package apps

import (
	"crypto/sha256"
	"fmt"
	"strings"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprintResourceSlug is the resource name a Blueprint `name` maps onto.
// Render puts no rule on a Blueprint name beyond uniqueness, while a bex
// resource name is ADR020's slug (lowercase letters, digits and hyphens, at
// most 30 characters). A name that already is one is kept unchanged; any other
// is lowercased, every other character becomes a hyphen, and a name still too
// long keeps its start plus a short hash of the whole, so the mapping is
// stable across syncs and distinct long names stay distinct (w5/m150).
func blueprintResourceSlug(name string) string {
	if appv1alpha1.ValidResourceName(name) {
		return name
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if slug == "" {
		slug = "resource"
	}
	if len(slug) > appv1alpha1.MaxResourceNameLength {
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))[:6]
		slug = strings.TrimRight(slug[:appv1alpha1.MaxResourceNameLength-len(sum)-1], "-") + "-" + sum
	}
	return slug
}

// blueprintDisplayName is the label a service keeps when its Blueprint name is
// not itself a resource name, "" otherwise or when it is no valid label
// (checkDisplayName's rules).
func blueprintDisplayName(name string) string {
	if blueprintResourceSlug(name) == name {
		return ""
	}
	if _, err := checkDisplayName(&appv1alpha1.App{}, name); err != nil {
		return ""
	}
	return name
}

// slugRefNames maps a reference's target onto the same resource name its
// declaration got, so fromService and fromDatabase resolve.
func slugRefNames(e *bexEnvVar) {
	if e.FromService != nil {
		e.FromService.Name = blueprintResourceSlug(e.FromService.Name)
	}
	if e.FromDatabase != nil {
		e.FromDatabase.Name = blueprintResourceSlug(e.FromDatabase.Name)
	}
}
