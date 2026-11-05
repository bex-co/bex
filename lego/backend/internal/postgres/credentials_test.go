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

package postgres

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w5/m265: MCP's credential tools drive the same rotation as REST — the new
// login becomes the default with owner privileges, the default cannot be
// deleted, and each write answers the resulting list.
func TestMCPCredentialRotation(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "dpg-rot")
	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()

	got := call("create_postgres_credential", map[string]any{"postgresId": "dpg-rot", "username": "rotated"})
	creds, _ := got["credentials"].([]any)
	if len(creds) != 2 {
		t.Fatalf("credentials = %v", got)
	}
	if c := creds[1].(map[string]any); c["username"] != "rotated" || c["default"] != true {
		t.Fatalf("new credential = %v, want rotated as default", c)
	}
	if c := creds[0].(map[string]any); c["default"] != false {
		t.Fatalf("owner = %v, want no longer default", c)
	}
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "dpg-rot"}, &db); err != nil {
		t.Fatal(err)
	}
	if db.Spec.DefaultUser != "rotated" || len(db.Spec.Users) != 1 || !db.Spec.Users[0].OwnerPrivileges || db.Spec.Users[0].SecretName == "" {
		t.Fatalf("spec = %+v", db.Spec)
	}

	if _, err := svc.DeleteCredential(context.Background(), "dpg-rot", "rotated"); err == nil {
		t.Fatal("deleting the default credential must be refused")
	}
	// The native auxiliary path refuses it too, so it cannot strand consumers.
	if err := svc.DeleteUser(context.Background(), "dpg-rot", "rotated"); err == nil {
		t.Fatal("native delete of the default credential must be refused")
	}
	// A rotated name cannot collide with the owner role.
	if _, err := svc.CreateCredential(context.Background(), "dpg-rot", db.Spec.EffectiveDatabaseUser("dpg-rot")); err == nil {
		t.Fatal("a credential named like the owner must be refused")
	}
}
