import { SetContextLink } from "@apollo/client/link/context";
import { ErrorLink } from "@apollo/client/link/error";
import { catchError, from, switchMap, throwError } from "rxjs";
import { isUnauthorized } from "./error-policy";

export type AuthCredentials = {
  getAccessToken: () => Promise<string>;
  forceRefresh: () => Promise<unknown>;
  signOut: () => Promise<unknown>;
};

/**
 * Auth header + single 401→refresh→retry links.
 *
 * SetContextLink skips recomputation when an operation already carries
 * `authorization` (logout cleanup passes an explicit bearer with
 * `skipAuthRefresh`). After a successful refresh the retry must clear that
 * stale automatic bearer so the next hop picks up the new token — otherwise
 * the retried request reuses the expired one and the session is signed out.
 */
export function createAuthLinks(credentials: AuthCredentials) {
  const authLink = new SetContextLink(async (context) => {
    if (context.headers?.authorization) return context;
    const accessToken = await credentials.getAccessToken();
    return {
      headers: {
        ...context.headers,
        authorization: `Bearer ${accessToken}`,
      },
    };
  });

  const refreshLink = new ErrorLink(({ error, operation, forward }) => {
    if (
      !isUnauthorized(error) ||
      operation.getContext().authRetried === true ||
      operation.getContext().skipAuthRefresh === true
    ) {
      return;
    }
    operation.setContext({ authRetried: true });
    return from(credentials.forceRefresh()).pipe(
      // Only a failed refresh ends the session. A non-auth failure on the
      // retried request must leave the refreshed credentials intact.
      catchError((refreshError) =>
        from(credentials.signOut()).pipe(
          switchMap(() => throwError(() => refreshError)),
        ),
      ),
      switchMap(() => {
        operation.setContext((previous) => {
          const headers = { ...(previous.headers ?? {}) };
          delete headers.authorization;
          return { headers };
        });
        return forward(operation);
      }),
    );
  });

  return { authLink, refreshLink };
}
