/**
 * Deletes started from the dashboard itself (w4/645). Between a delete's
 * commit and the navigation home, a resource page is still mounted and its
 * polls (its own and its sections': a datastore's allowlist, credentials,
 * persistence) read the id as not found. Apollo writes that null to the shared
 * root field, the page's detail query settles empty, and `useNotFoundRedirect`
 * would toast "doesn't exist or was deleted" beside the delete's own success.
 *
 * While a delete is in flight, and for a short grace after it succeeds (the
 * delete flow owns the navigation), the redirect stands down. A failed delete
 * re-arms it at once; a direct visit to a dead id is unaffected.
 */
const SUCCESS_GRACE_MS = 10_000;

let inFlight = 0;
let graceUntil = 0;

export function ownDeleteActive(now = Date.now()): boolean {
  return inFlight > 0 || now < graceUntil;
}

/** Runs one delete mutation inside the registry. */
export async function trackOwnDelete<T>(run: () => Promise<T>): Promise<T> {
  inFlight += 1;
  let ok = false;
  try {
    const result = await run();
    ok = true;
    return result;
  } finally {
    inFlight -= 1;
    if (ok) graceUntil = Date.now() + SUCCESS_GRACE_MS;
  }
}

/** Test-only: forget every tracked delete. */
export function resetOwnDeletesForTests(): void {
  inFlight = 0;
  graceUntil = 0;
}
