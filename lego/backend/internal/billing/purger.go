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

import "context"

// WorkspaceCanceller cancels a deleted workspace's Stripe billing contract.
// *StripeClient satisfies it; nil ⇒ Stripe is disabled (BEX_STRIPE_SECRET_KEY
// unset) and the purger is never wired, so workspace delete stays byte-identical.
type WorkspaceCanceller interface {
	CancelContract(ctx context.Context, workspaceID string) error
}

// WorkspacePurger adapts the Stripe client to the workspaces feature's
// WorkspacePurger seam (satisfied structurally — this package never imports
// workspaces), closing a w1/m61 gap: workspace delete cascaded the local billing
// rows but never told Stripe, so the metered Subscription stayed active against a
// Customer whose workspace was gone. It runs PRE-cascade in workspaces.Delete so
// the Customer/Subscription ids in the billing_provider_mappings row are still
// present to resolve.
type WorkspacePurger struct {
	Canceller WorkspaceCanceller
}

// PurgeWorkspace cancels the workspace's active Stripe Subscription. Idempotent:
// a nil canceller (Stripe off) or a workspace with no live subscription is a
// no-op, so a retried delete completes cleanly. The Customer is retained for its
// invoice history — CancelContract never deletes it. Never authorizes here: this
// runs as an internal system operation after the deleting caller's own
// authorization already passed.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, tenantID string) error {
	if p == nil || p.Canceller == nil {
		return nil
	}
	return p.Canceller.CancelContract(ctx, tenantID)
}
