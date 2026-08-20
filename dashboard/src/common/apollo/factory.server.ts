import {
  ApolloClient,
  ApolloLink,
  HttpLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { config } from "@/config/config";
import { apolloCacheConfig } from "./cache";
import { createRetryLink } from "./retry-link";
import { logServerError } from "@/common/lib/server-log";

import { getRequestHeader } from "@tanstack/react-start/server";

// Stateless and shared across the per-request SSR clients — one instance, not
// one per render.
const retryLink = createRetryLink();

/** DEV-only: verbose operation + variable dump. Never enable in production. */
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
 * Prod (and DEV): transport failures only — operation name + error message,
 * never variables/cookies (w4/m88). GraphQL result.errors still ride the
 * normal SSR error page path via reportRouteError.
 */
const errorOnlyLink = new ApolloLink((operation, forward) => {
  return new Observable((observer) => {
    const sub = forward(operation).subscribe({
      next: (result) => observer.next(result),
      error: (err) => {
        const name = operation.operationName || "anonymous";
        const detail =
          err instanceof Error && err.message
            ? err.message
            : "apollo_ssr_transport_error";
        logServerError({
          msg: `apollo_ssr ${name}: ${detail}`,
          status: 502,
        });
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
  // Retry transient read failures (w1/m52 t003) on the SSR side too, so a
  // page rendered mid-roll dehydrates data instead of a stranded error state.
  const links = [errorOnlyLink, retryLink, httpLink];
  return new ApolloClient({
    ssrMode: true,
    link: ApolloLink.from(
      import.meta.env.DEV ? [loggingLink, ...links] : links,
    ),
    cache: new InMemoryCache(apolloCacheConfig),
  });
}
