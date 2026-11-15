import { SUPPORTED_LANGUAGES } from "../../common/preferences/preferences";
import { en } from "../en";
import { zh } from "../zh";
import { ko } from "../ko";
import { ja } from "../ja";
import { es } from "../es";
import { pt } from "../pt";
import { ru } from "../ru";
import { de } from "../de";

const CATALOGS: Record<string, unknown> = { en, zh, ko, ja, es, pt, ru, de };

// Every leaf as [dotted key, message].
function leaves(value: unknown, prefix = ""): [string, unknown][] {
  if (!value || typeof value !== "object") return [[prefix, value]];
  return Object.entries(value).flatMap(([key, child]) =>
    leaves(child, prefix ? `${prefix}.${key}` : key),
  );
}

const placeholders = (message: string) =>
  [...new Set(message.match(/%\{[A-Za-z0-9_]+\}/g) ?? [])].sort().join();

const english = new Map(leaves(en) as [string, string][]);

describe("translations", () => {
  it("has a catalog for every supported language", () => {
    expect(Object.keys(CATALOGS).sort()).toEqual(
      [...SUPPORTED_LANGUAGES].sort(),
    );
  });

  for (const language of SUPPORTED_LANGUAGES) {
    const catalog = CATALOGS[language];

    it(`keeps ${language} key-complete with English`, () => {
      expect(
        leaves(catalog)
          .map(([key]) => key)
          .sort(),
      ).toEqual([...english.keys()].sort());
    });

    it(`keeps every ${language} message non-empty with English's placeholders`, () => {
      const broken = leaves(catalog)
        .filter(
          ([key, message]) =>
            typeof message !== "string" ||
            message.trim() === "" ||
            placeholders(message) !== placeholders(english.get(key) ?? ""),
        )
        .map(([key]) => key);
      expect(broken).toEqual([]);
    });

    // i18n-js interpolates only %{name} (and {{name}}); a bare {name} renders
    // literally — "Updated {time}" shipped that way on session detail.
    it(`uses only %{name} placeholders in ${language}`, () => {
      const bare = leaves(catalog)
        .filter(
          ([, message]) =>
            typeof message === "string" &&
            /(^|[^%{])\{[A-Za-z0-9_]+\}/.test(message),
        )
        .map(([key]) => key);
      expect(bare).toEqual([]);
    });
  }
});
