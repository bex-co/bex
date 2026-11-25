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

	"github.com/stripe/stripe-go/v86"
)

// SetCustomerEmail sets the email of workspaceID's Stripe Customer (w6/m627):
// the address Stripe sends the workspace's receipts and finalized invoices to,
// where the account's customer-email settings send them. A workspace with no
// Customer, or whose Customer is gone, has nothing to update; one created
// later takes the email its Checkout collects.
func (c *StripeClient) SetCustomerEmail(ctx context.Context, workspaceID, email string) error {
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil || !found {
		return err
	}
	params := &stripe.CustomerParams{Email: stripe.String(email)}
	params.Context = ctx
	if _, err := c.sc.Customers.Update(customerID, params); err != nil && !resourceMissing(err) {
		return fmt.Errorf("stripe: set the email of Customer %s for %s: %w", customerID, workspaceID, err)
	}
	return nil
}
