import { redirect } from "@tanstack/react-router";

/**
 * The /auth/login route's search params, all unset. TanStack Router's typed
 * search makes every navigate spell out each key — share one literal so adding
 * a param (as w4/m9's `login_challenge` did) is a one-line change, not an
 * every-call-site edit.
 */
export const EMPTY_LOGIN_SEARCH = {
  next: undefined,
  flow: undefined,
  login_challenge: undefined,
} as const;

/**
 * Creates a beforeLoad function that checks authentication using the root route context.
 * The root route's beforeLoad fetches the Kratos session once and passes it down via
 * context, so this function avoids duplicate network requests.
 */
export const requireAuth = (redirectPath?: string) => {
  return ({ context }: { context: { session: unknown } }): void => {
    if (!context.session) {
      throw redirect({
        to: "/auth/login",
        search: { ...EMPTY_LOGIN_SEARCH, next: redirectPath },
      });
    }
  };
};
