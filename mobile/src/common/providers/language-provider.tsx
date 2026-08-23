import {
  createContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { I18n } from "i18n-js";
import { en } from "@/translations/en";
import { zh } from "@/translations/zh";
import {
  loadLanguage,
  saveLanguage,
  type SupportedLanguage,
} from "@/common/preferences/preferences";

export type { SupportedLanguage };

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
  const [language, setLanguageState] = useState<SupportedLanguage>("en");

  // Restore the saved language once on mount; unset/corrupt storage keeps "en".
  useEffect(() => {
    let active = true;
    void loadLanguage().then((stored) => {
      if (active && stored) setLanguageState(stored);
    });
    return () => {
      active = false;
    };
  }, []);

  const value = useMemo(() => {
    const i18n = new I18n({ en, zh });
    i18n.enableFallback = true;
    i18n.defaultLocale = "en";
    i18n.locale = language;
    const setLanguage = (next: SupportedLanguage) => {
      setLanguageState(next);
      void saveLanguage(next);
    };
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
