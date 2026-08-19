import type { WorkspacePlanId } from "@/features/workspaces/types";

/** Pro, Scale, and Enterprise are paid capability plans (Hobby is not). */
export function isPaidWorkspacePlan(plan: WorkspacePlanId): boolean {
  return plan === "pro" || plan === "scale" || plan === "enterprise";
}

/**
 * Whether Create must wait on the current workspace's card. The paid-intent
 * gate (BEX_REQUIRE_PAYMENT_METHOD) is reported as paymentMethodRequired on
 * billing readiness; without a current workspace there is no Customer to bind.
 */
export function createBlockedByPayment(opts: {
  plan: WorkspacePlanId;
  requirePaymentMethod: boolean;
  paymentMethodReady: boolean;
  billingLoading: boolean;
  hasCurrentWorkspace: boolean;
}): boolean {
  if (!isPaidWorkspacePlan(opts.plan) || !opts.requirePaymentMethod) {
    return false;
  }
  if (!opts.hasCurrentWorkspace) {
    return false;
  }
  if (opts.billingLoading) {
    return true;
  }
  return !opts.paymentMethodReady;
}
