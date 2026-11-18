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

package v1alpha1

import (
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
)

// TestManagedOIDCProvidersAreRendersVariables pins each provider's trigger,
// file variable and audience to render.com/docs/oidc (read 2026-11-18,
// docs/ADR116-managed-oidc.md D9/D10). An app configured for Render's Managed
// OIDC moves to bex unchanged only while these are Render's spellings.
func TestManagedOIDCProvidersAreRendersVariables(t *testing.T) {
	want := []struct{ name, trigger, fileEnv, audience string }{
		{"aws", "AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE", "sts.amazonaws.com"},
		{"anthropic", "ANTHROPIC_FEDERATION_RULE_ID", "ANTHROPIC_IDENTITY_TOKEN_FILE", "api.anthropic.com"},
		{"openai", "OPENAI_IDENTITY_PROVIDER_ID", "OPENAI_IDENTITY_TOKEN_FILE", "api.openai.com"},
	}
	if len(ManagedOIDCProviders) != len(want) {
		t.Fatalf("%d providers, want %d", len(ManagedOIDCProviders), len(want))
	}
	for i, w := range want {
		p := ManagedOIDCProviders[i]
		if p.Name != w.name || p.Trigger != w.trigger || p.FileEnv != w.fileEnv || p.Audience != w.audience {
			t.Errorf("provider %d = %+v, want %+v", i, p, w)
		}
		if got, ok := ManagedOIDCProviderNamed(w.name); !ok || got != p {
			t.Errorf("ManagedOIDCProviderNamed(%q) = %+v, %v", w.name, got, ok)
		}
	}
}

// TestManagedOIDCProvidersNeverShareATokenOrAFile: a service with several
// providers gets one token per audience, each in its own file under its own
// variable. Two providers sharing an audience would get one token both
// accept; sharing a file or a variable, one would overwrite the other.
func TestManagedOIDCProvidersNeverShareATokenOrAFile(t *testing.T) {
	seen := map[string]map[string]string{"name": {}, "trigger": {}, "file variable": {}, "file": {}, "audience": {}}
	for _, p := range ManagedOIDCProviders {
		for field, v := range map[string]string{"name": p.Name, "trigger": p.Trigger, "file variable": p.FileEnv, "file": p.File, "audience": p.Audience} {
			if v == "" {
				t.Errorf("%s has no %s", p.Name, field)
			}
			if other, dup := seen[field][v]; dup {
				t.Errorf("%s and %s share the %s %q", other, p.Name, field, v)
			}
			seen[field][v] = p.Name
		}
		// The file is a Secret key and a file name in one directory.
		if errs := validation.IsConfigMapKey(p.File); len(errs) > 0 {
			t.Errorf("%s's file %q is not a valid Secret key: %v", p.Name, p.File, errs)
		}
	}
	// Named lookups keep the caller's order and skip a name no provider has.
	var names []string
	for _, p := range ManagedOIDCProvidersNamed([]string{"openai", "gcp-not-yet", "aws"}) {
		names = append(names, p.Name)
	}
	if !slices.Equal(names, []string{"openai", "aws"}) {
		t.Errorf("ManagedOIDCProvidersNamed = %v, want [openai aws]", names)
	}
}
