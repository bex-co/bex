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

package id

import (
	"regexp"
	"testing"
)

func TestBlueprintAliasContract(t *testing.T) {
	durable := New(Blueprint)
	public := BlueprintPublicID(durable)
	// The pinned Render CLI's IsBlueprintID discriminator (w2/m269).
	if !regexp.MustCompile(`^exs-[a-z0-9]{20}$`).MatchString(public) {
		t.Fatalf("blueprint %q fails the pinned Render CLI discriminator", public)
	}
	if public[4:] != durable[4:] {
		t.Fatalf("public %q does not keep durable %q's suffix", public, durable)
	}
	for _, input := range []string{public, durable} {
		if BlueprintPublicID(input) != public || BlueprintStorageID(input) != durable {
			t.Fatalf("alias %q no longer refers to %q / %q", input, public, durable)
		}
	}
	for _, input := range []string{"", "my-blueprint", "blp-prod", "exs-prod", "blp_" + durable[4:], "exs-" + durable[4:] + "0", "exs-0000000000000000000z", "EXS-" + durable[4:], "BLP-" + durable[4:], New(BlueprintSync), New(Environment)} {
		if BlueprintPublicID(input) != input || BlueprintStorageID(input) != input {
			t.Errorf("non-alias %q changed identity", input)
		}
	}
	if _, ok := kindForPrefix(BlueprintPublicPrefix); ok {
		t.Fatalf("public prefix %q collides with a registered kind", BlueprintPublicPrefix)
	}
}
