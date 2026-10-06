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

import type { Session } from "@ory/client-fetch";
import { safeNext } from "@/common/lib/safe-next";

/** The verification wall (ADR075 D8 revision 2026-10-06, w2/m168). */
export const EMAIL_VERIFICATION_PATH = "/auth/verification";

/**
 * Root-relative wall URL carrying the guarded deep link. The verification
 * page reads `next` and, on success, continues through the payment wall
 * (`/setup/payment?next=…`, ADR075 D7). `next` is safeNext-normalized here
 * AND re-validated when read, so a tampered value can never redirect.
 */
export function emailVerificationPath(next?: string | null): string {
  const target = safeNext(next);
  if (target === "/") return EMAIL_VERIFICATION_PATH;
  return `${EMAIL_VERIFICATION_PATH}?next=${encodeURIComponent(target)}`;
}

/** The session identity's trait email, or undefined when it has none. */
export function sessionTraitEmail(
  session: Session | null | undefined,
): string | undefined {
  const traits = session?.identity?.traits as
    | Record<string, unknown>
    | undefined;
  const email = traits?.email;
  return typeof email === "string" && email.length > 0 ? email : undefined;
}

/**
 * Whether a Kratos session is DEFINITIVELY unverified: signed in, with a trait
 * email, and a `verifiable_addresses` list holding no verified entry for that
 * email (compared case-insensitively — the same rule bex-api applies to
 * whoami). This is the one decision the dashboard wall makes, pure so it is
 * testable without a router.
 *
 * Fail-open on anything indeterminate: no session (requireAuth's job), no
 * trait email, or no `verifiable_addresses` list at all. bex-api's
 * `EMAIL_VERIFICATION_REQUIRED` refusal is the real gate; a false positive
 * here would lock a verified user out of the product.
 */
export function emailVerificationRequired(
  session: Session | null | undefined,
): boolean {
  const email = sessionTraitEmail(session);
  if (!email) return false;
  const addresses = session?.identity?.verifiable_addresses;
  if (!Array.isArray(addresses)) return false;
  const want = email.toLowerCase();
  return !addresses.some(
    (address) =>
      address.verified === true &&
      typeof address.value === "string" &&
      address.value.toLowerCase() === want,
  );
}
