/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package billing

// stripeTaxIDTypes are the customer tax ID types Stripe takes at the API
// version stripe-go v86 pins (TaxIDParams.Type), less `unknown`, which only
// legacy tax IDs carry. Billing info refuses any other type by name before
// asking Stripe; Stripe still checks the value against its type.
var stripeTaxIDTypes = map[string]bool{}

func init() {
	for _, t := range []string{
		"ad_nrt", "ae_trn", "al_tin", "am_tin", "ao_tin", "ar_cuit", "au_abn", "au_arn",
		"aw_tin", "az_tin", "ba_tin", "bb_tin", "bd_bin", "bf_ifu", "bg_uic", "bh_vat",
		"bj_ifu", "bo_tin", "br_cnpj", "br_cpf", "bs_tin", "by_tin", "ca_bn", "ca_gst_hst",
		"ca_pst_bc", "ca_pst_mb", "ca_pst_sk", "ca_qst", "cd_nif", "ch_uid", "ch_vat", "cl_tin",
		"cm_niu", "cn_tin", "co_nit", "cr_tin", "cv_nif", "de_stn", "do_rcn", "ec_ruc",
		"eg_tin", "es_cif", "et_tin", "eu_oss_vat", "eu_vat", "fo_vat", "gb_vat", "ge_vat",
		"gi_tin", "gn_nif", "hk_br", "hr_oib", "hu_tin", "ic_nif", "id_npwp", "il_vat",
		"in_gst", "is_vat", "it_cf", "jp_cn", "jp_rn", "jp_trn", "ke_pin", "kg_tin",
		"kh_tin", "kr_brn", "kz_bin", "la_tin", "li_uid", "li_vat", "lk_vat", "ma_vat",
		"md_vat", "me_pib", "mk_vat", "mr_nif", "mx_rfc", "my_frp", "my_itn", "my_sst",
		"ng_tin", "no_vat", "no_voec", "np_pan", "nz_gst", "om_vat", "pe_ruc", "ph_tin",
		"pl_nip", "py_ruc", "ro_tin", "rs_pib", "ru_inn", "ru_kpp", "sa_vat", "sg_gst",
		"sg_uen", "si_tin", "sn_ninea", "sr_fin", "sv_nit", "th_vat", "tj_tin", "tr_tin",
		"tw_vat", "tz_vat", "ua_vat", "ug_tin", "us_ein", "uy_ruc", "uz_tin", "uz_vat",
		"ve_rif", "vn_tin", "za_vat", "zm_tin", "zw_tin",
	} {
		stripeTaxIDTypes[t] = true
	}
}
