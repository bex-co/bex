import { getActiveI18n } from "@/i18n/request-scope";
import { CLIENT_BOOTSTRAP_GLOBAL, type ClientI18nBootstrap } from "@/i18n/init";
import { DEFAULT_LANGUAGE, type SupportedLanguage } from "@/i18n/config";

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
