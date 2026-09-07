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
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// postgresResourceScheme includes the unstructured Barman ObjectStore and CNPG
// Backup types exercised by recovery and export tests.
func postgresResourceScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(barmanCloudObjectStoreGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(cnpgBackupGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(cnpgBackupGVK.GroupVersion().WithKind("BackupList"), &unstructured.UnstructuredList{})
	return scheme
}

// newServicePostgresResources builds a Service backed by those resource types.
func newServicePostgresResources(objs ...client.Object) (*Service, client.Client) {
	cl := fake.NewClientBuilder().WithScheme(postgresResourceScheme()).WithObjects(objs...).Build()
	return &Service{Base: &core.Base{Client: cl, Namespace: "default"}}, cl
}

// seedBarmanRecoveryWindow creates the shared ObjectStore with one server's
// status. Non-empty bounds mark the PITR window as established; empty bounds
// leave it closed, the honest "backups enabled, none yet" state.
func seedBarmanRecoveryWindow(t *testing.T, cl client.Client, serverName, firstRecoverabilityPoint, lastSuccessfulBackupTime string) {
	t.Helper()
	store := &unstructured.Unstructured{}
	store.SetGroupVersionKind(barmanCloudObjectStoreGVK)
	store.SetNamespace("default")
	store.SetName(tenantBackupObjectStoreName)
	if firstRecoverabilityPoint != "" {
		if err := unstructured.SetNestedField(store.Object, firstRecoverabilityPoint, "status", "serverRecoveryWindow", serverName, "firstRecoverabilityPoint"); err != nil {
			t.Fatalf("set firstRecoverabilityPoint: %v", err)
		}
	}
	if lastSuccessfulBackupTime != "" {
		if err := unstructured.SetNestedField(store.Object, lastSuccessfulBackupTime, "status", "serverRecoveryWindow", serverName, "lastSuccessfulBackupTime"); err != nil {
			t.Fatalf("set lastSuccessfulBackupTime: %v", err)
		}
	}
	if err := cl.Create(context.Background(), store); err != nil {
		t.Fatalf("seed Barman ObjectStore: %v", err)
	}
}

// seedCNPGBackup creates a CNPG Backup labelled to its cluster (the intrinsic
// cnpg.io/cluster link listBackups selects on), with the given phase.
func seedCNPGBackup(t *testing.T, cl client.Client, clusterName, backupName, phase string) {
	t.Helper()
	b := &unstructured.Unstructured{}
	b.SetGroupVersionKind(cnpgBackupGVK)
	b.SetNamespace("default")
	b.SetName(backupName)
	b.SetLabels(map[string]string{labelCNPGCluster: clusterName})
	if phase != "" {
		if err := unstructured.SetNestedField(b.Object, phase, "status", "phase"); err != nil {
			t.Fatalf("set backup phase: %v", err)
		}
	}
	if err := cl.Create(context.Background(), b); err != nil {
		t.Fatalf("seed cnpg backup: %v", err)
	}
}

// seedDatabaseSpec adds a Ready Database with the given spec + a matching -app Secret.
func seedDatabaseSpec(t *testing.T, cl client.Client, name string, spec appv1alpha1.DatabaseSpec, backupsEnabled bool) *appv1alpha1.Database {
	t.Helper()
	if spec.Name == "" {
		spec.Name = name
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       spec,
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, Host: name + "-rw.default.svc", Port: 5432,
			SecretName: name + "-app", BackupsEnabled: backupsEnabled, BackupServerName: name,
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

// TestRecoveryInfoReportsAWindowWhenOneExists: with a readable Barman ObjectStore
// that has a complete recovery window and a completed Backup, the Recovery
// card's three facts agree — earliest, latest and the list all say backups exist.
func TestRecoveryInfoReportsAWindowWhenOneExists(t *testing.T) {
	svc, cl := newServicePostgresResources()
	db := seedDatabaseSpec(t, cl, "paid-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
	db.Status.BackupServerName = "tea-a-paid-db"
	if err := cl.Update(context.Background(), db); err != nil {
		t.Fatalf("set backup server name: %v", err)
	}
	seedBarmanRecoveryWindow(t, cl, "tea-a-paid-db", "2026-08-21T03:00:00Z", "2026-09-01T03:00:00Z")
	seedCNPGBackup(t, cl, "paid-db", "paid-db-backup-20260821030000", "completed")

	info, err := svc.RecoveryInfo(context.Background(), "paid-db")
	if err != nil {
		t.Fatalf("RecoveryInfo => %v", err)
	}
	if !info.Enabled {
		t.Fatal("a backed-up database should report recovery enabled")
	}
	if info.EarliestRecoveryTime != "2026-08-21T03:00:00Z" {
		t.Errorf("earliest = %q, want the ObjectStore's firstRecoverabilityPoint", info.EarliestRecoveryTime)
	}
	if info.LatestRecoveryTime != "2026-09-01T03:00:00Z" {
		t.Errorf("latest = %q, want the ObjectStore's lastSuccessfulBackupTime", info.LatestRecoveryTime)
	}
	if len(info.Backups) != 1 || info.Backups[0].Status != "completed" {
		t.Errorf("backups = %+v, want one completed backup", info.Backups)
	}
}

// TestRecoveryInfoNoWindowYetIsHonest pins the all-or-nothing window invariant:
// neither bound is reported until both authoritative bounds exist.
func TestRecoveryInfoNoWindowYetIsHonest(t *testing.T) {
	for _, tc := range []struct {
		name, start, end string
	}{
		{name: "empty status"},
		{name: "partial status", start: "2026-08-21T03:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newServicePostgresResources()
			seedDatabaseSpec(t, cl, "fresh-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
			seedBarmanRecoveryWindow(t, cl, "fresh-db", tc.start, tc.end)

			info, err := svc.RecoveryInfo(context.Background(), "fresh-db")
			if err != nil {
				t.Fatalf("RecoveryInfo => %v", err)
			}
			if !info.Enabled {
				t.Fatal("a backed-up plan should report recovery enabled")
			}
			if info.LatestRecoveryTime != "" || info.EarliestRecoveryTime != "" {
				t.Errorf("no complete window established, yet reported earliest=%q latest=%q",
					info.EarliestRecoveryTime, info.LatestRecoveryTime)
			}
			if len(info.Backups) != 0 {
				t.Errorf("backups = %+v, want none", info.Backups)
			}
		})
	}
}

// A failed Backup list must remain distinguishable from a verified empty list.
func TestRecoveryInfoUnreadableBackupsAreAnErrorNotEmpty(t *testing.T) {
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Group: "postgresql.cnpg.io", Resource: "backups"}, "",
		errors.New("RBAC: no access to backups"))
	cl := fake.NewClientBuilder().WithScheme(postgresResourceScheme()).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if list.GetObjectKind().GroupVersionKind().Group == cnpgBackupGVK.Group {
					return forbidden
				}
				return c.List(ctx, list, opts...)
			},
		}).Build()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	seedDatabaseSpec(t, cl, "unreadable-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
	seedBarmanRecoveryWindow(t, cl, "unreadable-db", "2026-08-21T03:00:00Z", "2026-09-01T03:00:00Z")

	if _, err := svc.RecoveryInfo(context.Background(), "unreadable-db"); err == nil {
		t.Fatal("a denied Backup list must return an error, not a false empty backup list")
	}
}

func TestRecoverCreatesNewInstanceLeavingSourceUntouched(t *testing.T) {
	svc, cl := newServicePostgresResources()
	seedDatabaseSpec(t, cl, "src-db", appv1alpha1.DatabaseSpec{
		Plan: "basic-1gb", Version: "16", DatabaseName: "orders_data", DatabaseUser: "orders_owner",
	}, true)
	seedBarmanRecoveryWindow(t, cl, "src-db", "2026-07-01T00:00:00Z", "2026-07-10T00:00:00Z")
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

// newServiceRecoveryWindowForbidden refuses Barman ObjectStore reads while
// leaving the rest of the fake client usable.
func newServiceRecoveryWindowForbidden() (*Service, client.Client) {
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Group: "barmancloud.cnpg.io", Resource: "objectstores"}, "",
		errors.New("RBAC: no access to objectstores"))
	cl := fake.NewClientBuilder().WithScheme(postgresResourceScheme()).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if obj.GetObjectKind().GroupVersionKind() == barmanCloudObjectStoreGVK {
					return forbidden
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).Build()
	return &Service{Base: &core.Base{Client: cl, Namespace: "default"}}, cl
}

// A failed ObjectStore read must remain distinguishable from a verified window
// whose server entry is empty.
func TestRecoveryInfoUnreadableObjectStoreIsAnErrorNotEmpty(t *testing.T) {
	svc, cl := newServiceRecoveryWindowForbidden()
	seedDatabaseSpec(t, cl, "blind-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)

	if _, err := svc.RecoveryInfo(context.Background(), "blind-db"); err == nil {
		t.Fatal("a denied ObjectStore read must return an error, not an empty recovery window")
	}
}

func TestRecoveryInfoMissingObjectStoreIsAnErrorNotEmpty(t *testing.T) {
	svc, cl := newServicePostgresResources()
	seedDatabaseSpec(t, cl, "misprojected-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)

	if _, err := svc.RecoveryInfo(context.Background(), "misprojected-db"); err == nil {
		t.Fatal("a missing shared ObjectStore must return an error, not an empty recovery window")
	}
}

// TestRecoverRefusesAnUnsubstantiatedWindow — w6/m117 t003: BackupsEnabled says
// backups are configured, not that anything restorable exists. Without an
// established ObjectStore recovery window, Recover must refuse — before any
// billing gate and before anything billable is created — and an unreadable
// window must surface as the read's error, never as "no backups" or a pass.
func TestRecoverRefusesAnUnsubstantiatedWindow(t *testing.T) {
	ctx := context.Background()
	sourceOnly := func(t *testing.T, cl client.Client) {
		t.Helper()
		var dbs appv1alpha1.DatabaseList
		if err := cl.List(ctx, &dbs); err != nil {
			t.Fatalf("list databases: %v", err)
		}
		if len(dbs.Items) != 1 {
			t.Fatalf("a refused recover left %d databases, want the source alone", len(dbs.Items))
		}
	}

	t.Run("missing shared ObjectStore is an error before the billing gates", func(t *testing.T) {
		svc, cl := newServicePostgresResources()
		src := seedDatabaseSpec(t, cl, "young-db", appv1alpha1.DatabaseSpec{Name: "young", Plan: "basic-1gb"}, true)
		src.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelWorkspace: "tea-a"}
		if err := cl.Update(ctx, src); err != nil {
			t.Fatalf("label source: %v", err)
		}
		svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
		gate := &rejectingPaymentGate{}
		svc.Payment = gate

		_, err := svc.Recover(ctxAs("user-a"), "young-db", RecoverRequest{Name: "restored"})
		if err == nil || errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("recover with a missing ObjectStore => %v, want a non-BadRequest read error", err)
		}
		if len(gate.calls) != 0 {
			t.Fatalf("the refusal must precede the billing gates; the payment gate saw %v", gate.calls)
		}
		sourceOnly(t, cl)
	})

	t.Run("ObjectStore present but window not open", func(t *testing.T) {
		svc, cl := newServicePostgresResources()
		seedDatabaseSpec(t, cl, "fresh-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
		seedBarmanRecoveryWindow(t, cl, "fresh-db", "", "")

		if _, err := svc.Recover(ctx, "fresh-db", RecoverRequest{Name: "restored"}); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("recover before the first recoverability point => %v, want ErrBadRequest", err)
		}
		sourceOnly(t, cl)
	})

	t.Run("an unreadable window is an error, not a refusal or a pass", func(t *testing.T) {
		svc, cl := newServiceRecoveryWindowForbidden()
		seedDatabaseSpec(t, cl, "blind-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)

		_, err := svc.Recover(ctx, "blind-db", RecoverRequest{Name: "restored"})
		if err == nil || errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("an unreadable window => %v, want a non-BadRequest error", err)
		}
		sourceOnly(t, cl)
	})
}

func TestExportsCreateAndList(t *testing.T) {
	svc, cl := newServicePostgresResources()
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
	if _, err := svc.CreateUser(ctx, "usr-db", strings.Repeat("a", 64)); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("overlength role name should be ErrBadRequest, got %v", err)
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
	if db.Spec.Users[0].SecretName == "usr-db-user-reporting" {
		t.Fatal("CreateUser reused the legacy deterministic Secret name")
	}
	var sec corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: db.Spec.Users[0].SecretName}, &sec); err != nil {
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
	// A revoke must leave an ensure:absent tombstone (spec.deletedUsers) so the
	// operator drops the live PostgreSQL role — removing it from spec.users alone
	// only stops CNPG managing it, leaving the login valid (codex #8/#2).
	if len(db.Spec.DeletedUsers) != 1 || db.Spec.DeletedUsers[0] != "reporting" {
		t.Fatalf("DeleteUser did not record the drop tombstone: %+v", db.Spec.DeletedUsers)
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: sec.Name}, &sec); err == nil {
		t.Fatal("DeleteUser did not delete the user secret")
	}
	if err := svc.DeleteUser(ctx, "usr-db", "ghost"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("deleting an unknown user should be ErrNotFound, got %v", err)
	}
}

func TestCreateUserReplacesLegacyCredentialAndClearsTombstone(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "rotate-db", appv1alpha1.DatabaseSpec{
		Plan:         "free",
		DeletedUsers: []string{"reporting", "retired"},
	}, false)
	ctx := context.Background()
	legacyName := "rotate-db-user-reporting"
	legacy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: legacyName, Namespace: "default"},
		Type:       corev1.SecretTypeBasicAuth,
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte("reporting"),
			corev1.BasicAuthPasswordKey: []byte("old-compromised-password"),
		},
	}
	if err := cl.Create(ctx, legacy); err != nil {
		t.Fatal(err)
	}

	result, err := svc.CreateUser(ctx, "rotate-db", "reporting")
	if err != nil {
		t.Fatalf("CreateUser recreation: %v", err)
	}
	if result.Password == "" || result.Password == "old-compromised-password" {
		t.Fatalf("CreateUser returned an invalid rotated password %q", result.Password)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(legacy), &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("legacy credential survived recreation: %v", err)
	}

	var db appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "rotate-db"}, &db); err != nil {
		t.Fatal(err)
	}
	if len(db.Spec.Users) != 1 || db.Spec.Users[0].Name != "reporting" || db.Spec.Users[0].SecretName == legacyName {
		t.Fatalf("recreated user = %+v", db.Spec.Users)
	}
	if len(db.Spec.DeletedUsers) != 1 || db.Spec.DeletedUsers[0] != "retired" {
		t.Fatalf("recreation tombstones = %v, want only retired", db.Spec.DeletedUsers)
	}
	var current corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: db.Spec.Users[0].SecretName}, &current); err != nil {
		t.Fatalf("rotated credential missing: %v", err)
	}
	if got := string(current.Data[corev1.BasicAuthPasswordKey]); got != result.Password {
		t.Fatalf("stored password does not match one-time response")
	}
}

func TestCreateUserPatchFailureCleansUnreferencedCredential(t *testing.T) {
	scheme := postgresResourceScheme()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "create-retry-db", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "create-retry-db", Plan: "free"},
	}
	failPatch := true
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if failPatch {
					failPatch = false
					return errors.New("transient Database patch failure")
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	ctx := context.Background()

	if result, err := svc.CreateUser(ctx, db.Name, "reporting"); err == nil || result.Password != "" {
		t.Fatalf("failed CreateUser = %+v, %v; want no credential response", result, err)
	}
	var secrets corev1.SecretList
	if err := cl.List(ctx, &secrets, client.InNamespace("default")); err != nil {
		t.Fatal(err)
	}
	if len(secrets.Items) != 0 {
		t.Fatalf("failed issuance left Secrets: %+v", secrets.Items)
	}
	var current appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKeyFromObject(db), &current); err != nil {
		t.Fatal(err)
	}
	if len(current.Spec.Users) != 0 {
		t.Fatalf("failed issuance changed spec.users: %+v", current.Spec.Users)
	}

	result, err := svc.CreateUser(ctx, db.Name, "reporting")
	if err != nil || result.Password == "" {
		t.Fatalf("CreateUser retry = %+v, %v", result, err)
	}
}

func TestConcurrentCreateUserReturnsOnlyReferencedCredential(t *testing.T) {
	scheme := postgresResourceScheme()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "concurrent-db", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "concurrent-db", Plan: "free"},
	}
	var patchMu sync.Mutex
	patches := 0
	releasePatches := make(chan struct{})
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patchMu.Lock()
				patches++
				if patches == 2 {
					close(releasePatches)
				}
				patchMu.Unlock()
				<-releasePatches
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	ctx := context.Background()

	type outcome struct {
		result CreateUserResult
		err    error
	}
	outcomes := make([]outcome, 2)
	var wg sync.WaitGroup
	for i := range outcomes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			outcomes[i].result, outcomes[i].err = svc.CreateUser(ctx, db.Name, "reporting")
		}()
	}
	wg.Wait()

	successes := 0
	var winner CreateUserResult
	for _, outcome := range outcomes {
		if outcome.err == nil {
			successes++
			winner = outcome.result
		} else if outcome.result.Password != "" {
			t.Fatalf("failed concurrent creator received a password: %+v, %v", outcome.result, outcome.err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent CreateUser successes = %d, outcomes=%+v", successes, outcomes)
	}
	var current appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKeyFromObject(db), &current); err != nil {
		t.Fatal(err)
	}
	if len(current.Spec.Users) != 1 {
		t.Fatalf("concurrent spec.users = %+v", current.Spec.Users)
	}
	var credential corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: current.Spec.Users[0].SecretName}, &credential); err != nil {
		t.Fatal(err)
	}
	if got := string(credential.Data[corev1.BasicAuthPasswordKey]); got != winner.Password {
		t.Fatal("winning response did not match the referenced credential")
	}
	var secrets corev1.SecretList
	if err := cl.List(ctx, &secrets, client.InNamespace("default")); err != nil {
		t.Fatal(err)
	}
	if len(secrets.Items) != 1 || secrets.Items[0].Name != credential.Name {
		t.Fatalf("losing concurrent credential was not cleaned up: %+v", secrets.Items)
	}
}

func TestDeleteUserSecretFailureLeavesRevocationRetryable(t *testing.T) {
	scheme := postgresResourceScheme()
	secretName := "delete-retry-db-user-generation"
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "delete-retry-db", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "delete-retry-db", Plan: "free",
			Users: []appv1alpha1.DatabaseUser{{Name: "reporting", SecretName: secretName}},
		},
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "default"}}
	failDelete := true
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db, secret).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				if failDelete && obj.GetName() == secretName {
					failDelete = false
					return errors.New("transient Secret delete failure")
				}
				return c.Delete(ctx, obj, opts...)
			},
		}).Build()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	ctx := context.Background()

	if err := svc.DeleteUser(ctx, db.Name, "reporting"); err == nil {
		t.Fatal("DeleteUser succeeded despite Secret deletion failure")
	}
	var current appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKeyFromObject(db), &current); err != nil {
		t.Fatal(err)
	}
	if len(current.Spec.Users) != 1 || len(current.Spec.DeletedUsers) != 0 {
		t.Fatalf("failed delete changed Database: users=%v tombstones=%v", current.Spec.Users, current.Spec.DeletedUsers)
	}
	if err := svc.DeleteUser(ctx, db.Name, "reporting"); err != nil {
		t.Fatalf("DeleteUser retry: %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(db), &current); err != nil {
		t.Fatal(err)
	}
	if len(current.Spec.Users) != 0 || len(current.Spec.DeletedUsers) != 1 || current.Spec.DeletedUsers[0] != "reporting" {
		t.Fatalf("completed delete: users=%v tombstones=%v", current.Spec.Users, current.Spec.DeletedUsers)
	}
}

func TestDeleteUserPatchFailureRemainsRetryable(t *testing.T) {
	scheme := postgresResourceScheme()
	secretName := "patch-retry-db-user-generation"
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "patch-retry-db", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "patch-retry-db", Plan: "free",
			Users: []appv1alpha1.DatabaseUser{{Name: "reporting", SecretName: secretName}},
		},
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "default"}}
	failPatch := true
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db, secret).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if failPatch {
					failPatch = false
					return errors.New("transient Database patch failure")
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	ctx := context.Background()

	if err := svc.DeleteUser(ctx, db.Name, "reporting"); err == nil {
		t.Fatal("DeleteUser succeeded despite Database patch failure")
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("credential survived the first revocation attempt: %v", err)
	}
	if err := svc.DeleteUser(ctx, db.Name, "reporting"); err != nil {
		t.Fatalf("DeleteUser retry after missing Secret: %v", err)
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
	seedDatabaseCA(t, cl, "pool-db")

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
	svc, cl := newServicePostgresResources()
	seedDatabaseSpec(t, cl, "shape-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb", Public: true}, true)
	seedBarmanRecoveryWindow(t, cl, "shape-db", "2026-08-01T00:00:00Z", "2026-08-02T00:00:00Z")

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
