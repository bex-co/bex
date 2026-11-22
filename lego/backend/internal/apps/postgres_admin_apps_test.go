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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const adminAppDB = "dpg-d0adminappstest0001"

// adminAppFixture is a workspace tea-a holding one ready Postgres in
// environment env-a (frankfurt), its connection Secret, and the Service under
// test with a recording secret seeder.
func adminAppFixture(t *testing.T, checker core.Checker) (*Service, client.Client, *recordingCreateSecretsSeeder, context.Context) {
	t.Helper()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: adminAppDB, Namespace: "tea-a",
			Labels:      map[string]string{core.LabelTenant: "tea-a", core.LabelEnvironment: "env-a"},
			Annotations: map[string]string{resourcemeta.AnnotRegion: "frankfurt"},
		},
		Spec:   appv1alpha1.DatabaseSpec{Name: "orders"},
		Status: appv1alpha1.DatabaseStatus{Host: adminAppDB + "-rw.tea-a.svc"},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: db.CurrentConnectionSecretName(), Namespace: "tea-a"},
		Data: map[string][]byte{
			"username": []byte("orders_user"), "password": []byte("p:ss\\word"), "dbname": []byte("orders_db"),
		},
	}
	cl := fakeClient(db, secret)
	seeder := &recordingCreateSecretsSeeder{}
	svc := &Service{
		Base: &core.Base{
			Client: cl, Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: checker,
		},
		Environments:  fixedEnvironmentResolver{"env-a": {ID: "env-a", ProjectID: "prj-a", TenantID: "tea-a"}},
		DomainTraffic: allowDomainTraffic{},
		CreateSecrets: seeder,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session", Email: "ada@example.com"})
	return svc, cl, seeder, ctx
}

func adminAppServices(t *testing.T, cl client.Client) []appv1alpha1.App {
	t.Helper()
	var list appv1alpha1.AppList
	if err := cl.List(context.Background(), &list, client.MatchingLabels{LabelPostgresAdminAppDatabase: adminAppDB}); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

// PgHero is Render's image service, connected by the database's internal URL,
// set server side, in the database's workspace, region and environment.
func TestDeployPgHeroCreatesRendersServiceOnTheInternalURL(t *testing.T) {
	svc, cl, seeder, ctx := adminAppFixture(t, nil)
	v, err := svc.DeployPostgresAdminApp(ctx, DeployPostgresAdminAppRequest{
		DatabaseID: adminAppDB, App: PostgresAdminAppPgHero, Username: "hero", Password: "secret-pass",
	})
	if err != nil {
		t.Fatalf("DeployPostgresAdminApp: %v", err)
	}
	if v.DisplayName != "PgHero-"+adminAppDB {
		t.Errorf("service name = %q, want Render's PgHero-<database id>", v.DisplayName)
	}
	apps := adminAppServices(t, cl)
	if len(apps) != 1 {
		t.Fatalf("admin app services = %d, want 1", len(apps))
	}
	a := apps[0]
	if a.Namespace != "tea-a" || a.Labels[core.LabelTenant] != "tea-a" || a.Labels[core.LabelEnvironment] != "env-a" {
		t.Errorf("placement = ns %q labels %v, want the database's workspace and environment", a.Namespace, a.Labels)
	}
	if a.Labels[LabelPostgresAdminApp] != PostgresAdminAppPgHero {
		t.Errorf("app label = %q, want pghero", a.Labels[LabelPostgresAdminApp])
	}
	if a.Annotations[resourcemeta.AnnotRegion] != "frankfurt" {
		t.Errorf("region = %q, want the database's frankfurt", a.Annotations[resourcemeta.AnnotRegion])
	}
	if a.Spec.Image != pgHeroImage || a.Spec.Tier != "free" || a.Spec.Port != pgHeroPort {
		t.Errorf("spec image %q tier %q port %d, want %s on free with port %d", a.Spec.Image, a.Spec.Tier, a.Spec.Port, pgHeroImage, pgHeroPort)
	}
	want := map[string]string{
		"DATABASE_URL":    "postgresql://orders_user:p:ss\\word@" + adminAppDB + "-rw.tea-a.svc:5432/orders_db",
		"PGHERO_USERNAME": "hero",
		"PGHERO_PASSWORD": "secret-pass",
		"WEB_CONCURRENCY": "2",
	}
	for k, w := range want {
		if got := seeder.env[k]; got != w {
			t.Errorf("env %s = %q, want %q", k, got, w)
		}
	}
	if len(seeder.files) != 0 {
		t.Errorf("PgHero carries no secret files, got %d", len(seeder.files))
	}
}

// pgAdmin is built from Render's repository on starter with its 1 GB disk; it
// opens already connected because servers.json and the passfile name the
// database's private host, and its login defaults to the caller's email.
func TestDeployPgAdminRegistersTheServerAndLogsInByEmail(t *testing.T) {
	svc, cl, seeder, ctx := adminAppFixture(t, nil)
	if _, err := svc.DeployPostgresAdminApp(ctx, DeployPostgresAdminAppRequest{DatabaseID: adminAppDB, App: PostgresAdminAppPgAdmin}); err != nil {
		t.Fatalf("DeployPostgresAdminApp: %v", err)
	}
	apps := adminAppServices(t, cl)
	if len(apps) != 1 {
		t.Fatalf("admin app services = %d, want 1", len(apps))
	}
	a := apps[0]
	if a.Spec.Repo != pgAdminRepo || a.Spec.Branch != "main" || a.Spec.Tier != "starter" {
		t.Errorf("source %q@%q tier %q, want Render's repository on starter", a.Spec.Repo, a.Spec.Branch, a.Spec.Tier)
	}
	if a.Spec.Disk == nil || a.Spec.Disk.MountPath != pgAdminMount || a.Spec.Disk.SizeGB != 1 {
		t.Errorf("disk = %+v, want 1 GB at %s", a.Spec.Disk, pgAdminMount)
	}
	if got := seeder.env["PGADMIN_DEFAULT_EMAIL"]; got != "ada@example.com" {
		t.Errorf("PGADMIN_DEFAULT_EMAIL = %q, want the caller's email", got)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(seeder.env["PGADMIN_DEFAULT_PASSWORD"]) {
		t.Errorf("generated password = %q, want 32 hex characters", seeder.env["PGADMIN_DEFAULT_PASSWORD"])
	}
	for k, w := range map[string]string{"PORT": "10000", "PGADMIN_LISTEN_PORT": "10000", "PGADMIN_SERVER_JSON_FILE": pgAdminServer, "PGADMIN_DATABASE_HOST": adminAppDB + "-rw.tea-a.svc"} {
		if got := seeder.env[k]; got != w {
			t.Errorf("env %s = %q, want %q", k, got, w)
		}
	}
	files := map[string]string{}
	for _, f := range seeder.files {
		files[f.Name] = f.Content
	}
	if got, want := files["pgpassfile"], adminAppDB+`-rw.tea-a.svc:5432:*:orders_user:p\:ss\\word`; got != want {
		t.Errorf("pgpassfile = %q, want %q (':' and '\\' escaped for libpq)", got, want)
	}
	var servers struct {
		Servers map[string]struct {
			Name, Host, Username, PassFile, MaintenanceDB string
			Port                                          int
		}
	}
	if err := json.Unmarshal([]byte(files["servers.json"]), &servers); err != nil {
		t.Fatalf("servers.json: %v", err)
	}
	server := servers.Servers["1"]
	if server.Host != adminAppDB+"-rw.tea-a.svc" || server.Port != 5432 || server.Username != "orders_user" || server.PassFile != "pgpassfile" || server.Name != "orders_db" {
		t.Errorf("servers.json server = %+v, want the database's private host, user and passfile", server)
	}
}

// Render's generator: `<app>-` and 8 hex characters, and 32 hex characters.
func TestDeployGeneratesRendersCredentials(t *testing.T) {
	svc, _, seeder, ctx := adminAppFixture(t, nil)
	if _, err := svc.DeployPostgresAdminApp(ctx, DeployPostgresAdminAppRequest{DatabaseID: adminAppDB, App: PostgresAdminAppPgHero}); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^pghero-[0-9a-f]{8}$`).MatchString(seeder.env["PGHERO_USERNAME"]) {
		t.Errorf("username = %q, want pghero-<8 hex>", seeder.env["PGHERO_USERNAME"])
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(seeder.env["PGHERO_PASSWORD"]) {
		t.Errorf("password = %q, want 32 hex characters", seeder.env["PGHERO_PASSWORD"])
	}
}

// The Apps page lists Render's catalog; a deployed app answers its service
// until that service is deleted, and a second app of a kind is refused.
func TestPostgresAdminAppsFollowTheDeployedService(t *testing.T) {
	svc, cl, _, ctx := adminAppFixture(t, nil)
	apps, err := svc.PostgresAdminApps(ctx, adminAppDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 || apps[0].ID != PostgresAdminAppPgAdmin || apps[1].ID != PostgresAdminAppPgHero {
		t.Fatalf("catalog = %+v, want pgAdmin then PgHero", apps)
	}
	if apps[0].Plan != "starter" || apps[0].DiskSizeGB != 1 || apps[1].Plan != "free" || apps[1].DiskSizeGB != 0 {
		t.Errorf("billing shape = %+v", apps)
	}
	// The dialog's billing lines come from the price sheet: Render's $7 and
	// $0.25 are bex's $4.90 and $0.175.
	if apps[0].MonthlyUSD != "4.90" || apps[0].DiskMonthlyUSD != "0.175" || apps[1].MonthlyUSD != "0.00" || apps[1].DiskMonthlyUSD != "" {
		t.Errorf("billing lines = pgAdmin %q + %q, PgHero %q + %q", apps[0].MonthlyUSD, apps[0].DiskMonthlyUSD, apps[1].MonthlyUSD, apps[1].DiskMonthlyUSD)
	}
	for _, a := range apps {
		if a.Service != nil {
			t.Fatalf("%s has a service before any deploy", a.ID)
		}
	}
	created, err := svc.DeployPostgresAdminApp(ctx, DeployPostgresAdminAppRequest{DatabaseID: adminAppDB, App: PostgresAdminAppPgHero})
	if err != nil {
		t.Fatal(err)
	}
	apps, err = svc.PostgresAdminApps(ctx, adminAppDB)
	if err != nil {
		t.Fatal(err)
	}
	if apps[1].Service == nil || apps[1].Service.ID != created.ID || apps[0].Service != nil {
		t.Fatalf("after deploying PgHero: pgAdmin %+v, PgHero %+v", apps[0].Service, apps[1].Service)
	}

	_, err = svc.DeployPostgresAdminApp(ctx, DeployPostgresAdminAppRequest{DatabaseID: adminAppDB, App: PostgresAdminAppPgHero})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != postgresAdminAppConflictCode {
		t.Fatalf("second PgHero = %v, want %s", err, postgresAdminAppConflictCode)
	}
	if n := len(adminAppServices(t, cl)); n != 1 {
		t.Fatalf("a refused second app left %d services", n)
	}

	deployed := adminAppServices(t, cl)[0]
	if err := cl.Delete(ctx, &deployed); err != nil {
		t.Fatal(err)
	}
	apps, err = svc.PostgresAdminApps(ctx, adminAppDB)
	if err != nil {
		t.Fatal(err)
	}
	if apps[1].Service != nil {
		t.Fatalf("a deleted PgHero is still deployed: %+v", apps[1].Service)
	}
}

// The app receives the database's credential: a caller without
// can_view_sensitive gets the shared forbidden error and nothing is created,
// though it may create services and read the page.
func TestDeployPostgresAdminAppNeedsSensitiveView(t *testing.T) {
	checker := &relationChecker{allow: map[string]bool{core.RelCanView: true, core.RelCanCreate: true, core.RelCanOperate: true}}
	svc, cl, _, ctx := adminAppFixture(t, checker)
	if _, err := svc.PostgresAdminApps(ctx, adminAppDB); err != nil {
		t.Fatalf("a viewer reads the Apps page: %v", err)
	}
	_, err := svc.DeployPostgresAdminApp(ctx, DeployPostgresAdminAppRequest{DatabaseID: adminAppDB, App: PostgresAdminAppPgHero})
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("deploy without can_view_sensitive = %v, want ErrForbidden", err)
	}
	if n := len(adminAppServices(t, cl)); n != 0 {
		t.Fatalf("a refused deploy created %d services", n)
	}
}

// A database with no private address or credential yet has nothing to
// connect an app to.
func TestDeployPostgresAdminAppWaitsForTheDatabase(t *testing.T) {
	svc, cl, _, ctx := adminAppFixture(t, nil)
	var db appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "tea-a", Name: adminAppDB}, &db); err != nil {
		t.Fatal(err)
	}
	db.Status.Host = ""
	if err := cl.Status().Update(ctx, &db); err != nil {
		if err := cl.Update(ctx, &db); err != nil {
			t.Fatal(err)
		}
	}
	_, err := svc.DeployPostgresAdminApp(ctx, DeployPostgresAdminAppRequest{DatabaseID: adminAppDB, App: PostgresAdminAppPgHero})
	if !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("deploy before the database is ready = %v, want ErrUnavailable", err)
	}
	if n := len(adminAppServices(t, cl)); n != 0 {
		t.Fatalf("a refused deploy created %d services", n)
	}
}

func TestDeployPostgresAdminAppRefusesAnUnknownApp(t *testing.T) {
	svc, _, _, ctx := adminAppFixture(t, nil)
	_, err := svc.DeployPostgresAdminApp(ctx, DeployPostgresAdminAppRequest{DatabaseID: adminAppDB, App: "adminer"})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "INVALID_POSTGRES_ADMIN_APP" {
		t.Fatalf("unknown app = %v, want INVALID_POSTGRES_ADMIN_APP", err)
	}
}

// protectedEnvironments refuses every placement as a protected environment
// refuses a non-Admin's (core.ProtectedPlacementRefusal).
type protectedEnvironments struct{ fixedEnvironmentResolver }

func (protectedEnvironments) RequirePlacement(_ context.Context, a core.EnvironmentAssignment) error {
	return core.ProtectedPlacementRefusal(a.ID)
}

// In a protected environment only an Admin creates resources, as on Render:
// a Developer's deploy is the shared create's coded refusal, and nothing is
// created.
func TestDeployPostgresAdminAppInAProtectedEnvironmentNeedsAnAdmin(t *testing.T) {
	svc, cl, _, ctx := adminAppFixture(t, nil)
	svc.Environments = protectedEnvironments{svc.Environments.(fixedEnvironmentResolver)}
	_, err := svc.DeployPostgresAdminApp(ctx, DeployPostgresAdminAppRequest{DatabaseID: adminAppDB, App: PostgresAdminAppPgHero})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.ProtectedPlacementCode {
		t.Fatalf("deploy into a protected environment = %v, want %s", err, core.ProtectedPlacementCode)
	}
	if n := len(adminAppServices(t, cl)); n != 0 {
		t.Fatalf("a refused deploy created %d services", n)
	}
}

func TestPgPassLineEscapesLibpqSeparators(t *testing.T) {
	got := pgPassLine(postgres.InternalConnection{Host: "h", User: `u:1`, Password: `p\:w`})
	if want := `h:5432:*:u\:1:p\\\:w`; got != want {
		t.Fatalf("pgPassLine = %q, want %q", got, want)
	}
}
