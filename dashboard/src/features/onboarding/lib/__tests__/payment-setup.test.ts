import { describe, expect, it } from "vitest";
import type { BillingReadiness } from "@/features/usage/hooks/use-billing-onboarding";
import {
  PAYMENT_SETUP_PATH,
  paymentSetupGateBlocks,
  paymentSetupPath,
  paymentSetupState,
} from "../payment-setup";

function readiness(onboardingRequired: boolean): BillingReadiness {
  return {
    workspaceId: "tea-a",
    mode: "test",
    customerReady: false,
    subscriptionReady: false,
    paymentMethodReady: false,
    paymentMethodBrand: "",
    paymentMethodLast4: "",
    paymentMethodRequired: true,
    paymentMethodOnboardingRequired: onboardingRequired,
    lifecycle: {
      status: "healthy",
      reason: "",
      graceDeadline: "",
      enforcementOwned: false,
      recoveryPending: false,
      allowedActions: [],
      updatedAt: "",
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

describe("paymentSetupPath", () => {
  it("is the bare wall when there is no deep link worth carrying", () => {
    expect(paymentSetupPath(undefined)).toBe(PAYMENT_SETUP_PATH);
    expect(paymentSetupPath(null)).toBe(PAYMENT_SETUP_PATH);
    expect(paymentSetupPath("/")).toBe(PAYMENT_SETUP_PATH);
  });

  it("carries a same-origin deep link, query included, URL-encoded", () => {
    expect(paymentSetupPath("/services/new?type=web#plan")).toBe(
      "/setup/payment?next=%2Fservices%2Fnew%3Ftype%3Dweb%23plan",
    );
  });

  it("drops anything that is not a same-origin path (open redirect guard)", () => {
    for (const hostile of [
      "https://evil.example",
      "//evil.example",
      "/\\evil.example",
      "javascript:alert(1)",
    ]) {
      expect(paymentSetupPath(hostile)).toBe(PAYMENT_SETUP_PATH);
    }
  });
});

describe("paymentSetupState", () => {
  const base = { loading: false, error: undefined, billingForbidden: false };

  it("puts the wall up only on the server's definitive 'required'", () => {
    expect(paymentSetupState({ ...base, readiness: readiness(true) })).toBe(
      "required",
    );
    expect(paymentSetupState({ ...base, readiness: readiness(false) })).toBe(
      "satisfied",
    );
  });

  it("never re-derives the gate from the other readiness flags", () => {
    // Not bound (paymentMethodReady false) yet the gate says a create would
    // pass — an excluded/comped workspace, or paid-intent-only mode.
    const exempt = { ...readiness(false), paymentMethodReady: false };
    expect(paymentSetupState({ ...base, readiness: exempt })).toBe("satisfied");
  });

  it("forwards a caller who cannot bind a card here, whatever readiness says", () => {
    expect(
      paymentSetupState({
        ...base,
        readiness: readiness(true),
        billingForbidden: true,
      }),
    ).toBe("forbidden");
    expect(
      paymentSetupState({ ...base, readiness: null, billingForbidden: true }),
    ).toBe("forbidden");
  });

  it("holds while readiness is unknown and reports a failed read", () => {
    expect(paymentSetupState({ ...base, readiness: null, loading: true })).toBe(
      "loading",
    );
    expect(paymentSetupState({ ...base, readiness: null })).toBe("loading");
    expect(
      paymentSetupState({
        ...base,
        readiness: null,
        error: new Error("billing unavailable"),
      }),
    ).toBe("unavailable");
  });

  it("prefers a stale-but-present readiness over an in-flight refetch", () => {
    expect(
      paymentSetupState({ ...base, readiness: readiness(true), loading: true }),
    ).toBe("required");
  });
});

describe("paymentSetupGateBlocks", () => {
  it("blocks only on a definitive server 'required' for a billing manager", () => {
    expect(
      paymentSetupGateBlocks({
        onboardingRequired: true,
        billingForbidden: false,
      }),
    ).toBe(true);
  });

  it("fails open on unknown, false, or a caller who cannot bind a card", () => {
    expect(
      paymentSetupGateBlocks({
        onboardingRequired: undefined,
        billingForbidden: false,
      }),
    ).toBe(false);
    expect(
      paymentSetupGateBlocks({
        onboardingRequired: null,
        billingForbidden: false,
      }),
    ).toBe(false);
    expect(
      paymentSetupGateBlocks({
        onboardingRequired: false,
        billingForbidden: false,
      }),
    ).toBe(false);
    expect(
      paymentSetupGateBlocks({
        onboardingRequired: true,
        billingForbidden: true,
      }),
    ).toBe(false);
  });
});
