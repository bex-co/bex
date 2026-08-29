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

import { safeNext } from "@/common/lib/safe-next";
import type { BillingReadiness } from "@/features/usage/hooks/use-billing-onboarding";

/** The sign-up payment wall (ADR075 D7, revised 2026-08-29). */
export const PAYMENT_SETUP_PATH = "/setup/payment";

/** The open-source project — the legitimate no-card path (ADR075 § Positioning). */
export const SELF_HOST_URL = "https://github.com/bex-co/bex";

/**
 * Root-relative wall URL carrying the guarded deep link, used both for the
 * in-app navigation onto the wall and as Stripe's return target (Checkout
 * appends `?billing=success|cancelled`). `next` is safeNext-normalized here
 * AND re-validated when read, so a tampered stash can never redirect.
 */
export function paymentSetupPath(next?: string | null): string {
  const target = safeNext(next);
  if (target === "/") return PAYMENT_SETUP_PATH;
  return `${PAYMENT_SETUP_PATH}?next=${encodeURIComponent(target)}`;
}

export type PaymentSetupState =
  /** Readiness not known yet — hold the wall's actions, show the skeleton. */
  | "loading"
  /** The gate refuses this workspace: show the wall. */
  | "required"
  /** A create would pass (bound, exempt, or gate off/paid-intent-only): continue. */
  | "satisfied"
  /** The caller cannot bind a card here (not a billing manager): continue —
   *  the API's 402 + interception dialog remain the backstop for them. */
  | "forbidden"
  /** Readiness could not be read at all: show the error with a way out. */
  | "unavailable";

/**
 * The wall's one decision, pure so it is testable without Apollo. The server's
 * `paymentMethodOnboardingRequired` is the only thing that puts the wall up —
 * the dashboard never re-derives the gate from the other readiness flags.
 */
export function paymentSetupState(opts: {
  readiness: BillingReadiness | null;
  loading: boolean;
  error: Error | undefined;
  /** `useCapabilities().loaded && !canManageBilling` — a definitive "cannot". */
  billingForbidden: boolean;
}): PaymentSetupState {
  if (opts.billingForbidden) return "forbidden";
  if (opts.readiness) {
    return opts.readiness.paymentMethodOnboardingRequired
      ? "required"
      : "satisfied";
  }
  if (opts.loading) return "loading";
  if (opts.error) return "unavailable";
  return "loading";
}

/**
 * Whether the root gate should send the caller to the wall from an app route.
 * Only a definitive server "required" moves anyone — an unknown, errored, or
 * forbidden read never blocks (the server stays the real gate; a false
 * positive here would lock a member out of a workspace they can use).
 */
export function paymentSetupGateBlocks(opts: {
  onboardingRequired: boolean | null | undefined;
  billingForbidden: boolean;
}): boolean {
  return opts.onboardingRequired === true && !opts.billingForbidden;
}
