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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Database disk autoscaling", func() {
	It("persists one grow-only resize and audit entry past the threshold", func(ctx SpecContext) {
		now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
		db := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "dpg-autoscale-envtest", Namespace: "default"},
			Spec: appv1alpha1.DatabaseSpec{
				Plan: "free", StorageGB: 10, DiskAutoscaling: true,
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), db) })

		recorder := record.NewFakeRecorder(10)
		r := &DatabaseReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Recorder: recorder,
			Now: func() time.Time { return now },
			DiskUsageReader: func(context.Context, string, string) (DatabaseDiskUsage, error) {
				return DatabaseDiskUsage{PVC: "dpg-autoscale-envtest-1", UsedBytes: 90, CapacityBytes: 100}, nil
			},
		}
		requeue, err := r.applyDiskAutoscaling(ctx, db)
		Expect(err).NotTo(HaveOccurred())
		Expect(requeue).To(Equal(databaseDiskAutoscaleInterval))

		got := &appv1alpha1.Database{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), got)).To(Succeed())
		Expect(got.Spec.StorageGB).To(Equal(int32(15)))
		Expect(got.Status.DiskResizeHistory).To(ConsistOf(appv1alpha1.DatabaseDiskResizeStatus{
			At: now.Format(time.RFC3339), FromGB: 10, ToGB: 15, UsedBytes: 90, CapacityBytes: 100,
		}))
		Expect(recorder.Events).To(Receive(ContainSubstring("DiskAutoscaled")))

		now = now.Add(time.Minute)
		requeue, err = r.applyDiskAutoscaling(ctx, got)
		Expect(err).NotTo(HaveOccurred())
		Expect(requeue).To(Equal(databaseDiskAutoscaleInterval))
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), got)).To(Succeed())
		Expect(got.Spec.StorageGB).To(Equal(int32(15)))
		Expect(got.Status.DiskResizeHistory).To(HaveLen(1))
	})
})

func TestDatabaseDiskAutoscaleDecision(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		current    int32
		used, cap  int64
		last       time.Time
		want       int32
		wantResize bool
	}{
		{name: "below threshold", current: 10, used: 899, cap: 1000, want: 10},
		{name: "threshold inclusive", current: 10, used: 900, cap: 1000, want: 15, wantResize: true},
		{name: "round half upward then five", current: 25, used: 90, cap: 100, want: 40, wantResize: true},
		{name: "one GB grows to five", current: 1, used: 90, cap: 100, want: 5, wantResize: true},
		{name: "cap bounded", current: 12000, used: 90, cap: 100, want: 16384, wantResize: true},
		{name: "at cap", current: 16384, used: 99, cap: 100, want: 16384},
		{name: "cooldown", current: 10, used: 99, cap: 100, last: now.Add(-time.Hour), want: 10},
		{name: "cooldown elapsed", current: 10, used: 99, cap: 100, last: now.Add(-12 * time.Hour), want: 15, wantResize: true},
		{name: "missing capacity", current: 10, used: 99, cap: 0, want: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, resized := databaseDiskAutoscaleDecision(tt.current, DatabaseDiskUsage{UsedBytes: tt.used, CapacityBytes: tt.cap}, tt.last, now)
			if got != tt.want || resized != tt.wantResize {
				t.Fatalf("decision = (%d, %v), want (%d, %v)", got, resized, tt.want, tt.wantResize)
			}
		})
	}
}

func TestPostgresDiskAutoscalingCapUsesSharedCatalogContract(t *testing.T) {
	if got := tiers.Postgres.DiskAutoscalingCapGB(); got != 16*1024 {
		t.Fatalf("operator cap source = %d GB, want the 16384 GB contract in lego/types/tiers/tiers.yaml", got)
	}
	if got := nextDatabaseDiskSize(12000); got != tiers.Postgres.DiskAutoscalingCapGB() {
		t.Fatalf("operator growth capped at %d GB, want shared catalog cap %d GB", got, tiers.Postgres.DiskAutoscalingCapGB())
	}
}

func TestDiskAutoscalingSampleFailuresDebouncePersistAndReset(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-sample-failures", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "free", StorageGB: 10, DiskAutoscaling: true},
		Status:     appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appv1alpha1.Database{}).WithObjects(db).Build()
	recorder := record.NewFakeRecorder(10)
	readerFails := true
	reader := func(context.Context, string, string) (DatabaseDiskUsage, error) {
		if readerFails {
			return DatabaseDiskUsage{}, fmt.Errorf("prometheus unavailable")
		}
		return DatabaseDiskUsage{PVC: "dpg-sample-failures-1", UsedBytes: 1, CapacityBytes: 10}, nil
	}
	key := client.ObjectKeyFromObject(db)
	apply := func(r *DatabaseReconciler) appv1alpha1.Database {
		t.Helper()
		var current appv1alpha1.Database
		if err := cl.Get(context.Background(), key, &current); err != nil {
			t.Fatal(err)
		}
		if _, err := r.applyDiskAutoscaling(context.Background(), &current); err != nil {
			t.Fatal(err)
		}
		if err := cl.Get(context.Background(), key, &current); err != nil {
			t.Fatal(err)
		}
		return current
	}

	r := &DatabaseReconciler{Client: cl, Scheme: scheme, Recorder: recorder, DiskUsageReader: reader}
	first := apply(r)
	if first.Annotations[annotDiskSampleFailures] != "1" {
		t.Fatalf("first failure count = %q, want 1", first.Annotations[annotDiskSampleFailures])
	}
	// A fresh reconciler proves the debounce counter survives manager restart.
	restarted := &DatabaseReconciler{Client: cl, Scheme: scheme, Recorder: recorder, DiskUsageReader: reader}
	second := apply(restarted)
	if second.Annotations[annotDiskSampleFailures] != "2" {
		t.Fatalf("second failure count after restart = %q, want 2", second.Annotations[annotDiskSampleFailures])
	}
	select {
	case event := <-recorder.Events:
		t.Fatalf("N-1 failures emitted an event: %q", event)
	default:
	}
	third := apply(restarted)
	if third.Annotations[annotDiskSampleFailures] != "3" {
		t.Fatalf("third failure count = %q, want 3", third.Annotations[annotDiskSampleFailures])
	}
	select {
	case event := <-recorder.Events:
		if !strings.Contains(event, "Warning DiskAutoscalingSampleUnavailable") || !strings.Contains(event, "3 consecutive") {
			t.Fatalf("warning event = %q", event)
		}
	default:
		t.Fatal("third consecutive sample failure did not emit a Warning event")
	}
	_ = apply(restarted)
	select {
	case event := <-recorder.Events:
		t.Fatalf("failure beyond threshold emitted a duplicate event: %q", event)
	default:
	}

	readerFails = false
	afterSuccess := apply(restarted)
	if _, exists := afterSuccess.Annotations[annotDiskSampleFailures]; exists {
		t.Fatalf("successful sample did not reset persisted failures: %v", afterSuccess.Annotations)
	}
}

func TestDiskAutoscalingSampleFailureStaysQuietBeforeDatabaseReady(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-provisioning", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{DiskAutoscaling: true},
		Status:     appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseProvisioning},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appv1alpha1.Database{}).WithObjects(db).Build()
	recorder := record.NewFakeRecorder(1)
	r := &DatabaseReconciler{Client: cl, Scheme: scheme, Recorder: recorder, DiskUsageReader: func(context.Context, string, string) (DatabaseDiskUsage, error) {
		return DatabaseDiskUsage{}, fmt.Errorf("no complete PVC usage sample")
	}}
	if _, err := r.applyDiskAutoscaling(context.Background(), db.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	var got appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(db), &got); err != nil {
		t.Fatal(err)
	}
	if got.Annotations[annotDiskSampleFailures] != "" {
		t.Fatalf("provisioning database accrued sample failures: %v", got.Annotations)
	}
	select {
	case event := <-recorder.Events:
		t.Fatalf("provisioning database emitted warning: %q", event)
	default:
	}
}

func TestApplyDatabaseDiskAutoscalingPersistsOneResizeEventAndStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-autoscale", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{
			Plan: "free", StorageGB: 10, DiskAutoscaling: true,
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appv1alpha1.Database{}).WithObjects(db).Build()
	recorder := record.NewFakeRecorder(10)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	r := &DatabaseReconciler{
		Client: cl, Scheme: scheme, Recorder: recorder, Now: func() time.Time { return now },
		DiskUsageReader: func(context.Context, string, string) (DatabaseDiskUsage, error) {
			return DatabaseDiskUsage{PVC: "dpg-autoscale-1", UsedBytes: 9, CapacityBytes: 10}, nil
		},
	}

	var current appv1alpha1.Database
	key := client.ObjectKeyFromObject(db)
	if err := cl.Get(context.Background(), key, &current); err != nil {
		t.Fatal(err)
	}
	requeue, err := r.applyDiskAutoscaling(context.Background(), &current)
	if err != nil {
		t.Fatal(err)
	}
	if requeue != time.Minute {
		t.Fatalf("requeue = %s, want 1m", requeue)
	}
	if err := cl.Get(context.Background(), key, &current); err != nil {
		t.Fatal(err)
	}
	if current.Spec.StorageGB != 15 {
		t.Fatalf("storageGB = %d, want 15", current.Spec.StorageGB)
	}
	if len(current.Status.DiskResizeHistory) != 1 {
		t.Fatalf("history = %+v, want one entry", current.Status.DiskResizeHistory)
	}
	entry := current.Status.DiskResizeHistory[0]
	if entry.FromGB != 10 || entry.ToGB != 15 || entry.UsedBytes != 9 || entry.CapacityBytes != 10 {
		t.Fatalf("history entry = %+v", entry)
	}
	select {
	case event := <-recorder.Events:
		if !strings.Contains(event, "DiskAutoscaled") || !strings.Contains(event, "10 GB to 15 GB") {
			t.Fatalf("event = %q", event)
		}
	default:
		t.Fatal("expected DiskAutoscaled event")
	}

	// The kubelet sample is intentionally still high. The persisted 12-hour
	// cooldown must prevent a second bump and a duplicate event.
	now = now.Add(time.Minute)
	if _, err := r.applyDiskAutoscaling(context.Background(), &current); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), key, &current); err != nil {
		t.Fatal(err)
	}
	if current.Spec.StorageGB != 15 || len(current.Status.DiskResizeHistory) != 1 {
		t.Fatalf("second pass changed database: storage=%d history=%+v", current.Spec.StorageGB, current.Status.DiskResizeHistory)
	}
	select {
	case event := <-recorder.Events:
		t.Fatalf("unexpected second event: %q", event)
	default:
	}
}

func TestApplyDatabaseDiskAutoscalingDisabledNeverReadsOrResizes(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-disabled", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{StorageGB: 10},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appv1alpha1.Database{}).WithObjects(db).Build()
	called := false
	r := &DatabaseReconciler{Client: cl, Scheme: scheme, DiskUsageReader: func(context.Context, string, string) (DatabaseDiskUsage, error) {
		called = true
		return DatabaseDiskUsage{UsedBytes: 100, CapacityBytes: 100}, nil
	}}
	if requeue, err := r.applyDiskAutoscaling(context.Background(), db.DeepCopy()); err != nil || requeue != 0 {
		t.Fatalf("apply = (%s, %v), want disabled no-op", requeue, err)
	}
	if called {
		t.Fatal("disabled autoscaling read usage")
	}
	var got appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(db), &got); err != nil {
		t.Fatal(err)
	}
	if got.Spec.StorageGB != 10 {
		t.Fatalf("disabled storageGB = %d, want 10", got.Spec.StorageGB)
	}
}

func TestPrometheusDatabaseDiskUsageReaderChoosesFullestPVC(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		query := req.URL.Query().Get("query")
		if !strings.Contains(query, `namespace="tenant"`) || !strings.Contains(query, `persistentvolumeclaim=~"^dpg-one-[0-9]+$"`) {
			t.Errorf("query selector = %q", query)
		}
		metric := "used"
		values := `[{"metric":{"persistentvolumeclaim":"dpg-one-1"},"value":[1,"80"]},{"metric":{"persistentvolumeclaim":"dpg-one-2"},"value":[1,"95"]}]`
		if strings.Contains(query, "capacity") {
			metric = "capacity"
			values = `[{"metric":{"persistentvolumeclaim":"dpg-one-1"},"value":[1,"100"]},{"metric":{"persistentvolumeclaim":"dpg-one-2"},"value":[1,"100"]}]`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":%s},"metric":%q}`, values, metric)
	}))
	defer ts.Close()

	usage, err := NewPrometheusDatabaseDiskUsageReader(ts.URL, ts.Client())(context.Background(), "tenant", "dpg-one")
	if err != nil {
		t.Fatal(err)
	}
	if usage.PVC != "dpg-one-2" || usage.UsedBytes != 95 || usage.CapacityBytes != 100 {
		t.Fatalf("usage = %+v", usage)
	}
}
