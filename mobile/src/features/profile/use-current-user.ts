import { useEffect, useMemo, useState } from "react";
import { authManager, useAuth } from "@/features/auth/auth-provider";
import { mobileConfig } from "@/features/auth/config";
import { CurrentUserClient } from "./current-user-client";
import {
  CurrentUserController,
  type CurrentUserState,
} from "./current-user-controller";

export type UseCurrentUser = CurrentUserState & {
  /** Re-run the read after an offline/unavailable result. */
  retry: () => void;
};

/**
 * Resolve the signed-in person's name/email for the drawer footer through
 * Render's `GET /v1/users`. The read is bounded, authenticated via the shared
 * {@link authManager} credentials, and torn down at the identity boundary: it
 * loads when a session appears and resets (aborting any in-flight request)
 * whenever the signed-in session id changes or the caller signs out. Name and
 * email live only in this in-memory state — never AsyncStorage, logs, or crash
 * output.
 */
export function useCurrentUser(): UseCurrentUser {
  const { state: auth } = useAuth();
  const sessionId = auth.status === "signedIn" ? auth.session.sessionId : null;

  const controller = useMemo(
    () =>
      new CurrentUserController(
        new CurrentUserClient(mobileConfig.apiOrigin, {
          getAccessToken: () => authManager.getAccessToken(),
          forceRefresh: () => authManager.forceRefresh(),
        }),
      ),
    [],
  );

  const [state, setState] = useState<CurrentUserState>(() =>
    controller.getState(),
  );
  useEffect(() => controller.subscribe(setState), [controller]);

  // The controller is stable (useMemo []), so binding the identity effect to it
  // never refires on render; it loads when a session appears and resets
  // (aborting any in-flight read) when the identity changes or signs out.
  useEffect(() => {
    if (sessionId) void controller.load();
    else controller.reset();
  }, [controller, sessionId]);
  useEffect(() => () => controller.reset(), [controller]);

  return useMemo(
    () => ({ ...state, retry: () => void controller.load() }),
    [controller, state],
  );
}
