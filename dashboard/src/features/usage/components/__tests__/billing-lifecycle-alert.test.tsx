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
import type {
  BillingOverdueInvoice,
  BillingReadiness,
} from "../../hooks/use-billing-onboarding";
import {
  BillingStatusAlerts,
  type BillingUpdatePaymentAction,
} from "../billing-lifecycle-alert";

const overdue: BillingOverdueInvoice = {
  id: "in_overdue",
  hostedInvoiceUrl: "https://invoice.stripe.com/i/acct_1/test_overdue",
};

function readinessIn(
  status: string,
  overdueInvoice: BillingOverdueInvoice | null,
): BillingReadiness {
  const enforcement = status !== "grace" && status !== "healthy";
  return {
    workspaceId: "tea-test",
    mode: "live",
    customerReady: true,
    subscriptionReady: true,
    paymentMethodReady: true,
    paymentMethodBrand: "visa",
    paymentMethodLast4: "0341",
    paymentMethodRequired: false,
    paymentMethodOnboardingRequired: false,
    paymentMethodRejection: "",
    lifecycle: {
      status,
      reason: "payment_failed",
      graceDeadline: status === "grace" ? "2026-12-01T00:00:00Z" : "",
      enforcementOwned: enforcement,
      recoveryPending: status === "recovering",
      allowedActions: ["update_payment_method", "open_portal"],
      updatedAt: "2026-11-24T00:00:00Z",
      overdueInvoice,
    },
    tax: {
      configured: false,
      enabled: false,
      reason: "",
      productTaxCode: "",
      taxBehavior: "",
      registrationCount: 0,
    },
  };
}

function renderAlert(
  readiness: BillingReadiness,
  update: Partial<BillingUpdatePaymentAction> = {},
) {
  const onUpdatePayment = vi.fn();
  render(
    <BillingStatusAlerts
      readiness={readiness}
      update={{
        onUpdatePayment,
        busy: false,
        canManageBilling: true,
        ...update,
      }}
    />,
  );
  return onUpdatePayment;
}

describe("BillingStatusAlerts overdue actions (w6/m626)", () => {
  it.each(["grace", "enforcing", "enforced"])(
    "offers Pay Now and Update payment method while %s with an open invoice",
    (status) => {
      const onUpdatePayment = renderAlert(readinessIn(status, overdue));
      const pay = screen.getByRole("link", { name: "Pay Now" });
      expect(pay).toHaveAttribute("href", overdue.hostedInvoiceUrl);
      expect(pay).toHaveAttribute("target", "_blank");
      expect(pay).toHaveAttribute("rel", "noopener noreferrer");
      fireEvent.click(
        screen.getByRole("button", { name: "Update payment method" }),
      );
      expect(onUpdatePayment).toHaveBeenCalledOnce();
    },
  );

  it.each(["grace", "enforced"])(
    "offers neither while %s with no open invoice",
    (status) => {
      renderAlert(readinessIn(status, null));
      expect(screen.getByTestId("billing-lifecycle-alert")).toBeInTheDocument();
      expect(screen.queryByRole("link")).not.toBeInTheDocument();
      expect(screen.queryByRole("button")).not.toBeInTheDocument();
    },
  );

  it("offers nothing once the payment is recovering", () => {
    renderAlert(readinessIn("recovering", overdue));
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("keeps Pay Now for a member who cannot change the card", () => {
    renderAlert(readinessIn("enforced", overdue), { canManageBilling: false });
    expect(screen.getByRole("link", { name: "Pay Now" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Update payment method" }),
    ).toBeDisabled();
  });

  it("drops a hosted link that is not plain https", () => {
    renderAlert(
      readinessIn("grace", {
        id: "in_overdue",
        hostedInvoiceUrl: "javascript:alert(1)",
      }),
    );
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Update payment method" }),
    ).toBeEnabled();
  });
});
