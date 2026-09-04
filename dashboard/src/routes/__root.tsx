import { createRootRouteWithContext } from "@tanstack/react-router";
import type { RouterContext } from "@/router";
import NotFoundPage from "@/common/root-route/not-found-page";
import ErrorPage from "@/common/root-route/error-page";
import { ShellComponent } from "@/common/root-route/shell-component";
import { RootComponent } from "@/common/root-route/root-component";
import { fetchSession } from "@/common/server-fn/session";
import { detectLanguage } from "@/i18n/detect-language";
import { ensureLanguageOn } from "@/i18n/init";
import { getActiveI18n } from "@/i18n/request-scope";
import { getDashboardOrigin, globalMetadata } from "@/common/lib/document-head";
import { getPersistedWorkspaceId } from "@/features/workspaces/lib/selection";

import appCss from "../style.css?inline";

export const Route = createRootRouteWithContext<RouterContext>()({
  notFoundComponent: NotFoundPage,
  errorComponent: ErrorPage,
  shellComponent: ShellComponent,
  component: RootComponent,
  beforeLoad: async () => {
    const language = detectLanguage();
    const sessionPromise = fetchSession();
    const dashboardOrigin = getDashboardOrigin();
    const workspaceId = getPersistedWorkspaceId();
    // Applied before this route's component renders (both on the server and
    // on the client's initial hydration pass) so the first render on each
    // side uses the same language — no hydration mismatch. Skipped when
    // unchanged (the common case past the first navigation) to avoid paying
    // i18next's resource-swap/subscriber-notify cost on every route change;
    // run alongside the (independent) session fetch rather than after it.
    // `getActiveI18n()` is this request's own instance on the server (never the
    // shared singleton — that is what let a concurrent request clobber the
    // rendered language, w6/m103 Bug B) and the singleton on the client.
    const activeI18n = getActiveI18n();
    if (activeI18n.language !== language) {
      // Lazy-loaded non-default catalog must be registered before the switch,
      // or SSR/first render would show raw keys (w9/m60 t003).
      await ensureLanguageOn(activeI18n, language);
      await activeI18n.changeLanguage(language);
    }
    // `aal2Required` rides along with the session: it is the same whoami call's
    // other answer (a live session that owes a second factor), and `requireAuth`
    // needs it to redirect to the step-up (w4/m17).
    const { session, aal2Required } = await sessionPromise;
    return {
      session,
      aal2Required,
      language,
      dashboardOrigin,
      workspaceId,
    };
  },
  loader: ({ context }) => {
    return {
      session: context.session,
      language: context.language,
      dashboardOrigin: context.dashboardOrigin,
      workspaceId: context.workspaceId,
    };
  },
  head: ({ loaderData }) => ({
    ...globalMetadata(
      loaderData?.dashboardOrigin,
      loaderData?.language ?? "en",
    ),
    links: [
      {
        rel: "icon",
        href: "/favicon.ico",
      },
      {
        rel: "apple-touch-icon",
        href: "/logo.png",
      },
    ],
    styles: [
      // appCss must stay the first head style: both it and Ory's theme sheet
      // declare @layer, and layer precedence is set by declaration order. The
      // Ory sheet itself no longer ships on every page — only the routes that
      // render Ory flow components add it via `oryThemeStyle`
      // (common/lib/ory/theme-styles.ts), and the router emits route head
      // styles after the root's, so its `ory-elements` layer is still declared
      // after Tailwind's `base` there (or our preflight would outrank Ory's
      // component styles and unstyle its forms). style.css's token bridge
      // still wins from first position because it's unlayered (see the bridge
      // block there).
      {
        children: appCss,
      },
    ],
  }),
});
