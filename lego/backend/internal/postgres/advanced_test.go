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
	"encoding/json"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// newServiceCNPG builds a Service whose fake client also knows the CNPG Backup
// unstructured types, so the export create/list paths exercise real client calls.
func newServiceCNPG(objs ...client.Object) (*Service, client.Client) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(cnpgBackupGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "BackupList"},
		&unstructured.UnstructuredList{},
	)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Service{Base: &core.Base{Client: cl, Namespace: "default"}}, cl
}

// seedDatabaseSpec adds a Ready Database with the given spec + a matching -app Secret.
func seedDatabaseSpec(t *testing.T, cl client.Client, name string, spec appv1alpha1.DatabaseSpec, backupsEnabled bool) *appv1alpha1.Database {
	t.Helper()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       spec,
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, Host: name + "-rw.default.svc", Port: 5432,
			SecretName: name + "-app", BackupsEnabled: backupsEnabled,
		},
	}
	dbn := spec.EffectiveDatabaseName(name)
	dbUser := spec.EffectiveDatabaseUser(name)
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-app", Namespace: "default"},
		Data: map[string][]byte{
			"username": []byte(dbUser), "password": []byte("s3cret"),
			"dbname": []byte(dbn),
			"uri":    []byte("postgresql://" + dbn + "_user:s3cret@" + name + "-rw.default:5432/" + dbn),
		},
	}
	if err := cl.Create(context.Background(), db); err != nil {
		t.Fatalf("seed db: %v", err)
	}
	if err := cl.Create(context.Background(), sec); err != nil {
		t.Fatalf("seed secret: %v", err)
	}
	return db
}

// --- lifecycle ---

func TestLifecycleVerbs(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "life-db")
	ctx := context.Background()

	if v, err := svc.Suspend(ctx, "life-db"); err != nil || v.Suspended != core.RenderSuspended {
		t.Fatalf("suspend => %v suspended=%q", err, v.Suspended)
	}
	var got appv1alpha1.Database
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "life-db"}, &got)
	if !got.Spec.Suspended {
		t.Fatal("suspend did not set spec.suspended")
	}

	if v, err := svc.Resume(ctx, "life-db"); err != nil || v.Suspended != core.RenderNotSuspended {
		t.Fatalf("resume => %v suspended=%q", err, v.Suspended)
	}
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "life-db"}, &got)
	if got.Spec.Suspended {
		t.Fatal("resume did not clear spec.suspended")
	}

	if _, err := svc.Restart(ctx, "life-db"); err != nil {
		t.Fatalf("restart => %v", err)
	}
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "life-db"}, &got)
	if got.Spec.RestartedAt == "" {
		t.Fatal("restart did not stamp spec.restartedAt")
	}
}

func TestRESTLifecycleStatusCodes(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "rest-life")
	for _, c := range []struct {
		verb string
		code int
	}{{"suspend", 202}, {"resume", 202}, {"restart", 200}} {
		if got := serveREST(svc, "POST", "/v1/postgres/rest-life/"+c.verb, "").Code; got != c.code {
			t.Errorf("%s => %d, want %d", c.verb, got, c.code)
		}
	}
}

// --- recovery / exports ---

func TestRecoveryInfoDisabledForFreePlan(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "free-db", appv1alpha1.DatabaseSpec{Plan: "free"}, false)
	info, err := svc.RecoveryInfo(context.Background(), "free-db")
	if err != nil {
		t.Fatalf("RecoveryInfo => %v", err)
	}
	if info.Enabled {
		t.Fatal("free plan should report recovery disabled, not error")
	}
}

func TestRecoveryInfoEnabled(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "paid-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
	info, err := svc.RecoveryInfo(context.Background(), "paid-db")
	if err != nil {
		t.Fatalf("RecoveryInfo => %v", err)
	}
	if !info.Enabled || info.LatestRecoveryTime == "" {
		t.Fatalf("enabled recovery should report a window: %+v", info)
	}
}

func TestRecoverCreatesNewInstanceLeavingSourceUntouched(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "src-db", appv1alpha1.DatabaseSpec{
		Plan: "basic-1gb", Version: "16", DatabaseName: "orders_data", DatabaseUser: "orders_owner",
	}, true)
	ctx := context.Background()

	v, err := svc.Recover(ctx, "src-db", RecoverRequest{Name: "restored-db", TargetTime: "2026-07-09T10:00:00Z"})
	if err != nil {
		t.Fatalf("Recover => %v", err)
	}
	if !strings.HasPrefix(v.ID, "dpg-") || v.Name != "restored-db" {
		t.Fatalf("recover returned %+v, want a dpg-... id and name restored-db", v)
	}
	// New instance carries the recovery bootstrap + inherits the source's plan/version.
	var made appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: v.ID}, &made); err != nil {
		t.Fatalf("new db not created: %v", err)
	}
	if made.Spec.Name != "restored-db" {
		t.Fatalf("new db display name = %q, want restored-db", made.Spec.Name)
	}
	if made.Spec.Recovery == nil || made.Spec.Recovery.SourceDatabase != "src-db" ||
		made.Spec.Recovery.TargetTime != "2026-07-09T10:00:00Z" {
		t.Fatalf("recovery spec wrong: %+v", made.Spec.Recovery)
	}
	if made.Spec.Plan != "basic-1gb" || made.Spec.Version != "16" {
		t.Fatalf("new db should inherit source plan/version: %+v", made.Spec)
	}
	if made.Spec.DatabaseName != "orders_data" || made.Spec.DatabaseUser != "orders_owner" ||
		v.DatabaseName != "orders_data" || v.DatabaseUser != "orders_owner" {
		t.Fatalf("recovery did not preserve physical identity: spec=%+v view=%+v", made.Spec, v)
	}
	// Source is untouched (no recovery block).
	var src appv1alpha1.Database
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "src-db"}, &src)
	if src.Spec.Recovery != nil {
		t.Fatal("source database must not be modified by recover")
	}
}

func TestRecoverRejectsBadInput(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "nb-db", appv1alpha1.DatabaseSpec{Plan: "free"}, false) // no backups
	seedDatabaseSpec(t, cl, "ok-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
	ctx := context.Background()

	// no backups => bad request
	if _, err := svc.Recover(ctx, "nb-db", RecoverRequest{Name: "x"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("recover without backups should be ErrBadRequest, got %v", err)
	}
	// same name => bad request
	if _, err := svc.Recover(ctx, "ok-db", RecoverRequest{Name: "ok-db"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("recover into the source name should be ErrBadRequest, got %v", err)
	}
	// bad targetTime => bad request
	if _, err := svc.Recover(ctx, "ok-db", RecoverRequest{Name: "y", TargetTime: "not-a-time"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("recover with bad targetTime should be ErrBadRequest, got %v", err)
	}
}

func TestExportsCreateAndList(t *testing.T) {
	svc, cl := newServiceCNPG()
	seedDatabaseSpec(t, cl, "exp-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
	seedDatabaseSpec(t, cl, "free-db", appv1alpha1.DatabaseSpec{Plan: "free"}, false)
	ctx := context.Background()

	// No backups => export unavailable.
	if _, err := svc.CreateExport(ctx, "free-db"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("export without backups should be ErrBadRequest, got %v", err)
	}

	exp, err := svc.CreateExport(ctx, "exp-db")
	if err != nil {
		t.Fatalf("CreateExport => %v", err)
	}
	if k, ok := id.KindOf(exp.ID); !ok || k.Prefix() != "exp" {
		t.Fatalf("export id %q is not a well-formed exp- id", exp.ID)
	}
	// The Backup lands, labeled to this database, and ListExports finds it.
	list, err := svc.ListExports(ctx, "exp-db")
	if err != nil {
		t.Fatalf("ListExports => %v", err)
	}
	if len(list) != 1 || list[0].ID != exp.ID {
		t.Fatalf("ListExports = %+v, want the created export", list)
	}
	// Another database's exports don't leak in.
	if other, _ := svc.ListExports(ctx, "free-db"); len(other) != 0 {
		t.Fatalf("free-db should have no exports, got %+v", other)
	}
}

// --- access: IP allowlist ---

func TestIPAllowList(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "acl-db", appv1alpha1.DatabaseSpec{Plan: "free", Public: true}, false)
	ctx := context.Background()

	// invalid CIDR rejected before any write
	if _, err := svc.SetIPAllowList(ctx, "acl-db", []core.IPAllowListEntry{{CIDRBlock: "nonsense"}}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("bad CIDR should be ErrBadRequest, got %v", err)
	}
	var db appv1alpha1.Database
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "acl-db"}, &db)
	if len(db.Spec.IPAllowList) != 0 {
		t.Fatal("a rejected allowlist must not be written")
	}

	// the create-time seed goes through the same gate (core.ValidateCIDRs)
	if _, err := svc.CreatePostgres(ctx, CreatePostgresRequest{Name: "acl-bad", IPAllowList: []core.IPAllowListEntry{{CIDRBlock: "nonsense"}}}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("create with bad CIDR should be ErrBadRequest, got %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "acl-bad"}, &db); err == nil {
		t.Fatal("a rejected create must not write the CR")
	}

	if _, err := svc.SetIPAllowList(ctx, "acl-db", []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}, {CIDRBlock: "10.0.0.0/8"}}); err != nil {
		t.Fatalf("SetIPAllowList => %v", err)
	}
	got, err := svc.GetIPAllowList(ctx, "acl-db")
	if err != nil || len(got) != 2 || got[0].CIDRBlock != "203.0.113.0/24" || got[0].Description != "office" {
		t.Fatalf("GetIPAllowList = %v (err %v)", got, err)
	}
	// empty clears it
	if _, err := svc.SetIPAllowList(ctx, "acl-db", nil); err != nil {
		t.Fatalf("clear allowlist => %v", err)
	}
	if got, _ := svc.GetIPAllowList(ctx, "acl-db"); len(got) != 0 {
		t.Fatalf("cleared allowlist should be empty, got %v", got)
	}
}

// --- access: users ---

func TestUsersCRUD(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "usr-db", appv1alpha1.DatabaseSpec{Plan: "free"}, false)
	ctx := context.Background()

	// invalid role name rejected
	if _, err := svc.CreateUser(ctx, "usr-db", "Bad-Name"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("invalid role name should be ErrBadRequest, got %v", err)
	}

	res, err := svc.CreateUser(ctx, "usr-db", "reporting")
	if err != nil || res.Name != "reporting" || res.Password == "" {
		t.Fatalf("CreateUser => %+v err=%v", res, err)
	}
	// spec.users records the role + its secret; the secret carries the password.
	var db appv1alpha1.Database
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "usr-db"}, &db)
	if len(db.Spec.Users) != 1 || db.Spec.Users[0].Name != "reporting" {
		t.Fatalf("spec.users wrong: %+v", db.Spec.Users)
	}
	var sec corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "usr-db-user-reporting"}, &sec); err != nil {
		t.Fatalf("user secret missing: %v", err)
	}
	if string(sec.Data["password"]) != res.Password {
		t.Fatal("user secret password mismatch")
	}

	// duplicate rejected
	if _, err := svc.CreateUser(ctx, "usr-db", "reporting"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("duplicate user should be ErrBadRequest, got %v", err)
	}

	users, _ := svc.ListUsers(ctx, "usr-db")
	if len(users) != 1 || users[0].Name != "reporting" {
		t.Fatalf("ListUsers = %+v", users)
	}

	if err := svc.DeleteUser(ctx, "usr-db", "reporting"); err != nil {
		t.Fatalf("DeleteUser => %v", err)
	}
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "usr-db"}, &db)
	if len(db.Spec.Users) != 0 {
		t.Fatal("DeleteUser did not remove the role from spec.users")
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "usr-db-user-reporting"}, &sec); err == nil {
		t.Fatal("DeleteUser did not delete the user secret")
	}
	if err := svc.DeleteUser(ctx, "usr-db", "ghost"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("deleting an unknown user should be ErrNotFound, got %v", err)
	}
}

// --- access: pooler connection strings ---

func TestPoolerConnectionStrings(t *testing.T) {
	svc, cl := newService()
	ctx := context.Background()
	// Seed a pooled, public, Ready database with its pooler status hosts set — the
	// operator surfaces these once the CNPG Pooler + route are projected.
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "pool-db", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "basic-1gb", Public: true, Pooler: true},
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, Host: "pool-db-rw.default.svc", Port: 5432,
			SecretName:         "pool-db-app",
			ExternalHost:       "pool-db.db.bex.co",
			PoolerHost:         "pool-db-pooler.default.svc",
			PoolerExternalHost: "pool-db-pool.db.bex.co",
		},
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pool-db-app", Namespace: "default"},
		Data: map[string][]byte{
			"username": []byte("pool_db_user"), "password": []byte("s3cret"),
			"dbname": []byte("pool_db"),
			"uri":    []byte("postgresql://pool_db_user:s3cret@pool-db-rw.default:5432/pool_db"),
		},
	}
	if err := cl.Create(ctx, db); err != nil {
		t.Fatalf("seed db: %v", err)
	}
	if err := cl.Create(ctx, sec); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	info, err := svc.PostgresConnectionInfo(ctx, "pool-db")
	if err != nil {
		t.Fatalf("PostgresConnectionInfo => %v", err)
	}
	if info.InternalConnectionPoolString == "" || info.ExternalConnectionPoolString == "" {
		t.Fatalf("pooler strings should be populated: %+v", info)
	}
	wantInternal := "postgresql://pool_db_user:s3cret@pool-db-pooler.default.svc:5432/pool_db"
	if info.InternalConnectionPoolString != wantInternal {
		t.Errorf("internal pool string = %q", info.InternalConnectionPoolString)
	}
}

// --- adapter shape smoke tests ---

func TestRESTRecoveryAndAccessShapes(t *testing.T) {
	svc, cl := newServiceCNPG()
	seedDatabaseSpec(t, cl, "shape-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb", Public: true}, true)

	// recovery-info uses Render's canonical POST.
	var info RecoveryInfoView
	_ = json.Unmarshal(serveREST(svc, "POST", "/v1/postgres/shape-db/recovery-info", "").Body.Bytes(), &info)
	if !info.Enabled {
		t.Error("recovery-info should be enabled for a backed-up db")
	}
	// recover => 201
	if serveREST(svc, "POST", "/v1/postgres/shape-db/recover", `{"name":"shape-restored"}`).Code != 201 {
		t.Error("recover => want 201")
	}
	// export create => Render's 202, list => 200
	if serveREST(svc, "POST", "/v1/postgres/shape-db/export", "").Code != 202 {
		t.Error("create export => want 202")
	}
	if serveREST(svc, "GET", "/v1/postgres/shape-db/export", "").Code != 200 {
		t.Error("list exports => want 200")
	}
	// ip-allow-list PUT => 200
	if serveREST(svc, "PUT", "/v1/postgres/shape-db/ip-allow-list", `{"cidrs":["10.0.0.0/8"]}`).Code != 200 {
		t.Error("put ip-allow-list => want 200")
	}
	// users create => 201 (returns password once), delete => 204
	rec := serveREST(svc, "POST", "/v1/postgres/shape-db/users", `{"name":"analytics"}`)
	if rec.Code != 201 {
		t.Fatalf("create user => want 201, got %d", rec.Code)
	}
	var ur CreateUserResult
	_ = json.Unmarshal(rec.Body.Bytes(), &ur)
	if ur.Password == "" {
		t.Error("create user should return a password")
	}
	if serveREST(svc, "DELETE", "/v1/postgres/shape-db/users/analytics", "").Code != 204 {
		t.Error("delete user => want 204")
	}
}
