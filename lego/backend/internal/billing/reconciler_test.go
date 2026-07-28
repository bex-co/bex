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

package billing

import (
	"context"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type reconcileCapture struct {
	mappings []store.BillingProviderMapping
	events   []store.StripeBillingEvent
	touched  []string
}

func (s *reconcileCapture) TouchBillingProviderMapping(_ context.Context, workspaceID string, _ time.Time) error {
	s.touched = append(s.touched, workspaceID)
	return nil
}

func (s *reconcileCapture) ListBillingProviderMappings(_ context.Context, _ int) ([]store.BillingProviderMapping, error) {
	return s.mappings, nil
}
func (s *reconcileCapture) RecordStripeBillingEvent(_ context.Context, e store.StripeBillingEvent, _ time.Duration) (store.BillingLifecycle, bool, bool, error) {
	s.events = append(s.events, e)
	return store.BillingLifecycle{WorkspaceID: e.WorkspaceID, Status: store.BillingGrace}, true, true, nil
}

type snapshotCapture struct{ calls int }

func (p *snapshotCapture) BillingSnapshot(_ context.Context, m store.BillingProviderMapping, at time.Time) (store.StripeBillingEvent, error) {
	p.calls++
	return store.StripeBillingEvent{
		EventID: "poll:in_1:past_due:open:1", EventType: "billing.subscription.reconciled",
		WorkspaceID: m.WorkspaceID, CustomerID: m.CustomerID, SubscriptionID: m.SubscriptionID,
		ObjectID: "in_1", ProviderCreatedAt: at, ReceivedAt: at,
		Outcome: store.BillingOutcomeFailure, Reason: "subscription_past_due",
	}, nil
}

func TestReconcilerFeedsMissedProviderStateThroughSharedTransitionStore(t *testing.T) {
	now := time.Date(2026, 7, 28, 3, 0, 0, 0, time.UTC)
	s := &reconcileCapture{mappings: []store.BillingProviderMapping{{WorkspaceID: "tea-a", CustomerID: "cus_1", SubscriptionID: "sub_1"}}}
	p := &snapshotCapture{}
	r := &Reconciler{Store: s, Provider: p, GracePeriod: 7 * 24 * time.Hour, Concurrency: 2, Clock: func() time.Time { return now }}
	if err := r.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if p.calls != 1 || len(s.events) != 1 || len(s.touched) != 1 || s.touched[0] != "tea-a" {
		t.Fatalf("poll calls=%d events=%d touched=%v", p.calls, len(s.events), s.touched)
	}
	if got := s.events[0]; got.EventType != "billing.subscription.reconciled" || got.Outcome != store.BillingOutcomeFailure || !got.ProviderCreatedAt.Equal(now) {
		t.Fatalf("normalized poll event = %+v", got)
	}
}
