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

func TestEnvironmentAliasContract(t *testing.T) {
	canonical := New(Environment)
	if !regexp.MustCompile(`^evm-[a-z0-9]{20}$`).MatchString(canonical) {
		t.Fatalf("environment %q fails the pinned Render CLI discriminator", canonical)
	}
	legacy := "env-" + canonical[4:]
	for _, input := range []string{canonical, legacy} {
		if EnvironmentPublicID(input) != canonical || EnvironmentStorageID(input) != legacy {
			t.Fatalf("alias %q no longer refers to %q / %q", input, canonical, legacy)
		}
	}
	for _, input := range []string{"", "production", "env-prod", "evm-prod", "env_" + canonical[4:], "evm-" + canonical[4:] + "0", "evm-0000000000000000000z", "ENV-" + canonical[4:], New(EnvGroup)} {
		if EnvironmentPublicID(input) != input || EnvironmentStorageID(input) != input {
			t.Errorf("non-alias %q changed identity", input)
		}
	}
}
