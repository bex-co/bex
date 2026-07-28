// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { BillingOnboardingView } from "../billing-onboarding";

const baseReadiness = {
  workspaceId: "tea-test",
  mode: "test",
  customerReady: true,
  subscriptionReady: true,
  paymentMethodReady: false,
  lifecycle: {
    status: "healthy",
    reason: "",
    graceDeadline: "",
    enforcementOwned: false,
    recoveryPending: false,
    allowedActions: ["update_payment_method", "open_portal"],
    updatedAt: "2026-07-28T01:00:00Z",
  },
  tax: {
    configured: false,
    enabled: false,
    reason: "product_tax_not_configured",
    productTaxCode: "",
    taxBehavior: "",
    registrationCount: 0,
  },
};

describe("BillingOnboardingView", () => {
  it("makes test mode and the fail-closed Tax state explicit", () => {
    render(
      <BillingOnboardingView
        readiness={baseReadiness}
        loading={false}
        checkoutBusy={false}
        portalBusy={false}
        onCheckout={() => undefined}
        onPortal={() => undefined}
      />,
    );

    expect(screen.getByText("Stripe Test Mode")).toBeInTheDocument();
    expect(screen.getByText(/Tax is not configured/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Add test payment method" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Open billing portal" }),
    ).toBeEnabled();
  });

  it("invokes only the hosted Checkout and Portal actions", () => {
    const onCheckout = vi.fn();
    const onPortal = vi.fn();
    render(
      <BillingOnboardingView
        readiness={{ ...baseReadiness, paymentMethodReady: true }}
        loading={false}
        checkoutBusy={false}
        portalBusy={false}
        onCheckout={onCheckout}
        onPortal={onPortal}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Update test payment method" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Open billing portal" }),
    );

    expect(onCheckout).toHaveBeenCalledOnce();
    expect(onPortal).toHaveBeenCalledOnce();
    expect(screen.getByText(/only on Stripe-hosted pages/)).toBeInTheDocument();
  });

  it("shows the authoritative grace deadline without inventing state", () => {
    render(
      <BillingOnboardingView
        readiness={{
          ...baseReadiness,
          lifecycle: {
            ...baseReadiness.lifecycle,
            status: "grace",
            reason: "payment_failed",
            graceDeadline: "2026-07-29T01:00:00Z",
          },
        }}
        loading={false}
        checkoutBusy={false}
        portalBusy={false}
        onCheckout={() => undefined}
        onPortal={() => undefined}
      />,
    );
    expect(screen.getByText(/2026-07-29T01:00:00Z/)).toBeInTheDocument();
    expect(screen.getByText(/payment failed/i)).toBeInTheDocument();
  });

  it.each([
    [
      "enforced",
      { enforcementOwned: true, recoveryPending: false },
      /enforcement is active/i,
    ],
    [
      "recovering",
      { enforcementOwned: true, recoveryPending: true },
      /restoring only resources changed/i,
    ],
  ])("renders the provider-neutral %s state", (status, flags, expected) => {
    render(
      <BillingOnboardingView
        readiness={{
          ...baseReadiness,
          lifecycle: {
            ...baseReadiness.lifecycle,
            ...flags,
            status,
            reason: "test_transition",
          },
        }}
        loading={false}
        checkoutBusy={false}
        portalBusy={false}
        onCheckout={() => undefined}
        onPortal={() => undefined}
      />,
    );
    expect(screen.getByText(expected)).toBeInTheDocument();
  });

  it("keeps Portal disabled until the unique contract is ready", () => {
    render(
      <BillingOnboardingView
        readiness={{
          ...baseReadiness,
          customerReady: false,
          subscriptionReady: false,
        }}
        loading={false}
        checkoutBusy={false}
        portalBusy={false}
        onCheckout={() => undefined}
        onPortal={() => undefined}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Open billing portal" }),
    ).toBeDisabled();
  });

  it("does not render actions when readiness is unavailable", () => {
    render(
      <BillingOnboardingView
        readiness={null}
        loading={false}
        error={new Error("forbidden")}
        checkoutBusy={false}
        portalBusy={false}
        onCheckout={() => undefined}
        onPortal={() => undefined}
      />,
    );

    expect(
      screen.getByText(/unavailable or you do not have workspace-admin access/),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
