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
 * URL > cookie chain, then falling back to the SSR-stamped <html lang>
 * before the default. The client cannot read Accept-Language, so without
 * that fallback a first-time visitor whose browser negotiated a non-default
 * language server-side would hydrate an English render over a translated
 * document — a whole-tree React #418. (No localStorage here — that is
 * applied post-hydration in useLanguageHydrationSync for the same reason.)
 */
export function detectLanguageOnClient(): SupportedLanguage {
  const urlLang = resolveUrlLanguage(getSearchParamOnClient);
  if (urlLang) return urlLang;

  const cookieLang = asSupportedLanguage(getCookie("i18nextLng"));
  if (cookieLang) return cookieLang;

  const documentLang = asSupportedLanguage(document.documentElement.lang);
  if (documentLang) return documentLang;

  return DEFAULT_LANGUAGE;
}
