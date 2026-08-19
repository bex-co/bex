import { describe, it, expect } from "vitest";
import {
  createBlockedByPayment,
  isPaidWorkspacePlan,
} from "@/features/workspaces/lib/create-workspace-payment";

describe("createBlockedByPayment", () => {
  it("never blocks Hobby", () => {
    expect(isPaidWorkspacePlan("hobby")).toBe(false);
    expect(
      createBlockedByPayment({
        plan: "hobby",
        requirePaymentMethod: true,
        paymentMethodReady: false,
        billingLoading: true,
        hasCurrentWorkspace: true,
      }),
    ).toBe(false);
  });

  it("blocks Pro/Scale/Enterprise only when the gate is on and a card is missing", () => {
    expect(
      createBlockedByPayment({
        plan: "pro",
        requirePaymentMethod: true,
        paymentMethodReady: false,
        billingLoading: false,
        hasCurrentWorkspace: true,
      }),
    ).toBe(true);
    expect(
      createBlockedByPayment({
        plan: "scale",
        requirePaymentMethod: false,
        paymentMethodReady: false,
        billingLoading: false,
        hasCurrentWorkspace: true,
      }),
    ).toBe(false);
    expect(
      createBlockedByPayment({
        plan: "enterprise",
        requirePaymentMethod: true,
        paymentMethodReady: true,
        billingLoading: false,
        hasCurrentWorkspace: true,
      }),
    ).toBe(false);
  });
});
