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

// Package dbrole single-sources the SSH gateway's least-privilege Postgres
// role surface: the grant DDL both the production script
// (scripts/ssh-gateway-db-role.sh) and the CI least-privilege proof
// (dbrole_integration_test.go) consume.
package dbrole

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// roleGrantsSQL is the least-privilege grant surface for the gateway's
// control-plane Postgres role (w7/m56, docs/ADR035-ssh.md §116). It is embedded
// from the same dbrole.sql that scripts/ssh-gateway-db-role.sh applies in
// production, so the tested boundary (dbrole_integration_test.go) and the shipped
// boundary cannot drift.
//
//go:embed dbrole.sql
var roleGrantsSQL string

// RoleGrantsSQL returns the grant DDL with the __ROLE__ placeholder bound to the
// concrete role name — the single-sourced privilege surface, so a change to the
// gateway's database needs must be made in dbrole.sql (which both the script and
// the CI test consume) and nowhere else.
func RoleGrantsSQL(role string) string {
	return strings.ReplaceAll(roleGrantsSQL, "__ROLE__", role)
}

type requiredPrivilege struct {
	relation  string
	privilege string
	schema    bool
}

var grantPattern = regexp.MustCompile(`(?m)^GRANT ([A-Z]+(?:, [A-Z]+)*) ON (?:(SCHEMA) )?([a-z_]+) TO __ROLE__;$`)

// requiredPrivileges derives the startup checks from the embedded DDL. The DDL
// remains the single source: adding or removing a grant changes provisioning,
// deploy reconciliation, scoped-role integration, and runtime preflight
// together instead of requiring a second hand-maintained table list.
func requiredPrivileges() ([]requiredPrivilege, error) {
	matches := grantPattern.FindAllStringSubmatch(roleGrantsSQL, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("gateway grant DDL contains no recognized grants")
	}
	out := make([]requiredPrivilege, 0, len(matches)*2)
	for _, match := range matches {
		for _, privilege := range strings.Split(match[1], ", ") {
			relation := match[3]
			schema := match[2] == "SCHEMA"
			if !schema {
				relation = "public." + relation
			}
			out = append(out, requiredPrivilege{relation: relation, privilege: privilege, schema: schema})
		}
	}
	return out, nil
}

// CheckRequiredPrivileges is the gateway's startup preflight. It checks the
// installed role, not merely dbrole.sql in source, so a migration whose grant
// reconciliation was skipped fails visibly before any user's SSH/attach request
// reaches a latent SQLSTATE 42501. Error text contains only relation/privilege
// names; it never includes the connection URI or tenant data.
func CheckRequiredPrivileges(ctx context.Context, pool *pgxpool.Pool) error {
	required, err := requiredPrivileges()
	if err != nil {
		return err
	}
	var allowed bool
	for _, privilege := range required {
		query := `SELECT has_table_privilege(current_user, $1, $2)`
		if privilege.schema {
			query = `SELECT has_schema_privilege(current_user, $1, $2)`
		}
		if err := pool.QueryRow(ctx, query, privilege.relation, privilege.privilege).Scan(&allowed); err != nil {
			return fmt.Errorf("check required gateway privilege %s:%s: %w", privilege.relation, privilege.privilege, err)
		}
		if !allowed {
			return fmt.Errorf("required gateway privilege missing: %s:%s", privilege.relation, privilege.privilege)
		}
	}
	return nil
}
