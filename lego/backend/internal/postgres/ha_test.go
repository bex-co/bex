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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// allowHighAvailabilityOnEveryPlan lets a test exercise the HA-enable path:
// the shipped catalog has no ≥1-CPU Postgres plan yet (w8/m43).
func allowHighAvailabilityOnEveryPlan(t *testing.T) {
	t.Helper()
	prev := PlanSupportsHighAvailability
	PlanSupportsHighAvailability = func(tiers.PostgresTier) bool { return true }
	t.Cleanup(func() { PlanSupportsHighAvailability = prev })
}

func TestHACreate(t *testing.T) {
	allowHighAvailabilityOnEveryPlan(t)
	svc, cl := newService()
	// Create with enableHighAvailability — verify the CR spec intent is wired.
	w := serveREST(svc, "POST", "/v1/postgres",
		`{"name":"ha-db","plan":"basic-1gb","enableHighAvailability":true}`)
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d: %s", w.Code, w.Body.String())
	}
	var view PostgresView
	_ = json.Unmarshal(w.Body.Bytes(), &view)
	var cr appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: view.ID}, &cr); err != nil {
		t.Fatalf("CR not created: %v", err)
	}
	if !cr.Spec.HighAvailability {
		t.Error("spec.highAvailability want true")
	}
}

// TestHARefusedBelowOneCPU pins Render's plan rule on every service write path
// (w8/m43): create, PATCH, the plan-only SetPlan, and their dry runs refuse HA
// on a sub-1-CPU plan with a 400 naming the requirement, and nothing is written.
func TestHARefusedBelowOneCPU(t *testing.T) {
	const want = "high availability requires a Postgres plan with at least 1 CPU"
	for _, plan := range []string{"", "free", "basic-256mb", "basic-1gb", "0.5c-1g"} {
		svc, cl := newService()
		w := serveREST(svc, "POST", "/v1/postgres",
			`{"name":"ha-db","plan":"`+plan+`","enableHighAvailability":true}`)
		if w.Code != 400 || !strings.Contains(w.Body.String(), want) || !strings.Contains(w.Body.String(), "POSTGRES_HA_PLAN_UNSUPPORTED") {
			t.Errorf("create plan %q + HA => want 400 %q, got %d: %s", plan, want, w.Code, w.Body.String())
		}
		var list appv1alpha1.DatabaseList
		if err := cl.List(context.Background(), &list); err != nil || len(list.Items) != 0 {
			t.Errorf("create plan %q + HA wrote %d databases (err %v)", plan, len(list.Items), err)
		}
	}

	svc, cl := newService()
	seedDatabase(t, cl, "free-db")
	for _, body := range []string{`{"enableHighAvailability":true}`, `{"enableHighAvailability":true,"plan":"basic-1gb"}`} {
		w := serveREST(svc, "PATCH", "/v1/postgres/free-db", body)
		if w.Code != 400 || !strings.Contains(w.Body.String(), want) {
			t.Errorf("PATCH %s => want 400 %q, got %d: %s", body, want, w.Code, w.Body.String())
		}
	}
	enabled := true
	if _, err := svc.UpdatePostgresDryRun(context.Background(), "free-db", PostgresPatch{EnableHighAvailability: &enabled}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("UpdatePostgresDryRun HA on free => want ErrBadRequest, got %v", err)
	}
	var cr appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "free-db"}, &cr); err != nil {
		t.Fatal(err)
	}
	if cr.Spec.HighAvailability {
		t.Error("refused PATCH must leave spec.highAvailability false")
	}
}

// TestHAPlanChangeUnderHARefused: moving an HA database to a plan without HA is
// refused with the remedy — never by silently switching HA off — while
// disabling HA, doing both in one PATCH, and unrelated edits stay allowed.
func TestHAPlanChangeUnderHARefused(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "ha-db", appv1alpha1.DatabaseSpec{Plan: "basic-256mb", HighAvailability: true}, false)

	const remedy = "disable high availability first"
	w := serveREST(svc, "PATCH", "/v1/postgres/ha-db", `{"plan":"free"}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), remedy) {
		t.Errorf("PATCH plan under HA => want 400 %q, got %d: %s", remedy, w.Code, w.Body.String())
	}
	if _, err := svc.SetPlan(ctx, "ha-db", "basic-1gb"); err == nil || !strings.Contains(err.Error(), remedy) {
		t.Errorf("SetPlan under HA => want %q, got %v", remedy, err)
	}
	if _, err := svc.SetPlanDryRun(ctx, "ha-db", "free"); err == nil || !strings.Contains(err.Error(), remedy) {
		t.Errorf("SetPlanDryRun under HA => want %q, got %v", remedy, err)
	}

	// A database already running HA on an unsupported plan keeps it through an
	// unrelated edit and an idempotent re-enable: no silent conversion.
	if w := serveREST(svc, "PATCH", "/v1/postgres/ha-db", `{"name":"renamed-db"}`); w.Code != 200 {
		t.Errorf("rename under grandfathered HA => 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := serveREST(svc, "PATCH", "/v1/postgres/ha-db", `{"enableHighAvailability":true}`); w.Code != 200 {
		t.Errorf("idempotent HA re-enable => 200, got %d: %s", w.Code, w.Body.String())
	}
	var cr appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "ha-db"}, &cr); err != nil {
		t.Fatal(err)
	}
	if cr.Spec.Plan != "basic-256mb" || !cr.Spec.HighAvailability {
		t.Fatalf("refused plan changes must leave plan/HA alone, got plan %q HA %v", cr.Spec.Plan, cr.Spec.HighAvailability)
	}

	// Disabling HA with a storage-compatible downgrade in one PATCH is allowed.
	if w := serveREST(svc, "PATCH", "/v1/postgres/ha-db", `{"plan":"free","enableHighAvailability":false}`); w.Code != 200 {
		t.Fatalf("downgrade + HA off => 200, got %d: %s", w.Code, w.Body.String())
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "ha-db"}, &cr); err != nil {
		t.Fatal(err)
	}
	if cr.Spec.Plan != "free" || cr.Spec.HighAvailability {
		t.Errorf("downgrade + HA off => plan free HA false, got plan %q HA %v", cr.Spec.Plan, cr.Spec.HighAvailability)
	}
}

func TestHACreateWithReadReplicas(t *testing.T) {
	svc, cl := newService()
	w := serveREST(svc, "POST", "/v1/postgres",
		`{"name":"rep-db","plan":"basic-1gb","diskSizeGB":10,"readReplicas":[{"name":"reader-1"},{"name":"reader-2"}]}`)
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d: %s", w.Code, w.Body.String())
	}
	var view PostgresView
	_ = json.Unmarshal(w.Body.Bytes(), &view)
	var cr appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: view.ID}, &cr); err != nil {
		t.Fatalf("CR not created: %v", err)
	}
	if len(cr.Spec.ReadReplicas) != 2 {
		t.Fatalf("spec.readReplicas = %d, want 2", len(cr.Spec.ReadReplicas))
	}
	if cr.Spec.ReadReplicas[0].Name != "reader-1" || cr.Spec.ReadReplicas[1].Name != "reader-2" {
		t.Errorf("readReplica names = %v", cr.Spec.ReadReplicas)
	}
}

func TestReadReplicaView(t *testing.T) {
	// pgView reads ReadReplicaStatuses from the operator and projects them to
	// ReadReplicaView entries. HighAvailabilityEnabled comes from operator status.
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "rr-db", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "basic-1gb", HighAvailability: true},
		Status: appv1alpha1.DatabaseStatus{
			Phase:                   appv1alpha1.DBPhaseReady,
			HighAvailabilityEnabled: true,
			ReadReplicaStatuses: []appv1alpha1.DatabaseReadReplicaStatus{
				{Name: "reader-1", InternalHost: "rr-db-ro.default.svc", ExternalHost: "rr-db-ro-reader-1.db.bex.co"},
				{Name: "reader-2", InternalHost: "rr-db-ro.default.svc"},
			},
		},
	}

	v := pgView(db)

	if !v.HighAvailabilityEnabled {
		t.Error("highAvailabilityEnabled want true")
	}
	if len(v.ReadReplicas) != 2 {
		t.Fatalf("readReplicas = %d, want 2", len(v.ReadReplicas))
	}
	r0 := v.ReadReplicas[0]
	if r0.ID != "rr-db-ro-reader-1" {
		t.Errorf("replica[0] id = %q, want rr-db-ro-reader-1 (w5/066)", r0.ID)
	}
	if r0.Name != "reader-1" || r0.ConnectionInfo == nil {
		t.Errorf("replica[0] = %+v", r0)
	}
	if again := pgView(db); again.ReadReplicas[0].ID != r0.ID {
		t.Errorf("read replica id must be stable across reads: %q vs %q", again.ReadReplicas[0].ID, r0.ID)
	}
	if r0.ConnectionInfo.InternalHost != "rr-db-ro.default.svc" {
		t.Errorf("replica[0] internalHost = %q", r0.ConnectionInfo.InternalHost)
	}
	if r0.ConnectionInfo.ExternalHost != "rr-db-ro-reader-1.db.bex.co" {
		t.Errorf("replica[0] externalHost = %q", r0.ConnectionInfo.ExternalHost)
	}
	// Reader-2 has only internal host — external should be empty.
	r1 := v.ReadReplicas[1]
	if r1.Name != "reader-2" || r1.ConnectionInfo == nil {
		t.Errorf("replica[1] = %+v", r1)
	}
	if r1.ConnectionInfo.ExternalHost != "" {
		t.Errorf("replica[1] externalHost want empty, got %q", r1.ConnectionInfo.ExternalHost)
	}
}

func TestFailoverREST(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "fo-db")

	// POST /failover => 202, no body — Render's documented contract.
	w := serveREST(svc, "POST", "/v1/postgres/fo-db/failover", "")
	if w.Code != 202 {
		t.Fatalf("failover => 202, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Errorf("failover body want empty, got %q", w.Body.String())
	}

	// spec.failoverAt must be stamped (operator picks it up for the switchover).
	var cr appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "fo-db"}, &cr); err != nil {
		t.Fatalf("reading CR after failover: %v", err)
	}
	if cr.Spec.FailoverAt == "" {
		t.Error("spec.failoverAt want RFC3339 timestamp after failover, got empty")
	}
}

func TestReadReplicaConnectionInfo(t *testing.T) {
	// PostgresConnectionInfo includes per-replica full connection strings when
	// ReadReplicaStatuses are present in the Database status.
	svc, cl := newService()

	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "rci-db", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "basic-1gb"},
		Status: appv1alpha1.DatabaseStatus{
			Phase:      appv1alpha1.DBPhaseReady,
			SecretName: "rci-db-app",
			ReadReplicaStatuses: []appv1alpha1.DatabaseReadReplicaStatus{
				{Name: "reader-a", InternalHost: "rci-db-ro.default.svc", ExternalHost: "rci-db-ro-reader-a.db.bex.co"},
				{Name: "reader-b", InternalHost: "rci-db-ro.default.svc"},
			},
		},
	}
	if err := cl.Create(context.Background(), db); err != nil {
		t.Fatalf("create db: %v", err)
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "rci-db-app", Namespace: "default"},
		Data: map[string][]byte{
			"username": []byte("rci_db_user"),
			"password": []byte("pw"),
			"dbname":   []byte("rci_db"),
			"uri":      []byte("postgresql://rci_db_user:pw@rci-db-rw.default:5432/rci_db"),
		},
	}
	if err := cl.Create(context.Background(), sec); err != nil {
		t.Fatalf("create secret: %v", err)
	}

	w := serveREST(svc, "GET", "/v1/postgres/rci-db/connection-info", "")
	if w.Code != 200 {
		t.Fatalf("connection-info => 200, got %d: %s", w.Code, w.Body.String())
	}
	var ci PostgresConnectionInfo
	_ = json.Unmarshal(w.Body.Bytes(), &ci)

	if len(ci.ReadReplicaConnectionStrings) != 2 {
		t.Fatalf("readReplicaConnectionStrings = %d, want 2", len(ci.ReadReplicaConnectionStrings))
	}
	ra := ci.ReadReplicaConnectionStrings[0]
	if ra.Name != "reader-a" {
		t.Errorf("replica[0] name = %q, want reader-a", ra.Name)
	}
	wantInternal := "postgresql://rci_db_user:pw@rci-db-ro.default.svc:5432/rci_db"
	if ra.InternalConnectionString != wantInternal {
		t.Errorf("replica[0] internal = %q, want %q", ra.InternalConnectionString, wantInternal)
	}
	wantExternal := "postgresql://rci_db_user:pw@rci-db-ro-reader-a.db.bex.co:5432/rci_db?sslmode=verify-full"
	if ra.ExternalConnectionString != wantExternal {
		t.Errorf("replica[0] external = %q, want %q", ra.ExternalConnectionString, wantExternal)
	}
	// reader-b has only internal host.
	rb := ci.ReadReplicaConnectionStrings[1]
	if rb.ExternalConnectionString != "" {
		t.Errorf("replica[1] external want empty, got %q", rb.ExternalConnectionString)
	}
}
