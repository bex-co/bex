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
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

const (
	// portalActorMetadataKey, portalActorMethodMetadataKey and
	// portalOpenedAtMetadataKey record on the Customer who last opened the
	// Billing Portal and when (w2/070). A Portal Session carries no metadata of
	// its own, so this is the only place a later customer.updated can learn who
	// changed the card there.
	portalActorMetadataKey       = "bex_portal_actor"
	portalActorMethodMetadataKey = "bex_portal_actor_method"
	portalOpenedAtMetadataKey    = "bex_portal_opened_at"
	// boundPaymentMethodMetadataKey is the payment method a bex Checkout bound
	// as the Customer's default. That Checkout already audits the change, so a
	// customer.updated whose default is this method is not recorded again.
	boundPaymentMethodMetadataKey = "bex_bound_payment_method"
	// portalAttributionWindow bounds how long after opening the portal a
	// change is still attributed to the member who opened it; later changes
	// (Stripe staff, the API) are the system's.
	portalAttributionWindow = time.Hour
)

// recordPortalOpener stamps the member opening a Portal Session on the
// Customer. Best-effort, like the Checkout start marker: an unattributed audit
// row is a smaller loss than refusing the portal the member is waiting for.
func (c *StripeClient) recordPortalOpener(ctx context.Context, customerID string) {
	actor, ok := core.IdentityFrom(ctx)
	if !ok || actor.Subject == "" {
		return
	}
	params := &stripe.CustomerParams{}
	params.AddMetadata(portalActorMetadataKey, actor.Subject)
	params.AddMetadata(portalActorMethodMetadataKey, actor.Method)
	params.AddMetadata(portalOpenedAtMetadataKey, strconv.FormatInt(time.Now().Unix(), 10))
	params.Context = ctx
	if _, err := c.sc.Customers.Update(customerID, params); err != nil {
		log.Printf("billing: record portal opener on %s: %v", customerID, err)
	}
}

// RecordPortalPaymentMethodChange audits a default-payment-method change made
// in the Stripe Billing Portal, which bex otherwise never hears about (w2/070):
// Render's audit log records UpdateWorkspacePaymentMethodEvent whichever way
// the card changed. The endpoint is account-wide, so a Customer that is not a
// bex workspace's current Customer is ignored. The row id derives from the
// event, so Stripe's retries cannot record it twice.
func (c *StripeClient) RecordPortalPaymentMethodChange(ctx context.Context, event *stripe.Event) error {
	if c.audit == nil || event == nil || event.Data == nil || !defaultPaymentMethodChanged(event.Data.PreviousAttributes) {
		return nil
	}
	// The event's own Customer snapshot decides dedupe and attribution, so a
	// later card change or portal visit cannot rewrite what this change was.
	// Ownership is judged on the live Customer, as every handler re-reads.
	var snapshot stripe.Customer
	if json.Unmarshal(event.Data.Raw, &snapshot) != nil || snapshot.ID == "" {
		log.Printf("billing: customer.updated event=%s carries no Customer; acknowledged", event.ID)
		return nil
	}
	if checkoutBind(snapshot, event.Data.PreviousAttributes) {
		return nil
	}
	customer, gone, err := c.liveCustomer(ctx, snapshot.ID)
	if err != nil || gone {
		return err
	}
	workspaceID := customer.Metadata[workspaceMetadataKey]
	if workspaceID == "" || customer.Metadata[deletedAtMetadataKey] != "" {
		return nil
	}
	current, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil {
		return err
	}
	if !found || current != customer.ID {
		return nil
	}
	caller, method := portalOpener(snapshot.Metadata, time.Unix(event.Created, 0))
	return c.auditPaymentMethodChange(ctx, ids.Derive(ids.Audit, "customer-updated", event.ID), workspaceID, caller, method)
}

// checkoutBind reports the write a bex Checkout makes when it binds a card:
// the default and bex_bound_payment_method change together, and that Checkout
// audits it itself. A portal change never touches metadata, so switching back
// to a Checkout-bound card in the portal is still recorded.
func checkoutBind(snapshot stripe.Customer, previous map[string]any) bool {
	_, metadataChanged := previous["metadata"]
	bound := snapshot.Metadata[boundPaymentMethodMetadataKey]
	return metadataChanged && bound != "" && bound == customerDefaultPaymentMethod(&snapshot)
}

// auditPaymentMethodChange records Render's UpdateWorkspacePaymentMethodEvent
// for both ways a card changes (Checkout and the Billing Portal). The id is
// derived from the Stripe object, so a redelivered webhook converges on one row;
// a failure fails the webhook so Stripe retries. No card details are stored.
func (c *StripeClient) auditPaymentMethodChange(ctx context.Context, rowID, workspaceID, caller, method string) error {
	if err := c.audit.Record(ctx, core.AuditEvent{
		ID:           rowID,
		Caller:       caller,
		CallerMethod: method,
		Verb:         core.AuditVerbUpdatePaymentMethod,
		Resource:     core.WorkspaceObject(workspaceID),
		Outcome:      core.AuditAllowed,
		At:           time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("stripe: audit payment-method change for %s: %w", workspaceID, err)
	}
	return nil
}

// defaultPaymentMethodChanged reports whether a customer.updated event's
// previous_attributes include invoice_settings.default_payment_method — the
// only field Stripe lists there is one that changed.
func defaultPaymentMethodChanged(previous map[string]any) bool {
	settings, ok := previous["invoice_settings"].(map[string]any)
	if !ok {
		return false
	}
	_, changed := settings["default_payment_method"]
	return changed
}

// portalOpener is the member who opened the portal within the attribution
// window before the change, or empty (the system) when none did.
func portalOpener(metadata map[string]string, changedAt time.Time) (string, string) {
	openedAt, err := strconv.ParseInt(metadata[portalOpenedAtMetadataKey], 10, 64)
	if err != nil || metadata[portalActorMetadataKey] == "" {
		return "", ""
	}
	if age := changedAt.Sub(time.Unix(openedAt, 0)); age < 0 || age > portalAttributionWindow {
		return "", ""
	}
	return metadata[portalActorMetadataKey], metadata[portalActorMethodMetadataKey]
}
