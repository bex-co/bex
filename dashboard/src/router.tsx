import { createRouter as createTanStackRouter } from "@tanstack/react-router";
import { routeTree } from "./routeTree.gen";
import { getClient } from "./common/apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import ErrorPage from "@/common/root-route/error-page";
import NotFoundPage from "@/common/root-route/not-found-page";
import PendingRouteTitle from "@/common/lib/document-head/pending-route-title";

export type { RouterContext } from "@/common/types/router-context";

export function getRouter() {
  const client = getClient();
  const router = createTanStackRouter({
    routeTree,
    scrollRestoration: true,
    defaultPreload: "intent",
    defaultPreloadStaleTime: 0,
    defaultErrorComponent: ErrorPage,
    defaultNotFoundComponent: NotFoundPage,
    defaultPendingComponent: PendingRouteTitle,
    defaultPendingMs: 0,
    defaultPendingMinMs: 0,
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
