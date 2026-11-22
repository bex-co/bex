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

package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// adminAppRoleChecker is a workspace member: sensitive decides
// can_view_sensitive (a Developer holds it, a Contributor does not), admin
// decides can_manage, and every other relation is held.
type adminAppRoleChecker struct{ sensitive, admin bool }

func (c *adminAppRoleChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	switch relation {
	case core.RelCanViewSensitive:
		return c.sensitive, nil
	case core.RelCanManage:
		return c.admin, nil
	}
	return true, nil
}

// TestDeployPostgresAdminAppOnGraphQLPG (w4/m551): a Postgres database's Apps
// page deploys PgHero as a web service in the database's environment,
// connected by its internal URL set server side. A caller without
// can_view_sensitive gets the shared forbidden error and a Developer in a
// protected environment the shared placement refusal; neither creates a
// service.
func TestDeployPostgresAdminAppOnGraphQLPG(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	owner, err := st.CreateWorkspace(ctx, "admin-apps-"+ids.New(ids.Workspace), store.PlanHobby, "client-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), owner.ID) })
	project, err := st.CreateProject(ctx, owner.ID, "shop")
	if err != nil {
		t.Fatal(err)
	}
	environment := func(name, protectedStatus string) store.Environment {
		e, err := st.CreateEnvironment(ctx, project.ID, owner.ID, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetEnvironmentACL(ctx, e.ID, protectedStatus, false, e.IPAllowList); err != nil {
			t.Fatal(err)
		}
		return e
	}
	staging, production := environment("staging", "unprotected"), environment("production", "protected")
	namespace := store.WorkspaceNamespace(owner.ID)
	database := func(environmentID string) (*appv1alpha1.Database, *corev1.Secret) {
		d := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: ids.New(ids.Postgres), Namespace: namespace,
				Labels: map[string]string{core.LabelTenant: owner.ID, core.LabelEnvironment: environmentID}},
			Spec: appv1alpha1.DatabaseSpec{Name: "orders"},
		}
		d.Status.Host = d.Name + "-rw." + namespace + ".svc"
		return d, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: d.CurrentConnectionSecretName(), Namespace: namespace},
			Data:       map[string][]byte{"username": []byte("orders_user"), "password": []byte("pw"), "dbname": []byte("orders_db")},
		}
	}
	stagingDB, stagingSecret := database(staging.ID)
	productionDB, productionSecret := database(production.ID)

	role := &adminAppRoleChecker{}
	base := serverBase(t, st)
	base.Client = fakeClient(stagingDB, stagingSecret, productionDB, productionSecret)
	base.Authz = role
	handler, _ := serverWith(t, base, Deps{
		WorkspaceStore: st, Store: st, ProjectsStore: st, EnvironmentsStore: st, AppPlacements: st,
		EnvGroupsStore: st, Secrets: newMemSecretStore(),
	})
	deploy := func(databaseID string) string {
		return fmt.Sprintf(`mutation { deployPostgresAdminApp(databaseId: %q, app: "pghero", username: "hero", password: "hero-pass") { id environmentId } }`, databaseID)
	}

	// A Contributor, who cannot read the database's credential, is refused.
	codedRefusal{status: http.StatusForbidden, code: "FORBIDDEN"}.onGraphQL(t, handler, deploy(stagingDB.Name))
	// A Developer cannot create in the protected environment.
	role.sensitive = true
	codedRefusal{status: http.StatusForbidden, code: core.ProtectedPlacementCode,
		params: map[string]any{"environmentId": ids.EnvironmentPublicID(production.ID)}}.onGraphQL(t, handler, deploy(productionDB.Name))
	if n := serviceRows(t, pool, owner.ID); n != 0 {
		t.Fatalf("a refused deploy wrote %d services", n)
	}

	created := gql(t, handler, deploy(stagingDB.Name))["deployPostgresAdminApp"].(map[string]any)
	if created["environmentId"] != ids.EnvironmentPublicID(staging.ID) {
		t.Fatalf("PgHero landed in %v, want the database's environment %s", created["environmentId"], ids.EnvironmentPublicID(staging.ID))
	}
	page := gql(t, handler, fmt.Sprintf(`{ postgresAdminApps(databaseId: %q) { id service { id envVar(key: "DATABASE_URL") { value } } } }`, stagingDB.Name))
	cards := page["postgresAdminApps"].([]any)
	hero := cards[1].(map[string]any)
	if hero["id"] != "pghero" || cards[0].(map[string]any)["service"] != nil {
		t.Fatalf("Apps page = %v, want pgAdmin undeployed and PgHero deployed", cards)
	}
	service := hero["service"].(map[string]any)
	if service["id"] != created["id"] {
		t.Fatalf("PgHero's service = %v, want the deployed %v", service["id"], created["id"])
	}
	want := "postgresql://orders_user:pw@" + stagingDB.Status.Host + ":5432/orders_db"
	if got := service["envVar"].(map[string]any)["value"]; got != want {
		t.Fatalf("DATABASE_URL = %v, want the internal URL %s", got, want)
	}
	if n := serviceRows(t, pool, owner.ID); n != 1 {
		t.Fatalf("services = %d, want the one PgHero", n)
	}
}
