import { describe, it, expect } from "vitest";
import {
  DEFAULT_LANGUAGE,
  SUPPORTED_LANGUAGES,
  type SupportedLanguage,
} from "@/i18n/config";

// Locale key parity across EVERY supported language, AUTO-DISCOVERED (w5/033).
// The i18n tooling only console.warns in dev on a missing key
// (use-translations.ts); nothing else fails CI when a translation lags en as
// features add strings. import.meta.glob discovers every per-feature + common
// catalog, so a new feature's — or a new language's — files are checked without
// editing this test, and any drift fails CI (yarn test gates deploy.yml).
//
// Path is relative to this file (src/i18n/__tests__/): ../../ == src/.
type Entry = { message: string; description: string };
type Catalog = Record<string, Entry>;
const modules = import.meta.glob<{ default: Catalog }>(
  "../../**/locales/*.ts",
  { eager: true },
);

// Group by locale directory (e.g. "features/metrics") -> one catalog per language.
type Namespace = { name: string } & Partial<Record<SupportedLanguage, Catalog>>;
const byDir: Record<string, Namespace> = {};
for (const [path, mod] of Object.entries(modules)) {
  const m = path.match(/\/([^/]+)\/locales\/([a-z]{2})\.ts$/);
  if (!m) continue;
  const [, name, lang] = m;
  (byDir[name] ??= { name })[lang as SupportedLanguage] = mod.default;
}
const NAMESPACES = Object.values(byDir).sort((a, b) =>
  a.name.localeCompare(b.name),
);

// Every translated language, and one (namespace, language) case per catalog.
const TRANSLATIONS = SUPPORTED_LANGUAGES.filter(
  (lang) => lang !== DEFAULT_LANGUAGE,
);
const CATALOGS = NAMESPACES.flatMap(({ name, en, ...rest }) =>
  TRANSLATIONS.map((lang) => ({
    name,
    lang,
    en: en ?? {},
    catalog: (rest as Namespace)[lang] ?? {},
  })),
);

// Intentional gaps: keys allowed to differ from en's key set. Keep empty — add
// a key here (with a why) only for a deliberate divergence.
const ALLOW_DIVERGENT = new Set<string>([]);

// Keys that interpolate {count} but need no plural form: the count is a label
// or a badge, with no noun agreeing with it ("Selected: {count}"). Everything
// else that carries {count} must use _one/_other (w6/062's rule, re-enforced by
// w1/m159 after "1 resources to sync" shipped).
const LABEL_ONLY_COUNTS = new Set<string>([
  "git.credentialsTrigger",
  "projects.selectedCount",
  "services.eventsFilterSelected",
  "services.envGroupsLinkedCount",
  "services.envGroupsAvailableCount",
  "webhooks.showMore",
  "usage.resourceCapsFinishingDeletion",
  // Compact duration abbreviations ("3d", "5h", "12m"): the unit letter has no
  // singular form, so "1d" is already correct and a plural pair would be two
  // identical strings. Added 2026-09-15 — f6604b5e1 landed them without an
  // exemption and broke this suite on main.
  "router.day",
  "router.hour",
  "router.minute",
]);

// Native plurals (w6/062): en authors `key_one` + `key_other`; every other
// language carries exactly its own CLDR cardinal categories for that key —
// zh/ja/ko `_other` only, de `_one`/`_other`, es/pt `_one`/`_many`/`_other`,
// ru `_one`/`_few`/`_many`/`_other` — the forms i18next selects through
// Intl.PluralRules. A missing form falls back to English; an extra one is
// unreachable (do not mechanically copy en's `_one` into zh/ja/ko).
const pluralBase = (en: Catalog, key: string) => {
  const m = key.match(/^(.*)_(one|other)$/);
  return m && en[`${m[1]}_other`] ? m[1] : null;
};
function expectedKeys(en: Catalog, lang: SupportedLanguage): string[] {
  const categories = new Intl.PluralRules(lang).resolvedOptions()
    .pluralCategories;
  const keys = new Set<string>();
  for (const key of Object.keys(en)) {
    const base = pluralBase(en, key);
    if (base)
      for (const category of categories) keys.add(`${base}_${category}`);
    else keys.add(key);
  }
  return [...keys].filter((key) => !ALLOW_DIVERGENT.has(key)).sort();
}

// The en message a translated key renders in place of: itself, or for a
// plural form the en `_one` (for `_one`) or `_other` (every other category).
function englishSource(en: Catalog, key: string): Entry | undefined {
  if (en[key]) return en[key];
  const m = key.match(/^(.*)_(zero|two|few|many|other)$/);
  return m ? en[`${m[1]}_other`] : undefined;
}

// The distinct placeholders a message interpolates. A translation may repeat
// one where its grammar wants the name again instead of a pronoun.
const placeholders = (message: string) =>
  [...new Set(message.match(/\{[A-Za-z0-9_]+\}/g) ?? [])].sort();

// Keys whose translations interpolate a different parameter than en, by
// design. deploys.reasonWait.duration: en shows bex-api's Go duration
// ({wait}, "3h0m0s") as the API words it; every other language reads the
// duration in words ({units}) — deploy-reason.ts passes both (w5/m356).
const TRANSLATED_PLACEHOLDERS: Record<string, string[]> = {
  "deploys.reasonWait.duration": ["{units}"],
};

describe("locale key parity", () => {
  it("discovers every catalog of every language (glob is not silently empty)", () => {
    // A broken glob would generate zero it.each cases and vacuously "pass"; this
    // guards that the discovery actually found the full set.
    expect(NAMESPACES.length).toBeGreaterThanOrEqual(20);
    for (const ns of NAMESPACES) {
      for (const lang of SUPPORTED_LANGUAGES) {
        expect(
          ns[lang],
          `${ns.name} is missing its ${lang} locale`,
        ).toBeDefined();
      }
    }
  });

  it.each(CATALOGS)(
    "$name/$lang: has en's keys, in $lang's own plural forms",
    ({ lang, en, catalog }) => {
      const actual = Object.keys(catalog)
        .filter((key) => !ALLOW_DIVERGENT.has(key))
        .sort();
      expect(actual).toEqual(expectedKeys(en, lang));
    },
  );

  it.each(CATALOGS)(
    "$name/$lang: every message keeps en's placeholders",
    ({ en, catalog }) => {
      // A dropped or renamed {placeholder} renders literally or loses the value
      // (i18next leaves an unknown {name} in place) — in one language only.
      const mismatched = Object.entries(catalog)
        .filter(([key, entry]) => {
          const source = englishSource(en, key);
          const expected =
            TRANSLATED_PLACEHOLDERS[key] ??
            (source && placeholders(source.message));
          return (
            expected && expected.join() !== placeholders(entry.message).join()
          );
        })
        .map(([key, entry]) => `${key}: ${entry.message}`);
      expect(mismatched).toEqual([]);
    },
  );

  it.each(NAMESPACES)("$name: en plural keys are well-formed", ({ en }) => {
    // en: a `_one` form must have its `_other` sibling (English needs both).
    for (const k of Object.keys(en ?? {})) {
      if (!k.endsWith("_one")) continue;
      const other = `${k.slice(0, -"_one".length)}_other`;
      expect(
        en?.[other],
        `en has "${k}" but not its plural sibling "${other}"`,
      ).toBeDefined();
    }
  });

  it.each(NAMESPACES)("$name: count messages are pluralized", ({ en }) => {
    // A message that interpolates {count} must select a plural form, or it
    // reads "1 resources to sync" at one — which is exactly how w1/098 was
    // found live. The _one/_other entries are the forms themselves, and
    // LABEL_ONLY_COUNTS carries the deliberate exemptions.
    const offenders = Object.entries(en ?? {})
      .filter(([key]) => !key.endsWith("_one") && !key.endsWith("_other"))
      .filter(
        ([key, entry]) =>
          entry.message.includes("{count}") && !LABEL_ONLY_COUNTS.has(key),
      )
      .map(([key]) => key)
      .sort();
    expect(
      offenders,
      "keys interpolating {count} without _one/_other forms",
    ).toEqual([]);
  });

  it.each(NAMESPACES)("$name: keys share one consistent prefix", ({ en }) => {
    // Feature prefixes are camelCase and not mechanically derivable from the
    // kebab-case dir, so assert internal consistency instead of a fixed name:
    // every key shares the first key's dotted prefix (catches a stray/typo'd
    // namespace on a new key).
    const keys = Object.keys(en ?? {});
    if (keys.length === 0) return;
    const prefix = keys[0].split(".")[0];
    for (const key of keys) {
      expect(
        key.startsWith(`${prefix}.`),
        `key "${key}" is not prefixed "${prefix}."`,
      ).toBe(true);
    }
  });

  it.each(NAMESPACES)(
    "$name: every entry in every language has a non-empty message",
    (ns) => {
      for (const lang of SUPPORTED_LANGUAGES) {
        for (const entry of Object.values(ns[lang] ?? {})) {
          expect(entry.message.trim().length).toBeGreaterThan(0);
        }
      }
    },
  );
});

// zh messages that are deliberately the same as en: product, runtime and
// command names, codes, and units with no Chinese form. A multi-word English
// sentence copied into zh is a missing translation (w4/205: the whole Disk
// feature shipped English into zh and passed key parity).
const ZH_SAME_AS_EN = new Set<string>([
  // Product names.
  "blueprints.previewKindKeyValue",
  "usage.resourceCapsKeyValues",
  "webhooks.group.keyValue",
  // Key Value stays the product's English name in zh (w4/646 glossary).
  "blueprints.previewKeyValue",
  "environments.tabKeyValues",
  "keyvalue.cardTitle",
  "keyvalue.resourceType",
  "projects.typeKeyValue",
  "usage.categoryKeyValue",
  "usage.billingMode",
  "agentSessions.agent.muse",
  "agentSessions.agent.deepseek",
  // Example commands in input placeholders.
  "services.buildCommandPlaceholder",
  "services.createFieldCommandPlaceholder",
]);

// Two English words in a row, after placeholders and code spans are removed.
const englishPhrase = (message: string) =>
  /[A-Za-z]{2,},? +[A-Za-z]{2,}/.test(
    message.replace(/\{[^}]*\}/g, "").replace(/`[^`]*`/g, ""),
  );

describe("zh is translated", () => {
  it.each(NAMESPACES)(
    "$name: no zh message is an untranslated English phrase",
    ({ en, zh }) => {
      const copied = Object.entries(zh ?? {})
        .filter(
          ([key, entry]) =>
            entry.message === en?.[key]?.message &&
            englishPhrase(entry.message) &&
            !ZH_SAME_AS_EN.has(key),
        )
        .map(([key, entry]) => `${key}: ${entry.message}`);
      expect(copied).toEqual([]);
    },
  );
});

// One voice addresses the user: the informal 你, the majority form and the one
// the confirmation dialogs use, never the formal 您 (w4/573).
describe("zh addresses the user one way", () => {
  it.each(NAMESPACES)("$name: no zh message uses 您", ({ zh }) => {
    const formal = Object.entries(zh ?? {})
      .filter(([, entry]) => entry.message.includes("您"))
      .map(([key, entry]) => `${key}: ${entry.message}`);
    expect(formal).toEqual([]);
  });
});

// One Chinese name per product term (w4/617): the Pre-Deploy Command is
// 预部署命令 on Settings, so no other page may call it 部署前命令.
describe("zh names each product term one way", () => {
  it.each(NAMESPACES)("$name: no zh message says 部署前命令", ({ zh }) => {
    const variant = Object.entries(zh ?? {})
      .filter(([, entry]) => entry.message.includes("部署前命令"))
      .map(([key, entry]) => `${key}: ${entry.message}`);
    expect(variant).toEqual([]);
  });
});

// The zh glossary (w4/646): one rendering per product term. A zh message whose
// en message names the term must not use another rendering of it. Keep 密钥
// where the English is a cryptographic key or a generated secret value, and 计划
// where it is a schedule: the rules below match only the file feature, a
// billing plan, env groups, unlinking and the Key Value product.
const GLOSSARY: {
  term: string;
  en: RegExp;
  zh: string;
  variants: string[];
}[] = [
  {
    term: "Secret File",
    en: /secret[- ]files?/i,
    zh: "机密文件",
    variants: ["密钥文件"],
  },
  {
    // A billing or instance plan. A Blueprint's sync plan and an agent's
    // plan are other words (SKIP_PLAN).
    term: "Plan",
    en: /\bplans?\b/i,
    zh: "套餐",
    variants: ["计划", "方案"],
  },
  {
    term: "Env Group",
    en: /\b(env|environment) groups?\b/i,
    zh: "环境组",
    variants: ["环境变量组"],
  },
  {
    term: "Unlink",
    en: /\bunlink/i,
    zh: "取消关联",
    variants: ["取消链接"],
  },
  {
    // The product's name, kept in English as on most of its pages.
    term: "Key Value",
    en: /\bkey ?values?\b/i,
    zh: "Key Value",
    variants: ["键值存储"],
  },
];

const SKIP_PLAN = new Set([
  "blueprints",
  "agentSessions",
  "agentAutomations",
  "agent-sessions",
  "agent-automations",
]);

// Linking an env group is 关联, as Unlink is 取消关联 (w4/662). Scoped to the
// env-group messages: elsewhere "link" is a hyperlink (邀请链接).
describe("zh links an env group one way (w4/662)", () => {
  it.each(NAMESPACES)("$name", ({ name, en, zh }) => {
    const misses = Object.entries(zh ?? {})
      .filter(
        ([key]) => key.startsWith("services.envGroup") || name === "env-groups",
      )
      .filter(([key]) => /\blink(s|ed)?\b/i.test(en?.[key]?.message ?? ""))
      .filter(([, entry]) => entry.message.includes("链接"))
      .map(([key, entry]) => `${key}: ${entry.message}`);
    expect(misses).toEqual([]);
  });
});

describe("zh renders each glossary term one way (w4/646)", () => {
  it.each(NAMESPACES)("$name", ({ name, en, zh }) => {
    const misses: string[] = [];
    for (const [key, entry] of Object.entries(zh ?? {})) {
      const english = en?.[key]?.message ?? "";
      for (const rule of GLOSSARY) {
        if (!rule.en.test(english)) continue;
        if (rule.term === "Plan" && SKIP_PLAN.has(name)) continue;
        const variant = rule.variants.find((v) => entry.message.includes(v));
        if (variant) {
          misses.push(`${key} (${rule.term} → ${rule.zh}): ${entry.message}`);
        }
      }
    }
    expect(misses).toEqual([]);
  });

  it("forbids 密钥文件 in every zh message", () => {
    const variant = NAMESPACES.flatMap(({ name, zh }) =>
      Object.entries(zh ?? {})
        .filter(([, entry]) => entry.message.includes("密钥文件"))
        .map(([key]) => `${name}/${key}`),
    );
    expect(variant).toEqual([]);
  });
});

// Messages ko/ja/es/pt/ru/de deliberately keep in English: Render's product,
// service-type and feature names (bex.co keeps them English in these locales
// too), example commands, and IDs. zh has its own list (ZH_SAME_AS_EN), since
// zh does translate the service types.
const SAME_AS_EN = new Set<string>([
  ...ZH_SAME_AS_EN,
  // Render service types.
  "services.typeWeb",
  "services.typePrivate",
  "services.typeWorker",
  "services.typeCron",
  "services.typeStatic",
  "services.staticTitle",
  // Feature names.
  "deploys.triggerDeployHook",
  "services.deployHookTitle",
  "services.deployHookURLLabel",
  "services.eventsTriggerDeployHook",
  "services.shellWebTitle",
  "services.settingsHealthChecksTitle",
  "projects.filterKeyValues",
  "webhooks.idLabel",
  // US region names German writes exactly as English does (w4/693).
  "common.regionOhio",
  "common.regionOregon",
  "common.regionVirginia",
  // Example commands in input placeholders.
  "agentEnvironment.repoEnvs.setupRunPlaceholder",
  "agentEnvironment.repoEnvs.checkRunPlaceholder",
]);

describe("every other language is translated", () => {
  it.each(CATALOGS.filter(({ lang }) => lang !== "zh"))(
    "$name/$lang: no message is an untranslated English phrase",
    ({ en, catalog }) => {
      const copied = Object.entries(catalog)
        .filter(
          ([key, entry]) =>
            entry.message === englishSource(en, key)?.message &&
            englishPhrase(entry.message) &&
            !SAME_AS_EN.has(key),
        )
        .map(([key, entry]) => `${key}: ${entry.message}`);
      expect(copied).toEqual([]);
    },
  );
});
