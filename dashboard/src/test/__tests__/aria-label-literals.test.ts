import { describe, expect, it } from "vitest";

// An accessible name is user-visible copy, so it goes through the catalogs like
// any other string. This finds a literal English name; the product's own name
// is the one exception.
const BRAND_NAMES = new Set(["bex"]);

const sources = import.meta.glob<string>(
  ["/src/**/*.tsx", "!/src/**/__tests__/**"],
  { query: "?raw", import: "default", eager: true },
);

const LITERAL =
  /aria-label=(?:"([^"{}]*[A-Za-z][^"{}]*)"|\{\s*["']([^"'{}]*[A-Za-z][^"'{}]*)["']\s*\})/g;

describe("accessible names", () => {
  it("are not English literals", () => {
    const found: string[] = [];
    for (const [path, text] of Object.entries(sources)) {
      for (const match of text.matchAll(LITERAL)) {
        const value = match[1] ?? match[2];
        if (BRAND_NAMES.has(value)) continue;
        const line = text.slice(0, match.index).split("\n").length;
        found.push(`${path}:${line}  aria-label="${value}"`);
      }
    }
    expect(
      found,
      `Move these to the catalogs (t("...")) in both languages:\n${found.join("\n")}`,
    ).toEqual([]);
  });

  it("reads the sources it is meant to read", () => {
    expect(Object.keys(sources).length).toBeGreaterThan(200);
  });

  it("finds the literals it is meant to find", () => {
    const sample = `<Input aria-label="Minimum instances" /><b aria-label={'Copy code'} /><i aria-label={t("x")} /><u aria-label="bex" />`;
    const values = [...sample.matchAll(LITERAL)].map((m) => m[1] ?? m[2]);
    expect(values).toEqual(["Minimum instances", "Copy code", "bex"]);
  });
});
