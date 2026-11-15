/**
 * The bex.co locale set (`eden-cms-v2`'s `docusaurus.config.ts` `i18n.locales`),
 * in the same order, so the marketing site and the dashboard offer the same
 * languages. Adding one: see "Adding a language" in dashboard/AGENTS.md.
 */
export const SUPPORTED_LANGUAGES = [
  "en",
  "zh",
  "ko",
  "ja",
  "es",
  "pt",
  "ru",
  "de",
] as const;

export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

export const DEFAULT_LANGUAGE: SupportedLanguage = "en";

/** Each language's name in its own script — a menu you cannot read is useless. */
export const LANGUAGE_NAMES: Record<SupportedLanguage, string> = {
  en: "English",
  zh: "简体中文",
  ko: "한국어",
  ja: "日本語",
  es: "Español",
  pt: "Português",
  ru: "Русский",
  de: "Deutsch",
};

/**
 * The BCP 47 tag each language formats dates and numbers with (Intl,
 * date-fns): `pt` is Brazilian Portuguese, matching its CLDR plural rules
 * (0 and 1 are both singular) and the bex.co `pt` copy.
 */
export const LANGUAGE_TAGS: Record<SupportedLanguage, string> = {
  en: "en-US",
  zh: "zh-CN",
  ko: "ko-KR",
  ja: "ja-JP",
  es: "es-ES",
  pt: "pt-BR",
  ru: "ru-RU",
  de: "de-DE",
};

/** Narrows an arbitrary string (URL param, cookie, header) to a supported language, or null. */
export function asSupportedLanguage(
  lang: string | null | undefined,
): SupportedLanguage | null {
  return lang && (SUPPORTED_LANGUAGES as readonly string[]).includes(lang)
    ? (lang as SupportedLanguage)
    : null;
}

/** `?lang=`/`?locale=` URL override, shared by every environment's detector (server, client, post-hydration sync). */
export function resolveUrlLanguage(
  getParam: (key: string) => string | null,
): SupportedLanguage | null {
  return (
    asSupportedLanguage(getParam("lang")) ??
    asSupportedLanguage(getParam("locale"))
  );
}

export interface TranslationEntry {
  message: string;
  description: string;
}
