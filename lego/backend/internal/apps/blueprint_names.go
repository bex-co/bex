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
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
	return capResourceSlug(name, foldedSlug(name), true)
}

// legacyBlueprintResourceSlug is the mapping before w4/385, which dropped
// accented letters and gave every name with no a–z or 0–9 the one name
// "resource". A service created under it keeps that name; the Blueprint plan
// still finds it (workspaceSnapshot.services).
func legacyBlueprintResourceSlug(name string) string {
	if appv1alpha1.ValidResourceName(name) {
		return name
	}
	slug := asciiSlug(strings.ToLower(name))
	if slug == "" {
		slug = "resource"
	}
	return capResourceSlug(name, slug, false)
}

// foldedSlug is name's slug with accents folded onto their base letters
// (NFKD, combining marks dropped), so "Ünï" reads "uni" rather than "n".
func foldedSlug(name string) string {
	var folded strings.Builder
	for _, r := range norm.NFKD.String(strings.ToLower(name)) {
		if !unicode.IsMark(r) {
			folded.WriteRune(r)
		}
	}
	return asciiSlug(folded.String())
}

// asciiSlug keeps a–z and 0–9 and hyphenates every other run.
func asciiSlug(lower string) string {
	var b strings.Builder
	dash := false
	for _, r := range lower {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// capResourceSlug applies the name-specific fallbacks: a name with nothing
// left takes "resource-<hash of the name>" so two different non-Latin names
// stay distinct and one name stays stable, and a slug past the cap keeps its
// start plus that hash.
func capResourceSlug(name, slug string, hashEmpty bool) string {
	sum := func() string { return fmt.Sprintf("%x", sha256.Sum256([]byte(name)))[:6] }
	if slug == "" && hashEmpty {
		return "resource-" + sum()
	}
	if len(slug) > appv1alpha1.MaxResourceNameLength {
		hash := sum()
		slug = strings.TrimRight(slug[:appv1alpha1.MaxResourceNameLength-len(hash)-1], "-") + "-" + hash
	}
	return slug
}

// serviceNameLabel is the label a service keeps when its free-form name is
// not itself a resource name, "" when it is. A name that is no valid label is
// refused with the label rule's own error, and an empty or blank one as a
// missing name, never with the resource-name rule it was never held to
// (w4/417).
func serviceNameLabel(name string) (string, error) {
	if appv1alpha1.ValidResourceName(name) {
		return "", nil
	}
	label, err := core.CheckLabel(name)
	if err != nil {
		return "", err
	}
	if label == "" {
		return "", core.NameRequiredError()
	}
	return name, nil
}

// slugRefNames maps a reference to a service onto the resource name its
// declaration got, so fromService resolves. A Postgres or Key Value keeps its
// free-form name (w5/m153), so references to one stay as written.
func slugRefNames(e *bexEnvVar) {
	if e.FromService != nil && !isKeyValueType(e.FromService.Type) {
		e.FromService.Name = blueprintResourceSlug(e.FromService.Name)
	}
}
