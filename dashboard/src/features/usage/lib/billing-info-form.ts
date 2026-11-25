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

import type { BillingInfoInput } from "../hooks/use-billing-info";
import { countryName } from "./countries";

/** The billing info dialog's fields as typed (w6/m628). */
export interface BillingInfoDraft {
  companyName: string;
  line1: string;
  line2: string;
  city: string;
  state: string;
  postalCode: string;
  /** ISO 3166-1 alpha-2, or "" for none. */
  country: string;
  /** The tax ID type the form shows, or "" when it shows no tax ID field. */
  taxType: string;
  taxValue: string;
}

/**
 * The replacement a save sends, every field trimmed: no address when no
 * address field and no country is filled, no tax ID when its value is empty
 * or the form shows no tax ID field. An address line without a country is the
 * one thing refused here ("address.country"), since bex-api takes no address
 * without one; every other rule is bex-api's and Stripe's to judge.
 */
export function billingInfoPayload(
  draft: BillingInfoDraft,
): { input: BillingInfoInput } | { missing: "address.country" } {
  const line1 = draft.line1.trim();
  const line2 = draft.line2.trim();
  const city = draft.city.trim();
  const state = draft.state.trim();
  const postalCode = draft.postalCode.trim();
  const country = draft.country.trim();
  const taxValue = draft.taxValue.trim();
  if (!country && (line1 || line2 || city || state || postalCode)) {
    return { missing: "address.country" };
  }
  return {
    input: {
      companyName: draft.companyName.trim(),
      address: country
        ? { line1, line2, city, state, postalCode, country }
        : null,
      taxId:
        draft.taxType && taxValue
          ? { type: draft.taxType, value: taxValue }
          : null,
    },
  };
}

/**
 * An address as the card shows it, one line per part that is set: the two
 * street lines, then city, state and postal code on one line, then the
 * country by its name in locale.
 */
export function addressLines(
  address: {
    line1: string;
    line2: string;
    city: string;
    state: string;
    postalCode: string;
    country: string;
  },
  locale: string,
): string[] {
  const place = [address.city, address.state, address.postalCode]
    .filter(Boolean)
    .join(", ");
  return [
    address.line1,
    address.line2,
    place,
    countryName(address.country, locale),
  ].filter(Boolean);
}
