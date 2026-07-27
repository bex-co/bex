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

// Package billing is the Stripe Billing export seam (ADR040): a backend-only
// sidecar ships sealed usage_hourly rows as meter events, then the same client
// reads Stripe's current invoice preview and finalized invoice history. bex
// keeps usage_hourly/usage_monthly as its operational record and pricing.yaml
// as the advisory estimate; Stripe owns rating, invoicing, and collection.
//
// The feature is gated by BEX_STRIPE_SECRET_KEY. With the key unset no client,
// emitter, billing reader, or webhook is constructed, preserving the original
// estimate-only behavior. The operator never imports this package.
package billing

import (
	"context"
	"time"
)

// Event is one sealed usage record exported to Stripe. TransactionID is the
// deterministic meter-event identifier; CustomerID is the bex workspace id
// resolved to a Stripe Customer by the client.
type Event struct {
	TransactionID string
	CustomerID    string
	EventType     string
	Timestamp     time.Time
	Properties    map[string]string
}

// Ingester is the slice the generic outbox emitter needs. Implementations
// provision the customer and rating contract before accepting meter events.
type Ingester interface {
	EnsureCustomer(ctx context.Context, tenantID string) error
	EnsureContract(ctx context.Context, tenantID string) error
	IngestBatch(ctx context.Context, events []Event) error
}
