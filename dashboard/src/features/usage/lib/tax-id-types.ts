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

/** One Stripe customer tax ID type a country's businesses use (w6/m628). */
export interface TaxIdTypeOption {
  /** Stripe's customer tax ID type: eu_vat, gb_vat, us_ein, … */
  readonly type: string;
  /** A value of that type, as Stripe's documentation gives it. */
  readonly example: string;
}

/**
 * The Stripe customer tax ID types each country offers, by ISO 3166-1
 * alpha-2 code, the first being the one a form preselects (w6/m628). From
 * Stripe's "Supported Tax ID types" table
 * (https://docs.stripe.com/billing/customer/tax-ids), less the types bex-api
 * does not take (stripeTaxIDTypes in
 * lego/backend/internal/billing/tax_id_types.go). Render's billing info form
 * shows its VAT field once an EU country is chosen; a country missing here
 * has no tax ID field.
 */
export const TAX_ID_TYPES: Readonly<
  Record<string, readonly TaxIdTypeOption[]>
> = {
  AD: [{ type: "ad_nrt", example: "A-123456-Z" }],
  AE: [{ type: "ae_trn", example: "123456789012345" }],
  AL: [{ type: "al_tin", example: "J12345678N" }],
  AM: [{ type: "am_tin", example: "02538904" }],
  AO: [{ type: "ao_tin", example: "5123456789" }],
  AR: [{ type: "ar_cuit", example: "12-3456789-01" }],
  AT: [{ type: "eu_vat", example: "ATU12345678" }],
  AU: [
    { type: "au_abn", example: "12345678912" },
    { type: "au_arn", example: "123456789123" },
  ],
  AW: [{ type: "aw_tin", example: "12345678" }],
  AZ: [{ type: "az_tin", example: "0123456789" }],
  BA: [{ type: "ba_tin", example: "123456789012" }],
  BB: [{ type: "bb_tin", example: "1123456789012" }],
  BD: [{ type: "bd_bin", example: "123456789-0123" }],
  BE: [{ type: "eu_vat", example: "BE0123456789" }],
  BF: [{ type: "bf_ifu", example: "12345678A" }],
  BG: [
    { type: "eu_vat", example: "BG0123456789" },
    { type: "bg_uic", example: "123456789" },
  ],
  BH: [{ type: "bh_vat", example: "123456789012345" }],
  BJ: [{ type: "bj_ifu", example: "1234567890123" }],
  BO: [{ type: "bo_tin", example: "123456789" }],
  BR: [
    { type: "br_cnpj", example: "01.234.456/5432-10" },
    { type: "br_cpf", example: "123.456.789-87" },
  ],
  BS: [{ type: "bs_tin", example: "123.456.789" }],
  BY: [{ type: "by_tin", example: "123456789" }],
  CA: [
    { type: "ca_bn", example: "123456789" },
    { type: "ca_gst_hst", example: "123456789RT0002" },
    { type: "ca_pst_bc", example: "PST-1234-5678" },
    { type: "ca_pst_mb", example: "123456-7" },
    { type: "ca_pst_sk", example: "1234567" },
    { type: "ca_qst", example: "1234567890TQ1234" },
  ],
  CD: [{ type: "cd_nif", example: "A0123456M" }],
  CH: [
    { type: "ch_uid", example: "CHE-123.456.789 HR" },
    { type: "ch_vat", example: "CHE-123.456.789 MWST" },
  ],
  CL: [{ type: "cl_tin", example: "12.345.678-K" }],
  CM: [{ type: "cm_niu", example: "M123456789000L" }],
  CN: [{ type: "cn_tin", example: "123456789012345678" }],
  CO: [{ type: "co_nit", example: "123.456.789-0" }],
  CR: [{ type: "cr_tin", example: "1-234-567890" }],
  CV: [{ type: "cv_nif", example: "213456789" }],
  CY: [{ type: "eu_vat", example: "CY12345678Z" }],
  CZ: [{ type: "eu_vat", example: "CZ1234567890" }],
  DE: [
    { type: "eu_vat", example: "DE123456789" },
    { type: "de_stn", example: "1234567890" },
  ],
  DK: [{ type: "eu_vat", example: "DK12345678" }],
  DO: [{ type: "do_rcn", example: "123-4567890-1" }],
  EC: [{ type: "ec_ruc", example: "1234567890001" }],
  EE: [{ type: "eu_vat", example: "EE123456789" }],
  EG: [{ type: "eg_tin", example: "123456789" }],
  ES: [
    { type: "eu_vat", example: "ESA1234567Z" },
    { type: "es_cif", example: "A12345678" },
  ],
  ET: [{ type: "et_tin", example: "1234567890" }],
  FI: [{ type: "eu_vat", example: "FI12345678" }],
  FO: [{ type: "fo_vat", example: "FO123456" }],
  FR: [{ type: "eu_vat", example: "FRAB123456789" }],
  GB: [
    { type: "gb_vat", example: "GB123456789" },
    { type: "eu_vat", example: "XI123456789" },
  ],
  GE: [{ type: "ge_vat", example: "123456789" }],
  GI: [{ type: "gi_tin", example: "12345" }],
  GN: [{ type: "gn_nif", example: "123456789" }],
  GR: [{ type: "eu_vat", example: "EL123456789" }],
  HK: [{ type: "hk_br", example: "12345678" }],
  HR: [
    { type: "eu_vat", example: "HR12345678912" },
    { type: "hr_oib", example: "12345678901" },
  ],
  HU: [
    { type: "eu_vat", example: "HU12345678" },
    { type: "hu_tin", example: "12345678-1-23" },
  ],
  ID: [{ type: "id_npwp", example: "012.345.678.9-012.345" }],
  IE: [{ type: "eu_vat", example: "IE1234567AB" }],
  IL: [{ type: "il_vat", example: "000012345" }],
  IN: [{ type: "in_gst", example: "12ABCDE3456FGZH" }],
  IS: [{ type: "is_vat", example: "123456" }],
  IT: [
    { type: "eu_vat", example: "IT12345678912" },
    { type: "it_cf", example: "ABCDEF12A12A123A" },
  ],
  JP: [
    { type: "jp_cn", example: "1234567891234" },
    { type: "jp_rn", example: "12345" },
    { type: "jp_trn", example: "T1234567891234" },
  ],
  KE: [{ type: "ke_pin", example: "P000111111A" }],
  KG: [{ type: "kg_tin", example: "12345678901234" }],
  KH: [{ type: "kh_tin", example: "1001-123456789" }],
  KR: [{ type: "kr_brn", example: "123-45-67890" }],
  KZ: [{ type: "kz_bin", example: "123456789012" }],
  LA: [{ type: "la_tin", example: "123456789-000" }],
  LI: [
    { type: "li_uid", example: "CHE123456789" },
    { type: "li_vat", example: "12345" },
  ],
  LK: [{ type: "lk_vat", example: "123456789-1234" }],
  LT: [{ type: "eu_vat", example: "LT123456789123" }],
  LU: [{ type: "eu_vat", example: "LU12345678" }],
  LV: [{ type: "eu_vat", example: "LV12345678912" }],
  MA: [{ type: "ma_vat", example: "12345678" }],
  MD: [{ type: "md_vat", example: "1234567" }],
  ME: [{ type: "me_pib", example: "12345678" }],
  MK: [{ type: "mk_vat", example: "MK1234567890123" }],
  MR: [{ type: "mr_nif", example: "12345678" }],
  MT: [{ type: "eu_vat", example: "MT12345678" }],
  MX: [{ type: "mx_rfc", example: "ABC010203AB9" }],
  MY: [
    { type: "my_frp", example: "12345678" },
    { type: "my_itn", example: "C 1234567890" },
    { type: "my_sst", example: "A12-3456-78912345" },
  ],
  NG: [{ type: "ng_tin", example: "1234567890123" }],
  NL: [{ type: "eu_vat", example: "NL123456789B12" }],
  NO: [
    { type: "no_vat", example: "123456789MVA" },
    { type: "no_voec", example: "1234567" },
  ],
  NP: [{ type: "np_pan", example: "123456789" }],
  NZ: [{ type: "nz_gst", example: "123456789" }],
  OM: [{ type: "om_vat", example: "OM1234567890" }],
  PE: [{ type: "pe_ruc", example: "12345678901" }],
  PH: [{ type: "ph_tin", example: "123456789012" }],
  PL: [
    { type: "eu_vat", example: "PL1234567890" },
    { type: "pl_nip", example: "1234567890" },
  ],
  PT: [{ type: "eu_vat", example: "PT123456789" }],
  PY: [{ type: "py_ruc", example: "12345678A" }],
  RO: [
    { type: "eu_vat", example: "RO1234567891" },
    { type: "ro_tin", example: "1234567890123" },
  ],
  RS: [{ type: "rs_pib", example: "123456789" }],
  RU: [
    { type: "ru_inn", example: "1234567891" },
    { type: "ru_kpp", example: "123456789" },
  ],
  SA: [{ type: "sa_vat", example: "123456789012345" }],
  SE: [{ type: "eu_vat", example: "SE123456789123" }],
  SG: [
    { type: "sg_gst", example: "M12345678X" },
    { type: "sg_uen", example: "123456789F" },
  ],
  SI: [
    { type: "eu_vat", example: "SI12345678" },
    { type: "si_tin", example: "12345678" },
  ],
  SK: [{ type: "eu_vat", example: "SK1234567891" }],
  SN: [{ type: "sn_ninea", example: "12345672A2" }],
  SR: [{ type: "sr_fin", example: "1234567890" }],
  SV: [{ type: "sv_nit", example: "1234-567890-123-4" }],
  TH: [{ type: "th_vat", example: "1234567891234" }],
  TJ: [{ type: "tj_tin", example: "123456789" }],
  TR: [{ type: "tr_tin", example: "0123456789" }],
  TW: [{ type: "tw_vat", example: "12345678" }],
  TZ: [{ type: "tz_vat", example: "12345678A" }],
  UA: [{ type: "ua_vat", example: "123456789" }],
  UG: [{ type: "ug_tin", example: "1014751879" }],
  US: [{ type: "us_ein", example: "12-3456789" }],
  UY: [{ type: "uy_ruc", example: "123456789012" }],
  UZ: [
    { type: "uz_tin", example: "123456789" },
    { type: "uz_vat", example: "123456789012" },
  ],
  VE: [{ type: "ve_rif", example: "A-12345678-9" }],
  VN: [{ type: "vn_tin", example: "1234567890" }],
  ZA: [{ type: "za_vat", example: "4123456789" }],
  ZM: [{ type: "zm_tin", example: "1004751879" }],
  ZW: [{ type: "zw_tin", example: "1234567890" }],
};

const NO_TYPES: readonly TaxIdTypeOption[] = [];

/** The tax ID types a country offers, its default first; none for "". */
export function taxIdTypesFor(country: string): readonly TaxIdTypeOption[] {
  return TAX_ID_TYPES[country.toUpperCase()] ?? NO_TYPES;
}

// Each type's example from the first country offering it, for a type the
// chosen country does not list.
const FIRST_EXAMPLES: ReadonlyMap<string, string> = new Map(
  Object.values(TAX_ID_TYPES)
    .flat()
    .reverse()
    .map(({ type, example }) => [type, example]),
);

/**
 * An example value of a tax ID type: the chosen country's own (EU VAT reads
 * "FRAB123456789" in France, "DE123456789" in Germany), else the first
 * country's that offers the type, else "".
 */
export function taxIdExample(type: string, country = ""): string {
  return (
    taxIdTypesFor(country).find((option) => option.type === type)?.example ??
    FIRST_EXAMPLES.get(type) ??
    ""
  );
}

/**
 * A tax ID type as a form and the Billing page show it: Stripe's code upper
 * cased, underscores as spaces ("eu_vat" → "EU VAT", "ca_gst_hst" →
 * "CA GST HST"), so it reads as the abbreviation a business knows.
 */
export function taxIdTypeLabel(type: string): string {
  return type.toUpperCase().replace(/_/g, " ");
}
