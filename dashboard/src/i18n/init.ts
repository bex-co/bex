import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import {
  DEFAULT_LANGUAGE,
  SUPPORTED_LANGUAGES,
  resources,
  loadLanguageResources,
} from "./index";
import type { SupportedLanguage } from "./config";

/**
 * No `LanguageDetector` plugin: it would read localStorage before hydration,
 * which can diverge from the server-rendered language and trigger a React
 * hydration mismatch. SSR/client language selection is threaded in
 * explicitly instead (t002).
 */
void i18n.use(initReactI18next).init({
  resources,
  lng: DEFAULT_LANGUAGE,
  fallbackLng: DEFAULT_LANGUAGE,
  supportedLngs: SUPPORTED_LANGUAGES,
  interpolation: {
    escapeValue: false,
    prefix: "{",
    suffix: "}",
  },
  react: {
    useSuspense: false,
  },
});

/**
 * Ensure `lang`'s catalog is registered before switching to it. The default
 * language ships in the entry bundle; any other is lazy-loaded once (its own
 * async chunk) and added via `addResourceBundle`. Callers must `await` this
 * before `changeLanguage(lang)` (w9/m60 t003) so the UI never renders raw keys
 * — including inside the root `beforeLoad`, which keeps SSR and hydration in
 * agreement for a non-default-language session.
 */
export async function ensureLanguage(lang: string): Promise<void> {
  if (lang === DEFAULT_LANGUAGE || i18n.hasResourceBundle(lang, "translation")) {
    return;
  }
  const messages = await loadLanguageResources(lang as SupportedLanguage);
  i18n.addResourceBundle(lang, "translation", messages, true, true);
}

export default i18n;
