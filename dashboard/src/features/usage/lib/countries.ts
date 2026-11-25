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

/**
 * Every officially assigned ISO 3166-1 alpha-2 code: the countries a billing
 * address may name (w6/m628). bex-api takes any two-letter code and Stripe
 * checks it, so the list only has to name what a member may choose.
 */
export const COUNTRY_CODES: readonly string[] = (
  "AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ " +
  "BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ " +
  "CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ " +
  "DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR " +
  "GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY " +
  "HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP " +
  "KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY " +
  "MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ " +
  "NA NC NE NF NG NI NL NO NP NR NU NZ OM " +
  "PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW " +
  "SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ " +
  "TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ " +
  "UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS YE YT ZA ZM ZW"
).split(" ");

// One per locale: the card names its address's country on every render.
const regionNamesByLocale = new Map<string, Intl.DisplayNames | null>();

function regionNames(locale: string): Intl.DisplayNames | null {
  if (!regionNamesByLocale.has(locale)) {
    let names: Intl.DisplayNames | null = null;
    try {
      names = new Intl.DisplayNames([locale], { type: "region" });
    } catch {
      // The runtime cannot name regions in locale: codes are shown.
    }
    regionNamesByLocale.set(locale, names);
  }
  return regionNamesByLocale.get(locale) ?? null;
}

function nameOf(names: Intl.DisplayNames | null, code: string): string {
  try {
    return names?.of(code) ?? code;
  } catch {
    // A code Intl does not take (not two letters) is shown as it came.
    return code;
  }
}

/** A country's name in locale, or its code when the runtime cannot name it. */
export function countryName(code: string, locale: string): string {
  return code ? nameOf(regionNames(locale), code.toUpperCase()) : "";
}

export interface CountryOption {
  code: string;
  name: string;
}

/**
 * Every country, named in locale and sorted by that name as locale sorts,
 * for a country picker. `extra` adds a stored code the list lacks, so a value
 * set elsewhere (the Stripe portal) stays choosable.
 */
export function countryOptions(locale: string, extra = ""): CountryOption[] {
  const names = regionNames(locale);
  const codes =
    extra && !COUNTRY_CODES.includes(extra)
      ? [...COUNTRY_CODES, extra]
      : COUNTRY_CODES;
  const collator = new Intl.Collator([locale]);
  return codes
    .map((code) => ({ code, name: nameOf(names, code) }))
    .sort((a, b) => collator.compare(a.name, b.name));
}
