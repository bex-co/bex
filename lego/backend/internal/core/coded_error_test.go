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

package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCodedErrorEnvelope verifies the general CodedError envelope independent
// of the plan-limit use-case: Error(), errors.Is through the sentinel, and the
// Extensions() shape the GraphQL formatter reads (gqlerrors.ExtendedError).
func TestCodedErrorEnvelope(t *testing.T) {
	ce := NewPlanLimitError("the hobby plan is limited to 1 workspace member(s)", "hobby", 1)

	if ce.Error() != "the hobby plan is limited to 1 workspace member(s)" {
		t.Errorf("Error() = %q", ce.Error())
	}
	if !errors.Is(ce, ErrBadRequest) {
		t.Error("errors.Is(ce, ErrBadRequest): want true")
	}

	ext := ce.Extensions()
	if ext["code"] != "PLAN_LIMIT" {
		t.Errorf("extensions[code] = %v, want PLAN_LIMIT", ext["code"])
	}
	if ext["plan"] != "hobby" {
		t.Errorf("extensions[plan] = %v, want hobby", ext["plan"])
	}
	if ext["limit"] != 1 {
		t.Errorf("extensions[limit] = %v, want 1", ext["limit"])
	}
}

// TestPlainErrDoesNotImplementExtendedError guards against accidentally
// broadening the interface — constErr should stay a plain error.
func TestPlainErrDoesNotImplementExtendedError(t *testing.T) {
	type extendedError interface {
		error
		Extensions() map[string]any
	}
	if _, ok := Err("plain").(extendedError); ok {
		t.Error("constErr unexpectedly implements ExtendedError")
	}
}

func TestPaymentRequiredCrossSurfaceEnvelope(t *testing.T) {
	err := NewPaymentRequiredError()
	if !errors.Is(err, ErrPaymentRequired) {
		t.Fatal("payment error does not wrap ErrPaymentRequired")
	}
	if err.Error() != PaymentRequiredMessage {
		t.Fatalf("message = %q", err.Error())
	}
	ext := err.Extensions()
	if ext["code"] != "PAYMENT_REQUIRED" || ext["checkoutTool"] != "create_billing_checkout_session" {
		t.Fatalf("GraphQL/MCP extensions = %#v", ext)
	}

	rec := httptest.NewRecorder()
	WriteErr(rec, err)
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("REST status = %d, want 402", rec.Code)
	}
	var body map[string]any
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &body); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if body["id"] != "payment_required" || body["code"] != "PAYMENT_REQUIRED" || body["message"] != PaymentRequiredMessage {
		t.Fatalf("REST body = %#v", body)
	}
	params, ok := body["params"].(map[string]any)
	if !ok || params["checkoutTool"] != "create_billing_checkout_session" {
		t.Fatalf("REST params = %#v", body["params"])
	}
}
