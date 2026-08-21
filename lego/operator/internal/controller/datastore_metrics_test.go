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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestDatastoreMetricsSurfaceAStuckDatabase is the regression for w7/036: two
// production defects left every new Database in Provisioning forever and nothing
// observed it. The collector's whole reason to exist is that this reads 0.
func TestDatastoreMetricsSurfaceAStuckDatabase(t *testing.T) {
	scheme := newTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "dpg-stuck", Namespace: "tea-ws"},
			Status:     appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseProvisioning},
		},
		&appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "dpg-fine", Namespace: "tea-ws"},
			Status:     appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady},
		},
		&appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: "red-fine", Namespace: "tea-ws"},
			Status:     appv1alpha1.KeyValueStatus{Phase: appv1alpha1.KVPhaseReady},
		},
	).Build()

	got := renderMetric(t, NewDatastoreCollector(cl), "bex_datastore_ready")
	for _, want := range []string{
		`bex_datastore_ready{kind="database",name="dpg-stuck",namespace="tea-ws",phase="Provisioning"} 0`,
		`bex_datastore_ready{kind="database",name="dpg-fine",namespace="tea-ws",phase="Ready"} 1`,
		`bex_datastore_ready{kind="keyvalue",name="red-fine",namespace="tea-ws",phase="Ready"} 1`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing series:\n  want %s\n  got:\n%s", want, got)
		}
	}
}

// A listing failure must not read as "everything healthy" — an alert on
// bex_datastore_ready would otherwise go quiet exactly when observation breaks.
func TestDatastoreMetricsCountObservationFailure(t *testing.T) {
	got := renderMetric(t, NewDatastoreCollector(failingLister{}), "bex_datastore_observe_errors_total")
	if !strings.Contains(got, "bex_datastore_observe_errors_total 1") {
		t.Errorf("a failed listing must be counted, got:\n%s", got)
	}
	if strings.Contains(got, "bex_datastore_ready") {
		t.Error("a failed listing must emit no readiness series at all")
	}
}

func TestDatastoreMetricsReportAge(t *testing.T) {
	scheme := newTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dpg-old", Namespace: "tea-ws",
				CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			},
			Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseProvisioning},
		},
	).Build()

	got := renderMetric(t, NewDatastoreCollector(cl), "bex_datastore_age_seconds")
	// The alert pairs age with readiness, so age must actually advance.
	if !strings.Contains(got, `bex_datastore_age_seconds{kind="database",name="dpg-old",namespace="tea-ws"} 7`) {
		t.Errorf("age should be ~7200s for a 2h-old CR, got:\n%s", got)
	}
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

type failingLister struct{}

func (failingLister) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errors.New("apiserver unavailable")
}

func renderMetric(t *testing.T, c prometheus.Collector, name string) string {
	t.Helper()
	var buf strings.Builder
	if err := testutil.CollectAndCompare(c, strings.NewReader(""), name); err != nil {
		// CollectAndCompare against empty always errors; we only want the dump.
		buf.WriteString(err.Error())
	}
	return buf.String()
}
