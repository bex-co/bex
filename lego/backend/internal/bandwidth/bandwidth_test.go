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

package bandwidth

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const gb = 1 << 30

type fakeStore struct {
	usage  []store.MonthUsage
	claims map[string]bool // "<ws>/<stage>"
}

func (f *fakeStore) EgressBytesByWorkspace(context.Context, time.Time, time.Time) ([]store.MonthUsage, error) {
	return f.usage, nil
}

func (f *fakeStore) ClaimBandwidthNotice(_ context.Context, ws string, _ time.Time, stage string) (bool, error) {
	if f.claims == nil {
		f.claims = map[string]bool{}
	}
	key := ws + "/" + stage
	if f.claims[key] {
		return false, nil
	}
	f.claims[key] = true
	return true, nil
}

// fakeApps suspends and resumes in memory the way apps.BillingSuspensionWorker does:
// never over an owner's suspension.
type fakeApps struct {
	now   time.Time
	items []appv1alpha1.App
}

func (f *fakeApps) Now() time.Time                                  { return f.now }
func (f *fakeApps) List(context.Context) ([]appv1alpha1.App, error) { return f.items, nil }

func (f *fakeApps) find(name string) *appv1alpha1.App {
	for i := range f.items {
		if f.items[i].Name == name {
			return &f.items[i]
		}
	}
	return nil
}

func (f *fakeApps) Suspend(_ context.Context, a *appv1alpha1.App, month string) error {
	if got := f.find(a.Name); !got.Spec.Suspended {
		got.Spec.Suspended = true
		metav1.SetMetaDataAnnotation(&got.ObjectMeta, core.AnnotBandwidthSuspended, month)
	}
	return nil
}

func (f *fakeApps) Resume(_ context.Context, a *appv1alpha1.App) error {
	if got := f.find(a.Name); got.Annotations[core.AnnotBandwidthSuspended] != "" {
		got.Spec.Suspended = false
		delete(got.Annotations, core.AnnotBandwidthSuspended)
	}
	return nil
}

func (f *fakeApps) suspendedIn(name string) string {
	return f.find(name).Annotations[core.AnnotBandwidthSuspended]
}

type sent struct {
	ws, stage string
	canPay    bool
}

type fakeNotifier struct{ sent []sent }

func (f *fakeNotifier) NotifyBandwidth(_ context.Context, ws, stage string, _, _ float64, canPay bool) error {
	f.sent = append(f.sent, sent{ws, stage, canPay})
	return nil
}

func app(name, ws string) appv1alpha1.App {
	return appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, Labels: map[string]string{core.LabelTenant: ws}}}
}

// plans answers each workspace's plan; one not listed has none.
type plans map[string]string

func (p plans) WorkspacePlanOf(_ context.Context, ws string) (string, bool, error) {
	plan, ok := p[ws]
	return plan, ok, nil
}

type failingPlans struct{}

func (failingPlans) WorkspacePlanOf(context.Context, string) (string, bool, error) {
	return "", false, errors.New("plan store unavailable")
}

var now = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func TestSweepSpinsDownOnlyACardlessWorkspacePastItsBandwidth(t *testing.T) {
	st := &fakeStore{usage: []store.MonthUsage{
		{WorkspaceID: "tea-out", Quantity: 5 * gb},                 // spent, no card
		{WorkspaceID: "tea-card", Quantity: 30 * gb, CanPay: true}, // spent, billed above
		{WorkspaceID: "tea-near", Quantity: 4*gb + 1},              // past 80%
		{WorkspaceID: "tea-none", Quantity: 9999 * gb},             // no plan applies
	}}
	owner := app("paused", "tea-out")
	owner.Spec.Suspended = true
	apps := &fakeApps{now: now, items: []appv1alpha1.App{
		app("web", "tea-out"), app("worker", "tea-out"), owner,
		app("web", "tea-card"), app("api", "tea-near"), app("svc", "tea-none"),
	}}
	apps.items[3].Name = "card-web"
	n := &fakeNotifier{}
	w := &Sweeper{Apps: apps, Store: st, Notifier: n, Plans: plans{"tea-out": "hobby", "tea-card": "hobby", "tea-near": "hobby"}}

	if err := w.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"web", "worker"} {
		if got := apps.suspendedIn(name); got != "2026-11" {
			t.Errorf("%s suspended in %q, want 2026-11: every service of a card-less workspace past 5 GB spins down", name, got)
		}
	}
	if apps.suspendedIn("paused") != "" || !apps.find("paused").Spec.Suspended {
		t.Error("the owner's suspension must stay the owner's")
	}
	for _, name := range []string{"card-web", "api", "svc"} {
		if apps.find(name).Spec.Suspended {
			t.Errorf("%s suspended: a workspace that can pay, is under its allowance, or has no plan keeps running", name)
		}
	}
	want := map[sent]bool{{"tea-out", StageExhausted, false}: true, {"tea-card", StageExhausted, true}: true, {"tea-near", StageWarned, false}: true}
	if len(n.sent) != len(want) {
		t.Errorf("sent %v, want %v", n.sent, want)
	}
	for _, s := range n.sent {
		if !want[s] {
			t.Errorf("unexpected notice %+v", s)
		}
	}

	// A second sweep sends nothing again and changes nothing.
	n.sent = nil
	if err := w.SweepOnce(context.Background()); err != nil || len(n.sent) != 0 {
		t.Errorf("second sweep: err=%v sent=%v", err, n.sent)
	}
}

func TestSweepResumesWhenACardIsBoundOrTheMonthTurns(t *testing.T) {
	suspended := func(name, ws, month string) appv1alpha1.App {
		a := app(name, ws)
		a.Spec.Suspended = true
		a.Annotations = map[string]string{core.AnnotBandwidthSuspended: month}
		return a
	}
	st := &fakeStore{usage: []store.MonthUsage{
		{WorkspaceID: "tea-carded", Quantity: 6 * gb, CanPay: true},
		{WorkspaceID: "tea-still", Quantity: 6 * gb},
	}}
	apps := &fakeApps{now: now, items: []appv1alpha1.App{
		suspended("carded", "tea-carded", "2026-11"),
		suspended("still", "tea-still", "2026-11"),
		suspended("lastmonth", "tea-quiet", "2026-10"), // no egress yet this month
	}}
	w := &Sweeper{Apps: apps, Store: st, Plans: plans{"tea-carded": "hobby", "tea-still": "hobby", "tea-quiet": "hobby"}}

	if err := w.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if apps.find("carded").Spec.Suspended || apps.find("lastmonth").Spec.Suspended {
		t.Error("a bound card or a new month must resume the services")
	}
	if apps.suspendedIn("still") != "2026-11" {
		t.Error("a workspace still out of bandwidth without a card stays spun down")
	}
}

func TestSweepLeavesServicesWhenThePlanCannotBeRead(t *testing.T) {
	st := &fakeStore{usage: []store.MonthUsage{{WorkspaceID: "tea-out", Quantity: 6 * gb}}}
	held := app("web", "tea-out")
	held.Spec.Suspended = true
	held.Annotations = map[string]string{core.AnnotBandwidthSuspended: "2026-11"}
	apps := &fakeApps{now: now, items: []appv1alpha1.App{held}}
	w := &Sweeper{Apps: apps, Store: st, Plans: failingPlans{}}

	if err := w.SweepOnce(context.Background()); err == nil {
		t.Error("a failed plan read must be reported")
	}
	if apps.suspendedIn("web") != "2026-11" {
		t.Error("a failed plan read resumed a workspace still out of bandwidth")
	}
}
