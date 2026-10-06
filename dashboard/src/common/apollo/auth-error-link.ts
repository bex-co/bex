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

/** bex-api's refusal of a human whose email is not verified (ADR075 D8
 * revision 2026-10-06, w2/m168). */
const EMAIL_VERIFICATION_REQUIRED = "EMAIL_VERIFICATION_REQUIRED";

/** True when a parsed bex-api error body (`{error,message,id,code,params}`)
 * carries the verification refusal. */
function bodyHasVerificationCode(bodyText: string): boolean {
  try {
    const body: unknown = JSON.parse(bodyText);
    return (
      typeof body === "object" &&
      body !== null &&
      (body as { code?: unknown }).code === EMAIL_VERIFICATION_REQUIRED
    );
  } catch {
    return false;
  }
}

/**
 * Whether an error is bex-api's `EMAIL_VERIFICATION_REQUIRED` refusal. The
 * auth middleware writes it as a REST-shaped 403 body BEFORE GraphQL runs, so
 * on `/graphql` it arrives as a transport `ServerError` (status 403, JSON
 * `bodyText`), not a GraphQL error. Keyed on the stable `code`, never the
 * message, and unwraps a `cause` chain like `isUnauthenticatedError`.
 */
export function isEmailVerificationRequiredError(error: unknown): boolean {
  if (ServerError.is(error)) {
    return error.statusCode === 403 && bodyHasVerificationCode(error.bodyText);
  }
  const cause = (error as { cause?: unknown } | null)?.cause;
  return (
    cause != null && cause !== error && isEmailVerificationRequiredError(cause)
  );
}

/** A GraphQL result that carries the code as `extensions.code` instead (the
 * shape bex-api uses for errors raised inside resolvers). */
function resultHasVerificationCode(result: {
  errors?: ReadonlyArray<{ extensions?: Record<string, unknown> }>;
}): boolean {
  return (result.errors ?? []).some(
    (item) => item.extensions?.["code"] === EMAIL_VERIFICATION_REQUIRED,
  );
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
 *
 * The same link routes bex-api's `EMAIL_VERIFICATION_REQUIRED` refusal to
 * `onEmailVerificationRequired` (ADR075 D8 revision, w2/m168). This is the
 * backstop behind `EmailVerificationGate` for the cases the session check
 * cannot see: a bare route's queries, or a session that turned unverified
 * after it was memoized. As with 401, the error still surfaces.
 */
export function createAuthErrorLink(
  onUnauthorized: () => void,
  onEmailVerificationRequired: () => void = () => {},
): ApolloLink {
  return new ApolloLink((operation, forward) => {
    return new Observable((observer) => {
      const sub = forward(operation).subscribe({
        next: (result) => {
          if (resultHasVerificationCode(result)) onEmailVerificationRequired();
          observer.next(result);
        },
        error: (error: ErrorLike) => {
          if (isUnauthenticatedError(error)) onUnauthorized();
          else if (isEmailVerificationRequiredError(error)) {
            onEmailVerificationRequired();
          }
          observer.error(error);
        },
        complete: () => observer.complete(),
      });
      return () => sub.unsubscribe();
    });
  });
}
