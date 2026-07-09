import { getRequestHeader } from "@tanstack/react-start/server";
import {
  asSupportedLanguage,
  resolveUrlLanguage,
  DEFAULT_LANGUAGE,
  type SupportedLanguage,
} from "@/i18n/config";
import { getSearchParamOnServer } from "@/common/lib/search-params/server";
import { getCookie } from "@/common/hooks/use-cookie-storage-state/cookie";

function matchAcceptLanguage(
  header: string | undefined,
): SupportedLanguage | null {
  if (!header) return null;
  for (const part of header.split(",")) {
    const tag = part.split(";")[0]?.trim().split("-")[0];
    const matched = asSupportedLanguage(tag);
    if (matched) return matched;
  }
  return null;
}

/**
 * Detect the preferred language on the server.
 * Priority: URL (?lang=/?locale=) > cookie (i18nextLng) > Accept-Language > default.
 * Server-only: do not import this file on the client.
 */
export function detectLanguageOnServer(): SupportedLanguage {
  const urlLang = resolveUrlLanguage(getSearchParamOnServer);
  if (urlLang) return urlLang;

  const cookieLang = asSupportedLanguage(getCookie("i18nextLng"));
  if (cookieLang) return cookieLang;

  const acceptLanguageLang = matchAcceptLanguage(
    getRequestHeader("accept-language"),
  );
  if (acceptLanguageLang) return acceptLanguageLang;

  return DEFAULT_LANGUAGE;
}
