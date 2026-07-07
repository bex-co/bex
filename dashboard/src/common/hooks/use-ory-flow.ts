import { useEffect, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import type {
  LoginFlow,
  RegistrationFlow,
  RecoveryFlow,
  SettingsFlow,
  FrontendApi,
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
 * Per-tab persistence of the active flow id, so a mid-form reload (or the
 * two-step recovery flow's email → code hop) resumes the same flow instead
 * of starting over. sessionStorage (not localStorage): flows are per-tab,
 * short-lived state — sharing them across tabs would cross CSRF contexts.
 */
const storageKey = (kind: keyof OryFlowMap) => `bex.ory-flow.${kind}`;

function readStoredFlowId(kind: keyof OryFlowMap): string | null {
  try {
    return window.sessionStorage.getItem(storageKey(kind));
  } catch {
    return null; // storage disabled (private mode etc.) — degrade to in-memory only
  }
}

function writeStoredFlowId(kind: keyof OryFlowMap, id: string | null) {
  try {
    if (id === null) window.sessionStorage.removeItem(storageKey(kind));
    else window.sessionStorage.setItem(storageKey(kind), id);
  } catch {
    // storage disabled — flow still lives in React state for this mount
  }
}

/** Call from a flow's onSuccess so the next visit doesn't retry a used-up flow id. */
export function clearStoredOryFlow(kind: keyof OryFlowMap) {
  writeStoredFlowId(kind, null);
}

function getFlow<K extends keyof OryFlowMap>(
  api: FrontendApi,
  kind: K,
  id: string,
): Promise<OryFlowMap[K]> {
  const req =
    kind === "login"
      ? api.getLoginFlow({ id })
      : kind === "registration"
        ? api.getRegistrationFlow({ id })
        : kind === "recovery"
          ? api.getRecoveryFlow({ id })
          : api.getSettingsFlow({ id });
  return req as Promise<OryFlowMap[K]>;
}

/**
 * Creates a fresh flow via AJAX. Kratos's `/self-service/{kind}/browser`
 * endpoints return the flow as JSON (no redirect) when called via fetch —
 * the same call opened as a link would 303 back to us with `?flow=` in the
 * URL, which is exactly what this hook exists to avoid. Anti-CSRF cookies
 * still get set: auth and dashboard share a site (bex.co / localhost), so
 * the browser sends them on Ory Elements' subsequent form-submit fetches.
 */
function createFlow<K extends keyof OryFlowMap>(
  api: FrontendApi,
  kind: K,
  returnTo: string | undefined,
): Promise<OryFlowMap[K]> {
  const req =
    kind === "login"
      ? api.createBrowserLoginFlow({ returnTo })
      : kind === "registration"
        ? api.createBrowserRegistrationFlow({ returnTo })
        : kind === "recovery"
          ? api.createBrowserRecoveryFlow({ returnTo })
          : api.createBrowserSettingsFlow({ returnTo });
  return req as Promise<OryFlowMap[K]>;
}

/** Extracts Kratos's machine-readable error id (e.g. `session_already_available`). */
async function oryErrorId(err: unknown): Promise<string | undefined> {
  const response = (err as { response?: Response })?.response;
  if (!response) return undefined;
  try {
    const body = (await response.clone().json()) as {
      error?: { id?: string };
    };
    return body.error?.id;
  } catch {
    return undefined;
  }
}

/**
 * Provides (client-side) the Ory Kratos self-service flow this page renders,
 * keeping the flow reference OUT of the address bar:
 *
 * - No known flow → mint one via AJAX (`createBrowser*Flow`), hold it in
 *   React state + sessionStorage. The URL never changes.
 * - `?flow=` present in the URL (arrivals we don't control: recovery /
 *   verification email links, OIDC round trips, Ory Elements' own
 *   expired-mid-submit restart, legacy bookmarks) → persist the id to
 *   sessionStorage, then strip the param with a replace-navigation; the
 *   effect re-runs and loads the flow from storage. One-shot, leaves a
 *   clean URL and an intact back-button.
 * - Stored/inbound id turns out expired, used, or foreign → silently fall
 *   through to minting a fresh flow. A days-old bookmark just works.
 *
 * `returnTo` must NOT be this page's own URL: Kratos short-circuits flow
 * creation when a valid session exists (`session_already_available`) and
 * we navigate to `returnTo` — pointing it back at an auth page would loop.
 */
export function useOryFlow<K extends keyof OryFlowMap>(
  kind: K,
  flowIdFromUrl: string | undefined,
  returnTo = "/",
): OryFlowMap[K] | null {
  const [flow, setFlow] = useState<OryFlowMap[K] | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    let cancelled = false;

    // Inbound ?flow= — adopt it, then scrub the URL. The search-param change
    // re-runs this effect, which then loads the id from sessionStorage.
    if (flowIdFromUrl) {
      writeStoredFlowId(kind, flowIdFromUrl);
      void navigate({
        to: ".",
        search: (prev: Record<string, unknown>) => ({
          ...prev,
          flow: undefined,
        }),
        replace: true,
      });
      return;
    }

    const api = createFrontendApi();
    const returnUrl = new URL(returnTo, window.location.origin).toString();

    async function load() {
      const knownId = readStoredFlowId(kind);
      if (knownId) {
        try {
          const existing = await getFlow(api, kind, knownId);
          if (!cancelled) setFlow(existing);
          return;
        } catch {
          writeStoredFlowId(kind, null); // expired/used/foreign — mint fresh
        }
      }

      try {
        let fresh: OryFlowMap[K];
        try {
          fresh = await createFlow(api, kind, returnUrl);
        } catch (err) {
          // Kratos rejects return_to values missing from its
          // allowed_return_urls (e.g. localhost dev against an environment
          // that doesn't allowlist it) — the flow itself still works
          // without one, so retry rather than fail the page.
          if ((await oryErrorId(err)) !== "self_service_flow_return_to_forbidden")
            throw err;
          fresh = await createFlow(api, kind, undefined);
        }
        if (cancelled) return;
        writeStoredFlowId(kind, fresh.id);
        setFlow(fresh);
      } catch (err) {
        if (cancelled) return;
        const errorId = await oryErrorId(err);
        if (errorId === "session_already_available") {
          // Already signed in — Kratos refuses to create login/registration
          // flows. Go where the user was headed instead.
          void navigate({ to: returnTo, replace: true });
          return;
        }
        if (errorId === "session_inactive") {
          // Settings without a session — sign in first.
          void navigate({
            to: "/auth/login",
            search: { next: undefined, flow: undefined },
            replace: true,
          });
          return;
        }
        // Unknown failure — fall back to Kratos's redirect-based bootstrap
        // (readds ?flow= for this one attempt, but keeps the page usable).
        window.location.href = `${KRATOS_PUBLIC_URL}/self-service/${kind}/browser?return_to=${encodeURIComponent(returnUrl)}`;
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [kind, flowIdFromUrl, returnTo, navigate]);

  return flow;
}
