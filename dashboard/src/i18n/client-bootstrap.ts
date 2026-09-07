import { getActiveI18n } from "@/i18n/request-scope";
import { CLIENT_BOOTSTRAP_GLOBAL, type ClientI18nBootstrap } from "@/i18n/init";
import {
  asSupportedLanguage,
  DEFAULT_LANGUAGE,
  type SupportedLanguage,
} from "@/i18n/config";

// JSON is almost script-safe; close the three holes that let embedded data
// break out of an inline <script>: a literal `</script>` (and `<!--`), and the
// U+2028/U+2029 line separators that are valid in JSON strings but terminate a
// script line.
function serializeForScript(value: unknown): string {
  return JSON.stringify(value)
    .replace(/</g, "\\u003c")
    .replace(/\u2028/g, "\\u2028")
    .replace(/\u2029/g, "\\u2029");
}

/**
 * The language the document shell renders this pass. The root context's
 * `language` is authoritative, but it is transiently absent at runtime despite
 * its non-optional type: during a client-side navigation the router
 * republishes the root match with its pre-`beforeLoad` context (base router
 * context only) while the root's session fetch is in flight, and a
 * `pendingMs: 0` detail route offers pending matches inside exactly that
 * window. Fall back to the active instance's current language — what the UI is
 * still rendering — so `<html lang>` stays stable and `getResourceBundle` is
 * never called with `undefined` (an i18next TypeError on every cold
 * detail-route navigation before this guard).
 */
export function resolveShellLanguage(
  contextLanguage: SupportedLanguage | undefined,
): SupportedLanguage {
  return (
    contextLanguage ??
    asSupportedLanguage(getActiveI18n().language) ??
    DEFAULT_LANGUAGE
  );
}

/**
 * The inline-script source that stamps this request's language + catalog onto
 * `globalThis` before the client bundle runs, so `i18n/init` can seed the
 * singleton and the first hydration render matches a non-default-language SSR
 * document (w6/m103). Returns `null` for the default language (its catalog is
 * already in the entry bundle) or if the catalog is somehow unregistered.
 */
export function i18nBootstrapScript(
  language: SupportedLanguage,
): string | null {
  if (language === DEFAULT_LANGUAGE) return null;
  const catalog = getActiveI18n().getResourceBundle(language, "translation") as
    | Record<string, string>
    | undefined;
  if (!catalog) return null;
  const payload: ClientI18nBootstrap = { lng: language, catalog };
  return `globalThis.${CLIENT_BOOTSTRAP_GLOBAL}=${serializeForScript(payload)}`;
}
