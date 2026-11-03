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

package diskusage

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeStore struct {
	disk   store.Disk
	state  store.DiskUsageState
	facts  []store.ServiceEventFact
	record func() error
}

func (f *fakeStore) ListDisks(context.Context, string, string) ([]store.Disk, error) {
	return []store.Disk{f.disk}, nil
}

func (f *fakeStore) GetDiskUsageState(context.Context, string) (store.DiskUsageState, error) {
	return f.state, nil
}

func (f *fakeStore) SetDiskUsagePending(_ context.Context, _ string, pending *bool, at time.Time) error {
	if pending == nil {
		f.state.PendingHigh, f.state.PendingSince = nil, nil
		return nil
	}
	if f.state.PendingHigh == nil || *f.state.PendingHigh != *pending {
		since := at
		f.state.PendingSince = &since
	}
	v := *pending
	f.state.PendingHigh = &v
	return nil
}

func (f *fakeStore) RecordDiskUsageCrossing(_ context.Context, fact store.ServiceEventFact) (bool, error) {
	if f.record != nil {
		if err := f.record(); err != nil {
			return false, err
		}
	}
	f.state = store.DiskUsageState{High: fact.Type == store.EventFactServiceDiskUsageHigh}
	f.facts = append(f.facts, fact)
	return true, nil
}

type mail struct {
	high      bool
	sourceKey string
	details   store.DiskUsageDetails
}

type recorder struct {
	mails []mail
	err   error
}

func (r *recorder) NotifyDiskUsage(_ context.Context, _, _, _ string, high bool, d store.DiskUsageDetails, key string, _ time.Time) error {
	if r.err != nil {
		return r.err
	}
	r.mails = append(r.mails, mail{high, key, d})
	return nil
}

const gib = float64(1 << 30)

func newSweeper(t *testing.T, used *float64, st *fakeStore, n Notifier, apps ...*appv1alpha1.App) (*Sweeper, *time.Time) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	b := fake.NewClientBuilder().WithScheme(scheme)
	for _, a := range apps {
		b = b.WithObjects(a)
	}
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	usage := func(_ context.Context, req metrics.DiskUsageRequest) ([]metrics.MetricSeries, error) {
		if used == nil {
			return nil, nil
		}
		if req.PVCPattern != "^"+appv1alpha1.DiskPVCName("web")+"$" {
			t.Errorf("queried PVC %q", req.PVCPattern)
		}
		return []metrics.MetricSeries{{Points: []metrics.MetricPoint{
			{Timestamp: req.End.Add(-2 * time.Minute).Format(time.RFC3339), Value: 0},
			{Timestamp: req.End.Format(time.RFC3339), Value: *used},
		}}}, nil
	}
	w := &Sweeper{Apps: b.Build(), Store: st, Usage: usage, Notifier: n}
	w.Clock = func() time.Time { return now }
	return w, &now
}

func diskApp(mut ...func(*appv1alpha1.App)) *appv1alpha1.App {
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", Labels: map[string]string{
			core.LabelTenant: "tea-a", core.LabelAppID: "srv-web",
		}},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 10}},
	}
	a.Status.Phase = appv1alpha1.PhaseRunning
	for _, m := range mut {
		m(a)
	}
	return a
}

func sweep(t *testing.T, w *Sweeper) {
	t.Helper()
	if err := w.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// w2/m255: a disk crossing 80% records one event and one email only once two
// consecutive sweeps agree, and dropping back below records the recovery.
func TestCrossingNeedsTwoSweepsBothWays(t *testing.T) {
	used := 8.5 * gib // 85% of 10 GiB
	st := &fakeStore{disk: store.Disk{ID: "dsk-1", Name: "data", MountPath: "/data", SizeGB: 10}}
	n := &recorder{}
	w, now := newSweeper(t, &used, st, n, diskApp())

	sweep(t, w)
	if len(st.facts) != 0 || len(n.mails) != 0 {
		t.Fatal("one sweep over the threshold must not flip the state")
	}
	*now = now.Add(5 * time.Minute)
	sweep(t, w)
	if len(st.facts) != 1 || st.facts[0].Type != store.EventFactServiceDiskUsageHigh {
		t.Fatalf("facts = %+v, want one high crossing", st.facts)
	}
	d := st.facts[0].DiskUsage
	if d == nil || d.DiskID != "dsk-1" || d.UsagePercent != 85 || d.ThresholdPercent != 80 || d.MountPath != "/data" {
		t.Errorf("details = %+v", d)
	}
	if len(n.mails) != 1 || !n.mails[0].high || n.mails[0].sourceKey != st.facts[0].SourceKey {
		t.Errorf("mails = %+v, want one high mail keyed like the fact", n.mails)
	}
	sweep(t, w) // still high: nothing more
	if len(st.facts) != 1 {
		t.Fatal("a disk that stays high recorded a second crossing")
	}

	// A single dip below the threshold does not flap.
	used = 7.9 * gib
	sweep(t, w)
	used = 8.6 * gib
	sweep(t, w)
	if len(st.facts) != 1 || st.state.PendingHigh != nil {
		t.Fatalf("a one-sweep dip recorded something: facts=%+v state=%+v", st.facts, st.state)
	}

	used = 5 * gib
	sweep(t, w)
	*now = now.Add(5 * time.Minute)
	sweep(t, w)
	if len(st.facts) != 2 || st.facts[1].Type != store.EventFactServiceDiskUsageRecovered || st.facts[1].DiskUsage.UsagePercent != 50 {
		t.Fatalf("facts = %+v, want a recovery at 50%%", st.facts)
	}
	if len(n.mails) != 2 || n.mails[1].high {
		t.Errorf("mails = %+v, want a recovery mail", n.mails)
	}
}

func TestNoSeriesSuspendedOrDisklessIsSkipped(t *testing.T) {
	st := &fakeStore{disk: store.Disk{ID: "dsk-1", SizeGB: 10}}
	w, _ := newSweeper(t, nil, st, &recorder{}, diskApp())
	sweep(t, w)
	sweep(t, w)
	if st.state.PendingHigh != nil || len(st.facts) != 0 {
		t.Error("an unmounted volume (no series) must not be judged")
	}

	used := 9 * gib
	for _, app := range []*appv1alpha1.App{
		diskApp(func(a *appv1alpha1.App) { a.Spec.Suspended = true }),
		diskApp(func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseHibernated }),
		diskApp(func(a *appv1alpha1.App) { a.Spec.Disk = nil }),
	} {
		st := &fakeStore{disk: store.Disk{ID: "dsk-1", SizeGB: 10}}
		w, _ := newSweeper(t, &used, st, &recorder{}, app)
		sweep(t, w)
		sweep(t, w)
		if st.state.PendingHigh != nil || len(st.facts) != 0 {
			t.Errorf("%+v was evaluated", app.Spec)
		}
	}
}

// The email goes out before the crossing is recorded: if it cannot, nothing is
// recorded and the next sweep tries again.
func TestUnmailedCrossingIsNotRecorded(t *testing.T) {
	used := 9 * gib
	st := &fakeStore{disk: store.Disk{ID: "dsk-1", SizeGB: 10}}
	n := &recorder{err: errors.New("queue down")}
	w, now := newSweeper(t, &used, st, n, diskApp())
	sweep(t, w)
	*now = now.Add(5 * time.Minute)
	if err := w.SweepOnce(context.Background()); err == nil {
		t.Fatal("a failed notification must surface")
	}
	if len(st.facts) != 0 || st.state.High {
		t.Fatalf("recorded without mailing: %+v", st.facts)
	}
	n.err = nil
	sweep(t, w)
	if len(st.facts) != 1 || len(n.mails) != 1 {
		t.Fatalf("retry: facts=%d mails=%d", len(st.facts), len(n.mails))
	}
}

// A sighting left over from before the service was parked (or its series
// went missing) is stale: the next sweep starts again instead of confirming.
func TestStaleSightingIsNotConfirmed(t *testing.T) {
	used := 9 * gib
	st := &fakeStore{disk: store.Disk{ID: "dsk-1", SizeGB: 10}}
	n := &recorder{}
	w, now := newSweeper(t, &used, st, n, diskApp())
	sweep(t, w)
	first := *st.state.PendingSince
	*now = now.Add(3 * 24 * time.Hour)
	sweep(t, w)
	if len(st.facts) != 0 || st.state.PendingSince == nil || st.state.PendingSince.Equal(first) {
		t.Fatalf("a days-old sighting confirmed or was kept: facts=%d state=%+v", len(st.facts), st.state)
	}
	*now = now.Add(5 * time.Minute)
	sweep(t, w)
	if len(st.facts) != 1 {
		t.Fatalf("a fresh pair of sweeps must confirm: facts=%d", len(st.facts))
	}
}
