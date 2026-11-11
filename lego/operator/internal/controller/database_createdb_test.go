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

package controller

import (
	"reflect"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// database_createdb_test.go is w8/m155. Render: "You can create additional
// databases in your Render Postgres instance … Run `CREATE DATABASE <name>`".
// On bex the default user answered `permission denied to create database`:
// managedRoles projected no `createdb`, and a managed role's omitted attribute
// is false.

func TestDefaultUsersMayCreateDatabases(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	users := []appv1alpha1.DatabaseUser{
		{Name: "second", SecretName: "s1", OwnerPrivileges: true},
		{Name: "reporting", SecretName: "s2"},
		{Name: "third", SecretName: "s3", OwnerPrivileges: true},
	}
	roles := managedRoleIndex(t, cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user",
		users: users, deletedUsers: []string{"first"}}))

	// The owner and each default credential carry the attribute themselves: a
	// role attribute is not inherited through membership.
	for _, name := range []string{"d_user", "second", "third"} {
		if roles[name]["createdb"] != true {
			t.Errorf("default user %q = %v, want createdb: true", name, roles[name])
		}
	}
	// An auxiliary user is granted nothing, and a retired login is no user.
	for _, name := range []string{"reporting", "first"} {
		if _, has := roles[name]["createdb"]; has {
			t.Errorf("%q = %v, want no createdb", name, roles[name])
		}
	}
	// No role is given more than that.
	for name, role := range roles {
		for _, attribute := range []string{"superuser", "createrole", "replication", "bypassrls"} {
			if _, has := role[attribute]; has {
				t.Errorf("%q carries %s: %v", name, attribute, role)
			}
		}
	}

	// The first default credential, with no owner to join, is still a default.
	alone := managedRoles("", []appv1alpha1.DatabaseUser{{Name: "only", OwnerPrivileges: true}}, nil)
	if len(alone) != 1 || alone[0].(map[string]any)["createdb"] != true {
		t.Errorf("a lone default credential = %v, want createdb: true", alone)
	}
}

// Render's "deleted" original user is kept with what it owns, retired like any
// other login. It stops being a default user, so the attribute goes with it.
func TestARetiredOwnerMayNotCreateDatabases(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	roles := managedRoleIndex(t, cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user",
		users:        []appv1alpha1.DatabaseUser{{Name: "second", SecretName: "s1", OwnerPrivileges: true}},
		deletedUsers: []string{"d_user"}}))
	if _, has := roles["d_user"]["createdb"]; has || roles["d_user"]["login"] != false {
		t.Errorf("retired owner = %v, want a retired login with no createdb", roles["d_user"])
	}
	if roles["second"]["createdb"] != true {
		t.Errorf("the credential that replaced it = %v, want createdb: true", roles["second"])
	}
}

// An existing database gains the attribute at its next reconcile: merging it
// into the live roles entry, which carries CloudNativePG's own defaults,
// settles in one pass, and retiring the owner later takes it away again.
func TestCreateDBMergesIntoALiveRoleInOnePass(t *testing.T) {
	before := managedRoles("d_user", nil, nil)
	for _, role := range before {
		delete(role.(map[string]any), "createdb") // the projection before w8/m155
	}
	// What the API server holds: the old projection plus CNPG's defaults.
	live := []any{map[string]any{
		"name": "d_user", "ensure": "present", "login": true, "inRoles": []any{"pg_monitor"},
		"connectionLimit": int64(-1), "inherit": true,
	}}
	desired := managedRoles("d_user", nil, nil)

	merged := mergeNamedProjectionList(live, before, desired)
	role := merged[0].(map[string]any)
	if role["createdb"] != true || role["connectionLimit"] != int64(-1) || role["inherit"] != true {
		t.Fatalf("merged role = %v, want createdb added and CNPG's defaults kept", role)
	}
	if again := mergeNamedProjectionList(merged, desired, desired); !reflect.DeepEqual(again, merged) {
		t.Fatalf("a second pass changed the role:\n first  %v\n second %v", merged, again)
	}

	retired := managedRoles("d_user", nil, []string{"d_user"})
	gone := mergeNamedProjectionList(merged, desired, retired)[0].(map[string]any)
	if _, has := gone["createdb"]; has || gone["login"] != false {
		t.Fatalf("retired owner after the merge = %v, want createdb withdrawn", gone)
	}
}

// A restored database bootstraps from a backup, not initdb. Its roles come
// from the same projection, so the restored owner keeps the ability.
func TestARestoredDatabaseKeepsCreateDB(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	rec := &appv1alpha1.DatabaseRecovery{SourceDatabase: "src", SourceBackupServerName: "src-pg16", TargetTime: "2026-07-09T10:00:00Z"}
	spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user", store: &testStore, recovery: rec})
	if _, initdb := spec["bootstrap"].(map[string]any)["initdb"]; initdb {
		t.Fatal("the fixture is not a recovery bootstrap")
	}
	if owner := managedRoleIndex(t, spec)["d_user"]; owner["createdb"] != true {
		t.Errorf("restored owner = %v, want createdb: true", owner)
	}
}
