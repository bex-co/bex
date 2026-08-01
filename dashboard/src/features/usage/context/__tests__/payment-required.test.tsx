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

import { useState } from "react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const billingMock = vi.hoisted(() => ({
  setPaymentReady: null as null | ((ready: boolean) => void),
  openCheckout: vi.fn(),
  openPortal: vi.fn(),
  refetch: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-a" }),
}));

vi.mock("@/features/usage/hooks/use-billing-onboarding", async () => {
  const React = await import("react");
  return {
    useBillingOnboarding: () => {
      const [paymentMethodReady, setPaymentMethodReady] = React.useState(false);
      billingMock.setPaymentReady = setPaymentMethodReady;
      return {
        readiness: {
          workspaceId: "tea-a",
          mode: "test",
          customerReady: true,
          subscriptionReady: true,
          paymentMethodReady,
          lifecycle: {
            status: "healthy",
            reason: "",
            graceDeadline: "",
            enforcementOwned: false,
            recoveryPending: false,
            allowedActions: ["update_payment_method", "open_portal"],
            updatedAt: "",
          },
          tax: {
            configured: true,
            enabled: true,
            reason: "",
            productTaxCode: "txcd_test",
            taxBehavior: "exclusive",
            registrationCount: 1,
          },
        },
        loading: false,
        error: undefined,
        checkoutBusy: false,
        portalBusy: false,
        openCheckout: billingMock.openCheckout,
        openPortal: billingMock.openPortal,
        refetch: billingMock.refetch,
      };
    },
  };
});

import { PaymentRequiredProvider } from "../payment-required";
import { usePaymentRequiredGate } from "../payment-required-context";
import { isPaymentOnboardingCancelled } from "../payment-required-error";

function paymentRequiredError() {
  return new CombinedGraphQLErrors({
    data: null,
    errors: [
      {
        message: "Payment information is required for paid plans.",
        extensions: { code: "PAYMENT_REQUIRED" },
      },
    ],
  });
}

function PaidAction({ action }: { action: () => Promise<string> }) {
  const paymentGate = usePaymentRequiredGate();
  const [status, setStatus] = useState("idle");
  const run = async () => {
    try {
      await paymentGate.run(action);
      setStatus("complete");
    } catch (error) {
      setStatus(isPaymentOnboardingCancelled(error) ? "cancelled" : "error");
    }
  };
  return (
    <>
      <button onClick={() => void run()}>Run paid action</button>
      <span>{status}</span>
    </>
  );
}

beforeEach(() => {
  billingMock.setPaymentReady = null;
  billingMock.openCheckout.mockReset();
  billingMock.openPortal.mockReset();
  billingMock.refetch.mockReset().mockResolvedValue(undefined);
});

describe("PaymentRequiredProvider", () => {
  it("opens hosted onboarding, polls readiness, and resumes the exact interrupted action", async () => {
    const action = vi
      .fn<() => Promise<string>>()
      .mockRejectedValueOnce(paymentRequiredError())
      .mockResolvedValueOnce("created");
    billingMock.openCheckout.mockImplementation(async () => {
      billingMock.setPaymentReady?.(true);
    });
    render(
      <PaymentRequiredProvider>
        <PaidAction action={action} />
      </PaymentRequiredProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Run paid action" }));
    expect(
      await screen.findByRole("dialog", {
        name: "Add a payment method to continue",
      }),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Add test payment method" }),
    );

    await waitFor(() => expect(action).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(screen.getByText("complete")).toBeInTheDocument(),
    );
    expect(billingMock.openCheckout).toHaveBeenCalledOnce();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("cancels without retrying or leaking a generic error", async () => {
    const action = vi
      .fn<() => Promise<string>>()
      .mockRejectedValue(paymentRequiredError());
    render(
      <PaymentRequiredProvider>
        <PaidAction action={action} />
      </PaymentRequiredProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Run paid action" }));
    await screen.findByRole("dialog");
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await waitFor(() =>
      expect(screen.getByText("cancelled")).toBeInTheDocument(),
    );
    expect(action).toHaveBeenCalledOnce();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
