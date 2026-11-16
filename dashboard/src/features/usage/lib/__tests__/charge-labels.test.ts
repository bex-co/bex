import { beforeAll, describe, expect, it } from "vitest";
import i18n, { ensureLanguage } from "@/i18n/init";
import { SUPPORTED_LANGUAGES } from "@/i18n/config";
import { goPackageConstants } from "@/test/go-source";
import { CATEGORY_LABEL_KEYS, CHARGE_LABEL_KEYS } from "../charge-labels";
import { CATEGORY_ORDER } from "../charges";

// The charge tree names what bex-api meters. bex-api declares the usage kinds
// (`UsageKind…`) and the resource kinds (`ResourceKind…`) as Go constants, and
// the three object storage meters reached `GET /v1/usage` with no label here,
// so a customer saw raw meter names (w21/086). Both lists are read from the Go
// source, so a kind added there without a label, in every language, fails this
// suite instead of reaching a customer.

const STORE = "lego/backend/internal/store";
// The files that declare them today. Naming them here makes
// scripts/ci_test_inputs.py require them in test (dashboard)'s trigger paths.
const DECLARING_FILES = [
  "lego/backend/internal/store/usage.go",
  "lego/backend/internal/store/agent_model_usage.go",
];

const usageKinds = goPackageConstants(STORE, "UsageKind");
const resourceKinds = goPackageConstants(STORE, "ResourceKind");

describe("the kinds bex-api declares", () => {
  it("are found in the Go source, the object storage meters among them", () => {
    // A pattern that stopped matching would pass every check below on nothing.
    expect(usageKinds.size).toBeGreaterThanOrEqual(15);
    const values = [...usageKinds.values()].map((c) => c.value);
    expect(values).toEqual(
      expect.arrayContaining([
        "instance_seconds",
        "object_storage_gb_seconds",
        "object_storage_ingress_bytes",
        "object_storage_egress_bytes",
        "agent_model_cost_microusd",
      ]),
    );
    expect([...resourceKinds.values()].map((c) => c.value)).toEqual(
      expect.arrayContaining(["service", "object_storage", "agent_session"]),
    );
  });

  it("are declared only in files whose changes run this suite", () => {
    const files = new Set(
      [...usageKinds.values(), ...resourceKinds.values()].map((c) => c.file),
    );
    for (const file of files) {
      expect(
        DECLARING_FILES,
        `${file} declares a usage or resource kind: add it to DECLARING_FILES here and to the paths of .github/workflows/dashboard-test.yml`,
      ).toContain(file);
    }
  });
});

describe("the charge tree's labels", () => {
  beforeAll(async () => {
    for (const lang of SUPPORTED_LANGUAGES) await ensureLanguage(lang);
  });

  it("name every usage kind", () => {
    for (const [name, { value }] of usageKinds) {
      expect(
        CHARGE_LABEL_KEYS[value],
        `${name} (${value}) has no label in CHARGE_LABEL_KEYS`,
      ).toBeDefined();
    }
  });

  it("name every resource kind, in the category order", () => {
    for (const [name, { value }] of resourceKinds) {
      expect(
        CATEGORY_LABEL_KEYS[value],
        `${name} (${value}) has no label in CATEGORY_LABEL_KEYS`,
      ).toBeDefined();
      expect(
        CATEGORY_ORDER as readonly string[],
        `${name} (${value}) is missing from CATEGORY_ORDER`,
      ).toContain(value);
    }
  });

  it("name nothing bex-api does not declare", () => {
    const declaredUsage = new Set([...usageKinds.values()].map((c) => c.value));
    for (const kind of Object.keys(CHARGE_LABEL_KEYS)) {
      expect(declaredUsage, `${kind} is not a UsageKind`).toContain(kind);
    }
    const declaredResource = new Set(
      [...resourceKinds.values()].map((c) => c.value),
    );
    for (const kind of Object.keys(CATEGORY_LABEL_KEYS)) {
      expect(declaredResource, `${kind} is not a ResourceKind`).toContain(kind);
    }
  });

  it.each(SUPPORTED_LANGUAGES)("read as words in %s", (lang) => {
    const keys = [
      ...Object.values(CHARGE_LABEL_KEYS),
      ...Object.values(CATEGORY_LABEL_KEYS),
    ];
    for (const key of keys) {
      // The language's own catalog, not the English one i18next falls back to.
      const label: unknown = i18n.getResource(lang, "translation", key);
      expect(label, `${key} has no ${lang} string`).toEqual(
        expect.stringMatching(/\S/),
      );
    }
  });
});
