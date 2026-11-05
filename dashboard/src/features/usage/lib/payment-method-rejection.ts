import { PaymentMethodRejection } from "@/graphql/schema-types";

const REJECTION_KEYS = {
  [PaymentMethodRejection.PrepaidCard]: "usage.paymentMethodRejectedPrepaid",
  [PaymentMethodRejection.UnpaidBalance]:
    "usage.paymentMethodRejectedUnpaidBalance",
} as const;

/**
 * The copy key for why a card was refused at binding (ADR091 D10.5–D10.6).
 * Takes billing readiness's PaymentMethodRejection enum, or the lower-case
 * `reason` a PAYMENT_METHOD_REJECTED error carries; null for none or a reason
 * this build does not know.
 */
export function paymentMethodRejectionKey(
  reason: string | null | undefined,
): (typeof REJECTION_KEYS)[PaymentMethodRejection] | null {
  const normalized = (reason ?? "").toUpperCase();
  return normalized in REJECTION_KEYS
    ? REJECTION_KEYS[normalized as PaymentMethodRejection]
    : null;
}
