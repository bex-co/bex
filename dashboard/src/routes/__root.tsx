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
