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

import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  TAX_ID_TYPES,
  taxIdExample,
  taxIdTypeLabel,
  taxIdTypesFor,
} from "../tax-id-types";
import { COUNTRY_CODES, countryName, countryOptions } from "../countries";
import { addressLines, billingInfoPayload } from "../billing-info-form";

// The tax ID types bex-api takes (stripeTaxIDTypes): a type the form offers
// that bex-api refuses would fail every save that used it. Naming the file
// here makes scripts/ci_test_inputs.py require it in the paths of
// .github/workflows/dashboard-test.yml.
const BACKEND_TYPES_FILE = "lego/backend/internal/billing/tax_id_types.go";

function backendTaxIdTypes(): Set<string> {
  const source = readFileSync(
    `${process.cwd()}/../${BACKEND_TYPES_FILE}`,
    "utf8",
  );
  const start = source.indexOf("for _, t := range []string{");
  expect(start, `${BACKEND_TYPES_FILE} lists no types`).toBeGreaterThan(0);
  const block = source.slice(start, source.indexOf("}", start));
  return new Set([...block.matchAll(/"([a-z_]+)"/g)].map((m) => m[1]));
}

const EU = (
  "AT BE BG CY CZ DE DK EE ES FI FR GR HR HU IE IT LT LU LV MT NL PL PT RO " +
  "SE SI SK"
).split(" ");

// w6/m628: the billing info form's tax ID field follows the chosen country.
describe("TAX_ID_TYPES", () => {
  it("offers only types bex-api takes", () => {
    const accepted = backendTaxIdTypes();
    // A pattern that stopped matching would pass the check below on nothing.
    expect(accepted.size).toBeGreaterThan(100);
    const offered = Object.values(TAX_ID_TYPES)
      .flat()
      .map((option) => option.type);
    expect(offered.filter((type) => !accepted.has(type))).toEqual([]);
  });

  it("is keyed by countries the country picker offers, each with an example", () => {
    for (const [country, options] of Object.entries(TAX_ID_TYPES)) {
      expect(COUNTRY_CODES).toContain(country);
      expect(options.length).toBeGreaterThan(0);
      for (const option of options) expect(option.example).not.toBe("");
    }
  });

  it("preselects EU VAT in every EU member state, as Render's VAT field", () => {
    for (const country of EU) {
      expect(taxIdTypesFor(country)[0]?.type, country).toBe("eu_vat");
    }
  });

  it("names a country's types, its default first, and none without a country", () => {
    expect(taxIdTypesFor("de").map((option) => option.type)).toEqual([
      "eu_vat",
      "de_stn",
    ]);
    expect(taxIdTypesFor("US")).toEqual([
      { type: "us_ein", example: "12-3456789" },
    ]);
    expect(taxIdTypesFor("")).toEqual([]);
    // A country Stripe has no tax ID type for.
    expect(taxIdTypesFor("AQ")).toEqual([]);
  });

  it("labels a type as its code, upper cased, underscores as spaces", () => {
    expect(taxIdTypeLabel("eu_vat")).toBe("EU VAT");
    expect(taxIdTypeLabel("ca_gst_hst")).toBe("CA GST HST");
  });

  it("gives a type's example in the chosen country, and none for a type it does not know", () => {
    expect(taxIdExample("eu_vat", "FR")).toBe("FRAB123456789");
    expect(taxIdExample("eu_vat", "DE")).toBe("DE123456789");
    // Outside the countries that list it, the first one's.
    expect(taxIdExample("eu_vat", "US")).toBe("ATU12345678");
    expect(taxIdExample("gb_vat")).toBe("GB123456789");
    expect(taxIdExample("xx_unknown")).toBe("");
  });
});

describe("COUNTRY_CODES", () => {
  it("lists each of the 249 assigned ISO 3166-1 alpha-2 codes once", () => {
    expect(COUNTRY_CODES).toHaveLength(249);
    expect(new Set(COUNTRY_CODES).size).toBe(249);
    for (const code of COUNTRY_CODES) expect(code).toMatch(/^[A-Z]{2}$/);
  });

  it("names and sorts every country in the UI language", () => {
    const en = countryOptions("en");
    expect(en).toHaveLength(249);
    expect(en.find((option) => option.code === "DE")?.name).toBe("Germany");
    const names = en.map((option) => option.name);
    const collator = new Intl.Collator(["en"]);
    expect([...names].sort(collator.compare)).toEqual(names);
    expect(
      countryOptions("de").find((option) => option.code === "DE")?.name,
    ).toBe("Deutschland");
    expect(countryName("jp", "en")).toBe("Japan");
  });

  it("keeps a stored code the list lacks choosable", () => {
    expect(countryOptions("en", "XK").map((option) => option.code)).toContain(
      "XK",
    );
    expect(countryOptions("en", "DE")).toHaveLength(249);
  });
});

describe("billingInfoPayload", () => {
  const draft = {
    companyName: "",
    line1: "",
    line2: "",
    city: "",
    state: "",
    postalCode: "",
    country: "",
    taxType: "",
    taxValue: "",
  };

  it("sends no address and no tax ID when none is filled", () => {
    expect(billingInfoPayload({ ...draft, companyName: "  " })).toEqual({
      input: { companyName: "", address: null, taxId: null },
    });
  });

  it("trims every field it sends", () => {
    expect(
      billingInfoPayload({
        ...draft,
        companyName: " Acme GmbH ",
        line1: " Hauptstr. 1 ",
        city: "Berlin ",
        postalCode: " 10115",
        country: "DE",
        taxType: "eu_vat",
        taxValue: " DE123456789 ",
      }),
    ).toEqual({
      input: {
        companyName: "Acme GmbH",
        address: {
          line1: "Hauptstr. 1",
          line2: "",
          city: "Berlin",
          state: "",
          postalCode: "10115",
          country: "DE",
        },
        taxId: { type: "eu_vat", value: "DE123456789" },
      },
    });
  });

  it("refuses an address line without a country", () => {
    expect(billingInfoPayload({ ...draft, city: "Berlin" })).toEqual({
      missing: "address.country",
    });
  });

  it("sends no tax ID without a value, or without a type to give it", () => {
    const taxId = (fields: Partial<typeof draft>) => {
      const payload = billingInfoPayload({ ...draft, ...fields });
      if (!("input" in payload)) throw new Error("refused");
      return payload.input.taxId;
    };
    expect(taxId({ country: "DE", taxType: "eu_vat" })).toBeNull();
    expect(taxId({ country: "AQ", taxValue: "123" })).toBeNull();
  });
});

describe("addressLines", () => {
  it("prints the parts that are set, the country by its name", () => {
    expect(
      addressLines(
        {
          line1: "Hauptstr. 1",
          line2: "",
          city: "Berlin",
          state: "",
          postalCode: "10115",
          country: "DE",
        },
        "en",
      ),
    ).toEqual(["Hauptstr. 1", "Berlin, 10115", "Germany"]);
  });
});
