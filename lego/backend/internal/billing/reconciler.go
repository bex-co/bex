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
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type ReconcileStore interface {
	ListBillingProviderMappings(context.Context, int) ([]store.BillingProviderMapping, error)
	TouchBillingProviderMapping(context.Context, string, time.Time) error
	RecordStripeBillingEvent(context.Context, store.StripeBillingEvent, time.Duration) (store.BillingLifecycle, bool, bool, error)
}

type SnapshotProvider interface {
	BillingSnapshot(context.Context, store.BillingProviderMapping, time.Time) (store.StripeBillingEvent, error)
}

type Reconciler struct {
	Store       ReconcileStore
	Provider    SnapshotProvider
	GracePeriod time.Duration
	Interval    time.Duration
	Concurrency int
	Clock       func() time.Time
	Metrics     *Metrics
}

func (r *Reconciler) now() time.Time {
	if r.Clock != nil {
		return r.Clock().UTC()
	}
	return time.Now().UTC()
}

func (r *Reconciler) Run(ctx context.Context) {
	interval := r.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := r.RunOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("billing reconcile: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Reconciler) RunOnce(ctx context.Context) error {
	if r == nil || r.Store == nil || r.Provider == nil {
		return fmt.Errorf("billing reconciler dependencies unavailable")
	}
	mappings, err := r.Store.ListBillingProviderMappings(ctx, 500)
	if err != nil {
		return err
	}
	limit := r.Concurrency
	if limit <= 0 {
		limit = 4
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	errCh := make(chan error, len(mappings))
	now := r.now()
	for _, mapping := range mappings {
		mapping := mapping
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			event, err := r.Provider.BillingSnapshot(ctx, mapping, now)
			if err == nil {
				_, _, _, err = r.Store.RecordStripeBillingEvent(ctx, event, r.GracePeriod)
			}
			if touchErr := r.Store.TouchBillingProviderMapping(ctx, mapping.WorkspaceID, now); touchErr != nil {
				err = errors.Join(err, fmt.Errorf("rotate reconcile queue: %w", touchErr))
			}
			if err != nil {
				errCh <- fmt.Errorf("%s: %w", mapping.WorkspaceID, err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		r.Metrics.Operation("lifecycle_reconcile", "error")
		return err
	}
	r.Metrics.Operation("lifecycle_reconcile", "success")
	return nil
}

func (c *StripeClient) BillingSnapshot(ctx context.Context, mapping store.BillingProviderMapping, observedAt time.Time) (store.StripeBillingEvent, error) {
	params := &stripe.SubscriptionParams{}
	params.Context = ctx
	params.AddExpand("latest_invoice")
	sub, err := c.sc.Subscriptions.Get(mapping.SubscriptionID, params)
	if err != nil {
		return store.StripeBillingEvent{}, fmt.Errorf("retrieve Subscription: %w", err)
	}
	if sub.Metadata[workspaceMetadataKey] != mapping.WorkspaceID || sub.Metadata[subscriptionMetadataKey] != "true" ||
		sub.Customer == nil || sub.Customer.ID != mapping.CustomerID || sub.Livemode != mapping.Livemode {
		return store.StripeBillingEvent{}, fmt.Errorf("authoritative Subscription ownership mismatch")
	}
	outcome, reason := subscriptionOutcome(sub.Status)
	objectID, invoiceState, attemptCount := sub.ID, "none", int64(0)
	if sub.LatestInvoice != nil {
		objectID = sub.LatestInvoice.ID
		invoiceState = string(sub.LatestInvoice.Status)
		attemptCount = sub.LatestInvoice.AttemptCount
	}
	return store.StripeBillingEvent{
		EventID:   fmt.Sprintf("poll:%s:%s:%s:%d", objectID, sub.Status, invoiceState, attemptCount),
		EventType: "billing.subscription.reconciled", WorkspaceID: mapping.WorkspaceID,
		CustomerID: mapping.CustomerID, SubscriptionID: mapping.SubscriptionID, ObjectID: objectID,
		Livemode: mapping.Livemode, ProviderCreatedAt: observedAt.UTC(), ReceivedAt: observedAt.UTC(),
		Outcome: outcome, Reason: reason,
	}, nil
}
