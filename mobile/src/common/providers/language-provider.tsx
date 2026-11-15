import {
  createContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { getLocales } from "expo-localization";
import { I18n } from "i18n-js";
import { en } from "@/translations/en";
import { zh } from "@/translations/zh";
import { ko } from "@/translations/ko";
import { ja } from "@/translations/ja";
import { es } from "@/translations/es";
import { pt } from "@/translations/pt";
import { ru } from "@/translations/ru";
import { de } from "@/translations/de";
import {
  loadLanguage,
  resolveDeviceLanguage,
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

const CATALOGS = { en, zh, ko, ja, es, pt, ru, de } satisfies Record<
  SupportedLanguage,
  unknown
>;

// The device's own language when the app speaks it — read synchronously so the
// first frame is already in it — else English. An explicit choice in Settings
// (restored below) always wins over the device.
function deviceLanguage(): SupportedLanguage {
  try {
    return resolveDeviceLanguage(getLocales()) ?? "en";
  } catch {
    return "en";
  }
}

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] =
    useState<SupportedLanguage>(deviceLanguage);

  // Restore the saved language once on mount; unset/corrupt storage keeps the
  // device language.
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
    const i18n = new I18n(CATALOGS);
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
