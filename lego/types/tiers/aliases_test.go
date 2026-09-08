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

package tiers

import "testing"

func TestPostgresCanonicalIDAcceptsRenderAliases(t *testing.T) {
	cases := map[string]string{
		"basic-256mb": "basic-256mb",
		"0.1c-256mb":  "basic-256mb",
		"0.5c-1g":     "basic-1gb",
		"basic_256mb": "basic-256mb",
		"basic_1gb":   "basic-1gb",
		"free":        "free",
		"2c-8g":       "2c-8g", // no bex rung — unchanged so ByID rejects
	}
	for in, want := range cases {
		if got := Postgres.CanonicalID(in); got != want {
			t.Errorf("Postgres.CanonicalID(%q) = %q, want %q", in, got, want)
		}
	}
	if _, ok := Postgres.ByID(Postgres.CanonicalID("0.1c-256mb")); !ok {
		t.Fatal("0.1c-256mb must resolve to a catalog tier")
	}
	if _, ok := Postgres.ByID(Postgres.CanonicalID("2c-8g")); ok {
		t.Fatal("2c-8g must not resolve to a catalog tier")
	}
}

func TestValkeyCanonicalIDAcceptsRenderAliases(t *testing.T) {
	cases := map[string]string{
		"starter":  "starter",
		"256mb":    "starter",
		"1g":       "standard",
		"standard": "standard",
		"5g":       "5g",
	}
	for in, want := range cases {
		if got := Valkey.CanonicalID(in); got != want {
			t.Errorf("Valkey.CanonicalID(%q) = %q, want %q", in, got, want)
		}
	}
	if _, ok := Valkey.ByID(Valkey.CanonicalID("1g")); !ok {
		t.Fatal("1g must resolve to standard")
	}
	if _, ok := Valkey.ByID(Valkey.CanonicalID("5g")); ok {
		t.Fatal("5g must not resolve to a catalog tier")
	}
}
