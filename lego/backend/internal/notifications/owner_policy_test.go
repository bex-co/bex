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

package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The workspace notification policy (w3/m271): Render's owner-level
// notification settings, and how a service on "default" follows it.

func staticOwners(p OwnerPolicy) OwnerPolicyLookup {
	return func(context.Context, string) (OwnerPolicy, error) { return p, nil }
}

// ownerRoleChecker models one workspace's roles: admin holds every relation,
// developer everything but can_manage, viewer only can_view; anyone else
// (including every subject on any other workspace) is denied.
type ownerRoleChecker struct {
	workspace string
	roles     map[string]string
}

func (c ownerRoleChecker) Check(_ context.Context, subject, relation, object string) (bool, error) {
	if object != "workspace:"+c.workspace {
		return false, nil
	}
	switch c.roles[strings.TrimPrefix(subject, "user:")] {
	case "admin":
		return true, nil
	case "developer":
		return relation != core.RelCanManage, nil
	case "viewer":
		return relation == core.RelCanView, nil
	}
	return false, nil
}

func ownerSettingsService(st NotificationsStore) *Service {
	return &Service{
		Base: &core.Base{Authz: ownerRoleChecker{workspace: "tea-a", roles: map[string]string{
			"admin": "admin", "dev": "developer", "viewer": "viewer",
		}}},
		Store: st,
	}
}

func as(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func codeOf(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

// TestOwnerSettingsDefaultsAndAdminWrites: any member reads Render's default
// (email on, failure only, no Slack, no previews) before anything is saved;
// only an admin writes, and a partial PATCH keeps the field it omits.
func TestOwnerSettingsDefaultsAndAdminWrites(t *testing.T) {
	st := newFakeStore()
	svc := ownerSettingsService(st)
	want := OwnerSettingsView{OwnerID: "tea-a", EmailEnabled: true, NotificationsToSend: "failure"}
	for _, who := range []string{"admin", "dev", "viewer"} {
		got, err := svc.GetOwnerSettings(as(who), "tea-a")
		if err != nil || got != want {
			t.Fatalf("%s reads %+v (%v), want Render's default %+v", who, got, err, want)
		}
	}

	all := "all"
	for _, who := range []string{"dev", "viewer"} {
		if _, err := svc.UpdateOwnerSettings(as(who), "tea-a", OwnerSettingsInput{NotificationsToSend: &all}); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("%s write error = %v, want ErrForbidden (admin only)", who, err)
		}
	}
	if len(st.owners) != 0 {
		t.Fatalf("a refused write stored %+v", st.owners)
	}
	if _, err := svc.UpdateOwnerSettings(as("admin"), "tea-other", OwnerSettingsInput{NotificationsToSend: &all}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("admin of tea-a writing tea-other = %v, want ErrForbidden", err)
	}

	got, err := svc.UpdateOwnerSettings(as("admin"), "tea-a", OwnerSettingsInput{NotificationsToSend: &all})
	if err != nil || got.NotificationsToSend != "all" || !got.EmailEnabled || got.SlackEnabled || got.PreviewNotificationsEnabled {
		t.Fatalf("admin sets all = %+v (%v)", got, err)
	}
	off, no := false, false
	got, err = svc.UpdateOwnerSettings(as("admin"), "tea-a", OwnerSettingsInput{EmailEnabled: &off, PreviewNotificationsEnabled: &no})
	if err != nil || got.EmailEnabled || got.NotificationsToSend != "all" {
		t.Fatalf("partial PATCH = %+v (%v), want email off and the level kept", got, err)
	}
	if read, err := svc.GetOwnerSettings(as("viewer"), "tea-a"); err != nil || read != got {
		t.Fatalf("viewer reads back %+v (%v), want %+v", read, err, got)
	}
}

// TestOwnerSettingsRefusals: previewNotificationsEnabled:true is refused by
// field name (PR previews are a non-goal), "default" or an unknown level is
// refused, and nothing is written by a refused PATCH, even its valid fields.
func TestOwnerSettingsRefusals(t *testing.T) {
	st := newFakeStore()
	svc := ownerSettingsService(st)
	yes, off := true, false
	if _, err := svc.UpdateOwnerSettings(as("admin"), "tea-a", OwnerSettingsInput{EmailEnabled: &off, PreviewNotificationsEnabled: &yes}); codeOf(err) != "NOTIFICATION_PREVIEWS_UNSUPPORTED" || !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("preview true error = %v, want 400 NOTIFICATION_PREVIEWS_UNSUPPORTED", err)
	} else if coded := (*core.CodedError)(nil); errors.As(err, &coded) && coded.Params["field"] != "previewNotificationsEnabled" {
		t.Fatalf("preview refusal params = %v, want the field named", coded.Params)
	}
	for _, level := range []string{"default", "everything", ""} {
		if _, err := svc.UpdateOwnerSettings(as("admin"), "tea-a", OwnerSettingsInput{EmailEnabled: &off, NotificationsToSend: &level}); codeOf(err) != "NOTIFICATION_LEVEL_INVALID" {
			t.Fatalf("level %q error = %v, want NOTIFICATION_LEVEL_INVALID", level, err)
		}
	}
	if len(st.owners) != 0 {
		t.Fatalf("a refused PATCH stored %+v", st.owners)
	}

	nilStore := ownerSettingsService(nil)
	if _, err := nilStore.GetOwnerSettings(as("admin"), "tea-a"); !errors.Is(err, core.ErrNotificationsUnavailable) {
		t.Fatalf("no store read = %v, want ErrNotificationsUnavailable", err)
	}
	if _, err := nilStore.UpdateOwnerSettings(as("admin"), "tea-a", OwnerSettingsInput{EmailEnabled: &off}); !errors.Is(err, core.ErrNotificationsUnavailable) {
		t.Fatalf("no store write = %v, want ErrNotificationsUnavailable", err)
	}
	st.ownerErr = errors.New("db down")
	if _, err := svc.GetOwnerSettings(as("admin"), "tea-a"); err == nil {
		t.Fatal("a store read failure must not answer the default as if it were saved")
	}
}

// TestEmailPolicyLayering is the decision table ADR052 D2 states: email off
// sends nothing; an explicit service level mails every member; a service on
// default follows the workspace level, narrowed by each member's preferences —
// a member on the failure-only default follows the workspace for every event.
func TestEmailPolicyLayering(t *testing.T) {
	onDefault := memberPrefs{failed: true}
	optedOutOfFailures := memberPrefs{failed: false}
	onlySuccess := memberPrefs{succeeded: true, failed: true}
	ws := func(level string) OwnerPolicy { return OwnerPolicy{EmailEnabled: true, NotificationsToSend: level} }
	for _, tc := range []struct {
		name    string
		owner   OwnerPolicy
		service string
		level   notifyLevel
		toggle  memberToggle
		member  memberPrefs
		want    bool
	}{
		{"default ws, failure, default member", defaultOwnerPolicy, "default", levelFailure, toggleFailed, onDefault, true},
		{"default ws, failure, opted-out member", defaultOwnerPolicy, "default", levelFailure, toggleFailed, optedOutOfFailures, false},
		{"default ws, success not sent", defaultOwnerPolicy, "default", levelAll, toggleSucceeded, onlySuccess, false},
		{"ws all, success to default member", ws("all"), "default", levelAll, toggleSucceeded, onDefault, true},
		{"ws all, started to default member", ws("all"), "default", levelAll, toggleStarted, onDefault, true},
		{"ws all, started not to member who chose only success", ws("all"), "default", levelAll, toggleStarted, onlySuccess, false},
		{"ws all, success to member who chose it", ws("all"), "default", levelAll, toggleSucceeded, onlySuccess, true},
		{"ws all, no-toggle event to everyone", ws("all"), "", levelAll, toggleNone, optedOutOfFailures, true},
		{"ws none suppresses failure", ws("none"), "default", levelFailure, toggleFailed, onDefault, false},
		{"service failure overrides ws none", ws("none"), "failure", levelFailure, toggleFailed, optedOutOfFailures, true},
		{"service none overrides ws all", ws("all"), "none", levelFailure, toggleFailed, onDefault, false},
		{"service all overrides ws failure", ws("failure"), "all", levelAll, toggleSucceeded, onDefault, true},
		{"email off silences an explicit override", OwnerPolicy{EmailEnabled: false, NotificationsToSend: "all"}, "all", levelFailure, toggleFailed, onDefault, false},
	} {
		if got := (emailPolicy{owner: tc.owner, service: tc.service}).wants(tc.level, tc.toggle, tc.member); got != tc.want {
			t.Errorf("%s: wants = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestDeployMailFollowsTheWorkspacePolicy drives the deploy fan-out end to
// end: "all" mails a deploy going live to a default-following service's
// members, "none" suppresses a failure, an explicit override still wins, and
// email off sends nothing.
func TestDeployMailFollowsTheWorkspacePolicy(t *testing.T) {
	identities := fakeIdentities{"alice": "alice@example.com", "bob": "bob@example.com"}
	run := func(t *testing.T, owner *store.NotificationOwnerPolicy, status, toSend string) []string {
		t.Helper()
		st := newFakeStore()
		st.recipients["tea-a"] = []store.NotifyRecipient{
			{Subject: "alice", DeployFailed: true},                     // on the default
			{Subject: "bob", DeployStarted: true, DeployFailed: false}, // customized: started only
		}
		if owner != nil {
			st.owners = map[string]store.NotificationOwnerPolicy{"tea-a": *owner}
		}
		mailer := &fakeMailer{}
		svc := newTestService(st, nil, mailer, identities)
		svc.NotifyDeploy(context.Background(), store.DeployNotification{TenantID: "tea-a", AppName: "web", Status: status, NotificationsToSend: toSend})
		var to []string
		for _, m := range mailer.sent {
			to = append(to, m.to)
		}
		return to
	}
	all := &store.NotificationOwnerPolicy{EmailEnabled: true, NotificationsToSend: "all"}
	none := &store.NotificationOwnerPolicy{EmailEnabled: true, NotificationsToSend: "none"}
	emailOff := &store.NotificationOwnerPolicy{EmailEnabled: false, NotificationsToSend: "all"}

	if got := run(t, nil, store.DeployLive, "default"); len(got) != 0 {
		t.Errorf("Render's default workspace policy mailed a deploy going live to %v", got)
	}
	if got := run(t, all, store.DeployLive, "default"); !equalUnordered(got, []string{"alice@example.com"}) {
		t.Errorf("workspace all, live: mailed %v, want alice (bob chose started only)", got)
	}
	if got := run(t, none, store.DeployUpdateFailed, "default"); len(got) != 0 {
		t.Errorf("workspace none mailed a failure to %v", got)
	}
	if got := run(t, none, store.DeployUpdateFailed, "failure"); !equalUnordered(got, []string{"alice@example.com", "bob@example.com"}) {
		t.Errorf("service failure under workspace none: mailed %v, want every member", got)
	}
	if got := run(t, emailOff, store.DeployUpdateFailed, "all"); len(got) != 0 {
		t.Errorf("email off mailed %v", got)
	}

	// The request-time started mail follows the same policy.
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{{Subject: "alice", DeployFailed: true}}
	st.owners = map[string]store.NotificationOwnerPolicy{"tea-a": *all}
	mailer := &fakeMailer{}
	newTestService(st, nil, mailer, identities).NotifyDeployStarted(context.Background(), "tea-a", "web", "default")
	if len(mailer.sent) != 1 || mailer.sent[0].subject != "Deploy started: web" {
		t.Errorf("workspace all, started: sent %+v, want alice's started mail", mailer.sent)
	}

	// A policy that cannot be read sends nothing rather than a guess.
	st.ownerErr = errors.New("db down")
	mailer = &fakeMailer{}
	newTestService(st, nil, mailer, identities).NotifyDeploy(context.Background(), store.DeployNotification{TenantID: "tea-a", AppName: "web", Status: store.DeployUpdateFailed})
	if len(mailer.sent) != 0 {
		t.Errorf("unreadable policy: sent %+v", mailer.sent)
	}
}

// TestAlertFeedFollowsTheWorkspacePolicy: the failure-alert worker resolves a
// default-following service from the workspace policy, and a policy read
// failure fails the page (retried next tick) instead of mailing on a guess.
func TestAlertFeedFollowsTheWorkspacePolicy(t *testing.T) {
	byTenant := map[string][]store.AlertEmailRecipient{"tea-a": {{TenantID: "tea-a", Subject: "alice", DeployFailed: true}}}
	rows := []store.WebhookEventRow{{TenantID: "tea-a", ServiceID: "srv-1", Key: "fact:1", Source: store.EventSourceFact, FactType: string(store.EventFactServerFailed)}}
	w := &AlertEmailWorker{Policy: func(context.Context, string, string) (string, error) {
		return appv1alpha1.NotificationsToSendDefault, nil
	}}
	if items, err := w.projectFeed(context.Background(), rows, byTenant); err != nil || len(items) != 1 {
		t.Fatalf("Render's default: items=%v err=%v, want alice's alert", items, err)
	}
	w.Owners = staticOwners(OwnerPolicy{EmailEnabled: true, NotificationsToSend: "none"})
	if items, err := w.projectFeed(context.Background(), rows, byTenant); err != nil || len(items) != 0 {
		t.Fatalf("workspace none: items=%v err=%v, want none", items, err)
	}
	w.Owners = func(context.Context, string) (OwnerPolicy, error) { return OwnerPolicy{}, errors.New("db down") }
	if _, err := w.projectFeed(context.Background(), rows, byTenant); err == nil {
		t.Fatal("an unreadable workspace policy must fail the page")
	}
}

// TestPushFollowsTheServiceLevel: a default-following service's pushes follow
// the workspace level (none drops a failure, failure drops a deploy going
// live, all sends it), an explicit override wins, and emailEnabled does not
// touch push. The member's own push policy still filters afterwards.
func TestPushFollowsTheServiceLevel(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	failure := store.WebhookEventRow{Key: "dep-x:ended", Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, Status: store.DeployUpdateFailed}
	live := store.WebhookEventRow{Key: "dep-y:ended", Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, Status: store.DeployLive}
	everyEvent := func(t *testing.T) []byte {
		return encodedPushPolicy(t, func(s *PushSettingsView) {
			s.Events = append([]DeliveryEvent(nil), orderedDeliveryEvents...)
			s.MinimumUrgency = DeliveryUrgencyRoutine
		})
	}
	for _, tc := range []struct {
		name      string
		owner     *OwnerPolicy
		service   string
		event     store.WebhookEventRow
		wantQueue int
	}{
		{"no workspace lookup keeps push unchanged", nil, "default", live, 1},
		{"default workspace drops a deploy going live", &defaultOwnerPolicy, "default", live, 0},
		{"default workspace sends a failure", &defaultOwnerPolicy, "default", failure, 1},
		{"workspace all sends a deploy going live", &OwnerPolicy{EmailEnabled: true, NotificationsToSend: "all"}, "default", live, 1},
		{"workspace none drops a failure", &OwnerPolicy{EmailEnabled: true, NotificationsToSend: "none"}, "default", failure, 0},
		{"service override all wins over workspace none", &OwnerPolicy{EmailEnabled: true, NotificationsToSend: "none"}, "all", live, 1},
		{"service override none wins over workspace all", &OwnerPolicy{EmailEnabled: true, NotificationsToSend: "all"}, "none", failure, 0},
		{"email off does not silence push", &OwnerPolicy{EmailEnabled: false, NotificationsToSend: "failure"}, "default", failure, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queue := policyProjectionQueue(now, everyEvent(t), tc.event)
			service := tc.service
			w := &PushWorker{Store: queue, Clock: func() time.Time { return now },
				Policy: func(context.Context, string, string) (string, error) { return service, nil }}
			if tc.owner != nil {
				w.Owners = staticOwners(*tc.owner)
			}
			if err := w.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			queue.mu.Lock()
			defer queue.mu.Unlock()
			if len(queue.notifications) != tc.wantQueue {
				t.Fatalf("queued %d pushes, want %d", len(queue.notifications), tc.wantQueue)
			}
		})
	}
}
