import i18n from "i18next";
import type { i18n as I18nInstance, InitOptions } from "i18next";
import { initReactI18next } from "react-i18next";
import {
  DEFAULT_LANGUAGE,
  SUPPORTED_LANGUAGES,
  resources,
  loadLanguageResources,
} from "./index";
import type { SupportedLanguage } from "./config";

/**
 * No `LanguageDetector` plugin: it would read localStorage before hydration,
 * which can diverge from the server-rendered language and trigger a React
 * hydration mismatch. SSR/client language selection is threaded in
 * explicitly instead: on the server each request renders through its own
 * instance (`createRequestI18nInstance`, scoped in `i18n/request-scope`) so one
 * request's language can never be clobbered by a concurrent one; on the client
 * the SSR HTML inlines the active catalog (`readClientBootstrap`) so the first
 * hydration render matches the document (w6/m103).
 */
const BASE_INIT_OPTIONS: InitOptions = {
  fallbackLng: DEFAULT_LANGUAGE,
  supportedLngs: SUPPORTED_LANGUAGES as unknown as string[],
  interpolation: {
    escapeValue: false,
    prefix: "{",
    suffix: "}",
  },
  react: {
    useSuspense: false,
  },
};

/** What the SSR HTML inlines (shell-component) for a non-default session. */
export interface ClientI18nBootstrap {
  lng: SupportedLanguage;
  catalog: Record<string, string>;
}

export const CLIENT_BOOTSTRAP_GLOBAL = "__BEX_I18N__";

/**
 * The active language + its catalog the server stamped into the initial HTML.
 * Only ever set by the browser-executed inline script (never on the server, so
 * it stays request-local — the server renders through per-request instances,
 * not this global). `null` for a default-language session (its catalog already
 * ships in the entry bundle) and always `null` during SSR.
 */
function readClientBootstrap(): ClientI18nBootstrap | null {
  const holder = globalThis as {
    [CLIENT_BOOTSTRAP_GLOBAL]?: ClientI18nBootstrap;
  };
  const boot = holder[CLIENT_BOOTSTRAP_GLOBAL];
  return boot && boot.lng !== DEFAULT_LANGUAGE ? boot : null;
}

// Seed the singleton's first render from the SSR-inlined catalog when present,
// so a non-default-language client hydrates in that language synchronously — the
// non-default catalog is otherwise a lazy `import()` the client can't resolve
// before the first paint, which would render the English fallback over a
// translated document (a whole-tree React #418). A no-op on the server and for a
// default-language session.
const bootstrap = readClientBootstrap();
const initialResources = bootstrap
  ? { ...resources, [bootstrap.lng]: { translation: bootstrap.catalog } }
  : resources;

void i18n.use(initReactI18next).init({
  ...BASE_INIT_OPTIONS,
  resources: initialResources,
  lng: bootstrap?.lng ?? DEFAULT_LANGUAGE,
});

/**
 * A fresh, fully independent i18next instance for one SSR request. Initialized
 * synchronously with the default catalog only; a non-default request registers
 * its catalog via `ensureLanguageOn` in the root `beforeLoad` before switching.
 * Because it is per-request (held in `i18n/request-scope`), a concurrent
 * request's `changeLanguage` can never change the language this request renders.
 */
export function createRequestI18nInstance(): I18nInstance {
  const instance = i18n.createInstance();
  void instance.use(initReactI18next).init({
    ...BASE_INIT_OPTIONS,
    // A fresh top-level object per instance so `addResourceBundle` writes only
    // to this instance's store, never a shared one.
    resources: { ...resources },
    lng: DEFAULT_LANGUAGE,
  });
  return instance;
}

/**
 * Ensure `lang`'s catalog is registered on `instance` before switching to it.
 * The default language ships in the entry bundle; any other is lazy-loaded once
 * (its own async chunk) and added via `addResourceBundle`. Callers must `await`
 * this before `changeLanguage(lang)` (w9/m60 t003) so the UI never renders raw
 * keys — including inside the root `beforeLoad`, which keeps SSR and hydration
 * in agreement for a non-default-language session.
 */
export async function ensureLanguageOn(
  instance: I18nInstance,
  lang: string,
): Promise<void> {
  if (
    lang === DEFAULT_LANGUAGE ||
    instance.hasResourceBundle(lang, "translation")
  ) {
    return;
  }
  const messages = await loadLanguageResources(lang as SupportedLanguage);
  instance.addResourceBundle(lang, "translation", messages, true, true);
}

/** `ensureLanguageOn` bound to the shared client/singleton instance. */
export async function ensureLanguage(lang: string): Promise<void> {
  return ensureLanguageOn(i18n, lang);
}

/**
 * Switch the shared client instance's language the one correct way: register
 * the (possibly lazy) catalog first, then change. Every client-side switch
 * point — the account-menu switcher, the pre-login `LanguageSwitcher`, and the
 * post-hydration sync — must go through this so none can reintroduce Bug A
 * (`changeLanguage` before the catalog is loaded → English fallback with
 * `i18n.language` already moved, w6/m103). Not for the server render path, which
 * uses per-request instances via `ensureLanguageOn` in the root `beforeLoad`.
 */
export async function switchLanguage(lang: string): Promise<void> {
  await ensureLanguage(lang);
  await i18n.changeLanguage(lang);
}

export default i18n;
