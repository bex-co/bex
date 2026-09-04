import type { i18n as I18nInstance } from "i18next";
import i18n from "@/i18n/init";

/**
 * On the client there is exactly one session, so the shared singleton is the
 * active instance — the switcher and `useLanguageHydrationSync` mutate it, and
 * `readClientBootstrap` (in `i18n/init`) has already seeded it with the SSR
 * language + catalog before the first render. Client-only: no node:async_hooks.
 */
export function getActiveI18nOnClient(): I18nInstance {
  return i18n;
}
