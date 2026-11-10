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

package workspaces

import (
	"testing"

	"github.com/graphql-go/graphql"
)

// w6/m309: the pipeline spend limit is a dollar amount with cents.
func TestParseUSDCents(t *testing.T) {
	for raw, want := range map[string]int64{"25": 2500, "25.5": 2550, "25.50": 2550, " $7.05 ": 705, "0": 0, "1000000": 100_000_000} {
		got, err := parseUSDCents(raw)
		if err != nil || got != want {
			t.Errorf("parseUSDCents(%q) = %d, %v; want %d", raw, got, err, want)
		}
	}
	for _, raw := range []string{"-1", "1.005", "abc", "1000000.01", "1e3", "100/4", "0x10", "1_000", "25.500", "",
		"184467440737095516.16", "92233720368547758.08"} {
		if _, err := parseUSDCents(raw); err == nil {
			t.Errorf("parseUSDCents(%q) accepted it", raw)
		}
	}
	if s := pipelineSettings("tea-a", nil); s.SpendLimitUSD != nil {
		t.Errorf("no limit = %+v, want nil", s)
	}
	cents := int64(2550)
	if s := pipelineSettings("tea-a", &cents); s.SpendLimitUSD == nil || *s.SpendLimitUSD != "25.50" {
		t.Errorf("limit = %+v, want 25.50", s)
	}
}

// The GraphQL field reads null for no limit and the amount otherwise.
func TestPipelineSettingsGraphQLSpendLimit(t *testing.T) {
	field := pipelineSettingsGQLType.Fields()["spendLimitUsd"]
	cents := int64(2550)
	for _, tc := range []struct {
		in   PipelineSettings
		want any
	}{
		{pipelineSettings("tea-a", nil), nil},
		{pipelineSettings("tea-a", &cents), "25.50"},
	} {
		got, err := field.Resolve(graphql.ResolveParams{Source: tc.in})
		if err != nil {
			t.Fatal(err)
		}
		if out := graphql.String.Serialize(got); out != tc.want {
			t.Errorf("spendLimitUsd = %#v, want %#v", out, tc.want)
		}
	}
}
