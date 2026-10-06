import { describe, it, expect } from "vitest";
import { en } from "@/i18n";

// Every `t("…")` call site must name a key that actually exists (w1/m86).
//
// The failure this catches is silent and user-visible: i18next returns the KEY
// when it has no message for it, so a typo'd or never-added key renders the
// literal string "common.cancel" on a button. Nothing else stops it —
// `useTranslations` types the resource bundle as Record<string, string>, so
// there is no compile error, and the dev-only console.warn is invisible in CI.
// locale-parity.test.ts checks en against zh, which agree perfectly when a key
// is missing from BOTH.
//
// Found by the w1/m86 parity audit: three Cancel buttons on the Disk tab
// rendered "common.cancel" in every language, because that key had never been
// added to src/common/locales.
const modules = import.meta.glob("../../**/*.{ts,tsx}", {
  eager: true,
  query: "?raw",
  import: "default",
});

// `t("some.key"` — the literal form. A computed key (a variable, a template
// literal, a conditional) can't be checked statically and is skipped rather
// than guessed at.
const CALL = /\bt\(\s*"([a-zA-Z0-9_]+\.[a-zA-Z0-9_.]+)"/g;

// Any quoted "<namespace>.<key>": a label table holds keys for a later
// t(table[id]) call, which CALL cannot see (w5/m125). Its `keyof typeof en`
// type proves nothing, since `en` is a Record<string, string>.
const LITERAL = /"([a-zA-Z0-9_]+\.[a-zA-Z0-9_.]+)"/g;

const NAMESPACES = new Set(Object.keys(en).map((key) => key.split(".")[0]));

type Site = { key: string; file: string };

function sites(pattern: RegExp): Site[] {
  const found: Site[] = [];
  for (const [path, source] of Object.entries(modules)) {
    if (typeof source !== "string") continue;
    // Locale files define keys; test files may assert on absent ones; and the
    // i18n hook itself DOCUMENTS the convention in an error message
    // (`Use t("namespace.keyName")`) rather than calling it.
    if (
      path.includes("/locales/") ||
      path.includes("__tests__") ||
      // Vite normalizes a same-directory glob match to "./<file>", which the
      // "__tests__" substring check above misses — but everything sharing this
      // directory IS a __tests__ file.
      path.startsWith("./") ||
      path.endsWith("use-translations.ts")
    )
      continue;
    for (const match of source.matchAll(pattern)) {
      found.push({ key: match[1], file: path.replace("../../", "src/") });
    }
  }
  return found;
}

function callSites(): Site[] {
  return sites(CALL);
}

/** Namespaced literals, less file names such as "auth.login.tsx". */
function literalSites(): Site[] {
  return sites(LITERAL).filter(
    ({ key }) =>
      NAMESPACES.has(key.split(".")[0]) && !/\.(tsx?|json)$/.test(key),
  );
}

/** "key  ←  files" for every site whose key has no message. */
function missingReport(found: Site[]): string[] {
  const missing = new Map<string, Set<string>>();
  for (const { key, file } of found) {
    if (key in en) continue;
    // Native-plural base key (w6/062): `t("ns.key", { count })` resolves via
    // the `_one`/`_other` suffixed entries; the base key itself is never in
    // the catalog. Require BOTH forms — English uses both, so authoring only
    // half the pair is a bug this test should still catch.
    if (`${key}_one` in en && `${key}_other` in en) continue;
    const files = missing.get(key) ?? new Set<string>();
    files.add(file);
    missing.set(key, files);
  }
  // Name the key AND where it is used: the fix is either adding the key or
  // correcting the call site, and which one depends on the file.
  return [...missing.entries()]
    .map(([key, files]) => `  ${key}  ←  ${[...files].sort().join(", ")}`)
    .sort();
}

describe("translation keys exist", () => {
  it("discovers call sites (the glob is not silently empty)", () => {
    // A regex or glob that quietly stops matching would turn this whole file
    // into a no-op that still reports green.
    expect(callSites().length).toBeGreaterThan(200);
    expect(literalSites().length).toBeGreaterThan(callSites().length);
    expect(Object.keys(en).length).toBeGreaterThan(200);
  });

  it("resolves every statically-written t() key to a message", () => {
    const report = missingReport(callSites());
    expect(
      report,
      `translation keys with no message:\n${report.join("\n")}`,
    ).toEqual([]);
  });

  it("resolves every key a label table names to a message", () => {
    const report = missingReport(literalSites());
    expect(
      report,
      `label-table keys with no message:\n${report.join("\n")}`,
    ).toEqual([]);
  });
});
