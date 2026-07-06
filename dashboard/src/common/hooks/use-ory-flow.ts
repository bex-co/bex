import { useEffect, useState } from "react";
import type {
  LoginFlow,
  RegistrationFlow,
  RecoveryFlow,
  SettingsFlow,
} from "@ory/client-fetch";
import { createFrontendApi } from "@/common/lib/ory/frontend";
import { KRATOS_PUBLIC_URL } from "@/common/lib/ory/config";

type OryFlowMap = {
  login: LoginFlow;
  registration: RegistrationFlow;
  recovery: RecoveryFlow;
  settings: SettingsFlow;
};

/**
 * Fetches (client-side) the Ory Kratos self-service flow this page renders.
 * If the URL has no `?flow=` id yet, kicks off a fresh browser flow at
 * Kratos directly — Kratos redirects back here once one exists.
 *
 * `returnTo` must NOT be this page's own URL. When a valid session already
 * exists, Kratos's `/self-service/{kind}/browser` short-circuits — it skips
 * creating a flow entirely and redirects straight to `return_to`. Pointing
 * that at this same login/registration page (e.g. `window.location.href`)
 * recreates the exact same no-flow URL, which immediately re-triggers this
 * effect — an infinite redirect loop. Always pass a destination outside the
 * auth pages (default: dashboard home).
 */
export function useOryFlow<K extends keyof OryFlowMap>(
  kind: K,
  flowId: string | undefined,
  returnTo = "/",
): OryFlowMap[K] | null {
  const [flow, setFlow] = useState<OryFlowMap[K] | null>(null);

  useEffect(() => {
    let cancelled = false;
    const returnUrl = new URL(returnTo, window.location.origin).toString();
    const restartUrl = `${KRATOS_PUBLIC_URL}/self-service/${kind}/browser?return_to=${encodeURIComponent(returnUrl)}`;

    async function load() {
      if (!flowId) {
        window.location.href = restartUrl;
        return;
      }

      const api = createFrontendApi();
      try {
        const result = await (kind === "login"
          ? api.getLoginFlow({ id: flowId })
          : kind === "registration"
            ? api.getRegistrationFlow({ id: flowId })
            : kind === "recovery"
              ? api.getRecoveryFlow({ id: flowId })
              : api.getSettingsFlow({ id: flowId }));
        if (!cancelled) setFlow(result as OryFlowMap[K]);
      } catch {
        // Flow expired, was already used, or doesn't exist — restart it.
        if (!cancelled) {
          window.location.href = restartUrl;
        }
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [kind, flowId, returnTo]);

  return flow;
}
