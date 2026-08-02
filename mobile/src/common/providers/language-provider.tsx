import { createContext, useMemo, useState, type ReactNode } from "react";
import { I18n } from "i18n-js";
import { en } from "@/translations/en";
import { zh } from "@/translations/zh";

export type SupportedLanguage = "en" | "zh";
type LanguageContextValue = {
  language: SupportedLanguage;
  setLanguage: (language: SupportedLanguage) => void;
  t: (key: string) => string;
};

export const LanguageContext = createContext<LanguageContextValue>({
  language: "en" as SupportedLanguage,
  setLanguage: () => {},
  t: (key: string) => key,
});

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [language, setLanguage] = useState<SupportedLanguage>("en");
  const value = useMemo(() => {
    const i18n = new I18n({ en, zh });
    i18n.enableFallback = true;
    i18n.defaultLocale = "en";
    i18n.locale = language;
    return { language, setLanguage, t: (key: string) => i18n.t(key) };
  }, [language]);
  return (
    <LanguageContext.Provider value={value}>
      {children}
    </LanguageContext.Provider>
  );
}
