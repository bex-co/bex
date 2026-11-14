import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

// w21/m395 t004: an accessible name is user-visible copy, so it goes through
// the catalogs like any other string (the Chinese UI read "Minimum instances"
// and "Copy code" to a screen reader). This finds a literal English name; the
// product's own name is the one exception.
const SRC = resolve(import.meta.dirname, "../..");
const BRAND_NAMES = new Set(["bex"]);

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) {
      return name === "__tests__" || name === "graphql" ? [] : sources(path);
    }
    return /\.tsx$/.test(name) && !/\.test\.tsx$/.test(name) ? [path] : [];
  });
}

const LITERAL =
  /aria-label=(?:"([^"{}]*[A-Za-z][^"{}]*)"|\{\s*["']([^"'{}]*[A-Za-z][^"'{}]*)["']\s*\})/g;

describe("accessible names", () => {
  it("are not English literals", () => {
    const found: string[] = [];
    for (const path of sources(SRC)) {
      const text = readFileSync(path, "utf8");
      for (const match of text.matchAll(LITERAL)) {
        const value = match[1] ?? match[2];
        if (BRAND_NAMES.has(value)) continue;
        const line = text.slice(0, match.index).split("\n").length;
        found.push(`${relative(SRC, path)}:${line}  aria-label="${value}"`);
      }
    }
    expect(
      found,
      `Move these to the catalogs (t("...")) in both languages:\n${found.join("\n")}`,
    ).toEqual([]);
  });

  it("finds the literals it is meant to find", () => {
    const sample = `<Input aria-label="Minimum instances" /><b aria-label={'Copy code'} /><i aria-label={t("x")} /><u aria-label="bex" />`;
    const values = [...sample.matchAll(LITERAL)].map((m) => m[1] ?? m[2]);
    expect(values).toEqual(["Minimum instances", "Copy code", "bex"]);
  });
});
