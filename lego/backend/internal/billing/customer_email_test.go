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
	"net/http"
	"net/url"
	"slices"
	"testing"
)

// w6/m627: a workspace's billing email becomes its Stripe Customer's email.
func TestSetCustomerEmailUpdatesTheWorkspacesCustomer(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodPost && path == "/v1/customers/cus_1" {
			return 200, `{"id":"cus_1","object":"customer","email":"finance@example.com"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.storeCustomer("tea-a", "cus_1")
	if err := c.SetCustomerEmail(context.Background(), "tea-a", "finance@example.com"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stub.hits, []string{"/v1/customers/cus_1"}) {
		t.Fatalf("hits = %v, want the one Customer update", stub.hits)
	}
	form, err := url.ParseQuery(stub.bodies[0])
	if err != nil || form.Get("email") != "finance@example.com" || len(form) != 1 {
		t.Fatalf("update body = %q, want only the email", stub.bodies[0])
	}
}

// A workspace with no Customer has nothing to update, and none is created:
// a Customer made later takes the email its Checkout collects.
func TestSetCustomerEmailWithoutACustomerChangesNothing(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/customers/search" {
			return 200, `{"object":"search_result","data":[],"has_more":false,"url":"/v1/customers/search"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	if err := c.SetCustomerEmail(context.Background(), "tea-a", "finance@example.com"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stub.hits, []string{"/v1/customers/search"}) {
		t.Fatalf("hits = %v, want only the search", stub.hits)
	}
}

// A failed update is returned, so the caller leaves bex's record unchanged;
// a Customer deleted under bex is not an error.
func TestSetCustomerEmailReportsAFailedUpdate(t *testing.T) {
	for name, tc := range map[string]struct {
		status  int
		body    string
		wantErr bool
	}{
		"stripe down":   {500, `{"error":{"type":"api_error","message":"down"}}`, true},
		"customer gone": {404, `{"error":{"type":"invalid_request_error","code":"resource_missing","message":"No such customer"}}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := newStripeTest(t, func(string, string) (int, string) { return tc.status, tc.body })
			c.storeCustomer("tea-a", "cus_1")
			err := c.SetCustomerEmail(context.Background(), "tea-a", "finance@example.com")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
		})
	}
}
