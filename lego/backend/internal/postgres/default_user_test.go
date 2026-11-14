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
	"encoding/json"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/610: the view's default user is the login the connection info uses — the
// rotated default once a credential was added, the create-time owner before —
// while REST's databaseUser stays the create-time owner and carries no
// defaultUser.
func TestPostgresViewDefaultUserFollowsTheRotation(t *testing.T) {
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-a"}}
	db.Spec.DatabaseUser = "qa_owner"
	if v := pgView(db); v.DefaultUser != "qa_owner" || v.DatabaseUser != "qa_owner" {
		t.Fatalf("before a rotation: defaultUser %q, databaseUser %q; want both qa_owner", v.DefaultUser, v.DatabaseUser)
	}
	db.Spec.DefaultUser = "qa_user2"
	v := pgView(db)
	if v.DefaultUser != "qa_user2" || v.DatabaseUser != "qa_owner" {
		t.Fatalf("after a rotation: defaultUser %q, databaseUser %q; want qa_user2 and qa_owner", v.DefaultUser, v.DatabaseUser)
	}
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "defaultUser") || strings.Contains(string(body), "qa_user2") {
		t.Errorf("REST view carries the GraphQL-only default user: %s", body)
	}
}
