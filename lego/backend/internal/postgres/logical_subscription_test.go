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
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const testPublisherURL = "postgresql://source_user:s3cret-pw@dpg-source.oregon-postgres.render.com:5432/source?sslmode=require"

// The subscriber form's own refusals need no SQL and write nothing: no spec
// change, no password Secret, no audit row (m542 DoD 2, t004).
func TestLogicalReplicationSubscriberRefusesBeforeWriting(t *testing.T) {
	subscription := func(mutate func(*LogicalSubscriptionRequest)) *LogicalSubscriptionRequest {
		s := &LogicalSubscriptionRequest{Name: "from_render", PublisherURL: testPublisherURL, Publication: "pub"}
		if mutate != nil {
			mutate(s)
		}
		return s
	}
	full := &appv1alpha1.DatabaseLogicalReplication{}
	for i := range appv1alpha1.LogicalSubscriptionsMax {
		full.Subscriptions = append(full.Subscriptions, appv1alpha1.DatabaseLogicalSubscription{Name: "sub_" + string(rune('a'+i)), PasswordSecret: "s"})
	}
	for _, tc := range []struct {
		name   string
		plan   string
		before *appv1alpha1.DatabaseLogicalReplication
		req    LogicalReplicationRequest
		code   string
	}{
		{name: "nothing named", plan: "pro", code: logicalReplicationInvalidRequestCode},
		{name: "a Hobby workspace", plan: "hobby", req: LogicalReplicationRequest{SubscriberRole: "cdc_reader"}, code: LogicalReplicationPlanRequiredCode},
		{name: "a subscriber role the database lacks", plan: "pro", req: LogicalReplicationRequest{SubscriberRole: "nobody"}, code: LogicalReplicationRoleUnknownCode},
		{name: "a retired subscriber role", plan: "pro", req: LogicalReplicationRequest{SubscriberRole: "old_reader"}, code: LogicalReplicationRoleUnknownCode},
		{name: "a reserved subscriber role", plan: "pro", req: LogicalReplicationRequest{SubscriberRole: "pg_create_subscription"}, code: LogicalReplicationRoleUnknownCode},
		{name: "a subscription name that is no identifier", plan: "pro", req: LogicalReplicationRequest{Subscription: subscription(func(s *LogicalSubscriptionRequest) { s.Name = "From-Render" })}, code: logicalReplicationInvalidRequestCode},
		{name: "no publication", plan: "pro", req: LogicalReplicationRequest{Subscription: subscription(func(s *LogicalSubscriptionRequest) { s.Publication = "" })}, code: logicalReplicationInvalidRequestCode},
		{name: "not a postgresql URL", plan: "pro", req: LogicalReplicationRequest{Subscription: subscription(func(s *LogicalSubscriptionRequest) {
			s.PublisherURL = "https://source_user:s3cret-pw@example.test/source"
		})}, code: logicalReplicationInvalidRequestCode},
		{name: "no password", plan: "pro", req: LogicalReplicationRequest{Subscription: subscription(func(s *LogicalSubscriptionRequest) {
			s.PublisherURL = "postgresql://source_user@example.test/source"
		})}, code: logicalReplicationInvalidRequestCode},
		{name: "no database", plan: "pro", req: LogicalReplicationRequest{Subscription: subscription(func(s *LogicalSubscriptionRequest) {
			s.PublisherURL = "postgresql://source_user:s3cret-pw@example.test"
		})}, code: logicalReplicationInvalidRequestCode},
		{name: "a parameter other than sslmode", plan: "pro", req: LogicalReplicationRequest{Subscription: subscription(func(s *LogicalSubscriptionRequest) {
			s.PublisherURL = testPublisherURL + "&passfile=/etc/shadow"
		})}, code: logicalReplicationInvalidRequestCode},
		{name: "verify-full", plan: "pro", req: LogicalReplicationRequest{Subscription: subscription(func(s *LogicalSubscriptionRequest) {
			s.PublisherURL = strings.Replace(testPublisherURL, "require", "verify-full", 1)
		})}, code: logicalReplicationInvalidRequestCode},
		{name: "one subscription too many", plan: "pro", before: full, req: LogicalReplicationRequest{Subscription: subscription(nil)}, code: logicalReplicationInvalidRequestCode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := publisherDatabase("dpg-lr")
			d.Spec.LogicalReplication = tc.before
			svc, sink := newLogicalReplicationService(t, tc.plan, d)
			req := tc.req
			req.Database, req.Enable = d.Name, true
			body, _ := json.Marshal(req)
			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, LogicalReplicationPath, strings.NewReader(string(body)))
			r.Header.Set("Authorization", "Bearer cp-token")
			LogicalReplicationHandler(svc, "cp-token").ServeHTTP(rec, r)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.code) {
				t.Fatalf("answer = %d %s, want 400 naming %s", rec.Code, rec.Body, tc.code)
			}
			if strings.Contains(rec.Body.String(), "s3cret-pw") {
				t.Fatalf("the refusal repeats the publisher's password: %s", rec.Body)
			}
			if _, err := svc.SetLogicalReplication(context.Background(), req); codeOf(err) != tc.code {
				t.Fatalf("SetLogicalReplication = %v, want %s", err, tc.code)
			}
			var got appv1alpha1.Database
			if err := svc.Client.Get(context.Background(), client.ObjectKeyFromObject(d), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Spec.LogicalReplication, tc.before) || len(sink.events) != 0 {
				t.Errorf("a refusal wrote: spec %+v, %d audit rows", got.Spec.LogicalReplication, len(sink.events))
			}
			var secrets corev1.SecretList
			if err := svc.Client.List(context.Background(), &secrets); err != nil {
				t.Fatal(err)
			}
			for _, sec := range secrets.Items {
				if strings.HasPrefix(sec.Name, d.Name+"-sub-") {
					t.Errorf("a refusal stored the password Secret %s", sec.Name)
				}
			}
		})
	}
}

func TestParseSubscriptionRequest(t *testing.T) {
	sub, password, err := parseSubscriptionRequest(LogicalSubscriptionRequest{Name: "from_render", PublisherURL: testPublisherURL, Publication: "pub"}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	want := appv1alpha1.DatabaseLogicalSubscription{
		Name: "from_render", Publication: "pub", Host: "dpg-source.oregon-postgres.render.com", Port: 5432,
		DBName: "source", User: "source_user", SSLMode: "require",
	}
	if sub != want || password != "s3cret-pw" {
		t.Fatalf("parsed = %+v / %q, want %+v and the password", sub, password, want)
	}
	// Defaults stay empty, for the operator to fill (5432, require); an
	// escaped password is unescaped.
	sub, password, err = parseSubscriptionRequest(LogicalSubscriptionRequest{Name: "s", PublisherURL: "postgres://u:p%40ss%2Fw@[2001:db8::1]/db", Publication: "pub"}, map[string]any{})
	if err != nil || sub.Port != 0 || sub.SSLMode != "" || sub.Host != "2001:db8::1" || password != "p@ss/w" {
		t.Fatalf("parsed = %+v / %q, %v", sub, password, err)
	}
}

// What a disable leaves: one subscription, one half, or nothing (nil).
func TestDisabledHalves(t *testing.T) {
	both := &appv1alpha1.DatabaseLogicalReplication{
		PublisherRole: "cdc_reader", Schemas: []string{"public"}, AllTables: true,
		SubscriberRole: "orders_user",
		Subscriptions:  []appv1alpha1.DatabaseLogicalSubscription{{Name: "a"}, {Name: "b"}},
	}
	for _, tc := range []struct {
		name string
		req  LogicalReplicationRequest
		want *appv1alpha1.DatabaseLogicalReplication
	}{
		{name: "everything", req: LogicalReplicationRequest{}, want: nil},
		{name: "both flags", req: LogicalReplicationRequest{Publisher: true, Subscriber: true}, want: nil},
		{name: "the publisher", req: LogicalReplicationRequest{Publisher: true}, want: &appv1alpha1.DatabaseLogicalReplication{
			SubscriberRole: "orders_user", Subscriptions: both.Subscriptions}},
		{name: "the subscriber", req: LogicalReplicationRequest{Subscriber: true}, want: &appv1alpha1.DatabaseLogicalReplication{
			PublisherRole: "cdc_reader", Schemas: []string{"public"}, AllTables: true}},
		{name: "one subscription", req: LogicalReplicationRequest{Subscription: &LogicalSubscriptionRequest{Name: "a"}}, want: &appv1alpha1.DatabaseLogicalReplication{
			PublisherRole: "cdc_reader", Schemas: []string{"public"}, AllTables: true, SubscriberRole: "orders_user",
			Subscriptions: []appv1alpha1.DatabaseLogicalSubscription{{Name: "b"}}}},
		{name: "a subscription it does not have", req: LogicalReplicationRequest{Subscription: &LogicalSubscriptionRequest{Name: "z"}}, want: both},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := disabledHalves(both, tc.req); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("disabledHalves = %+v, want %+v", got, tc.want)
			}
		})
	}
	// The last subscription gone leaves nothing enabled.
	only := &appv1alpha1.DatabaseLogicalReplication{Subscriptions: []appv1alpha1.DatabaseLogicalSubscription{{Name: "a"}}}
	if got := disabledHalves(only, LogicalReplicationRequest{Subscription: &LogicalSubscriptionRequest{Name: "a"}}); got != nil {
		t.Errorf("removing the last subscription = %+v, want nil", got)
	}
	if got := disabledHalves(nil, LogicalReplicationRequest{}); got != nil {
		t.Errorf("disabling a database that was never enabled = %+v", got)
	}
}

// An enable sets the halves it names and keeps the others.
func TestEnableKeepsTheOtherHalf(t *testing.T) {
	d := publisherDatabase("dpg-lr")
	d.Spec.LogicalReplication = &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}}
	svc, _ := newLogicalReplicationService(t, "pro", d)
	change, err := svc.checkLogicalReplicationRequest(context.Background(), d, LogicalReplicationRequest{Database: d.Name, Enable: true, SubscriberRole: d.Spec.EffectiveDatabaseUser(d.Name)})
	if err != nil {
		t.Fatal(err)
	}
	want := &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}, SubscriberRole: d.Spec.EffectiveDatabaseUser(d.Name)}
	if !reflect.DeepEqual(change.next, want) || change.publisher || !change.subscriberRole || change.subscription >= 0 {
		t.Fatalf("change = %+v (next %+v), want the subscriber role added to the publisher", change, change.next)
	}
	if d.Spec.LogicalReplication.SubscriberRole != "" {
		t.Error("checking the request changed the database's copy of the spec")
	}
	// A subscription of an existing name replaces it in place, keeping its
	// Secret for storeSubscriptionPassword to judge.
	d.Spec.LogicalReplication.Subscriptions = []appv1alpha1.DatabaseLogicalSubscription{{Name: "a", Publication: "old", PasswordSecret: "dpg-lr-sub-x-1"}, {Name: "b", PasswordSecret: "dpg-lr-sub-y-1"}}
	change, err = svc.checkLogicalReplicationRequest(context.Background(), d, LogicalReplicationRequest{Database: d.Name, Enable: true,
		Subscription: &LogicalSubscriptionRequest{Name: "a", PublisherURL: testPublisherURL, Publication: "new"}})
	if err != nil {
		t.Fatal(err)
	}
	if subs := change.next.Subscriptions; change.subscription != 0 || len(subs) != 2 || subs[0].Publication != "new" || subs[0].PasswordSecret != "dpg-lr-sub-x-1" || subs[1].Name != "b" {
		t.Fatalf("replaced subscriptions = %+v at %d", subs, change.subscription)
	}
}

// A subscription's password lives in a Secret of its own, owned by the
// Database: the same password keeps it, a new one gets a new Secret.
func TestStoreSubscriptionPassword(t *testing.T) {
	d := publisherDatabase("dpg-lr")
	d.UID = "uid-dpg-lr"
	svc, _ := newLogicalReplicationService(t, "pro", d)
	ctx := context.Background()
	entry := &appv1alpha1.DatabaseLogicalSubscription{Name: "from_render", User: "source_user"}
	if err := svc.storeSubscriptionPassword(ctx, d, entry, "first"); err != nil {
		t.Fatal(err)
	}
	first := entry.PasswordSecret
	if !strings.HasPrefix(first, appv1alpha1.LogicalSubscriptionObjectName(d.Name, "from_render")+"-") {
		t.Fatalf("Secret name %q", first)
	}
	var sec corev1.Secret
	if err := svc.Client.Get(ctx, client.ObjectKey{Namespace: d.Namespace, Name: first}, &sec); err != nil {
		t.Fatal(err)
	}
	if string(sec.Data["password"]) != "first" || len(sec.OwnerReferences) != 1 || sec.OwnerReferences[0].UID != d.UID || sec.OwnerReferences[0].Kind != "Database" {
		t.Fatalf("Secret = %+v", sec)
	}
	if err := svc.storeSubscriptionPassword(ctx, d, entry, "first"); err != nil || entry.PasswordSecret != first {
		t.Fatalf("the same password again = %v, Secret %s", err, entry.PasswordSecret)
	}
	if err := svc.storeSubscriptionPassword(ctx, d, entry, "second"); err != nil || entry.PasswordSecret == first {
		t.Fatalf("a new password = %v, Secret %s", err, entry.PasswordSecret)
	}
	if problems := svc.deleteSubscriptionSecrets(ctx, d, []string{first, "already-gone"}); len(problems) != 0 {
		t.Fatalf("delete = %v", problems)
	}
	if err := svc.Client.Get(ctx, client.ObjectKey{Namespace: d.Namespace, Name: first}, &sec); err == nil {
		t.Error("the replaced Secret survived")
	}
}

func TestRemovedSubscriptionSecrets(t *testing.T) {
	previous := &appv1alpha1.DatabaseLogicalReplication{Subscriptions: []appv1alpha1.DatabaseLogicalSubscription{{Name: "a", PasswordSecret: "sa"}, {Name: "b", PasswordSecret: "sb"}}}
	next := &appv1alpha1.DatabaseLogicalReplication{Subscriptions: []appv1alpha1.DatabaseLogicalSubscription{{Name: "b", PasswordSecret: "sb"}}}
	if got := removedSubscriptionSecrets(previous, next); !reflect.DeepEqual(got, []string{"sa"}) {
		t.Errorf("one removed = %v", got)
	}
	if got := removedSubscriptionSecrets(previous, nil); !reflect.DeepEqual(got, []string{"sa", "sb"}) {
		t.Errorf("all removed = %v", got)
	}
	if got := removedSubscriptionSecrets(nil, next); got != nil {
		t.Errorf("nothing before = %v", got)
	}
}
