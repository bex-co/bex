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
	"strings"
	"testing"
)

func TestValidPostgresIdentifier(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want bool
	}{
		{name: "orders", want: true},
		{name: "orders_2026", want: true},
		{name: "_internal", want: true},
		{name: "1orders", want: false},
		{name: "orders-api", want: false},
		{name: "Orders", want: false},
		{name: "", want: false},
		{name: strings.Repeat("a", 63), want: true},
		{name: strings.Repeat("a", 64), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ValidPostgresIdentifier(tc.name); got != tc.want {
				t.Fatalf("ValidPostgresIdentifier(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestEffectiveDatabaseIdentifiersDefaultIndependently(t *testing.T) {
	t.Parallel()
	const resourceID = "dpg-abc123"

	for _, tc := range []struct {
		name     string
		spec     DatabaseSpec
		wantDB   string
		wantUser string
	}{
		{name: "both default", wantDB: "dpg_abc123", wantUser: "dpg_abc123_user"},
		{name: "custom database", spec: DatabaseSpec{DatabaseName: "orders"}, wantDB: "orders", wantUser: "dpg_abc123_user"},
		{name: "custom user", spec: DatabaseSpec{DatabaseUser: "reporter"}, wantDB: "dpg_abc123", wantUser: "reporter"},
		{name: "both custom", spec: DatabaseSpec{DatabaseName: "orders", DatabaseUser: "orders_owner"}, wantDB: "orders", wantUser: "orders_owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.EffectiveDatabaseName(resourceID); got != tc.wantDB {
				t.Fatalf("EffectiveDatabaseName() = %q, want %q", got, tc.wantDB)
			}
			if got := tc.spec.EffectiveDatabaseUser(resourceID); got != tc.wantUser {
				t.Fatalf("EffectiveDatabaseUser() = %q, want %q", got, tc.wantUser)
			}
		})
	}
}
