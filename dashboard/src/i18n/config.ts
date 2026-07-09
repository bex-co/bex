export const SUPPORTED_LANGUAGES = ["en", "zh"] as const;

export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

export const DEFAULT_LANGUAGE: SupportedLanguage = "en";

export const LANGUAGE_NAMES: Record<SupportedLanguage, string> = {
  en: "English",
  zh: "中文",
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
