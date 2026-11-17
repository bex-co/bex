import { config } from "@/config/config";

/** How long a sign-out waits for its audit report before going ahead. */
export const SIGN_OUT_REPORT_TIMEOUT_MS = 2000;

const RECORD_SIGN_OUT = "mutation RecordSignOut { recordSignOut }";

/**
 * Tell bex-api the member is signing out, while the session still exists, so
 * each of their workspaces' audit logs records Render's `LogoutEvent`
 * (w9/m452). Kratos ends the session without telling bex-api, so this report
 * is the only record of it.
 *
 * Best effort: it never throws and never waits past
 * `SIGN_OUT_REPORT_TIMEOUT_MS`, so a failing report never keeps anyone signed
 * in. A plain request, not the Apollo client, whose 401 handling would
 * redirect a deleted account mid-sign-out and whose cache is about to be
 * cleared.
 */
export async function reportSignOut(): Promise<void> {
  try {
    const res = await fetch(config.apiUrl, {
      method: "POST",
      credentials: "include",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ query: RECORD_SIGN_OUT }),
      signal: AbortSignal.timeout(SIGN_OUT_REPORT_TIMEOUT_MS),
    });
    await res.body?.cancel();
  } catch {
    // The sign-out goes ahead without its row.
  }
}
