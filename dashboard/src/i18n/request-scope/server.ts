import { AsyncLocalStorage } from "node:async_hooks";
import type { i18n as I18nInstance } from "i18next";
import i18n from "@/i18n/init";

/**
 * Per-request i18next instance for SSR. `createStart`'s request middleware
 * (`src/start.ts`) runs each request's whole render — `beforeLoad`, loaders,
 * `head()` resolvers, and the React render — inside `runWithRequestI18n`, so
 * `getActiveI18nOnServer()` returns that request's own instance throughout.
 * One request's `changeLanguage` therefore can never leak into a concurrent
 * request's rendered HTML (w6/m103 Bug B). Server-only: imports node:async_hooks.
 */
const store = new AsyncLocalStorage<I18nInstance>();

export function runWithRequestI18n<T>(instance: I18nInstance, fn: () => T): T {
  return store.run(instance, fn);
}

/**
 * The current request's instance, or the shared singleton when called outside a
 * request scope (e.g. a standalone server function). The singleton fallback is
 * never used to render a document — those always run inside the middleware.
 */
export function getActiveI18nOnServer(): I18nInstance {
  return store.getStore() ?? i18n;
}
