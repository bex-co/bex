import { useEffect } from "react";
import i18n from "./init";
import { persistLanguage } from "./utils";
import { asSupportedLanguage, resolveUrlLanguage } from "./config";
import { getSearchParamOnClient } from "@/common/lib/search-params/client";

/**
 * Runs once after hydration, never during the initial render, so it can't
 * cause a mismatch. The initial render already matches the server's
 * detected language (URL > cookie > Accept-Language); this reconciles two
 * cases detection alone can't:
 *  - `?lang=`/`?locale=` was just used to pick a language — persist it so it
 *    survives past this page view.
 *  - localStorage holds a different preference than the cookie (e.g. the
 *    cookie expired or was cleared) — apply and re-persist it.
 */
export function useLanguageHydrationSync(): void {
  useEffect(() => {
    const urlLang = resolveUrlLanguage(getSearchParamOnClient);
    if (urlLang) {
      persistLanguage(urlLang);
      return;
    }

    const storedLang = asSupportedLanguage(localStorage.getItem("i18nextLng"));
    if (storedLang && storedLang !== i18n.language) {
      void i18n.changeLanguage(storedLang);
      persistLanguage(storedLang);
    }
  }, []);
}
