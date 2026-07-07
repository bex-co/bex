import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import { DEFAULT_LANGUAGE, SUPPORTED_LANGUAGES, resources } from "./index";

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

export default i18n;
