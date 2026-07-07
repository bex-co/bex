import {
  asSupportedLanguage,
  resolveUrlLanguage,
  DEFAULT_LANGUAGE,
  type SupportedLanguage,
} from "@/i18n/config";
import { getSearchParamOnClient } from "@/common/lib/search-params/client";
import { getCookie } from "@/common/hooks/use-cookie-storage-state/cookie";

/**
 * Detect the preferred language on the client, mirroring the server's
 * URL > cookie chain (no Accept-Language client-side, no localStorage here —
 * localStorage is applied post-hydration in useLanguageHydrationSync to
 * avoid a hydration mismatch against the SSR-rendered language).
 */
export function detectLanguageOnClient(): SupportedLanguage {
  const urlLang = resolveUrlLanguage(getSearchParamOnClient);
  if (urlLang) return urlLang;

  const cookieLang = asSupportedLanguage(getCookie("i18nextLng"));
  if (cookieLang) return cookieLang;

  return DEFAULT_LANGUAGE;
}
