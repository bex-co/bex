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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w5/175: a WAL archive that hangs keeps CNPG's ContinuousArchiving=True, so
// the operator reports the stall itself and postgres_wal_archive_failed fires.

func TestPrometheusWALArchiveStallReaderNeedsWaitingSegmentsAndNoArchival(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		query := req.URL.Query().Get("query")
		if !strings.Contains(query, `namespace="tenant",cnpg_io_cluster="dpg-one"`) {
			t.Errorf("query selector = %q", query)
		}
		// dpg-one-1 has waited the whole window and archived nothing; dpg-one-2
		// is a standby with no waiting segments; dpg-one-3 waits but archives.
		values := `[{"metric":{"pod":"dpg-one-1"},"value":[1,"3"]},{"metric":{"pod":"dpg-one-3"},"value":[1,"7"]}]`
		switch {
		case strings.Contains(query, "seconds_since_last_archival"):
			if !strings.Contains(query, "> 900") {
				t.Errorf("archival threshold = %q", query)
			}
			values = `[{"metric":{"pod":"dpg-one-1"},"value":[1,"1380"]},{"metric":{"pod":"dpg-one-2"},"value":[1,"7200"]}]`
		case !strings.Contains(query, `min_over_time(cnpg_collector_pg_wal_archive_status{`) ||
			!strings.Contains(query, `value="ready"}[15m]`):
			t.Errorf("waiting-segment query = %q", query)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":%s}}`, values)
	}))
	defer ts.Close()

	stall, err := NewPrometheusWALArchiveStallReader(ts.URL, ts.Client())(context.Background(), "tenant", "dpg-one")
	if err != nil {
		t.Fatal(err)
	}
	if stall == nil || stall.Pod != "dpg-one-1" || stall.ReadySegments != 3 || stall.SinceLastArchive != 23*time.Minute {
		t.Fatalf("stall = %+v", stall)
	}
}

func TestPrometheusWALArchiveStallReaderIdleDatabaseIsNotStalled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// An idle database: hours since the last archival, nothing waiting.
		values := `[]`
		if strings.Contains(req.URL.Query().Get("query"), "seconds_since_last_archival") {
			values = `[{"metric":{"pod":"dpg-one-1"},"value":[1,"7200"]}]`
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":%s}}`, values)
	}))
	defer ts.Close()

	stall, err := NewPrometheusWALArchiveStallReader(ts.URL, ts.Client())(context.Background(), "tenant", "dpg-one")
	if err != nil || stall != nil {
		t.Fatalf("idle database = %+v, %v", stall, err)
	}
}

func TestWALArchiveStallOverridesCNPGTrue(t *testing.T) {
	stalled := func(context.Context, string, string) (*WALArchiveStall, error) {
		return &WALArchiveStall{Pod: "dpg-x-1", ReadySegments: 2, SinceLastArchive: 23 * time.Minute}, nil
	}
	healthy := func(context.Context, string, string) (*WALArchiveStall, error) { return nil, nil }
	unreadable := func(context.Context, string, string) (*WALArchiveStall, error) {
		return nil, errors.New("prometheus: status 503")
	}
	pass := func(db *appv1alpha1.Database, reader WALArchiveStallReader, cnpgStatus string) *metav1.Condition {
		t.Helper()
		(&DatabaseReconciler{WALArchiveStallReader: reader}).stampWALArchiving(context.Background(), db,
			clusterWithArchiving(cnpgStatus, "ContinuousArchiving"+cnpgStatus, ""), true)
		return meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionWALArchiving)
	}

	db := &appv1alpha1.Database{}
	if c := pass(db, stalled, "True"); c == nil || c.Status != metav1.ConditionFalse || c.Reason != reasonWALArchiveStalled ||
		!strings.Contains(c.Message, "waited on dpg-x-1 for over 15m0s") {
		t.Fatalf("stalled archive = %+v", c)
	}
	// A continuing stall keeps its transition time, so it is not rewritten
	// on every pass.
	began := metav1.NewTime(time.Now().Add(-time.Hour))
	meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionWALArchiving).LastTransitionTime = began
	if c := pass(db, stalled, "True"); c == nil || !c.LastTransitionTime.Equal(&began) {
		t.Fatalf("continuing stall = %+v", c)
	}
	// A probe that cannot read keeps the stall rather than flapping to True,
	// which would re-arm and re-send the event on the next good read.
	if c := pass(db, unreadable, "True"); c == nil || c.Reason != reasonWALArchiveStalled {
		t.Fatalf("unreadable probe = %+v", c)
	}
	if c := pass(db, healthy, "True"); c == nil || c.Status != metav1.ConditionTrue {
		t.Fatalf("recovered archive = %+v", c)
	}
	// An unreadable probe never invents a stall.
	if c := pass(db, unreadable, "True"); c == nil || c.Status != metav1.ConditionTrue {
		t.Fatalf("unreadable probe on a healthy archive = %+v", c)
	}
	// CNPG's own failure verdict stands; the probe does not replace it.
	if c := pass(db, stalled, "False"); c == nil || c.Reason != "ContinuousArchivingFalse" {
		t.Fatalf("CNPG failure = %+v", c)
	}
}

func TestWALArchiveStallProbeRequeuesOnlyBackupEnabledDatabases(t *testing.T) {
	r := &DatabaseReconciler{}
	if got := r.walArchiveStallRequeue(true); got != 0 {
		t.Fatalf("no reader requeue = %s", got)
	}
	r.WALArchiveStallReader = func(context.Context, string, string) (*WALArchiveStall, error) { return nil, nil }
	if got := r.walArchiveStallRequeue(false); got != 0 {
		t.Fatalf("backups off requeue = %s", got)
	}
	if got := r.walArchiveStallRequeue(true); got != walArchiveStallProbeInterval {
		t.Fatalf("backup-enabled requeue = %s", got)
	}
}

func TestDatastoreMetricsReportAStalledArchiveAsNotArchiving(t *testing.T) {
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cluster.SetNamespace("tea-ws")
	cluster.SetName("dpg-hung")
	_ = unstructured.SetNestedSlice(cluster.Object, []any{
		map[string]any{"type": "ContinuousArchiving", "status": "True"},
	}, "status", "conditions")
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-hung", Namespace: "tea-ws"},
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, BackupsEnabled: true,
			Conditions: []metav1.Condition{{
				Type: appv1alpha1.ConditionWALArchiving, Status: metav1.ConditionFalse, Reason: reasonWALArchiveStalled,
			}},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(db).WithRuntimeObjects(cluster).Build()

	got := renderMetric(t, NewDatastoreCollector(cl), "bex_datastore_wal_archiving")
	if want := `bex_datastore_wal_archiving{name="dpg-hung",namespace="tea-ws"} 0`; !strings.Contains(got, want) {
		t.Fatalf("missing %q in:\n%s", want, got)
	}
}
