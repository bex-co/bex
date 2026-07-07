import { createRootRouteWithContext } from "@tanstack/react-router";
import type { RouterContext } from "@/router";
import NotFoundPage from "@/common/root-route/not-found-page";
import ErrorPage from "@/common/root-route/error-page";
import { ShellComponent } from "@/common/root-route/shell-component";
import { RootComponent } from "@/common/root-route/root-component";
import { fetchSession } from "@/common/server-fn/session";

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
    return {
      session: await fetchSession(),
    };
  },
  loader: ({ context }) => {
    return {
      session: context.session,
    };
  },
});
