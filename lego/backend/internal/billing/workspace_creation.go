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
	"fmt"
	"log"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

const workspaceCreationAttemptMetadataKey = "bex_workspace_creation_attempt"

// WorkspaceSetup contains only browser-safe setup state. ClientSecret is
// returned to the authorized attempt owner and is never stored by bex.
type WorkspaceSetup struct {
	CustomerID     string
	SetupIntentID  string
	ClientSecret   string
	PublishableKey string
	Livemode       bool
}

type VerifiedWorkspaceSetup struct {
	PaymentMethodID string
}

// PrepareWorkspaceSetup creates or re-reads the dedicated Customer and
// SetupIntent for one reserved workspace. It never consults a current
// workspace mapping, and deliberately omits payment_method_types so Stripe's
// dynamic payment-method configuration remains authoritative.
func (c *StripeClient) PrepareWorkspaceSetup(ctx context.Context, attemptID, workspaceID, billingEmail, customerID, setupIntentID string) (WorkspaceSetup, error) {
	if c.publishableKey == "" {
		return WorkspaceSetup{}, fmt.Errorf("stripe: publishable key is not configured")
	}
	customer, err := c.workspaceCreationCustomer(ctx, attemptID, workspaceID, billingEmail, customerID)
	if err != nil {
		return WorkspaceSetup{}, err
	}
	setup, err := c.workspaceCreationSetupIntent(ctx, attemptID, workspaceID, customer.ID, setupIntentID)
	if err != nil {
		return WorkspaceSetup{}, err
	}
	return WorkspaceSetup{
		CustomerID: customer.ID, SetupIntentID: setup.ID,
		ClientSecret: setup.ClientSecret, PublishableKey: c.publishableKey,
		Livemode: setup.Livemode,
	}, nil
}

func (c *StripeClient) workspaceCreationCustomer(ctx context.Context, attemptID, workspaceID, billingEmail, customerID string) (*stripe.Customer, error) {
	if customerID != "" {
		params := &stripe.CustomerParams{}
		params.Context = ctx
		customer, err := c.sc.Customers.Get(customerID, params)
		if err != nil {
			return nil, fmt.Errorf("stripe: retrieve workspace-create Customer: %w", err)
		}
		if customer.Metadata[workspaceMetadataKey] != workspaceID || customer.Metadata[workspaceCreationAttemptMetadataKey] != attemptID || customer.Email != billingEmail || !c.expectedLivemode(customer.Livemode) {
			return nil, &inputError{message: "workspace-create Customer correlation is invalid"}
		}
		return customer, nil
	}
	params := &stripe.CustomerParams{
		Name: stripe.String(workspaceID), Email: stripe.String(billingEmail),
		Metadata: map[string]string{
			workspaceMetadataKey:                workspaceID,
			workspaceCreationAttemptMetadataKey: attemptID,
		},
	}
	params.Context = ctx
	params.SetIdempotencyKey("bex-workspace-create-customer-" + attemptID)
	customer, err := c.sc.Customers.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe: create workspace Customer: %w", err)
	}
	if customer.ID == "" || !c.expectedLivemode(customer.Livemode) {
		return nil, &inputError{message: "workspace-create Customer mode is invalid"}
	}
	return customer, nil
}

func (c *StripeClient) workspaceCreationSetupIntent(ctx context.Context, attemptID, workspaceID, customerID, setupIntentID string) (*stripe.SetupIntent, error) {
	if setupIntentID != "" {
		params := &stripe.SetupIntentParams{}
		params.Context = ctx
		setup, err := c.sc.SetupIntents.Get(setupIntentID, params)
		if err != nil {
			return nil, fmt.Errorf("stripe: retrieve workspace SetupIntent: %w", err)
		}
		if err := c.validateWorkspaceSetup(setup, attemptID, workspaceID, customerID); err != nil {
			return nil, err
		}
		return setup, nil
	}
	params := &stripe.SetupIntentParams{
		Customer: stripe.String(customerID),
		Usage:    stripe.String(string(stripe.SetupIntentUsageOffSession)),
		Metadata: map[string]string{
			workspaceMetadataKey:                workspaceID,
			workspaceCreationAttemptMetadataKey: attemptID,
		},
	}
	params.Context = ctx
	params.SetIdempotencyKey("bex-workspace-create-setup-" + attemptID)
	setup, err := c.sc.SetupIntents.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe: create workspace SetupIntent: %w", err)
	}
	if err := c.validateWorkspaceSetup(setup, attemptID, workspaceID, customerID); err != nil {
		return nil, err
	}
	return setup, nil
}

func (c *StripeClient) validateWorkspaceSetup(setup *stripe.SetupIntent, attemptID, workspaceID, customerID string) error {
	if setup == nil || setup.ID == "" || setup.Customer == nil || setup.Customer.ID != customerID ||
		setup.Metadata[workspaceMetadataKey] != workspaceID ||
		setup.Metadata[workspaceCreationAttemptMetadataKey] != attemptID ||
		!c.expectedLivemode(setup.Livemode) {
		return &inputError{message: "workspace SetupIntent correlation is invalid"}
	}
	return nil
}

// VerifyWorkspaceSetup re-reads Stripe's authoritative objects. Browser
// redirects and client-reported success are never accepted as proof.
func (c *StripeClient) VerifyWorkspaceSetup(ctx context.Context, attemptID, workspaceID, customerID, setupIntentID string) (VerifiedWorkspaceSetup, error) {
	setup, err := c.workspaceCreationSetupIntent(ctx, attemptID, workspaceID, customerID, setupIntentID)
	if err != nil {
		return VerifiedWorkspaceSetup{}, err
	}
	if setup.Status != stripe.SetupIntentStatusSucceeded || setup.PaymentMethod == nil {
		return VerifiedWorkspaceSetup{}, &inputError{message: "workspace payment setup is not complete"}
	}
	params := &stripe.PaymentMethodParams{}
	params.Context = ctx
	method, err := c.sc.PaymentMethods.Get(setup.PaymentMethod.ID, params)
	if err != nil {
		return VerifiedWorkspaceSetup{}, fmt.Errorf("stripe: retrieve workspace PaymentMethod: %w", err)
	}
	if method.Customer == nil || method.Customer.ID != customerID {
		return VerifiedWorkspaceSetup{}, &inputError{message: "workspace PaymentMethod is not attached to its Customer"}
	}
	return VerifiedWorkspaceSetup{PaymentMethodID: method.ID}, nil
}

// PrepareWorkspaceContract binds the verified method and creates/reuses the
// reserved workspace's metered contract without writing the tenant-FK mapping;
// the store finalizer writes that mapping atomically with the tenant.
func (c *StripeClient) PrepareWorkspaceContract(ctx context.Context, attemptID, workspaceID, customerID, paymentMethodID string) (string, error) {
	customerUpdate := &stripe.CustomerParams{InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{DefaultPaymentMethod: stripe.String(paymentMethodID)}}
	customerUpdate.Context = ctx
	customerUpdate.SetIdempotencyKey("bex-workspace-create-default-" + attemptID)
	if _, err := c.sc.Customers.Update(customerID, customerUpdate); err != nil {
		return "", fmt.Errorf("stripe: bind workspace Customer default: %w", err)
	}

	params := &stripe.SubscriptionListParams{
		ListParams: stripe.ListParams{Limit: stripe.Int64(100)},
		Customer:   stripe.String(customerID), Status: stripe.String("all"),
	}
	params.Context = ctx
	iter := c.sc.Subscriptions.List(params)
	var existing string
	for iter.Next() {
		sub := iter.Subscription()
		if sub.Metadata[workspaceMetadataKey] == workspaceID && sub.Metadata[subscriptionMetadataKey] == "true" && sub.Status != stripe.SubscriptionStatusCanceled && sub.Status != stripe.SubscriptionStatusIncompleteExpired {
			if existing != "" && existing != sub.ID {
				return "", fmt.Errorf("stripe: duplicate contracts for reserved workspace %s", workspaceID)
			}
			existing = sub.ID
		}
	}
	if err := iter.Err(); err != nil {
		return "", fmt.Errorf("stripe: list reserved workspace contracts: %w", err)
	}
	if existing != "" {
		return existing, nil
	}

	priceIDs, err := c.resolvePriceIDs(ctx)
	if err != nil {
		return "", err
	}
	items := make([]*stripe.SubscriptionItemsParams, 0, len(priceIDs))
	for _, priceID := range priceIDs {
		items = append(items, &stripe.SubscriptionItemsParams{Price: stripe.String(priceID)})
	}
	subParams := &stripe.SubscriptionParams{
		Customer:             stripe.String(customerID),
		CollectionMethod:     stripe.String(string(stripe.SubscriptionCollectionMethodChargeAutomatically)),
		DefaultPaymentMethod: stripe.String(paymentMethodID),
		Items:                items,
		Metadata:             map[string]string{workspaceMetadataKey: workspaceID, subscriptionMetadataKey: "true", workspaceCreationAttemptMetadataKey: attemptID},
	}
	subParams.Context = ctx
	subParams.SetIdempotencyKey("bex-workspace-create-contract-" + attemptID)
	if !c.billingEpoch.IsZero() && c.billingEpoch.Before(time.Now()) {
		subParams.BackdateStartDate = stripe.Int64(c.billingEpoch.Unix())
	}
	sub, err := c.sc.Subscriptions.New(subParams)
	if err != nil {
		return "", fmt.Errorf("stripe: create reserved workspace contract: %w", err)
	}
	if sub.ID == "" {
		return "", fmt.Errorf("stripe: reserved workspace contract returned an empty id")
	}
	return sub.ID, nil
}

// CleanupWorkspaceSetup is safe only for an expired/cancelled, unfinalized
// attempt. Cancellation/deletion are individually idempotent at Stripe.
func (c *StripeClient) CleanupWorkspaceSetup(ctx context.Context, workspaceID, customerID, setupIntentID string) error {
	if customerID != "" {
		params := &stripe.SubscriptionListParams{ListParams: stripe.ListParams{Limit: stripe.Int64(100)}, Customer: stripe.String(customerID), Status: stripe.String("all")}
		params.Context = ctx
		iter := c.sc.Subscriptions.List(params)
		for iter.Next() {
			sub := iter.Subscription()
			if sub.Metadata[workspaceMetadataKey] != workspaceID || sub.Status == stripe.SubscriptionStatusCanceled {
				continue
			}
			cancel := &stripe.SubscriptionCancelParams{}
			cancel.Context = ctx
			if _, err := c.sc.Subscriptions.Cancel(sub.ID, cancel); err != nil && !resourceMissing(err) {
				return fmt.Errorf("stripe: cancel abandoned Subscription: %w", err)
			}
		}
		if err := iter.Err(); err != nil {
			return fmt.Errorf("stripe: list abandoned Subscriptions: %w", err)
		}
	}
	if setupIntentID != "" {
		get := &stripe.SetupIntentParams{}
		get.Context = ctx
		setup, err := c.sc.SetupIntents.Get(setupIntentID, get)
		if err != nil && !resourceMissing(err) {
			return fmt.Errorf("stripe: retrieve abandoned SetupIntent: %w", err)
		}
		if setup != nil && setup.Status != stripe.SetupIntentStatusSucceeded && setup.Status != stripe.SetupIntentStatusCanceled {
			params := &stripe.SetupIntentCancelParams{}
			params.Context = ctx
			if _, err := c.sc.SetupIntents.Cancel(setupIntentID, params); err != nil && !resourceMissing(err) {
				return fmt.Errorf("stripe: cancel abandoned SetupIntent: %w", err)
			}
		}
	}
	if customerID != "" {
		params := &stripe.CustomerParams{}
		params.Context = ctx
		if _, err := c.sc.Customers.Del(customerID, params); err != nil && !resourceMissing(err) {
			return fmt.Errorf("stripe: delete abandoned Customer: %w", err)
		}
	}
	return nil
}

type WorkspaceCreationAttemptExpirer interface {
	ExpireWorkspaceCreationAttempts(context.Context, int) ([]store.WorkspaceCreationAttempt, error)
	FinishWorkspaceCreationCleanup(context.Context, string, bool) error
}

// WorkspaceCreationCleaner bounds abandoned local/provider state. The store
// claims a maximum of 10 attempts per pass before provider work begins, so a
// slow Stripe account cannot turn cleanup into an unbounded scan.
type WorkspaceCreationCleaner struct {
	Store    WorkspaceCreationAttemptExpirer
	Provider *StripeClient
	Metrics  *Metrics
	Interval time.Duration
}

func (w *WorkspaceCreationCleaner) Run(ctx context.Context) {
	if w.Store == nil || w.Provider == nil {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	w.runOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *WorkspaceCreationCleaner) runOnce(ctx context.Context) {
	attempts, err := w.Store.ExpireWorkspaceCreationAttempts(ctx, 10)
	if err != nil {
		log.Printf("billing: workspace-create cleanup claim failed: %v", err)
		w.Metrics.Operation("workspace_create_cleanup", "error")
		return
	}
	for _, attempt := range attempts {
		if err := w.Provider.CleanupWorkspaceSetup(ctx, attempt.WorkspaceID, attempt.ProviderCustomerID, attempt.ProviderSetupIntentID); err != nil {
			log.Printf("billing: workspace-create cleanup failed attempt=%s: %v", attempt.ID, err)
			w.Metrics.Operation("workspace_create_cleanup", "error")
			_ = w.Store.FinishWorkspaceCreationCleanup(ctx, attempt.ID, false)
			continue
		}
		if err := w.Store.FinishWorkspaceCreationCleanup(ctx, attempt.ID, true); err != nil {
			log.Printf("billing: workspace-create cleanup completion failed attempt=%s: %v", attempt.ID, err)
			w.Metrics.Operation("workspace_create_cleanup", "error")
			continue
		}
		w.Metrics.Operation("workspace_create_cleanup", "success")
	}
}

func resourceMissing(err error) bool {
	if se, ok := err.(*stripe.Error); ok {
		return se.HTTPStatusCode == 404 || string(se.Code) == "resource_missing"
	}
	return false
}
