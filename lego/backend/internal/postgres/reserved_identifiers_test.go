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
	"errors"
	"net/http"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m170: names PostgreSQL owns are refused before anything is written — a
// "postgres" owner or user wedged the reconcile, and "template1" handed the
// tenant a database it could not create a table in.
func TestReservedPostgresIdentifiersAreRefused(t *testing.T) {
	svc, cl := newService()
	for _, body := range []string{
		`{"name":"res-user","databaseUser":"postgres"}`,
		`{"name":"res-repl","databaseUser":"streaming_replica"}`,
		`{"name":"res-pg","databaseUser":"pg_monitor"}`,
		`{"name":"res-tpl1","databaseName":"template1"}`,
		`{"name":"res-tpl0","databaseName":"template0"}`,
		`{"name":"res-pgdb","databaseName":"postgres"}`,
	} {
		rec := serveREST(svc, http.MethodPost, "/v1/postgres", body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "POSTGRES_IDENTIFIER_RESERVED") {
			t.Errorf("POST %s = %d %s, want 400 POSTGRES_IDENTIFIER_RESERVED", body, rec.Code, rec.Body.String())
		}
	}
	// Controls still create.
	if _, err := svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "res-ok", DatabaseName: "orders_data", DatabaseUser: "qa_owner"}); err != nil {
		t.Fatalf("control create = %v", err)
	}

	seedDatabaseSpec(t, cl, "res-db", appv1alpha1.DatabaseSpec{Plan: "free"}, false)
	for _, role := range []string{"postgres", "streaming_replica", "pg_signal_backend"} {
		if _, err := svc.CreateUser(context.Background(), "res-db", role); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "reserved") {
			t.Errorf("CreateUser(%q) = %v, want a reserved-name 400", role, err)
		}
	}
	if _, err := svc.CreateUser(context.Background(), "res-db", "qa_extra"); err != nil {
		t.Fatalf("control add-user = %v", err)
	}
}

// An unavailable database says why with a fixed sentence — never the raw
// condition text, which can carry API-server detail — and nothing else does.
func TestUnavailableDatabaseCarriesAValueFreeReason(t *testing.T) {
	failed := func(reason, message string) *appv1alpha1.Database {
		d := &appv1alpha1.Database{Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseFailed}}
		meta.SetStatusCondition(&d.Status.Conditions, metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, Message: message})
		return d
	}
	cluster := pgView(failed("ClusterFailed", `admission webhook denied: secret "dpg-x-app" spec.managed.roles[1]`))
	if cluster.Status != "unavailable" || !strings.Contains(cluster.StatusReason, "rejected its configuration") || strings.Contains(cluster.StatusReason, "dpg-x-app") {
		t.Fatalf("ClusterFailed view = %q / %q", cluster.Status, cluster.StatusReason)
	}
	if got := pgView(failed("StorageShrinkRejected", "Postgres storage is grow-only: requested 1 GB is below the allocated 5 GB")).StatusReason; !strings.Contains(got, "grow-only") {
		t.Fatalf("operator-authored message dropped: %q", got)
	}
	if got := pgView(failed("SomethingNew", "raw")).StatusReason; got != "The database failed to reconcile (SomethingNew)." {
		t.Fatalf("unknown reason = %q", got)
	}
	healthy := &appv1alpha1.Database{Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady}}
	if got := pgView(healthy).StatusReason; got != "" {
		t.Fatalf("a non-failed database carries a reason: %q", got)
	}
}
