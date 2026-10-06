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
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// postgresIdentifierVectorsPath is the one table of Postgres identifiers: the
// operator's TestManagedRolesNeverProjectReservedRoles and the dashboard's
// identifiers test read the same file, and these predicates are its source of
// truth (w5/m118).
const postgresIdentifierVectorsPath = "testdata/postgres-identifiers.json"

func TestPostgresIdentifierVectors(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(postgresIdentifierVectorsPath)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Name             string `json:"name"`
		Repeat           int    `json:"repeat"`
		Valid            bool   `json:"valid"`
		ReservedRole     bool   `json:"reservedRole"`
		ReservedDatabase bool   `json:"reservedDatabase"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	reserved := 0
	for _, row := range rows {
		name := row.Name
		if row.Repeat > 0 {
			name = strings.Repeat(name, row.Repeat)
		}
		if got := ValidPostgresIdentifier(name); got != row.Valid {
			t.Errorf("ValidPostgresIdentifier(%.20q) = %v, the table says %v", name, got, row.Valid)
		}
		if got := ReservedPostgresRole(name); got != row.ReservedRole {
			t.Errorf("ReservedPostgresRole(%q) = %v, the table says %v", name, got, row.ReservedRole)
		}
		if got := ReservedPostgresDatabaseName(name); got != row.ReservedDatabase {
			t.Errorf("ReservedPostgresDatabaseName(%q) = %v, the table says %v", name, got, row.ReservedDatabase)
		}
		if row.ReservedRole || row.ReservedDatabase {
			reserved++
		}
	}
	if reserved < 10 || len(rows) < 20 {
		t.Fatalf("vector table too small: %d rows, %d reserved", len(rows), reserved)
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
