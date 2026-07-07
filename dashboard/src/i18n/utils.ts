import type { SupportedLanguage } from "./config";
import { setCookie } from "@/common/hooks/use-cookie-storage-state/cookie";

const COOKIE_NAME = "i18nextLng";

/**
 * Persist a language choice to both cookie (read by the server on the next
 * request, `detectLanguageOnServer`) and localStorage (read post-hydration,
 * `useLanguageHydrationSync`). Client-only.
 */
export function persistLanguage(lang: SupportedLanguage): void {
  setCookie(COOKIE_NAME, lang, { expires: 365, sameSite: "lax", path: "/" });
  localStorage.setItem(COOKIE_NAME, lang);
}
