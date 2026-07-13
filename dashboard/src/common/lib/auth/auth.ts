import { redirect } from "@tanstack/react-router";
import type { RouterContext } from "@/common/types/router-context";

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
  aal: undefined,
} as const;

/**
 * What `requireAuth` reads off the root route's context — narrowed from the
 * real `RouterContext` rather than restated, so the guard provably reads what
 * the root's `beforeLoad` actually supplies.
 */
type AuthContext = Pick<RouterContext, "session" | "aal2Required">;

/**
 * Creates a beforeLoad function that checks authentication using the root route context.
 * The root route's beforeLoad fetches the Kratos session once and passes it down via
 * context, so this function avoids duplicate network requests.
 *
 * Someone owing a second factor has no usable session either (`whoami` 403s), but
 * they are already signed in — a sign-in form is not what they need. The session
 * fetch already knows which case this is, so say so in the redirect (`aal=aal2`)
 * rather than leave the login page to rediscover it by minting a trial flow (w4/m17).
 */
export const requireAuth = (redirectPath?: string) => {
  return ({ context }: { context: AuthContext }): void => {
    if (!context.session) {
      throw redirect({
        to: "/auth/login",
        search: {
          ...EMPTY_LOGIN_SEARCH,
          next: redirectPath,
          aal: context.aal2Required ? "aal2" : undefined,
        },
      });
    }
  };
};
