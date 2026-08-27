import { useNavigate } from "@tanstack/react-router";
import { EMPTY_LOGIN_SEARCH } from "./auth";
import { currentHref } from "@/common/lib/safe-next";

/**
 * An onClick that sends an expired session back to sign-in, carrying `next` back
 * to the current page so login round-trips the user to where they were (the
 * login page normalizes `next` through `safeNext`). This is the *manual* path —
 * the button in the expired-session card (w3/m80 t002); the client Apollo auth
 * link (t001) usually auto-redirects first, so a real user rarely needs it, but
 * it's the deterministic recovery when the auto-redirect hasn't landed yet.
 *
 * A soft router navigation, not a hard reload: on a manual click the app is
 * still mounted, and the login page owns the rest of the flow.
 */
export function useSignInAgain(): () => void {
  const navigate = useNavigate();
  return () => {
    const next = currentHref() || undefined;
    void navigate({ to: "/auth/login", search: { ...EMPTY_LOGIN_SEARCH, next } });
  };
}
