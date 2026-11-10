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

package buildminutes

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

type fakeStore struct {
	usage  []store.MonthUsage
	claims map[string]bool // "<ws>/<stage>"
}

func (f *fakeStore) BuildSecondsByWorkspace(context.Context, time.Time, time.Time) ([]store.MonthUsage, error) {
	return f.usage, nil
}

func (f *fakeStore) ClaimBuildMinutesNotice(_ context.Context, ws string, _ time.Time, stage string) (bool, error) {
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

type fakeApps struct {
	now   time.Time
	items []appv1alpha1.App
}

func (f *fakeApps) Now() time.Time { return f.now }
func (f *fakeApps) List(context.Context) ([]appv1alpha1.App, error) {
	return f.items, nil
}
func (f *fakeApps) Mark(_ context.Context, a *appv1alpha1.App, month string) error {
	for i := range f.items {
		if f.items[i].Name != a.Name {
			continue
		}
		if month == "" {
			delete(f.items[i].Annotations, appv1alpha1.AnnotationBuildMinutesExhausted)
		} else {
			if f.items[i].Annotations == nil {
				f.items[i].Annotations = map[string]string{}
			}
			f.items[i].Annotations[appv1alpha1.AnnotationBuildMinutesExhausted] = month
		}
	}
	return nil
}

type sent struct {
	ws, stage string
	canPay    bool
}

type fakeNotifier struct{ sent []sent }

func (f *fakeNotifier) NotifyBuildMinutes(_ context.Context, ws, stage string, _, _ float64, canPay bool) error {
	f.sent = append(f.sent, sent{ws, stage, canPay})
	return nil
}

func app(name, ws string) appv1alpha1.App {
	return appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, Labels: map[string]string{core.LabelTenant: ws}}}
}

func (f *fakeApps) mark(name string) string {
	for _, a := range f.items {
		if a.Name == name {
			return a.Annotations[appv1alpha1.AnnotationBuildMinutesExhausted]
		}
	}
	return "missing"
}

// plans answers each workspace's plan; one not listed has none.
type plans map[string]string

func (p plans) WorkspacePlanOf(_ context.Context, ws string) (string, bool, error) {
	plan, ok := p[ws]
	return plan, ok, nil
}

func sweeper(st *fakeStore, apps *fakeApps, n *fakeNotifier) *Sweeper {
	return &Sweeper{Apps: apps, Store: st, Notifier: n, Plans: plans{
		"tea-out": "hobby", "tea-card": "hobby", "tea-near": "hobby",
	}}
}

func TestSweepStopsOnlyAWorkspaceOutOfMinutesThatCannotPay(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	st := &fakeStore{usage: []store.MonthUsage{
		{WorkspaceID: "tea-out", Quantity: 500 * 60},                // spent, no card
		{WorkspaceID: "tea-card", Quantity: 600 * 60, CanPay: true}, // spent, buys more
		{WorkspaceID: "tea-near", Quantity: 420 * 60},               // 84%
		{WorkspaceID: "tea-none", Quantity: 9999 * 60},              // no plan applies
	}}
	apps := &fakeApps{now: now, items: []appv1alpha1.App{
		app("web", "tea-out"), app("api", "tea-out"), app("paid", "tea-card"), app("near", "tea-near"), app("other", "tea-none"),
	}}
	n := &fakeNotifier{}

	if err := sweeper(st, apps, n).SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"web": "2026-11", "api": "2026-11", "paid": "", "near": "", "other": ""} {
		if got := apps.mark(name); got != want {
			t.Errorf("%s mark = %q, want %q", name, got, want)
		}
	}
	wantSent := map[sent]bool{{"tea-out", StageExhausted, false}: true, {"tea-card", StageExhausted, true}: true, {"tea-near", StageWarned, false}: true}
	if len(n.sent) != len(wantSent) {
		t.Fatalf("sent %v, want %v", n.sent, wantSent)
	}
	for _, s := range n.sent {
		if !wantSent[s] {
			t.Errorf("unexpected notice %+v", s)
		}
	}
	if !st.claims["tea-out/"+StageWarned] {
		t.Error("an exhausted workspace must not get the warning afterwards")
	}

	// A second sweep sends nothing again.
	n.sent = nil
	if err := sweeper(st, apps, n).SweepOnce(context.Background()); err != nil || len(n.sent) != 0 {
		t.Errorf("second sweep sent %v (err %v), want nothing", n.sent, err)
	}
}

func TestSweepLiftsTheStopWhenTheWorkspaceCanPayOrTheMonthTurns(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	web := app("web", "tea-out")
	web.Annotations = map[string]string{appv1alpha1.AnnotationBuildMinutesExhausted: "2026-11"}
	old := app("old", "tea-old")
	old.Annotations = map[string]string{appv1alpha1.AnnotationBuildMinutesExhausted: "2026-10"}
	apps := &fakeApps{now: now, items: []appv1alpha1.App{web, old}}
	// tea-out added a card; tea-old has no build minutes this month.
	st := &fakeStore{usage: []store.MonthUsage{{WorkspaceID: "tea-out", Quantity: 600 * 60, CanPay: true}}}

	if err := sweeper(st, apps, &fakeNotifier{}).SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if apps.mark("web") != "" || apps.mark("old") != "" {
		t.Errorf("marks = web %q, old %q; want both lifted", apps.mark("web"), apps.mark("old"))
	}
}

// w6/m309: a workspace that can pay stops once the overage it bought reaches
// its pipeline spend limit; below the limit it keeps buying.
func TestSweepStopsAWorkspaceAtItsSpendLimit(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	limit := int64(35) // $0.35: 100 minutes above Hobby's 500 at $0.0035
	st := &fakeStore{usage: []store.MonthUsage{
		{WorkspaceID: "tea-out", Quantity: 600 * 60, CanPay: true, SpendLimitCents: &limit},  // at the limit
		{WorkspaceID: "tea-card", Quantity: 599 * 60, CanPay: true, SpendLimitCents: &limit}, // just under it
	}}
	apps := &fakeApps{now: now, items: []appv1alpha1.App{app("web", "tea-out"), app("paid", "tea-card")}}
	n := &fakeNotifier{}

	if err := sweeper(st, apps, n).SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if apps.mark("web") != "2026-11" || apps.mark("paid") != "" {
		t.Errorf("marks = web %q, paid %q; want only the workspace at its limit stopped", apps.mark("web"), apps.mark("paid"))
	}
	var limited []string
	for _, s := range n.sent {
		if s.stage == StageLimited {
			limited = append(limited, s.ws)
		}
	}
	if len(limited) != 1 || limited[0] != "tea-out" {
		t.Errorf("limit notices = %v, want only tea-out", limited)
	}
	for _, s := range n.sent {
		if s.ws == "tea-out" && s.stage != StageLimited {
			t.Errorf("tea-out was also sent %q; a workspace stopped by its limit gets only the limit notice", s.stage)
		}
	}
}

// failingPlans fails every plan read.
type failingPlans struct{}

func (failingPlans) WorkspacePlanOf(context.Context, string) (string, bool, error) {
	return "", false, errors.New("plan store unavailable")
}

func TestSweepKeepsTheMarkWhenThePlanCannotBeRead(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	st := &fakeStore{usage: []store.MonthUsage{{WorkspaceID: "tea-out", Quantity: 600 * 60}}}
	marked := app("web", "tea-out")
	marked.Annotations = map[string]string{appv1alpha1.AnnotationBuildMinutesExhausted: appv1alpha1.BuildMinutesMonth(now)}
	apps := &fakeApps{now: now, items: []appv1alpha1.App{marked}}
	w := &Sweeper{Apps: apps, Store: st, Notifier: &fakeNotifier{}, Plans: failingPlans{}}

	if err := w.SweepOnce(context.Background()); err == nil {
		t.Error("a failed plan read must be reported")
	}
	if got := apps.mark("web"); got != appv1alpha1.BuildMinutesMonth(now) {
		t.Errorf("mark = %q: a failed plan read lifted the stop of a workspace out of minutes", got)
	}
}
