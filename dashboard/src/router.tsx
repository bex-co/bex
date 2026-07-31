import { createRouter as createTanStackRouter } from "@tanstack/react-router";
import { routeTree } from "./routeTree.gen";
import { getClient } from "./common/apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import ErrorPage from "@/common/root-route/error-page";
import NotFoundPage from "@/common/root-route/not-found-page";
import RoutePending from "@/common/root-route/route-pending";

export type { RouterContext } from "@/common/types/router-context";

export function getRouter() {
  const client = getClient();
  const router = createTanStackRouter({
    routeTree,
    scrollRestoration: true,
    // "intent" preloads a link's chunk + loader on hover; the (default) 30s
    // preloadStaleTime lets the actual click reuse that result instead of
    // re-running it (the previous explicit `0` discarded every preload, so
    // each detail navigation ran its title query twice).
    defaultPreload: "intent",
    defaultErrorComponent: ErrorPage,
    defaultNotFoundComponent: NotFoundPage,
    defaultPendingComponent: RoutePending,
    // Hold the outgoing page through quick transitions instead of swapping to
    // the pending fallback at 0ms — with the previous 0/0 every navigation
    // that wasn't same-tick unmounted the page into a DOM-less pending
    // component (a blank white document) and mounted the chrome twice. 150ms
    // covers cached chunks / memoized session reads; anything slower shows
    // RoutePending, held ≥150ms so it can't strobe for a single frame.
    defaultPendingMs: 150,
    defaultPendingMinMs: 150,
    context: {
      client,
    },
    dehydrate: () => {
      // console.log("dehydrate", client.cache.extract());
      return {
        apolloState: client.cache.extract() as Record<string, string>,
      };
    },
    hydrate: (data) => {
      // console.log("hydrate", data.apolloState);
      client.cache.restore(data.apolloState as Record<string, string>);
    },
    Wrap(props) {
      return <ApolloProvider client={client}>{props.children}</ApolloProvider>;
    },
  });

  return router;
}
declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof getRouter>;
  }
}
