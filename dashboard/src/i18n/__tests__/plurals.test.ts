import { describe, it, expect, afterEach, beforeAll } from "vitest";
import i18n, { ensureLanguage } from "@/i18n/init";

// THE pluralization convention (w6/062) — copy this pattern for any new
// count-bearing string:
//
//   en.ts:  "ns.key_one":   { message: "{count} thing", ... }
//           "ns.key_other": { message: "{count} things", ... }
//   zh.ts:  "ns.key_other": { message: "{count} 个东西", ... }   // _other ONLY
//   call:   t("ns.key", { count })                               // count: number
//
// i18next resolves the `_one`/`_other` suffix natively from the numeric
// `count` via Intl.PluralRules — no ternaries, no hand-rolled One/Many key
// pairs, no "(s)". zh's only cardinal plural category is "other", so its
// catalog never carries `_one` (locale-parity.test.ts enforces both shapes).
//
// This test proves the resolution works against the app's REAL init config —
// flat dotted keys ("deploys.listCount") and the custom single-brace
// interpolation prefix/suffix — not a synthetic catalog.
describe("pluralization convention (native _one/_other)", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("t(key, { count: 1 }) picks the _one form in en", () => {
    expect(i18n.t("deploys.listCount", { count: 1 })).toBe("1 deploy");
  });

  it("t(key, { count: n≠1 }) picks the _other form in en (including 0)", () => {
    expect(i18n.t("deploys.listCount", { count: 2 })).toBe("2 deploys");
    expect(i18n.t("deploys.listCount", { count: 0 })).toBe("0 deploys");
  });

  it("zh resolves every count through its single _other form", () => {
    // zh's catalog deliberately has no `_one` entry (see locale-parity).
    expect(i18n.t("deploys.listCount", { count: 1, lng: "zh" })).toBe(
      "1 个部署",
    );
    expect(i18n.t("deploys.listCount", { count: 5, lng: "zh" })).toBe(
      "5 个部署",
    );
  });

  it("a two-count sentence is composed from two pluralized halves", () => {
    // "{a} variables changed · {b} files changed" cannot be one key — a
    // single i18next `count` drives one plural decision. Split it and compose
    // at the call site (service-environment-editor.tsx is the reference).
    const vars = i18n.t("services.environmentUnsavedVariables", { count: 1 });
    const files = i18n.t("services.environmentUnsavedFiles", { count: 2 });
    expect(`${vars} · ${files}`).toBe("1 variable changed · 2 files changed");
  });
});

// Every other language carries exactly its own CLDR categories (locale-parity
// enforces the key shapes); these prove i18next selects them from the real
// catalogs. Each is a lazy chunk, registered the way the app does it.
describe("pluralization in languages with other plural categories", () => {
  beforeAll(async () => {
    for (const lang of ["ru", "es", "pt", "de", "ja", "ko"]) {
      await ensureLanguage(lang);
    }
  });

  const count = (lng: string, n: number) =>
    i18n.t("deploys.listCount", { count: n, lng });

  it("ru picks _one, _few and _many by the last digits", () => {
    expect(count("ru", 1)).toBe("1 деплой");
    expect(count("ru", 21)).toBe("21 деплой");
    expect(count("ru", 3)).toBe("3 деплоя");
    expect(count("ru", 24)).toBe("24 деплоя");
    expect(count("ru", 5)).toBe("5 деплоев");
    expect(count("ru", 11)).toBe("11 деплоев");
    expect(count("ru", 0)).toBe("0 деплоев");
  });

  it("pt reads 0 as singular; es and pt have a _many form for millions", () => {
    expect(count("pt", 0)).toBe("0 deploy");
    expect(count("pt", 2)).toBe("2 deploys");
    expect(count("es", 0)).toBe("0 despliegues");
    expect(count("es", 1)).toBe("1 despliegue");
    expect(count("es", 1_000_000)).toBe("1000000 de despliegues");
  });

  it("de has _one/_other; ja and ko use their single _other form", () => {
    expect(count("de", 1)).toBe("1 Deployment");
    expect(count("de", 2)).toBe("2 Deployments");
    expect(count("ja", 1)).toBe("1 件のデプロイ");
    expect(count("ko", 7)).toBe("배포 7개");
  });
});
