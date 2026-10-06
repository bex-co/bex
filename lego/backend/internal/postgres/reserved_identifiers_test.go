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
		`{"name":"res-cnpg","databaseUser":"cnpg_reader"}`,
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

	// Every add-user refusal is coded and names the field (w5/m118); an
	// invalid name used to answer an uncoded 400.
	seedDatabaseSpec(t, cl, "res-db", appv1alpha1.DatabaseSpec{Plan: "free"}, false)
	for role, code := range map[string]string{
		"postgres": "POSTGRES_IDENTIFIER_RESERVED", "streaming_replica": "POSTGRES_IDENTIFIER_RESERVED",
		"pg_signal_backend": "POSTGRES_IDENTIFIER_RESERVED", "cnpg_reader": "POSTGRES_IDENTIFIER_RESERVED",
		"Bad-Name": "POSTGRES_IDENTIFIER_INVALID", "": "POSTGRES_IDENTIFIER_INVALID",
		strings.Repeat("a", 64): "POSTGRES_IDENTIFIER_INVALID",
	} {
		_, err := svc.CreateUser(context.Background(), "res-db", role)
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != code || coded.Params["field"] != "name" || !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("CreateUser(%.20q) = %v, want a 400 coded %s for field name", role, err, code)
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
	cluster := pgView(failed(appv1alpha1.ReasonClusterFailed, `admission webhook denied: secret "dpg-x-app" spec.managed.roles[1]`))
	if cluster.Status != "unavailable" || !strings.Contains(cluster.StatusReason, "rejected its configuration") || strings.Contains(cluster.StatusReason, "dpg-x-app") {
		t.Fatalf("ClusterFailed view = %q / %q", cluster.Status, cluster.StatusReason)
	}
	// w5/079: the reason travels as a code a client can translate.
	if cluster.StatusReasonCode != appv1alpha1.ReasonClusterFailed {
		t.Fatalf("ClusterFailed code = %q", cluster.StatusReasonCode)
	}
	shrink := pgView(failed(appv1alpha1.ReasonStorageShrinkRejected, "Postgres storage is grow-only: requested 1 GB is below the allocated 5 GB"))
	if !strings.Contains(shrink.StatusReason, "grow-only") || shrink.StatusReasonCode != appv1alpha1.ReasonStorageShrinkRejected {
		t.Fatalf("operator-authored message or its code dropped: %q / %q", shrink.StatusReason, shrink.StatusReasonCode)
	}
	unknown := pgView(failed("SomethingNew", "raw"))
	if unknown.StatusReason != "The database failed to reconcile (SomethingNew)." || unknown.StatusReasonCode != "SomethingNew" {
		t.Fatalf("unknown reason = %q / %q", unknown.StatusReason, unknown.StatusReasonCode)
	}
	healthy := &appv1alpha1.Database{Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady}}
	if got := pgView(healthy); got.StatusReason != "" || got.StatusReasonCode != "" {
		t.Fatalf("a non-failed database carries a reason: %q / %q", got.StatusReason, got.StatusReasonCode)
	}
}
