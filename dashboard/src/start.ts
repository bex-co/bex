import { createStart, createMiddleware } from "@tanstack/react-start";

/**
 * Give every SSR request its own i18next instance so a concurrent request's
 * language can never bleed into this one's rendered HTML (w6/m103 Bug B). The
 * request middleware wraps the entire render — `beforeLoad`, loaders, `head()`
 * title/metadata resolvers, and the React render — in an AsyncLocalStorage
 * scope; `getActiveI18n()` (i18n/request-scope) returns that scoped instance
 * everywhere on the server. The instance factory and the ALS live behind a
 * dynamic `import()` inside the server handler so neither i18next nor
 * node:async_hooks is pulled into the client bundle.
 */
const i18nRequestMiddleware = createMiddleware({ type: "request" }).server(
  async ({ next }) => {
    const [{ createRequestI18nInstance }, { runWithRequestI18n }] =
      await Promise.all([
        import("@/i18n/init"),
        import("@/i18n/request-scope/server"),
      ]);
    return runWithRequestI18n(createRequestI18nInstance(), () => next());
  },
);

export const startInstance = createStart(() => ({
  requestMiddleware: [i18nRequestMiddleware],
}));
