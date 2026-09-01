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
	"strings"
	"testing"

	stripe "github.com/stripe/stripe-go/v86"
)

func TestPrepareWorkspaceSetupOwnsCustomerAndUsesDynamicPaymentMethods(t *testing.T) {
	const (
		attemptID   = "wca-c1234567890123456789"
		workspaceID = "tea-c1234567890123456789"
	)
	stub := &stripeStub{route: func(method, path string) (int, string) {
		switch path {
		case "/v1/customers":
			return http.StatusOK, `{"id":"cus_new","object":"customer","email":"billing@example.com","livemode":false,"metadata":{"bex_workspace":"` + workspaceID + `","bex_workspace_creation_attempt":"` + attemptID + `"}}`
		case "/v1/setup_intents":
			return http.StatusOK, `{"id":"seti_new","object":"setup_intent","client_secret":"seti_new_secret_safe","status":"requires_payment_method","livemode":false,"customer":{"id":"cus_new"},"metadata":{"bex_workspace":"` + workspaceID + `","bex_workspace_creation_attempt":"` + attemptID + `"}}`
		default:
			return http.StatusNotFound, `{"error":{"code":"resource_missing","message":"missing"}}`
		}
	}}
	c := NewStripe(StripeConfig{
		SecretKey: "rk_test_server", PublishableKey: "pk_test_browser",
		HTTPClient: &http.Client{Transport: stub}, BaseURL: "https://stub.stripe.test",
		MaxNetworkRetries: stripe.Int64(0),
	})
	setup, err := c.PrepareWorkspaceSetup(context.Background(), attemptID, workspaceID, "billing@example.com", "", "")
	if err != nil {
		t.Fatalf("PrepareWorkspaceSetup: %v", err)
	}
	if setup.CustomerID != "cus_new" || setup.SetupIntentID != "seti_new" || setup.ClientSecret == "" || setup.PublishableKey != "pk_test_browser" {
		t.Fatalf("setup = %#v", setup)
	}
	customerRequests := stub.requests("/v1/customers")
	if len(customerRequests) != 1 || !strings.Contains(customerRequests[0].body, "email=billing%40example.com") || !strings.Contains(customerRequests[0].body, "metadata[bex_workspace]="+workspaceID) {
		t.Fatalf("customer request = %#v", customerRequests)
	}
	setupRequests := stub.requests("/v1/setup_intents")
	if len(setupRequests) != 1 || strings.Contains(setupRequests[0].body, "payment_method_types") {
		t.Fatalf("SetupIntent request should use dynamic methods: %#v", setupRequests)
	}
	if got := setupRequests[0].header.Get("Idempotency-Key"); got != "bex-workspace-create-setup-"+attemptID {
		t.Fatalf("SetupIntent idempotency key = %q", got)
	}
}

func TestVerifyWorkspaceSetupRejectsClientSuccessWithoutStripeProof(t *testing.T) {
	const (
		attemptID   = "wca-c1234567890123456789"
		workspaceID = "tea-c1234567890123456789"
	)
	c, _ := newStripeTest(t, func(_ string, path string) (int, string) {
		if path == "/v1/setup_intents/seti_pending" {
			return http.StatusOK, `{"id":"seti_pending","object":"setup_intent","status":"requires_action","livemode":false,"customer":{"id":"cus_new"},"metadata":{"bex_workspace":"` + workspaceID + `","bex_workspace_creation_attempt":"` + attemptID + `"}}`
		}
		return http.StatusNotFound, `{"error":{"code":"resource_missing","message":"missing"}}`
	})
	if _, err := c.VerifyWorkspaceSetup(context.Background(), attemptID, workspaceID, "cus_new", "seti_pending"); err == nil || !strings.Contains(err.Error(), "not complete") {
		t.Fatalf("VerifyWorkspaceSetup error = %v, want incomplete", err)
	}
}

func TestVerifyWorkspaceSetupRequiresAttachedSameCustomerMethod(t *testing.T) {
	const (
		attemptID   = "wca-c1234567890123456789"
		workspaceID = "tea-c1234567890123456789"
	)
	c, _ := newStripeTest(t, func(_ string, path string) (int, string) {
		switch path {
		case "/v1/setup_intents/seti_ok":
			return http.StatusOK, `{"id":"seti_ok","object":"setup_intent","status":"succeeded","livemode":false,"customer":{"id":"cus_new"},"payment_method":{"id":"pm_new"},"metadata":{"bex_workspace":"` + workspaceID + `","bex_workspace_creation_attempt":"` + attemptID + `"}}`
		case "/v1/payment_methods/pm_new":
			return http.StatusOK, `{"id":"pm_new","object":"payment_method","customer":{"id":"cus_other"}}`
		default:
			return http.StatusNotFound, `{"error":{"code":"resource_missing","message":"missing"}}`
		}
	})
	if _, err := c.VerifyWorkspaceSetup(context.Background(), attemptID, workspaceID, "cus_new", "seti_ok"); err == nil || !strings.Contains(err.Error(), "not attached") {
		t.Fatalf("VerifyWorkspaceSetup error = %v, want customer mismatch", err)
	}
}
