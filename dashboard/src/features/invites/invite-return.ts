import { safeNext } from "@/common/lib/safe-next";

const KEY = "bex.inviteReturn";

export function rememberInviteReturn(href: string) {
  const next = safeNext(href);
  // Billing and authentication are intermediate hops, not dashboard exits.
  if (
    next.startsWith("/setup/") ||
    next.startsWith("/auth/") ||
    next.startsWith("/invite")
  )
    return;
  try {
    window.sessionStorage.setItem(KEY, next);
  } catch {
    /* optional return hint */
  }
}

export function takeInviteReturn(): string {
  try {
    const next = safeNext(window.sessionStorage.getItem(KEY));
    window.sessionStorage.removeItem(KEY);
    return next;
  } catch {
    return "/";
  }
}
