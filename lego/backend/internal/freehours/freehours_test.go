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

package freehours

import (
	"context"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

type fakeStore struct {
	mu      sync.Mutex
	seconds map[string]int64
	claimed map[string]bool
}

func (f *fakeStore) FreeInstanceSecondsByWorkspace(context.Context, time.Time, time.Time) (map[string]int64, error) {
	return f.seconds, nil
}

func (f *fakeStore) ClaimFreeHoursNotice(_ context.Context, ws string, _ time.Time, stage string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := ws + "/" + stage
	if f.claimed[key] {
		return false, nil
	}
	f.claimed[key] = true
	return true, nil
}

type recordingNotifier struct{ stages []string }

func (n *recordingNotifier) NotifyFreeHours(_ context.Context, ws, stage string, _, _ float64) error {
	n.stages = append(n.stages, ws+"/"+stage)
	return nil
}

func app(name, ws, tier string, typ string, suspended bool, annotation string) *appv1alpha1.App {
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, Labels: map[string]string{core.LabelTenant: ws}},
		Spec:       appv1alpha1.AppSpec{Type: typ, Tier: tier, Image: "nginx:1", Suspended: suspended},
	}
	if annotation != "" {
		a.Annotations = map[string]string{core.AnnotFreeHoursSuspended: annotation}
	}
	return a
}

func newSweeper(t *testing.T, now time.Time, st *fakeStore, objs ...client.Object) (*Sweeper, client.Client, *recordingNotifier) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	base := &core.Base{Client: cl, Namespace: "default", Clock: func() time.Time { return now }}
	n := &recordingNotifier{}
	return &Sweeper{Apps: apps.FreeHoursWorker{Svc: &apps.Service{Base: base}}, Store: st, Notifier: n, BudgetHours: 10}, cl, n
}

func get(t *testing.T, cl client.Client, ws, name string) *appv1alpha1.App {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: ws, Name: name}, &a); err != nil {
		t.Fatal(err)
	}
	return &a
}

func TestSweepWarnsThenSuspendsFreeWebServicesOnly(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	st := &fakeStore{claimed: map[string]bool{}, seconds: map[string]int64{"tea-a": 8 * 3600, "tea-b": 11 * 3600}}
	sw, cl, notes := newSweeper(t, now, st,
		app("free-a", "tea-a", "free", appv1alpha1.TypeWebService, false, ""),
		app("free-b", "tea-b", "free", appv1alpha1.TypeWebService, false, ""),
		app("paid-b", "tea-b", "starter", appv1alpha1.TypeWebService, false, ""),
		app("site-b", "tea-b", "free", appv1alpha1.TypeStaticSite, false, ""),
		app("user-b", "tea-b", "free", appv1alpha1.TypeWebService, true, ""),
	)
	for range 2 { // a second sweep changes nothing and sends nothing
		if err := sw.SweepOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := get(t, cl, "tea-a", "free-a"); got.Spec.Suspended {
		t.Error("80% warns but must not suspend")
	}
	if got := get(t, cl, "tea-b", "free-b"); !got.Spec.Suspended || got.Annotations[core.AnnotFreeHoursSuspended] != "2026-10" {
		t.Errorf("an exhausted workspace's Free web service must be suspended for 2026-10: %+v", got.Annotations)
	}
	for _, kept := range []string{"paid-b", "site-b"} {
		if get(t, cl, "tea-b", kept).Spec.Suspended {
			t.Errorf("%s must keep running", kept)
		}
	}
	if user := get(t, cl, "tea-b", "user-b"); user.Annotations[core.AnnotFreeHoursSuspended] != "" {
		t.Error("an owner's own suspension must not be re-labelled")
	}
	if len(notes.stages) != 2 || notes.stages[0] == notes.stages[1] {
		t.Errorf("each workspace gets exactly one email: %v", notes.stages)
	}
}

func TestSweepResumesAtMonthEndAndOnUpgrade(t *testing.T) {
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	st := &fakeStore{claimed: map[string]bool{}, seconds: map[string]int64{}}
	sw, cl, _ := newSweeper(t, now, st,
		app("rolled", "tea-b", "free", appv1alpha1.TypeWebService, true, "2026-10"),
		app("upgraded", "tea-b", "starter", appv1alpha1.TypeWebService, true, "2026-11"),
		app("still", "tea-b", "free", appv1alpha1.TypeWebService, true, "2026-11"),
		app("owner", "tea-b", "free", appv1alpha1.TypeWebService, true, ""),
	)
	if err := sw.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	for name, wantSuspended := range map[string]bool{"rolled": false, "upgraded": false, "still": true, "owner": true} {
		if got := get(t, cl, "tea-b", name); got.Spec.Suspended != wantSuspended {
			t.Errorf("%s suspended = %v, want %v", name, got.Spec.Suspended, wantSuspended)
		}
	}
	if get(t, cl, "tea-b", "rolled").Annotations[core.AnnotFreeHoursSuspended] != "" {
		t.Error("resume must clear the marker")
	}
}
