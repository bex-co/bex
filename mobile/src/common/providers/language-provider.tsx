import { createContext, useMemo, useState, type ReactNode } from "react";
import { useLocales } from "expo-localization";
import { I18n } from "i18n-js";
import { en } from "@/translations/en";
import { zh } from "@/translations/zh";
import { resolveSupportedLanguage, type SupportedLanguage } from "./language";

export type { SupportedLanguage } from "./language";
type LanguageContextValue = {
  language: SupportedLanguage;
  setLanguage: (language: SupportedLanguage) => void;
  t: (key: string, options?: Record<string, unknown>) => string;
};

export const LanguageContext = createContext<LanguageContextValue>({
  language: "en" as SupportedLanguage,
  setLanguage: () => {},
  t: (key: string) => key,
});

export function LanguageProvider({ children }: { children: ReactNode }) {
  const locales = useLocales();
  const systemLanguage = resolveSupportedLanguage(locales[0]?.languageCode);
  const [languageOverride, setLanguage] = useState<SupportedLanguage | null>(
    null,
  );
  const language = languageOverride ?? systemLanguage;
  const value = useMemo(() => {
    const i18n = new I18n({ en, zh });
    i18n.enableFallback = true;
    i18n.defaultLocale = "en";
    i18n.locale = language;
    return {
      language,
      setLanguage,
      t: (key: string, options?: Record<string, unknown>) =>
        i18n.t(key, options),
    };
  }, [language]);
  return (
    <LanguageContext.Provider value={value}>
      {children}
    </LanguageContext.Provider>
  );
}
