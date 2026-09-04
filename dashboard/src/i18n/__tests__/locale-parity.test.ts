import { describe, it, expect } from "vitest";

// en <-> zh locale key parity, AUTO-DISCOVERED (w5/033). The i18n tooling only
// console.warns in dev on a missing key (use-translations.ts); nothing failed
// CI when zh lagged en as features add strings — and this test used to cover
// only 4 hardcoded namespaces (common/auth/metrics/services), leaving the other
// ~20 features to drift silently. import.meta.glob now discovers every
// per-feature + common locale pair, so a newly added feature's pair is checked
// without editing this test, and any drift fails CI (yarn test gates deploy.yml).
//
// Path is relative to this file (src/i18n/__tests__/): ../../ == src/.
type Entry = { message: string; description: string };
const modules = import.meta.glob<{ default: Record<string, Entry> }>(
  "../../**/locales/{en,zh}.ts",
  { eager: true },
);

// Group by locale directory (e.g. "features/metrics") -> { en, zh } modules.
type Pair = {
  name: string;
  en?: Record<string, Entry>;
  zh?: Record<string, Entry>;
};
const byDir: Record<string, Pair> = {};
for (const [path, mod] of Object.entries(modules)) {
  const m = path.match(/\/([^/]+)\/locales\/(en|zh)\.ts$/);
  if (!m) continue;
  const [, name, lang] = m;
  (byDir[name] ??= { name })[lang as "en" | "zh"] = mod.default;
}
const NAMESPACES = Object.values(byDir).sort((a, b) =>
  a.name.localeCompare(b.name),
);

// Intentional gaps: keys allowed in one language but not the other. Keep empty
// — add a key here (with a why) only for a deliberate divergence.
const ALLOW_ONLY_EN = new Set<string>([]);
const ALLOW_ONLY_ZH = new Set<string>([]);

describe("locale key parity", () => {
  it("discovers every locale pair (glob is not silently empty)", () => {
    // A broken glob would generate zero it.each cases and vacuously "pass"; this
    // guards that the discovery actually found the full set.
    expect(NAMESPACES.length).toBeGreaterThanOrEqual(20);
    for (const ns of NAMESPACES) {
      expect(ns.en, `${ns.name} is missing its en locale`).toBeDefined();
      expect(ns.zh, `${ns.name} is missing its zh locale`).toBeDefined();
    }
  });

  it.each(NAMESPACES)("$name: en and zh have the same keys", ({ en, zh }) => {
    const enKeys = Object.keys(en ?? {});
    const zhKeys = Object.keys(zh ?? {});
    // Native plurals (w6/062): en authors `key_one` + `key_other`; zh has only
    // one plural category, so its catalog carries only `key_other`. An en-only
    // `key_one` is therefore expected parity, not drift, as long as both
    // languages have the `key_other` it pairs with.
    const isEnPluralOne = (k: string) => {
      if (!k.endsWith("_one")) return false;
      const other = `${k.slice(0, -"_one".length)}_other`;
      return Boolean(en?.[other] && zh?.[other]);
    };
    const onlyEn = enKeys
      .filter((k) => !zh?.[k] && !ALLOW_ONLY_EN.has(k) && !isEnPluralOne(k))
      .sort();
    const onlyZh = zhKeys
      .filter((k) => !en?.[k] && !ALLOW_ONLY_ZH.has(k))
      .sort();
    expect(onlyEn, "keys in en but missing from zh").toEqual([]);
    expect(onlyZh, "keys in zh but missing from en").toEqual([]);
  });

  it.each(NAMESPACES)("$name: plural keys are well-formed", ({ en, zh }) => {
    // en: a `_one` form must have its `_other` sibling (English needs both).
    for (const k of Object.keys(en ?? {})) {
      if (!k.endsWith("_one")) continue;
      const other = `${k.slice(0, -"_one".length)}_other`;
      expect(
        en?.[other],
        `en has "${k}" but not its plural sibling "${other}"`,
      ).toBeDefined();
    }
    // zh: no `_one` forms — Chinese's only cardinal plural category is
    // "other", so a zh `_one` entry is unreachable (do not mechanically
    // double zh when adding a pluralized string).
    const zhOnes = Object.keys(zh ?? {})
      .filter((k) => k.endsWith("_one"))
      .sort();
    expect(zhOnes, "zh must carry only the _other plural form").toEqual([]);
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
    "$name: every entry has a non-empty message",
    ({ en, zh }) => {
      for (const entry of [
        ...Object.values(en ?? {}),
        ...Object.values(zh ?? {}),
      ]) {
        expect(entry.message.length).toBeGreaterThan(0);
      }
    },
  );
});
