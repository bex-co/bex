import { useCallback, useState } from "react";
import { startGoogleSignIn } from "@/features/auth/lib/google-sign-in";

export interface UseGoogleSignInResult {
  /** Signs the session in again with Google, then returns to this page. */
  start: () => Promise<void>;
  /** True from the click until the browser leaves for Google. */
  busy: boolean;
  /** Kratos did not start the sign-in; the button can be pressed again. */
  failed: boolean;
}

/**
 * "Sign in with Google" for a member a workspace refuses for want of a Google
 * sign-in (w7/m465): the blocked page, and the admin's lock-out refusal on
 * the setting's card. It comes back to the page it was pressed on.
 */
export function useGoogleSignIn(): UseGoogleSignInResult {
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);

  const start = useCallback(async () => {
    setBusy(true);
    setFailed(false);
    try {
      // Stays busy on success: the page is leaving for Google.
      await startGoogleSignIn(window.location.href);
    } catch {
      setFailed(true);
      setBusy(false);
    }
  }, []);

  return { start, busy, failed };
}
