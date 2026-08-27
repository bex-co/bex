import { ApolloLink, Observable, type ErrorLike } from "@apollo/client";
import { ServerError } from "@apollo/client/errors";

/**
 * Whether an error — one surfaced by the link chain, or a settled query's
 * `error` — is bex-api's 401: an expired or absent session, not a GraphQL
 * execution error and not a 5xx blip. bex-api rejects an invalid session at its
 * auth gate before any resolver runs (docs/ADR012-auth.md), so expiry always
 * arrives as a transport `ServerError` with `statusCode` 401 — never a GraphQL
 * `UNAUTHENTICATED` extension. This is the one classifier both the redirect
 * (auth-redirect.ts) and the error surfaces (w3/m80 t002) read, so "a 401 means
 * re-auth" is decided in exactly one place.
 *
 * Unwraps a `cause` chain because some Apollo paths wrap a link's terminal
 * error; the `ServerError` underneath is what carries the HTTP status.
 */
export function isUnauthenticatedError(error: unknown): boolean {
  if (ServerError.is(error)) return error.statusCode === 401;
  const cause = (error as { cause?: unknown } | null)?.cause;
  return cause != null && cause !== error && isUnauthenticatedError(cause);
}

/**
 * Front-of-chain link that reacts to a 401 on an already-mounted page (w3/m80
 * t001). The only redirect-to-login path used to live in the root route's
 * `beforeLoad`, so a session that expired AFTER a page mounted surfaced as a
 * generic "The request to bex-api failed" card with a dead-end retry. This
 * calls `onUnauthorized` the moment it sees the 401, then still lets the error
 * surface so the query settles into its (now auth-aware) error state while the
 * redirect is arranged — it never swallows the error or retries.
 *
 * Not restricted to reads: a mutation that 401s is just as much an expired
 * session, and re-auth is the right response to either. Whether the session is
 * *truly* gone (vs. a transient bex-api auth-upstream blip) is `onUnauthorized`'s
 * call, not this link's.
 */
export function createAuthErrorLink(onUnauthorized: () => void): ApolloLink {
  return new ApolloLink((operation, forward) => {
    return new Observable((observer) => {
      const sub = forward(operation).subscribe({
        next: (result) => observer.next(result),
        error: (error: ErrorLike) => {
          if (isUnauthenticatedError(error)) onUnauthorized();
          observer.error(error);
        },
        complete: () => observer.complete(),
      });
      return () => sub.unsubscribe();
    });
  });
}
