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
	"slices"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m126: ADR009 §Free-plan expiry.

type expiryNotice struct {
	database, stage     string
	expiresAt, deleteAt time.Time
}

type fakeExpiryNotifier struct{ notices []expiryNotice }

func (f *fakeExpiryNotifier) NotifyFreeDatabaseExpiry(_ context.Context, _, databaseID, _, stage string, expiresAt, deleteAt time.Time) error {
	f.notices = append(f.notices, expiryNotice{databaseID, stage, expiresAt, deleteAt})
	return nil
}

// created is after the rollout, so the database expires 30 days later.
var expiryCreated = time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC)

func freeDatabase(name, plan string, created time.Time) *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "default", CreationTimestamp: metav1.NewTime(created),
			Labels: map[string]string{core.LabelTenant: "tea-expiry"},
		},
		Spec:   appv1alpha1.DatabaseSpec{Name: "orders", Plan: plan},
		Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady},
	}
}

type expiryHarness struct {
	t        *testing.T
	svc      *Service
	cl       client.Client
	now      time.Time
	notifier *fakeExpiryNotifier
	audit    *webhookAuditSink
	sweeper  *FreeExpirySweeper
}

func newExpiryHarness(t *testing.T, dbs ...*appv1alpha1.Database) *expiryHarness {
	objs := make([]client.Object, len(dbs))
	for i, d := range dbs {
		objs[i] = d
	}
	svc, cl := newService(objs...)
	h := &expiryHarness{t: t, svc: svc, cl: cl, notifier: &fakeExpiryNotifier{}, audit: &webhookAuditSink{}}
	svc.Clock = func() time.Time { return h.now }
	svc.Audit = h.audit
	svc.Retirements = &fakeRetirements{}
	h.sweeper = &FreeExpirySweeper{Service: svc, Notifier: h.notifier}
	return h
}

func (h *expiryHarness) sweepAt(at time.Time) {
	h.t.Helper()
	h.now = at
	if err := h.sweeper.SweepOnce(context.Background()); err != nil {
		h.t.Fatalf("SweepOnce at %s: %v", at, err)
	}
}

func (h *expiryHarness) get(name string) (*appv1alpha1.Database, error) {
	var d appv1alpha1.Database
	err := h.cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &d)
	return &d, err
}

func (h *expiryHarness) stages() []string {
	var out []string
	for _, n := range h.notifier.notices {
		out = append(out, n.stage)
	}
	return out
}

func TestFreeExpiresAt(t *testing.T) {
	expires := expiryCreated.Add(30 * 24 * time.Hour)
	beforeRollout := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	downgraded := freeDatabase("dpg-c", "free", beforeRollout)
	downgraded.Annotations = map[string]string{annotationFreeSince: "2026-12-01T00:00:00Z"}
	cases := []struct {
		name string
		d    *appv1alpha1.Database
		want time.Time
		ok   bool
	}{
		{"free: created + 30 days", freeDatabase("dpg-a", "free", expiryCreated), expires, true},
		{"default plan is free", freeDatabase("dpg-a", "", expiryCreated), expires, true},
		{"paid: never", freeDatabase("dpg-a", "basic-256mb", expiryCreated), time.Time{}, false},
		{"pre-rollout: rollout + 30 days", freeDatabase("dpg-b", "free", beforeRollout), FreeExpiryRollout.Add(30 * 24 * time.Hour), true},
		{"changed to free: that time + 30 days", downgraded, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), true},
	}
	for _, c := range cases {
		got, ok := FreeExpiresAt(c.d)
		if ok != c.ok || !got.Equal(c.want) {
			t.Errorf("%s: FreeExpiresAt = %s, %v; want %s, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestDueFreeExpiryStage(t *testing.T) {
	e := expiryCreated.Add(FreeLifetime)
	day := 24 * time.Hour
	cases := []struct {
		at   time.Time
		want freeExpiryStage
	}{
		{e.Add(-7*day - time.Second), stageNone},
		{e.Add(-7 * day), stageWarned},
		{e.Add(-time.Second), stageWarned},
		{e, stageExpired},
		{e.Add(11*day - time.Second), stageExpired},
		{e.Add(11 * day), stageFinalWarned},
		{e.Add(14*day - time.Second), stageFinalWarned},
		{e.Add(14 * day), stageDeleted},
	}
	for _, c := range cases {
		if got := dueFreeExpiryStage(e, c.at); got != c.want {
			t.Errorf("dueFreeExpiryStage(%s) = %v, want %v", c.at.Sub(e), got, c.want)
		}
	}
}

func TestPostgresViewReportsExpiresAtOnFreeOnly(t *testing.T) {
	if got := pgView(freeDatabase("dpg-a", "free", expiryCreated)).ExpiresAt; got != "2026-12-02T09:30:00Z" {
		t.Errorf("free ExpiresAt = %q, want 2026-12-02T09:30:00Z", got)
	}
	paid := pgView(freeDatabase("dpg-a", "basic-256mb", expiryCreated))
	if paid.ExpiresAt != "" {
		t.Errorf("paid ExpiresAt = %q, want omitted", paid.ExpiresAt)
	}
	raw, _ := json.Marshal(paid)
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	if _, present := fields["expiresAt"]; present {
		t.Errorf("paid JSON carries expiresAt: %s", raw)
	}
}

// Each stage happens once, in order, with an audit row and one notice, and
// the delete goes through the user delete's path (usage retired first).
func TestFreeExpirySweeperDrivesEveryStageOnce(t *testing.T) {
	h := newExpiryHarness(t, freeDatabase("dpg-a", "free", expiryCreated), freeDatabase("dpg-paid", "basic-256mb", expiryCreated))
	e := expiryCreated.Add(FreeLifetime)
	day := 24 * time.Hour

	h.sweepAt(e.Add(-8 * day))
	if len(h.notifier.notices) != 0 {
		t.Fatalf("notices before the warning window = %v", h.stages())
	}
	h.sweepAt(e.Add(-7 * day))
	h.sweepAt(e.Add(-6 * day)) // same stage again: nothing new
	if got := h.stages(); len(got) != 1 || got[0] != "warned" {
		t.Fatalf("stages after warning = %v, want [warned]", got)
	}
	if n := h.notifier.notices[0]; !n.expiresAt.Equal(e) || !n.deleteAt.Equal(e.Add(14*day)) {
		t.Fatalf("warning notice dates = %s / %s", n.expiresAt, n.deleteAt)
	}

	h.sweepAt(e)
	d, err := h.get("dpg-a")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Spec.Suspended {
		t.Fatal("expired database is not suspended")
	}
	view := h.svc.view(d)
	if view.Status != "suspended" || len(view.Suspenders) != 1 || view.Suspenders[0] != "billing" || view.StatusReasonCode != "POSTGRES_FREE_EXPIRED" {
		t.Fatalf("expired view = status %q suspenders %v reason %q", view.Status, view.Suspenders, view.StatusReasonCode)
	}
	if _, err := h.svc.Resume(ctxAs("user-a"), "dpg-a"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("Resume of an expired database = %v, want a bad request", err)
	}

	h.sweepAt(e.Add(11 * day))
	h.sweepAt(e.Add(14*day - time.Second))
	if _, err := h.get("dpg-a"); err != nil {
		t.Fatalf("database deleted before the grace period ended: %v", err)
	}
	h.sweepAt(e.Add(14 * day))
	h.sweepAt(e.Add(15 * day))
	if _, err := h.get("dpg-a"); !apierrors.IsNotFound(err) {
		t.Fatalf("database after grace = %v, want deleted", err)
	}
	if got := h.svc.Retirements.(*fakeRetirements).recorded; len(got) != 1 {
		t.Fatalf("usage retirements = %d, want 1 (the user delete's path)", len(got))
	}

	want := []string{"warned", "expired", "final-warned", "deleted"}
	if got := h.stages(); !slices.Equal(got, want) {
		t.Fatalf("stages = %v, want %v", got, want)
	}
	var verbs []string
	for _, ev := range h.audit.events {
		if !strings.HasPrefix(ev.Verb, "postgres.FreeExpiry") {
			continue // the refused Resume's own authorization row
		}
		verbs = append(verbs, ev.Verb)
		if ev.Caller != "free-expiry-sweeper" || ev.Target != core.DatabaseTarget("dpg-a") {
			t.Errorf("audit row %s caller %q target %q", ev.Verb, ev.Caller, ev.Target)
		}
	}
	wantVerbs := []string{"postgres.FreeExpiryWarned", "postgres.FreeExpiryExpired", "postgres.FreeExpiryFinalWarned", "postgres.FreeExpiryDeleted"}
	if !slices.Equal(verbs, wantVerbs) {
		t.Fatalf("audit verbs = %v, want %v", verbs, wantVerbs)
	}
	if _, err := h.get("dpg-paid"); err != nil {
		t.Fatalf("paid database touched: %v", err)
	}
}

// A sweeper that was down past several stages takes only the furthest one:
// nobody is warned about an expiry that already happened.
func TestFreeExpirySweeperCollapsesOverdueStages(t *testing.T) {
	h := newExpiryHarness(t, freeDatabase("dpg-a", "free", expiryCreated))
	h.sweepAt(expiryCreated.Add(FreeLifetime + 2*24*time.Hour))
	if got := h.stages(); !slices.Equal(got, []string{"expired"}) {
		t.Fatalf("stages = %v, want [expired]", got)
	}
}

// Two replicas holding the same read: only the first claim lands, so the
// second sends nothing and deletes nothing.
func TestFreeExpirySweeperClaimsEachStageOnceAcrossReplicas(t *testing.T) {
	h := newExpiryHarness(t, freeDatabase("dpg-a", "free", expiryCreated))
	e := expiryCreated.Add(FreeLifetime)
	h.now = e
	stale, err := h.get("dpg-a")
	if err != nil {
		t.Fatal(err)
	}
	other := &FreeExpirySweeper{Service: h.svc, Notifier: h.notifier}
	if err := h.sweeper.advance(context.Background(), stale.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	if err := other.advance(context.Background(), stale.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	if got := h.stages(); !slices.Equal(got, []string{"expired"}) {
		t.Fatalf("stages after two replicas = %v, want one expired", got)
	}

	h.now = e.Add(FreeExpiryGrace)
	stale, err = h.get("dpg-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.sweeper.advance(context.Background(), stale.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	if err := other.advance(context.Background(), stale.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	if got := h.stages(); !slices.Equal(got, []string{"expired", "deleted"}) {
		t.Fatalf("stages after two deletes = %v, want one deleted", got)
	}
	if got := h.svc.Retirements.(*fakeRetirements).recorded; len(got) > 2 {
		t.Fatalf("retirements = %d", len(got))
	}
}

// Upgrading drops expiresAt and resumes the suspension the expiry made; a
// user's own suspension stays.
func TestUpgradeClearsFreeExpiry(t *testing.T) {
	for _, userSuspended := range []bool{false, true} {
		d := freeDatabase("dpg-a", "free", expiryCreated)
		d.Spec.Suspended = userSuspended
		h := newExpiryHarness(t, d)
		h.sweepAt(expiryCreated.Add(FreeLifetime))
		view, err := h.svc.SetPlan(ctxAs("user-a"), "dpg-a", "basic-256mb")
		if err != nil {
			t.Fatalf("SetPlan: %v", err)
		}
		if view.ExpiresAt != "" {
			t.Errorf("upgraded ExpiresAt = %q, want omitted", view.ExpiresAt)
		}
		got, _ := h.get("dpg-a")
		if got.Spec.Suspended != userSuspended {
			t.Errorf("user suspended %v: suspended after upgrade = %v", userSuspended, got.Spec.Suspended)
		}
		for _, key := range []string{annotationFreeExpiry, annotationFreeExpirySuspended, annotationFreeSince} {
			if _, ok := got.Annotations[key]; ok {
				t.Errorf("annotation %s survives the upgrade", key)
			}
		}
		h.sweepAt(expiryCreated.Add(FreeLifetime + FreeExpiryGrace))
		if _, err := h.get("dpg-a"); err != nil {
			t.Fatalf("upgraded database deleted: %v", err)
		}
	}
}

// A change back to free starts a new 30-day clock, and the PATCH path does
// the same as SetPlan.
func TestDowngradeToFreeStartsANewClock(t *testing.T) {
	h := newExpiryHarness(t, freeDatabase("dpg-a", "basic-256mb", expiryCreated))
	h.now = time.Date(2027, 3, 1, 12, 0, 0, 0, time.UTC)
	free := "free"
	view, err := h.svc.UpdatePostgres(ctxAs("user-a"), "dpg-a", PostgresPatch{Plan: &free})
	if err != nil {
		t.Fatalf("UpdatePostgres: %v", err)
	}
	if view.ExpiresAt != "2027-03-31T12:00:00Z" {
		t.Fatalf("ExpiresAt after downgrade = %q, want 2027-03-31T12:00:00Z", view.ExpiresAt)
	}
	h.sweepAt(h.now.Add(24 * time.Hour))
	if len(h.notifier.notices) != 0 {
		t.Fatalf("a fresh clock produced notices: %v", h.stages())
	}
}

// A free database that predates the rule is never deleted earlier than
// rollout + 30 days + grace, however old it is.
func TestPreRolloutFreeDatabaseGetsTheRolloutWindow(t *testing.T) {
	old := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	h := newExpiryHarness(t, freeDatabase("dpg-old", "free", old))
	h.sweepAt(time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC))
	if len(h.notifier.notices) != 0 {
		t.Fatalf("notices at rollout = %v, want none", h.stages())
	}
	safe := FreeExpiryRollout.Add(FreeLifetime + FreeExpiryGrace)
	h.sweepAt(safe.Add(-time.Second))
	if _, err := h.get("dpg-old"); err != nil {
		t.Fatalf("pre-rollout database deleted before %s: %v", safe, err)
	}
	h.sweepAt(safe)
	if _, err := h.get("dpg-old"); !apierrors.IsNotFound(err) {
		t.Fatalf("pre-rollout database at %s = %v, want deleted", safe, err)
	}
}
