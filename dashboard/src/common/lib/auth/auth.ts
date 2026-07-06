import { redirect } from "@tanstack/react-router";

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
        search: {
          next: redirectPath,
          flow: undefined,
        },
      });
    }
  };
};
