import { createRootRouteWithContext } from "@tanstack/react-router";
import type { RouterContext } from "@/router";
import NotFoundPage from "@/common/root-route/not-found-page";
import ErrorPage from "@/common/root-route/error-page";
import { ShellComponent } from "@/common/root-route/shell-component";
import { RootComponent } from "@/common/root-route/root-component";
import { fetchSession } from "@/common/server-fn/session";
import { detectLanguage } from "@/i18n/detect-language";
import i18n from "@/i18n/init";

import appCss from "../style.css?inline";
import oryElementsCss from "@ory/elements-react/theme/styles.css?inline";

export const Route = createRootRouteWithContext<RouterContext>()({
  head: () => ({
    meta: [
      {
        charSet: "utf-8",
      },
      {
        name: "viewport",
        content: "width=device-width, initial-scale=1",
      },
      {
        title: "bex dashboard",
      },
    ],
    links: [
      {
        rel: "icon",
        href: "/favicon.ico",
      },
    ],
    styles: [
      // Order matters: both sheets declare @layer, and layer precedence is
      // set by declaration order. Ory's `ory-elements` layer must be declared
      // after Tailwind's `base`, or our preflight (`button { background:
      // transparent }`) outranks Ory's component styles and unstyles its
      // forms. style.css's token bridge still wins from first position
      // because it's unlayered (see the bridge block there).
      {
        children: appCss,
      },
      {
        children: oryElementsCss,
      },
    ],
  }),
  notFoundComponent: NotFoundPage,
  errorComponent: ErrorPage,
  shellComponent: ShellComponent,
  component: RootComponent,
  beforeLoad: async () => {
    const language = detectLanguage();
    const sessionPromise = fetchSession();
    // Applied before this route's component renders (both on the server and
    // on the client's initial hydration pass) so the first render on each
    // side uses the same language — no hydration mismatch. Skipped when
    // unchanged (the common case past the first navigation) to avoid paying
    // i18next's resource-swap/subscriber-notify cost on every route change;
    // run alongside the (independent) session fetch rather than after it.
    if (i18n.language !== language) await i18n.changeLanguage(language);
    // `aal2Required` rides along with the session: it is the same whoami call's
    // other answer (a live session that owes a second factor), and `requireAuth`
    // needs it to redirect to the step-up (w4/m17).
    const { session, aal2Required } = await sessionPromise;
    return {
      session,
      aal2Required,
      language,
    };
  },
  loader: ({ context }) => {
    return {
      session: context.session,
      language: context.language,
    };
  },
});
