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

package sshgateway

import (
	_ "embed"
	"strings"
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
