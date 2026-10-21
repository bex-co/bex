// Free-plan expiry as bex-api reports it (ADR009 §Free-plan expiry, w8/m126):
// a free database expires at `expiresAt`, is suspended then, and is deleted
// 14 days later unless it is upgraded. The dashboard warns 7 days ahead, the
// same lead as bex-api's first email.

const DAY_MS = 24 * 60 * 60 * 1000;
export const FREE_EXPIRY_WARNING_MS = 7 * DAY_MS;
export const FREE_EXPIRY_GRACE_MS = 14 * DAY_MS;

/** none: paid (or no instant); active: free, not near expiry; soon: within the warning lead; expired: past expiresAt. */
export type FreeExpiryState = "none" | "active" | "soon" | "expired";

export function freeExpiryState(
  expiresAt: string | null | undefined,
  now: number,
): FreeExpiryState {
  const at = expiresAt ? Date.parse(expiresAt) : Number.NaN;
  if (Number.isNaN(at)) return "none";
  if (now >= at) return "expired";
  if (now >= at - FREE_EXPIRY_WARNING_MS) return "soon";
  return "active";
}

/** The instant an expired free database is deleted: expiresAt + 14 days. */
export function freeDeletionAt(expiresAt: string): string {
  return new Date(Date.parse(expiresAt) + FREE_EXPIRY_GRACE_MS).toISOString();
}

/** An instant's UTC calendar date, "2026-12-02 UTC" — the same on server and client. */
export function utcDate(instant: string): string {
  return `${new Date(Date.parse(instant)).toISOString().slice(0, 10)} UTC`;
}

/** True near or past expiry: the states that warn. */
export function isExpiryWarning(state: FreeExpiryState): boolean {
  return state === "soon" || state === "expired";
}

/** The text colour of a warning in that state. */
export function expiryTone(state: FreeExpiryState): string {
  return state === "expired"
    ? "text-destructive"
    : "text-amber-700 dark:text-amber-400";
}
