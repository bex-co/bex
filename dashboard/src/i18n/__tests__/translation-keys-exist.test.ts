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

type Site = { key: string; file: string };

function callSites(): Site[] {
  const sites: Site[] = [];
  for (const [path, source] of Object.entries(modules)) {
    if (typeof source !== "string") continue;
    // Locale files define keys; test files may assert on absent ones; and the
    // i18n hook itself DOCUMENTS the convention in an error message
    // (`Use t("namespace.keyName")`) rather than calling it.
    if (
      path.includes("/locales/") ||
      path.includes("__tests__") ||
      path.endsWith("use-translations.ts")
    )
      continue;
    for (const match of source.matchAll(CALL)) {
      sites.push({ key: match[1], file: path.replace("../../", "src/") });
    }
  }
  return sites;
}

describe("translation keys exist", () => {
  it("discovers call sites (the glob is not silently empty)", () => {
    // A regex or glob that quietly stops matching would turn this whole file
    // into a no-op that still reports green.
    expect(callSites().length).toBeGreaterThan(200);
    expect(Object.keys(en).length).toBeGreaterThan(200);
  });

  it("resolves every statically-written t() key to a message", () => {
    const missing = new Map<string, Set<string>>();
    for (const { key, file } of callSites()) {
      if (key in en) continue;
      const files = missing.get(key) ?? new Set<string>();
      files.add(file);
      missing.set(key, files);
    }

    // Name the key AND where it is used: the fix is either adding the key or
    // correcting the call site, and which one depends on the file.
    const report = [...missing.entries()]
      .map(([key, files]) => `  ${key}  ←  ${[...files].sort().join(", ")}`)
      .sort();
    expect(report, `translation keys with no message:\n${report.join("\n")}`).toEqual([]);
  });
});
