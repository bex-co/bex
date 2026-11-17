import { describe, expect, it } from "vitest";
import { OryLocales } from "@ory/elements-react";
import { SUPPORTED_LANGUAGES } from "@/i18n/config";
import { loadLanguageResources } from "@/i18n";
import { ORY_SETTINGS_COPY_SOURCE } from "../keys";

/**
 * The settings flow's loading frame reserves Ory's own words (w3/067), so each
 * language's mirror must be exactly what Ory renders: a reworded card in an
 * `@ory/elements-react` upgrade would otherwise leave the frame wrapping as the
 * old copy did.
 */
const mirrors = import.meta.glob<{ default: Record<string, string> }>(
  "../*.ts",
  { eager: true },
);

function mirror(lang: string): Record<string, string> | undefined {
  return mirrors[`../${lang}.ts`]?.default;
}

const KEYS = Object.keys(ORY_SETTINGS_COPY_SOURCE).sort();

describe("Ory settings copy mirrors (w3/067)", () => {
  it.each(SUPPORTED_LANGUAGES)("%s copies Ory's catalog verbatim", (lang) => {
    const copy = mirror(lang);
    expect(copy, `missing ory-settings-copy/${lang}.ts`).toBeDefined();
    expect(Object.keys(copy!).sort()).toEqual(KEYS);
    const ory = OryLocales[lang] as Record<string, string>;
    for (const [key, source] of Object.entries(ORY_SETTINGS_COPY_SOURCE)) {
      expect(copy![key], `${lang} ${key} (Ory's ${source})`).toBe(ory[source]);
    }
  });

  it.each(SUPPORTED_LANGUAGES)(
    "%s's catalog carries its mirror",
    async (lang) => {
      const catalog = await loadLanguageResources(lang);
      expect(catalog).toMatchObject(mirror(lang)!);
    },
  );
});
