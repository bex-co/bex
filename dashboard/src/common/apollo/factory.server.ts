import {
  ApolloClient,
  ApolloLink,
  HttpLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { config } from "@/config/config";
import { apolloCacheConfig } from "./cache";

import { getCookie } from "@tanstack/react-start/server";

const loggingLink = new ApolloLink((operation, forward) => {
  const start = Date.now();
  console.log(`[Apollo SSR] ${operation.operationName} →`, operation.variables);
  return new Observable((observer) => {
    const sub = forward(operation).subscribe({
      next: (result) => {
        console.log(
          `[Apollo SSR] ${operation.operationName} ← ${Date.now() - start}ms`,
          result.errors ?? "ok",
        );
        observer.next(result);
      },
      error: (err) => {
        console.log(`[Apollo SSR] ${operation.operationName} ✗`, err);
        observer.error(err);
      },
      complete: () => observer.complete(),
    });
    return () => sub.unsubscribe();
  });
});

/**
 * Create an Apollo client for server-side rendering. Forwards the incoming
 * request cookie to bex-api as a Bearer token (docs/ADR006-bex-api.md's auth model)
 * so authenticated queries work on the server.
 *
 * TODO: nothing sets the "bex-dashboard-token" cookie yet — there's no login
 * flow in this scaffold. `token` will be undefined until real auth lands.
 */
export function createApolloSsrClient() {
  const token = getCookie("bex-dashboard-token");
  const httpLink = new HttpLink({
    uri: config.ssrApiUrl,
    fetch,
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  return new ApolloClient({
    ssrMode: true,
    link: import.meta.env.DEV
      ? ApolloLink.from([loggingLink, httpLink])
      : httpLink,
    cache: new InMemoryCache(apolloCacheConfig),
  });
}
