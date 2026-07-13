import {
  ApolloClient,
  ApolloLink,
  HttpLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { config } from "@/config/config";
import { apolloCacheConfig } from "./cache";

import { getRequestHeader } from "@tanstack/react-start/server";

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
 * request's Cookie header to bex-api so the ory_kratos_session cookie reaches
 * Kratos whoami and authenticated SSR queries return real data on first paint
 * (docs/ADR012-auth.md). When there is no session cookie the header is omitted
 * and bex-api sees an unauthenticated request — no crash, correct public-route
 * behavior.
 */
export function createApolloSsrClient() {
  const cookieHeader = getRequestHeader("cookie");
  const httpLink = new HttpLink({
    uri: config.ssrApiUrl,
    fetch,
    headers: cookieHeader ? { Cookie: cookieHeader } : {},
  });
  return new ApolloClient({
    ssrMode: true,
    link: import.meta.env.DEV
      ? ApolloLink.from([loggingLink, httpLink])
      : httpLink,
    cache: new InMemoryCache(apolloCacheConfig),
  });
}
