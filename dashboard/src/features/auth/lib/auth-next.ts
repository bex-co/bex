import { safeNext } from "@/common/lib/safe-next";

const KEY = "bex.auth.next";

/**
 * Same-tab relay for the guarded `?next=` deep link across the D8 auth
 * restructure (ADR075 D3/D8, w6/m42): sign-up → verification → login. The
 * middle hop is an Ory-Elements-internal full-page redirect whose URL Kratos
 * builds (`/auth/verification?flow=…`), so a query param cannot survive it —
 * sessionStorage carries it instead (per-tab, the same shape as the /invite
 * token relay). Values are normalized by safeNext at write AND re-validated at
 * read, so a tampered stash can never become an open redirect.
 */
export function stashAuthNext(next: string | undefined): void {
  const target = safeNext(next);
  if (target === "/") return; // the default — nothing worth carrying
  try {
    sessionStorage.setItem(KEY, target);
  } catch {
    // sessionStorage unavailable (SSR pass, privacy mode): continuity simply
    // degrades to landing on "/" after login.
  }
}

/** Read and clear the relayed target; undefined when absent or unsafe. */
export function takeAuthNext(): string | undefined {
  try {
    const raw = sessionStorage.getItem(KEY);
    sessionStorage.removeItem(KEY);
    if (raw == null) return undefined;
    const target = safeNext(raw);
    return target === "/" ? undefined : target;
  } catch {
    return undefined;
  }
}

/** Drop a pending relay (a completed login makes any stash stale). */
export function clearAuthNext(): void {
  try {
    sessionStorage.removeItem(KEY);
  } catch {
    // ignore — nothing stashed that could be read either
  }
}
